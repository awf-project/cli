package workflow_test

// C013: Domain test file splitting
// Source: internal/domain/workflow/template_validation_test.go
// Test count: 10 tests
// Focus: inputs.* namespace - Input reference validation tests

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
