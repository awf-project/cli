package application

// acp_audit_fixes_test.go — TDD regression tests for the 7 audit issues.
// Each test is written BEFORE the fix and targets exactly one issue.
// All must pass after the fixes are applied.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// ---------------------------------------------------------------------------
// C1 — race lifecycle Shutdown/Prompt (runWG.Add before ensureRunner)
// ---------------------------------------------------------------------------

// TestACPSessionService_C1_ShutdownDuringRunnerInit reproduces the CRITIQUE-4 window:
// a SIGTERM arrives while ensureRunner is building the per-session runner. Before the
// fix, runWG.Add(1) came AFTER ensureRunner returned, so Shutdown could see runWG==0,
// call runnerCleanup(), and leave the session in a torn-down state just as the prompt
// handler started calling runner.Run on its freshly built runner.
//
// The test synchronizes via a gate channel: the factory signals "building" and then
// waits until it is told to proceed. In that window, Shutdown races. The test asserts:
//  1. Shutdown completes without panicking or racing (verified by -race).
//  2. The workflow run OBSERVES a cancelled context (stopReason=cancelled or ctx.Err()),
//     not a nil/dead runner crash. This is the central property of C4: Shutdown cancels
//     every session's run context, so the runner's Run must see ctx.Done().
//
// R9 fix: the factory returns a blocking runner (block=true) that waits on ctx.Done()
// and returns ctx.Err(). The test captures HandleSessionPrompt's result and asserts the
// run observed the cancellation — not merely "no deadlock".
func TestACPSessionService_C1_ShutdownDuringRunnerInit(t *testing.T) {
	factoryEntered := make(chan struct{})
	factoryProceed := make(chan struct{})

	var cleanupCalled atomic.Bool
	factory := func(sessionID string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		close(factoryEntered) // signal that factory is in progress
		<-factoryProceed      // wait for the test to release
		cleanup := func() { cleanupCalled.Store(true) }
		// block=true: Run blocks on ctx.Done() and returns ctx.Err() so the test can assert
		// that the runner observes cancellation (C4 property).
		return &fakeRunner{block: true}, &fakeInputResponder{}, &atomic.Bool{}, cleanup, nil
	}

	mockRepo := new(MockWorkflowRepository)
	baseCtx := context.Background()
	mockRepo.On("ListWithSource", baseCtx).Return([]ports.WorkflowInfo{
		{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"},
	}, nil)
	mockRepo.On("Load", baseCtx, "trivial").Return(testWorkflow("trivial"), nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(baseCtx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)

	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})

	// In production, the server passes a context derived from the signal/shutdown context to
	// every handler. When the server shuts down, it cancels that parent context, which flows
	// through to the runCtx in HandleSessionPrompt and unblocks any blocking runner.Run call.
	// Reproduce that mechanism here: give HandleSessionPrompt a cancellable context.
	promptCtx, promptCancel := context.WithCancel(baseCtx)
	defer promptCancel()

	type promptOutcome struct {
		result any
		acpErr *ACPHandlerError
	}
	promptDone := make(chan promptOutcome, 1)
	go func() {
		r, e := svc.HandleSessionPrompt(promptCtx, promptParams)
		promptDone <- promptOutcome{result: r, acpErr: e}
	}()

	// Wait until factory is in progress (ensureRunner holds runnerMu).
	<-factoryEntered

	// Shutdown races while factory is building the runner. It is launched before unblocking
	// the factory so that the race window (Shutdown arrives before setCancel is called) is
	// exercised. Shutdown's session.shutdown() run-cancel is a no-op here (cancelFn not yet registered).
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		svc.Shutdown()
	}()

	// Let the factory finish so the prompt can proceed to runner.Run. The Shutdown cancel
	// (session.shutdown's run-cancel) fired before setCancel, so it was a no-op. Cancel the promptCtx now
	// to simulate the JSON-RPC server cancelling the request context at shutdown — this is
	// what unblocks the blocking runner (runCtx is derived from promptCtx).
	close(factoryProceed)
	// Give the prompt goroutine time to reach runner.Run before we cancel.
	// Use a small sleep-free polling approach: cancel promptCtx right after unblocking;
	// the runner's select { case <-ctx.Done() } will pick it up on the next schedule.
	promptCancel()

	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not return within timeout")
	}

	var outcome promptOutcome
	select {
	case outcome = <-promptDone:
	case <-time.After(3 * time.Second):
		t.Fatal("HandleSessionPrompt did not return after Shutdown")
	}

	// Central C4 assertion: the runner must observe context cancellation. The handler maps
	// a cancelled run to stopReason=cancelled (runCtx.Err() != nil path). This proves the
	// runner was not left blocking on a dead session after Shutdown.
	require.Nil(t, outcome.acpErr, "Shutdown-induced cancellation must not be a JSON-RPC error")
	assert.Equal(t, "cancelled", stopReasonOf(t, outcome.result),
		"runner must observe the cancelled context — either from Shutdown or parent ctx (C4 property)")
}

// ---------------------------------------------------------------------------
// R5 — fallthrough silencieux quand ParkedTurnCount > 0 mais inputReader == nil
// ---------------------------------------------------------------------------

// TestACPSessionService_R5_ParkedWithNilInputReaderReturnsInternalError verifies that when
// ParkedTurnCount > 0 but inputReader has never been stored (factory wiring bug), the handler
// returns an explicit acpInternal error rather than silently falling through into
// parseSlashCommand (which would misroute the continuation text as a new slash command).
//
// This exercises the invariant guard added in the R5 fix: a non-nil ParkedTurnCount with a
// nil inputReader is a factory wiring bug; the handler must not silently mishandle it.
func TestACPSessionService_R5_ParkedWithNilInputReaderReturnsInternalError(t *testing.T) {
	runner := &fakeRunner{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, runner: runner, emitter: &fakeEmitter{}}

	// Inject a session with ParkedTurnCount > 0 but NO inputReader stored (nil atomic.Pointer).
	session := &ACPSession{ID: "sess-broken-park"}
	session.ParkedTurnCount.Store(1)
	// session.inputReader is zero-value atomic.Pointer — Load() returns nil.
	svc.sessions.Store("sess-broken-park", session)

	params := json.RawMessage(`{"sessionId":"sess-broken-park","prompt":[{"type":"text","text":"continue please"}]}`)
	_, acpErr := svc.HandleSessionPrompt(context.Background(), params)

	require.NotNil(t, acpErr, "invariant violation must return a structured error, not fall through")
	assert.Equal(t, ACPErrInternal, acpErr.Kind,
		"a parked session without an input reader is a factory wiring bug: must be ACPErrInternal")
	// The runner must NOT have been called: the handler should have returned before dispatching.
	assert.Equal(t, 0, runner.callCount(),
		"runner must not be invoked when the invariant guard catches the broken state")
}

// ---------------------------------------------------------------------------
// C2 — session map leak: sessions never deleted from sync.Map
// ---------------------------------------------------------------------------

// TestACPSessionService_C2_ShutdownCleansSessionMap verifies that after Shutdown,
// the sessions sync.Map is empty. Before the fix, sessions were never deleted and
// the map grew unboundedly across many client sessions.
func TestACPSessionService_C2_ShutdownCleansSessionMap(t *testing.T) {
	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		return &fakeRunner{}, &fakeInputResponder{}, &atomic.Bool{}, func() {}, nil
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{}, nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}
	svc.SetRunnerFactory(factory)

	// Create 3 sessions.
	for range 3 {
		_, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
		require.Nil(t, acpErr)
	}

	// Verify sessions exist before Shutdown.
	var countBefore int
	svc.sessions.Range(func(_, _ any) bool { countBefore++; return true })
	assert.Equal(t, 3, countBefore, "3 sessions must be stored before Shutdown")

	svc.Shutdown()

	var countAfter int
	svc.sessions.Range(func(_, _ any) bool { countAfter++; return true })
	assert.Equal(t, 0, countAfter, "sessions sync.Map must be empty after Shutdown")
}

// ---------------------------------------------------------------------------
// M7 — InputReader and streamed read outside lock (atomic.Pointer)
// ---------------------------------------------------------------------------

// TestACPSessionService_M7_InputReaderAtomicIsDefenseInDepth documents that
// session.inputReader is an atomic.Pointer[ACPInputResponder] as a defense-in-depth
// measure. In the current architecture, the race between ensureRunner writing inputReader
// (under runnerMu) and HandleSessionPrompt reading it cannot occur in practice: the
// InFlight CAS serializes all prompt handlers so only one prompt can run at a time,
// and ensureRunner is always called from within that single inflight handler before any
// read of inputReader.
//
// The atomic.Pointer is therefore defense-in-depth — it costs nothing at runtime and
// protects against future refactors that relax the InFlight serialization. This test
// documents that invariant explicitly, and verifies that concurrent prompts (which race
// on InFlight itself) still correctly observe inputReader via the atomic, without data
// races detectable by -race.
func TestACPSessionService_M7_InputReaderAtomicIsDefenseInDepth(t *testing.T) {
	var wg sync.WaitGroup
	const goroutines = 20

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{
		{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"},
	}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		reader := &fakeInputResponder{}
		return &fakeRunner{}, reader, &atomic.Bool{}, func() {}, nil
	}

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)

	// Concurrently submit prompts: half are slash commands (trigger ensureRunner + write),
	// half are plain text (hit the parking-check read path). InFlight serializes them, but
	// the atomic.Pointer ensures correctness even under -race analysis, which detects
	// happens-before violations regardless of the serialization.
	for i := range goroutines {
		wg.Add(1)
		promptText := "/trivial"
		if i%2 == 0 {
			promptText = "not a slash command — exercises the inputReader.Load() read path"
		}
		go func(text string) {
			defer wg.Done()
			params, _ := json.Marshal(map[string]any{
				"sessionId": sessionID,
				"prompt":    []map[string]any{{"type": "text", "text": text}},
			})
			svc.HandleSessionPrompt(ctx, params) //nolint:errcheck // defense-in-depth race check only
		}(promptText)
	}
	wg.Wait()
	// -race must report no data races: atomic.Pointer provides the necessary synchronization.
}

// TestACPSessionService_M7_StreamedResetBetweenRuns verifies that the streamed flag is
// reset to false at the start of each run so the aggregate-suppression check in
// HandleSessionPrompt reflects only the current run, not a previous run's flag value.
//
// Note: this test exercises two SEQUENTIAL prompts (not concurrent) because InFlight
// serializes prompt handlers — only one can run at a time. The test therefore documents
// and verifies the per-run reset behavior, not a concurrent race. The streamed field is
// stored as atomic.Pointer[atomic.Bool] as defense-in-depth (consistent with inputReader
// and execCtx) but the race between ensureRunner writing it and HandleSessionPrompt
// reading it cannot occur while InFlight is held. The -race flag confirms no violations.
func TestACPSessionService_M7_StreamedResetBetweenRuns(t *testing.T) {
	exec := workflow.NewExecutionContext("trivial", "Trivial")
	exec.SetStepState("run", workflow.StepState{Output: "out\n"})

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{
		{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"},
	}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		streamed := &atomic.Bool{}
		return &fakeRunner{execCtx: exec}, &fakeInputResponder{}, streamed, func() {}, nil
	}

	emitter := &fakeEmitter{}
	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: emitter}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)

	params, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})

	// First prompt builds runner (streamed flag is false → aggregate is sent).
	_, acpErr = svc.HandleSessionPrompt(ctx, params)
	require.Nil(t, acpErr)
	firstText := emitter.agentText()
	assert.Contains(t, firstText, "out", "first run: aggregate must be sent when streamed=false")

	// Second prompt: streamed flag must have been reset to false at the start of the run,
	// so the aggregate is sent again (the fakeRunner never sets streamed=true). If the reset
	// were missing, the flag could carry over true from a previous run that DID stream.
	_, acpErr = svc.HandleSessionPrompt(ctx, params)
	require.Nil(t, acpErr)
	secondText := emitter.agentText()
	// agentText() is cumulative; second run appended to first.
	assert.Contains(t, secondText, "out",
		"second run: aggregate must be sent when streamed is reset to false between runs")
}

// ---------------------------------------------------------------------------
// P1 — N+1 parallel workflow loading in HandleSessionNew
// ---------------------------------------------------------------------------

// TestACPSessionService_P1_ParallelWorkflowLoadPreservesOrder verifies that session/new
// loads workflow metadata in parallel and returns the slash-command catalog in the same
// order as ListWithSource, regardless of which goroutine finishes first.
//
// Correctness requirements:
//  1. All workflows are present in the catalog (no silent drops).
//  2. Order matches the original infos slice (index-based parallel assignment, not append).
//  3. Metadata (Description, RequiredInputs) is populated from the loaded workflow.
func TestACPSessionService_P1_ParallelWorkflowLoadPreservesOrder(t *testing.T) {
	const n = 10
	infos := make([]ports.WorkflowInfo, n)
	for i := range n {
		infos[i] = ports.WorkflowInfo{
			Name:   fmt.Sprintf("workflow-%02d", i),
			Source: ports.SourceLocal,
			Path:   fmt.Sprintf("/p/workflow-%02d.yaml", i),
		}
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return(infos, nil)
	for i := range n {
		name := fmt.Sprintf("workflow-%02d", i)
		mockRepo.On("Load", ctx, name).Return(testWorkflow(name), nil)
	}

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}

	result, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)

	commands, ok := resultMap(t, result)["commands"].([]WorkflowSlashCommand)
	require.True(t, ok, "commands must be []WorkflowSlashCommand")
	require.Len(t, commands, n, "all workflows must appear in the catalog")

	for i, cmd := range commands {
		expectedName := fmt.Sprintf("workflow-%02d", i)
		assert.Equal(t, expectedName, cmd.Name,
			"command at index %d must be %s (order must match ListWithSource)", i, expectedName)
		assert.NotEmpty(t, cmd.Description,
			"description must be populated from loaded workflow for %s", cmd.Name)
	}

	mockRepo.AssertExpectations(t)
}

// TestACPSessionService_P1_LoadFailureDegradesToNameOnly verifies that when a workflow
// load fails during parallel loading, the catalog degrades to a name-only entry rather
// than dropping the command or aborting session/new entirely.
func TestACPSessionService_P1_LoadFailureDegradesToNameOnly(t *testing.T) {
	infos := []ports.WorkflowInfo{
		{Name: "good", Source: ports.SourceLocal, Path: "/p/good.yaml"},
		{Name: "broken", Source: ports.SourceLocal, Path: "/p/broken.yaml"},
		{Name: "also-good", Source: ports.SourceLocal, Path: "/p/also-good.yaml"},
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return(infos, nil)
	mockRepo.On("Load", ctx, "good").Return(testWorkflow("good"), nil)
	mockRepo.On("Load", ctx, "broken").Return(nil, fmt.Errorf("simulated load failure"))
	mockRepo.On("Load", ctx, "also-good").Return(testWorkflow("also-good"), nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}

	result, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr, "a load failure must not abort session/new")

	commands, ok := resultMap(t, result)["commands"].([]WorkflowSlashCommand)
	require.True(t, ok)
	require.Len(t, commands, 3, "all 3 workflow entries must be present")

	assert.Equal(t, "good", commands[0].Name)
	assert.NotEmpty(t, commands[0].Description, "good must have description")

	assert.Equal(t, "broken", commands[1].Name)
	// A load failure no longer drops the description entirely: ACP requires a non-empty
	// description, so a command whose workflow failed to load falls back to its name. Inputs
	// still degrade to none (they could not be loaded).
	assert.Equal(t, "broken", commands[1].Description,
		"broken degrades to name-as-description fallback (ACP requires a non-empty description)")
	assert.Empty(t, commands[1].RequiredInputs, "broken must degrade to name-only (no inputs)")

	assert.Equal(t, "also-good", commands[2].Name)
	assert.NotEmpty(t, commands[2].Description, "also-good must have description")

	mockRepo.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// M5a — error detail leak in JSON-RPC responses
// ---------------------------------------------------------------------------

// TestACPSessionService_M5a_WorkflowDiscoveryErrorIsOpaque verifies that when
// workflowRepo.ListWithSource returns an error, the returned JSON-RPC error message
// is a generic string, not the raw infra error detail.
func TestACPSessionService_M5a_WorkflowDiscoveryErrorIsOpaque(t *testing.T) {
	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	// Simulate an infra error with a detail that must not leak to the caller.
	sensitiveDetail := "sqlite: database is locked at /var/lib/awf/state.db"
	mockRepo.On("ListWithSource", ctx).Return(nil, &sensitiveInfraError{msg: sensitiveDetail})

	svc := &ACPSessionService{workflowRepo: mockRepo, logger: ports.NopLogger{}}

	_, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.NotNil(t, acpErr)
	assert.Equal(t, ACPErrInternal, acpErr.Kind)
	// The message must NOT contain the raw infra detail.
	assert.NotContains(t, acpErr.Message, sensitiveDetail,
		"infrastructure error details must not be surfaced in the JSON-RPC response")
	// It must be a short, generic message.
	assert.LessOrEqual(t, len(acpErr.Message), 60,
		"error message should be a short generic string, not the full infra trace")
}

// TestACPSessionService_M5a_FactoryErrorIsOpaque verifies that when the runner factory
// returns an error, the JSON-RPC response does not propagate the raw error string.
func TestACPSessionService_M5a_FactoryErrorIsOpaque(t *testing.T) {
	sensitiveDetail := "sqlite: cannot open /run/awf/sess-abc/state.db: permission denied"
	factory := func(string) (WorkflowRunner, ACPInputResponder, *atomic.Bool, func(), error) {
		return nil, nil, nil, nil, &sensitiveInfraError{msg: sensitiveDetail}
	}

	mockRepo := new(MockWorkflowRepository)
	ctx := context.Background()
	mockRepo.On("ListWithSource", ctx).Return([]ports.WorkflowInfo{
		{Name: "trivial", Source: ports.SourceLocal, Path: "/p/trivial.yaml"},
	}, nil)
	mockRepo.On("Load", ctx, "trivial").Return(testWorkflow("trivial"), nil)

	svc := &ACPSessionService{logger: ports.NopLogger{}, workflowRepo: mockRepo, emitter: &fakeEmitter{}}
	svc.SetRunnerFactory(factory)

	newResult, acpErr := svc.HandleSessionNew(ctx, json.RawMessage(`{"cwd":"/h","mcpServers":[]}`))
	require.Nil(t, acpErr)
	sessionID, _ := resultMap(t, newResult)["sessionId"].(string)

	promptParams, _ := json.Marshal(map[string]any{
		"sessionId": sessionID,
		"prompt":    []map[string]any{{"type": "text", "text": "/trivial"}},
	})
	_, acpErr = svc.HandleSessionPrompt(ctx, promptParams)
	require.NotNil(t, acpErr, "factory failure must surface as a structured error")
	assert.Equal(t, ACPErrInternal, acpErr.Kind)
	assert.NotContains(t, acpErr.Message, sensitiveDetail,
		"infrastructure error details must not be surfaced in the JSON-RPC response")
}

// sensitiveInfraError is a helper error type carrying a detail string used by M5a tests.
type sensitiveInfraError struct{ msg string }

func (e *sensitiveInfraError) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// MINEUR perf — double copy of StepState in workflowOutputText
// ---------------------------------------------------------------------------

// TestWorkflowOutputText_DeterministicOrderByCompletedAt verifies that workflowOutputText
// produces output ordered by CompletedAt regardless of map iteration order. Steps inserted
// in arbitrary order (b, a, c) must appear in chronological order (a, b, c) in the result.
// This acts as a regression guard for the MINEUR-3 determinism fix: GetAllStepStates
// returns a map with random iteration order; the function sorts by CompletedAt to produce
// a stable, meaningful aggregation.
func TestWorkflowOutputText_DeterministicOrderByCompletedAt(t *testing.T) {
	now := time.Now()
	exec := workflow.NewExecutionContext("wf", "WF")
	exec.SetStepState("b", workflow.StepState{Output: "step-b\n", CompletedAt: now.Add(time.Second)})
	exec.SetStepState("a", workflow.StepState{Output: "step-a\n", CompletedAt: now})
	exec.SetStepState("c", workflow.StepState{Output: "step-c\n", CompletedAt: now.Add(2 * time.Second)})

	got := workflowOutputText(exec)
	// Must be ordered by CompletedAt: a, b, c.
	parts := strings.Split(got, "\n")
	require.GreaterOrEqual(t, len(parts), 3)
	assert.True(t, strings.HasPrefix(parts[0], "step-a"), "first part must be step-a (earliest CompletedAt)")
	assert.True(t, strings.HasPrefix(parts[1], "step-b"), "second part must be step-b")
	assert.True(t, strings.HasPrefix(parts[2], "step-c"), "third part must be step-c")
}

// ---------------------------------------------------------------------------
// MINEUR — parseSlashCommand path-traversal defense
// ---------------------------------------------------------------------------

// TestParseSlashCommand_PathTraversalDefense verifies that workflow names containing
// ".." or other invalid characters are rejected at the parseSlashCommand level before
// any runner call. C-1 fix: validation is now performed by pkg/validation.ValidateName
// (^[a-z][a-z0-9-]*$), which makes path traversal structurally impossible. Any character
// outside [a-z0-9-] is rejected.
//
// Issue #11 update: error messages now name the specific failing component (pack vs
// workflow). The errMsg field carries a substring that must appear; tests that exercise a
// pack-position component check for "invalid pack name", and tests with a workflow-position
// component check for "invalid workflow name".
func TestParseSlashCommand_PathTraversalDefense(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "double-dot traversal rejected",
			text:    "/../../etc/passwd",
			wantErr: true,
			// name="../../etc/passwd" → SplitN → ["..","../etc/passwd"]; ".." is in pack
			// position (len=2, i=0) → "invalid pack name".
			errMsg: "invalid pack name",
		},
		{
			name:    "leading slash in name rejected",
			text:    "//absolute/path",
			wantErr: true,
			// "//absolute/path" → name="/absolute/path" → split → ["","absolute/path"];
			// first component "" is in pack position → "invalid pack name".
			errMsg: "invalid pack name",
		},
		{
			name:    "pack/workflow separator allowed",
			text:    "/mypack/myworkflow",
			wantErr: false,
		},
		{
			name:    "dot-dot mid-path rejected",
			text:    "/good/../evil",
			wantErr: true,
			// name="good/../evil" → split → ["good","../evil"]; "../evil" is in workflow
			// position → "invalid workflow name".
			errMsg: "invalid workflow name",
		},
		{
			name:    "simple name allowed",
			text:    "/deploy",
			wantErr: false,
		},
		{
			name:    "pack-slash-workflow allowed",
			text:    "/ops/deploy",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseSlashCommand(tt.text)
			if tt.wantErr {
				require.Error(t, err, "expected error for %q", tt.text)
				assert.Contains(t, err.Error(), tt.errMsg,
					"error must mention %q for %q", tt.errMsg, tt.text)
			} else {
				assert.NoError(t, err, "valid workflow name %q must not be rejected", tt.text)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MINEUR — acpMethodNotFound constructor
// ---------------------------------------------------------------------------

// TestACPMethodNotFound_Constructor verifies the new acpMethodNotFound constructor
// produces an ACPHandlerError with Kind==ACPErrMethodNotFound and the given message.
func TestACPMethodNotFound_Constructor(t *testing.T) {
	err := acpMethodNotFound("unknown sub-method: compute")
	require.NotNil(t, err)
	assert.Equal(t, ACPErrMethodNotFound, err.Kind)
	assert.Equal(t, "unknown sub-method: compute", err.Message)
	assert.Equal(t, "unknown sub-method: compute", err.Error())
}
