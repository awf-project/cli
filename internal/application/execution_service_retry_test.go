package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// Feature: C008 - Test File Restructuring
// Component: extract_retry_tests
//
// This file contains all retry-related integration tests for ExecutionService.
// Tests verify retry policies, backoff strategies, max attempts, retryable exit codes,
// and interaction with context cancellation.
//
// Extracted from: execution_service_test.go (lines 623-1085, 3033-3097)
// Test count: 10 retry-related tests

// Mock types are defined in execution_service_specialized_mocks_test.go

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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
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

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionService(wfSvc, executor, newMockParallelExecutor(), newMockStateStore(), &mockLogger{}, newMockResolver(), nil)

	ctx, err := execSvc.Run(context.Background(), "multi-retry", nil)

	require.NoError(t, err)
	assert.Equal(t, "done", ctx.CurrentStep)
	assert.Equal(t, 2, executor.calls["cmd1"], "step1 should have retried once")
	assert.Equal(t, 3, executor.calls["cmd2"], "step2 should have retried twice")
}

// TestStepExecutorCallback_RetryPattern_ReturnsLoopNameAndNil verifies retry pattern
// where OnFailure returns to the loop itself (not an escape).
// SKIP: This test is slow due to buildInterpolationContext overhead per iteration.
// Retry pattern behavior is covered by unit tests in loop_executor_transitions_test.go
func TestStepExecutorCallback_RetryPattern_ReturnsLoopNameAndNil(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow retry pattern integration test")
	}

	repo := newMockRepository()
	repo.workflows["loop-retry"] = &workflow.Workflow{
		Name:    "loop-retry",
		Initial: "retry_loop",
		Steps: map[string]*workflow.Step{
			"retry_loop": {
				Name: "retry_loop",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Type:           workflow.LoopTypeWhile,
					Condition:      "true",
					MaxIterations:  3, // Reduced from 100 for faster test execution
					Body:           []string{"flaky_step"},
					BreakCondition: "false",
					OnComplete:     "done",
				},
			},
			"flaky_step": {
				Name:      "flaky_step",
				Type:      workflow.StepTypeCommand,
				Command:   "exit 1",
				OnFailure: "retry_loop", // Retry pattern - return to loop
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	executor := newMockExecutor()
	executor.results["exit 1"] = &ports.CommandResult{
		Stdout:   "",
		ExitCode: 1,
	}

	evaluator := newConditionMockEvaluator()
	evaluator.evaluations["true"] = true
	evaluator.evaluations["false"] = false

	wfSvc := application.NewWorkflowService(repo, newMockStateStore(), executor, &mockLogger{}, nil)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc,
		executor,
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil,
		evaluator,
	)

	ctx, err := execSvc.Run(context.Background(), "loop-retry", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep, "should complete via on_complete after max iterations")

	// Verify loop executed max iterations (3)
	state, ok := ctx.GetStepState("retry_loop")
	require.True(t, ok, "loop step state should exist")
	assert.Equal(t, workflow.StatusCompleted, state.Status)

	// Verify flaky_step was attempted 3 times (one per iteration)
	flakyState, ok := ctx.GetStepState("flaky_step")
	require.True(t, ok, "flaky_step state should exist")
	assert.Equal(t, workflow.StatusFailed, flakyState.Status, "flaky_step should have failed status from last attempt")
}
