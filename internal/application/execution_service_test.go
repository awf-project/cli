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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "timeout-test", nil)

	require.NoError(t, err) // workflow completes via error path
	assert.Equal(t, "error", ctx.CurrentStep)
}

func TestExecutionService_Run_WorkflowNotFound(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "immediate", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestNewExecutionService(t *testing.T) {
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

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

// =============================================================================
// Resume Tests (F013)
// =============================================================================

func TestExecutionService_Resume_Success(t *testing.T) {
	// Setup: Create a workflow with 3 steps, interrupt after step1, resume from step2
	repo := newMockRepository()
	repo.workflows["resume-test"] = &workflow.Workflow{
		Name:    "resume-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["echo 2"] = &ports.CommandResult{Stdout: "output2\n", ExitCode: 0}
	executor.results["echo 3"] = &ports.CommandResult{Stdout: "output3\n", ExitCode: 0}

	store := newMockStateStore()
	// Pre-populate store with interrupted state (step1 completed, paused at step2)
	interruptedState := &workflow.ExecutionContext{
		WorkflowID:   "test-id-123",
		WorkflowName: "resume-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       map[string]any{"key": "value"},
		States: map[string]workflow.StepState{
			"step1": {
				Name:      "step1",
				Status:    workflow.StatusCompleted,
				Output:    "output1\n",
				ExitCode:  0,
				StartedAt: time.Now().Add(-time.Minute),
			},
		},
		Env:       make(map[string]string),
		StartedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
	}
	store.states["test-id-123"] = interruptedState

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	// Execute resume
	ctx, err := execSvc.Resume(context.Background(), "test-id-123", nil)

	require.NoError(t, err, "resume should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status, "workflow should be completed")
	assert.Equal(t, "done", ctx.CurrentStep, "should end at terminal state")

	// Verify step1 was NOT re-executed (output preserved from before)
	state1, ok := ctx.GetStepState("step1")
	require.True(t, ok, "step1 state should exist")
	assert.Equal(t, "output1\n", state1.Output, "step1 output should be preserved")

	// Verify step2 and step3 were executed
	state2, ok := ctx.GetStepState("step2")
	require.True(t, ok, "step2 state should exist")
	assert.Equal(t, workflow.StatusCompleted, state2.Status, "step2 should be completed")

	state3, ok := ctx.GetStepState("step3")
	require.True(t, ok, "step3 state should exist")
	assert.Equal(t, workflow.StatusCompleted, state3.Status, "step3 should be completed")
}

func TestExecutionService_Resume_NotFound(t *testing.T) {
	// Resume non-existent workflow should fail
	store := newMockStateStore()
	wfSvc := application.NewWorkflowService(newMockRepository(), store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.Resume(context.Background(), "non-existent-id", nil)

	require.Error(t, err, "resume should fail for non-existent workflow")
	assert.Contains(t, err.Error(), "not found", "error should indicate workflow not found")
}

func TestExecutionService_Resume_AlreadyCompleted(t *testing.T) {
	// Resume already-completed workflow should fail
	store := newMockStateStore()
	store.states["completed-id"] = &workflow.ExecutionContext{
		WorkflowID:   "completed-id",
		WorkflowName: "some-workflow",
		Status:       workflow.StatusCompleted, // Already completed
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	repo := newMockRepository()
	repo.workflows["some-workflow"] = &workflow.Workflow{
		Name:    "some-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {Name: "start", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.Resume(context.Background(), "completed-id", nil)

	require.Error(t, err, "resume should fail for completed workflow")
	assert.Contains(t, err.Error(), "completed", "error should mention workflow is completed")
}

func TestExecutionService_Resume_WorkflowDefinitionNotFound(t *testing.T) {
	// Resume workflow when definition no longer exists
	store := newMockStateStore()
	store.states["orphan-id"] = &workflow.ExecutionContext{
		WorkflowID:   "orphan-id",
		WorkflowName: "deleted-workflow", // This workflow no longer exists in repo
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	repo := newMockRepository()
	// No workflows added - "deleted-workflow" doesn't exist

	wfSvc := application.NewWorkflowService(repo, store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.Resume(context.Background(), "orphan-id", nil)

	require.Error(t, err, "resume should fail when workflow definition not found")
	assert.Contains(t, err.Error(), "not found", "error should indicate workflow definition not found")
}

func TestExecutionService_Resume_StepNotFound(t *testing.T) {
	// Resume when current step no longer exists in workflow (definition changed)
	store := newMockStateStore()
	store.states["stale-id"] = &workflow.ExecutionContext{
		WorkflowID:   "stale-id",
		WorkflowName: "changed-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "old_step", // This step was removed from workflow
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	repo := newMockRepository()
	repo.workflows["changed-workflow"] = &workflow.Workflow{
		Name:    "changed-workflow",
		Initial: "new_step",
		Steps: map[string]*workflow.Step{
			"new_step": {Name: "new_step", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
			"done":     {Name: "done", Type: workflow.StepTypeTerminal},
			// Note: "old_step" doesn't exist anymore
		},
	}

	wfSvc := application.NewWorkflowService(repo, store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	_, err := execSvc.Resume(context.Background(), "stale-id", nil)

	require.Error(t, err, "resume should fail when current step no longer exists")
	assert.Contains(t, err.Error(), "old_step", "error should mention the missing step name")
}

func TestExecutionService_Resume_InputOverrides(t *testing.T) {
	// Resume with input overrides - verify overrides are merged
	repo := newMockRepository()
	repo.workflows["override-test"] = &workflow.Workflow{
		Name:    "override-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo {{inputs.key}}", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	store := newMockStateStore()
	store.states["override-id"] = &workflow.ExecutionContext{
		WorkflowID:   "override-id",
		WorkflowName: "override-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step1",
		Inputs:       map[string]any{"key": "original", "unchanged": "value"},
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	executor := newMockExecutor()
	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	// Resume with overrides
	overrides := map[string]any{"key": "overridden"}
	ctx, err := execSvc.Resume(context.Background(), "override-id", overrides)

	require.NoError(t, err, "resume with overrides should succeed")

	// Check that "key" was overridden
	val, ok := ctx.GetInput("key")
	require.True(t, ok, "key input should exist")
	assert.Equal(t, "overridden", val, "key should be overridden")

	// Check that "unchanged" was preserved
	val, ok = ctx.GetInput("unchanged")
	require.True(t, ok, "unchanged input should exist")
	assert.Equal(t, "value", val, "unchanged should be preserved")
}

func TestExecutionService_Resume_SkipsCompletedSteps(t *testing.T) {
	// Verify that completed steps are skipped during resume
	repo := newMockRepository()
	repo.workflows["skip-test"] = &workflow.Workflow{
		Name:    "skip-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "step1_cmd", OnSuccess: "step2"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "step2_cmd", OnSuccess: "step3"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "step3_cmd", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Track which commands were executed
	executor := newRetryCountingExecutor()

	store := newMockStateStore()
	// step1 and step2 already completed, currently at step3
	store.states["skip-id"] = &workflow.ExecutionContext{
		WorkflowID:   "skip-id",
		WorkflowName: "skip-test",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step3",
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", Status: workflow.StatusCompleted, Output: "out1"},
			"step2": {Name: "step2", Status: workflow.StatusCompleted, Output: "out2"},
		},
		Env: make(map[string]string),
	}

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Resume(context.Background(), "skip-id", nil)

	require.NoError(t, err, "resume should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify step1 and step2 were NOT executed (commands not called)
	assert.Equal(t, 0, executor.calls["step1_cmd"], "step1 should not be re-executed")
	assert.Equal(t, 0, executor.calls["step2_cmd"], "step2 should not be re-executed")

	// Verify step3 WAS executed
	assert.Equal(t, 1, executor.calls["step3_cmd"], "step3 should be executed")
}

func TestExecutionService_Resume_ParallelStep(t *testing.T) {
	// Resume from a parallel step
	repo := newMockRepository()
	repo.workflows["parallel-resume"] = &workflow.Workflow{
		Name:    "parallel-resume",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				Strategy:  "all_succeed",
				OnSuccess: "done",
			},
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand, Command: "echo b1", OnSuccess: "done"},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand, Command: "echo b2", OnSuccess: "done"},
			"done":    {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	store := newMockStateStore()
	store.states["parallel-id"] = &workflow.ExecutionContext{
		WorkflowID:   "parallel-id",
		WorkflowName: "parallel-resume",
		Status:       workflow.StatusRunning,
		CurrentStep:  "parallel",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Resume(context.Background(), "parallel-id", nil)

	require.NoError(t, err, "resume with parallel step should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}

func TestExecutionService_Resume_FailedStatus(t *testing.T) {
	// Can resume a workflow that was in failed status (retry after fixing issue)
	repo := newMockRepository()
	repo.workflows["failed-resume"] = &workflow.Workflow{
		Name:    "failed-resume",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	store := newMockStateStore()
	store.states["failed-id"] = &workflow.ExecutionContext{
		WorkflowID:   "failed-id",
		WorkflowName: "failed-resume",
		Status:       workflow.StatusFailed, // Previously failed
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Resume(context.Background(), "failed-id", nil)

	require.NoError(t, err, "resume of failed workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

func TestExecutionService_Resume_CancelledStatus(t *testing.T) {
	// Can resume a workflow that was cancelled
	repo := newMockRepository()
	repo.workflows["cancelled-resume"] = &workflow.Workflow{
		Name:    "cancelled-resume",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo ok", OnSuccess: "done"},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	store := newMockStateStore()
	store.states["cancelled-id"] = &workflow.ExecutionContext{
		WorkflowID:   "cancelled-id",
		WorkflowName: "cancelled-resume",
		Status:       workflow.StatusCancelled, // Previously cancelled
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	}

	wfSvc := application.NewWorkflowService(repo, store, executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Resume(context.Background(), "cancelled-id", nil)

	require.NoError(t, err, "resume of cancelled workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}

// =============================================================================
// ListResumable Tests (F013)
// =============================================================================

func TestExecutionService_ListResumable_FiltersCompleted(t *testing.T) {
	// ListResumable should only return non-completed executions
	store := newMockStateStore()

	// Add various states
	store.states["running-1"] = &workflow.ExecutionContext{
		WorkflowID:   "running-1",
		WorkflowName: "wf1",
		Status:       workflow.StatusRunning,
		CurrentStep:  "step2",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}
	store.states["failed-1"] = &workflow.ExecutionContext{
		WorkflowID:   "failed-1",
		WorkflowName: "wf2",
		Status:       workflow.StatusFailed,
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}
	store.states["completed-1"] = &workflow.ExecutionContext{
		WorkflowID:   "completed-1",
		WorkflowName: "wf3",
		Status:       workflow.StatusCompleted, // Should be filtered out
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}
	store.states["cancelled-1"] = &workflow.ExecutionContext{
		WorkflowID:   "cancelled-1",
		WorkflowName: "wf4",
		Status:       workflow.StatusCancelled,
		CurrentStep:  "step1",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}
	store.states["pending-1"] = &workflow.ExecutionContext{
		WorkflowID:   "pending-1",
		WorkflowName: "wf5",
		Status:       workflow.StatusPending,
		CurrentStep:  "",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}

	wfSvc := application.NewWorkflowService(newMockRepository(), store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err, "list resumable should succeed")
	assert.Len(t, resumable, 4, "should return 4 resumable workflows (all except completed)")

	// Verify completed is not in the list
	for _, exec := range resumable {
		assert.NotEqual(t, workflow.StatusCompleted, exec.Status, "completed workflows should not be included")
	}
}

func TestExecutionService_ListResumable_Empty(t *testing.T) {
	// ListResumable with no states should return empty list
	store := newMockStateStore()

	wfSvc := application.NewWorkflowService(newMockRepository(), store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err, "list resumable should succeed")
	assert.Empty(t, resumable, "should return empty list when no states")
}

func TestExecutionService_ListResumable_AllCompleted(t *testing.T) {
	// ListResumable when all workflows are completed should return empty
	store := newMockStateStore()
	store.states["completed-1"] = &workflow.ExecutionContext{
		WorkflowID:   "completed-1",
		WorkflowName: "wf1",
		Status:       workflow.StatusCompleted,
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}
	store.states["completed-2"] = &workflow.ExecutionContext{
		WorkflowID:   "completed-2",
		WorkflowName: "wf2",
		Status:       workflow.StatusCompleted,
		CurrentStep:  "done",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
	}

	wfSvc := application.NewWorkflowService(newMockRepository(), store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err, "list resumable should succeed")
	assert.Empty(t, resumable, "should return empty list when all completed")
}

func TestExecutionService_ListResumable_ReturnsCorrectFields(t *testing.T) {
	// Verify ListResumable returns all required fields
	store := newMockStateStore()
	now := time.Now()
	store.states["test-id"] = &workflow.ExecutionContext{
		WorkflowID:   "test-id",
		WorkflowName: "test-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "current",
		Inputs:       map[string]any{"key": "value"},
		States: map[string]workflow.StepState{
			"step1": {Name: "step1", Status: workflow.StatusCompleted},
		},
		StartedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}

	wfSvc := application.NewWorkflowService(newMockRepository(), store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	resumable, err := execSvc.ListResumable(context.Background())

	require.NoError(t, err)
	require.Len(t, resumable, 1)

	exec := resumable[0]
	assert.Equal(t, "test-id", exec.WorkflowID)
	assert.Equal(t, "test-workflow", exec.WorkflowName)
	assert.Equal(t, workflow.StatusRunning, exec.Status)
	assert.Equal(t, "current", exec.CurrentStep)
	assert.Equal(t, now.Round(time.Second), exec.UpdatedAt.Round(time.Second))
}

// =============================================================================
// Call Workflow Dispatcher Tests (F023: T013)
// =============================================================================
// These tests verify that the dispatcher correctly routes StepTypeCallWorkflow
// steps to the executeCallWorkflowStep method in both Run() and Resume() paths.

func TestExecutionService_Run_CallWorkflow_DispatcherRouting(t *testing.T) {
	// Test that the dispatcher correctly routes call_workflow steps
	repo := newMockRepository()

	// Simple child workflow
	repo.workflows["child"] = &workflow.Workflow{
		Name:    "child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo child",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent workflow with call_workflow step
	repo.workflows["parent"] = &workflow.Workflow{
		Name:    "parent",
		Initial: "call_child",
		Steps: map[string]*workflow.Step{
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "child",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "parent", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify call_child step was executed (recorded in state)
	state, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step should be recorded")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

func TestExecutionService_Run_CallWorkflow_InSequence(t *testing.T) {
	// Test call_workflow step in a sequence with other step types
	repo := newMockRepository()

	// Child workflow
	repo.workflows["helper"] = &workflow.Workflow{
		Name:    "helper",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo helper",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent: command -> call_workflow -> command -> done
	repo.workflows["sequence"] = &workflow.Workflow{
		Name:    "sequence",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step1",
				OnSuccess: "call_helper",
			},
			"call_helper": {
				Name: "call_helper",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "helper",
				},
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step2",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "sequence", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// All steps should be executed
	_, hasStep1 := ctx.GetStepState("step1")
	_, hasCallHelper := ctx.GetStepState("call_helper")
	_, hasStep2 := ctx.GetStepState("step2")

	assert.True(t, hasStep1, "step1 should be recorded")
	assert.True(t, hasCallHelper, "call_helper should be recorded")
	assert.True(t, hasStep2, "step2 should be recorded")
}

func TestExecutionService_Run_CallWorkflow_FailureTransition(t *testing.T) {
	// Test that call_workflow step follows on_failure transition when child fails
	repo := newMockRepository()

	// Failing child workflow
	repo.workflows["failing-child"] = &workflow.Workflow{
		Name:    "failing-child",
		Initial: "fail",
		Steps: map[string]*workflow.Step{
			"fail": {
				Name:      "fail",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with on_failure transition
	repo.workflows["parent-with-handler"] = &workflow.Workflow{
		Name:    "parent-with-handler",
		Initial: "call_child",
		Steps: map[string]*workflow.Step{
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "failing-child",
				},
				OnSuccess: "success",
				OnFailure: "error_handler",
			},
			"success":       {Name: "success", Type: workflow.StepTypeTerminal},
			"error_handler": {Name: "error_handler", Type: workflow.StepTypeTerminal},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{ExitCode: 1, Stderr: "command failed"}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "parent-with-handler", nil)

	require.NoError(t, err) // Workflow completes via error handler
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "error_handler", ctx.CurrentStep, "should follow on_failure transition")
}

func TestExecutionService_Resume_CallWorkflow_DispatcherRouting(t *testing.T) {
	// Test that Resume correctly routes call_workflow steps
	// This tests the executeFromStep dispatcher
	repo := newMockRepository()

	// Child workflow
	repo.workflows["child"] = &workflow.Workflow{
		Name:    "child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo child",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with call_workflow
	repo.workflows["resume-parent"] = &workflow.Workflow{
		Name:    "resume-parent",
		Initial: "prep",
		Steps: map[string]*workflow.Step{
			"prep": {
				Name:      "prep",
				Type:      workflow.StepTypeCommand,
				Command:   "echo prep",
				OnSuccess: "call_child",
			},
			"call_child": {
				Name: "call_child",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "child",
				},
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Create persisted state at call_child step
	store := newMockStateStore()
	store.states["resume-test-id"] = &workflow.ExecutionContext{
		WorkflowID:   "resume-test-id",
		WorkflowName: "resume-parent",
		Status:       workflow.StatusRunning,
		CurrentStep:  "call_child", // Resuming at call_workflow step
		Inputs:       make(map[string]any),
		States: map[string]workflow.StepState{
			"prep": {Name: "prep", Status: workflow.StatusCompleted},
		},
	}

	wfSvc := application.NewWorkflowService(repo, store, newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), store, &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Resume(context.Background(), "resume-test-id", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)

	// Verify call_child was executed via resume
	state, ok := ctx.GetStepState("call_child")
	require.True(t, ok, "call_child step should be recorded after resume")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

func TestExecutionService_Run_CallWorkflow_MixedStepTypes(t *testing.T) {
	// Test dispatcher correctly handles workflow with mixed step types
	// Testing: command -> call_workflow -> command sequence
	repo := newMockRepository()

	// Child workflow
	repo.workflows["subflow"] = &workflow.Workflow{
		Name:    "subflow",
		Initial: "sub_work",
		Steps: map[string]*workflow.Step{
			"sub_work": {
				Name:      "sub_work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo sub",
				OnSuccess: "sub_done",
			},
			"sub_done": {Name: "sub_done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with command and call_workflow steps interleaved
	repo.workflows["mixed-types"] = &workflow.Workflow{
		Name:    "mixed-types",
		Initial: "first_step",
		Steps: map[string]*workflow.Step{
			"first_step": {
				Name:      "first_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "call_subflow",
			},
			"call_subflow": {
				Name: "call_subflow",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "subflow",
				},
				OnSuccess: "final_step",
			},
			"final_step": {
				Name:      "final_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo final",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "mixed-types", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify all step types were dispatched correctly
	_, hasFirst := ctx.GetStepState("first_step")
	_, hasCallSubflow := ctx.GetStepState("call_subflow")
	_, hasFinal := ctx.GetStepState("final_step")

	assert.True(t, hasFirst, "first_step should be recorded")
	assert.True(t, hasCallSubflow, "call_subflow should be recorded")
	assert.True(t, hasFinal, "final_step should be recorded")
}

func TestExecutionService_Run_CallWorkflow_DefaultStep(t *testing.T) {
	// Ensure call_workflow doesn't fall through to default case
	// This test verifies call_workflow is handled before the default case
	repo := newMockRepository()

	// Child workflow
	repo.workflows["simple-child"] = &workflow.Workflow{
		Name:    "simple-child",
		Initial: "work",
		Steps: map[string]*workflow.Step{
			"work": {
				Name:      "work",
				Type:      workflow.StepTypeCommand,
				Command:   "echo done",
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	// Parent with only call_workflow step
	repo.workflows["only-call"] = &workflow.Workflow{
		Name:    "only-call",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "simple-child",
				},
				OnSuccess: "done",
				// Note: No Command field - if dispatcher used default case,
				// it would try to execute empty command and fail
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), newMockExecutor(), &mockLogger{})
	execSvc := application.NewExecutionService(wfSvc, newMockExecutor(), newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "only-call", nil)

	// Should succeed without trying to execute empty command
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
}
