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

// =============================================================================
// ContinueOnError Tests (F009)
// =============================================================================

func TestExecutionService_Run_ContinueOnErrorFollowsOnSuccess(t *testing.T) {
	// When continue_on_error is true, even if the step fails (non-zero exit),
	// it should follow on_success instead of on_failure
	repo := newMockRepository()
	repo.workflows["continue-on-error"] = &workflow.Workflow{
		Name:    "continue-on-error",
		Initial: "flaky",
		Steps: map[string]*workflow.Step{
			"flaky": {
				Name:            "flaky",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1",
				ContinueOnError: true, // this is key
				OnSuccess:       "success",
				OnFailure:       "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "continue-on-error", nil)

	require.NoError(t, err, "workflow should complete without error")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "success", ctx.CurrentStep, "should follow on_success despite exit code 1")

	// The step state should still reflect the failure
	state, ok := ctx.GetStepState("flaky")
	require.True(t, ok)
	assert.Equal(t, 1, state.ExitCode, "step exit code should be 1")
}

func TestExecutionService_Run_ContinueOnErrorWithExecutorError(t *testing.T) {
	// When continue_on_error is true and the executor returns an error,
	// it should still follow on_success
	repo := newMockRepository()
	repo.workflows["continue-exec-error"] = &workflow.Workflow{
		Name:    "continue-exec-error",
		Initial: "failing",
		Steps: map[string]*workflow.Step{
			"failing": {
				Name:            "failing",
				Type:            workflow.StepTypeCommand,
				Command:         "nonexistent_command",
				ContinueOnError: true,
				OnSuccess:       "success",
				OnFailure:       "failure",
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	}

	executor := &errorMockExecutor{err: errors.New("command not found")}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "continue-exec-error", nil)

	require.NoError(t, err, "workflow should complete without error")
	assert.Equal(t, "success", ctx.CurrentStep, "should follow on_success despite executor error")
}

func TestExecutionService_Run_ContinueOnErrorFalseFollowsOnFailure(t *testing.T) {
	// When continue_on_error is false (default), failure should follow on_failure
	repo := newMockRepository()
	repo.workflows["normal-failure"] = &workflow.Workflow{
		Name:    "normal-failure",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:            "step",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1",
				ContinueOnError: false, // default behavior
				OnSuccess:       "success",
				OnFailure:       "failure",
			},
			"success": {Name: "success", Type: workflow.StepTypeTerminal},
			"failure": {Name: "failure", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "normal-failure", nil)

	require.NoError(t, err) // workflow completes via failure path
	assert.Equal(t, "failure", ctx.CurrentStep, "should follow on_failure when continue_on_error is false")
}

func TestExecutionService_Run_ContinueOnErrorMultipleSteps(t *testing.T) {
	// Test that continue_on_error works correctly across multiple steps
	repo := newMockRepository()
	repo.workflows["multi-continue"] = &workflow.Workflow{
		Name:    "multi-continue",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:            "step1",
				Type:            workflow.StepTypeCommand,
				Command:         "fail1",
				ContinueOnError: true,
				OnSuccess:       "step2",
				OnFailure:       "error",
			},
			"step2": {
				Name:            "step2",
				Type:            workflow.StepTypeCommand,
				Command:         "fail2",
				ContinueOnError: true,
				OnSuccess:       "step3",
				OnFailure:       "error",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "success",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["fail1"] = &ports.CommandResult{ExitCode: 1}
	executor.results["fail2"] = &ports.CommandResult{ExitCode: 2}
	executor.results["success"] = &ports.CommandResult{ExitCode: 0}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "multi-continue", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep, "should reach done despite step1 and step2 failures")

	// Verify all steps were executed
	_, ok := ctx.GetStepState("step1")
	assert.True(t, ok, "step1 should have been executed")
	_, ok = ctx.GetStepState("step2")
	assert.True(t, ok, "step2 should have been executed")
	_, ok = ctx.GetStepState("step3")
	assert.True(t, ok, "step3 should have been executed")
}

func TestExecutionService_Run_ContinueOnErrorNoOnFailure(t *testing.T) {
	// When continue_on_error is true and there's no on_failure defined,
	// it should still follow on_success on failure
	repo := newMockRepository()
	repo.workflows["continue-no-onfailure"] = &workflow.Workflow{
		Name:    "continue-no-onfailure",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:            "step",
				Type:            workflow.StepTypeCommand,
				Command:         "exit 1",
				ContinueOnError: true,
				OnSuccess:       "done",
				// no OnFailure defined
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "continue-no-onfailure", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep, "should follow on_success when continue_on_error is true")
}
