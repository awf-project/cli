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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "timeout-test", nil)

	require.NoError(t, err) // workflow completes via error path
	assert.Equal(t, "error", ctx.CurrentStep)
}

func TestExecutionService_Run_WorkflowNotFound(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "immediate", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestNewExecutionService(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "continue-no-onfailure", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep, "should follow on_success when continue_on_error is true")
}

// =============================================================================
// Retry Tests (F011)
// =============================================================================

// retryCountingExecutor tracks execution count per command
type retryCountingExecutor struct {
	calls       map[string]int
	results     map[string][]*ports.CommandResult // multiple results for successive calls
	defaultErr  error
	callHistory []string
}

func newRetryCountingExecutor() *retryCountingExecutor {
	return &retryCountingExecutor{
		calls:       make(map[string]int),
		results:     make(map[string][]*ports.CommandResult),
		callHistory: make([]string, 0),
	}
}

func (m *retryCountingExecutor) Execute(ctx context.Context, cmd ports.Command) (*ports.CommandResult, error) {
	m.calls[cmd.Program]++
	m.callHistory = append(m.callHistory, cmd.Program)

	if results, ok := m.results[cmd.Program]; ok {
		idx := m.calls[cmd.Program] - 1
		if idx < len(results) {
			return results[idx], nil
		}
		// Return the last result for additional calls
		return results[len(results)-1], nil
	}

	if m.defaultErr != nil {
		return &ports.CommandResult{ExitCode: -1}, m.defaultErr
	}
	return &ports.CommandResult{ExitCode: 0, Stdout: "ok"}, nil
}

func TestExecutionService_Run_WithRetry_SucceedsOnRetry(t *testing.T) {
	// Step fails on first attempt, succeeds on second
	repo := newMockRepository()
	repo.workflows["retry-success"] = &workflow.Workflow{
		Name:    "retry-success",
		Initial: "flaky",
		Steps: map[string]*workflow.Step{
			"flaky": {
				Name:    "flaky",
				Type:    workflow.StepTypeCommand,
				Command: "flaky_command",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 10, // 10ms for fast tests
					MaxDelayMs:     100,
					Backoff:        "constant",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	// First call fails, second succeeds
	executor.results["flaky_command"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "temporary failure"},
		{ExitCode: 0, Stdout: "success on retry"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "retry-success", nil)

	require.NoError(t, err, "workflow should complete successfully")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep, "should reach done after retry")

	// Verify the command was called twice
	assert.Equal(t, 2, executor.calls["flaky_command"], "should have retried once")

	// Verify step state reflects success
	state, ok := ctx.GetStepState("flaky")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, 0, state.ExitCode)
}

func TestExecutionService_Run_WithRetry_ExhaustsAttempts(t *testing.T) {
	// Step fails all attempts
	repo := newMockRepository()
	repo.workflows["retry-exhausted"] = &workflow.Workflow{
		Name:    "retry-exhausted",
		Initial: "failing",
		Steps: map[string]*workflow.Step{
			"failing": {
				Name:    "failing",
				Type:    workflow.StepTypeCommand,
				Command: "always_fail",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 10,
					MaxDelayMs:     100,
					Backoff:        "constant",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	// All calls fail
	executor.results["always_fail"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "fail 1"},
		{ExitCode: 1, Stderr: "fail 2"},
		{ExitCode: 1, Stderr: "fail 3"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "retry-exhausted", nil)

	// Should complete via error path (has on_failure)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error", ctx.CurrentStep, "should follow on_failure after exhausting retries")

	// Verify all attempts were made
	assert.Equal(t, 3, executor.calls["always_fail"], "should have made all 3 attempts")

	// Verify step state reflects failure
	state, ok := ctx.GetStepState("failing")
	require.True(t, ok)
	assert.Equal(t, workflow.StatusFailed, state.Status)
	assert.Equal(t, 1, state.ExitCode)
}

func TestExecutionService_Run_WithRetry_ContextCancelled(t *testing.T) {
	// Retry should stop when context is cancelled
	repo := newMockRepository()
	repo.workflows["retry-cancel"] = &workflow.Workflow{
		Name:    "retry-cancel",
		Initial: "slow",
		Steps: map[string]*workflow.Step{
			"slow": {
				Name:    "slow",
				Type:    workflow.StepTypeCommand,
				Command: "slow_fail",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    10,
					InitialDelayMs: 500, // 500ms delay
					MaxDelayMs:     1000,
					Backoff:        "constant",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	// Always fail
	executor.results["slow_fail"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "fail"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// Cancel after 100ms (before many retries can happen)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "retry-cancel", nil)

	// Should have been cancelled
	require.Error(t, err)
	assert.Equal(t, workflow.StatusCancelled, execCtx.Status)

	// Should have made very few attempts due to cancellation
	assert.LessOrEqual(t, executor.calls["slow_fail"], 3, "should have been cancelled before many retries")
}

func TestExecutionService_Run_NoRetryConfig(t *testing.T) {
	// Without retry config, step should behave normally (fail immediately)
	repo := newMockRepository()
	repo.workflows["no-retry"] = &workflow.Workflow{
		Name:    "no-retry",
		Initial: "failing",
		Steps: map[string]*workflow.Step{
			"failing": {
				Name:      "failing",
				Type:      workflow.StepTypeCommand,
				Command:   "fail_once",
				OnSuccess: "done",
				OnFailure: "error",
				// No Retry config
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	executor.results["fail_once"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "failed"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "no-retry", nil)

	require.NoError(t, err) // completes via error path
	assert.Equal(t, "error", ctx.CurrentStep)

	// Should have been called exactly once (no retry)
	assert.Equal(t, 1, executor.calls["fail_once"], "should not retry without retry config")
}

func TestExecutionService_Run_WithRetry_OnlySpecificExitCodes(t *testing.T) {
	// Retry only on specific exit codes
	repo := newMockRepository()
	repo.workflows["retry-codes"] = &workflow.Workflow{
		Name:    "retry-codes",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "exit_code_test",
				Retry: &workflow.RetryConfig{
					MaxAttempts:        3,
					InitialDelayMs:     10,
					MaxDelayMs:         100,
					Backoff:            "constant",
					RetryableExitCodes: []int{1, 2}, // only retry on 1 or 2
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	// First call returns exit code 3 (not retryable)
	executor.results["exit_code_test"] = []*ports.CommandResult{
		{ExitCode: 3, Stderr: "not retryable"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "retry-codes", nil)

	require.NoError(t, err)
	assert.Equal(t, "error", ctx.CurrentStep)

	// Should NOT have retried because exit code 3 is not in retryable list
	assert.Equal(t, 1, executor.calls["exit_code_test"], "should not retry non-retryable exit code")
}

func TestExecutionService_Run_WithRetry_RetryableExitCodeSucceeds(t *testing.T) {
	// Retry on specific exit code, eventually succeeds
	repo := newMockRepository()
	repo.workflows["retry-specific-code"] = &workflow.Workflow{
		Name:    "retry-specific-code",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "code_check",
				Retry: &workflow.RetryConfig{
					MaxAttempts:        3,
					InitialDelayMs:     10,
					MaxDelayMs:         100,
					Backoff:            "constant",
					RetryableExitCodes: []int{1, 2, 130},
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	// First two calls return retryable exit code, third succeeds
	executor.results["code_check"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "retry me"},
		{ExitCode: 2, Stderr: "retry me again"},
		{ExitCode: 0, Stdout: "finally worked"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "retry-specific-code", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep)
	assert.Equal(t, 3, executor.calls["code_check"], "should have made 3 attempts")
}

func TestExecutionService_Run_WithRetry_MaxAttemptsOne(t *testing.T) {
	// MaxAttempts=1 means no retry (only initial attempt)
	repo := newMockRepository()
	repo.workflows["no-retry-one"] = &workflow.Workflow{
		Name:    "no-retry-one",
		Initial: "once",
		Steps: map[string]*workflow.Step{
			"once": {
				Name:    "once",
				Type:    workflow.StepTypeCommand,
				Command: "single_try",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    1, // only one attempt, no retries
					InitialDelayMs: 10,
					MaxDelayMs:     100,
					Backoff:        "constant",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	executor.results["single_try"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "failed"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "no-retry-one", nil)

	require.NoError(t, err)
	assert.Equal(t, "error", ctx.CurrentStep)
	assert.Equal(t, 1, executor.calls["single_try"], "MaxAttempts=1 means no retries")
}

func TestExecutionService_Run_WithRetry_ExponentialBackoff(t *testing.T) {
	// Verify exponential backoff is used (indirectly through timing)
	repo := newMockRepository()
	repo.workflows["retry-exponential"] = &workflow.Workflow{
		Name:    "retry-exponential",
		Initial: "exp",
		Steps: map[string]*workflow.Step{
			"exp": {
				Name:    "exp",
				Type:    workflow.StepTypeCommand,
				Command: "exp_fail",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 50,
					MaxDelayMs:     1000,
					Backoff:        "exponential",
					Multiplier:     2.0,
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	executor.results["exp_fail"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "fail 1"},
		{ExitCode: 0, Stdout: "success"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	start := time.Now()
	ctx, err := execSvc.Run(context.Background(), "retry-exponential", nil)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep)

	// With exponential backoff: first retry after ~50ms (actually ~100ms with multiplier)
	// This is a rough check - at least some delay occurred
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "should have waited for backoff delay")
}

func TestExecutionService_Run_WithRetry_MultipleStepsWithRetry(t *testing.T) {
	// Multiple steps each with their own retry config
	repo := newMockRepository()
	repo.workflows["multi-retry"] = &workflow.Workflow{
		Name:    "multi-retry",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:    "step1",
				Type:    workflow.StepTypeCommand,
				Command: "cmd1",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    2,
					InitialDelayMs: 10,
					MaxDelayMs:     100,
					Backoff:        "constant",
				},
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:    "step2",
				Type:    workflow.StepTypeCommand,
				Command: "cmd2",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 10,
					MaxDelayMs:     100,
					Backoff:        "constant",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newRetryCountingExecutor()
	// step1: succeeds on 2nd try
	// step2: succeeds on 3rd try
	executor.results["cmd1"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "fail"},
		{ExitCode: 0, Stdout: "ok"},
	}
	executor.results["cmd2"] = []*ports.CommandResult{
		{ExitCode: 1, Stderr: "fail"},
		{ExitCode: 1, Stderr: "fail"},
		{ExitCode: 0, Stdout: "ok"},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	ctx, err := execSvc.Run(context.Background(), "multi-retry", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep)
	assert.Equal(t, 2, executor.calls["cmd1"], "step1 should have retried once")
	assert.Equal(t, 3, executor.calls["cmd2"], "step2 should have retried twice")
}

// =============================================================================
// Input Validation Tests (F012)
// =============================================================================

func TestExecutionService_Run_InputValidation_ValidInputs(t *testing.T) {
	// Workflow with input validation - all inputs valid
	min1, max100 := 1, 100
	repo := newMockRepository()
	repo.workflows["input-validation"] = &workflow.Workflow{
		Name:    "input-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "email",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	inputs := map[string]any{
		"email": "test@example.com",
		"count": 50,
	}

	ctx, err := execSvc.Run(context.Background(), "input-validation", inputs)

	require.NoError(t, err, "workflow should complete with valid inputs")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestExecutionService_Run_InputValidation_InvalidEmail(t *testing.T) {
	// Workflow with input validation - invalid email pattern
	repo := newMockRepository()
	repo.workflows["invalid-email"] = &workflow.Workflow{
		Name:    "invalid-email",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "email",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	inputs := map[string]any{
		"email": "not-an-email",
	}

	_, err := execSvc.Run(context.Background(), "invalid-email", inputs)

	require.Error(t, err, "should fail with invalid email")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "email")
}

func TestExecutionService_Run_InputValidation_RequiredMissing(t *testing.T) {
	// Required input not provided
	repo := newMockRepository()
	repo.workflows["required-missing"] = &workflow.Workflow{
		Name:    "required-missing",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "required_field",
				Type:     "string",
				Required: true,
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// Empty inputs - required field missing
	_, err := execSvc.Run(context.Background(), "required-missing", map[string]any{})

	require.Error(t, err, "should fail with missing required input")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "required_field")
}

func TestExecutionService_Run_InputValidation_IntegerOutOfRange(t *testing.T) {
	// Integer input outside min/max range
	min1, max100 := 1, 100
	repo := newMockRepository()
	repo.workflows["integer-range"] = &workflow.Workflow{
		Name:    "integer-range",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// count=150 exceeds max=100
	inputs := map[string]any{
		"count": 150,
	}

	_, err := execSvc.Run(context.Background(), "integer-range", inputs)

	require.Error(t, err, "should fail with integer out of range")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "count")
}

func TestExecutionService_Run_InputValidation_EnumInvalid(t *testing.T) {
	// Enum input with invalid value
	repo := newMockRepository()
	repo.workflows["enum-validation"] = &workflow.Workflow{
		Name:    "enum-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "env",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Enum: []string{"dev", "staging", "prod"},
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// "local" not in enum list
	inputs := map[string]any{
		"env": "local",
	}

	_, err := execSvc.Run(context.Background(), "enum-validation", inputs)

	require.Error(t, err, "should fail with invalid enum value")
	assert.Contains(t, err.Error(), "validation")
	assert.Contains(t, err.Error(), "env")
}

func TestExecutionService_Run_InputValidation_MultipleErrors(t *testing.T) {
	// Multiple validation errors should be aggregated
	min1, max100 := 1, 100
	repo := newMockRepository()
	repo.workflows["multiple-errors"] = &workflow.Workflow{
		Name:    "multiple-errors",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "email",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
			{
				Name:     "env",
				Type:     "string",
				Required: true,
				Validation: &workflow.InputValidation{
					Enum: []string{"dev", "staging", "prod"},
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// All inputs invalid
	inputs := map[string]any{
		"email": "not-an-email",
		"count": 999,
		"env":   "local",
	}

	_, err := execSvc.Run(context.Background(), "multiple-errors", inputs)

	require.Error(t, err, "should fail with multiple validation errors")
	assert.Contains(t, err.Error(), "validation")
	// Error should mention multiple errors or contain multiple error messages
}

func TestExecutionService_Run_InputValidation_DefaultAppliedBeforeValidation(t *testing.T) {
	// Default values should be applied before validation runs
	min1, max100 := 1, 100
	repo := newMockRepository()
	repo.workflows["default-values"] = &workflow.Workflow{
		Name:    "default-values",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Default:  50, // Default value within range
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// No inputs provided - default should be used
	ctx, err := execSvc.Run(context.Background(), "default-values", nil)

	require.NoError(t, err, "should succeed with default value")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify default was applied
	val, ok := ctx.GetInput("count")
	assert.True(t, ok)
	assert.Equal(t, 50, val)
}

func TestExecutionService_Run_InputValidation_TypeCoercion(t *testing.T) {
	// String "42" should be coerced to integer 42 for validation
	min1, max100 := 1, 100
	repo := newMockRepository()
	repo.workflows["type-coercion"] = &workflow.Workflow{
		Name:    "type-coercion",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: true,
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// String "42" should be coerced to integer
	inputs := map[string]any{
		"count": "42",
	}

	ctx, err := execSvc.Run(context.Background(), "type-coercion", inputs)

	require.NoError(t, err, "should succeed with coerced type")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Run_InputValidation_NoValidationRules(t *testing.T) {
	// Inputs without validation rules should still be accepted
	repo := newMockRepository()
	repo.workflows["no-validation"] = &workflow.Workflow{
		Name:    "no-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "name",
				Type:     "string",
				Required: true,
				// No Validation field
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	inputs := map[string]any{
		"name": "anything_goes",
	}

	ctx, err := execSvc.Run(context.Background(), "no-validation", inputs)

	require.NoError(t, err, "should succeed without validation rules")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Run_InputValidation_NoInputDefinitions(t *testing.T) {
	// Workflow without any input definitions should work
	repo := newMockRepository()
	repo.workflows["no-inputs"] = &workflow.Workflow{
		Name:    "no-inputs",
		Initial: "start",
		// No Inputs field
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	// Pass some inputs anyway - should be ignored
	inputs := map[string]any{
		"extra": "value",
	}

	ctx, err := execSvc.Run(context.Background(), "no-inputs", inputs)

	require.NoError(t, err, "should succeed without input definitions")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Run_InputValidation_BooleanType(t *testing.T) {
	// Boolean type validation
	repo := newMockRepository()
	repo.workflows["boolean-validation"] = &workflow.Workflow{
		Name:    "boolean-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "verbose",
				Type:     "boolean",
				Required: true,
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{name: "true bool", input: true, wantErr: false},
		{name: "false bool", input: false, wantErr: false},
		{name: "string true", input: "true", wantErr: false},
		{name: "string false", input: "false", wantErr: false},
		{name: "string yes", input: "yes", wantErr: false},
		{name: "string no", input: "no", wantErr: false},
		{name: "invalid string", input: "maybe", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]any{"verbose": tt.input}
			_, err := execSvc.Run(context.Background(), "boolean-validation", inputs)

			if tt.wantErr {
				require.Error(t, err, "should fail with invalid boolean")
			} else {
				require.NoError(t, err, "should succeed with valid boolean")
			}
		})
	}
}

func TestExecutionService_Run_InputValidation_OptionalWithValidation(t *testing.T) {
	// Optional input with validation rules - should validate if provided
	min1, max100 := 1, 100
	repo := newMockRepository()
	repo.workflows["optional-validation"] = &workflow.Workflow{
		Name:    "optional-validation",
		Initial: "start",
		Inputs: []workflow.Input{
			{
				Name:     "count",
				Type:     "integer",
				Required: false, // Optional
				Validation: &workflow.InputValidation{
					Min: &min1,
					Max: &max100,
				},
			},
		},
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver())

	t.Run("not provided should succeed", func(t *testing.T) {
		ctx, err := execSvc.Run(context.Background(), "optional-validation", nil)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	})

	t.Run("valid value should succeed", func(t *testing.T) {
		inputs := map[string]any{"count": 50}
		ctx, err := execSvc.Run(context.Background(), "optional-validation", inputs)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	})

	t.Run("invalid value should fail", func(t *testing.T) {
		inputs := map[string]any{"count": 999}
		_, err := execSvc.Run(context.Background(), "optional-validation", inputs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation")
	})
}
