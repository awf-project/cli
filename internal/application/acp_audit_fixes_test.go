package application

// acp_audit_fixes_test.go — TDD regression tests for the 7 audit issues.
// Each test is written BEFORE the fix and targets exactly one issue.
// All must pass after the fixes are applied.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

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
	svc := &ACPSessionService{logger: ports.NopLogger{}, emitter: &fakeEmitter{}}

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
	// The handler must have returned at the invariant guard, before any facade dispatch.
}

// ---------------------------------------------------------------------------
// C2 — session map leak: sessions never deleted from sync.Map
// ---------------------------------------------------------------------------

// TestACPSessionService_C2_ShutdownCleansSessionMap verifies that after Shutdown,
// the sessions sync.Map is empty. Before the fix, sessions were never deleted and
// the map grew unboundedly across many client sessions.
func TestACPSessionService_C2_ShutdownCleansSessionMap(t *testing.T) {
	factory := func(string) (ACPInputResponder, *atomic.Bool, func(), ports.WorkflowFacade, error) {
		return &fakeInputResponder{}, &atomic.Bool{}, func() {}, nil, nil
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
	factory := func(string) (ACPInputResponder, *atomic.Bool, func(), ports.WorkflowFacade, error) {
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
