package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// TestACPSessionService_CancelDuringRun_ReturnsCancelled is the C1 regression test. The
// prompt handler records the cancel function (setCancel) only after runWG.Add(1), so once
// the workflow Run is in flight a concurrent session/cancel both (a) finds a non-nil
// cancelFn to interrupt the run and (b) is guaranteed to observe a counted runWG. Here we
// drive the cancel path end-to-end: a blocking runner is cancelled mid-run and the prompt
// must resolve with stopReason=cancelled.
func TestACPSessionService_CancelDuringRun_ReturnsCancelled(t *testing.T) {
	runner := &fakeRunner{block: true}
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: &fakeEmitter{}}
	svc.sessions.Store("sess-cancel-run", &ACPSession{ID: "sess-cancel-run"})

	params := json.RawMessage(`{"sessionId":"sess-cancel-run","prompt":[{"type":"text","text":"/workflow-1"}]}`)
	type cancelRunOutcome struct {
		result any
		err    *ACPHandlerError
	}
	done := make(chan cancelRunOutcome, 1)
	go func() {
		r, e := svc.HandleSessionPrompt(context.Background(), params)
		done <- cancelRunOutcome{result: r, err: e}
	}()

	// The runner blocks on its run context; once it has been entered, setCancel has already
	// run (it precedes runner.Run), so the recorded cancelFn is live.
	require.Eventually(t, func() bool { return runner.callCount() == 1 }, time.Second, time.Millisecond,
		"runner must enter its blocking Run before we cancel")

	_, cancelErr := svc.HandleSessionCancel(context.Background(), json.RawMessage(`{"sessionId":"sess-cancel-run"}`))
	require.Nil(t, cancelErr)

	select {
	case got := <-done:
		require.Nil(t, got.err)
		assert.Equal(t, "cancelled", stopReasonOf(t, got.result),
			"a run cancelled mid-flight must resolve with stopReason=cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("prompt did not return after session/cancel")
	}
}

// TestACPSessionService_InFlightReleasedAfterPrompt is the M6 regression test. InFlight is
// released by the deferred Store(false) when the handler returns; the JSON-RPC server
// schedules that return before it writes the response frame, so InFlight is observably
// false once HandleSessionPrompt returns and a subsequent sequential prompt is admitted
// rather than rejected as PROMPT_IN_FLIGHT.
func TestACPSessionService_InFlightReleasedAfterPrompt(t *testing.T) {
	exec := workflow.NewExecutionContext("workflow-1", "Test Workflow")
	exec.SetStepState("run", workflow.StepState{Output: "ok\n"})
	runner := &fakeRunner{execCtx: exec}
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: &fakeEmitter{}}
	session := &ACPSession{ID: "sess-seq"}
	svc.sessions.Store("sess-seq", session)

	params := json.RawMessage(`{"sessionId":"sess-seq","prompt":[{"type":"text","text":"/workflow-1"}]}`)

	_, err := svc.HandleSessionPrompt(context.Background(), params)
	require.Nil(t, err)
	assert.False(t, session.InFlight.Load(), "InFlight must be released once the handler returns")

	_, err2 := svc.HandleSessionPrompt(context.Background(), params)
	require.Nil(t, err2, "a second sequential prompt must be admitted after the first completes")
	assert.Equal(t, 2, runner.callCount(), "both sequential prompts must dispatch")
}

// TestACPSessionService_Issue1_ShutdownCancelsRunViaSetCancel is the deterministic regression
// test for issue #1 (race between setCancel and Shutdown).
//
// Pre-fix ordering: runWG.Add → ensureRunner → runner.Run → setCancel
// In that ordering, a Shutdown arriving after runWG.Add but before setCancel sees a non-zero
// counter (so it waits via runWG.Wait), but session.cancel() finds cancelFn==nil and is a
// no-op — leaving runner.Run blocked forever and Shutdown deadlocked.
//
// Post-fix ordering: setCancel → defer cancel → runWG.Add → ensureRunner → runner.Run
// A Shutdown arriving any time after setCancel finds a non-nil cancelFn, cancels the context,
// and runner.Run receives it immediately.
//
// The test drives the cancel via session.shutdown() directly (the same path Shutdown uses) and
// verifies the blocking prompt resolves with stopReason=cancelled — not a timeout/deadlock.
func TestACPSessionService_Issue1_ShutdownCancelsRunViaSetCancel(t *testing.T) {
	runStarted := make(chan struct{})
	var runDone atomic.Bool

	// blockingRunner is defined in acp_session_service_test.go (same package).
	runner := &blockingRunner{started: runStarted, done: &runDone}
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: &fakeEmitter{}}
	svc.sessions.Store("sess-issue1", &ACPSession{ID: "sess-issue1"})

	params := json.RawMessage(`{"sessionId":"sess-issue1","prompt":[{"type":"text","text":"/workflow-1"}]}`)

	type outcome struct {
		result any
		err    *ACPHandlerError
	}
	done := make(chan outcome, 1)
	go func() {
		r, e := svc.HandleSessionPrompt(context.Background(), params)
		done <- outcome{r, e}
	}()

	// Wait until runner.Run is entered. At this point the fix guarantees setCancel was already
	// called (setCancel precedes runWG.Add, which precedes runner.Run in the fixed ordering).
	<-runStarted

	// Simulate what Shutdown does: cancel the session.
	val, ok := svc.sessions.Load("sess-issue1")
	require.True(t, ok)
	session := val.(*ACPSession)
	session.shutdown()

	select {
	case got := <-done:
		require.Nil(t, got.err,
			"issue #1: a run cancelled via session.cancel() must not produce a JSON-RPC error")
		assert.Equal(t, "cancelled", stopReasonOf(t, got.result),
			"issue #1: run cancelled after setCancel is live must resolve with stopReason=cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("issue #1: prompt did not return — likely nil cancelFn race (setCancel called too late)")
	}
	assert.True(t, runDone.Load(),
		"issue #1: blocking runner must have observed context cancellation and set done=true")
}

// blockingWorkflowRepo is a WorkflowRepository that blocks Load calls until released.
// Used to exercise the semaphore ctx-cancellation path in HandleSessionNew (issue #2).
type blockingWorkflowRepo struct {
	infos   []ports.WorkflowInfo
	block   chan struct{} // closed to release all blocked Load calls
	started chan struct{} // receives one send per Load that started
}

func newBlockingWorkflowRepo(n int) *blockingWorkflowRepo {
	infos := make([]ports.WorkflowInfo, n)
	for i := range infos {
		infos[i] = ports.WorkflowInfo{
			Name:   fmt.Sprintf("wf-%d", i),
			Source: ports.SourceLocal,
		}
	}
	return &blockingWorkflowRepo{
		infos:   infos,
		block:   make(chan struct{}),
		started: make(chan struct{}, n),
	}
}

func (r *blockingWorkflowRepo) ListWithSource(_ context.Context) ([]ports.WorkflowInfo, error) {
	return r.infos, nil
}

func (r *blockingWorkflowRepo) Load(ctx context.Context, _ string) (*workflow.Workflow, error) {
	r.started <- struct{}{}
	select {
	case <-r.block:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *blockingWorkflowRepo) List(_ context.Context) ([]string, error) {
	names := make([]string, len(r.infos))
	for i, info := range r.infos {
		names[i] = info.Name
	}
	return names, nil
}

func (r *blockingWorkflowRepo) Exists(_ context.Context, name string) (bool, error) {
	for _, info := range r.infos {
		if info.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// TestACPSessionService_Issue2_SemaphoreUnblocksOnCtxCancel verifies that cancelling the
// context while HandleSessionNew is running its bounded parallel workflow loads causes all
// goroutines to exit and wg.Wait() to return — not deadlock.
//
// Pre-fix: sem <- struct{}{} was a plain channel send that blocked forever when all 8 slots
// were occupied and ctx was cancelled. With 12 workflows, 4 goroutines would queue and never
// unblock if the context was cancelled before a slot freed up.
//
// Post-fix: the select { case sem <- struct{}{}: case <-ctx.Done(): return } unblocks queued
// goroutines immediately when ctx is cancelled, allowing wg.Wait() and the handler to return.
func TestACPSessionService_Issue2_SemaphoreUnblocksOnCtxCancel(t *testing.T) {
	// 12 workflows: more than maxParallelLoads (8), so 4 goroutines will queue on the semaphore.
	const numWorkflows = 12
	repo := newBlockingWorkflowRepo(numWorkflows)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	svc := &ACPSessionService{workflowRepo: repo, logger: ports.NopLogger{}}

	handlerDone := make(chan *ACPHandlerError, 1)
	go func() {
		_, err := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
		handlerDone <- err
	}()

	// Wait until at least 8 Load calls have started (semaphore is now full).
	// The remaining 4 goroutines are blocked waiting for a slot.
	for i := range 8 {
		select {
		case <-repo.started:
		case <-time.After(2 * time.Second):
			t.Fatalf("issue #2: only %d Load calls started before timeout (expected 8)", i)
		}
	}

	// Cancel the context. The 4 queued goroutines must unblock via ctx.Done() and return.
	// The 8 running goroutines will also return via ctx.Done() in their Load implementation.
	cancelCtx()

	select {
	case <-handlerDone:
		// Handler returned — wg.Wait() unblocked correctly. Test passes.
	case <-time.After(3 * time.Second):
		t.Fatal("issue #2: HandleSessionNew deadlocked after ctx cancellation — semaphore not ctx-aware")
	}
}

// TestACPSessionService_Issue8_ShutdownRejectsNewSessions verifies that once Shutdown has
// started, HandleSessionNew returns an ACPErrInternal immediately rather than creating a
// session whose resources would be leaked (created between the two Range passes in Shutdown).
func TestACPSessionService_Issue8_ShutdownRejectsNewSessions(t *testing.T) {
	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{}, nil)

	svc := &ACPSessionService{workflowRepo: mockRepo, logger: ports.NopLogger{}}

	// Confirm that before Shutdown, HandleSessionNew succeeds.
	_, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr, "session/new must succeed before Shutdown is called")

	// Trigger shutdown.
	svc.Shutdown()

	// After Shutdown, HandleSessionNew must be rejected immediately.
	_, acpErr = svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.NotNil(t, acpErr, "issue #8: session/new must be rejected after Shutdown")
	assert.Equal(t, ACPErrInternal, acpErr.Kind,
		"issue #8: post-shutdown session/new must return ACPErrInternal")
	assert.Contains(t, acpErr.Message, "shutting down",
		"issue #8: error message must indicate the server is shutting down")
}
