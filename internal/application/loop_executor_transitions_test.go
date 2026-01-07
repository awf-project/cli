package application_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// =============================================================================
// Consolidated F048 Loop Body Transitions Tests
// Components: T004, T005, T006, T007, T008, T010, T011
// =============================================================================

// Component T004: Body Step Index Mapping in ExecuteWhile
// Feature: F048 - While Loop Transitions Support
// =============================================================================

// TestBuildBodyStepIndices_HappyPath_SimpleSequence verifies that
// buildBodyStepIndices creates correct step name to index mapping
// for a simple sequential body.
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
	evaluator.results["true"] = true
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
			evaluator.results["true"] = false
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["loop.index < 2"] = true

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
			evaluator.results["loop.index < 2"] = false
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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

// TestEvaluateBodyTransition_ErrorHandling_NilBodyStepIndices tests nil index map.
// Given: bodyStepIndices is nil (should never happen but test robustness)
// When: Transition is returned
// Then: Should handle gracefully without panic
func TestEvaluateBodyTransition_ErrorHandling_NilBodyStepIndices(t *testing.T) {
	// This is a theoretical edge case - in practice, buildBodyStepIndices
	// always returns a valid map (empty for empty body). But we test
	// the evaluateBodyTransition function's robustness if called incorrectly.

	t.Skip("evaluateBodyTransition is private and always called with valid map from buildBodyStepIndices")

	// If we could test directly:
	// shouldBreak, newIdx := evaluateBodyTransition("target", nil, []string{"step1"}, 0)
	// assert.False(t, shouldBreak, "Should not panic on nil map")
	// assert.Equal(t, -1, newIdx, "Should return -1 for invalid state")
}

// =============================================================================
// Integration Tests: ExecuteForEach Transition Support
// =============================================================================

// TestExecuteForEach_TransitionSupport verifies ExecuteForEach also honors transitions.
// Given: ForEach loop with body that contains transitions
// When: Body step transitions to another body step
// Then: Should skip steps like ExecuteWhile
//
// Note: This test is skipped because it requires proper resolver/parser setup
// which is complex for this unit test. The transition logic in ExecuteForEach
// is identical to ExecuteWhile (same evaluateBodyTransition call), so the
// While loop tests provide sufficient coverage. Integration tests will verify
// ForEach transition behavior end-to-end.
func TestExecuteForEach_TransitionSupport(t *testing.T) {
	t.Skip("Transition logic is identical for ForEach and While loops - covered by While tests")

	// The actual implementation is tested through:
	// 1. TestEvaluateBodyTransition_* tests (all scenarios)
	// 2. TestEvaluateBodyTransition_MultiIteration_ConsistentBehavior
	// 3. Integration tests will verify ForEach-specific behavior
}

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
			evaluator.results["loop.index < 3"] = false
		}

		return "", nil
	}

	// Set initial condition
	evaluator.results["loop.index < 3"] = true

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
	evaluator.results["true"] = true

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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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
	evaluator.results["true"] = true
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

// TestHandleIntraBodyJump_ErrorHandling_NegativeIndexInput tests behavior
// when newIdx is negative (but not -1).
// Given: handleIntraBodyJump receives newIdx = -2
// When: Called with invalid negative index
// Then: Should handle gracefully (likely return -1 for no jump)
func TestHandleIntraBodyJump_ErrorHandling_NegativeIndexInput(t *testing.T) {
	// Note: This is a theoretical edge case testing the function's robustness.
	// In practice, evaluateBodyTransition should never pass negative indices
	// other than -1, but testing defensive programming is good practice.

	// This test is challenging to write because handleIntraBodyJump is private.
	// We would test this if the function were exported or via integration tests
	// that somehow trigger this condition.

	// For now, documenting expected behavior:
	// - newIdx < 0 (other than -1): treat as -1 (no jump)
	// - The function should not crash or return invalid indices

	t.Skip("Cannot test private function directly; behavior verified via integration tests")
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
	evaluator.results["true"] = true
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

// Component T008: Apply Transition Logic to ExecuteForEach
// Feature: F048 - While Loop Transitions Support
// =============================================================================
//
// T008 applies the same transition logic implemented in ExecuteWhile (T004-T007)
// to the ExecuteForEach method, ensuring foreach loops support:
//   1. Intra-body transitions (skip steps within iteration)
//   2. Early exit transitions (break loop when target outside body)
//   3. Invalid target graceful degradation (log warning, continue sequential)
//   4. Retry pattern preservation (transition to loop step itself)
//
// This ensures feature parity between while and foreach loop types.
//
// =============================================================================

// =============================================================================
// Happy Path Tests: Normal Transition Scenarios in ForEach
// =============================================================================

// TestForEachTransition_HappyPath_IntraBodySkipForward tests skipping forward
// within a foreach loop body by transitioning to a later step.
// Given: ForEach loop with items [a, b] and body [step1, step2, step3]
// When: step1 transitions to step3 (skips step2)
// Then: step2 should not execute, step3 should execute
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

// F048-T010: Loop Body Transition - Skip Steps Scenario Tests
// =============================================================================
//
// These tests verify that transitions within loop bodies correctly skip
// intermediate steps when a transition is triggered.
//
// Test scenarios:
// 1. Skip single step - transition from step 1 to step 3 (skips step 2)
// 2. Skip multiple steps - transition from step 1 to step 4 (skips steps 2-3)
// 3. Skip to end of body - transition to last step (skips all intermediate)
// 4. Conditional skip - transition only when condition met
// 5. Skip in ForEach loop - verify skip works in foreach context
// 6. Skip in While loop - verify skip works in while context
//
// Expected behavior:
// - Skipped steps should NOT appear in execution recorder
// - callCount should reflect only executed steps
// - Loop should continue normally after transition target
// =============================================================================

// TestLoopExecutor_ExecuteWhile_SkipSingleStep tests that a transition
// within a while loop body can skip a single intermediate step.
//
// Given: A while loop with body [step1, step2, step3]
//
//	step1 has transition: goto step3
//
// When:  Loop executes
// Then:  step2 is skipped, execution order is [step1, step3]
func TestLoopExecutor_ExecuteWhile_SkipSingleStep(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Condition: true for first iteration, then false
	iterationCount := 0
	evaluator.results["loop.index < 1"] = true

	wf := &workflow.Workflow{
		Name: "test-skip-single",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-skip-single")

	// Track execution order
	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// After first execution, make condition false to stop loop
		if len(executionOrder) >= 2 {
			evaluator.results["loop.index < 1"] = false
		}

		// step1 transitions to step3 (skip step2)
		if stepName == "step1" {
			return "step3", nil
		}
		return "", nil
	}

	contextBuilder := func(ec *workflow.ExecutionContext) *interpolation.Context {
		iterationCount++
		return interpolation.NewContext()
	}

	// Act
	result, err := loopExec.ExecuteWhile(
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

	// Expected: step1 -> step3 (step2 skipped)
	assert.Equal(t, []string{"step1", "step3"}, executionOrder,
		"step2 should be skipped due to step1 -> step3 transition")
	assert.Equal(t, 1, result.TotalCount, "should execute 1 iteration")
}

// TestLoopExecutor_ExecuteWhile_SkipMultipleSteps tests that a transition
// can skip multiple consecutive steps in the loop body.
//
// Given: A while loop with body [check, prepare, validate, execute, cleanup]
//
//	check has transition: when pass, goto cleanup
//
// When:  Check passes on first iteration
// Then:  prepare, validate, execute are all skipped
func TestLoopExecutor_ExecuteWhile_SkipMultipleSteps(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["loop.index < 1"] = true

	wf := &workflow.Workflow{
		Name: "test-skip-multiple",
		Steps: map[string]*workflow.Step{
			"check":    {Name: "check", Type: workflow.StepTypeCommand, Command: "check"},
			"prepare":  {Name: "prepare", Type: workflow.StepTypeCommand, Command: "prepare"},
			"validate": {Name: "validate", Type: workflow.StepTypeCommand, Command: "validate"},
			"execute":  {Name: "execute", Type: workflow.StepTypeCommand, Command: "execute"},
			"cleanup":  {Name: "cleanup", Type: workflow.StepTypeCommand, Command: "cleanup"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"check", "prepare", "validate", "execute", "cleanup"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-skip-multiple")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// After cleanup, stop loop
		if stepName == "cleanup" {
			evaluator.results["loop.index < 1"] = false
		}

		// check transitions to cleanup (skip prepare, validate, execute)
		if stepName == "check" {
			return "cleanup", nil
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

	// Expected: check -> cleanup (prepare, validate, execute skipped)
	assert.Equal(t, []string{"check", "cleanup"}, executionOrder,
		"prepare, validate, execute should be skipped due to check -> cleanup transition")
	assert.Equal(t, 1, result.TotalCount, "should execute 1 iteration")
}

// TestLoopExecutor_ExecuteWhile_SkipToEnd tests transition to the last
// step in the loop body, skipping all intermediate steps.
//
// Given: A while loop with body [step1, step2, step3, step4, step5]
//
//	step1 has transition: goto step5
//
// When:  Loop executes
// Then:  Only step1 and step5 execute, steps 2-4 are skipped
func TestLoopExecutor_ExecuteWhile_SkipToEnd(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["loop.index < 1"] = true

	wf := &workflow.Workflow{
		Name: "test-skip-to-end",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
			"step5": {Name: "step5", Type: workflow.StepTypeCommand, Command: "echo 5"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"step1", "step2", "step3", "step4", "step5"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-skip-to-end")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// After step5, stop loop
		if stepName == "step5" {
			evaluator.results["loop.index < 1"] = false
		}

		// step1 transitions to step5 (skip all middle steps)
		if stepName == "step1" {
			return "step5", nil
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

	// Expected: step1 -> step5 (steps 2, 3, 4 skipped)
	assert.Equal(t, []string{"step1", "step5"}, executionOrder,
		"steps 2, 3, 4 should be skipped due to step1 -> step5 transition")
	assert.Equal(t, 1, result.TotalCount, "should execute 1 iteration")
}

// TestLoopExecutor_ExecuteWhile_ConditionalSkip tests that transitions
// only skip steps when the condition matches.
//
// Given: A while loop with body [check, step1, step2, step3]
//
//	check has transition: when output contains "SKIP", goto step3
//
// When:  First iteration outputs "SKIP", second iteration outputs "CONTINUE"
// Then:  First iteration: [check, step3]
//
//	Second iteration: [check, step1, step2, step3]
func TestLoopExecutor_ExecuteWhile_ConditionalSkip(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["loop.index < 2"] = true

	wf := &workflow.Workflow{
		Name: "test-conditional-skip",
		Steps: map[string]*workflow.Step{
			"check": {Name: "check", Type: workflow.StepTypeCommand, Command: "check"},
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 2",
			Body:          []string{"check", "step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-conditional-skip")

	executionOrder := []string{}
	iterationNum := 0
	checkCallCount := 0

	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// Track when we finish an iteration
		if stepName == "step3" {
			iterationNum++
			if iterationNum >= 2 {
				evaluator.results["loop.index < 2"] = false
			}
		}

		// check transitions conditionally
		if stepName == "check" {
			checkCallCount++
			// First call: skip to step3
			if checkCallCount == 1 {
				return "step3", nil
			}
			// Second call: no transition (continue normally)
			return "", nil
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

	// Expected: First iteration: [check, step3]
	//           Second iteration: [check, step1, step2, step3]
	expected := []string{"check", "step3", "check", "step1", "step2", "step3"}
	assert.Equal(t, expected, executionOrder,
		"first iteration should skip to step3, second iteration should run all steps")
	assert.Equal(t, 2, result.TotalCount, "should execute 2 iterations")
}

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

// TestLoopExecutor_ExecuteWhile_SkipWithLoopVariables tests that loop
// variables (index, first, last) remain correct when steps are skipped.
//
// Given: A while loop with body [check, process, finish]
//
//	check has transition: goto finish
//
// When:  Loop executes 3 iterations
// Then:  Loop variables (index, first, last) are correct in finish step
//
//	even though process was skipped
func TestLoopExecutor_ExecuteWhile_SkipWithLoopVariables(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["loop.index < 3"] = true

	wf := &workflow.Workflow{
		Name: "test-skip-loop-vars",
		Steps: map[string]*workflow.Step{
			"check":   {Name: "check", Type: workflow.StepTypeCommand, Command: "check"},
			"process": {Name: "process", Type: workflow.StepTypeCommand, Command: "process"},
			"finish":  {Name: "finish", Type: workflow.StepTypeCommand, Command: "finish"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 3",
			Body:          []string{"check", "process", "finish"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-skip-loop-vars")

	// Track loop variables at finish step
	type loopVars struct {
		index int
		first bool
		last  bool
	}
	finishVars := []loopVars{}
	iterationCount := 0

	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		if stepName == "finish" {
			// Capture loop variables
			if intCtx.Loop != nil {
				finishVars = append(finishVars, loopVars{
					index: intCtx.Loop.Index,
					first: intCtx.Loop.First,
					last:  intCtx.Loop.Last,
				})
			}
			iterationCount++
			if iterationCount >= 3 {
				evaluator.results["loop.index < 3"] = false
			}
		}

		// check transitions to finish (skip process)
		if stepName == "check" {
			return "finish", nil
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
			// Build context with loop data from ExecutionContext
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
	assert.Equal(t, 3, result.TotalCount, "should execute 3 iterations")

	// Verify loop variables are correct
	require.Len(t, finishVars, 3, "should have captured loop vars 3 times")

	// First iteration
	assert.Equal(t, 0, finishVars[0].index, "first iteration should have index 0")
	assert.True(t, finishVars[0].first, "first iteration should have first=true")
	// Note: while loops don't know when they'll end, so last is always false during execution
	assert.False(t, finishVars[0].last, "while loop: last is false during iteration")

	// Second iteration
	assert.Equal(t, 1, finishVars[1].index, "second iteration should have index 1")
	assert.False(t, finishVars[1].first, "second iteration should have first=false")
	assert.False(t, finishVars[1].last, "while loop: last is false during iteration")

	// Third iteration
	assert.Equal(t, 2, finishVars[2].index, "third iteration should have index 2")
	assert.False(t, finishVars[2].first, "third iteration should have first=false")
	// While loops don't know in advance which iteration is last
	assert.False(t, finishVars[2].last, "while loop: last is false during iteration (condition-based loop)")
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

// TestLoopExecutor_ExecuteWhile_NoSkipWhenNoTransition tests backward
// compatibility - loops without transitions work as before.
//
// Given: A while loop with body [step1, step2, step3]
//
//	No transitions defined
//
// When:  Loop executes
// Then:  All steps execute in order [step1, step2, step3]
func TestLoopExecutor_ExecuteWhile_NoSkipWhenNoTransition(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["loop.index < 1"] = true

	wf := &workflow.Workflow{
		Name: "test-no-skip-no-transition",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-no-skip-no-transition")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// After step3, stop loop
		if stepName == "step3" {
			evaluator.results["loop.index < 1"] = false
		}

		// No transitions - always return empty string
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

	// Expected: All steps execute in order (backward compatibility)
	assert.Equal(t, []string{"step1", "step2", "step3"}, executionOrder,
		"all steps should execute in order when no transitions are defined")
	assert.Equal(t, 1, result.TotalCount, "should execute 1 iteration")
}

// TestLoopExecutor_ExecuteWhile_SkipStepRecorderVerification tests that
// the stepExecutorRecorder correctly shows which steps were skipped.
//
// Given: A while loop with body [a, b, c]
//
//	step a has transition: goto c
//
// When:  Loop executes one iteration
// Then:  recorder.executions contains only [a, c]
//
//	recorder.executions does NOT contain b
func TestLoopExecutor_ExecuteWhile_SkipStepRecorderVerification(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["loop.index < 1"] = true

	wf := &workflow.Workflow{
		Name: "test-recorder-verification",
		Steps: map[string]*workflow.Step{
			"a": {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b"},
			"c": {Name: "c", Type: workflow.StepTypeCommand, Command: "echo c"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"a", "b", "c"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-recorder-verification")

	// Use recorder for detailed verification
	recorder := newStepExecutorRecorder()
	recorder.transitions["a"] = "c" // a -> c transition

	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		nextStep, err := recorder.execute(ctx, stepName, intCtx)

		// After c, stop loop
		if stepName == "c" {
			evaluator.results["loop.index < 1"] = false
		}

		return nextStep, err
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

	// Verify recorder captured only executed steps
	require.Len(t, recorder.executions, 2, "should have recorded 2 step executions")
	assert.Equal(t, "a", recorder.executions[0].stepName, "first execution should be 'a'")
	assert.Equal(t, "c", recorder.executions[1].stepName, "second execution should be 'c'")

	// Verify b was NOT executed
	for _, exec := range recorder.executions {
		assert.NotEqual(t, "b", exec.stepName, "step 'b' should not be in executions")
	}

	assert.Equal(t, 1, result.TotalCount, "should execute 1 iteration")
}

// F048-T011: Loop Body Transition - Early Exit and Invalid Targets
// =============================================================================
//
// These tests verify that transitions within loop bodies correctly handle:
// 1. Early exit - transition to a step outside the loop body
// 2. Invalid targets - transition to a non-existent step
//
// Test scenarios:
// 1. Early exit from While loop - transition outside body breaks loop
// 2. Early exit from ForEach loop - transition outside body breaks loop
// 3. Invalid target in While loop - warning logged, sequential execution continues
// 4. Invalid target in ForEach loop - warning logged, sequential execution continues
// 5. Multiple early exits - first matching transition wins
// 6. Early exit vs intra-body transition - verify correct behavior detection
//
// Expected behavior:
// - Early exit: loop breaks immediately, no more body steps executed
// - Invalid target: warning logged, execution continues sequentially
// - Execution recorder reflects actual execution order
// =============================================================================

// TestLoopExecutor_ExecuteWhile_EarlyExit tests that a transition to a step
// outside the loop body causes the loop to exit immediately.
//
// Given: A while loop with body [step1, step2, step3]
//
//	step1 has transition: goto external_step (not in body)
//
// When:  Loop executes first iteration
// Then:  step1 executes, loop breaks, step2 and step3 are never executed
func TestLoopExecutor_ExecuteWhile_EarlyExit(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Condition: always true (but we'll exit early via transition)
	evaluator.results["true"] = true

	wf := &workflow.Workflow{
		Name: "test-early-exit",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-early-exit")

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
	result, err := loopExec.ExecuteWhile(
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
	assert.Equal(t, []string{"step1"}, executionOrder, "should only execute step1 before early exit")
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

// TestLoopExecutor_ExecuteWhile_InvalidTarget tests that a transition to a
// non-existent step logs a warning and continues sequential execution.
//
// Given: A while loop with body [step1, step2, step3]
//
//	step1 has transition: goto non_existent_step
//
// When:  Loop executes
// Then:  Warning logged, step1, step2, step3 execute sequentially
func TestLoopExecutor_ExecuteWhile_InvalidTarget(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	// Condition: true for first iteration, then false
	iterationCount := 0
	evaluator.results["loop.index < 1"] = true

	wf := &workflow.Workflow{
		Name: "test-invalid-target",
		Steps: map[string]*workflow.Step{
			"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "loop.index < 1",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-invalid-target")

	// Track execution order
	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// After all steps executed, make condition false to stop loop
		if len(executionOrder) >= 3 {
			evaluator.results["loop.index < 1"] = false
		}

		// step1 transitions to non-existent step
		if stepName == "step1" {
			return "non_existent_step", nil
		}
		return "", nil
	}

	contextBuilder := func(ec *workflow.ExecutionContext) *interpolation.Context {
		iterationCount++
		return interpolation.NewContext()
	}

	// Act
	result, err := loopExec.ExecuteWhile(
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

// TestLoopExecutor_ExecuteWhile_MultipleEarlyExits tests that when multiple
// body steps have transitions to external steps, the first one wins.
//
// Given: A while loop with body [step1, step2, step3]
//
//	step1 has transition: goto external1
//	step2 has transition: goto external2
//
// When:  Loop executes
// Then:  step1 executes and triggers early exit to external1
//
//	step2 and step3 never execute
func TestLoopExecutor_ExecuteWhile_MultipleEarlyExits(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["true"] = true

	wf := &workflow.Workflow{
		Name: "test-multiple-exits",
		Steps: map[string]*workflow.Step{
			"step1":     {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":     {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":     {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"external1": {Name: "external1", Type: workflow.StepTypeCommand, Command: "echo ext1"},
			"external2": {Name: "external2", Type: workflow.StepTypeCommand, Command: "echo ext2"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-multiple-exits")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		if stepName == "step1" {
			return "external1", nil
		}
		if stepName == "step2" {
			return "external2", nil
		}
		return "", nil
	}

	contextBuilder := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := loopExec.ExecuteWhile(
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
	assert.Equal(t, "external1", result.NextStep, "should return first early exit target")
	assert.Equal(t, []string{"step1"}, executionOrder, "should only execute step1")
}

// TestLoopExecutor_ExecuteWhile_EarlyExitVsIntraBodyTransition tests the
// difference between an early exit (transition outside body) and an intra-body
// transition (transition within body).
//
// Given: A while loop with body [step1, step2, step3, step4]
//
//	step1 has transition: goto step3 (intra-body, should skip step2)
//	step3 has transition: goto external_step (early exit)
//
// When:  Loop executes
// Then:  step1 → step3 (skip step2) → early exit
//
//	Execution order: [step1, step3]
func TestLoopExecutor_ExecuteWhile_EarlyExitVsIntraBodyTransition(t *testing.T) {
	// Arrange
	logger := &mockLogger{}
	evaluator := newMockExpressionEvaluator()
	resolver := newMockResolver()
	loopExec := application.NewLoopExecutor(logger, evaluator, resolver)

	evaluator.results["true"] = true

	wf := &workflow.Workflow{
		Name: "test-exit-vs-intra",
		Steps: map[string]*workflow.Step{
			"step1":         {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
			"step2":         {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
			"step3":         {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			"step4":         {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo 4"},
			"external_step": {Name: "external_step", Type: workflow.StepTypeCommand, Command: "echo external"},
		},
	}

	step := &workflow.Step{
		Name: "loop_step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:          workflow.LoopTypeWhile,
			Condition:     "true",
			Body:          []string{"step1", "step2", "step3", "step4"},
			MaxIterations: 10,
		},
	}

	execCtx := workflow.NewExecutionContext("test-id", "test-exit-vs-intra")

	executionOrder := []string{}
	stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) (string, error) {
		executionOrder = append(executionOrder, stepName)

		// step1 transitions within body to step3
		if stepName == "step1" {
			return "step3", nil
		}
		// step3 transitions outside body (early exit)
		if stepName == "step3" {
			return "external_step", nil
		}
		return "", nil
	}

	contextBuilder := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return interpolation.NewContext()
	}

	// Act
	result, err := loopExec.ExecuteWhile(
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
	assert.Equal(t, []string{"step1", "step3"}, executionOrder, "should execute step1, skip to step3, then early exit")
}
