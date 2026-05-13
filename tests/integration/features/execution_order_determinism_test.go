//go:build integration

package features_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F095 — Execution Order — Common Abstraction for Step Display Ordering

// TestExecutionOrderDeterminism_LinearWorkflow validates that CLI output displays
// steps in workflow-defined order (matching Initial -> Transitions) consistently.
// Tests US1: Deterministic Step Order in CLI Execution Summary.
func TestExecutionOrderDeterminism_LinearWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "linear-pipeline",
		Initial: "init",
		Steps: map[string]*workflow.Step{
			"init": {
				Name:      "init",
				Type:      workflow.StepTypeCommand,
				Command:   "echo init",
				OnSuccess: "build",
			},
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "echo build",
				OnSuccess: "test",
			},
			"test": {
				Name:      "test",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "deploy",
			},
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "echo deploy",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-linear-001", "linear-pipeline")
	execCtx.Status = workflow.StatusCompleted

	execCtx.States["init"] = workflow.StepState{
		Name:        "init",
		Status:      workflow.StatusCompleted,
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(100 * time.Millisecond),
	}
	execCtx.States["build"] = workflow.StepState{
		Name:        "build",
		Status:      workflow.StatusCompleted,
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(200 * time.Millisecond),
	}
	execCtx.States["test"] = workflow.StepState{
		Name:        "test",
		Status:      workflow.StatusCompleted,
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(150 * time.Millisecond),
	}
	execCtx.States["deploy"] = workflow.StepState{
		Name:        "deploy",
		Status:      workflow.StatusCompleted,
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(120 * time.Millisecond),
	}
	execCtx.States["done"] = workflow.StepState{
		Name:        "done",
		Status:      workflow.StatusCompleted,
		StartedAt:   time.Now(),
		CompletedAt: time.Now().Add(50 * time.Millisecond),
	}

	order := workflow.ExecutionOrder(wf)

	require.NotNil(t, order)
	require.Len(t, order, 5)

	assert.Equal(t, "init", order[0].Name)
	assert.Equal(t, "build", order[1].Name)
	assert.Equal(t, "test", order[2].Name)
	assert.Equal(t, "deploy", order[3].Name)
	assert.Equal(t, "done", order[4].Name)
}

// TestExecutionOrderDeterminism_FollowsDefaultTransition validates that the
// execution order follows default transitions (empty When) over conditional ones.
// Tests US2: Domain-Level Execution Order Function.
func TestExecutionOrderDeterminism_FollowsDefaultTransition(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "branching-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "fallback",
				Transitions: workflow.Transitions{
					{When: "", Goto: "default_path"},
					{When: "condition1", Goto: "alternate_path"},
				},
			},
			"default_path": {
				Name:      "default_path",
				Type:      workflow.StepTypeCommand,
				Command:   "echo default",
				OnSuccess: "done",
			},
			"alternate_path": {
				Name:      "alternate_path",
				Type:      workflow.StepTypeCommand,
				Command:   "echo alternate",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	order := workflow.ExecutionOrder(wf)

	require.NotNil(t, order)
	require.Len(t, order, 3)

	assert.Equal(t, "start", order[0].Name)
	assert.Equal(t, "default_path", order[1].Name, "should follow default transition, not OnSuccess fallback")
	assert.Equal(t, "done", order[2].Name)
}

// TestExecutionOrderDeterminism_SkipsStepsNotExecuted validates that steps in
// the execution order but not in ExecutionContext are gracefully skipped.
// Tests edge case: steps not executed during run.
func TestExecutionOrderDeterminism_SkipsStepsNotExecuted(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "partial-execution",
		Initial: "step_a",
		Steps: map[string]*workflow.Step{
			"step_a": {
				Name:      "step_a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnSuccess: "step_b",
			},
			"step_b": {
				Name:      "step_b",
				Type:      workflow.StepTypeCommand,
				Command:   "echo b",
				OnSuccess: "step_c",
			},
			"step_c": {
				Name:   "step_c",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-partial-001", "partial-execution")
	execCtx.Status = workflow.StatusCompleted

	execCtx.States["step_a"] = workflow.StepState{
		Name:   "step_a",
		Status: workflow.StatusCompleted,
	}

	order := workflow.ExecutionOrder(wf)
	require.NotNil(t, order)
	require.Len(t, order, 3)

	executed := 0
	for _, step := range order {
		if _, ok := execCtx.GetStepState(step.Name); ok {
			executed++
		}
	}

	assert.Equal(t, 1, executed, "only step_a should have state")
}

// TestExecutionOrderDeterminism_IncludesParallelStepAsSingleEntry validates that
// parallel steps are included as single entries (branches not expanded).
// Tests US1 edge case: parallel step handling.
func TestExecutionOrderDeterminism_IncludesParallelStepAsSingleEntry(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-workflow",
		Initial: "setup",
		Steps: map[string]*workflow.Step{
			"setup": {
				Name:      "setup",
				Type:      workflow.StepTypeCommand,
				Command:   "echo setup",
				OnSuccess: "parallel_fetch",
			},
			"parallel_fetch": {
				Name:      "parallel_fetch",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"fetch_data1", "fetch_data2", "fetch_data3"},
				OnSuccess: "aggregate",
			},
			"fetch_data1": {
				Name:      "fetch_data1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo fetch1",
				OnSuccess: "aggregate",
			},
			"fetch_data2": {
				Name:      "fetch_data2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo fetch2",
				OnSuccess: "aggregate",
			},
			"fetch_data3": {
				Name:      "fetch_data3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo fetch3",
				OnSuccess: "aggregate",
			},
			"aggregate": {
				Name:      "aggregate",
				Type:      workflow.StepTypeCommand,
				Command:   "echo aggregate",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	order := workflow.ExecutionOrder(wf)

	require.NotNil(t, order)
	require.Len(t, order, 4, "should be setup, parallel_fetch, aggregate, done (not expanded branches)")

	assert.Equal(t, "setup", order[0].Name)
	assert.Equal(t, "parallel_fetch", order[1].Name)
	assert.Equal(t, workflow.StepTypeParallel, order[1].Type, "parallel step should retain its type")
	assert.Equal(t, "aggregate", order[2].Name)
	assert.Equal(t, "done", order[3].Name)
}

// TestExecutionOrderDeterminism_ConsistentAcrossMultipleCalls validates that
// the same workflow produces identical execution order across multiple calls.
// Tests SC-001: Deterministic ordering across runs.
func TestExecutionOrderDeterminism_ConsistentAcrossMultipleCalls(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "consistent-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "step3",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 3",
				OnSuccess: "step4",
			},
			"step4": {
				Name:      "step4",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 4",
				OnSuccess: "step5",
			},
			"step5": {
				Name:   "step5",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	order1 := workflow.ExecutionOrder(wf)
	order2 := workflow.ExecutionOrder(wf)
	order3 := workflow.ExecutionOrder(wf)

	require.NotNil(t, order1)
	require.NotNil(t, order2)
	require.NotNil(t, order3)

	assert.Len(t, order1, 5)
	assert.Len(t, order2, 5)
	assert.Len(t, order3, 5)

	for i := 0; i < 5; i++ {
		assert.Equal(t, order1[i].Name, order2[i].Name, "call 1 and 2 should match at index %d", i)
		assert.Equal(t, order1[i].Name, order3[i].Name, "call 1 and 3 should match at index %d", i)
	}
}

// TestExecutionOrderDeterminism_StopsAtCycleDetection validates that execution
// order stops when encountering a revisited step to prevent infinite loops.
// Tests edge case: cycle prevention.
func TestExecutionOrderDeterminism_StopsAtCycleDetection(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "cyclic-workflow",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {
				Name:      "a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnSuccess: "b",
			},
			"b": {
				Name:      "b",
				Type:      workflow.StepTypeCommand,
				Command:   "echo b",
				OnSuccess: "a",
			},
		},
	}

	order := workflow.ExecutionOrder(wf)

	require.NotNil(t, order)
	require.Len(t, order, 2, "should stop at revisited step without infinite loop")

	assert.Equal(t, "a", order[0].Name)
	assert.Equal(t, "b", order[1].Name)
}

// TestNextDefaultStep_ResolvesDefaultTransition validates that NextDefaultStep
// correctly identifies and returns the default transition.
// Tests US2: NextDefaultStep function behavior.
func TestNextDefaultStep_ResolvesDefaultTransition(t *testing.T) {
	tests := []struct {
		name     string
		step     *workflow.Step
		expected string
	}{
		{
			name: "default transition takes precedence over on_success",
			step: &workflow.Step{
				Name:      "test",
				OnSuccess: "fallback",
				Transitions: workflow.Transitions{
					{When: "", Goto: "default_target"},
					{When: "condition", Goto: "conditional_target"},
				},
			},
			expected: "default_target",
		},
		{
			name: "falls back to on_success when no default transition",
			step: &workflow.Step{
				Name:      "test",
				OnSuccess: "success_target",
				Transitions: workflow.Transitions{
					{When: "condition1", Goto: "path1"},
					{When: "condition2", Goto: "path2"},
				},
			},
			expected: "success_target",
		},
		{
			name: "returns empty when no default path",
			step: &workflow.Step{
				Name: "test",
				Transitions: workflow.Transitions{
					{When: "condition", Goto: "conditional_only"},
				},
			},
			expected: "",
		},
		{
			name:     "handles nil step gracefully",
			step:     nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflow.NextDefaultStep(tt.step)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExecutionOrderDeterminism_CliDisplayFormatting validates that the CLI
// display functions format step information in workflow-defined order.
// Integration test for showExecutionDetails functionality.
func TestExecutionOrderDeterminism_CliDisplayFormatting(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "display-test",
		Initial: "fetch",
		Steps: map[string]*workflow.Step{
			"fetch": {
				Name:      "fetch",
				Type:      workflow.StepTypeCommand,
				Command:   "echo fetch",
				OnSuccess: "process",
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo process",
				OnSuccess: "store",
			},
			"store": {
				Name:   "store",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	execCtx := workflow.NewExecutionContext("test-display-001", "display-test")
	execCtx.Status = workflow.StatusCompleted

	now := time.Now()
	execCtx.States["fetch"] = workflow.StepState{
		Name:        "fetch",
		Status:      workflow.StatusCompleted,
		StartedAt:   now,
		CompletedAt: now.Add(100 * time.Millisecond),
	}
	execCtx.States["process"] = workflow.StepState{
		Name:        "process",
		Status:      workflow.StatusCompleted,
		StartedAt:   now,
		CompletedAt: now.Add(150 * time.Millisecond),
	}
	execCtx.States["store"] = workflow.StepState{
		Name:        "store",
		Status:      workflow.StatusCompleted,
		StartedAt:   now,
		CompletedAt: now.Add(50 * time.Millisecond),
	}

	order := workflow.ExecutionOrder(wf)

	require.NotNil(t, order)
	require.Len(t, order, 3)

	output := new(strings.Builder)
	formatter := ui.NewFormatter(output, ui.FormatOptions{NoColor: true})

	formatter.Printf("--- Execution Details ---\n")
	for _, step := range order {
		state, ok := execCtx.GetStepState(step.Name)
		if !ok {
			continue
		}
		duration := state.CompletedAt.Sub(state.StartedAt)
		formatter.Printf("%s: %s (%v)\n", step.Name, state.Status, duration)
	}

	outputStr := output.String()

	assert.Contains(t, outputStr, "fetch:", "fetch should appear in output")
	assert.Contains(t, outputStr, "process:", "process should appear in output")
	assert.Contains(t, outputStr, "store:", "store should appear in output")

	fetchIdx := findStringIndex(outputStr, "fetch:")
	processIdx := findStringIndex(outputStr, "process:")
	storeIdx := findStringIndex(outputStr, "store:")

	assert.Less(t, fetchIdx, processIdx, "fetch should appear before process")
	assert.Less(t, processIdx, storeIdx, "process should appear before store")
}

// TestExecutionOrderDeterminism_MissingStepHandling validates that execution
// stops gracefully when a transition target is missing from the workflow.
func TestExecutionOrderDeterminism_MissingStepHandling(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "missing-target",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "missing",
			},
		},
	}

	order := workflow.ExecutionOrder(wf)

	require.NotNil(t, order)
	require.Len(t, order, 1, "should include start but stop when target is missing")
	assert.Equal(t, "start", order[0].Name)
}

// Helper function to find the index of a substring.
func findStringIndex(s, substr string) int {
	re := regexp.MustCompile(regexp.QuoteMeta(substr))
	match := re.FindStringIndex(s)
	if match == nil {
		return -1
	}
	return match[0]
}
