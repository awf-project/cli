package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/infrastructure/expression"
	"github.com/awf-project/awf/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildInteractiveExecutorWithExecutor wires up an InteractiveExecutor with a specific
// command executor and a workflow registered in the mock repository.
func buildInteractiveExecutorWithExecutor(
	t *testing.T,
	wf *workflow.Workflow,
	executor *mockExecutor,
	prompt *mockInteractivePrompt,
) *application.InteractiveExecutor {
	t.Helper()
	repo := newMockRepository()
	repo.workflows[wf.Name] = wf
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	resolver := interpolation.NewTemplateResolver()
	evaluator := expression.NewExprEvaluator()
	return application.NewInteractiveExecutor(
		wfSvc, executor, newMockParallelExecutor(),
		newMockStateStore(), &mockLogger{}, resolver, evaluator, prompt,
	)
}

// newFailingExecutor creates a mock executor that returns exit code 1 for any command.
func newFailingExecutor(command string) *mockExecutor {
	executor := newMockExecutor()
	executor.results[command] = &ports.CommandResult{ExitCode: 1, Stderr: "simulated failure"}
	return executor
}

// TestInteractiveExecutor_Terminal_FailureWithMessage verifies that a TerminalFailure
// step returns an error containing the step message and sets StatusFailed.
func TestInteractiveExecutor_Terminal_FailureWithMessage(t *testing.T) {
	const workCommand = "echo work"

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   workCommand,
				OnFailure: "error_terminal",
				OnSuccess: "done",
			},
			"error_terminal": {
				Name:    "error_terminal",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Build failed: deployment aborted",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	prompt := newMockPrompt(workflow.ActionContinue)
	exec := buildInteractiveExecutorWithExecutor(t, wf, newFailingExecutor(workCommand), prompt)

	execCtx, err := exec.Run(context.Background(), "test", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Build failed: deployment aborted",
		"error must include terminal step message, got: %s", err.Error())
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
}

// TestInteractiveExecutor_Terminal_SuccessNoError verifies that a TerminalSuccess
// step returns nil error and sets StatusCompleted — no regression.
func TestInteractiveExecutor_Terminal_SuccessNoError(t *testing.T) {
	const workCommand = "echo work"

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   workCommand,
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	prompt := newMockPrompt(workflow.ActionContinue)
	// Default executor returns exit code 0 for unregistered commands
	exec := buildInteractiveExecutorWithExecutor(t, wf, newMockExecutor(), prompt)

	execCtx, err := exec.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.True(t, prompt.completeCalled, "ShowCompleted must be called on success terminal")
}

// TestInteractiveExecutor_Terminal_FailureWithTemplateMessage verifies that the
// terminal step message is interpolated before being returned as an error.
func TestInteractiveExecutor_Terminal_FailureWithTemplateMessage(t *testing.T) {
	const deployCommand = "make deploy"

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "deploy",
		Steps: map[string]*workflow.Step{
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   deployCommand,
				OnFailure: "error_terminal",
				OnSuccess: "done",
			},
			"error_terminal": {
				Name:    "error_terminal",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "Deploy failed for env {{inputs.environment}}",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	prompt := newMockPrompt(workflow.ActionContinue)
	exec := buildInteractiveExecutorWithExecutor(t, wf, newFailingExecutor(deployCommand), prompt)

	inputs := map[string]any{"environment": "production"}
	execCtx, err := exec.Run(context.Background(), "test", inputs)

	require.Error(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.True(t, strings.Contains(err.Error(), "production"),
		"interpolated input must appear in error message, got: %s", err.Error())
	assert.False(t, strings.Contains(err.Error(), "{{inputs.environment}}"),
		"raw template must not appear after interpolation, got: %s", err.Error())
}

// TestInteractiveExecutor_Terminal_FailureEmptyMessageGenericFallback verifies that
// a TerminalFailure with no message produces a generic error referencing the step name.
func TestInteractiveExecutor_Terminal_FailureEmptyMessageGenericFallback(t *testing.T) {
	const workCommand = "echo work"

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   workCommand,
				OnFailure: "error_terminal",
				OnSuccess: "done",
			},
			"error_terminal": {
				Name:    "error_terminal",
				Type:    workflow.StepTypeTerminal,
				Status:  workflow.TerminalFailure,
				Message: "", // empty — falls back to generic
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	prompt := newMockPrompt(workflow.ActionContinue)
	exec := buildInteractiveExecutorWithExecutor(t, wf, newFailingExecutor(workCommand), prompt)

	execCtx, err := exec.Run(context.Background(), "test", nil)

	require.Error(t, err)
	require.NotNil(t, execCtx)
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "error_terminal",
		"generic error must reference the terminal step name, got: %s", err.Error())
}
