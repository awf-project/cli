package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TemplateNotFoundError Tests
// =============================================================================

func TestTemplateNotFoundError_Error_WithReferencedBy(t *testing.T) {
	// Happy path: error with both TemplateName and ReferencedBy
	err := &TemplateNotFoundError{
		TemplateName: "user-notification",
		ReferencedBy: "workflow.yaml",
	}

	result := err.Error()

	assert.Contains(t, result, "user-notification")
	assert.Contains(t, result, "workflow.yaml")
	assert.Contains(t, result, "not found")
	assert.Contains(t, result, "referenced by")
	assert.Equal(t, `template "user-notification" not found (referenced by workflow.yaml)`, result)
}

func TestTemplateNotFoundError_Error_WithoutReferencedBy(t *testing.T) {
	// Edge case: error without ReferencedBy (empty string)
	err := &TemplateNotFoundError{
		TemplateName: "email-template",
		ReferencedBy: "",
	}

	result := err.Error()

	assert.Contains(t, result, "email-template")
	assert.Contains(t, result, "not found")
	assert.NotContains(t, result, "referenced by")
	assert.Equal(t, `template "email-template" not found`, result)
}

func TestTemplateNotFoundError_ImplementsErrorInterface(t *testing.T) {
	// Verify error interface implementation
	var err error = &TemplateNotFoundError{
		TemplateName: "test-template",
	}

	require.NotNil(t, err)
	require.NotEmpty(t, err.Error())
}

func TestTemplateNotFoundError_EmptyTemplateName(t *testing.T) {
	// Edge case: empty template name
	err := &TemplateNotFoundError{
		TemplateName: "",
		ReferencedBy: "main.yaml",
	}

	result := err.Error()

	assert.Contains(t, result, "not found")
	assert.Contains(t, result, "main.yaml")
	// Should format as: template "" not found (referenced by main.yaml)
	assert.Equal(t, `template "" not found (referenced by main.yaml)`, result)
}

func TestTemplateNotFoundError_AllFieldsEmpty(t *testing.T) {
	// Edge case: all fields empty
	err := &TemplateNotFoundError{
		TemplateName: "",
		ReferencedBy: "",
	}

	result := err.Error()

	// Should still produce valid error message
	assert.Contains(t, result, "not found")
	assert.Equal(t, `template "" not found`, result)
}

func TestTemplateNotFoundError_SpecialCharactersInNames(t *testing.T) {
	// Edge case: special characters and spaces in names
	tests := []struct {
		name         string
		templateName string
		referencedBy string
		expected     string
	}{
		{
			name:         "with spaces",
			templateName: "my template",
			referencedBy: "my workflow.yaml",
			expected:     `template "my template" not found (referenced by my workflow.yaml)`,
		},
		{
			name:         "with special chars",
			templateName: "template-v1.2.3",
			referencedBy: "configs/workflow@prod.yaml",
			expected:     `template "template-v1.2.3" not found (referenced by configs/workflow@prod.yaml)`,
		},
		{
			name:         "with unicode",
			templateName: "тест-шаблон",
			referencedBy: "файл.yaml",
			expected:     `template "тест-шаблон" not found (referenced by файл.yaml)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &TemplateNotFoundError{
				TemplateName: tt.templateName,
				ReferencedBy: tt.referencedBy,
			}

			result := err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// CircularTemplateError Tests
// =============================================================================

func TestCircularTemplateError_Error_SingleCycle(t *testing.T) {
	// Happy path: circular reference with single cycle
	err := &CircularTemplateError{
		Chain: []string{"template-a", "template-b", "template-a"},
	}

	result := err.Error()

	assert.Contains(t, result, "circular template reference detected")
	assert.Contains(t, result, "template-a")
	assert.Contains(t, result, "template-b")
}

func TestCircularTemplateError_Error_EmptyChain(t *testing.T) {
	// Edge case: empty chain
	err := &CircularTemplateError{
		Chain: []string{},
	}

	result := err.Error()

	assert.Contains(t, result, "circular template reference detected")
	// Should show empty slice: []
	assert.Contains(t, result, "[]")
}

func TestCircularTemplateError_Error_SingleElement(t *testing.T) {
	// Edge case: single element (self-reference)
	err := &CircularTemplateError{
		Chain: []string{"self-referencing-template"},
	}

	result := err.Error()

	assert.Contains(t, result, "circular template reference detected")
	assert.Contains(t, result, "self-referencing-template")
}

func TestCircularTemplateError_Error_LongChain(t *testing.T) {
	// Edge case: long chain of references
	err := &CircularTemplateError{
		Chain: []string{"a", "b", "c", "d", "e", "f", "g", "a"},
	}

	result := err.Error()

	assert.Contains(t, result, "circular template reference detected")
	// Verify all elements are in the message
	for _, elem := range err.Chain {
		assert.Contains(t, result, elem)
	}
}

func TestCircularTemplateError_ImplementsErrorInterface(t *testing.T) {
	// Verify error interface implementation
	var err error = &CircularTemplateError{
		Chain: []string{"a", "b", "a"},
	}

	require.NotNil(t, err)
	require.NotEmpty(t, err.Error())
}

// =============================================================================
// MissingParameterError Tests
// =============================================================================

func TestMissingParameterError_Error_WithRequiredList(t *testing.T) {
	// Happy path: error with all fields populated
	err := &MissingParameterError{
		TemplateName:  "notification-template",
		ParameterName: "recipient_email",
		Required:      []string{"recipient_email", "subject", "body"},
	}

	result := err.Error()

	assert.Contains(t, result, "notification-template")
	assert.Contains(t, result, "recipient_email")
	assert.Contains(t, result, "missing required parameter")
	assert.Equal(t, `template "notification-template" missing required parameter "recipient_email"`, result)
}

func TestMissingParameterError_Error_WithoutRequiredList(t *testing.T) {
	// Edge case: error without Required list
	err := &MissingParameterError{
		TemplateName:  "email",
		ParameterName: "to",
		Required:      nil,
	}

	result := err.Error()

	assert.Contains(t, result, "email")
	assert.Contains(t, result, "to")
	assert.Contains(t, result, "missing required parameter")
	assert.Equal(t, `template "email" missing required parameter "to"`, result)
}

func TestMissingParameterError_Error_EmptyRequiredList(t *testing.T) {
	// Edge case: empty Required list
	err := &MissingParameterError{
		TemplateName:  "template",
		ParameterName: "param",
		Required:      []string{},
	}

	result := err.Error()

	assert.Contains(t, result, "template")
	assert.Contains(t, result, "param")
	assert.Equal(t, `template "template" missing required parameter "param"`, result)
}

func TestMissingParameterError_Error_EmptyFields(t *testing.T) {
	// Edge case: empty template and parameter names
	err := &MissingParameterError{
		TemplateName:  "",
		ParameterName: "",
		Required:      []string{},
	}

	result := err.Error()

	// Should still produce valid error message
	assert.Contains(t, result, "missing required parameter")
	assert.Equal(t, `template "" missing required parameter ""`, result)
}

func TestMissingParameterError_ImplementsErrorInterface(t *testing.T) {
	// Verify error interface implementation
	var err error = &MissingParameterError{
		TemplateName:  "test",
		ParameterName: "param",
	}

	require.NotNil(t, err)
	require.NotEmpty(t, err.Error())
}

func TestMissingParameterError_SpecialCharactersInNames(t *testing.T) {
	// Edge case: special characters in names
	tests := []struct {
		name          string
		templateName  string
		parameterName string
		expected      string
	}{
		{
			name:          "with dots",
			templateName:  "template.v1",
			parameterName: "param.nested",
			expected:      `template "template.v1" missing required parameter "param.nested"`,
		},
		{
			name:          "with dashes",
			templateName:  "my-template",
			parameterName: "user-id",
			expected:      `template "my-template" missing required parameter "user-id"`,
		},
		{
			name:          "with underscores",
			templateName:  "some_template",
			parameterName: "some_param",
			expected:      `template "some_template" missing required parameter "some_param"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &MissingParameterError{
				TemplateName:  tt.templateName,
				ParameterName: tt.parameterName,
				Required:      []string{tt.parameterName},
			}

			result := err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Integration Tests: All Template Errors
// =============================================================================

func TestTemplateErrors_AllImplementErrorInterface(t *testing.T) {
	// Verify all template error types implement error interface
	errors := []error{
		&TemplateNotFoundError{TemplateName: "test"},
		&CircularTemplateError{Chain: []string{"a", "b"}},
		&MissingParameterError{TemplateName: "test", ParameterName: "param"},
	}

	for i, err := range errors {
		t.Run(err.Error(), func(t *testing.T) {
			require.NotNil(t, err, "error %d should not be nil", i)
			require.NotEmpty(t, err.Error(), "error %d should have non-empty message", i)
		})
	}
}

func TestTemplateErrors_UniqueMessages(t *testing.T) {
	// Verify each error type produces unique messages
	errors := []error{
		&TemplateNotFoundError{TemplateName: "template"},
		&CircularTemplateError{Chain: []string{"template"}},
		&MissingParameterError{TemplateName: "template", ParameterName: "param"},
	}

	messages := make(map[string]bool)
	for _, err := range errors {
		msg := err.Error()
		assert.False(t, messages[msg], "duplicate error message: %s", msg)
		messages[msg] = true
	}

	assert.Len(t, messages, 3, "should have 3 unique error messages")
}
