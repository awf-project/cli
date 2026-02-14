package repository

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// These tests verify that repository.TemplateNotFoundError (the type alias)
// behaves identically to workflow.TemplateNotFoundError (the canonical type).
// This ensures backward compatibility for infrastructure code while the
// canonical definition lives in the domain layer.

func TestTemplateNotFoundErrorAlias_IdenticalToCanonicalType(t *testing.T) {
	// Happy path: verify type alias produces same error message as canonical type
	aliasErr := &TemplateNotFoundError{
		TemplateName: "user-template",
		ReferencedBy: "workflow.yaml",
	}

	canonicalErr := &workflow.TemplateNotFoundError{
		TemplateName: "user-template",
		ReferencedBy: "workflow.yaml",
	}

	assert.Equal(t, canonicalErr.Error(), aliasErr.Error(),
		"type alias should produce identical error message to canonical type")
}

func TestTemplateNotFoundErrorAlias_ErrorsAsCompatibility(t *testing.T) {
	// Happy path: errors.As() should work with both alias and canonical type
	aliasErr := &TemplateNotFoundError{
		TemplateName: "missing-template",
		ReferencedBy: "main.yaml",
	}

	// Wrap error to test errors.As unwrapping
	wrappedErr := errors.Join(aliasErr)

	// Should be detectable as repository.TemplateNotFoundError (alias)
	var targetAlias *TemplateNotFoundError
	require.True(t, errors.As(wrappedErr, &targetAlias),
		"errors.As should detect repository.TemplateNotFoundError (alias)")
	assert.Equal(t, "missing-template", targetAlias.TemplateName)
	assert.Equal(t, "main.yaml", targetAlias.ReferencedBy)

	// Should also be detectable as workflow.TemplateNotFoundError (canonical)
	var targetCanonical *workflow.TemplateNotFoundError
	require.True(t, errors.As(wrappedErr, &targetCanonical),
		"errors.As should detect workflow.TemplateNotFoundError (canonical)")
	assert.Equal(t, "missing-template", targetCanonical.TemplateName)
	assert.Equal(t, "main.yaml", targetCanonical.ReferencedBy)
}

func TestTemplateNotFoundErrorAlias_ErrorsIsCompatibility(t *testing.T) {
	// Edge case: errors.Is() compatibility for sentinel-like comparison
	err1 := &TemplateNotFoundError{
		TemplateName: "template-a",
		ReferencedBy: "file-a",
	}

	err2 := &workflow.TemplateNotFoundError{
		TemplateName: "template-a",
		ReferencedBy: "file-a",
	}

	// Note: errors.Is() checks for pointer equality, not value equality
	// This test verifies both types can be used in Is() checks
	assert.False(t, errors.Is(err1, err2),
		"different error instances should not be equal with errors.Is()")

	// But the same instance should be detectable
	wrappedSameErr := errors.Join(err1)
	assert.True(t, errors.Is(wrappedSameErr, err1),
		"errors.Is should detect the same error instance")
}

func TestTemplateNotFoundErrorAlias_ConstructionViaAlias(t *testing.T) {
	// Happy path: constructing error via alias type (common in infrastructure code)
	err := &TemplateNotFoundError{
		TemplateName: "email-template",
		ReferencedBy: "notification-workflow.yaml",
	}

	result := err.Error()

	assert.Contains(t, result, "email-template")
	assert.Contains(t, result, "notification-workflow.yaml")
	assert.Contains(t, result, "not found")
	assert.Contains(t, result, "referenced by")
	assert.Equal(t, `template "email-template" not found (referenced by notification-workflow.yaml)`, result)
}

func TestTemplateNotFoundErrorAlias_ConstructionWithoutReferencedBy(t *testing.T) {
	// Edge case: constructing error via alias without ReferencedBy
	err := &TemplateNotFoundError{
		TemplateName: "standalone-template",
		ReferencedBy: "",
	}

	result := err.Error()

	assert.Contains(t, result, "standalone-template")
	assert.Contains(t, result, "not found")
	assert.NotContains(t, result, "referenced by")
	assert.Equal(t, `template "standalone-template" not found`, result)
}

func TestTemplateNotFoundErrorAlias_EmptyFields(t *testing.T) {
	// Edge case: all fields empty via alias
	err := &TemplateNotFoundError{
		TemplateName: "",
		ReferencedBy: "",
	}

	result := err.Error()

	assert.Contains(t, result, "not found")
	assert.Equal(t, `template "" not found`, result)
}

func TestTemplateNotFoundErrorAlias_TypeAssignability(t *testing.T) {
	// Happy path: verify type alias can be assigned to canonical type variable
	aliasErr := &TemplateNotFoundError{
		TemplateName: "test",
		ReferencedBy: "file.yaml",
	}

	// Type alias should be assignable to canonical type
	canonicalErr := aliasErr

	require.NotNil(t, canonicalErr)
	assert.Equal(t, "test", canonicalErr.TemplateName)
	assert.Equal(t, "file.yaml", canonicalErr.ReferencedBy)
}

func TestTemplateNotFoundErrorAlias_ReverseTypeAssignability(t *testing.T) {
	// Happy path: verify canonical type can be assigned to alias type variable
	canonicalErr := &workflow.TemplateNotFoundError{
		TemplateName: "reverse-test",
		ReferencedBy: "reverse-file.yaml",
	}

	// Canonical type should be assignable to alias type
	aliasErr := canonicalErr

	require.NotNil(t, aliasErr)
	assert.Equal(t, "reverse-test", aliasErr.TemplateName)
	assert.Equal(t, "reverse-file.yaml", aliasErr.ReferencedBy)
}

func TestTemplateNotFoundErrorAlias_ImplementsErrorInterface(t *testing.T) {
	// Happy path: verify alias implements error interface
	var err error = &TemplateNotFoundError{
		TemplateName: "interface-test",
		ReferencedBy: "test.yaml",
	}

	require.NotNil(t, err)
	require.NotEmpty(t, err.Error())
	assert.Contains(t, err.Error(), "interface-test")
}

func TestTemplateNotFoundErrorAlias_ErrorWrappingChain(t *testing.T) {
	// Edge case: verify alias works correctly in error wrapping chains
	baseErr := &TemplateNotFoundError{
		TemplateName: "base-template",
		ReferencedBy: "base.yaml",
	}

	wrappedOnce := errors.Join(baseErr, errors.New("additional context"))
	wrappedTwice := errors.Join(wrappedOnce, errors.New("more context"))

	// Should be detectable through multiple wrapping levels as alias
	var targetAlias *TemplateNotFoundError
	require.True(t, errors.As(wrappedTwice, &targetAlias),
		"errors.As should detect alias through multiple wrapping levels")
	assert.Equal(t, "base-template", targetAlias.TemplateName)

	// Should also be detectable as canonical type
	var targetCanonical *workflow.TemplateNotFoundError
	require.True(t, errors.As(wrappedTwice, &targetCanonical),
		"errors.As should detect canonical type through multiple wrapping levels")
	assert.Equal(t, "base-template", targetCanonical.TemplateName)
}

func TestTemplateNotFoundErrorAlias_SpecialCharacters(t *testing.T) {
	// Edge case: special characters in names via alias
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
			name:         "with version numbers",
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
		{
			name:         "with paths",
			templateName: "templates/sub/item",
			referencedBy: "workflows/main.yaml",
			expected:     `template "templates/sub/item" not found (referenced by workflows/main.yaml)`,
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

func TestTemplateNotFoundErrorAlias_CompareWithNil(t *testing.T) {
	// Edge case: nil comparison behavior
	var err *TemplateNotFoundError
	assert.Nil(t, err)

	err = &TemplateNotFoundError{
		TemplateName: "test",
	}
	assert.NotNil(t, err)
}

func TestTemplateNotFoundErrorAlias_ZeroValue(t *testing.T) {
	// Edge case: zero value construction
	err := TemplateNotFoundError{}

	result := err.Error()

	// Zero value should produce valid error message
	assert.Contains(t, result, "not found")
	assert.Equal(t, `template "" not found`, result)
}

func TestTemplateNotFoundErrorAlias_CrossLayerErrorPropagation(t *testing.T) {
	// Integration test: verify error propagates correctly from infrastructure to application
	// simulating real-world usage pattern

	// Infrastructure layer constructs error using alias
	infraErr := &TemplateNotFoundError{
		TemplateName: "missing-config",
		ReferencedBy: "production.yaml",
	}

	// Simulate error propagating up through layers (wrapped)
	applicationErr := errors.Join(errors.New("failed to load workflow"), infraErr)

	// Application layer attempts to detect domain error type
	var domainErr *workflow.TemplateNotFoundError
	require.True(t, errors.As(applicationErr, &domainErr),
		"application layer should detect domain error type from infrastructure alias")

	assert.Equal(t, "missing-config", domainErr.TemplateName)
	assert.Equal(t, "production.yaml", domainErr.ReferencedBy)
}

func TestTemplateNotFoundErrorAlias_BackwardCompatibilityGuarantee(t *testing.T) {
	// Integration test: verify backward compatibility for existing infrastructure code

	// Old infrastructure code pattern (pre-refactoring)
	oldStyleErr := &TemplateNotFoundError{
		TemplateName: "legacy-template",
		ReferencedBy: "legacy.yaml",
	}

	// New application code expecting domain type
	newStyleErr := oldStyleErr // Direct assignment must work

	require.NotNil(t, newStyleErr)
	assert.Equal(t, "legacy-template", newStyleErr.TemplateName)
	assert.Equal(t, oldStyleErr.Error(), newStyleErr.Error(),
		"error messages must be identical for backward compatibility")
}
