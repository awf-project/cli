package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/expression"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// InteractiveExecutor Tests (F020)
// =============================================================================

// mockInteractivePrompt simulates user interactions for testing.
type mockInteractivePrompt struct {
	actions        []workflow.InteractiveAction
	actionIndex    int
	editValues     map[string]any
	headerCalled   bool
	lastStepInfo   *workflow.InteractiveStepInfo
	executingCalls []string
	resultShown    bool
	contextShown   bool
	abortCalled    bool
	skipCalls      []string
	completeCalled bool
	errorCalled    bool
}

func newMockPrompt(actions ...workflow.InteractiveAction) *mockInteractivePrompt {
	return &mockInteractivePrompt{
		actions:    actions,
		editValues: make(map[string]any),
	}
}

func (m *mockInteractivePrompt) ShowHeader(workflowName string) {
	m.headerCalled = true
}

func (m *mockInteractivePrompt) ShowStepDetails(info *workflow.InteractiveStepInfo) {
	m.lastStepInfo = info
}

func (m *mockInteractivePrompt) PromptAction(hasRetry bool) (workflow.InteractiveAction, error) {
	if m.actionIndex >= len(m.actions) {
		return workflow.ActionAbort, nil
	}
	action := m.actions[m.actionIndex]
	m.actionIndex++
	return action, nil
}

func (m *mockInteractivePrompt) ShowExecuting(stepName string) {
	m.executingCalls = append(m.executingCalls, stepName)
}

func (m *mockInteractivePrompt) ShowStepResult(state *workflow.StepState, nextStep string) {
	m.resultShown = true
}

func (m *mockInteractivePrompt) ShowContext(ctx *workflow.RuntimeContext) {
	m.contextShown = true
}

func (m *mockInteractivePrompt) EditInput(name string, current any) (any, error) {
	if val, ok := m.editValues[name]; ok {
		return val, nil
	}
	return current, nil
}

func (m *mockInteractivePrompt) ShowAborted() {
	m.abortCalled = true
}

func (m *mockInteractivePrompt) ShowSkipped(stepName string, nextStep string) {
	m.skipCalls = append(m.skipCalls, stepName)
}

func (m *mockInteractivePrompt) ShowCompleted(status workflow.ExecutionStatus) {
	m.completeCalled = true
}

func (m *mockInteractivePrompt) ShowError(err error) {
	m.errorCalled = true
}

func TestInteractiveExecutor_Run_ContinueThroughAllSteps(t *testing.T) {
	// Setup mock workflow
	repo := newMockRepository()
	repo.workflows["linear"] = &workflow.Workflow{
		Name:    "linear",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start":   {Name: "start", Type: workflow.StepTypeCommand, Command: "echo start", OnSuccess: "process"},
			"process": {Name: "process", Type: workflow.StepTypeCommand, Command: "echo process", OnSuccess: "done"},
			"done":    {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	prompt := newMockPrompt(workflow.ActionContinue, workflow.ActionContinue, workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, err := exec.Run(context.Background(), "linear", nil)

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.True(t, prompt.headerCalled, "should show header")
	assert.True(t, prompt.completeCalled, "should show completion")
	assert.Len(t, prompt.executingCalls, 2, "should execute 2 command steps")
}

func TestInteractiveExecutor_Run_AbortStopsExecution(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["linear"] = &workflow.Workflow{
		Name:    "linear",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start":   {Name: "start", Type: workflow.StepTypeCommand, Command: "echo start", OnSuccess: "process"},
			"process": {Name: "process", Type: workflow.StepTypeCommand, Command: "echo process", OnSuccess: "done"},
			"done":    {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	prompt := newMockPrompt(workflow.ActionContinue, workflow.ActionAbort)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, err := exec.Run(context.Background(), "linear", nil)

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.True(t, prompt.abortCalled, "should show abort message")
	assert.Len(t, prompt.executingCalls, 1, "should only execute first step")
}

func TestInteractiveExecutor_Run_SkipJumpsToOnSuccess(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["linear"] = &workflow.Workflow{
		Name:    "linear",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start":   {Name: "start", Type: workflow.StepTypeCommand, Command: "echo start", OnSuccess: "process"},
			"process": {Name: "process", Type: workflow.StepTypeCommand, Command: "echo process", OnSuccess: "done"},
			"done":    {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	// Skip first step, continue second
	prompt := newMockPrompt(workflow.ActionSkip, workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, err := exec.Run(context.Background(), "linear", nil)

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.Contains(t, prompt.skipCalls, "start", "should record skip of start")
	assert.Len(t, prompt.executingCalls, 1, "should only execute process step")
}

func TestInteractiveExecutor_Run_InspectShowsContext(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["simple"] = &workflow.Workflow{
		Name:    "simple",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo hi", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	// Inspect, then continue
	prompt := newMockPrompt(workflow.ActionInspect, workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	_, err := exec.Run(context.Background(), "simple", nil)

	require.NoError(t, err)
	assert.True(t, prompt.contextShown, "should show context on inspect")
}

func TestInteractiveExecutor_Run_EditModifiesInput(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["with-input"] = &workflow.Workflow{
		Name:    "with-input",
		Initial: "start",
		Inputs:  []workflow.Input{{Name: "file", Type: "string", Required: true}},
		Steps: map[string]*workflow.Step{
			// Use simple command without template variables for testing
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo test", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	prompt := newMockPrompt(workflow.ActionEdit, workflow.ActionContinue)
	prompt.editValues["file"] = "new-file.txt"

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, err := exec.Run(context.Background(), "with-input", map[string]any{"file": "old-file.txt"})

	require.NoError(t, err)
	require.NotNil(t, ctx)
	// After edit, the input should be updated
	assert.Equal(t, "new-file.txt", ctx.Inputs["file"])
}

func TestInteractiveExecutor_Run_RetryReExecutesStep(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["linear"] = &workflow.Workflow{
		Name:    "linear",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo step1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo step2", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	// Continue step1 → execute → at step2, retry → go back to step1 → continue → execute → at step2 → continue → execute → done
	prompt := newMockPrompt(workflow.ActionContinue, workflow.ActionRetry, workflow.ActionContinue, workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, err := exec.Run(context.Background(), "linear", nil)

	require.NoError(t, err)
	require.NotNil(t, ctx)
	// Should have executed step1 twice (once, then retry from step2)
	step1Count := 0
	for _, name := range prompt.executingCalls {
		if name == "step1" {
			step1Count++
		}
	}
	assert.Equal(t, 2, step1Count, "should execute step1 twice with retry")
}

func TestInteractiveExecutor_SetBreakpoints_PausesOnlyAtSpecified(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["multi"] = &workflow.Workflow{
		Name:    "multi",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	// Only one continue needed since we only breakpoint at step2
	prompt := newMockPrompt(workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)
	exec.SetBreakpoints([]string{"step2"})

	ctx, err := exec.Run(context.Background(), "multi", nil)

	require.NoError(t, err)
	require.NotNil(t, ctx)
	// Should only pause and prompt at step2
	require.NotNil(t, prompt.lastStepInfo)
	assert.Equal(t, "step2", prompt.lastStepInfo.Name, "should only prompt at breakpoint step")
}

func TestInteractiveExecutor_Run_WorkflowNotFound(t *testing.T) {
	repo := newMockRepository()
	// No workflows added

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	prompt := newMockPrompt()

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	_, err := exec.Run(context.Background(), "nonexistent", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInteractiveExecutor_Run_ContextCancelled(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["simple"] = &workflow.Workflow{
		Name:    "simple",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo hi", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	prompt := newMockPrompt(workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := exec.Run(ctx, "simple", nil)

	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestInteractiveExecutor_Run_ShowsStepDetails(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["detailed"] = &workflow.Workflow{
		Name:    "detailed",
		Initial: "validate",
		Steps: map[string]*workflow.Step{
			"validate": {
				Name:      "validate",
				Type:      workflow.StepTypeCommand,
				Command:   "echo validate",
				Timeout:   10,
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	prompt := newMockPrompt(workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	_, err := exec.Run(context.Background(), "detailed", nil)

	require.NoError(t, err)
	require.NotNil(t, prompt.lastStepInfo)
	assert.Equal(t, "validate", prompt.lastStepInfo.Name)
	assert.Equal(t, 1, prompt.lastStepInfo.Index)
	assert.Equal(t, "echo validate", prompt.lastStepInfo.Command, "command should be shown")
}

func TestInteractiveExecutor_Run_ParallelStep(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["parallel"] = &workflow.Workflow{
		Name:    "parallel",
		Initial: "multi",
		Steps: map[string]*workflow.Step{
			"multi": {
				Name:      "multi",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"a", "b"},
				Strategy:  "all_succeed",
				OnSuccess: "done",
			},
			"a":    {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a", OnSuccess: "done"},
			"b":    {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b", OnSuccess: "done"},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	// Continue at parallel step (runs all branches without individual prompts)
	prompt := newMockPrompt(workflow.ActionContinue)

	exec := application.NewInteractiveExecutor(
		wfSvc, newMockExecutor(), newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)

	ctx, err := exec.Run(context.Background(), "parallel", nil)

	require.NoError(t, err)
	require.NotNil(t, ctx)
	assert.True(t, prompt.completeCalled)
}
