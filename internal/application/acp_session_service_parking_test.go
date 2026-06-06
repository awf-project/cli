package application

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// parkingResponder is an ACPInputResponder test double that mirrors the real
// infrastructure ACPInputReader: ReadInput fires the OnPark hook then blocks until
// Respond is called (or ctx is cancelled). It lets an application-layer test drive the
// US2 conversation-parking flow without importing internal/infrastructure/acp.
type parkingResponder struct {
	responseCh chan string
	onPark     func()
	onUnpark   func()
	responses  []string
}

func newParkingResponder() *parkingResponder {
	return &parkingResponder{responseCh: make(chan string, 1)}
}

func (r *parkingResponder) ReadInput(ctx context.Context) (string, error) {
	if r.onPark != nil {
		r.onPark()
	}
	if r.onUnpark != nil {
		defer r.onUnpark()
	}
	select {
	case t := <-r.responseCh:
		return t, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (r *parkingResponder) Respond(text string) {
	r.responses = append(r.responses, text)
	select {
	case r.responseCh <- text:
	default:
	}
}

func (r *parkingResponder) SetParkHooks(onPark, onUnpark func()) {
	r.onPark = onPark
	r.onUnpark = onUnpark
}

// parkingRunner is a WorkflowRunner that parks `turns` times (each park blocks on the
// shared reader's ReadInput) before completing with execCtx. It models a multi-turn
// agent conversation that waits for user input between turns.
type parkingRunner struct {
	reader  *parkingResponder
	turns   int
	execCtx *workflow.ExecutionContext
}

func (r *parkingRunner) Run(ctx context.Context, _ string, _ map[string]any) (*workflow.ExecutionContext, error) {
	for i := 0; i < r.turns; i++ {
		if _, err := r.reader.ReadInput(ctx); err != nil {
			return nil, err
		}
	}
	return r.execCtx, nil
}

// TestACPSessionService_Prompt_ParksAndResumesAcrossTurns is the core US2 test: a workflow
// that waits for user input must end the FIRST turn with stopReason=end_turn (so the editor
// re-enables its input field) while the run goroutine stays alive, parked. The NEXT prompt is
// routed to the parked reader via Respond, the workflow then completes, and its output is
// streamed back on that turn.
func TestACPSessionService_Prompt_ParksAndResumesAcrossTurns(t *testing.T) {
	exec := workflow.NewExecutionContext("workflow-1", "Test Workflow")
	exec.SetStepState("recall", workflow.StepState{Output: "FORTY-TWO\n"})

	reader := newParkingResponder()
	runner := &parkingRunner{reader: reader, turns: 1, execCtx: exec}
	streamed := &atomic.Bool{}
	emitter := &fakeEmitter{}

	svc := &ACPSessionService{logger: ports.NopLogger{}, emitter: emitter}
	svc.SetRunnerFactory(func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		return runner, reader, streamed, func() {}, nil
	})
	session := &ACPSession{ID: "sess-park"}
	svc.sessions.Store("sess-park", session)

	// --- Turn 1: dispatch the workflow; it parks waiting for input. ---
	turn1 := json.RawMessage(`{"sessionId":"sess-park","prompt":[{"type":"text","text":"/workflow-1"}]}`)
	type outcome struct {
		result any
		err    *ACPHandlerError
	}
	done := make(chan outcome, 1)
	go func() {
		r, e := svc.HandleSessionPrompt(context.Background(), turn1)
		done <- outcome{r, e}
	}()

	select {
	case got := <-done:
		require.Nil(t, got.err)
		assert.Equal(t, "end_turn", stopReasonOf(t, got.result),
			"a parked workflow must end the turn with end_turn so the editor re-enables input")
	case <-time.After(2 * time.Second):
		t.Fatal("turn 1 did not end while the workflow was parked (synchronous run blocks the turn)")
	}

	require.Eventually(t, func() bool { return session.ParkedTurnCount.Load() > 0 }, time.Second, time.Millisecond,
		"the workflow goroutine must be parked after turn 1 ends")
	require.False(t, session.InFlight.Load(), "InFlight must be released so the next prompt is admitted")

	// --- Turn 2: the user's reply is routed to the parked reader; the workflow completes. ---
	turn2 := json.RawMessage(`{"sessionId":"sess-park","prompt":[{"type":"text","text":"the answer"}]}`)
	r2, e2 := svc.HandleSessionPrompt(context.Background(), turn2)
	require.Nil(t, e2)
	assert.Equal(t, "end_turn", stopReasonOf(t, r2))
	assert.Equal(t, []string{"the answer"}, reader.responses,
		"the continuation prompt text must be routed to the parked reader via Respond")
	assert.Contains(t, emitter.agentText(), "FORTY-TWO",
		"the completed workflow's output must be streamed back on the resuming turn")
}

// promptTurn runs one session/prompt turn with a timeout guard so a wiring bug surfaces as a
// failure rather than a hung test. It asserts the turn returns without a JSON-RPC error.
func promptTurn(t *testing.T, svc *ACPSessionService, sessionID, text string) any {
	t.Helper()
	params, err := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	})
	require.NoError(t, err)
	type outcome struct {
		result any
		err    *ACPHandlerError
	}
	ch := make(chan outcome, 1)
	go func() {
		r, e := svc.HandleSessionPrompt(context.Background(), params)
		ch <- outcome{r, e}
	}()
	select {
	case got := <-ch:
		require.Nil(t, got.err)
		return got.result
	case <-time.After(2 * time.Second):
		t.Fatalf("session/prompt %q did not resolve (turn blocked)", text)
		return nil
	}
}

// TestACPSessionService_Prompt_RunCtxSurvivesTurn1RequestCancellation is a regression test for
// the "Invalid prompt: must begin with a /<workflow> slash command" bug. The SDK cancels the
// per-request context when the Prompt handler returns end_turn (via defer cancel in connection.go).
// Before the fix, runCtx was a child of that per-request ctx, so the parked ReadInput goroutine
// was killed by the cancellation and ParkedTurnCount dropped to 0 before turn2 arrived, causing
// the second prompt to be misrouted as a new slash command instead of a continuation.
//
// After the fix, runCtx derives from session.sessionCtx (session-lifetime, independent of any
// single request context), so cancelling turn1's request context must NOT kill the parked run.
func TestACPSessionService_Prompt_RunCtxSurvivesTurn1RequestCancellation(t *testing.T) {
	exec := workflow.NewExecutionContext("wf-survive", "Survive Test")
	exec.SetStepState("out", workflow.StepState{Output: "survived\n"})

	reader := newParkingResponder()
	runner := &parkingRunner{reader: reader, turns: 1, execCtx: exec}
	streamed := &atomic.Bool{}
	emitter := &fakeEmitter{}

	svc := &ACPSessionService{logger: ports.NopLogger{}, emitter: emitter}
	svc.SetRunnerFactory(func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		return runner, reader, streamed, func() {}, nil
	})
	session := &ACPSession{ID: "sess-survive"}
	svc.sessions.Store("sess-survive", session)

	// Turn 1: dispatch with a cancellable request context (models the SDK per-request ctx).
	turn1Ctx, turn1Cancel := context.WithCancel(context.Background())
	turn1 := json.RawMessage(`{"sessionId":"sess-survive","prompt":[{"type":"text","text":"/wf-survive"}]}`)
	done := make(chan struct{}, 1)
	go func() {
		defer close(done)
		svc.HandleSessionPrompt(turn1Ctx, turn1) //nolint:errcheck // result checked indirectly
	}()

	// Wait for the workflow to park (ParkedTurnCount > 0) so we know ReadInput is blocking.
	require.Eventually(t, func() bool { return session.ParkedTurnCount.Load() > 0 },
		time.Second, time.Millisecond, "workflow must park after turn 1 dispatches")

	// Simulate what the SDK does when the Prompt handler returns end_turn: cancel the
	// per-request context. This must NOT kill the parked run goroutine.
	turn1Cancel()
	<-done // turn 1 handler has returned

	// Give the race detector a moment to surface any use-after-cancel on runCtx.
	time.Sleep(10 * time.Millisecond)

	// The run goroutine must still be parked — ParkedTurnCount must be positive.
	require.Greater(t, session.ParkedTurnCount.Load(), int32(0),
		"parked workflow must survive cancellation of turn 1's request context; "+
			"if this fails, runCtx was derived from the per-request ctx (regression)")

	// Turn 2: the user's reply must resume the parked workflow normally.
	turn2 := json.RawMessage(`{"sessionId":"sess-survive","prompt":[{"type":"text","text":"continue"}]}`)
	r2, e2 := svc.HandleSessionPrompt(context.Background(), turn2)
	require.Nil(t, e2)
	assert.Equal(t, "end_turn", stopReasonOf(t, r2),
		"turn 2 must complete via the parked reader, not fail with 'Invalid prompt'")
	assert.Contains(t, emitter.agentText(), "survived")
}

// TestACPSessionService_Prompt_MultiTurnParkingResumesEachTurn verifies a workflow that parks
// more than once: each user reply resumes the SAME run, the workflow re-parks (ending the turn
// with end_turn), and the run completes only after the final reply — with replies routed to the
// reader in order. This exercises the parkedCh handshake being reused across successive turns.
func TestACPSessionService_Prompt_MultiTurnParkingResumesEachTurn(t *testing.T) {
	exec := workflow.NewExecutionContext("workflow-1", "Test Workflow")
	exec.SetStepState("done", workflow.StepState{Output: "RESULT-7\n"})

	reader := newParkingResponder()
	runner := &parkingRunner{reader: reader, turns: 2, execCtx: exec}
	streamed := &atomic.Bool{}
	emitter := &fakeEmitter{}

	svc := &ACPSessionService{logger: ports.NopLogger{}, emitter: emitter}
	svc.SetRunnerFactory(func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		return runner, reader, streamed, func() {}, nil
	})
	session := &ACPSession{ID: "sess-multi"}
	svc.sessions.Store("sess-multi", session)

	parked := func() bool { return session.ParkedTurnCount.Load() > 0 }

	// Turn 1: dispatch → first park.
	assert.Equal(t, "end_turn", stopReasonOf(t, promptTurn(t, svc, "sess-multi", "/workflow-1")))
	require.Eventually(t, parked, time.Second, time.Millisecond, "workflow must park after turn 1")

	// Turn 2: first reply → workflow consumes it and re-parks.
	assert.Equal(t, "end_turn", stopReasonOf(t, promptTurn(t, svc, "sess-multi", "first reply")))
	require.Eventually(t, parked, time.Second, time.Millisecond, "workflow must re-park after turn 2")

	// Turn 3: second reply → workflow completes and streams its output.
	assert.Equal(t, "end_turn", stopReasonOf(t, promptTurn(t, svc, "sess-multi", "second reply")))

	assert.Equal(t, []string{"first reply", "second reply"}, reader.responses,
		"each continuation turn must route its text to the parked reader, in order")
	assert.Contains(t, emitter.agentText(), "RESULT-7",
		"the workflow output must be streamed once the run completes")
}
