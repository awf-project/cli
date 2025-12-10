package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// ValidateGraph Tests
// =============================================================================

func TestValidateGraph_ValidLinearWorkflow(t *testing.T) {
	// Linear: start -> middle -> done (terminal)
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo start",
			OnSuccess: "middle",
			OnFailure: "error",
		},
		"middle": {
			Name:      "middle",
			Type:      workflow.StepTypeCommand,
			Command:   "echo middle",
			OnSuccess: "done",
			OnFailure: "error",
		},
		"done": {
			Name:   "done",
			Type:   workflow.StepTypeTerminal,
			Status: workflow.TerminalSuccess,
		},
		"error": {
			Name:   "error",
			Type:   workflow.StepTypeTerminal,
			Status: workflow.TerminalFailure,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.False(t, result.HasErrors(), "valid workflow should have no errors")
	assert.False(t, result.HasWarnings(), "linear workflow should have no cycle warnings")
}

func TestValidateGraph_InvalidTransitionTarget(t *testing.T) {
	// on_success references a non-existent state
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "nonexistent", // does not exist
			OnFailure: "error",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
		"error": {
			Name: "error",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors(), "should have error for invalid transition")

	// Check error details
	found := false
	for _, err := range result.Errors {
		if err.Code == workflow.ErrInvalidTransition {
			found = true
			assert.Contains(t, err.Message, "nonexistent")
		}
	}
	assert.True(t, found, "should have ErrInvalidTransition error")
}

func TestValidateGraph_InvalidOnFailureTarget(t *testing.T) {
	// on_failure references a non-existent state
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "nonexistent", // does not exist
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors(), "should have error for invalid on_failure target")
}

func TestValidateGraph_UnreachableState(t *testing.T) {
	// "orphan" state is not reachable from "start"
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
		"orphan": {
			Name:      "orphan",
			Type:      workflow.StepTypeCommand,
			Command:   "echo unreachable",
			OnSuccess: "done",
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors(), "should detect unreachable state")

	found := false
	for _, err := range result.Errors {
		if err.Code == workflow.ErrUnreachableState {
			found = true
			assert.Contains(t, err.Message, "orphan")
		}
	}
	assert.True(t, found, "should have ErrUnreachableState error for 'orphan'")
}

func TestValidateGraph_SimpleCycleWarning(t *testing.T) {
	// A -> B -> A (simple cycle)
	// Cycles should be warnings, not errors
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "middle",
			OnFailure: "done",
		},
		"middle": {
			Name:      "middle",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "start", // cycle back to start
			OnFailure: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.False(t, result.HasErrors(), "cycles should not be errors")
	assert.True(t, result.HasWarnings(), "cycles should produce warnings")

	found := false
	for _, warn := range result.Warnings {
		if warn.Code == workflow.ErrCycleDetected {
			found = true
		}
	}
	assert.True(t, found, "should have ErrCycleDetected warning")
}

func TestValidateGraph_SelfLoopWarning(t *testing.T) {
	// A -> A (self loop)
	steps := map[string]*workflow.Step{
		"retry": {
			Name:      "retry",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "retry", // self loop on failure
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "retry")

	require.NotNil(t, result)
	assert.False(t, result.HasErrors(), "self-loop cycles should not be errors")
	assert.True(t, result.HasWarnings(), "self-loop should produce cycle warning")
}

func TestValidateGraph_NoCycle(t *testing.T) {
	// Diamond pattern: start -> A, start -> B, A -> done, B -> done
	// No cycles
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "pathA",
			OnFailure: "pathB",
		},
		"pathA": {
			Name:      "pathA",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "done",
		},
		"pathB": {
			Name:      "pathB",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.False(t, result.HasErrors())
	assert.False(t, result.HasWarnings(), "diamond pattern should have no cycles")
}

func TestValidateGraph_MultipleUnreachableStates(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
		"orphan1": {
			Name:      "orphan1",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "orphan2",
		},
		"orphan2": {
			Name: "orphan2",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors())

	// Should report both orphan states
	unreachableCount := 0
	for _, err := range result.Errors {
		if err.Code == workflow.ErrUnreachableState {
			unreachableCount++
		}
	}
	assert.Equal(t, 2, unreachableCount, "should detect both orphan states")
}

func TestValidateGraph_ComplexCycle(t *testing.T) {
	// A -> B -> C -> A (longer cycle)
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "step2",
			OnFailure: "error",
		},
		"step2": {
			Name:      "step2",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "step3",
			OnFailure: "error",
		},
		"step3": {
			Name:      "step3",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "start", // cycle back
			OnFailure: "error",
		},
		"error": {
			Name: "error",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.False(t, result.HasErrors())
	assert.True(t, result.HasWarnings(), "should detect cycle")
}

func TestValidateGraph_ParallelBranches(t *testing.T) {
	// Parallel step with multiple branches
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeParallel,
			Branches:  []string{"branch1", "branch2", "branch3"},
			OnSuccess: "done",
			OnFailure: "error",
		},
		"branch1": {
			Name:      "branch1",
			Type:      workflow.StepTypeCommand,
			Command:   "echo 1",
			OnSuccess: "done",
		},
		"branch2": {
			Name:      "branch2",
			Type:      workflow.StepTypeCommand,
			Command:   "echo 2",
			OnSuccess: "done",
		},
		"branch3": {
			Name:      "branch3",
			Type:      workflow.StepTypeCommand,
			Command:   "echo 3",
			OnSuccess: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
		"error": {
			Name: "error",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.False(t, result.HasErrors(), "parallel branches should be valid")
}

func TestValidateGraph_ParallelInvalidBranch(t *testing.T) {
	// Parallel step referencing non-existent branch
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeParallel,
			Branches:  []string{"branch1", "nonexistent"},
			OnSuccess: "done",
		},
		"branch1": {
			Name:      "branch1",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors(), "should detect invalid branch reference")
}

func TestValidateGraph_EmptySteps(t *testing.T) {
	steps := map[string]*workflow.Step{}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors(), "empty steps should be an error")
}

func TestValidateGraph_MissingInitialState(t *testing.T) {
	steps := map[string]*workflow.Step{
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	result := workflow.ValidateGraph(steps, "nonexistent")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors())

	found := false
	for _, err := range result.Errors {
		if err.Code == workflow.ErrMissingInitialState {
			found = true
		}
	}
	assert.True(t, found, "should have ErrMissingInitialState error")
}

func TestValidateGraph_NoTerminalState(t *testing.T) {
	// No terminal states - infinite loop potential
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "step2",
		},
		"step2": {
			Name:      "step2",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "start", // cycles forever
		},
	}

	result := workflow.ValidateGraph(steps, "start")

	require.NotNil(t, result)
	assert.True(t, result.HasErrors(), "should detect no terminal state")

	found := false
	for _, err := range result.Errors {
		if err.Code == workflow.ErrNoTerminalState {
			found = true
		}
	}
	assert.True(t, found, "should have ErrNoTerminalState error")
}

// =============================================================================
// findReachableStates Tests
// =============================================================================

func TestFindReachableStates_AllReachable(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "middle",
			OnFailure: "error",
		},
		"middle": {
			Name:      "middle",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "error",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
		"error": {
			Name: "error",
			Type: workflow.StepTypeTerminal,
		},
	}

	reachable := workflow.FindReachableStates(steps, "start")

	assert.Len(t, reachable, 4)
	assert.True(t, reachable["start"])
	assert.True(t, reachable["middle"])
	assert.True(t, reachable["done"])
	assert.True(t, reachable["error"])
}

func TestFindReachableStates_WithOrphans(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
		"orphan": {
			Name:      "orphan",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
		},
	}

	reachable := workflow.FindReachableStates(steps, "start")

	assert.Len(t, reachable, 2)
	assert.True(t, reachable["start"])
	assert.True(t, reachable["done"])
	assert.False(t, reachable["orphan"])
}

func TestFindReachableStates_InvalidInitial(t *testing.T) {
	steps := map[string]*workflow.Step{
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	reachable := workflow.FindReachableStates(steps, "nonexistent")

	assert.Empty(t, reachable)
}

// =============================================================================
// detectCycles Tests
// =============================================================================

func TestDetectCycles_NoCycle(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.Empty(t, cycles)
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "middle",
		},
		"middle": {
			Name:      "middle",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "start", // back edge
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.NotEmpty(t, cycles)
}

func TestDetectCycles_SelfLoop(t *testing.T) {
	steps := map[string]*workflow.Step{
		"retry": {
			Name:      "retry",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "retry", // self loop
		},
		"done": {
			Name: "done",
			Type: workflow.StepTypeTerminal,
		},
	}

	cycles := workflow.DetectCycles(steps, "retry")

	assert.NotEmpty(t, cycles, "should detect self-loop")
}

func TestDetectCycles_MultipleCycles(t *testing.T) {
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "pathA",
			OnFailure: "pathB",
		},
		"pathA": {
			Name:      "pathA",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "start", // cycle 1
		},
		"pathB": {
			Name:      "pathB",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "pathB", // cycle 2 (self loop)
		},
	}

	cycles := workflow.DetectCycles(steps, "start")

	assert.NotEmpty(t, cycles, "should detect cycles")
}

// =============================================================================
// getTransitions Tests
// =============================================================================

func TestGetTransitions_CommandStep(t *testing.T) {
	step := &workflow.Step{
		Name:      "cmd",
		Type:      workflow.StepTypeCommand,
		Command:   "echo",
		OnSuccess: "next",
		OnFailure: "error",
	}

	transitions := workflow.GetTransitions(step)

	assert.Contains(t, transitions, "next")
	assert.Contains(t, transitions, "error")
	assert.Len(t, transitions, 2)
}

func TestGetTransitions_CommandStepOnlySuccess(t *testing.T) {
	step := &workflow.Step{
		Name:      "cmd",
		Type:      workflow.StepTypeCommand,
		Command:   "echo",
		OnSuccess: "next",
	}

	transitions := workflow.GetTransitions(step)

	assert.Contains(t, transitions, "next")
	assert.Len(t, transitions, 1)
}

func TestGetTransitions_TerminalStep(t *testing.T) {
	step := &workflow.Step{
		Name: "done",
		Type: workflow.StepTypeTerminal,
	}

	transitions := workflow.GetTransitions(step)

	assert.Empty(t, transitions)
}

func TestGetTransitions_ParallelStep(t *testing.T) {
	step := &workflow.Step{
		Name:      "parallel",
		Type:      workflow.StepTypeParallel,
		Branches:  []string{"branch1", "branch2", "branch3"},
		OnSuccess: "next",
		OnFailure: "error",
	}

	transitions := workflow.GetTransitions(step)

	// Should include branches + on_success + on_failure
	assert.Contains(t, transitions, "branch1")
	assert.Contains(t, transitions, "branch2")
	assert.Contains(t, transitions, "branch3")
	assert.Contains(t, transitions, "next")
	assert.Contains(t, transitions, "error")
	assert.Len(t, transitions, 5)
}

// =============================================================================
// ValidationResult Tests
// =============================================================================

func TestValidationResult_ToError(t *testing.T) {
	tests := []struct {
		name      string
		result    *workflow.ValidationResult
		wantError bool
	}{
		{
			name:      "no errors",
			result:    &workflow.ValidationResult{},
			wantError: false,
		},
		{
			name: "only warnings",
			result: &workflow.ValidationResult{
				Warnings: []workflow.ValidationError{
					{Level: workflow.ValidationLevelWarning, Code: workflow.ErrCycleDetected, Message: "cycle found"},
				},
			},
			wantError: false,
		},
		{
			name: "with errors",
			result: &workflow.ValidationResult{
				Errors: []workflow.ValidationError{
					{Level: workflow.ValidationLevelError, Code: workflow.ErrUnreachableState, Message: "orphan state"},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.ToError()
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      workflow.ValidationError
		contains string
	}{
		{
			name: "with path",
			err: workflow.ValidationError{
				Level:   workflow.ValidationLevelError,
				Code:    workflow.ErrInvalidTransition,
				Message: "unknown state 'foo'",
				Path:    "states.start.on_success",
			},
			contains: "states.start.on_success",
		},
		{
			name: "without path",
			err: workflow.ValidationError{
				Level:   workflow.ValidationLevelWarning,
				Code:    workflow.ErrCycleDetected,
				Message: "cycle detected",
			},
			contains: "cycle detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			assert.Contains(t, errStr, tt.contains)
		})
	}
}
