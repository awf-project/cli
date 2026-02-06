package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// ForEach Loop Transition Tests
// Component: T014 (extracted from T008)
// Feature: F048 - ForEach Loop Transitions Support
// Source: loop_executor_transitions_test.go
// =============================================================================
//
// This file contains 19 tests extracted for C014-T014 to split monolithic test files.
// Tests verify ForEach loop transition behavior with feature parity to While loops.
//
// Test Organization:
// 1. Basic Transition Support (1 test - placeholder)
// 2. Happy Path Tests (5 tests - intra-body and early exit)
// 3. Edge Case Tests (4 tests - boundaries and limits)
// 4. Error Handling Tests (2 tests - invalid targets and errors)
// 5. Integration Tests (2 tests - retry pattern and complex scenarios)
// 6. Skip Steps Tests (5 tests - F048-T010 scenarios)
//
// Git History: Maintains traceability per ADR-004
// =============================================================================

// =============================================================================
// Happy Path Tests: Intra-Body Skip Forward/Backward and Early Exit
// =============================================================================

// TestForEachTransition_HappyPath_IntraBodySkipForward tests intra-body skip in foreach.
// Given: ForEach loop with items [a, b] and body [step1, step2, step3]
// When: step1 transitions to step3 (skip step2)
func TestForEachTransition_HappyPath_IntraBodySkipForward(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a,b"] = "a,b" // Return two items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-skip-forward",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a,b",
			Body:  []string{"step1", "step2", "step3"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-skip-forward")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to step3 (skip step2)
		if stepName == "step1" {
			return "step3", nil
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

	// Then: Should have 2 iterations (items a and b)
	assert.Equal(t, 2, result.TotalCount, "should execute 2 iterations")
	assert.Len(t, result.Iterations, 2)

	// Then: In each iteration, step2 should be skipped
	// Iteration 1: step1, step3 (step2 skipped)
	// Iteration 2: step1, step3 (step2 skipped)
	expectedOrder := []string{"step1", "step3", "step1", "step3"}
	assert.Equal(t, expectedOrder, executionOrder, "step2 should be skipped in both iterations")

	// Then: result.NextStep should be empty (no early exit)
	assert.Empty(t, result.NextStep, "no early exit should occur")
}

// TestForEachTransition_HappyPath_IntraBodySkipToEnd tests transitioning to
// the last step in a foreach loop body.
// Given: ForEach loop with items [a] and body [step1, step2, step3, step4]
// When: step1 transitions to step4 (skips step2, step3)
// Then: step2 and step3 should not execute, only step1 and step4
func TestForEachTransition_HappyPath_IntraBodySkipToEnd(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a"] = "a" // Return single item
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-skip-to-end",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a",
			Body:  []string{"step1", "step2", "step3", "step4"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-skip-to-end")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to step4 (skip step2, step3)
		if stepName == "step1" {
			return "step4", nil
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

	// Then: Should execute only step1 and step4 (step2, step3 skipped)
	expectedOrder := []string{"step1", "step4"}
	assert.Equal(t, expectedOrder, executionOrder, "step2 and step3 should be skipped")
	assert.Equal(t, 1, result.TotalCount, "should have 1 iteration")
}

// TestForEachTransition_HappyPath_EarlyExitFromFirstStep tests early loop exit
// when the first step in a foreach loop body transitions to an external step.
// Given: ForEach loop with items [a, b, c] and body [step1, step2, step3]
// When: step1 transitions to external_step (not in body)
// Then: Loop should exit immediately, only 1 iteration, result.NextStep set
func TestForEachTransition_HappyPath_EarlyExitFromFirstStep(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a,b,c"] = "a,b,c" // Return three items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-early-exit",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a,b,c",
			Body:  []string{"step1", "step2", "step3"},
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
		// When: step1 transitions to external_step (early exit)
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

	// Then: Should exit after first iteration (only "a" processed)
	assert.Equal(t, 1, result.TotalCount, "should exit after first iteration")
	assert.Len(t, result.Iterations, 1)

	// Then: Should only execute step1 (step2, step3 not executed)
	expectedOrder := []string{"step1"}
	assert.Equal(t, expectedOrder, executionOrder, "only step1 should execute")

	// Then: result.NextStep should be set to external_step
	assert.Equal(t, "external_step", result.NextStep, "result.NextStep should be external_step")
}

// TestForEachTransition_HappyPath_EarlyExitFromMiddleStep tests early exit
// when a middle step in the foreach loop body triggers an external transition.
// Given: ForEach loop with items [x, y] and body [step1, step2, step3]
// When: step2 transitions to external_step
// Then: step1 and step2 execute, step3 skipped, early exit on first iteration
func TestForEachTransition_HappyPath_EarlyExitFromMiddleStep(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["x,y"] = "x,y" // Return two items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-exit-middle",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "x,y",
			Body:  []string{"step1", "step2", "step3"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-exit-middle")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step2 transitions to external_step
		if stepName == "step2" {
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

	// Then: Should exit after first iteration
	assert.Equal(t, 1, result.TotalCount, "should exit after first iteration")

	// Then: step1 and step2 execute, step3 skipped
	expectedOrder := []string{"step1", "step2"}
	assert.Equal(t, expectedOrder, executionOrder, "step3 should not execute")

	// Then: result.NextStep should be external_step
	assert.Equal(t, "external_step", result.NextStep, "result.NextStep should be external_step")
}

// TestForEachTransition_HappyPath_NoTransition tests normal sequential execution
// when no transitions are triggered in a foreach loop.
// Given: ForEach loop with items [a, b] and body [step1, step2, step3]
// When: No step triggers a transition (all return empty nextStep)
// Then: All steps execute in all iterations sequentially
func TestForEachTransition_HappyPath_NoTransition(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a,b"] = "a,b" // Return two items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-no-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a,b",
			Body:  []string{"step1", "step2", "step3"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-no-transition")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: No transitions (all return empty)
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

	// Then: Should execute all steps in both iterations
	assert.Equal(t, 2, result.TotalCount, "should have 2 iterations")
	expectedOrder := []string{"step1", "step2", "step3", "step1", "step2", "step3"}
	assert.Equal(t, expectedOrder, executionOrder, "all steps should execute sequentially")

	// Then: No early exit
	assert.Empty(t, result.NextStep, "no early exit should occur")
}

// =============================================================================
// Edge Case Tests: Boundary Conditions
// =============================================================================

// =============================================================================
// Edge Case Tests: Boundary Conditions
// =============================================================================

// TestForEachTransition_EdgeCase_SingleStepBody tests transition behavior
// when the loop body contains only a single step.
// Given: ForEach loop with items [a] and body [step1]
// When: step1 transitions to external_step
// Then: Loop exits immediately, result.NextStep set
func TestForEachTransition_EdgeCase_SingleStepBody(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a"] = "a" // Single item
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-single-step",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a",
			Body:  []string{"step1"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-single-step")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: Only step transitions to external
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

	// Then: Should exit after step1 executes
	assert.Equal(t, 1, result.TotalCount, "should have 1 iteration")
	assert.Equal(t, []string{"step1"}, executionOrder, "only step1 should execute")
	assert.Equal(t, "external_step", result.NextStep, "result.NextStep should be external_step")
}

// TestForEachTransition_EdgeCase_EmptyItems tests behavior when items list is empty.
// Given: ForEach loop with empty items and body [step1, step2]
// When: Items evaluates to empty array
// Then: No iterations execute, no error
func TestForEachTransition_EdgeCase_EmptyItems(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["[]"] = "[]" // Empty JSON array
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-empty-items",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "[]",
			Body:  []string{"step1", "step2"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-empty-items")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
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

	// Then: No iterations, no steps executed
	assert.Equal(t, 0, result.TotalCount, "should have 0 iterations")
	assert.Empty(t, executionOrder, "no steps should execute")
	assert.Empty(t, result.NextStep, "no nextStep should be set")
}

// TestForEachTransition_EdgeCase_MaxIterations tests that max_iterations limits
// both items and transitions work correctly with the limit.
// Given: ForEach loop with items [a, b, c, d] and max_iterations=2
// When: step1 transitions to step3 in each iteration
// Then: Only first 2 items processed, transitions work in both
func TestForEachTransition_EdgeCase_MaxIterations(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a,b,c,d"] = "a,b,c,d" // Four items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-max-iterations",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         "a,b,c,d",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 2, // Limit to 2 items
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-max-iterations")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to step3 (skip step2)
		if stepName == "step1" {
			return "step3", nil
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

	// Then: Should process only 2 items (max_iterations limit)
	assert.Equal(t, 2, result.TotalCount, "should have 2 iterations (max_iterations)")

	// Then: Transitions should work in both iterations
	expectedOrder := []string{"step1", "step3", "step1", "step3"}
	assert.Equal(t, expectedOrder, executionOrder, "transitions should work with max_iterations")
}

// TestForEachTransition_EdgeCase_EarlyExitOnSecondIteration tests early exit
// when transition occurs in the second iteration, not the first.
// Given: ForEach loop with items [a, b, c] and body [step1, step2]
// When: step1 transitions to external only on second iteration (item b)
// Then: First iteration completes normally, second iteration exits early
func TestForEachTransition_EdgeCase_EarlyExitOnSecondIteration(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a,b,c"] = "a,b,c" // Three items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-exit-second-iter",
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
			Items: "a,b,c",
			Body:  []string{"step1", "step2"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-exit-second-iter")

	executionOrder := []string{}
	iterationCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Track iteration by counting step1 calls
		if stepName == "step1" {
			iterationCount++
			// When: Second iteration (item b) transitions to external
			if iterationCount == 2 {
				return "external_step", nil
			}
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

	// Then: Should have 2 iterations (first completes, second exits early)
	assert.Equal(t, 2, result.TotalCount, "should have 2 iterations")

	// Then: First iteration: step1, step2; Second iteration: step1 (exits)
	expectedOrder := []string{"step1", "step2", "step1"}
	assert.Equal(t, expectedOrder, executionOrder, "second iteration should exit after step1")

	// Then: result.NextStep should be external_step
	assert.Equal(t, "external_step", result.NextStep, "result.NextStep should be external_step")
}

// =============================================================================
// Error Handling Tests: Invalid Transitions
// =============================================================================

// =============================================================================
// Error Handling Tests: Invalid Transitions
// =============================================================================

// TestForEachTransition_ErrorHandling_InvalidTarget tests graceful degradation
// when a transition targets a non-existent step (not in body, not in workflow).
// Given: ForEach loop with items [a] and body [step1, step2]
// When: step1 transitions to "nonexistent_step"
// Then: Warning logged, loop exits early (treats as external), result.NextStep set
func TestForEachTransition_ErrorHandling_InvalidTarget(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a"] = "a" // Single item
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-invalid-target",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a",
			Body:  []string{"step1", "step2"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-invalid-target")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to non-existent step
		if stepName == "step1" {
			return "nonexistent_step", nil
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

	// Per ADR-005: Graceful degradation - invalid targets continue sequential execution
	// F048 T011: Invalid transition targets log warning and continue execution
	assert.Equal(t, 1, result.TotalCount, "should have 1 iteration")
	assert.Equal(t, []string{"step1", "step2"}, executionOrder, "all steps should execute sequentially")

	// Then: result.NextStep should be empty (loop completes normally)
	assert.Empty(t, result.NextStep, "result.NextStep should be empty after normal loop completion")

	// Then: Warning should be logged
	assert.Greater(t, len(logger.warnings), 0, "should log warning for invalid target")
}

// TestForEachTransition_ErrorHandling_StepError tests that step execution errors
// combined with transitions are handled correctly.
// Given: ForEach loop with items [a] and body [step1, step2]
// When: step1 returns error AND transition
// Then: Error is propagated, loop exits, transition is not processed
func TestForEachTransition_ErrorHandling_StepError(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a"] = "a" // Single item
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-step-error",
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
			Items: "a",
			Body:  []string{"step1", "step2"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-step-error")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 returns error with transition
		if stepName == "step1" {
			return "external_step", assert.AnError
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
	require.Error(t, err, "should return error from step execution")
	require.NotNil(t, result)

	// Then: Should exit after error
	assert.Equal(t, 1, result.TotalCount, "should have 1 iteration")
	assert.Equal(t, []string{"step1"}, executionOrder, "only step1 should execute")

	// Then: Error takes precedence, no nextStep propagated
	assert.Empty(t, result.NextStep, "error should prevent transition processing")
}

// =============================================================================
// Integration Tests: Complex Scenarios
// =============================================================================

// =============================================================================
// Integration Tests: Complex Scenarios
// =============================================================================

// TestForEachTransition_Integration_RetryPattern tests ADR-004 retry pattern:
// when a body step transitions back to the loop step itself.
// Given: ForEach loop with items [a] and body [step1, step2]
// When: step1 transitions to "loop" (the loop step itself)
// Then: Transition is ignored (retry pattern), execution continues sequentially
func TestForEachTransition_Integration_RetryPattern(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a"] = "a" // Single item
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-retry-pattern",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
		},
	}

	step := &workflow.Step{
		Name: "loop", // Loop step name
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a",
			Body:  []string{"step1", "step2"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-retry-pattern")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to "loop" (retry pattern)
		if stepName == "step1" {
			return "loop", nil
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

	// Then: Retry pattern should be ignored, sequential execution continues
	assert.Equal(t, 1, result.TotalCount, "should have 1 iteration")
	expectedOrder := []string{"step1", "step2"}
	assert.Equal(t, expectedOrder, executionOrder, "should execute all steps sequentially (retry ignored)")

	// Then: No early exit
	assert.Empty(t, result.NextStep, "retry pattern should not cause early exit")
}

// TestForEachTransition_Integration_MultipleTransitions tests scenario where
// different items trigger different transition behaviors.
// Given: ForEach loop with items [a, b, c] and body [step1, step2, step3]
// When: Item a: no transition, Item b: skip to step3, Item c: early exit
// Then: Verify each iteration behaves correctly according to transition
func TestForEachTransition_Integration_MultipleTransitions(t *testing.T) {
	// Item: T008
	// Feature: F048

	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	resolver.results["a,b,c"] = "a,b,c" // Three items
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-multiple-transitions",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "a,b,c",
			Body:  []string{"step1", "step2", "step3"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-multiple-transitions")

	executionOrder := []string{}
	currentItem := ""
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// Track current item from loop context
		if intCtx.Loop != nil && intCtx.Loop.Item != nil {
			currentItem = intCtx.Loop.Item.(string)
		}

		// When: Different transition for each item
		if stepName == "step1" {
			switch currentItem {
			case "a":
				// Item a: no transition (sequential)
				return "", nil
			case "b":
				// Item b: skip to step3
				return "step3", nil
			case "c":
				// Item c: early exit
				return "external_step", nil
			}
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
			ctx := interpolation.NewContext()
			if ec.CurrentLoop != nil {
				ctx.Loop = &interpolation.LoopData{
					Item:   ec.CurrentLoop.Item,
					Index:  ec.CurrentLoop.Index,
					First:  ec.CurrentLoop.First,
					Last:   ec.CurrentLoop.Last,
					Length: ec.CurrentLoop.Length,
				}
			}
			return ctx
		},
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Then: Should have 3 iterations (exits on third)
	assert.Equal(t, 3, result.TotalCount, "should have 3 iterations")

	// Then: Verify execution order
	// Item a: step1, step2, step3 (sequential)
	// Item b: step1, step3 (skip step2)
	// Item c: step1 (early exit)
	expectedOrder := []string{
		"step1", "step2", "step3", // Item a
		"step1", "step3", // Item b
		"step1", // Item c
	}
	assert.Equal(t, expectedOrder, executionOrder, "execution order should match transition logic")

	// Then: Early exit from item c
	assert.Equal(t, "external_step", result.NextStep, "result.NextStep should be external_step")
}

// =============================================================================
// F048-T010: Loop Body Transition - Skip Steps Scenario Tests (ForEach)
// =============================================================================
//
// These tests verify that transitions within foreach loop bodies correctly skip
// intermediate steps when a transition is triggered.
// =============================================================================

// TestLoopExecutor_ExecuteForEach_SkipSingleStep tests skip behavior
// in a foreach loop context.
//
// Given: A foreach loop with items [a, b] and body [step1, step2, step3]
//
//	step1 has transition: goto step3
//
// When:  Loop processes both items
// Then:  For each item: step2 is skipped, execution is [step1, step3]
func TestLoopExecutor_ExecuteForEach_SkipSingleStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-skip-single",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b"]`,
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-skip-single")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// step1 transitions to step3 (skip step2)
		if stepName == "step1" {
			return "step3", nil
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

	// Expected: For each of 2 items: [step1, step3] (step2 skipped)
	expected := []string{"step1", "step3", "step1", "step3"}
	assert.Equal(t, expected, executionOrder,
		"step2 should be skipped for both iterations")
	assert.Equal(t, 2, result.TotalCount, "should process 2 items")
}

// TestLoopExecutor_ExecuteForEach_SkipMultipleSteps tests skipping
// multiple steps in a foreach loop.
//
// Given: A foreach loop with items [1, 2, 3] and body [validate, transform, save]
//
//	validate has transition: when valid, goto save
//
// When:  All items are valid
// Then:  transform is skipped for all iterations
func TestLoopExecutor_ExecuteForEach_SkipMultipleSteps(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-skip-multiple",
		Steps: map[string]*workflow.Step{
			"validate":  {Name: "validate", Type: workflow.StepTypeCommand, Command: "validate"},
			"transform": {Name: "transform", Type: workflow.StepTypeCommand, Command: "transform"},
			"save":      {Name: "save", Type: workflow.StepTypeCommand, Command: "save"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `[1, 2, 3]`,
			Body:          []string{"validate", "transform", "save"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-skip-multiple")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// validate transitions to save (skip transform)
		if stepName == "validate" {
			return "save", nil
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

	// Expected: For each of 3 items: [validate, save] (transform skipped)
	expected := []string{"validate", "save", "validate", "save", "validate", "save"}
	assert.Equal(t, expected, executionOrder,
		"transform should be skipped for all 3 iterations")
	assert.Equal(t, 3, result.TotalCount, "should process 3 items")
}

// TestLoopExecutor_ExecuteForEach_SkipPreservesItem tests that the current
// loop item is preserved when steps are skipped.
//
// Given: A foreach loop with items ["apple", "banana"] and body [check, use, log]
//
//	check has transition: goto log
//
// When:  Loop processes items
// Then:  log step receives correct item even though use was skipped
func TestLoopExecutor_ExecuteForEach_SkipPreservesItem(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-preserve-item",
		Steps: map[string]*workflow.Step{
			"check": {Name: "check", Type: workflow.StepTypeCommand, Command: "check"},
			"use":   {Name: "use", Type: workflow.StepTypeCommand, Command: "use"},
			"log":   {Name: "log", Type: workflow.StepTypeCommand, Command: "log"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["apple", "banana"]`,
			Body:          []string{"check", "use", "log"},
			MaxIterations: 100,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-preserve-item")

	// Track items at log step
	loggedItems := []interface{}{}

	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		if stepName == "log" && intCtx.Loop != nil {
			loggedItems = append(loggedItems, intCtx.Loop.Item)
		}

		// check transitions to log (skip use)
		if stepName == "check" {
			return "log", nil
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
	assert.Equal(t, 2, result.TotalCount, "should process 2 items")

	// Verify items are preserved correctly
	require.Len(t, loggedItems, 2, "should have logged 2 items")
	assert.Equal(t, "apple", loggedItems[0], "first item should be 'apple'")
	assert.Equal(t, "banana", loggedItems[1], "second item should be 'banana'")
}

// TestLoopExecutor_ExecuteForEach_EarlyExit tests that a transition to a step
// outside the loop body causes the foreach loop to exit immediately.
//
// Given: A foreach loop with body [step1, step2, step3], items [a, b, c]
//
//	step1 has transition: goto external_step (not in body)
//
// When:  Loop executes first iteration
// Then:  step1 executes for first item, loop breaks, remaining items not processed
func TestLoopExecutor_ExecuteForEach_EarlyExit(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-early-exit",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a", "b", "c"]`,
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-early-exit")

	// Track execution order
	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// step1 transitions to external_step (outside loop body)
		if stepName == "step1" {
			return "external_step", nil
		}
		return "", nil
	}

	contextBuilder := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		contextBuilder,
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "external_step", result.NextStep, "should return early exit target")
	assert.Equal(t, []string{"step1"}, executionOrder, "should only execute step1 for first item before early exit")
}

// TestLoopExecutor_ExecuteForEach_InvalidTarget tests that a transition to a
// non-existent step logs a warning and continues sequential execution in foreach.
//
// Given: A foreach loop with body [step1, step2, step3], items [a]
//
//	step1 has transition: goto non_existent_step
//
// When:  Loop executes
// Then:  Warning logged, step1, step2, step3 execute sequentially for item
func TestLoopExecutor_ExecuteForEach_InvalidTarget(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-invalid-target",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeForEach,
			Items:         `["a"]`,
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-invalid-target")

	// Track execution order
	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// step1 transitions to non-existent step
		if stepName == "step1" {
			return "non_existent_step", nil
		}
		return "", nil
	}

	contextBuilder := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := loopExec.ExecuteForEach(
		context.Background(),
		wf,
		step,
		execCtx,
		stepExecutor,
		contextBuilder,
	)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.NextStep, "should return empty NextStep when loop completes normally")
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder, "should execute all steps sequentially despite invalid target")

	// Verify warning was logged (check substring in any warning)
	foundWarning := false
	for _, w := range logger.warnings {
		if strings.Contains(w, "transition target") {
			foundWarning = true
			break
		}
	}
	assert.True(t, foundWarning, "should log warning about invalid target")
}
