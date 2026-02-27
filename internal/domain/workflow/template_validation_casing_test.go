package workflow_test

import (
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: Validate that state property references use correct uppercase casing

// TestValidateStateRef_LowercaseCasingErrors tests that lowercase state
// properties are detected and emit errors with uppercase suggestions.
// This is the core test for F050 - validating the breaking change that enforces
// uppercase property names matching Go's export conventions.
func TestValidateStateRef_LowercaseCasingErrors(t *testing.T) {
	tests := []struct {
		name              string
		property          string
		wantError         bool
		wantSuggestion    string
		wantErrorContains string
	}{
		{
			name:              "lowercase output should error with Output suggestion",
			property:          "output",
			wantError:         true,
			wantSuggestion:    "Output",
			wantErrorContains: "use 'Output' instead",
		},
		{
			name:              "lowercase stderr should error with Stderr suggestion",
			property:          "stderr",
			wantError:         true,
			wantSuggestion:    "Stderr",
			wantErrorContains: "use 'Stderr' instead",
		},
		{
			name:              "lowercase exit_code should error with ExitCode suggestion",
			property:          "exit_code",
			wantError:         true,
			wantSuggestion:    "ExitCode",
			wantErrorContains: "use 'ExitCode' instead",
		},
		{
			name:              "lowercase status should error with Status suggestion",
			property:          "status",
			wantError:         true,
			wantSuggestion:    "Status",
			wantErrorContains: "use 'Status' instead",
		},
		{
			name:              "uppercase Output should pass",
			property:          "Output",
			wantError:         false,
			wantSuggestion:    "",
			wantErrorContains: "",
		},
		{
			name:              "uppercase Stderr should pass",
			property:          "Stderr",
			wantError:         false,
			wantSuggestion:    "",
			wantErrorContains: "",
		},
		{
			name:              "uppercase ExitCode should pass",
			property:          "ExitCode",
			wantError:         false,
			wantSuggestion:    "",
			wantErrorContains: "",
		},
		{
			name:              "uppercase Status should pass",
			property:          "Status",
			wantError:         false,
			wantSuggestion:    "",
			wantErrorContains: "",
		},
		{
			name:              "invalid property 'stdout' should error without suggestion",
			property:          "stdout",
			wantError:         true,
			wantSuggestion:    "",
			wantErrorContains: "invalid state property",
		},
		{
			name:              "invalid property 'result' should error without suggestion",
			property:          "result",
			wantError:         true,
			wantSuggestion:    "",
			wantErrorContains: "invalid state property",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &workflow.Workflow{
				Name:    "casing-casing-test",
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
						Command:   "echo {{states.step1." + tt.property + "}}",
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

			if tt.wantError {
				require.True(t, result.HasErrors(), "Expected validation error for property %q", tt.property)
				assert.Equal(t, workflow.ErrInvalidStateProperty, result.Errors[0].Code)

				errorMsg := result.Errors[0].Message
				assert.Contains(t, errorMsg, tt.wantErrorContains,
					"Error message should contain %q for property %q", tt.wantErrorContains, tt.property)

				if tt.wantSuggestion != "" {
					assert.Contains(t, errorMsg, tt.wantSuggestion,
						"Error message should suggest %q for property %q", tt.wantSuggestion, tt.property)
				}
			} else {
				assert.False(t, result.HasErrors(),
					"Expected no validation errors for valid uppercase property %q", tt.property)
			}
		})
	}
}

// TestValidateStateRef_LowercaseCasingInLoopConditions tests that lowercase
// properties in loop conditions are detected.
func TestValidateStateRef_LowercaseCasingInLoopConditions(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "casing-loop-casing-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo countdown",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"loop_step": {
				Name:    "loop_step",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{loop.Index}}",
				Loop: &workflow.LoopConfig{
					Condition: "{{states.step1.exit_code}} == 0", // lowercase exit_code
				},
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

	require.True(t, result.HasErrors(), "Expected error for lowercase exit_code in loop condition")
	assert.Equal(t, workflow.ErrInvalidStateProperty, result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "use 'ExitCode' instead")
}

// TestValidateStateRef_UppercaseCasingInLoopConditions tests that uppercase
// properties in loop conditions pass validation.
func TestValidateStateRef_UppercaseCasingInLoopConditions(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "casing-loop-uppercase-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo countdown",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"loop_step": {
				Name:    "loop_step",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{loop.Index}}",
				Loop: &workflow.LoopConfig{
					Condition: "{{states.step1.ExitCode}} == 0", // uppercase ExitCode
				},
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

	assert.False(t, result.HasErrors(), "Expected no errors for uppercase ExitCode in loop condition")
}

// TestValidateStateRef_MixedCasing tests workflows with both correct
// and incorrect casing to ensure all errors are reported.
func TestValidateStateRef_MixedCasing(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "mixed-casing-test",
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
				Command:   "echo {{states.step1.output}} {{states.step1.Stderr}}", // mixed: lowercase output, uppercase Stderr
				OnSuccess: "step3",
				OnFailure: "error",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.exit_code}} {{states.step2.Output}}", // mixed: lowercase exit_code, uppercase Output
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

	// Should have 2 errors: one for lowercase "output", one for lowercase "exit_code"
	require.True(t, result.HasErrors())
	assert.GreaterOrEqual(t, len(result.Errors), 2, "Expected at least 2 errors for mixed casing")

	errorMessages := make([]string, len(result.Errors))
	for i, err := range result.Errors {
		errorMessages[i] = err.Message
	}
	combinedErrors := strings.Join(errorMessages, " | ")

	assert.Contains(t, combinedErrors, "use 'Output' instead")
	assert.Contains(t, combinedErrors, "use 'ExitCode' instead")
}

// TestValidateStateRef_AllUppercaseProperties tests that all four
// uppercase properties pass validation without errors.
func TestValidateStateRef_AllUppercaseProperties(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "casing-all-uppercase-test",
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
				Command:   "echo {{states.step1.Output}}",
				OnSuccess: "step3",
				OnFailure: "error",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.Stderr}}",
				OnSuccess: "step4",
				OnFailure: "error",
			},
			"step4": {
				Name:      "step4",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.ExitCode}}",
				OnSuccess: "step5",
				OnFailure: "error",
			},
			"step5": {
				Name:      "step5",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{states.step1.Status}}",
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

	assert.False(t, result.HasErrors(), "Expected no errors for all uppercase properties")
}

// TestValidateStateRef_ErrorMessageFormat tests that error messages
// follow the expected format with step name, property, and suggestion.
func TestValidateStateRef_ErrorMessageFormat(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "casing-error-format-test",
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
				Command:   "echo {{states.step1.output}}", // lowercase
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
	errorMsg := result.Errors[0].Message

	// Error message should contain:
	// 1. The invalid property name ('output')
	// 2. The step name ('step1')
	// 3. The suggestion ('Output')
	assert.Contains(t, errorMsg, "output", "Error should mention the invalid property")
	assert.Contains(t, errorMsg, "step1", "Error should mention the step name")
	assert.Contains(t, errorMsg, "Output", "Error should suggest the correct uppercase")
	assert.Contains(t, errorMsg, "use 'Output' instead", "Error should have clear suggestion format")
}

// TestValidateStateRef_CaseSensitivity tests that casing is strictly enforced.
func TestValidateStateRef_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name      string
		property  string
		wantError bool
	}{
		{"lowercase 'output' fails", "output", true},
		{"uppercase 'Output' passes", "Output", false},
		{"uppercase 'OUTPUT' fails", "OUTPUT", true}, // all-caps not valid
		{"mixed 'OuTpUt' fails", "OuTpUt", true},     // wrong casing
		{"lowercase 'exitcode' (no underscore) fails", "exitcode", true},
		{"lowercase 'exit_code' fails", "exit_code", true},
		{"uppercase 'ExitCode' passes", "ExitCode", false},
		{"uppercase 'EXITCODE' fails", "EXITCODE", true}, // all-caps not valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &workflow.Workflow{
				Name:    "casing-case-sensitivity-test",
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

			if tt.wantError {
				assert.True(t, result.HasErrors(), "Expected error for property %q", tt.property)
			} else {
				assert.False(t, result.HasErrors(), "Expected no error for property %q", tt.property)
			}
		})
	}
}

// TestValidateStateRef_HooksWithLowercaseCasing tests that lowercase
// properties in hooks are also detected.
func TestValidateStateRef_HooksWithLowercaseCasing(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "casing-hooks-casing-test",
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
				Name:    "step2",
				Type:    workflow.StepTypeCommand,
				Command: "echo second",
				Hooks: workflow.StepHooks{
					Post: []workflow.HookAction{
						{
							Command: "echo {{states.step1.output}}", // lowercase in hook
						},
					},
				},
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

	require.True(t, result.HasErrors(), "Expected error for lowercase property in hook")
	assert.Contains(t, result.Errors[0].Message, "use 'Output' instead")
}

// TestValidateStateRef_BackwardCompatibilityBreak tests that the
// breaking change is properly enforced - lowercase syntax no longer works.
func TestValidateStateRef_BackwardCompatibilityBreak(t *testing.T) {
	// This test explicitly verifies the breaking change:
	// Previously documented lowercase syntax should now fail validation

	legacySyntaxExamples := []struct {
		template string
		wantErr  bool
	}{
		{"{{states.step1.output}}", true},    // legacy: should fail
		{"{{states.step1.stderr}}", true},    // legacy: should fail
		{"{{states.step1.exit_code}}", true}, // legacy: should fail
		{"{{states.step1.Output}}", false},   // new: should pass
		{"{{states.step1.Stderr}}", false},   // new: should pass
		{"{{states.step1.ExitCode}}", false}, // new: should pass
	}

	for _, example := range legacySyntaxExamples {
		t.Run(example.template, func(t *testing.T) {
			w := &workflow.Workflow{
				Name:    "casing-breaking-change-test",
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
						Command:   "echo " + example.template,
						OnSuccess: "done",
						OnFailure: "error",
					},
					"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
					"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
				},
			}

			v := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := v.Validate()

			if example.wantErr {
				assert.True(t, result.HasErrors(),
					"Legacy lowercase syntax %q should fail validation (breaking change)", example.template)
			} else {
				assert.False(t, result.HasErrors(),
					"New uppercase syntax %q should pass validation", example.template)
			}
		})
	}
}
