package workflow_test

// C013: Domain test file splitting
// Source: internal/domain/workflow/template_validation_test.go
// Test count: 22 tests
// Focus: states.* namespace - State reference validation and execution order tests

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateValidator_ValidStateReference(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "state-ref-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo result: {{states.step1.Output}}", // valid: step1 runs before step2
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
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "referencing previous step output should be valid")
}

func TestTemplateValidator_ValidStateReferenceAllProperties(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "state-props-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.Output}} {{states.step1.Stderr}} {{states.step1.ExitCode}} {{states.step1.Status}}",
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
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "all valid state properties should pass")
}

func TestTemplateValidator_UndefinedStep(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{states.nonexistent.Output}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrUndefinedStep, result.Errors[0].Code)
}

func TestTemplateValidator_ForwardReference(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "forward-ref-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step2.output}}", // ERROR: step2 runs AFTER step1
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo second",
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
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrForwardReference, result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "step2")
}

func TestTemplateValidator_ForwardReferenceMultipleStepsAhead(t *testing.T) {
	w := newLinearWorkflow()
	// step1 tries to reference step3 which is 2 steps ahead
	w.Steps["step1"].Command = "echo {{states.step3.output}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrForwardReference, result.Errors[0].Code)
}

func TestTemplateValidator_SelfReference(t *testing.T) {
	// A step referencing its own output (which doesn't exist yet)
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{states.start.Output}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrForwardReference, result.Errors[0].Code)
}

func TestTemplateValidator_InvalidStateProperty(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "invalid-prop-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo first",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.invalid_property}}", // invalid property
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
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrInvalidStateProperty, result.Errors[0].Code)
}

func TestTemplateValidator_InvalidStatePropertyCommonMistakes(t *testing.T) {
	tests := []struct {
		name     string
		property string
	}{
		{"stdout instead of output", "stdout"},
		{"result instead of output", "result"},
		{"exitcode without underscore", "exitcode"},
		{"code instead of exit_code", "code"},
		{"err instead of stderr", "err"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &workflow.Workflow{
				Name:    "test",
				Initial: "step1",
				Steps: map[string]*workflow.Step{
					"step1": {
						Name:      "step1",
						Type:      workflow.StepTypeCommand,
						Command:   "echo test",
						OnSuccess: "step2",
						OnFailure: "error",
					},
					"step2": {
						Name:      "step2",
						Type:      workflow.StepTypeCommand,
						Command:   "echo {{states.step1." + tt.property + "}}",
						OnSuccess: "done",
						OnFailure: "error",
					},
					"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
					"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
				},
			}

			v := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := v.Validate()

			require.True(t, result.HasErrors())
			assert.Equal(t, workflow.ErrInvalidStateProperty, result.Errors[0].Code)
		})
	}
}

// TestValidateStateRef_CasingErrors verifies that lowercase state property references
// are detected and emit errors with uppercase suggestions (F050).
func TestValidateStateRef_CasingErrors(t *testing.T) {
	tests := []struct {
		name           string
		property       string
		wantError      bool
		wantSuggestion string
	}{
		{"lowercase output errors", "output", true, "Output"},
		{"lowercase stderr errors", "stderr", true, "Stderr"},
		{"lowercase exit_code errors", "exit_code", true, "ExitCode"},
		{"lowercase status errors", "status", true, "Status"},
		{"uppercase Output passes", "Output", false, ""},
		{"uppercase Stderr passes", "Stderr", false, ""},
		{"uppercase ExitCode passes", "ExitCode", false, ""},
		{"uppercase Status passes", "Status", false, ""},
		{"invalid property no suggestion", "stdout", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &workflow.Workflow{
				Name:    "test",
				Initial: "step1",
				Steps: map[string]*workflow.Step{
					"step1": {
						Name:      "step1",
						Type:      workflow.StepTypeCommand,
						Command:   "echo",
						OnSuccess: "step2",
					},
					"step2": {
						Name:      "step2",
						Type:      workflow.StepTypeCommand,
						Command:   "echo {{states.step1." + tt.property + "}}",
						OnSuccess: "done",
					},
					"done": {
						Name:   "done",
						Type:   workflow.StepTypeTerminal,
						Status: workflow.TerminalSuccess,
					},
				},
			}

			validator := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := validator.Validate()

			if tt.wantError {
				require.True(t, result.HasErrors(), "expected error for property: %s", tt.property)
				if tt.wantSuggestion != "" {
					assert.Contains(t, result.Errors[0].Message, tt.wantSuggestion)
				}
			} else {
				assert.False(t, result.HasErrors(), "unexpected error for property: %s", tt.property)
			}
		})
	}
}

func TestTemplateValidator_StateRefWithoutProperty(t *testing.T) {
	// Reference without the property (e.g., {{states.step1}} instead of {{states.step1.output}})
	w := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1}}", // Missing property
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should error - missing property
	require.True(t, result.HasErrors())
}

func TestComputeExecutionOrder_LinearWorkflow(t *testing.T) {
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

	order, err := workflow.ComputeExecutionOrder(steps, "start")

	require.NoError(t, err)
	require.NotEmpty(t, order)

	// start should come before middle
	startIdx := indexOf(order, "start")
	middleIdx := indexOf(order, "middle")

	require.GreaterOrEqual(t, startIdx, 0, "start should be in order")
	require.GreaterOrEqual(t, middleIdx, 0, "middle should be in order")
	assert.Less(t, startIdx, middleIdx, "start should come before middle")
}

func TestComputeExecutionOrder_LongerChain(t *testing.T) {
	steps := map[string]*workflow.Step{
		"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "step2"},
		"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "step3"},
		"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "step4"},
		"step4": {Name: "step4", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
		"done":  {Name: "done", Type: workflow.StepTypeTerminal},
	}

	order, err := workflow.ComputeExecutionOrder(steps, "step1")

	require.NoError(t, err)

	// Verify order: step1 < step2 < step3 < step4
	for i := 1; i <= 3; i++ {
		stepN := indexOf(order, "step"+string(rune('0'+i)))
		stepN1 := indexOf(order, "step"+string(rune('0'+i+1)))
		if stepN >= 0 && stepN1 >= 0 {
			assert.Less(t, stepN, stepN1, "step%d should come before step%d", i, i+1)
		}
	}
}

func TestComputeExecutionOrder_EmptySteps(t *testing.T) {
	order, err := workflow.ComputeExecutionOrder(map[string]*workflow.Step{}, "start")

	// Should handle gracefully - either error or empty result
	if err == nil {
		assert.Empty(t, order)
	}
}

func TestComputeExecutionOrder_BranchingWorkflow(t *testing.T) {
	// Diamond pattern with conditional paths
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "pathA",
			OnFailure: "pathB",
		},
		"pathA": {Name: "pathA", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
		"pathB": {Name: "pathB", Type: workflow.StepTypeCommand, Command: "echo", OnSuccess: "done"},
		"done":  {Name: "done", Type: workflow.StepTypeTerminal},
	}

	order, err := workflow.ComputeExecutionOrder(steps, "start")

	require.NoError(t, err)
	require.NotEmpty(t, order)

	// start should be first
	startIdx := indexOf(order, "start")
	assert.Equal(t, 0, startIdx, "start should be first in order")
}

func TestComputeExecutionOrder_WithCycles(t *testing.T) {
	// Cycles are allowed (loops) - should still compute a valid order
	steps := map[string]*workflow.Step{
		"start": {
			Name:      "start",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "retry",
			OnFailure: "error",
		},
		"retry": {
			Name:      "retry",
			Type:      workflow.StepTypeCommand,
			Command:   "echo",
			OnSuccess: "done",
			OnFailure: "retry", // self-loop on failure
		},
		"done":  {Name: "done", Type: workflow.StepTypeTerminal},
		"error": {Name: "error", Type: workflow.StepTypeTerminal},
	}

	order, err := workflow.ComputeExecutionOrder(steps, "start")

	// Should not error - cycles are valid in workflows
	require.NoError(t, err)
	require.NotEmpty(t, order)
}

func TestTemplateValidator_ParallelStepBranchRefs(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "parallel-test",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				Strategy:  "all_succeed",
				OnSuccess: "merge",
				OnFailure: "error",
			},
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "merge"},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand, Command: "echo 2", OnSuccess: "merge"},
			"merge": {
				Name:      "merge",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.branch1.Output}} {{states.branch2.Output}}", // Valid: branches run before merge
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "referencing parallel branch outputs after merge should be valid")
}

func TestTemplateValidator_ParallelBranchRefBeforeCompletion(t *testing.T) {
	// One branch trying to reference another branch's output during parallel execution
	// This depends on strategy - for all_succeed, branches run concurrently
	w := &workflow.Workflow{
		Name:    "parallel-cross-ref",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				Strategy:  "all_succeed",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand, Command: "echo 1", OnSuccess: "done"},
			"branch2": {
				Name:      "branch2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.branch1.output}}", // May be invalid: branch1 might not be done yet
				OnSuccess: "done",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// This is a potential forward reference issue - branches run concurrently
	// The validator should detect this
	require.True(t, result.HasErrors() || result.HasWarnings(),
		"cross-referencing parallel branches should be flagged")
}
