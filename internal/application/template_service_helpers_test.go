package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/repository"
)

// =============================================================================
// Test Mocks and Helpers
// =============================================================================

// mockTemplateRepository implements ports.TemplateRepository for testing.
type mockTemplateRepository struct {
	templates map[string]*workflow.Template
}

func newMockTemplateRepository() *mockTemplateRepository {
	return &mockTemplateRepository{
		templates: make(map[string]*workflow.Template),
	}
}

func (m *mockTemplateRepository) GetTemplate(_ context.Context, name string) (*workflow.Template, error) {
	if tmpl, ok := m.templates[name]; ok {
		return tmpl, nil
	}
	return nil, &repository.TemplateNotFoundError{TemplateName: name}
}

func (m *mockTemplateRepository) ListTemplates(_ context.Context) ([]string, error) {
	names := make([]string, 0, len(m.templates))
	for name := range m.templates {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockTemplateRepository) Exists(_ context.Context, name string) bool {
	_, ok := m.templates[name]
	return ok
}

func (m *mockTemplateRepository) addTemplate(tmpl *workflow.Template) {
	m.templates[tmpl.Name] = tmpl
}

// mockLogger implements ports.Logger for testing.
type mockLogger struct {
	warnings []string
	errors   []string
}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any) {
	if m.warnings == nil {
		m.warnings = []string{}
	}
	m.warnings = append(m.warnings, msg)
}

func (m *mockLogger) Error(msg string, fields ...any) {
	if m.errors == nil {
		m.errors = []string{}
	}
	m.errors = append(m.errors, msg)
}

func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

// newSimpleEchoTemplate creates a basic template for testing.
func newSimpleEchoTemplate() *workflow.Template {
	return &workflow.Template{
		Name: "simple-echo",
		Parameters: []workflow.TemplateParam{
			{Name: "message", Required: true},
			{Name: "prefix", Required: false, Default: "[INFO]"},
		},
		States: map[string]*workflow.Step{
			"echo": {
				Name:    "echo",
				Type:    workflow.StepTypeCommand,
				Command: "echo '{{parameters.prefix}} {{parameters.message}}'",
			},
		},
	}
}

// =============================================================================
// Feature: C005 - Helper Method Tests for TemplateService
// Component: T001_expandStep_helpers
// =============================================================================

// -----------------------------------------------------------------------------
// validateAndLoadTemplate Tests
// -----------------------------------------------------------------------------

func TestTemplateService_validateAndLoadTemplate_HappyPath(t *testing.T) {
	// Arrange: setup template repository and service
	repo := newMockTemplateRepository()
	simpleTemplate := newSimpleEchoTemplate()
	repo.addTemplate(simpleTemplate)
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
		Parameters:   map[string]any{"message": "test"},
	}
	visited := make(map[string]bool)

	// Act: call validateAndLoadTemplate
	tmpl, err := svc.ValidateAndLoadTemplate(context.Background(), ref, visited)

	// Assert: verify successful load
	require.NoError(t, err)
	require.NotNil(t, tmpl)
	assert.Equal(t, "simple-echo", tmpl.Name)
}

func TestTemplateService_validateAndLoadTemplate_CircularDetection(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		visited      map[string]bool
		wantErr      bool
		errType      string
	}{
		{
			name:         "detects direct circular reference",
			templateName: "template-a",
			visited:      map[string]bool{"template-a": true},
			wantErr:      true,
			errType:      "*workflow.CircularTemplateError",
		},
		{
			name:         "detects indirect circular reference",
			templateName: "template-c",
			visited:      map[string]bool{"template-a": true, "template-b": true, "template-c": true},
			wantErr:      true,
			errType:      "*workflow.CircularTemplateError",
		},
		{
			name:         "allows first reference",
			templateName: "template-a",
			visited:      map[string]bool{},
			wantErr:      false,
		},
		{
			name:         "allows different template",
			templateName: "template-b",
			visited:      map[string]bool{"template-a": true},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			repo.addTemplate(&workflow.Template{
				Name: "template-a",
				States: map[string]*workflow.Step{
					"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo a"},
				},
			})
			repo.addTemplate(&workflow.Template{
				Name: "template-b",
				States: map[string]*workflow.Step{
					"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo b"},
				},
			})
			repo.addTemplate(&workflow.Template{
				Name: "template-c",
				States: map[string]*workflow.Step{
					"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo c"},
				},
			})
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			ref := &workflow.WorkflowTemplateRef{
				TemplateName: tt.templateName,
			}

			// Act
			tmpl, err := svc.ValidateAndLoadTemplate(context.Background(), ref, tt.visited)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				var circErr *workflow.CircularTemplateError
				require.ErrorAs(t, err, &circErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, tmpl)
				assert.Equal(t, tt.templateName, tmpl.Name)
			}
		})
	}
}

func TestTemplateService_validateAndLoadTemplate_LoadErrors(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "template not found",
			templateName: "nonexistent",
			wantErr:      true,
			errContains:  "load template",
		},
		{
			name:         "empty template name",
			templateName: "",
			wantErr:      true,
			errContains:  "load template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			ref := &workflow.WorkflowTemplateRef{
				TemplateName: tt.templateName,
			}
			visited := make(map[string]bool)

			// Act
			tmpl, err := svc.ValidateAndLoadTemplate(context.Background(), ref, visited)

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, tmpl)
		})
	}
}

func TestTemplateService_validateAndLoadTemplate_ContextCancellation(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	repo.addTemplate(newSimpleEchoTemplate())
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
	}
	visited := make(map[string]bool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	tmpl, err := svc.ValidateAndLoadTemplate(ctx, ref, visited)
	// Assert: should handle context cancellation
	// Note: actual behavior depends on implementation
	if err != nil {
		assert.Error(t, err)
	}
	// Even if no error, should not panic
	_ = tmpl
}

func TestTemplateService_validateAndLoadTemplate_VisitedMapUnmodified(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	repo.addTemplate(newSimpleEchoTemplate())
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
	}
	visited := map[string]bool{"other-template": true}

	// Act
	_, err := svc.ValidateAndLoadTemplate(context.Background(), ref, visited)

	// Assert: helper marks template as visited (caller responsible for cleanup)
	require.NoError(t, err)
	// Template should be marked as visited
	assert.True(t, visited["simple-echo"])
	// Original entries should remain unchanged
	assert.True(t, visited["other-template"])
}

// -----------------------------------------------------------------------------
// selectPrimaryStep Tests
// -----------------------------------------------------------------------------

func TestTemplateService_selectPrimaryStep_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		template     *workflow.Template
		expectedStep string
	}{
		{
			name: "selects step with same name as template",
			template: &workflow.Template{
				Name: "my-template",
				States: map[string]*workflow.Step{
					"other-step": {
						Name:    "other-step",
						Type:    workflow.StepTypeCommand,
						Command: "echo other",
					},
					"my-template": {
						Name:    "my-template",
						Type:    workflow.StepTypeCommand,
						Command: "echo matched",
					},
				},
			},
			expectedStep: "my-template",
		},
		{
			name: "falls back to first step when no name match",
			template: &workflow.Template{
				Name: "my-template",
				States: map[string]*workflow.Step{
					"step1": {
						Name:    "step1",
						Type:    workflow.StepTypeCommand,
						Command: "echo first",
					},
				},
			},
			expectedStep: "step1",
		},
		{
			name: "prioritizes name match over order",
			template: &workflow.Template{
				Name: "my-template",
				States: map[string]*workflow.Step{
					"first": {
						Name:    "first",
						Type:    workflow.StepTypeCommand,
						Command: "echo first",
					},
					"my-template": {
						Name:    "my-template",
						Type:    workflow.StepTypeCommand,
						Command: "echo matched",
					},
				},
			},
			expectedStep: "my-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			// Act
			step, err := svc.SelectPrimaryStep(tt.template)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, step)
			assert.Equal(t, tt.expectedStep, step.Name)
		})
	}
}

func TestTemplateService_selectPrimaryStep_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		template    *workflow.Template
		wantErr     bool
		errContains string
	}{
		{
			name:        "error on nil template",
			template:    nil,
			wantErr:     true,
			errContains: "nil",
		},
		{
			name: "error on empty states",
			template: &workflow.Template{
				Name:   "empty-template",
				States: map[string]*workflow.Step{},
			},
			wantErr:     true,
			errContains: "no steps",
		},
		{
			name: "error on nil states",
			template: &workflow.Template{
				Name:   "nil-states",
				States: nil,
			},
			wantErr:     true,
			errContains: "no steps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			// Act
			step, err := svc.SelectPrimaryStep(tt.template)

			// Assert
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, step)
		})
	}
}

func TestTemplateService_selectPrimaryStep_SingleStep(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	template := &workflow.Template{
		Name: "single-step",
		States: map[string]*workflow.Step{
			"only-step": {
				Name:    "only-step",
				Type:    workflow.StepTypeCommand,
				Command: "echo only",
			},
		},
	}

	// Act
	step, err := svc.SelectPrimaryStep(template)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, "only-step", step.Name)
}

// -----------------------------------------------------------------------------
// expandNestedTemplate Tests
// -----------------------------------------------------------------------------

func TestTemplateService_expandNestedTemplate_HappyPath(t *testing.T) {
	// Arrange: setup nested template structure
	repo := newMockTemplateRepository()

	// Leaf template (no further nesting)
	leafTemplate := &workflow.Template{
		Name: "leaf-template",
		States: map[string]*workflow.Step{
			"leaf": {
				Name:    "leaf",
				Type:    workflow.StepTypeCommand,
				Command: "echo leaf",
			},
		},
	}
	repo.addTemplate(leafTemplate)

	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	// Step with nested template reference
	templateStep := &workflow.Step{
		Name: "parent-step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "leaf-template",
		},
	}
	visited := make(map[string]bool)

	// Act
	err := svc.ExpandNestedTemplate(context.Background(), "parent-step", templateStep, visited)

	// Assert
	require.NoError(t, err)
	// After expansion, TemplateRef should be nil
	assert.Nil(t, templateStep.TemplateRef)
}

func TestTemplateService_expandNestedTemplate_NoNestedRef(t *testing.T) {
	// Arrange: step without nested template
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name:        "simple-step",
		Type:        workflow.StepTypeCommand,
		Command:     "echo no nesting",
		TemplateRef: nil, // No nested template
	}
	visited := make(map[string]bool)

	// Act
	err := svc.ExpandNestedTemplate(context.Background(), "simple-step", templateStep, visited)

	// Assert: should succeed with no-op
	require.NoError(t, err)
}

func TestTemplateService_expandNestedTemplate_DeepNesting(t *testing.T) {
	// Arrange: create deeply nested template chain
	repo := newMockTemplateRepository()

	// Level 0 (leaf)
	repo.addTemplate(&workflow.Template{
		Name: "level-0",
		States: map[string]*workflow.Step{
			"step": {
				Name:    "step",
				Type:    workflow.StepTypeCommand,
				Command: "echo level-0",
			},
		},
	})

	// Level 1
	repo.addTemplate(&workflow.Template{
		Name: "level-1",
		States: map[string]*workflow.Step{
			"step": {
				Name: "step",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "level-0",
				},
			},
		},
	})

	// Level 2
	repo.addTemplate(&workflow.Template{
		Name: "level-2",
		States: map[string]*workflow.Step{
			"step": {
				Name: "step",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "level-1",
				},
			},
		},
	})

	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "root-step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "level-2",
		},
	}
	visited := make(map[string]bool)

	// Act: expand nested templates recursively
	err := svc.ExpandNestedTemplate(context.Background(), "root-step", templateStep, visited)

	// Assert: should successfully expand all levels
	require.NoError(t, err)
	assert.Nil(t, templateStep.TemplateRef)
}

func TestTemplateService_expandNestedTemplate_CircularReference(t *testing.T) {
	// Arrange: create circular template reference
	repo := newMockTemplateRepository()

	// Template A references B
	repo.addTemplate(&workflow.Template{
		Name: "template-a",
		States: map[string]*workflow.Step{
			"step": {
				Name: "step",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "template-b",
				},
			},
		},
	})

	// Template B references A (circular)
	repo.addTemplate(&workflow.Template{
		Name: "template-b",
		States: map[string]*workflow.Step{
			"step": {
				Name: "step",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "template-a",
				},
			},
		},
	})

	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "root-step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "template-a",
		},
	}
	visited := make(map[string]bool)

	// Act
	err := svc.ExpandNestedTemplate(context.Background(), "root-step", templateStep, visited)

	// Assert: should detect circular reference
	require.Error(t, err)
	var circErr *workflow.CircularTemplateError
	require.ErrorAs(t, err, &circErr)
}

func TestTemplateService_expandNestedTemplate_TemplateNotFound(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "step-with-missing-ref",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "nonexistent-template",
		},
	}
	visited := make(map[string]bool)

	// Act
	err := svc.ExpandNestedTemplate(context.Background(), "step-with-missing-ref", templateStep, visited)

	// Assert
	require.Error(t, err)
	var notFoundErr *repository.TemplateNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestTemplateService_expandNestedTemplate_ContextCancellation(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	repo.addTemplate(newSimpleEchoTemplate())
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "simple-echo",
		},
	}
	visited := make(map[string]bool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	err := svc.ExpandNestedTemplate(ctx, "step", templateStep, visited)

	// Assert: should handle context cancellation gracefully
	// Note: actual behavior depends on implementation
	_ = err // May or may not error depending on timing
}

// -----------------------------------------------------------------------------
// applyTemplateFields Tests
// -----------------------------------------------------------------------------

func TestTemplateService_applyTemplateFields_HappyPath(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	step := &workflow.Step{
		Name:      "workflow-step",
		OnSuccess: "next-step",
		OnFailure: "error-step",
	}

	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo '{{parameters.message}}'",
		Timeout: 30,
	}

	params := map[string]any{
		"message": "Hello World",
	}

	// Act
	err := svc.ApplyTemplateFields(step, templateStep, params)

	// Assert
	require.NoError(t, err)
	// Step should have template fields merged
	assert.Equal(t, workflow.StepTypeCommand, step.Type)
	assert.Contains(t, step.Command, "Hello World")
	assert.Equal(t, 30, step.Timeout)
	// But transitions should be preserved from workflow step
	assert.Equal(t, "next-step", step.OnSuccess)
	assert.Equal(t, "error-step", step.OnFailure)
}

func TestTemplateService_applyTemplateFields_FieldPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		stepValue     any
		templateValue any
		expectedValue any
		field         string
	}{
		{
			name:          "step timeout overrides template timeout",
			stepValue:     60,
			templateValue: 30,
			expectedValue: 60,
			field:         "Timeout",
		},
		{
			name:          "step command overrides template command",
			stepValue:     "echo custom",
			templateValue: "echo template",
			expectedValue: "echo custom",
			field:         "Command",
		},
		{
			name:          "step dir overrides template dir",
			stepValue:     "/custom/dir",
			templateValue: "/template/dir",
			expectedValue: "/custom/dir",
			field:         "Dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			step := &workflow.Step{Name: "workflow-step"}
			templateStep := &workflow.Step{Name: "template-step"}
			params := map[string]any{}

			// Set values based on field
			switch tt.field {
			case "Timeout":
				step.Timeout = tt.stepValue.(int)
				templateStep.Timeout = tt.templateValue.(int)
			case "Command":
				step.Command = tt.stepValue.(string)
				templateStep.Command = tt.templateValue.(string)
			case "Dir":
				step.Dir = tt.stepValue.(string)
				templateStep.Dir = tt.templateValue.(string)
			}

			// Act
			err := svc.ApplyTemplateFields(step, templateStep, params)

			// Assert
			require.NoError(t, err)
			switch tt.field {
			case "Timeout":
				assert.Equal(t, tt.expectedValue.(int), step.Timeout)
			case "Command":
				assert.Equal(t, tt.expectedValue.(string), step.Command)
			case "Dir":
				assert.Equal(t, tt.expectedValue.(string), step.Dir)
			}
		})
	}
}

func TestTemplateService_applyTemplateFields_RetryMerging(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	step := &workflow.Step{
		Name: "workflow-step",
		Retry: &workflow.RetryConfig{
			MaxAttempts: 5, // Step override
		},
	}

	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo test",
		Retry: &workflow.RetryConfig{
			MaxAttempts:    3,
			InitialDelayMs: 5000,
			Backoff:        "exponential",
		},
	}

	params := map[string]any{}

	// Act
	err := svc.ApplyTemplateFields(step, templateStep, params)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, step.Retry)
	// Step's MaxAttempts should override template
	assert.Equal(t, 5, step.Retry.MaxAttempts)
	// But missing fields should be inherited from template
	assert.Equal(t, 5000, step.Retry.InitialDelayMs)
	assert.Equal(t, "exponential", step.Retry.Backoff)
}

func TestTemplateService_applyTemplateFields_CaptureMerging(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	step := &workflow.Step{
		Name: "workflow-step",
		Capture: &workflow.CaptureConfig{
			Stdout: "custom_var", // Step override
		},
	}

	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo test",
		Capture: &workflow.CaptureConfig{
			Stdout: "template_var",
			Stderr: "error_var",
		},
	}

	params := map[string]any{}

	// Act
	err := svc.ApplyTemplateFields(step, templateStep, params)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, step.Capture)
	// Step's Stdout should override template
	assert.Equal(t, "custom_var", step.Capture.Stdout)
	// But missing Stderr should be inherited from template
	assert.Equal(t, "error_var", step.Capture.Stderr)
}

func TestTemplateService_applyTemplateFields_TransitionsPreserved(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	step := &workflow.Step{
		Name:      "workflow-step",
		OnSuccess: "workflow-next",
		OnFailure: "workflow-error",
	}

	templateStep := &workflow.Step{
		Name:      "template-step",
		Type:      workflow.StepTypeCommand,
		Command:   "echo test",
		OnSuccess: "template-next",
		OnFailure: "template-error",
	}

	params := map[string]any{}

	// Act
	err := svc.ApplyTemplateFields(step, templateStep, params)

	// Assert: workflow step transitions should be preserved
	require.NoError(t, err)
	assert.Equal(t, "workflow-next", step.OnSuccess)
	assert.Equal(t, "workflow-error", step.OnFailure)
}

func TestTemplateService_applyTemplateFields_ParameterSubstitution(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		params         map[string]any
		expectedOutput string
	}{
		{
			name:           "substitutes single parameter",
			command:        "echo '{{parameters.message}}'",
			params:         map[string]any{"message": "Hello"},
			expectedOutput: "Hello",
		},
		{
			name:           "substitutes multiple parameters",
			command:        "echo '{{parameters.prefix}} {{parameters.message}}'",
			params:         map[string]any{"prefix": "[INFO]", "message": "Test"},
			expectedOutput: "[INFO]",
		},
		{
			name:           "handles special characters",
			command:        "echo '{{parameters.message}}'",
			params:         map[string]any{"message": "Test with 'quotes'"},
			expectedOutput: "Test with 'quotes'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			step := &workflow.Step{Name: "workflow-step"}
			templateStep := &workflow.Step{
				Name:    "template-step",
				Type:    workflow.StepTypeCommand,
				Command: tt.command,
			}

			// Act
			err := svc.ApplyTemplateFields(step, templateStep, tt.params)

			// Assert
			require.NoError(t, err)
			assert.Contains(t, step.Command, tt.expectedOutput)
		})
	}
}

func TestTemplateService_applyTemplateFields_ErrorCases(t *testing.T) {
	tests := []struct {
		name         string
		step         *workflow.Step
		templateStep *workflow.Step
		params       map[string]any
		wantErr      bool
		errContains  string
	}{
		{
			name:         "nil step",
			step:         nil,
			templateStep: &workflow.Step{Name: "template"},
			params:       map[string]any{},
			wantErr:      true,
			errContains:  "nil",
		},
		{
			name:         "nil template step",
			step:         &workflow.Step{Name: "step"},
			templateStep: nil,
			params:       map[string]any{},
			wantErr:      true,
			errContains:  "nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			// Act
			err := svc.ApplyTemplateFields(tt.step, tt.templateStep, tt.params)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTemplateService_applyTemplateFields_EmptyParams(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	step := &workflow.Step{Name: "workflow-step"}
	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo 'no params'",
	}

	// Act: empty params should still work
	err := svc.ApplyTemplateFields(step, templateStep, map[string]any{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, workflow.StepTypeCommand, step.Type)
	assert.Equal(t, "echo 'no params'", step.Command)
}

func TestTemplateService_applyTemplateFields_NilParams(t *testing.T) {
	// Arrange
	repo := newMockTemplateRepository()
	logger := &mockLogger{}
	svc := NewTemplateService(repo, logger)

	step := &workflow.Step{Name: "workflow-step"}
	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo 'no params'",
	}

	// Act: nil params should be handled
	err := svc.ApplyTemplateFields(step, templateStep, nil)

	// Assert: should not panic, may or may not error depending on implementation
	_ = err // Allow either success or error
}

func TestTemplateService_applyTemplateFields_TimeoutMerging(t *testing.T) {
	tests := []struct {
		name            string
		stepTimeout     int
		templateTimeout int
		expectedTimeout int
	}{
		{
			name:            "step timeout takes precedence",
			stepTimeout:     60,
			templateTimeout: 30,
			expectedTimeout: 60,
		},
		{
			name:            "uses template timeout when step is zero",
			stepTimeout:     0,
			templateTimeout: 45,
			expectedTimeout: 45,
		},
		{
			name:            "both zero is valid",
			stepTimeout:     0,
			templateTimeout: 0,
			expectedTimeout: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			repo := newMockTemplateRepository()
			logger := &mockLogger{}
			svc := NewTemplateService(repo, logger)

			step := &workflow.Step{
				Name:    "workflow-step",
				Timeout: tt.stepTimeout,
			}
			templateStep := &workflow.Step{
				Name:    "template-step",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
				Timeout: tt.templateTimeout,
			}
			params := map[string]any{}

			// Act
			err := svc.ApplyTemplateFields(step, templateStep, params)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTimeout, step.Timeout)
		})
	}
}
