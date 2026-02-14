package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// Source: loop_executor_test.go | Tests: 10 | Component: C014-T009
//
// This file contains the core loop executor tests:
// - Constructor tests (TestNewLoopExecutor)
// - Result type tests (TestLoopResult_*)
// - StepExecutorFunc signature tests (TestStepExecutorFunc_*)
//
// Shared mock implementations are in loop_executor_mocks_test.go
//
// Related test files:
// - loop_foreach_test.go: ForEach loop execution tests
// - loop_while_test.go: While loop execution tests
// - loop_iterations_test.go: Iteration resolution tests
// - loop_transitions_*.go: State transition tests

func TestNewLoopExecutor(t *testing.T) {
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()

	executor := application.NewLoopExecutor(logger, evaluator, resolver)

	assert.NotNil(t, executor)
}

func TestLoopResult_Duration(t *testing.T) {
	result := workflow.NewLoopResult()
	result.StartedAt = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	result.CompletedAt = result.StartedAt.Add(5 * time.Second)

	duration := result.Duration()
	assert.Equal(t, 5*time.Second, duration)
}

func TestLoopResult_AllSucceeded(t *testing.T) {
	result := workflow.NewLoopResult()

	// Empty should be false
	assert.False(t, result.AllSucceeded())

	// All success
	result.Iterations = []workflow.IterationResult{
		{Error: nil},
		{Error: nil},
	}
	assert.True(t, result.AllSucceeded())

	// One failure
	result.Iterations = []workflow.IterationResult{
		{Error: nil},
		{Error: errors.New("failed")},
	}
	assert.False(t, result.AllSucceeded())
}

func TestStepExecutorFunc_TypeSignature_ReturnsNextStepAndError(t *testing.T) {
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Return a transition to another step
		return "target_step", nil
	}

	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	assert.NoError(t, err)
	assert.Equal(t, "target_step", nextStep, "should return nextStep value")
}

func TestStepExecutorFunc_NoTransition_ReturnsEmptyString(t *testing.T) {
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// No transition - return empty nextStep
		return "", nil
	}

	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	assert.NoError(t, err)
	assert.Equal(t, "", nextStep, "should return empty string when no transition")
}

func TestStepExecutorFunc_ErrorWithoutTransition(t *testing.T) {
	expectedErr := errors.New("step execution failed")
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Error case - return empty nextStep with error
		return "", expectedErr
	}

	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, "", nextStep, "should return empty nextStep on error")
}

func TestStepExecutorFunc_ErrorWithTransition(t *testing.T) {
	expectedErr := errors.New("step failed but has on_failure transition")
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// On-failure transition case
		return "error_handler_step", expectedErr
	}

	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, "error_handler_step", nextStep, "should return nextStep even on error")
}

func TestStepExecutorFunc_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Check context
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "next_step", nil
	}

	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "test_step", intCtx)

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, "", nextStep, "Should return empty nextStep on cancellation")
}

func TestStepExecutorFunc_NilInterpolationContext(t *testing.T) {
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// Verify nil is handled (executor should validate)
		if intCtx == nil {
			return "", errors.New("interpolation context is nil")
		}
		return "", nil
	}

	ctx := context.Background()
	nextStep, err := stepExecutor(ctx, "test_step", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
	assert.Equal(t, "", nextStep)
}

func TestStepExecutorFunc_EmptyStepName(t *testing.T) {
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		if stepName == "" {
			return "", errors.New("step name cannot be empty")
		}
		return "", nil
	}

	ctx := context.Background()
	intCtx := interpolation.NewContext()
	nextStep, err := stepExecutor(ctx, "", intCtx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
	assert.Equal(t, "", nextStep)
}
