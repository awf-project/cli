package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

func TestExecuteSingleStep_Success(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo hello"] = &ports.CommandResult{Stdout: "hello\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	result, err := execSvc.ExecuteSingleStep(context.Background(), "test", "start", nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "start", result.StepName)
	assert.Equal(t, "hello\n", result.Output)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, workflow.StatusCompleted, result.Status)
}

func TestExecuteSingleStep_StepNotFound(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["test"] = &workflow.Workflow{
		Name:    "test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.ExecuteSingleStep(context.Background(), "test", "nonexistent", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecuteSingleStep_WorkflowNotFound(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.ExecuteSingleStep(context.Background(), "nonexistent", "step", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecuteSingleStep_WithInputs(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["input-test"] = &workflow.Workflow{
		Name:    "input-test",
		Initial: "process",
		Inputs:  []workflow.Input{{Name: "data", Type: "string"}},
		Steps: map[string]*workflow.Step{
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.data}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo test-value"] = &ports.CommandResult{Stdout: "test-value\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	// Use a resolver that actually interpolates
	resolver := &interpolatingMockResolver{
		mapping: map[string]string{
			"echo {{inputs.data}}": "echo test-value",
		},
	}
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, resolver)

	inputs := map[string]any{"data": "test-value"}
	result, err := execSvc.ExecuteSingleStep(context.Background(), "input-test", "process", inputs, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-value\n", result.Output)
}

func TestExecuteSingleStep_WithMocks(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["mock-test"] = &workflow.Workflow{
		Name:    "mock-test",
		Initial: "fetch",
		Steps: map[string]*workflow.Step{
			"fetch": {
				Name:      "fetch",
				Type:      workflow.StepTypeCommand,
				Command:   "curl http://api",
				OnSuccess: "process",
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.fetch.output}}",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["echo mocked-data"] = &ports.CommandResult{Stdout: "mocked-data\n", ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	// Use a resolver that interpolates the mocked state
	resolver := &interpolatingMockResolver{
		mapping: map[string]string{
			"echo {{states.fetch.output}}": "echo mocked-data",
		},
	}
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, resolver)

	// Mock the output of the "fetch" step
	mocks := map[string]string{
		"states.fetch.output": "mocked-data",
	}

	result, err := execSvc.ExecuteSingleStep(context.Background(), "mock-test", "process", nil, mocks)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "mocked-data\n", result.Output)
}

func TestExecuteSingleStep_ExecutesHooks(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["hook-test"] = &workflow.Workflow{
		Name:    "hook-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo main",
				OnSuccess: "done",
				Hooks: workflow.StepHooks{
					Pre:  workflow.Hook{{Command: "echo pre-hook"}},
					Post: workflow.Hook{{Command: "echo post-hook"}},
				},
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := &hookTrackingExecutor{
		executedCommands: []string{},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.ExecuteSingleStep(context.Background(), "hook-test", "start", nil, nil)

	require.NoError(t, err)
	// Verify hooks were executed
	assert.Contains(t, executor.executedCommands, "echo pre-hook")
	assert.Contains(t, executor.executedCommands, "echo post-hook")
}

func TestExecuteSingleStep_TerminalStepError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["terminal-test"] = &workflow.Workflow{
		Name:    "terminal-test",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.ExecuteSingleStep(context.Background(), "terminal-test", "done", nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

func TestExecuteSingleStep_CommandFails(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["fail-test"] = &workflow.Workflow{
		Name:    "fail-test",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	result, err := execSvc.ExecuteSingleStep(context.Background(), "fail-test", "fail", nil, nil)

	// Single step execution returns result even on failure (no state machine transition)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.ExitCode)
	assert.Equal(t, workflow.StatusFailed, result.Status)
	assert.Equal(t, "command failed", result.Stderr)
}

// hookTrackingExecutor records all executed commands
type hookTrackingExecutor struct {
	executedCommands []string
}

func (h *hookTrackingExecutor) Execute(ctx context.Context, cmd ports.Command) (*ports.CommandResult, error) {
	h.executedCommands = append(h.executedCommands, cmd.Program)
	return &ports.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
}

// interpolatingMockResolver resolves templates based on a predefined mapping
type interpolatingMockResolver struct {
	mapping map[string]string
}

func (r *interpolatingMockResolver) Resolve(template string, ctx *interpolation.Context) (string, error) {
	if resolved, ok := r.mapping[template]; ok {
		return resolved, nil
	}
	return template, nil
}
