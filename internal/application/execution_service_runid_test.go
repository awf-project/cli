package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
)

// newRunIDTestService builds a minimal ExecutionService capable of running
// prepareExecution (logger + resolver + hookExecutor wired).
func newRunIDTestService() *ExecutionService {
	logger := &mockLogger{}
	resolver := newMockResolver()
	return &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		logger:        logger,
		resolver:      resolver,
		hookExecutor:  NewHookExecutor(newMockExecutor(), logger, resolver),
	}
}

func terminalOnlyWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "rid-workflow",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
}

// TestPrepareExecution_UsesProvidedRunID verifies the explicit run identity threads
// into execCtx.WorkflowID (so the transcript filename and event run_id match) and that
// the parent run id is propagated for sub-workflow linkage.
func TestPrepareExecution_UsesProvidedRunID(t *testing.T) {
	svc := newRunIDTestService()
	wf := terminalOnlyWorkflow()

	_, span, execCtx, err := svc.prepareExecution(context.Background(), wf, nil, nil, "fixed-run-id", "parent-run-id")
	require.NoError(t, err)
	if span != nil {
		defer span.End()
	}

	assert.Equal(t, "fixed-run-id", execCtx.WorkflowID, "execCtx.WorkflowID must equal the provided run id")
	assert.Equal(t, "parent-run-id", execCtx.ParentRunID, "parent run id must propagate to the child context")
}

// TestPrepareExecution_GeneratesRunIDWhenEmpty verifies that an empty run id falls back
// to a generated UUID (preserving existing top-level run behavior).
func TestPrepareExecution_GeneratesRunIDWhenEmpty(t *testing.T) {
	svc := newRunIDTestService()
	wf := terminalOnlyWorkflow()

	_, span, execCtx, err := svc.prepareExecution(context.Background(), wf, nil, nil, "", "")
	require.NoError(t, err)
	if span != nil {
		defer span.End()
	}

	assert.NotEmpty(t, execCtx.WorkflowID, "an empty run id must fall back to a generated identifier")
	assert.Empty(t, execCtx.ParentRunID, "no parent run id when none provided")
}

// TestRunWithWorkflowAndRunID_StampsTranscriptRunID verifies the public entry point
// threads the CLI-provided run id all the way into emitted transcript events, so the
// <run-id>.jsonl filename and the events' run_id are the same identifier.
func TestRunWithWorkflowAndRunID_StampsTranscriptRunID(t *testing.T) {
	svc := newRunIDTestService()
	rec := &fakeRecorder{}
	svc.SetRecorder(rec)
	svc.store = testmocks.NewMockStateStore()

	wf := &workflow.Workflow{
		Name:    "rid-workflow",
		Initial: "s1",
		Steps: map[string]*workflow.Step{
			"s1":   {Name: "s1", Type: workflow.StepTypeCommand, OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}
	svc.executor = newMockExecutor()

	execCtx, err := svc.RunWithWorkflowAndRunID(context.Background(), wf, nil, "cli-run-id")
	require.NoError(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, "cli-run-id", execCtx.WorkflowID)

	require.NotEmpty(t, rec.events, "expected transcript events to be emitted")
	for _, ev := range rec.events {
		assert.Equal(t, "cli-run-id", ev.RunID, "every transcript event must carry the provided run id")
	}
}

// TestRun_ChildRecorderViaContext_IsolatesEvents verifies F106 US5: a sub-run's events
// are routed to the context-scoped child Recorder (its own file) rather than the parent's,
// the child carries the provided ParentRunID, and the parent recorder is untouched.
func TestRun_ChildRecorderViaContext_IsolatesEvents(t *testing.T) {
	svc := newRunIDTestService()
	parentRec := &fakeRecorder{}
	childRec := &fakeRecorder{}
	svc.SetRecorder(parentRec)
	svc.store = testmocks.NewMockStateStore()
	svc.executor = newMockExecutor()

	wf := &workflow.Workflow{
		Name:    "child-workflow",
		Initial: "s1",
		Steps: map[string]*workflow.Step{
			"s1":   {Name: "s1", Type: workflow.StepTypeCommand, OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	ctx := withRecorder(context.Background(), childRec)
	execCtx, err := svc.runWithCallStackAndWorkflow(ctx, "", wf, nil, nil, "child-run", "parent-run")
	require.NoError(t, err)
	require.NotNil(t, execCtx)

	assert.Equal(t, "child-run", execCtx.WorkflowID)
	assert.Equal(t, "parent-run", execCtx.ParentRunID)

	assert.Empty(t, parentRec.events, "parent recorder must not receive the child run's events")
	require.NotEmpty(t, childRec.events, "child recorder must receive the child run's events")
	for _, ev := range childRec.events {
		assert.Equal(t, "child-run", ev.RunID, "child events must carry the child run id")
	}
}
