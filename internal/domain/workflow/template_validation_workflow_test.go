package workflow_test

// C013: Domain test file splitting
// Source: internal/domain/workflow/template_validation_test.go
// Test count: 14 tests
// Focus: workflow.*, context.*, env.* namespaces - Runtime context reference validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Validate - Valid Workflow
// =============================================================================

func TestTemplateValidator_ValidWorkflow(t *testing.T) {
	w := newTestWorkflow()
	v := workflow.NewTemplateValidator(w, newTestAnalyzer())

	result := v.Validate()

	require.NotNil(t, result)
	assert.False(t, result.HasErrors(), "valid workflow should have no errors")
}

func TestTemplateValidator_NoTemplateReferences(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo plain text without templates"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

// =============================================================================
// Validate - Valid Workflow References
// =============================================================================

func TestTemplateValidator_ValidWorkflowReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo Workflow: {{workflow.Name}} ID: {{workflow.ID}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidWorkflowReferenceAllProperties(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{workflow.ID}} {{workflow.Name}} {{workflow.CurrentState}} {{workflow.StartedAt}} {{workflow.Duration}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
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
// Validate - Valid Env References
// =============================================================================

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

// =============================================================================
// Validate - Valid Context References
// =============================================================================

func TestTemplateValidator_ValidContextReference(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "cd {{context.WorkingDir}} && whoami"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
}

func TestTemplateValidator_ValidContextReferenceAllProperties(t *testing.T) {
	w := newTestWorkflow()
	w.Steps["start"].Command = "echo {{context.WorkingDir}} {{context.User}} {{context.Hostname}}"

	v := workflow.NewTemplateValidator(w, newTestAnalyzer())
	result := v.Validate()

	assert.False(t, result.HasErrors())
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
