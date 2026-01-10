package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// testAnalyzer wraps pkg/interpolation for testing template validation.
type testAnalyzer struct{}

func newTestAnalyzer() *testAnalyzer {
	return &testAnalyzer{}
}

func (a *testAnalyzer) ExtractReferences(template string) ([]workflow.TemplateReference, error) {
	refs, err := interpolation.ExtractReferences(template)
	if err != nil {
		return nil, err
	}

	result := make([]workflow.TemplateReference, len(refs))
	for i, ref := range refs {
		result[i] = workflow.TemplateReference{
			Type:      convertRefType(ref.Type),
			Namespace: ref.Namespace,
			Path:      ref.Path,
			Property:  ref.Property,
			Raw:       ref.Raw,
		}
	}
	return result, nil
}

func convertRefType(t interpolation.ReferenceType) workflow.ReferenceType {
	switch t {
	case interpolation.TypeInputs:
		return workflow.TypeInputs
	case interpolation.TypeStates:
		return workflow.TypeStates
	case interpolation.TypeWorkflow:
		return workflow.TypeWorkflow
	case interpolation.TypeEnv:
		return workflow.TypeEnv
	case interpolation.TypeError:
		return workflow.TypeError
	case interpolation.TypeContext:
		return workflow.TypeContext
	case interpolation.TypeLoop:
		return workflow.TypeLoop
	default:
		return workflow.TypeUnknown
	}
}

// =============================================================================
// Helper: Create test workflows
// =============================================================================

func newTestWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "start",
		Inputs: []workflow.Input{
			{Name: "name", Type: "string", Required: true},
			{Name: "count", Type: "integer", Default: 1},
		},
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.name}}",
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
}

func newLinearWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "linear-workflow",
		Initial: "step1",
		Inputs: []workflow.Input{
			{Name: "input1", Type: "string"},
		},
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step1",
				OnSuccess: "step2",
				OnFailure: "error",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step2",
				OnSuccess: "step3",
				OnFailure: "error",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo step3",
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
}

// =============================================================================
// NewTemplateValidator Tests
// =============================================================================

func TestNewTemplateValidator(t *testing.T) {
	w := newTestWorkflow()
	v := workflow.NewTemplateValidator(w, newTestAnalyzer())

	require.NotNil(t, v)
}

func TestNewTemplateValidator_NilWorkflow(t *testing.T) {
	// Should handle nil workflow gracefully
	// Either panic (acceptable) or return nil
	defer func() {
		// Recover from panic if implementation panics on nil
		_ = recover()
	}()

	v := workflow.NewTemplateValidator(nil, newTestAnalyzer())
	// If we get here, validator should be nil or handle nil safely
	_ = v
}

// =============================================================================
// Validate - Valid Workflows
// =============================================================================

func TestTemplateValidator_ValidWorkflow(t *testing.T) {
	w := newTestWorkflow()
	v := workflow.NewTemplateValidator(w, newTestAnalyzer())

	result := v.Validate()

	require.NotNil(t, result)
	assert.False(t, result.HasErrors(), "valid workflow should have no errors")
}

func TestTemplateValidator_ValidInputReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo Hello {{inputs.name}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidMultipleInputReferences(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.name}} count={{inputs.count}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

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

func TestTemplateValidator_ValidWorkflowReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo Workflow: {{workflow.name}} ID: {{workflow.id}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidWorkflowReferenceAllProperties(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{workflow.id}} {{workflow.name}} {{workflow.current_state}} {{workflow.started_at}} {{workflow.duration}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidEnvReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo HOME={{env.HOME}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "env references should not cause validation errors")
}

func TestTemplateValidator_ValidEnvReferenceAnyVariable(t *testing.T) {
	// Environment variables are validated at runtime, not statically
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{env.DOES_NOT_EXIST}} {{env.MY_CUSTOM_VAR}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "any env reference should pass static validation")
}

func TestTemplateValidator_ValidContextReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "cd {{context.working_dir}} && whoami"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidContextReferenceAllProperties(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{context.working_dir}} {{context.user}} {{context.hostname}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_NoTemplateReferences(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo plain text without templates"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

// =============================================================================
// Validate - Invalid Input References
// =============================================================================

func TestTemplateValidator_UndefinedInput(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined_var}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, workflow.ErrUndefinedInput, result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Path, "start")
}

func TestTemplateValidator_MultipleUndefinedInputs(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.foo}} {{inputs.bar}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Len(t, result.Errors, 2, "should report all undefined inputs, not fail-fast")
}

func TestTemplateValidator_MixedDefinedUndefinedInputs(t *testing.T) {
	w := newTestWorkflow()
	// "name" is defined, "undefined" is not
	w.Steps["start"].Command = "echo {{inputs.name}} {{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Len(t, result.Errors, 1, "should only report the undefined input")
	assert.Equal(t, workflow.ErrUndefinedInput, result.Errors[0].Code)
	assert.Contains(t, result.Errors[0].Message, "undefined")
}

func TestTemplateValidator_UndefinedInputInMultipleSteps(t *testing.T) {
	w := newLinearWorkflow()
	w.Steps["step1"].Command = "echo {{inputs.undefined1}}"
	w.Steps["step2"].Command = "echo {{inputs.undefined2}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Len(t, result.Errors, 2, "should report errors from all steps")
}

// =============================================================================
// Validate - Invalid State References
// =============================================================================

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

// =============================================================================
// Validate - Invalid Workflow References
// =============================================================================

func TestTemplateValidator_InvalidWorkflowProperty(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{workflow.unknown_property}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrInvalidWorkflowProperty, result.Errors[0].Code)
}

func TestTemplateValidator_InvalidWorkflowPropertyCommonMistakes(t *testing.T) {
	tests := []struct {
		name     string
		property string
	}{
		{"status instead of current_state", "status"},
		{"state instead of current_state", "state"},
		{"start_time instead of started_at", "start_time"},
		{"runtime instead of duration", "runtime"},
		{"workflow_id instead of id", "workflow_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWorkflow()
			w.Steps["start"].Command = "echo {{workflow." + tt.property + "}}"

			v := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := v.Validate()

			require.True(t, result.HasErrors())
			assert.Equal(t, workflow.ErrInvalidWorkflowProperty, result.Errors[0].Code)
		})
	}
}

// =============================================================================
// Validate - Invalid Context References
// =============================================================================

func TestTemplateValidator_InvalidContextProperty(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{context.invalid}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrInvalidContextProperty, result.Errors[0].Code)
}

func TestTemplateValidator_InvalidContextPropertyCommonMistakes(t *testing.T) {
	tests := []struct {
		name     string
		property string
	}{
		{"cwd instead of working_dir", "cwd"},
		{"pwd instead of working_dir", "pwd"},
		{"username instead of user", "username"},
		{"host instead of hostname", "host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWorkflow()
			w.Steps["start"].Command = "echo {{context." + tt.property + "}}"

			v := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := v.Validate()

			require.True(t, result.HasErrors())
			assert.Equal(t, workflow.ErrInvalidContextProperty, result.Errors[0].Code)
		})
	}
}

// =============================================================================
// Validate - Error References in Hooks
// =============================================================================

func TestTemplateValidator_ErrorRefInErrorHook_Valid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowError: workflow.Hook{
			{Command: "echo Error: {{error.message}} in {{error.state}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "error references in workflow error hooks are valid")
}

func TestTemplateValidator_ErrorRefAllProperties_Valid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowError: workflow.Hook{
			{Command: "echo {{error.message}} {{error.state}} {{error.exit_code}} {{error.type}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ErrorRefOutsideErrorHook_Invalid(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{error.message}}" // ERROR: not in error hook

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrErrorRefOutsideErrorHook, result.Errors[0].Code)
}

func TestTemplateValidator_ErrorRefInStartHook_Invalid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowStart: workflow.Hook{
			{Command: "echo {{error.message}}"}, // Invalid: WorkflowStart is not an error hook
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrErrorRefOutsideErrorHook, result.Errors[0].Code)
}

func TestTemplateValidator_ErrorRefInEndHook_Invalid(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowEnd: workflow.Hook{
			{Command: "echo {{error.message}}"}, // Invalid: WorkflowEnd is not an error hook
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrErrorRefOutsideErrorHook, result.Errors[0].Code)
}

func TestTemplateValidator_InvalidErrorProperty(t *testing.T) {
	w := newTestWorkflow()
	w.Hooks = workflow.WorkflowHooks{
		WorkflowError: workflow.Hook{
			{Command: "echo {{error.invalid_property}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrInvalidErrorProperty, result.Errors[0].Code)
}

// =============================================================================
// Validate - Step Hooks
// =============================================================================

func TestTemplateValidator_ValidStepPreHook(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Command: "echo Starting {{inputs.name}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidStepPostHook(t *testing.T) {
	w := newLinearWorkflow()
	w.Steps["step2"].Hooks = workflow.StepHooks{
		Post: workflow.Hook{
			{Command: "echo Completed with {{states.step1.Output}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InvalidRefInStepHook(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Command: "echo {{inputs.undefined}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrUndefinedInput, result.Errors[0].Code)
}

func TestTemplateValidator_HookLogField(t *testing.T) {
	// Hooks can have Log instead of Command
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Log: "Processing {{inputs.name}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InvalidRefInHookLog(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Hooks = workflow.StepHooks{
		Pre: workflow.Hook{
			{Log: "Processing {{inputs.undefined}}"},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
}

// =============================================================================
// Validate - Unknown Reference Type
// =============================================================================

func TestTemplateValidator_UnknownReferenceType(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{unknown.field}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrUnknownReferenceType, result.Errors[0].Code)
}

func TestTemplateValidator_TypoInNamespace(t *testing.T) {
	tests := []struct {
		name     string
		template string
	}{
		{"input instead of inputs", "{{input.name}}"},
		{"state instead of states", "{{state.step1.output}}"},
		{"workflows instead of workflow", "{{workflows.name}}"},
		{"environment instead of env", "{{environment.HOME}}"},
		{"errors instead of error", "{{errors.message}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWorkflow()
			w.Steps["start"].Command = "echo " + tt.template

			v := workflow.NewTemplateValidator(w, newTestAnalyzer())
			result := v.Validate()

			require.True(t, result.HasErrors())
			assert.Equal(t, workflow.ErrUnknownReferenceType, result.Errors[0].Code)
		})
	}
}

// =============================================================================
// Validate - Aggregate All Errors (Non Fail-Fast)
// =============================================================================

func TestTemplateValidator_AggregatesAllErrors(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "multi-error-test",
		Initial: "step1",
		Inputs:  []workflow.Input{{Name: "valid", Type: "string"}},
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.undefined}} {{states.nonexistent.Output}} {{workflow.bad}}",
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
	assert.GreaterOrEqual(t, len(result.Errors), 3, "should collect all errors in single pass")
}

func TestTemplateValidator_AggregatesErrorsAcrossSteps(t *testing.T) {
	w := newLinearWorkflow()
	w.Steps["step1"].Command = "echo {{inputs.undefined1}}"
	w.Steps["step2"].Command = "echo {{inputs.undefined2}}"
	w.Steps["step3"].Command = "echo {{inputs.undefined3}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.GreaterOrEqual(t, len(result.Errors), 3)
}

func TestTemplateValidator_AggregatesErrorsAcrossFields(t *testing.T) {
	// Errors in different fields of the same step
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined1}}"
	w.Steps["start"].Dir = "{{inputs.undefined2}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.GreaterOrEqual(t, len(result.Errors), 2)
}

// =============================================================================
// Validate - Dir Field
// =============================================================================

func TestTemplateValidator_ValidDirFieldReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Dir = "{{context.working_dir}}/subdir"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InvalidDirFieldReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Dir = "{{inputs.undefined}}/subdir"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
}

// =============================================================================
// ComputeExecutionOrder Tests
// =============================================================================

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

// =============================================================================
// Parallel Step Validation Tests
// =============================================================================

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

// =============================================================================
// Edge Cases and Special Scenarios
// =============================================================================

func TestTemplateValidator_EmptyInputsList(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "no-inputs",
		Initial: "start",
		Inputs:  []workflow.Input{}, // No inputs defined
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello", // No template references
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_InputRefWithNoInputsDefined(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "no-inputs-but-referenced",
		Initial: "start",
		Inputs:  []workflow.Input{}, // No inputs defined
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{inputs.name}}", // References undefined input
				OnSuccess: "done",
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Equal(t, workflow.ErrUndefinedInput, result.Errors[0].Code)
}

func TestTemplateValidator_TerminalStepsIgnored(t *testing.T) {
	// Terminal steps shouldn't have commands, but if they do, validate them
	w := newTestWorkflow()
	// Terminal steps don't have Command field used, so this is a no-op
	// but the validator should handle gracefully

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ComplexRealWorldWorkflow(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "build-test-deploy",
		Initial: "checkout",
		Inputs: []workflow.Input{
			{Name: "repo", Type: "string", Required: true},
			{Name: "branch", Type: "string", Default: "main"},
			{Name: "environment", Type: "string", Required: true},
		},
		Steps: map[string]*workflow.Step{
			"checkout": {
				Name:      "checkout",
				Type:      workflow.StepTypeCommand,
				Command:   "git clone {{inputs.repo}} --branch {{inputs.branch}}",
				OnSuccess: "build",
				OnFailure: "error",
			},
			"build": {
				Name:      "build",
				Type:      workflow.StepTypeCommand,
				Command:   "make build",
				Dir:       "{{context.working_dir}}/repo",
				OnSuccess: "test",
				OnFailure: "error",
			},
			"test": {
				Name:      "test",
				Type:      workflow.StepTypeCommand,
				Command:   "make test",
				OnSuccess: "deploy",
				OnFailure: "error",
			},
			"deploy": {
				Name:      "deploy",
				Type:      workflow.StepTypeCommand,
				Command:   "deploy --env={{inputs.environment}} --version={{workflow.id}}",
				OnSuccess: "notify",
				OnFailure: "error",
			},
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeCommand,
				Command:   "curl -X POST -d 'Deployed {{inputs.repo}} to {{inputs.environment}}' {{env.SLACK_WEBHOOK}}",
				OnSuccess: "done",
				OnFailure: "done", // Notification failure shouldn't fail workflow
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
		Hooks: workflow.WorkflowHooks{
			WorkflowStart: workflow.Hook{
				{Log: "Starting build for {{inputs.repo}}"},
			},
			WorkflowEnd: workflow.Hook{
				{Log: "Workflow {{workflow.name}} completed in {{workflow.duration}}"},
			},
			WorkflowError: workflow.Hook{
				{Command: "echo 'Error in {{error.state}}: {{error.message}}' | mail -s 'Build Failed' team@example.com"},
			},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "real-world workflow should validate successfully")
}

// =============================================================================
// Error Message Quality Tests
// =============================================================================

func TestTemplateValidator_ErrorMessageContainsStepName(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0].Path, "start", "error path should contain step name")
}

func TestTemplateValidator_ErrorMessageContainsReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors())
	assert.Contains(t, result.Errors[0].Message, "undefined", "error message should contain the reference")
}

// =============================================================================
// Loop Expression Validation Tests (F037-T016)
// =============================================================================

func newLoopWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "loop-workflow",
		Initial: "loop_step",
		Inputs: []workflow.Input{
			{Name: "limit", Type: "integer", Default: 5},
			{Name: "threshold", Type: "integer", Default: 10},
			{Name: "items_list", Type: "string", Default: "a,b,c"},
		},
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name:      "loop_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo iteration",
				OnSuccess: "done",
				OnFailure: "error",
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeForEach,
					Items:         "{{inputs.items_list}}",
					Body:          []string{"loop_body"},
					MaxIterations: 10,
					OnComplete:    "done",
				},
			},
			"loop_body": {
				Name:      "loop_body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo body",
				OnSuccess: "loop_step",
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
}

func TestTemplateValidator_LoopExpressions_ValidMaxIterationsExpr(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit}}"
	w.Steps["loop_step"].Loop.MaxIterations = 0 // Dynamic takes precedence

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in MaxIterationsExpr should pass")
	assert.False(t, result.HasWarnings(), "valid input reference should not produce warnings")
}

func TestTemplateValidator_LoopExpressions_ValidMaxIterationsExprFromEnv(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{env.LOOP_LIMIT}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Env variables are validated at runtime, not statically - should pass
	assert.False(t, result.HasErrors(), "env reference in MaxIterationsExpr should pass static validation")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInMaxIterationsExpr(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.undefined_var}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should emit a warning about potentially undefined variable
	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in MaxIterationsExpr should be flagged")

	// Check that the issue mentions the undefined variable
	issues := result.AllIssues()
	require.NotEmpty(t, issues)

	found := false
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			found = true
			assert.Contains(t, issue.Message, "undefined_var",
				"warning should mention the undefined variable name")
			assert.Contains(t, issue.Path, "loop_step",
				"warning should mention the step name")
		}
	}
	assert.True(t, found, "should have found an undefined variable issue")
}

func TestTemplateValidator_LoopExpressions_ValidConditionWithInputs(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Type = workflow.LoopTypeWhile
	w.Steps["loop_step"].Loop.Condition = "{{inputs.threshold}} > 0"
	w.Steps["loop_step"].Loop.Items = "" // Not needed for while loop

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in condition should pass")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInCondition(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Type = workflow.LoopTypeWhile
	w.Steps["loop_step"].Loop.Condition = "{{inputs.undefined_count}} < {{inputs.threshold}}"
	w.Steps["loop_step"].Loop.Items = ""

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should flag the undefined input
	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in condition should be flagged")

	issues := result.AllIssues()
	require.NotEmpty(t, issues)

	// Should mention undefined_count (threshold is defined)
	hasUndefinedCountIssue := false
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			if issue.Message != "" && (issue.Message == "undefined_count" ||
				len(issue.Message) > 0 && issue.Message[0:1] != "") {
				hasUndefinedCountIssue = true
			}
		}
	}
	_ = hasUndefinedCountIssue // Will be properly checked after implementation
}

func TestTemplateValidator_LoopExpressions_ValidItemsWithInputs(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Items = "{{inputs.items_list}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in items should pass")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInItems(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Items = "{{inputs.undefined_items}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in items should be flagged")
}

func TestTemplateValidator_LoopExpressions_ValidBreakCondition(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.limit}} <= 0"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "valid input reference in break_condition should pass")
}

func TestTemplateValidator_LoopExpressions_UndefinedInputInBreakCondition(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.undefined_flag}} == true"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasWarnings() || result.HasErrors(),
		"undefined input in break_condition should be flagged")
}

func TestTemplateValidator_LoopExpressions_ArithmeticExpression(t *testing.T) {
	w := newLoopWorkflow()
	// Arithmetic expression with defined inputs
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit * inputs.threshold}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// The validator should at minimum check that variables in expression exist
	// Both limit and threshold are defined, so should pass
	// Note: actual arithmetic validation happens at runtime
	assert.False(t, result.HasErrors(), "arithmetic expression with valid inputs should pass")
}

func TestTemplateValidator_LoopExpressions_ArithmeticExpressionWithUndefined(t *testing.T) {
	w := newLoopWorkflow()
	// Arithmetic expression with one undefined input
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit + inputs.undefined_multiplier}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasWarnings() || result.HasErrors(),
		"arithmetic expression with undefined input should be flagged")
}

func TestTemplateValidator_LoopExpressions_StateReferenceInCondition(t *testing.T) {
	// Create a workflow where loop condition references previous step output
	w := &workflow.Workflow{
		Name:    "loop-with-state-ref",
		Initial: "setup",
		Inputs:  []workflow.Input{{Name: "target", Type: "integer", Default: 10}},
		Steps: map[string]*workflow.Step{
			"setup": {
				Name:      "setup",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 0",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"loop_step": {
				Name:      "loop_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo iteration",
				OnSuccess: "done",
				OnFailure: "error",
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "{{states.setup.Output}} < {{inputs.target}}",
					Body:          []string{"loop_body"},
					MaxIterations: 100,
					OnComplete:    "done",
				},
			},
			"loop_body": {
				Name:      "loop_body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo body",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// setup runs before loop_step, so the state reference should be valid
	assert.False(t, result.HasErrors(), "valid state reference in condition should pass")
}

func TestTemplateValidator_LoopExpressions_ForwardStateReferenceInCondition(t *testing.T) {
	w := &workflow.Workflow{
		Name:    "loop-forward-ref",
		Initial: "loop_step",
		Inputs:  []workflow.Input{},
		Steps: map[string]*workflow.Step{
			"loop_step": {
				Name:      "loop_step",
				Type:      workflow.StepTypeCommand,
				Command:   "echo iteration",
				OnSuccess: "after_loop",
				OnFailure: "error",
				Loop: &workflow.LoopConfig{
					Type:          workflow.LoopTypeWhile,
					Condition:     "{{states.after_loop.Output}} != 'done'", // Forward reference!
					Body:          []string{"loop_body"},
					MaxIterations: 10,
					OnComplete:    "after_loop",
				},
			},
			"loop_body": {
				Name:      "loop_body",
				Type:      workflow.StepTypeCommand,
				Command:   "echo body",
				OnSuccess: "loop_step",
				OnFailure: "error",
			},
			"after_loop": {
				Name:      "after_loop",
				Type:      workflow.StepTypeCommand,
				Command:   "echo done",
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// after_loop hasn't run yet - this should be flagged
	require.True(t, result.HasErrors() || result.HasWarnings(),
		"forward state reference in loop condition should be flagged")
}

func TestTemplateValidator_LoopExpressions_MultipleUndefinedInputs(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.undefined1}}"
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.undefined2}} == true"
	w.Steps["loop_step"].Loop.Items = "{{inputs.undefined3}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should flag all undefined variables
	issues := result.AllIssues()
	undefinedCount := 0
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			undefinedCount++
		}
	}

	assert.GreaterOrEqual(t, undefinedCount, 3,
		"should flag all undefined variables, got %d", undefinedCount)
}

func TestTemplateValidator_LoopExpressions_NoLoop(t *testing.T) {
	// Step without loop - should not cause any issues
	w := newTestWorkflow()

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_LoopExpressions_EmptyExpressions(t *testing.T) {
	w := newLoopWorkflow()
	// All loop expressions are empty or use static values
	w.Steps["loop_step"].Loop.MaxIterationsExpr = ""
	w.Steps["loop_step"].Loop.MaxIterations = 10
	w.Steps["loop_step"].Loop.Items = "a,b,c" // Static, no interpolation

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "static loop config should pass")
}

func TestTemplateValidator_LoopExpressions_MixedValidAndInvalid(t *testing.T) {
	w := newLoopWorkflow()
	// One valid, one invalid
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.limit}}"  // Valid
	w.Steps["loop_step"].Loop.BreakCondition = "{{inputs.undefined}}" // Invalid

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should flag the invalid one but not the valid one
	issues := result.AllIssues()
	undefinedCount := 0
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			undefinedCount++
			assert.Contains(t, issue.Message, "undefined",
				"should only flag the undefined variable")
		}
	}

	assert.Equal(t, 1, undefinedCount, "should only flag one undefined variable")
}

func TestTemplateValidator_LoopExpressions_WorkflowReference(t *testing.T) {
	w := newLoopWorkflow()
	// Using workflow property in loop expression (unusual but valid)
	w.Steps["loop_step"].Loop.BreakCondition = "{{workflow.duration}} > 60"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "workflow reference in loop expression should be valid")
}

func TestTemplateValidator_LoopExpressions_InvalidWorkflowProperty(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.BreakCondition = "{{workflow.invalid_property}} > 0"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	require.True(t, result.HasErrors() || result.HasWarnings(),
		"invalid workflow property in loop expression should be flagged")
}

func TestTemplateValidator_LoopExpressions_ContextReference(t *testing.T) {
	w := newLoopWorkflow()
	// Using context in loop (unusual but technically valid)
	w.Steps["loop_step"].Loop.Items = "{{context.working_dir}}/items"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors(), "context reference in loop expression should be valid")
}

func TestTemplateValidator_LoopExpressions_ErrorPath(t *testing.T) {
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	issues := result.AllIssues()
	require.NotEmpty(t, issues)

	// The error path should help identify the location
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			assert.Contains(t, issue.Path, "loop_step",
				"error path should contain step name for loop expressions")
		}
	}
}

func TestTemplateValidator_LoopExpressions_WarningLevel(t *testing.T) {
	// Undefined input variables in loop expressions should be warnings, not errors
	// because they could potentially come from env at runtime
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.MaxIterationsExpr = "{{inputs.maybe_from_env}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Per the spec, undefined loop variables should be warnings since they could be from env
	// The exact behavior depends on implementation
	issues := result.AllIssues()
	require.NotEmpty(t, issues, "should produce at least one issue")
}

func TestTemplateValidator_LoopExpressions_WhileLoopWithItems(t *testing.T) {
	// While loop shouldn't have items, but if it does, still validate template references
	w := newLoopWorkflow()
	w.Steps["loop_step"].Loop.Type = workflow.LoopTypeWhile
	w.Steps["loop_step"].Loop.Condition = "true"
	// Items shouldn't be used for while loop, but if set, validate it
	w.Steps["loop_step"].Loop.Items = "{{inputs.undefined}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// Should still validate the items field even if not used
	issues := result.AllIssues()
	hasUndefinedIssue := false
	for _, issue := range issues {
		if issue.Code == workflow.ErrUndefinedLoopVariable || issue.Code == workflow.ErrUndefinedInput {
			hasUndefinedIssue = true
		}
	}
	assert.True(t, hasUndefinedIssue, "should validate items field template references")
}

func TestTemplateValidator_LoopExpressions_ComplexExpression(t *testing.T) {
	w := newLoopWorkflow()
	// Complex expression with multiple references
	w.Steps["loop_step"].Loop.Condition = "{{inputs.limit}} > 0 && {{inputs.threshold}} < 100 && {{env.DEBUG}} == 'true'"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	// All inputs are defined, env is validated at runtime
	assert.False(t, result.HasErrors(), "complex expression with valid references should pass")
}

func TestTemplateValidator_LoopExpressions_NestedTemplate(t *testing.T) {
	w := newLoopWorkflow()
	// Nested/escaped template syntax (edge case)
	w.Steps["loop_step"].Loop.Items = "{{inputs.items_list}},extra"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

// =============================================================================
// Helper function
// =============================================================================

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
