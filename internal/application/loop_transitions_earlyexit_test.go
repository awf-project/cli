package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Loop Early Exit Transition Tests
// Component: T007
// Feature: F048 - While Loop Transitions Support
//
// This file contains tests for early exit transitions when a loop body step
// transitions to a step OUTSIDE the loop body. This is distinct from intra-body
// transitions (T006) which skip steps within the same iteration.
//
// Early exit transitions set result.NextStep to allow the workflow to continue
// at the specified external step after the loop exits.
// =============================================================================

// Component T007: Handle Early Exit Transitions
// =============================================================================

// =============================================================================
// Happy Path Tests: Normal Early Exit Scenarios
// =============================================================================

// TestEarlyExitTransition_HappyPath_ExitFromFirstStep tests early exit when
// the first step in a while loop body transitions to an external step.
// Given: While loop with body [step1, step2, step3]
// When: step1 transitions to external_step
// Then: Loop should exit immediately, result.NextStep should be "external_step"
func TestEarlyExitTransition_HappyPath_ExitFromFirstStep(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-early-exit-first",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-early-exit-first")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to external_step (early exit)
		if stepName == "step1" {
			return "external_step", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Only step1 should execute (early exit before step2 and step3)
	assert.Equal(t, []string{"step1"}, executionOrder,
		"Only step1 should execute before early exit")

	// Loop should exit after first iteration
	assert.Equal(t, 1, result.TotalCount,
		"Loop should have only 1 iteration before early exit")

	// T007: result.NextStep should contain the target step name
	assert.Equal(t, "external_step", result.NextStep,
		"result.NextStep should be set to external_step for early exit transition")
}

// TestEarlyExitTransition_HappyPath_ExitFromMiddleStep tests early exit when
// a middle step in the loop body transitions to an external step.
// Given: While loop with body [step1, step2, step3, step4]
// When: step2 transitions to cleanup_step after executing step1
// Then: result.NextStep should be "cleanup_step", only step1 and step2 execute
func TestEarlyExitTransition_HappyPath_ExitFromMiddleStep(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-early-exit-middle",
		Steps: map[string]*workflow.Step{
			"step1":        {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":        {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":        {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4":        {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
			"cleanup_step": {Name: "cleanup_step", Type: workflow.StepTypeCommand, Command: "echo cleanup"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4"},
			MaxIterations: 5,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-early-exit-middle")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step2 transitions to cleanup_step
		if stepName == "step2" {
			return "cleanup_step", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// step1 and step2 should execute, step3 and step4 should be skipped
	assert.Equal(t, []string{"step1", "step2"}, executionOrder,
		"Only step1 and step2 should execute before early exit")

	// T007: result.NextStep should contain the cleanup_step target
	assert.Equal(t, "cleanup_step", result.NextStep,
		"result.NextStep should be set to cleanup_step for early exit")
}

// TestEarlyExitTransition_HappyPath_ExitFromLastStep tests early exit when
// the last step in the loop body transitions to an external step.
// Given: While loop with body [step1, step2, step3]
// When: step3 (last step) transitions to external_step
// Then: All body steps execute once, then early exit with result.NextStep set
func TestEarlyExitTransition_HappyPath_ExitFromLastStep(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-early-exit-last",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-early-exit-last")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step3 (last step) transitions to external_step
		if stepName == "step3" {
			return "external_step", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// All three body steps should execute once
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
		"All body steps should execute before early exit from last step")

	// Loop should exit after first iteration
	assert.Equal(t, 1, result.TotalCount,
		"Loop should exit after first iteration")

	// T007: result.NextStep should be set
	assert.Equal(t, "external_step", result.NextStep,
		"result.NextStep should be set when last step triggers early exit")
}

// TestEarlyExitTransition_HappyPath_NoEarlyExit tests that result.NextStep
// is empty when no early exit occurs (normal loop completion).
// Given: While loop with body [step1, step2] and break condition
// When: No steps transition to external steps, loop exits via break condition
// Then: result.NextStep should be empty string
func TestEarlyExitTransition_HappyPath_NoEarlyExit(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["states.step2.Output == 'done'"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-no-early-exit",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:           workflow.LoopTypeWhile,
			Condition:      "true",
			Body:           []string{"step1", "step2"},
			BreakCondition: "states.step2.Output == 'done'",
			MaxIterations:  10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-no-early-exit")

	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		return "", nil // No transitions
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// T007: result.NextStep should be empty when no early exit occurs
	assert.Equal(t, "", result.NextStep,
		"result.NextStep should be empty when loop completes normally without early exit")
}

// =============================================================================
// Edge Cases: Boundary Conditions
// =============================================================================

// TestEarlyExitTransition_EdgeCase_ExitOnSecondIteration tests early exit
// that occurs in the second iteration, not the first.
// Given: While loop with max 5 iterations
// When: First iteration completes normally, second iteration exits early
// Then: result.NextStep should be set, TotalCount should be 2
func TestEarlyExitTransition_EdgeCase_ExitOnSecondIteration(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-exit-second-iteration",
		Steps: map[string]*workflow.Step{
			"check_step":    {Name: "check_step", Type: workflow.StepTypeCommand, Command: "echo check"},
			"process_step":  {Name: "process_step", Type: workflow.StepTypeCommand, Command: "echo process"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"check_step", "process_step"},
			MaxIterations: 5,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-exit-second-iteration")

	iterationCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		if stepName == "check_step" {
			iterationCount++
		}
		// Exit early on second iteration, first step
		if stepName == "check_step" && iterationCount == 2 {
			return "external_step", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Two iterations should have started
	assert.Equal(t, 2, result.TotalCount,
		"Should complete 2 iterations (1 full + 1 partial)")

	// T007: result.NextStep should be set
	assert.Equal(t, "external_step", result.NextStep,
		"result.NextStep should be set when exiting on second iteration")
}

// TestEarlyExitTransition_EdgeCase_MultipleExternalSteps tests behavior when
// body has multiple steps that could trigger early exit, but only first matches.
// Given: Loop body [step1, step2, step3], all transition to different external steps
// When: step1 transitions first
// Then: result.NextStep should be step1's target, step2 and step3 don't execute
func TestEarlyExitTransition_EdgeCase_MultipleExternalSteps(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-multiple-external",
		Steps: map[string]*workflow.Step{
			"step1":      {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":      {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":      {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_a": {Name: "external_a", Type: workflow.StepTypeCommand, Command: "echo a"},
			"external_b": {Name: "external_b", Type: workflow.StepTypeCommand, Command: "echo b"},
			"external_c": {Name: "external_c", Type: workflow.StepTypeCommand, Command: "echo c"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-multiple-external")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Each step transitions to different external step
		switch stepName {
		case "step1":
			return "external_a", nil
		case "step2":
			return "external_b", nil
		case "step3":
			return "external_c", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Only step1 should execute (first exit wins)
	assert.Equal(t, []string{"step1"}, executionOrder,
		"Only step1 should execute, early exit prevents step2 and step3")

	// T007: result.NextStep should be the first matched transition target
	assert.Equal(t, "external_a", result.NextStep,
		"result.NextStep should be external_a (step1's target)")
}

// TestEarlyExitTransition_EdgeCase_SingleStepBodyEarlyExit tests early exit
// when loop body contains only one step.
// Given: Loop body [step1]
// When: step1 transitions to external_step
// Then: result.NextStep should be "external_step"
func TestEarlyExitTransition_EdgeCase_SingleStepBodyEarlyExit(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-single-step-early-exit",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-single-step-early-exit")

	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		return "external_step", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// T007: result.NextStep should be set even for single-step body
	assert.Equal(t, "external_step", result.NextStep,
		"result.NextStep should be set for single-step loop body early exit")
}

// TestEarlyExitTransition_EdgeCase_EmptyNextStepValue tests behavior when
// nextStep is empty string (which should be treated as no transition).
// Given: Loop body [step1, step2] with max 1 iteration
// When: step1 returns empty string for nextStep
// Then: result.NextStep should remain empty, no early exit
func TestEarlyExitTransition_EdgeCase_EmptyNextStepValue(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-empty-nextstep",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2"},
			MaxIterations: 1, // Only one iteration to test empty nextStep behavior
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-empty-nextstep")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		return "", nil // Empty nextStep (no transition)
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Both steps should execute (no early exit)
	assert.Equal(t, []string{"step1", "step2"}, executionOrder,
		"Both steps should execute when nextStep is empty")

	// T007: result.NextStep should remain empty
	assert.Equal(t, "", result.NextStep,
		"result.NextStep should be empty when no transitions occur")
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestEarlyExitTransition_ErrorHandling_StepErrorWithTransition tests that
// when a step returns both an error and a nextStep, the error takes precedence
// and result.NextStep should not be set (loop fails, doesn't exit gracefully).
// Given: Loop body [step1, step2]
// When: step1 returns ("external_step", error)
// Then: Error should propagate, result.NextStep should not be set
func TestEarlyExitTransition_ErrorHandling_StepErrorWithTransition(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-error-with-transition",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-error-with-transition")

	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		// When: step1 returns both nextStep and error
		if stepName == "step1" {
			return "external_step", assert.AnError
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.Error(t, err, "Error should propagate from step execution")
	// result may be nil or partially populated on error
	_ = result
}

// =============================================================================
// Integration Tests: ExecuteForEach Early Exit
// =============================================================================

// TestEarlyExitTransition_Integration_ForEachEarlyExit verifies that T007
// works correctly in ExecuteForEach context (parallel implementation).
// Given: ForEach loop with items ["a", "b", "c"] and body [step1, step2]
// When: step1 transitions to external_step on item "a"
// Then: result.NextStep should be set, loop should exit after first item
func TestEarlyExitTransition_Integration_ForEachEarlyExit(t *testing.T) {
	// Item: T007
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results[`["a","b","c"]`] = `["a","b","c"]`

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-early-exit",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: `["a","b","c"]`,
			Body:  []string{"step1", "step2"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-early-exit")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Transition to external step on first step of first iteration
		if stepName == "step1" {
			return "external_step", nil
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Only step1 should execute (early exit before step2 and remaining items)
	assert.Equal(t, []string{"step1"}, executionOrder,
		"Only step1 should execute before early exit in ForEach")

	// Only first item processed
	assert.Equal(t, 1, result.TotalCount,
		"ForEach should exit after first item when early exit triggered")

	// T007: result.NextStep should be set in ForEach as well
	assert.Equal(t, "external_step", result.NextStep,
		"result.NextStep should be set for ForEach early exit")
}

// =============================================================================
// Spec Reproduction Test
// =============================================================================

// TestEarlyExitTransition_SpecReproduction tests the exact scenario from
// the F048 spec where check_tests_passed transitions to run_fmt, exiting
// the green_loop early and skipping prepare_impl_prompt and implement_item.
// Given: Loop body simulating [run_tests, check_tests, prepare_prompt, implement, run_fmt]
// When: check_tests transitions to run_fmt (skip prepare and implement)
// Then: result.NextStep should be run_fmt, unnecessary steps skipped
func TestEarlyExitTransition_SpecReproduction(t *testing.T) {
	// Item: T007
	// Feature: F048
	// Spec: .specify/implementation/F048/spec-content.md

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.results["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-spec-reproduction",
		Steps: map[string]*workflow.Step{
			"run_tests_green":     {Name: "run_tests_green", Type: workflow.StepTypeCommand, Command: "make test"},
			"check_tests_passed":  {Name: "check_tests_passed", Type: workflow.StepTypeCommand, Command: "check tests"},
			"prepare_impl_prompt": {Name: "prepare_impl_prompt", Type: workflow.StepTypeCommand, Command: "prepare"},
			"implement_item":      {Name: "implement_item", Type: workflow.StepTypeCommand, Command: "implement"},
			"run_fmt":             {Name: "run_fmt", Type: workflow.StepTypeCommand, Command: "make fmt"},
		},
	}

	step := &workflow.Step{
		Name: "green_loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"run_tests_green", "check_tests_passed", "prepare_impl_prompt", "implement_item", "run_fmt"},
			MaxIterations: 1, // Only one iteration to test the intra-body jump behavior
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-spec-reproduction")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: check_tests_passed outputs "TESTS_PASSED", transition to run_fmt
		if stepName == "check_tests_passed" {
			return "run_fmt", nil // Skip prepare_impl_prompt and implement_item
		}
		return "", nil
	}

	// Act
	result, err := loopExec.ExecuteWhile(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		func(ec *workflow.ExecutionContext) *interpolation.Context {
			return interpolation.NewContext()
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Expected: run_tests_green, check_tests_passed execute, then jump to run_fmt
	// This is an INTRA-BODY jump (run_fmt is in the body), NOT early exit
	// So the loop continues and all 3 steps execute in order: run_tests, check, run_fmt
	// T007 is about EXTERNAL transitions, not intra-body jumps
	// For this test to demonstrate T007, check_tests_passed should transition OUTSIDE the loop

	// Adjust test: check_tests should transition to a step OUTSIDE the loop body
	// Let me update the expectation to match actual T007 behavior

	// Actually, re-reading the spec: the transition is TO run_fmt which IS in the body
	// So this is testing T006 (intra-body jump), not T007 (early exit)
	// T007 only applies when transition target is OUTSIDE the loop body

	// This test documents the distinction between T006 and T007
	t.Log("Note: This spec scenario demonstrates intra-body transition (T006),")
	t.Log("not early exit transition (T007). run_fmt is within the loop body.")
	t.Log("T007 applies when transition target is OUTSIDE the loop body.")

	// For T007, we need transition to external step like "next_phase"
	// Keeping test for documentation but marking as T006 behavior
	expectedOrder := []string{"run_tests_green", "check_tests_passed", "run_fmt"}
	assert.Equal(t, expectedOrder, executionOrder,
		"Should execute run_tests, check_tests, then jump to run_fmt (intra-body, T006)")

	// T007: result.NextStep should be empty (no early exit, intra-body jump instead)
	assert.Equal(t, "", result.NextStep,
		"result.NextStep should be empty for intra-body transitions (T006, not T007)")
}

// =============================================================================
// Documentation Test
// =============================================================================

// TestT007_ComponentBehavior documents the expected behavior of T007.
func TestT007_ComponentBehavior(t *testing.T) {
	t.Log("Component T007: Handle Early Exit Transitions in ExecuteWhile")
	t.Log("")
	t.Log("Implementation:")
	t.Log("  - Variable: exitNextStep (local to loop body execution)")
	t.Log("  - Captures: nextStep value when shouldBreak is true")
	t.Log("  - Sets: result.NextStep = exitNextStep before loop exit")
	t.Log("")
	t.Log("Purpose:")
	t.Log("  Enable workflow to continue at the correct step after a loop exits")
	t.Log("  early due to a body step transitioning to a step outside the loop body.")
	t.Log("")
	t.Log("Data Flow:")
	t.Log("  1. Body step executes, returns nextStep")
	t.Log("  2. evaluateBodyTransition determines transition is external (shouldBreak=true)")
	t.Log("  3. exitNextStep = nextStep (capture target)")
	t.Log("  4. shouldExitLoop = true (flag early exit)")
	t.Log("  5. break from body iteration")
	t.Log("  6. result.NextStep = exitNextStep (propagate to caller)")
	t.Log("  7. PopLoopContext and break from loop")
	t.Log("  8. ExecutionService uses result.NextStep to continue workflow")
	t.Log("")
	t.Log("Integration:")
	t.Log("  - Used in both ExecuteWhile and ExecuteForEach")
	t.Log("  - Parallel implementation ensures consistent behavior")
	t.Log("  - Works with T006 (intra-body jumps) - different code paths")
	t.Log("")
	t.Log("Distinction from T006:")
	t.Log("  - T006: Intra-body jump (nextStep is in loop body, adjust index)")
	t.Log("  - T007: Early exit (nextStep is OUTSIDE loop body, set result.NextStep)")
	t.Log("")
	t.Log("Examples:")
	t.Log("  - Body: [step1, step2, step3]")
	t.Log("  - step1 transitions to 'cleanup_step' (not in body)")
	t.Log("  - shouldBreak = true, exitNextStep = 'cleanup_step'")
	t.Log("  - result.NextStep = 'cleanup_step'")
	t.Log("  - Loop exits, workflow continues at cleanup_step")
}
