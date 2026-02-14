//go:build integration

// Feature: C003 - Reduce Graph Algorithm Cognitive Complexity
//
// Functional tests for Graph Algorithm Cognitive Complexity Refactoring.
// These tests validate that the refactored graph algorithms (DetectCycles and
// ComputeExecutionOrder) maintain backward compatibility and correct behavior
// after introducing visitState enum and extracting helper functions.
//
// Acceptance Criteria Coverage:
// - AC1: DetectCycles cognitive complexity ≤20 (verified by gocognit)
// - AC2: ComputeExecutionOrder cognitive complexity ≤20 (verified by gocognit)
// - AC3: All existing unit tests pass without modification
// - AC4: All integration tests pass
// - AC5: No changes to public API signatures
// - AC6: Backward compatibility maintained with all existing workflows
//
// Test Categories:
// - Happy Path: Normal graph traversal with various workflow structures
// - Edge Cases: Complex cycles, parallel branches, deep nesting, empty graphs
// - Error Handling: Invalid states, missing transitions, unreachable states
// - Integration: Full workflow validation with cycle detection and execution order

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// TestDetectCycles_Integration_LinearWorkflow validates that linear workflows
// (no cycles) are correctly identified as cycle-free.
func TestDetectCycles_Integration_LinearWorkflow(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "process",
			OnFailure: "error",
		},
		"process": {
			Type:      "command",
			OnSuccess: "finish",
			OnFailure: "error",
		},
		"error": {
			Type: "terminal",
		},
		"finish": {
			Type: "terminal",
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.Empty(t, cycles, "Linear workflow should have no cycles")
}

// TestDetectCycles_Integration_ComplexParallelWorkflow validates cycle detection
// in workflows with parallel branches and multiple paths.
func TestDetectCycles_Integration_ComplexParallelWorkflow(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "parallel_step",
			OnFailure: "error",
		},
		"parallel_step": {
			Type: workflow.StepTypeParallel,
			Branches: []string{
				"branch_a",
				"branch_b",
				"branch_c",
			},
			OnSuccess: "merge",
			OnFailure: "error",
		},
		"branch_a": {
			Type:      "command",
			OnSuccess: "merge",
		},
		"branch_b": {
			Type:      "command",
			OnSuccess: "validate",
		},
		"branch_c": {
			Type:      "command",
			OnSuccess: "merge",
		},
		"validate": {
			Type:      "command",
			OnSuccess: "merge",
		},
		"merge": {
			Type:      "command",
			OnSuccess: "finish",
		},
		"error": {
			Type: "terminal",
		},
		"finish": {
			Type: "terminal",
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.Empty(t, cycles, "Parallel workflow without cycles should be detected correctly")
}

// TestComputeExecutionOrder_Integration_LinearWorkflow validates that execution
// order is correctly computed for simple linear workflows.
func TestComputeExecutionOrder_Integration_LinearWorkflow(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "process",
			OnFailure: "error",
		},
		"process": {
			Type:      "command",
			OnSuccess: "finish",
			OnFailure: "error",
		},
		"error": {
			Type: "terminal",
		},
		"finish": {
			Type: "terminal",
		},
	}

	order, err := workflow.ComputeExecutionOrder(steps, "start")

	require.NoError(t, err)
	require.NotEmpty(t, order)

	// Verify that initial state comes first
	assert.Equal(t, "start", order[0], "Start state should be first")

	// Verify all states are included
	assert.Contains(t, order, "start")
	assert.Contains(t, order, "process")
	assert.Contains(t, order, "error")
	assert.Contains(t, order, "finish")
}

// TestComputeExecutionOrder_Integration_ParallelWorkflow validates execution
// order computation for workflows with parallel branches.
func TestComputeExecutionOrder_Integration_ParallelWorkflow(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "parallel_step",
		},
		"parallel_step": {
			Type: workflow.StepTypeParallel,
			Branches: []string{
				"branch_a",
				"branch_b",
			},
			OnSuccess: "merge",
		},
		"branch_a": {
			Type:      "command",
			OnSuccess: "merge",
		},
		"branch_b": {
			Type:      "command",
			OnSuccess: "merge",
		},
		"merge": {
			Type:      "command",
			OnSuccess: "finish",
		},
		"finish": {
			Type: "terminal",
		},
	}

	order, err := workflow.ComputeExecutionOrder(steps, "start")

	require.NoError(t, err)
	require.NotEmpty(t, order)

	// Verify that parallel_step comes before its branches
	parallelIdx := indexOf(order, "parallel_step")
	branchAIdx := indexOf(order, "branch_a")
	branchBIdx := indexOf(order, "branch_b")

	assert.Greater(t, branchAIdx, parallelIdx, "branch_a should come after parallel_step")
	assert.Greater(t, branchBIdx, parallelIdx, "branch_b should come after parallel_step")
}

// TestDetectCycles_Integration_MultipleCyclesInGraph validates detection of
// multiple distinct cycles in a single workflow graph.
func TestDetectCycles_Integration_MultipleCyclesInGraph(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "cycle1_a",
			OnFailure: "cycle2_a",
		},
		// First cycle: cycle1_a -> cycle1_b -> cycle1_a
		"cycle1_a": {
			Type:      "command",
			OnSuccess: "cycle1_b",
		},
		"cycle1_b": {
			Type:      "command",
			OnSuccess: "cycle1_a", // Back to cycle1_a
			OnFailure: "terminal",
		},
		// Second cycle: cycle2_a -> cycle2_b -> cycle2_c -> cycle2_a
		"cycle2_a": {
			Type:      "command",
			OnSuccess: "cycle2_b",
		},
		"cycle2_b": {
			Type:      "command",
			OnSuccess: "cycle2_c",
		},
		"cycle2_c": {
			Type:      "command",
			OnSuccess: "cycle2_a", // Back to cycle2_a
			OnFailure: "terminal",
		},
		"terminal": {
			Type: "terminal",
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.NotEmpty(t, cycles, "Should detect multiple cycles")
	assert.GreaterOrEqual(t, len(cycles), 2, "Should detect at least 2 cycles")
}

// TestDetectCycles_Integration_SelfLoopCycle validates detection of self-referencing
// steps (a step that transitions to itself).
func TestDetectCycles_Integration_SelfLoopCycle(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "retry",
		},
		"retry": {
			Type:      "command",
			OnSuccess: "finish",
			OnFailure: "retry", // Self-loop
		},
		"finish": {
			Type: "terminal",
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.NotEmpty(t, cycles, "Should detect self-loop cycle")
	assert.Contains(t, cycles[0], "retry", "Cycle should include the retry step")
}

// TestDetectCycles_Integration_DeepNestedCycle validates cycle detection in
// deeply nested workflow structures (>10 levels).
func TestDetectCycles_Integration_DeepNestedCycle(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start":  {Type: "command", OnSuccess: "step1"},
		"step1":  {Type: "command", OnSuccess: "step2"},
		"step2":  {Type: "command", OnSuccess: "step3"},
		"step3":  {Type: "command", OnSuccess: "step4"},
		"step4":  {Type: "command", OnSuccess: "step5"},
		"step5":  {Type: "command", OnSuccess: "step6"},
		"step6":  {Type: "command", OnSuccess: "step7"},
		"step7":  {Type: "command", OnSuccess: "step8"},
		"step8":  {Type: "command", OnSuccess: "step9"},
		"step9":  {Type: "command", OnSuccess: "step10"},
		"step10": {Type: "command", OnSuccess: "step5"}, // Cycle back to step5
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.NotEmpty(t, cycles, "Should detect deep nested cycle")
}

// TestComputeExecutionOrder_Integration_EmptyGraph validates handling of
// empty workflow graphs.
func TestComputeExecutionOrder_Integration_EmptyGraph(t *testing.T) {
	tests := []struct {
		name    string
		steps   map[string]*workflow.Step
		initial string
	}{
		{
			name:    "nil steps map",
			steps:   nil,
			initial: "start",
		},
		{
			name:    "empty steps map",
			steps:   make(map[string]*workflow.Step),
			initial: "start",
		},
		{
			name: "empty initial state",
			steps: map[string]*workflow.Step{
				"start": {Type: "command"},
			},
			initial: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := workflow.ComputeExecutionOrder(tt.steps, tt.initial)

			require.NoError(t, err, "Empty graph should not return error")
			assert.Empty(t, order, "Empty graph should return empty order")
		})
	}
}

// TestComputeExecutionOrder_Integration_MaxBreadth validates handling of
// workflows with many parallel branches (stress test for enqueueIfNotVisited).
func TestComputeExecutionOrder_Integration_MaxBreadth(t *testing.T) {
	// Create a workflow with 20 parallel branches
	branches := make([]string, 20)
	steps := make(map[string]*workflow.Step)

	for i := 0; i < 20; i++ {
		branchName := "branch_" + string(rune('a'+i%26))
		branches[i] = branchName
		steps[branchName] = &workflow.Step{
			Type:      "command",
			OnSuccess: "merge",
		}
	}

	steps["start"] = &workflow.Step{
		Type:      "command",
		OnSuccess: "parallel",
	}
	steps["parallel"] = &workflow.Step{
		Type:      workflow.StepTypeParallel,
		Branches:  branches,
		OnSuccess: "merge",
	}
	steps["merge"] = &workflow.Step{
		Type:      "command",
		OnSuccess: "finish",
	}
	steps["finish"] = &workflow.Step{
		Type: "terminal",
	}

	order, err := workflow.ComputeExecutionOrder(steps, "start")

	require.NoError(t, err)
	assert.Greater(t, len(order), 20, "Should include all branches")

	// Verify parallel step comes before all branches
	parallelIdx := indexOf(order, "parallel")
	for _, branch := range branches {
		branchIdx := indexOf(order, branch)
		if branchIdx >= 0 {
			assert.Greater(t, branchIdx, parallelIdx, "Branch %s should come after parallel step", branch)
		}
	}
}

// TestDetectCycles_Integration_InvalidTransitions validates handling of
// transitions to non-existent states.
func TestDetectCycles_Integration_InvalidTransitions(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "nonexistent", // Invalid transition
			OnFailure: "error",
		},
		"error": {
			Type: "terminal",
		},
	}

	// Should handle gracefully without panicking
	cycles := workflow.DetectCycles(steps, "start")

	// cycles can be nil or empty slice, both are valid
	_ = cycles // Just verify it doesn't panic
}

// TestComputeExecutionOrder_Integration_InvalidInitialState validates handling
// of non-existent initial states.
func TestComputeExecutionOrder_Integration_InvalidInitialState(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "finish",
		},
		"finish": {
			Type: "terminal",
		},
	}

	order, err := workflow.ComputeExecutionOrder(steps, "nonexistent")

	require.NoError(t, err, "Invalid initial state should not error")
	assert.Empty(t, order, "Invalid initial state should return empty order")
}

// TestComputeExecutionOrder_Integration_CircularDependencies validates handling
// of workflows with cycles (which should be caught by DetectCycles first).
func TestComputeExecutionOrder_Integration_CircularDependencies(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Type:      "command",
			OnSuccess: "step_a",
		},
		"step_a": {
			Type:      "command",
			OnSuccess: "step_b",
		},
		"step_b": {
			Type:      "command",
			OnSuccess: "step_a", // Cycle
		},
	}

	// ComputeExecutionOrder should handle cycles gracefully
	order, err := workflow.ComputeExecutionOrder(steps, "start")

	require.NoError(t, err, "Should not panic on circular dependencies")

	// With visited tracking via enqueueIfNotVisited, should terminate
	assert.NotEmpty(t, order, "Should still produce some order")
	assert.Contains(t, order, "start")
}

// TestFullWorkflowValidation_Integration validates that DetectCycles and
// ComputeExecutionOrder work correctly together for complete workflow validation.
func TestFullWorkflowValidation_Integration(t *testing.T) {
	tests := []struct {
		name         string
		steps        map[string]*workflow.Step
		initial      string
		expectCycles bool
		expectOrder  bool
		minOrderLen  int
	}{
		{
			name: "valid complex workflow",
			steps: map[string]*workflow.Step{
				"start": {
					Type:      "command",
					OnSuccess: "validate",
					OnFailure: "error",
				},
				"validate": {
					Type:      "command",
					OnSuccess: "parallel_process",
					OnFailure: "error",
				},
				"parallel_process": {
					Type: workflow.StepTypeParallel,
					Branches: []string{
						"process_a",
						"process_b",
					},
					OnSuccess: "merge",
					OnFailure: "error",
				},
				"process_a": {
					Type:      "command",
					OnSuccess: "merge",
				},
				"process_b": {
					Type:      "command",
					OnSuccess: "merge",
				},
				"merge": {
					Type:      "command",
					OnSuccess: "finalize",
					OnFailure: "error",
				},
				"finalize": {
					Type:      "command",
					OnSuccess: "success",
					OnFailure: "error",
				},
				"success": {
					Type: "terminal",
				},
				"error": {
					Type: "terminal",
				},
			},
			initial:      "start",
			expectCycles: false,
			expectOrder:  true,
			minOrderLen:  5,
		},
		{
			name: "workflow with detected cycle",
			steps: map[string]*workflow.Step{
				"start": {
					Type:      "command",
					OnSuccess: "step_a",
				},
				"step_a": {
					Type:      "command",
					OnSuccess: "step_b",
				},
				"step_b": {
					Type:      "command",
					OnSuccess: "step_a", // Cycle
				},
			},
			initial:      "start",
			expectCycles: true,
			expectOrder:  true,
			minOrderLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First, detect cycles
			cycles := workflow.DetectCycles(tt.steps, tt.initial)

			if tt.expectCycles {
				assert.NotEmpty(t, cycles, "Should detect cycles")
			} else {
				assert.Empty(t, cycles, "Should not detect cycles")
			}

			// Then, compute execution order
			order, err := workflow.ComputeExecutionOrder(tt.steps, tt.initial)

			if tt.expectOrder {
				require.NoError(t, err)
				assert.GreaterOrEqual(t, len(order), tt.minOrderLen,
					"Execution order should have at least %d steps", tt.minOrderLen)

				// Verify initial state is first
				if len(order) > 0 {
					assert.Equal(t, tt.initial, order[0],
						"Initial state should be first in execution order")
				}
			}
		})
	}
}

// TestBackwardCompatibility_Integration validates that refactored algorithms
// maintain exact same behavior as before C003 refactoring.
func TestBackwardCompatibility_Integration(t *testing.T) {
	// This test uses known workflow patterns that existed before C003
	// to ensure backward compatibility
	knownWorkflows := []struct {
		name               string
		steps              map[string]*workflow.Step
		initial            string
		expectedCycleCount int
	}{
		{
			name: "simple linear (pre-C003)",
			steps: map[string]*workflow.Step{
				"init":    {Type: "command", OnSuccess: "process"},
				"process": {Type: "command", OnSuccess: "done"},
				"done":    {Type: "terminal"},
			},
			initial:            "init",
			expectedCycleCount: 0,
		},
		{
			name: "simple cycle (pre-C003)",
			steps: map[string]*workflow.Step{
				"a": {Type: "command", OnSuccess: "b"},
				"b": {Type: "command", OnSuccess: "a"},
			},
			initial:            "a",
			expectedCycleCount: 1,
		},
	}

	for _, wf := range knownWorkflows {
		t.Run(wf.name, func(t *testing.T) {
			cycles := workflow.DetectCycles(wf.steps, wf.initial)
			assert.Equal(t, wf.expectedCycleCount, len(cycles),
				"Cycle count should match pre-C003 behavior")

			order, err := workflow.ComputeExecutionOrder(wf.steps, wf.initial)
			require.NoError(t, err)

			if wf.expectedCycleCount == 0 {
				assert.NotEmpty(t, order, "Valid workflows should produce execution order")
			}
		})
	}
}

// indexOf returns the index of target in slice, or -1 if not found.
func indexOf(slice []string, target string) int {
	for i, s := range slice {
		if s == target {
			return i
		}
	}
	return -1
}
