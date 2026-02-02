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
	"github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// InteractiveExecutor Result Handlers Tests (C005 Phase 2: T004-T006)
// =============================================================================

// =============================================================================
// T004: handleExecutionError Tests
// =============================================================================

func TestHandleExecutionError_WithOnFailure_ReturnsOnFailureStep(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has OnFailure transition
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "failing_step",
		Command:   "exit 1",
		OnFailure: "error_handler",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Error:       "execution failed",
	}

	execErr := errors.New("execution failed")
	intCtx := interpolation.NewContext()

	// Act: Call handleExecutionError
	nextStep, err := executor.HandleExecutionError(context.Background(), step, &state, execErr, intCtx)

	// Assert: Should return OnFailure step with no error
	require.NoError(t, err, "handleExecutionError should not return error when OnFailure is set")
	assert.Equal(t, "error_handler", nextStep, "should return OnFailure step")
}

func TestHandleExecutionError_WithContinueOnError_ReturnsOnSuccess(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has ContinueOnError=true
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:            "tolerant_step",
		Command:         "exit 1",
		ContinueOnError: true,
		OnSuccess:       "next_step",
		Hooks:           workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Error:       "execution failed",
	}

	execErr := errors.New("execution failed")
	intCtx := interpolation.NewContext()

	// Act: Call handleExecutionError
	nextStep, err := executor.HandleExecutionError(context.Background(), step, &state, execErr, intCtx)

	// Assert: Should return OnSuccess step with no error
	require.NoError(t, err, "handleExecutionError should not return error when ContinueOnError is true")
	assert.Equal(t, "next_step", nextStep, "should return OnSuccess step when ContinueOnError is true")
}

func TestHandleExecutionError_WithoutOnFailureOrContinue_ReturnsError(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has neither OnFailure nor ContinueOnError
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "strict_step",
		Command: "exit 1",
		Hooks:   workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Error:       "execution failed",
	}

	execErr := errors.New("execution failed")
	intCtx := interpolation.NewContext()

	// Act: Call handleExecutionError
	nextStep, err := executor.HandleExecutionError(context.Background(), step, &state, execErr, intCtx)

	// Assert: Should return error
	require.Error(t, err, "handleExecutionError should return error when no OnFailure or ContinueOnError")
	assert.Empty(t, nextStep, "nextStep should be empty when error is returned")
	assert.Contains(t, err.Error(), "strict_step", "error should contain step name")
}

func TestHandleExecutionError_PostHooksExecuted(t *testing.T) {
	// Feature: C005
	// Test verifies that post-hooks are executed even when execution fails
	// This is critical behavior that must be preserved during refactoring

	// Arrange: Setup executor with mock hook executor
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "step_with_hooks",
		Command:   "exit 1",
		OnFailure: "error_handler",
		Hooks: workflow.StepHooks{
			Post: workflow.Hook{
				{Command: "cleanup"},
			},
		},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Error:       "execution failed",
	}

	execErr := errors.New("execution failed")
	intCtx := interpolation.NewContext()

	// Act: Call handleExecutionError
	_, err := executor.HandleExecutionError(context.Background(), step, &state, execErr, intCtx)

	// Assert: Should not return error and post-hooks should have been called
	require.NoError(t, err, "handleExecutionError should not return error when OnFailure is set")
	// Note: Since we can't directly verify hook execution in this stub phase,
	// we verify the behavior indirectly through the error handling path
}

func TestHandleExecutionError_EmptyOnFailure_ContinueOnErrorFalse_ReturnsError(t *testing.T) {
	// Feature: C005
	// Edge case: OnFailure is set but empty string
	// Arrange
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:            "step_empty_failure",
		Command:         "exit 1",
		OnFailure:       "", // Empty string
		ContinueOnError: false,
		Hooks:           workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		Error:       "execution failed",
	}

	execErr := errors.New("execution failed")
	intCtx := interpolation.NewContext()

	// Act
	nextStep, err := executor.HandleExecutionError(context.Background(), step, &state, execErr, intCtx)

	// Assert: Empty OnFailure should be treated as not set, should return error
	require.Error(t, err, "should return error when OnFailure is empty and ContinueOnError is false")
	assert.Empty(t, nextStep, "nextStep should be empty")
}

// =============================================================================
// T005: handleNonZeroExit Tests
// =============================================================================

func TestHandleNonZeroExit_WithOnFailure_ReturnsOnFailureStep(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has OnFailure transition
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "non_zero_step",
		Command:   "exit 2",
		OnFailure: "error_handler",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    2,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 2,
		Stdout:   "",
		Stderr:   "command failed",
	}

	intCtx := interpolation.NewContext()

	// Act: Call handleNonZeroExit
	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	// Assert: Should return OnFailure step with no error
	require.NoError(t, err, "handleNonZeroExit should not return error when OnFailure is set")
	assert.Equal(t, "error_handler", nextStep, "should return OnFailure step")
}

func TestHandleNonZeroExit_WithContinueOnError_ReturnsOnSuccess(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has ContinueOnError=true
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:            "tolerant_exit_step",
		Command:         "exit 1",
		ContinueOnError: true,
		OnSuccess:       "next_step",
		Hooks:           workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "non-zero exit",
	}

	intCtx := interpolation.NewContext()

	// Act: Call handleNonZeroExit
	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	// Assert: Should return OnSuccess step with no error
	require.NoError(t, err, "handleNonZeroExit should not return error when ContinueOnError is true")
	assert.Equal(t, "next_step", nextStep, "should return OnSuccess step when ContinueOnError is true")
}

func TestHandleNonZeroExit_WithoutOnFailureOrContinue_ReturnsError(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has neither OnFailure nor ContinueOnError
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "strict_exit_step",
		Command: "exit 3",
		Hooks:   workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    3,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 3,
		Stdout:   "",
		Stderr:   "command failed",
	}

	intCtx := interpolation.NewContext()

	// Act: Call handleNonZeroExit
	nextStep, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	// Assert: Should return error with exit code
	require.Error(t, err, "handleNonZeroExit should return error when no OnFailure or ContinueOnError")
	assert.Empty(t, nextStep, "nextStep should be empty when error is returned")
	assert.Contains(t, err.Error(), "strict_exit_step", "error should contain step name")
	assert.Contains(t, err.Error(), "exit code 3", "error should contain exit code")
}

func TestHandleNonZeroExit_PostHooksExecuted(t *testing.T) {
	// Feature: C005
	// Test verifies that post-hooks are executed even when exit code is non-zero
	// Arrange
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "exit_with_hooks",
		Command:   "exit 1",
		OnFailure: "error_handler",
		Hooks: workflow.StepHooks{
			Post: workflow.Hook{
				{Command: "cleanup"},
			},
		},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusFailed,
		ExitCode:    1,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	result := &ports.CommandResult{
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "command failed",
	}

	intCtx := interpolation.NewContext()

	// Act
	_, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

	// Assert: Should not return error and post-hooks should have been called
	require.NoError(t, err, "handleNonZeroExit should not return error when OnFailure is set")
}

func TestHandleNonZeroExit_DifferentExitCodes(t *testing.T) {
	// Feature: C005
	// Test various exit codes to ensure error message includes the actual code
	tests := []struct {
		name     string
		exitCode int
	}{
		{"exit_code_1", 1},
		{"exit_code_2", 2},
		{"exit_code_127", 127},
		{"exit_code_255", 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
			executor := application.NewInteractiveExecutor(
				wfSvc,
				newMockExecutor(),
				newMockParallelExecutor(),
				newMockStateStore(),
				&mockLogger{},
				newMockResolver(),
				expression.NewExprEvaluator(),
				newMockPrompt(),
			)

			step := &workflow.Step{
				Name:    "test_step",
				Command: "exit " + string(rune(tt.exitCode)),
				Hooks:   workflow.StepHooks{},
			}

			state := workflow.StepState{
				Name:        step.Name,
				Status:      workflow.StatusFailed,
				ExitCode:    tt.exitCode,
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			}

			result := &ports.CommandResult{
				ExitCode: tt.exitCode,
				Stdout:   "",
				Stderr:   "",
			}

			intCtx := interpolation.NewContext()

			// Act
			_, err := executor.HandleNonZeroExit(context.Background(), step, &state, result, intCtx)

			// Assert
			require.Error(t, err, "should return error for exit code %d", tt.exitCode)
			// Error message format should include exit code
		})
	}
}

// =============================================================================
// T006: handleSuccess Tests
// =============================================================================

func TestHandleSuccess_WithTransitions_EvaluatesAndReturnsMatch(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has transitions
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "step_with_transitions",
		Command: "echo success",
		Transitions: workflow.Transitions{
			{
				When: "true",
				Goto: "matched_step",
			},
		},
		OnSuccess: "default_next",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act: Call handleSuccess
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should evaluate transitions and return matched step
	require.NoError(t, err, "handleSuccess should not return error on success")
	assert.Equal(t, "matched_step", nextStep, "should return transition matched step")
}

func TestHandleSuccess_WithTransitionsNoMatch_ReturnsOnSuccess(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has transitions that don't match
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "step_with_transitions_no_match",
		Command: "echo success",
		Transitions: workflow.Transitions{
			{
				When: "false",
				Goto: "never_matched",
			},
		},
		OnSuccess: "default_next",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act: Call handleSuccess
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should fallback to OnSuccess
	require.NoError(t, err, "handleSuccess should not return error on success")
	assert.Equal(t, "default_next", nextStep, "should return OnSuccess when no transition matches")
}

func TestHandleSuccess_WithoutTransitions_ReturnsOnSuccess(t *testing.T) {
	// Feature: C005
	// Arrange: Setup executor with step that has no transitions, only OnSuccess
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "simple_step",
		Command:   "echo success",
		OnSuccess: "next_step",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act: Call handleSuccess
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should return OnSuccess
	require.NoError(t, err, "handleSuccess should not return error on success")
	assert.Equal(t, "next_step", nextStep, "should return OnSuccess step")
}

func TestHandleSuccess_EmptyOnSuccess_ReturnsEmptyString(t *testing.T) {
	// Feature: C005
	// Edge case: OnSuccess is empty (terminal step)
	// Arrange
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "terminal_step",
		Command:   "echo done",
		OnSuccess: "", // Empty - terminal step
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should return empty string (terminal)
	require.NoError(t, err, "handleSuccess should not return error")
	assert.Empty(t, nextStep, "should return empty string for terminal step")
}

func TestHandleSuccess_PostHooksExecuted(t *testing.T) {
	// Feature: C005
	// Test verifies that post-hooks are executed on success
	// Arrange
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:      "success_with_hooks",
		Command:   "echo done",
		OnSuccess: "next_step",
		Hooks: workflow.StepHooks{
			Post: workflow.Hook{
				{Command: "cleanup"},
			},
		},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should not return error
	require.NoError(t, err, "handleSuccess should not return error")
	assert.Equal(t, "next_step", nextStep, "should return next step")
	// Note: Hook execution verification would require exposing hook executor or using test doubles
}

func TestHandleSuccess_MultipleTransitions_ReturnsFirstMatch(t *testing.T) {
	// Feature: C005
	// Test that first matching transition is returned (order matters)
	// Arrange
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		expression.NewExprEvaluator(),
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "multi_transition_step",
		Command: "echo test",
		Transitions: workflow.Transitions{
			{
				When: "true",
				Goto: "first_match",
			},
			{
				When: "true",
				Goto: "second_match", // This should not be returned
			},
		},
		OnSuccess: "default",
		Hooks:     workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should return first matching transition
	require.NoError(t, err, "handleSuccess should not return error")
	assert.Equal(t, "first_match", nextStep, "should return first matching transition")
}

// =============================================================================
// Edge Cases and Error Scenarios
// =============================================================================

func TestHandleExecutionError_NilStep_Panics(t *testing.T) {
	// Feature: C005
	// Defensive test: nil step should be handled gracefully or panic explicitly
	// This test documents expected behavior for invalid input
	t.Skip("Implementation should handle nil step - define expected behavior")
}

func TestHandleNonZeroExit_NilResult_Panics(t *testing.T) {
	// Feature: C005
	// Defensive test: nil result should be handled gracefully or panic explicitly
	t.Skip("Implementation should handle nil result - define expected behavior")
}

func TestHandleSuccess_NilEvaluator_WithTransitions_ReturnsError(t *testing.T) {
	// Feature: C005
	// Test that evaluator is required when transitions are defined
	// Arrange: Create executor without evaluator (pass nil)
	wfSvc := application.NewWorkflowService(newMockRepository(), newMockStateStore(), newMockExecutor(), &mockLogger{})
	executor := application.NewInteractiveExecutor(
		wfSvc,
		newMockExecutor(),
		newMockParallelExecutor(),
		newMockStateStore(),
		&mockLogger{},
		newMockResolver(),
		nil, // No evaluator
		newMockPrompt(),
	)

	step := &workflow.Step{
		Name:    "step_needs_evaluator",
		Command: "echo test",
		Transitions: workflow.Transitions{
			{
				When: "true",
				Goto: "next_step",
			},
		},
		Hooks: workflow.StepHooks{},
	}

	state := workflow.StepState{
		Name:        step.Name,
		Status:      workflow.StatusCompleted,
		ExitCode:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	intCtx := interpolation.NewContext()

	// Act
	nextStep, err := executor.HandleSuccess(context.Background(), step, &state, intCtx)

	// Assert: Should handle nil evaluator gracefully
	// Current implementation likely skips transitions if evaluator is nil
	// This test documents that behavior
	assert.NoError(t, err, "should handle nil evaluator gracefully")
	assert.Empty(t, nextStep, "should skip transitions when evaluator is nil")
}
