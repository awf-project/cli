package application_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// Source: loop_executor_transitions_test.go | Tests: 59 | Component: C014-T010
// =============================================================================
//
// Intra-body transition tests for F048 (While Loop Transitions Support).
// Tests body step index mapping, transition evaluation, and skip-step functionality.
//
// Coverage:
// - TestBuildBodyStepIndices_* (12 tests)
// - TestEvaluateBodyTransition_* (excluding EarlyExit, 19 tests)
// - TestHandleIntraBodyJump_* (17 tests)
// - TestExecuteWhile/ForEach_*Skip* tests (11 tests)
//
// =============================================================================

func TestBuildBodyStepIndices_HappyPath_SimpleSequence(t *testing.T) {
	// Arrange: Create loop executor
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	body := []string{"step1", "step2", "step3"}

	// Expected behavior after GREEN phase:
	expectedIndices := map[string]int{
		"step1": 0,
		"step2": 1,
		"step3": 2,
	}

	// Act: Call BuildBodyStepIndices (now exported for testing)
	indices, err := loopExec.BuildBodyStepIndices(body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedIndices, indices)
}

// TestBuildBodyStepIndices_HappyPath_SingleStep verifies mapping
// for a loop body with only one step.
func TestBuildBodyStepIndices_HappyPath_SingleStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	body := []string{"only_step"}

	// Expected mapping
	expectedIndices := map[string]int{
		"only_step": 0,
	}

	// Act: Call BuildBodyStepIndices
	indices, err := loopExec.BuildBodyStepIndices(body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedIndices, indices)
}

// TestBuildBodyStepIndices_HappyPath_ManySteps verifies mapping
// works correctly with larger body (boundary condition).
func TestBuildBodyStepIndices_HappyPath_ManySteps(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Create body with 10 steps
	body := []string{
		"step_0", "step_1", "step_2", "step_3", "step_4",
		"step_5", "step_6", "step_7", "step_8", "step_9",
	}

	// Expected: 0-indexed mapping
	expectedIndices := map[string]int{
		"step_0": 0,
		"step_1": 1,
		"step_2": 2,
		"step_3": 3,
		"step_4": 4,
		"step_5": 5,
		"step_6": 6,
		"step_7": 7,
		"step_8": 8,
		"step_9": 9,
	}

	// Act: Call BuildBodyStepIndices
	indices, err := loopExec.BuildBodyStepIndices(body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedIndices, indices)
}

// TestBuildBodyStepIndices_EdgeCase_EmptyBody verifies behavior
// when loop body is empty (should return empty map).
func TestBuildBodyStepIndices_EdgeCase_EmptyBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	body := []string{}

	// Expected: Empty map
	expectedIndices := map[string]int{}

	// Act: Call BuildBodyStepIndices
	indices, err := loopExec.BuildBodyStepIndices(body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedIndices, indices)
}

// TestBuildBodyStepIndices_EdgeCase_NilBody verifies graceful handling
// when body is nil (edge case that shouldn't happen in practice).
func TestBuildBodyStepIndices_EdgeCase_NilBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	var body []string // nil slice

	// Expected: Empty map (graceful degradation)
	expectedIndices := map[string]int{}

	// Act: Call BuildBodyStepIndices
	indices, err := loopExec.BuildBodyStepIndices(body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, expectedIndices, indices)
}

// TestBuildBodyStepIndices_EdgeCase_DuplicateStepNames verifies behavior
// when body contains duplicate step names (should return error).
func TestBuildBodyStepIndices_EdgeCase_DuplicateStepNames(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Body with duplicate "step2"
	body := []string{"step1", "step2", "step3", "step2", "step4"}

	// Act: Call BuildBodyStepIndices
	indices, err := loopExec.BuildBodyStepIndices(body)

	// Assert: Should return error for duplicate step names
	require.Error(t, err)
	assert.Nil(t, indices)
	assert.Contains(t, err.Error(), "duplicate step 'step2'")
	assert.Contains(t, err.Error(), "1")
	assert.Contains(t, err.Error(), "3")
}

// =============================================================================
// Integration Tests: buildBodyStepIndices Usage in ExecuteWhile
// =============================================================================

// TestExecuteWhile_BuildsBodyStepIndices_CalledOncePerIteration verifies
// that buildBodyStepIndices is called during ExecuteWhile execution.
// This test validates the stub integration, not the mapping logic itself.
func TestExecuteWhile_BuildsBodyStepIndices_CalledOncePerIteration(t *testing.T) {
	// Arrange: Create minimal while loop
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	// First iteration: true, second: false
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-build-indices",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2"},
			MaxIterations: 5,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-build-indices")

	stepExecutions := 0
	callCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		stepExecutions++
		callCount++
		// Make condition false after first iteration (2 steps)
		if callCount >= 2 {
			evaluator.boolResults["true"] = false
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

	// Assert: Loop executed successfully
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCount, "Should complete one iteration")
	assert.Equal(t, 2, stepExecutions, "Should execute 2 body steps")

	// Note: We cannot verify buildBodyStepIndices was called directly,
	// but we can verify the loop executed (which calls it at line 253)
	// RED phase: buildBodyStepIndices returns empty map (stub)
	// GREEN phase: Will return proper mapping
}

// TestExecuteWhile_BodyStepIndices_NotUsedInStubPhase verifies that
// the current stub implementation compiles and runs even though
// buildBodyStepIndices returns an empty map.
func TestExecuteWhile_BodyStepIndices_NotUsedInStubPhase(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-stub-phase",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1, // Only one iteration
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-stub-phase")

	// Executor returns transition (which stub currently ignores)
	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// step1 transitions to step3 (should skip step2 after GREEN phase)
		if stepName == "step1" {
			return "step3", nil
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

	// RED phase (stub): All steps execute sequentially
	// The buildBodyStepIndices map is created but not used (line 254: _ = bodyStepIndices)
	// assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
	// 	"Stub phase: bodyStepIndices created but not used, all steps execute")

	// GREEN phase (after T005): Should be ["step1", "step3"]
	assert.Equal(t, []string{"step1", "step3"}, executionOrder,
		"GREEN phase: bodyStepIndices map is used by T005 transition logic")
}

// =============================================================================
// Error Handling and Boundary Conditions
// =============================================================================

// TestExecuteWhile_BodyStepIndices_WithComplexStepNames verifies mapping
// works with various step name formats (underscores, dashes, numbers).
func TestExecuteWhile_BodyStepIndices_WithComplexStepNames(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-complex-names",
		Steps: map[string]*workflow.Step{
			"step-1":         {Name: "step-1", Type: workflow.StepTypeCommand},
			"step_2":         {Name: "step_2", Type: workflow.StepTypeCommand},
			"step3":          {Name: "step3", Type: workflow.StepTypeCommand},
			"UPPERCASE_STEP": {Name: "UPPERCASE_STEP", Type: workflow.StepTypeCommand},
			"step.with.dots": {Name: "step.with.dots", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:      workflow.LoopTypeWhile,
			Condition: "true",
			Body: []string{
				"step-1",
				"step_2",
				"step3",
				"UPPERCASE_STEP",
				"step.with.dots",
			},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-complex-names")

	executionCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount++
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

	// Assert: Loop executes successfully with complex names
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 5, executionCount, "Should execute all 5 steps with complex names")

	// Expected indices after GREEN phase:
	// "step-1": 0, "step_2": 1, "step3": 2, "UPPERCASE_STEP": 3, "step.with.dots": 4
}

// TestExecuteWhile_BodyStepIndices_WithSpecialCharacters verifies mapping
// handles edge case step names with special characters.
func TestExecuteWhile_BodyStepIndices_WithSpecialCharacters(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Note: YAML spec may restrict step names, but test the mapper's robustness
	wf := &workflow.Workflow{
		Name: "test-special-chars",
		Steps: map[string]*workflow.Step{
			"step@1": {Name: "step@1", Type: workflow.StepTypeCommand},
			"step#2": {Name: "step#2", Type: workflow.StepTypeCommand},
			"step$3": {Name: "step$3", Type: workflow.StepTypeCommand},
			"step%4": {Name: "step%4", Type: workflow.StepTypeCommand},
			"step&5": {Name: "step&5", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step@1", "step#2", "step$3", "step%4", "step&5"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-special-chars")

	executionCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount++
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
	assert.Equal(t, 5, executionCount, "Should handle special characters in step names")
}

// TestExecuteWhile_BodyStepIndices_PreservesOrderAcrossIterations verifies
// that the index mapping is consistent across multiple loop iterations.
func TestExecuteWhile_BodyStepIndices_PreservesOrderAcrossIterations(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	// True for first 2 iterations
	evaluator.boolResults["loop.index < 2"] = true

	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-order-preservation",
		Steps: map[string]*workflow.Step{
			"first":  {Name: "first", Type: workflow.StepTypeCommand},
			"second": {Name: "second", Type: workflow.StepTypeCommand},
			"third":  {Name: "third", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 2",
			Body:          []string{"first", "second", "third"},
			MaxIterations: 5,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-order-preservation")

	executionOrder := []string{}
	stepCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		stepCount++
		// After 2 iterations (6 steps), make condition false
		if stepCount >= 6 {
			evaluator.boolResults["loop.index < 2"] = false
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

	// Assert: Order should be consistent across iterations
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.TotalCount, "Should complete 2 iterations")

	// Expected: first,second,third repeated twice
	expected := []string{
		"first", "second", "third",
		"first", "second", "third",
	}
	assert.Equal(t, expected, executionOrder,
		"Step order should be consistent across iterations")

	// After GREEN phase: indices should be {first: 0, second: 1, third: 2} in both iterations
}

// =============================================================================
// Documentation Tests
// =============================================================================

// TestBuildBodyStepIndices_ExpectedBehavior documents the expected behavior
// of buildBodyStepIndices for future implementers.
func TestBuildBodyStepIndices_ExpectedBehavior(t *testing.T) {
	t.Log("Expected buildBodyStepIndices behavior:")
	t.Log("  Input:  []string{\"step1\", \"step2\", \"step3\"}")
	t.Log("  Output: map[string]int{\"step1\": 0, \"step2\": 1, \"step3\": 2}")
	t.Log("")
	t.Log("  Input:  []string{}")
	t.Log("  Output: map[string]int{}")
	t.Log("")
	t.Log("  Input:  []string{\"step\", \"step\", \"other\"}")
	t.Log("  Output: map[string]int{\"step\": 1, \"other\": 2} // Last occurrence wins")
	t.Log("")
	t.Log("RED Phase: buildBodyStepIndices returns empty map (stub)")
	t.Log("GREEN Phase (T005): Will implement actual index mapping logic")
	t.Log("This enables T005 to use the map for transition jumps within loop body")
}

// =============================================================================
// RED Phase Tests - These SHOULD FAIL until GREEN implementation
// =============================================================================

// TestT004_RED_ExecuteWhile_TransitionWithinBody_ShouldSkipSteps is a RED phase test
// that documents the expected behavior after buildBodyStepIndices is properly implemented.
// Current stub returns empty map, so transitions won't work.
// After GREEN phase (T005), this test should PASS when transition logic uses the index map.
func TestT004_RED_ExecuteWhile_TransitionWithinBody_ShouldSkipSteps(t *testing.T) {
	// Given: A while loop with transitions within the body
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-transition-skip",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-transition-skip")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 returns transition to step3 (should skip step2)
		if stepName == "step1" {
			return "step3", nil
		}
		return "", nil
	}

	// Then: Execute the loop
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

	require.NoError(t, err)
	require.NotNil(t, result)

	// RED Phase Assertion (current behavior - WILL FAIL after GREEN):
	// Stub implementation: buildBodyStepIndices returns empty map
	// T005 hasn't implemented transition logic yet, so all steps execute
	// assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
	// 	"RED phase: Transition ignored, all steps execute sequentially")

	// GREEN Phase Assertion (after T005 implementation - SHOULD PASS):
	// After T005 implements transition logic using bodyStepIndices map:
	assert.Equal(t, []string{"step1", "step3"}, executionOrder,
		"GREEN phase: step2 should be skipped when step1 transitions to step3")

	t.Log("T004 Component Status: buildBodyStepIndices called but map not used yet")
	t.Log("Expected after T005: Transition logic will use the index map to skip steps")
}

// TestT004_RED_ExecuteWhile_MultipleTransitions_ShouldUseIndices verifies
// that buildBodyStepIndices creates correct mapping for complex transition chains.
// RED phase: Map created but not used, all steps execute.
// GREEN phase: Map enables proper index-based jumping.
func TestT004_RED_ExecuteWhile_MultipleTransitions_ShouldUseIndices(t *testing.T) {
	// Given: A loop with multiple potential transitions
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-multi-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand},
			"step5": {Name: "step5", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4", "step5"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-multi-transition")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: Complex transition chain
		// step1 -> step3 (skip step2)
		// step3 -> step5 (skip step4)
		switch stepName {
		case "step1":
			return "step3", nil
		case "step3":
			return "step5", nil
		}
		return "", nil
	}

	// Then: Execute
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

	require.NoError(t, err)
	require.NotNil(t, result)

	// RED phase: Map created but not used, sequential execution
	// assert.Equal(t, []string{"step1", "step2", "step3", "step4", "step5"},
	// 	executionOrder,
	// 	"RED phase: buildBodyStepIndices map created but not used")

	// GREEN phase (after T005):
	assert.Equal(t, []string{"step1", "step3", "step5"}, executionOrder,
		"GREEN phase: Indices should enable jumping step1->step3->step5")

	t.Log("T004: Index map structure created: {step1:0, step2:1, step3:2, step4:3, step5:4}")
	t.Log("T005: Will implement logic to use these indices for transition jumps")
}

// Component T005: Transition Evaluation in Loop Body
// Feature: F048 - While Loop Transitions Support
// =============================================================================

// =============================================================================
// Happy Path Tests: Normal Transition Scenarios
// =============================================================================

// TestEvaluateBodyTransition_HappyPath_IntraBodyJump tests the basic scenario
// where a step transitions to another step within the loop body (forward jump).
// Given: A loop body with steps [step1, step2, step3, step4]
// When: step1 transitions to step3 (nextStep = "step3")
// Then: Should return (shouldBreak=false, newIdx=2) to jump to index 2
func TestEvaluateBodyTransition_HappyPath_IntraBodyJump(t *testing.T) {
	// Arrange: Create loop with body that contains transition target
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-intra-body-jump",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-intra-body-jump")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to step3
		if stepName == "step1" {
			return "step3", nil // Should skip step2
		}
		return "", nil
	}

	// Act: Execute loop
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

	// RED phase: Stub returns (false, -1), so all steps execute sequentially
	// assert.Equal(t, []string{"step1", "step2", "step3", "step4"}, executionOrder,
	// 	"RED phase: Stub doesn't implement transition logic, all steps execute")

	// GREEN phase: Should skip step2
	assert.Equal(t, []string{"step1", "step3", "step4"}, executionOrder,
		"GREEN phase: step2 should be skipped when step1 transitions to step3")
}

// TestEvaluateBodyTransition_HappyPath_JumpToEnd tests transitioning to the last step.
// Given: Body with [step1, step2, step3]
// When: step1 transitions to step3 (last step)
// Then: Should jump to step3, skipping step2
func TestEvaluateBodyTransition_HappyPath_JumpToEnd(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-jump-to-end",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-jump-to-end")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		if stepName == "step1" {
			return "step3", nil // Jump to end
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

	// RED phase: Sequential execution
	// assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
	// 	"RED phase: All steps execute")

	// GREEN phase: Should jump to end
	assert.Equal(t, []string{"step1", "step3"}, executionOrder,
		"GREEN phase: Should skip step2 and jump to step3")
}

// TestEvaluateBodyTransition_HappyPath_MultipleJumps tests chaining multiple transitions.
// Given: Body with [step1, step2, step3, step4, step5]
// When: step1 → step3, step3 → step5
// Then: Should execute step1, step3, step5 (skip step2 and step4)
func TestEvaluateBodyTransition_HappyPath_MultipleJumps(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-multiple-jumps",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand},
			"step5": {Name: "step5", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4", "step5"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-multiple-jumps")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		switch stepName {
		case "step1":
			return "step3", nil // Skip step2
		case "step3":
			return "step5", nil // Skip step4
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

	// RED phase: All steps execute
	// assert.Equal(t, []string{"step1", "step2", "step3", "step4", "step5"}, executionOrder,
	// 	"RED phase: All steps execute sequentially")

	// GREEN phase: Should chain transitions
	assert.Equal(t, []string{"step1", "step3", "step5"}, executionOrder,
		"GREEN phase: Should execute step1→step3→step5, skipping step2 and step4")
}

// TestEvaluateBodyTransition_HappyPath_EarlyExit tests transitioning outside the loop body.
// Given: Body with [step1, step2, step3] and external step "external_step"
// When: step1 transitions to "external_step" (not in body)
// Then: Should return (shouldBreak=true, newIdx=-1) to exit loop early
func TestEvaluateBodyTransition_HappyPath_EarlyExit(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-early-exit",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-early-exit")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		if stepName == "step1" {
			return "external_step", nil // Transition outside body
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

	// RED phase: All steps execute
	// assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
	// 	"RED phase: Transition to external step ignored, all body steps execute")

	// GREEN phase: Should exit after step1
	assert.Equal(t, []string{"step1"}, executionOrder,
		"GREEN phase: Should exit loop body after step1 transitions to external_step")
}

// TestEvaluateBodyTransition_HappyPath_NoTransition tests sequential execution
// when no transition is returned (empty nextStep).
// Given: Body with [step1, step2, step3]
// When: All steps return empty nextStep
// Then: Should execute all steps sequentially (shouldBreak=false, newIdx=-1)
func TestEvaluateBodyTransition_HappyPath_NoTransition(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-no-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-no-transition")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
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

	// Both RED and GREEN phases: Sequential execution
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
		"All steps should execute sequentially when no transitions are returned")
}

// =============================================================================
// Edge Cases: Boundary Conditions
// =============================================================================

// TestEvaluateBodyTransition_EdgeCase_JumpToFirst tests jumping to the first step.
// Given: Body with [step1, step2, step3]
// When: step2 transitions to step1 (backward jump)
// Then: Should jump back to step1, creating a loop within iteration
func TestEvaluateBodyTransition_EdgeCase_JumpToFirst(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-jump-to-first",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-jump-to-first")

	executionOrder := []string{}
	executionCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		executionCount++
		// Prevent infinite loop: only jump once
		if stepName == "step2" && executionCount == 2 {
			return "step1", nil // Backward jump
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

	// RED phase: Sequential execution
	// assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
	// 	"RED phase: Sequential execution, backward jump not implemented")

	// GREEN phase: Should create mini-loop within iteration
	assert.Equal(t, []string{"step1", "step2", "step1", "step2", "step3"}, executionOrder,
		"GREEN phase: step2 should jump back to step1, then continue to step3")
}

// TestEvaluateBodyTransition_EdgeCase_SelfTransition tests step transitioning to itself.
// Given: Body with [step1, step2]
// When: step1 transitions to "step1" (self-loop)
// Then: Should handle gracefully (could infinite loop without protection)
func TestEvaluateBodyTransition_EdgeCase_SelfTransition(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-self-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-self-transition")

	executionCount := 0
	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount++
		executionOrder = append(executionOrder, stepName)
		// Only self-transition once to avoid infinite loop
		if stepName == "step1" && executionCount == 1 {
			return "step1", nil
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

	// RED phase: Sequential
	// assert.Equal(t, []string{"step1", "step2"}, executionOrder,
	// 	"RED phase: Self-transition ignored")

	// GREEN phase: Should create self-loop (with protection against infinite)
	assert.Equal(t, []string{"step1", "step1", "step2"}, executionOrder,
		"GREEN phase: step1 should execute twice due to self-transition")
}

// TestEvaluateBodyTransition_EdgeCase_EmptyBody tests behavior with empty loop body.
// Given: Empty body []
// When: Loop executes
// Then: Should handle gracefully (no transitions possible)
func TestEvaluateBodyTransition_EdgeCase_EmptyBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name:  "test-empty-body",
		Steps: map[string]*workflow.Step{},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{}, // Empty body
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-empty-body")

	executionCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount++
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
	assert.Equal(t, 0, executionCount, "Empty body should execute no steps")
}

// TestEvaluateBodyTransition_EdgeCase_SingleStepBody tests body with only one step.
// Given: Body with [single_step]
// When: single_step transitions to itself or external
// Then: Should handle both scenarios correctly
func TestEvaluateBodyTransition_EdgeCase_SingleStepBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-single-step",
		Steps: map[string]*workflow.Step{
			"only_step":     {Name: "only_step", Type: workflow.StepTypeCommand},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"only_step"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-single-step")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Transition to external step
		if stepName == "only_step" {
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

	// RED/GREEN phase: Single step executes, external transition breaks loop
	assert.Equal(t, []string{"only_step"}, executionOrder,
		"Should execute only_step then exit due to external transition")
}

// =============================================================================
// Error Handling: Invalid Inputs and Edge Cases
// =============================================================================

// TestEvaluateBodyTransition_ErrorHandling_InvalidTarget tests transition to non-existent step.
// Given: Body with [step1, step2, step3]
// When: step1 transitions to "nonexistent_step"
// Then: Should handle gracefully (log warning, continue sequentially, or exit)
func TestEvaluateBodyTransition_ErrorHandling_InvalidTarget(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-invalid-target",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-invalid-target")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		if stepName == "step1" {
			return "nonexistent_step", nil // Invalid target
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
	require.NoError(t, err, "Should not error on invalid target (graceful degradation)")
	require.NotNil(t, result)

	// Per ADR-005: Graceful degradation - invalid targets continue sequential execution
	// F048 T011: Invalid transition targets log warning and continue execution
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
		"Invalid transition ignored, all steps execute sequentially")
	assert.Greater(t, len(logger.warnings), 0, "Should log warning for invalid target")
}

// TestEvaluateBodyTransition_ErrorHandling_DuplicateStepNames tests body with duplicate names.
// Given: Body with duplicate step names [step1, step2, step1]
// When: Loop attempts to build body step indices
// Then: Should return error rejecting duplicate step names (PR-67 review item)
func TestEvaluateBodyTransition_ErrorHandling_DuplicateStepNames(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-duplicate-names",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step1"}, // Duplicate "step1"
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-duplicate-names")

	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
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
	require.Error(t, err, "should reject duplicate step names")
	require.Nil(t, result)
	require.Contains(t, err.Error(), "duplicate step 'step1'")
	require.Contains(t, err.Error(), "indices 0 and 2")
}

// =============================================================================
// Integration Tests: ExecuteForEach Transition Support
// =============================================================================

// =============================================================================
// Multi-Iteration Transition Tests
// =============================================================================

// TestEvaluateBodyTransition_MultiIteration_ConsistentBehavior tests that
// transitions work consistently across multiple loop iterations.
// Given: While loop with 3 iterations, each with same transition pattern
// When: Each iteration has step1 → step3 transition
// Then: Should skip step2 in all iterations
func TestEvaluateBodyTransition_MultiIteration_ConsistentBehavior(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-multi-iteration",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 3",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-multi-iteration")

	iterationCount := 0
	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// Transition in every iteration
		if stepName == "step1" {
			return "step3", nil
		}

		// Update condition after 3 iterations (6 steps in GREEN: 2 steps × 3 iterations)
		if len(executionOrder) >= 6 {
			evaluator.boolResults["loop.index < 3"] = false
		}

		return "", nil
	}

	// Set initial condition
	evaluator.boolResults["loop.index < 3"] = true

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
	assert.Equal(t, 3, result.TotalCount, "Should complete 3 iterations")

	// RED phase: All steps in all iterations
	// expectedRed := []string{
	// 	"step1", "step2", "step3", // iteration 1
	// 	"step1", "step2", "step3", // iteration 2
	// 	"step1", "step2", "step3", // iteration 3
	// }
	// assert.Equal(t, expectedRed, executionOrder,
	// 	"RED phase: All steps execute in all iterations")

	// GREEN phase: step2 skipped in all iterations
	expectedGreen := []string{
		"step1", "step3", // iteration 1
		"step1", "step3", // iteration 2
		"step1", "step3", // iteration 3
	}
	assert.Equal(t, expectedGreen, executionOrder,
		"GREEN phase: step2 should be skipped in all 3 iterations")

	_ = iterationCount
}

// =============================================================================
// Documentation Test
// =============================================================================

// TestT005_ComponentBehavior documents the expected behavior of T005.
func TestT005_ComponentBehavior(t *testing.T) {
	t.Log("Component T005: Transition Evaluation in Loop Body")
	t.Log("")
	t.Log("Function: evaluateBodyTransition(nextStep, bodyStepIndices, body, currentIdx)")
	t.Log("")
	t.Log("Expected Behavior:")
	t.Log("  1. If nextStep is empty: return (false, -1) - sequential execution")
	t.Log("  2. If nextStep exists in bodyStepIndices: return (false, targetIdx) - intra-body jump")
	t.Log("  3. If nextStep does not exist in body: return (true, -1) - early exit")
	t.Log("  4. Invalid targets: gracefully degrade (log warning, treat as external)")
	t.Log("")
	t.Log("Integration with ExecuteWhile/ExecuteForEach:")
	t.Log("  - Called after each stepExecutor invocation")
	t.Log("  - shouldBreak=true: immediately exits body loop (goto external step)")
	t.Log("  - newIdx≥0: sets bodyIdx to newIdx-1 (loop increments, so -1 compensates)")
	t.Log("  - newIdx=-1: continues sequential execution")
	t.Log("")
	t.Log("RED Phase: Stub returns (false, -1) always")
	t.Log("GREEN Phase: Implements transition logic per ADR-003")
}

// =============================================================================
// Component T006: Handle Intra-Body Jumps in Loop Executor
// Feature: F048 - While Loop Transitions Support
// =============================================================================
//
// handleIntraBodyJump is responsible for calculating the adjusted loop body
// index when a transition jumps to a target step within the loop body.
//
// The function must account for the for-loop's automatic increment by
// subtracting 1 from the target index, so that after the loop increments,
// the body executes at the correct position.
//
// Behavior:
//   - newIdx = -1 (no jump): returns -1 (sequential execution continues)
//   - newIdx >= 0 (intra-body jump): returns newIdx - 1 (compensate for loop increment)
//
// =============================================================================

// =============================================================================
// Happy Path Tests: Normal Intra-Body Jump Scenarios
// =============================================================================

// TestHandleIntraBodyJump_HappyPath_ForwardJump tests jumping forward within
// the loop body (e.g., skip steps 2-3 to go directly to step 4).
// Given: A loop body with 5 steps, currently at index 1
// When: Transition to index 3 (newIdx=3)
// Then: Should return 2 (newIdx - 1) to compensate for loop increment
func TestHandleIntraBodyJump_HappyPath_ForwardJump(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-forward-jump",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
			"step5": {Name: "step5", Type: workflow.StepTypeCommand, Command: "echo 5"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4", "step5"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-forward-jump")
	evaluator.boolResults["true"] = true

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step2 (index 1) transitions to step4 (index 3)
		if stepName == "step2" {
			return "step4", nil
		}
		return "", nil
	}

	// Act: Execute loop (integration test to verify handleIntraBodyJump behavior)
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

	// RED phase: handleIntraBodyJump returns -1, so all steps execute sequentially
	// assert.Equal(t, []string{"step1", "step2", "step3", "step4", "step5"}, executionOrder,
	// 	"RED phase: All steps execute sequentially (stub returns -1)")

	// GREEN phase: handleIntraBodyJump returns newIdx-1=2, loop increments to 3, executes step4
	assert.Equal(t, []string{"step1", "step2", "step4", "step5"}, executionOrder,
		"GREEN phase: Should skip step3 when jumping from step2 to step4")
}

// TestHandleIntraBodyJump_HappyPath_JumpToEnd tests jumping to the last step.
// Given: Loop body with [step1, step2, step3, step4]
// When: step1 (index 0) transitions to step4 (index 3)
// Then: Should return 2 (index 3 - 1) to land on step4 after increment
func TestHandleIntraBodyJump_HappyPath_JumpToEnd(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-jump-to-end",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-jump-to-end")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to step4 (last step)
		if stepName == "step1" {
			return "step4", nil
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

	// RED phase: All steps execute
	// assert.Equal(t, []string{"step1", "step2", "step3", "step4"}, executionOrder,
	// 	"RED phase: All steps execute (stub ignores jump)")

	// GREEN phase: Skip to last step
	assert.Equal(t, []string{"step1", "step4"}, executionOrder,
		"GREEN phase: Should skip step2 and step3, jump directly to step4")
}

// TestHandleIntraBodyJump_HappyPath_NoJump tests sequential execution when
// no jump is requested (newIdx = -1).
// Given: Loop body with [step1, step2, step3]
// When: All steps return empty nextStep (newIdx = -1)
// Then: Should return -1 (no index adjustment, sequential execution)
func TestHandleIntraBodyJump_HappyPath_NoJump(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-no-jump",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-no-jump")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
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

	// Both RED and GREEN phases: Sequential execution (handleIntraBodyJump(-1, x) returns -1)
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
		"All steps should execute sequentially when no jumps occur")
}

// =============================================================================
// Edge Cases: Boundary Conditions
// =============================================================================

// TestHandleIntraBodyJump_EdgeCase_JumpToNextStep tests jumping to the
// immediately next step (minimal jump distance of 1).
// Given: Loop body with [step1, step2, step3]
// When: step1 (index 0) transitions to step2 (index 1)
// Then: Should return 0 (index 1 - 1) resulting in step2 execution
func TestHandleIntraBodyJump_EdgeCase_JumpToNextStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-jump-next",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-jump-next")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 transitions to step2 (immediate next)
		if stepName == "step1" {
			return "step2", nil
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

	// RED phase: All steps execute (ignores jump)
	// assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
	// 	"RED phase: All steps execute")

	// GREEN phase: Normal sequential execution (jump to next is same as no jump)
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
		"GREEN phase: Jump to immediate next step should execute all steps normally")
}

// TestHandleIntraBodyJump_EdgeCase_BackwardJump tests jumping backward
// within the same iteration (e.g., step3 jumps back to step1).
// Given: Loop body with [step1, step2, step3]
// When: step3 (index 2) transitions to step1 (index 0)
// Then: Should return -1 (index 0 - 1 = -1) creating a mini-loop within iteration
func TestHandleIntraBodyJump_EdgeCase_BackwardJump(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-backward-jump",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-backward-jump")

	executionCount := make(map[string]int)
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount[stepName]++
		// Prevent infinite loop: only jump back once
		if stepName == "step3" && executionCount["step3"] == 1 {
			return "step1", nil // Jump backward
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

	// RED phase: Each step executes once (no backward jump)
	// assert.Equal(t, 1, executionCount["step1"], "RED: step1 once")
	// assert.Equal(t, 1, executionCount["step2"], "RED: step2 once")
	// assert.Equal(t, 1, executionCount["step3"], "RED: step3 once")

	// GREEN phase: Backward jump causes step1, step2, step3 to execute twice
	// Pattern: step1, step2, step3 (jump to step1), step1, step2, step3
	assert.Equal(t, 2, executionCount["step1"],
		"GREEN phase: step1 should execute twice (initial + after backward jump)")
	assert.Equal(t, 2, executionCount["step2"],
		"GREEN phase: step2 should execute twice")
	assert.Equal(t, 2, executionCount["step3"],
		"GREEN phase: step3 should execute twice")
}

// TestHandleIntraBodyJump_EdgeCase_JumpToFirstStep tests jumping to the
// very first step in the body (index 0).
// Given: Loop body with [step1, step2, step3]
// When: step2 transitions to step1 (index 0)
// Then: Should return -1 (0 - 1 = -1) to restart from beginning
func TestHandleIntraBodyJump_EdgeCase_JumpToFirstStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-jump-first",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-jump-first")

	executionCount := make(map[string]int)
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount[stepName]++
		// Jump back to first step only once to avoid infinite loop
		if stepName == "step2" && executionCount["step2"] == 1 {
			return "step1", nil
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

	// RED phase: Linear execution
	// assert.Equal(t, 1, executionCount["step1"], "RED: step1 once")
	// assert.Equal(t, 1, executionCount["step2"], "RED: step2 once")
	// assert.Equal(t, 1, executionCount["step3"], "RED: step3 once")

	// GREEN phase: Jump to first creates loop
	// Pattern: step1, step2 (jump to step1), step1, step2, step3
	assert.Equal(t, 2, executionCount["step1"],
		"GREEN phase: step1 executes twice (initial + after jump)")
	assert.Equal(t, 2, executionCount["step2"],
		"GREEN phase: step2 executes twice (before and after jump)")
	assert.Equal(t, 1, executionCount["step3"],
		"GREEN phase: step3 executes once (after second step2)")
}

// TestHandleIntraBodyJump_EdgeCase_SingleStepBody tests behavior when
// the loop body contains only one step (no jump possible).
// Given: Loop body with [step1] only
// When: step1 returns any transition (becomes early exit since no other body steps)
// Then: Should handle gracefully (this tests integration with T005)
func TestHandleIntraBodyJump_EdgeCase_SingleStepBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-single-step",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo ext"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1"}, // Single step body
			MaxIterations: 2,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-single-step")

	executionCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount++
		// Transition to external step (early exit)
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

	// RED phase: Both iterations execute
	// assert.Equal(t, 2, executionCount, "RED: 2 iterations (transition ignored)")

	// GREEN phase: Early exit on first iteration
	assert.Equal(t, 1, executionCount,
		"GREEN phase: Should exit after first iteration when transitioning to external step")
}

// TestHandleIntraBodyJump_EdgeCase_MultipleJumpsInIteration tests multiple
// transitions within a single loop iteration.
// Given: Loop body with [step1, step2, step3, step4]
// When: step1→step3, step3→step4
// Then: Should handle both jumps correctly (step1, step3, step4)
func TestHandleIntraBodyJump_EdgeCase_MultipleJumpsInIteration(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-multiple-jumps",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-multiple-jumps")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Multiple jumps in same iteration
		if stepName == "step1" {
			return "step3", nil // Skip step2
		}
		if stepName == "step3" {
			return "step4", nil // Skip to end
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

	// RED phase: All steps execute
	// assert.Equal(t, []string{"step1", "step2", "step3", "step4"}, executionOrder,
	// 	"RED: All steps execute")

	// GREEN phase: Two jumps in succession
	assert.Equal(t, []string{"step1", "step3", "step4"}, executionOrder,
		"GREEN phase: Should handle two consecutive jumps (step1→step3→step4, skipping step2)")
}

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestHandleIntraBodyJump_ErrorHandling_JumpWithStepError tests that when
// a step returns both a nextStep and an error, the error takes precedence
// and the transition is not processed.
// Given: Loop body with [step1, step2, step3]
// When: step1 returns ("step3", error)
// Then: Should handle error, not process jump
func TestHandleIntraBodyJump_ErrorHandling_JumpWithStepError(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-jump-with-error",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-jump-with-error")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// When: step1 returns error WITH nextStep (on_failure transition scenario)
		if stepName == "step1" {
			return "step3", assert.AnError // Error takes precedence
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
	// Error should propagate (loop execution fails)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assert.AnError")

	// Only step1 should have executed before error
	assert.Equal(t, []string{"step1"}, executionOrder,
		"Should stop after step1 error, regardless of nextStep value")
	_ = result // May be nil on error
}

// =============================================================================
// Integration Tests: ExecuteForEach with handleIntraBodyJump
// =============================================================================

// TestHandleIntraBodyJump_Integration_ForEachJump verifies that
// handleIntraBodyJump works correctly in ExecuteForEach context.
// Given: ForEach loop with items ["a", "b", "c"] and body [step1, step2, step3]
// When: step1 transitions to step3 in each iteration
// Then: Should skip step2 in all iterations
func TestHandleIntraBodyJump_Integration_ForEachJump(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newConfigurableMockResolver()

	// Configure resolver for items parsing
	resolver.results[`["a","b","c"]`] = `["a","b","c"]`

	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	wf := &workflow.Workflow{
		Name: "test-foreach-jump",
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
			Items: `["a","b","c"]`,
			Body:  []string{"step1", "step2", "step3"},
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-foreach-jump")

	executionOrder := []string{}
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionOrder = append(executionOrder, stepName)
		// Jump from step1 to step3 in every iteration
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
	assert.Equal(t, 3, result.TotalCount, "Should process 3 items")

	// RED phase: All steps in all iterations
	// expectedRed := []string{
	// 	"step1", "step2", "step3", // item "a"
	// 	"step1", "step2", "step3", // item "b"
	// 	"step1", "step2", "step3", // item "c"
	// }
	// assert.Equal(t, expectedRed, executionOrder, "RED: All steps in all iterations")

	// GREEN phase: step2 skipped in all iterations
	expectedGreen := []string{
		"step1", "step3", // item "a"
		"step1", "step3", // item "b"
		"step1", "step3", // item "c"
	}
	assert.Equal(t, expectedGreen, executionOrder,
		"GREEN phase: step2 should be skipped in all 3 ForEach iterations")
}

// =============================================================================
// Performance/Stress Tests
// =============================================================================

// TestHandleIntraBodyJump_Performance_LargeBody tests handling of jumps
// in a loop with a large body (edge case: performance and index calculation).
// Given: Loop body with 100 steps
// When: Jump from step 10 to step 90
// Then: Should correctly calculate index adjustment (89 = 90 - 1)
func TestHandleIntraBodyJump_Performance_LargeBody(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	evaluator.boolResults["true"] = true
	resolver := newConfigurableMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Create workflow with 100 steps
	steps := make(map[string]*workflow.Step)
	bodySteps := make([]string, 100)
	for i := 0; i < 100; i++ {
		stepName := fmt.Sprintf("step%d", i)
		steps[stepName] = &workflow.Step{
			Name:    stepName,
			Type:    workflow.StepTypeCommand,
			Command: fmt.Sprintf("echo %d", i),
		}
		bodySteps[i] = stepName
	}

	wf := &workflow.Workflow{
		Name:  "test-large-body",
		Steps: steps,
	}

	step := &workflow.Step{
		Name: "loop",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          bodySteps,
			MaxIterations: 1,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-large-body")

	executionCount := 0
	var stepExecutor application.StepExecutorFunc = func(
		ctx context.Context,
		stepName string,
		intCtx *interpolation.Context,
	) (string, error) {
		executionCount++
		// Jump from step10 to step90
		if stepName == "step10" {
			return "step90", nil
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

	// RED phase: All 100 steps execute
	// assert.Equal(t, 100, executionCount, "RED: 100 steps")

	// GREEN phase: Steps 0-10, then 90-99 (11 + 10 = 21 steps)
	assert.Equal(t, 21, executionCount,
		"GREEN phase: Should execute steps 0-10 (11 steps) then 90-99 (10 steps) = 21 total")
}

// =============================================================================
// Documentation Test
// =============================================================================

// TestT006_ComponentBehavior documents the expected behavior of T006.
func TestT006_ComponentBehavior(t *testing.T) {
	t.Log("Component T006: Handle Intra-Body Jumps in Loop Executor")
	t.Log("")
	t.Log("Function: handleIntraBodyJump(newIdx int, currentIdx int) int")
	t.Log("")
	t.Log("Purpose:")
	t.Log("  Adjusts the loop body index when a transition jumps to a target step")
	t.Log("  within the loop body. Accounts for the for-loop's automatic increment")
	t.Log("  by returning newIdx - 1.")
	t.Log("")
	t.Log("Parameters:")
	t.Log("  - newIdx: target step index within body (-1 means no jump)")
	t.Log("  - currentIdx: current position in the loop body")
	t.Log("")
	t.Log("Returns:")
	t.Log("  - adjustedIdx: the index to assign to bodyIdx")
	t.Log("    - If newIdx = -1: returns -1 (no change, sequential execution)")
	t.Log("    - If newIdx >= 0: returns newIdx - 1 (compensate for loop increment)")
	t.Log("")
	t.Log("Integration:")
	t.Log("  Called in ExecuteWhile and ExecuteForEach after evaluateBodyTransition")
	t.Log("  when newIdx >= 0 (intra-body jump detected)")
	t.Log("")
	t.Log("Examples:")
	t.Log("  - Body: [step1, step2, step3, step4]")
	t.Log("  - Current: bodyIdx=1 (step2)")
	t.Log("  - Transition: step2 → step4 (newIdx=3)")
	t.Log("  - Returns: 2 (3 - 1)")
	t.Log("  - Loop increment: bodyIdx becomes 3")
	t.Log("  - Next iteration: executes step4 (index 3)")
	t.Log("")
	t.Log("RED Phase:")
	t.Log("  Stub implementation returns -1 always (ignores jumps)")
	t.Log("  All tests should execute sequentially")
	t.Log("")
	t.Log("GREEN Phase:")
	t.Log("  Implements: return newIdx == -1 ? -1 : newIdx - 1")
	t.Log("  Tests verify correct index calculation and jump behavior")
}

// Component T007: Handle Early Exit Transitions in ExecuteWhile
// Feature: F048 - While Loop Transitions Support
// =============================================================================
//
// T007 is responsible for capturing the target step name when a loop body step
// transitions to a step outside the loop body, triggering early loop exit.
//
// The implementation adds an exitNextStep variable that:
//   1. Is initialized at the start of the body execution loop
//   2. Captures the nextStep value when shouldBreak is true (external transition)
//   3. Sets result.NextStep = exitNextStep before breaking from the loop
//   4. Allows the execution service to continue workflow at the correct step
//
// This enables early exit from loops with proper workflow continuation, preventing
// unnecessary agent execution and supporting complex control flow patterns.
//
// =============================================================================

// =============================================================================
// Happy Path Tests: Normal Early Exit Scenarios
// =============================================================================

// TestEarlyExitTransition_HappyPath_ExitFromFirstStep tests early exit when
// the first step in a while loop body transitions to an external step.
// Given: While loop with body [step1, step2, step3]
// When: step1 transitions to external_step
// Then: Loop should exit immediately, result.NextStep should be "external_step"
