package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestExecutionService_Run_SingleStepWorkflow(t *testing.T) {
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

	ctx, err := execSvc.Run(context.Background(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// verify step state captured
	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Equal(t, "hello\n", state.Output)
	assert.Equal(t, 0, state.ExitCode)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

func TestExecutionService_Run_MultiStepWorkflow(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["multi"] = &workflow.Workflow{
		Name:    "multi",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor() // default returns ExitCode: 0

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "multi", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// all 3 steps should have been executed
	_, ok := ctx.GetStepState("step1")
	assert.True(t, ok, "step1 should be executed")
	_, ok = ctx.GetStepState("step2")
	assert.True(t, ok, "step2 should be executed")
	_, ok = ctx.GetStepState("step3")
	assert.True(t, ok, "step3 should be executed")
}

func TestExecutionService_Run_FailureTransition(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["fail-test"] = &workflow.Workflow{
		Name:    "fail-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "success",
				OnFailure: "error",
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"error":   {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "failed"}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "fail-test", nil)

	require.NoError(t, err) // workflow completes, just via error path
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep) // ended on error terminal

	state, ok := ctx.GetStepState("start")
	require.True(t, ok)
	assert.Equal(t, 1, state.ExitCode)
	assert.Equal(t, workflow.StatusFailed, state.Status)
}

func TestExecutionService_Run_FailureNoTransition(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["fail-no-transition"] = &workflow.Workflow{
		Name:    "fail-no-transition",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
				// no OnFailure
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "fail-no-transition", nil)

	require.Error(t, err) // workflow fails
	assert.Equal(t, workflow.StatusFailed, ctx.Status)
}

func TestExecutionService_Run_StepTimeout(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["timeout-test"] = &workflow.Workflow{
		Name:    "timeout-test",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:      "slow",
				Type:      workflow.StepTypeCommand,
				Command:   "sleep 10",
				Timeout:   1, // 1 second
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	// use mock that returns timeout error
	executor := &timeoutMockExecutor{
		timeout: 500 * time.Millisecond,
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "timeout-test", nil)

	require.NoError(t, err) // workflow completes via error path
	assert.Equal(t, "error", ctx.CurrentStep)
}

func TestExecutionService_Run_WorkflowNotFound(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.Run(context.Background(), "nonexistent", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecutionService_Run_WithInputs(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["input-test"] = &workflow.Workflow{
		Name:    "input-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo test", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	inputs := map[string]any{"name": "test", "count": 42}
	ctx, err := execSvc.Run(context.Background(), "input-test", inputs)

	require.NoError(t, err)

	val, ok := ctx.GetInput("name")
	assert.True(t, ok)
	assert.Equal(t, "test", val)

	val, ok = ctx.GetInput("count")
	assert.True(t, ok)
	assert.Equal(t, 42, val)
}

func TestExecutionService_Run_StepNotFound(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["bad-ref"] = &workflow.Workflow{
		Name:    "bad-ref",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "nonexistent", // bad reference
			},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.Run(context.Background(), "bad-ref", nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExecutionService_Run_ImmediateTerminal(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["immediate"] = &workflow.Workflow{
		Name:    "immediate",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "immediate", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestNewExecutionService(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	assert.NotNil(t, execSvc)
}

// timeoutMockExecutor simulates timeout behavior
type timeoutMockExecutor struct {
	timeout time.Duration
}

func (m *timeoutMockExecutor) Execute(ctx context.Context, cmd ports.Command) (*ports.CommandResult, error) {
	// simulate slow execution that gets cancelled
	select {
	case <-time.After(m.timeout):
		return &ports.CommandResult{ExitCode: -1}, context.DeadlineExceeded
	case <-ctx.Done():
		return &ports.CommandResult{ExitCode: -1}, ctx.Err()
	}
}

// errorMockExecutor always returns an error
type errorMockExecutor struct {
	err error
}

func (m *errorMockExecutor) Execute(ctx context.Context, cmd ports.Command) (*ports.CommandResult, error) {
	return &ports.CommandResult{ExitCode: -1}, m.err
}

func TestExecutionService_Run_ExecutorError(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["exec-error"] = &workflow.Workflow{
		Name:    "exec-error",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "something",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := &errorMockExecutor{err: errors.New("command not found")}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "exec-error", nil)

	require.NoError(t, err) // workflow should complete via error path
	assert.Equal(t, "error", ctx.CurrentStep)
}

func TestExecutionService_Run_WithDir(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["dir-test"] = &workflow.Workflow{
		Name:    "dir-test",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				Dir:       "/tmp/project",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newCapturingMockExecutor()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.Run(context.Background(), "dir-test", nil)

	require.NoError(t, err)
	require.NotNil(t, executor.lastCmd, "executor should have received a command")
	assert.Equal(t, "/tmp/project", executor.lastCmd.Dir, "Dir should be passed to executor")
}

func TestExecutionService_Run_WithDirEmpty(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["no-dir-test"] = &workflow.Workflow{
		Name:    "no-dir-test",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newCapturingMockExecutor()

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	_, err := execSvc.Run(context.Background(), "no-dir-test", nil)

	require.NoError(t, err)
	require.NotNil(t, executor.lastCmd, "executor should have received a command")
	assert.Equal(t, "", executor.lastCmd.Dir, "Dir should be empty when not specified")
}

func TestExecutionService_Run_SavesCheckpoints(t *testing.T) {
	repo := newMockRepository()
	repo.workflows["checkpoint-test"] = &workflow.Workflow{
		Name:    "checkpoint-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	store := newMockStateStore()
	executor := newMockExecutor()

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, store, &mockLogger{}, newMockResolver())

	execCtx, err := execSvc.Run(context.Background(), "checkpoint-test", nil)
	require.NoError(t, err)

	// State should have been saved (checkpointed)
	saved, err := store.Load(context.Background(), execCtx.WorkflowID)
	require.NoError(t, err)
	require.NotNil(t, saved, "state should be checkpointed after execution")
	assert.Equal(t, workflow.StatusCompleted, saved.Status)
	assert.Equal(t, "done", saved.CurrentStep)
}
