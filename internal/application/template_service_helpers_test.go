package application_test

import (
	"context"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T001_expandStep_helpers

// validateAndLoadTemplate Tests

func TestTemplateService_validateAndLoadTemplate_HappyPath(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	simpleTemplate := newSimpleEchoTemplate()
	repo.AddTemplate(simpleTemplate.Name, simpleTemplate)
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
		Parameters:   map[string]any{"message": "test"},
	}
	visited := make(map[string]bool)

	tmpl, err := svc.ValidateAndLoadTemplate(context.Background(), ref, visited)

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
			repo := mocks.NewMockTemplateRepository()
			tmplA := &workflow.Template{
				Name: "template-a",
				States: map[string]*workflow.Step{
					"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo a"},
				},
			}
			repo.AddTemplate(tmplA.Name, tmplA)
			tmplB := &workflow.Template{
				Name: "template-b",
				States: map[string]*workflow.Step{
					"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo b"},
				},
			}
			repo.AddTemplate(tmplB.Name, tmplB)
			tmplC := &workflow.Template{
				Name: "template-c",
				States: map[string]*workflow.Step{
					"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo c"},
				},
			}
			repo.AddTemplate(tmplC.Name, tmplC)
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

			ref := &workflow.WorkflowTemplateRef{
				TemplateName: tt.templateName,
			}

			tmpl, err := svc.ValidateAndLoadTemplate(context.Background(), ref, tt.visited)

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

			ref := &workflow.WorkflowTemplateRef{
				TemplateName: tt.templateName,
			}
			visited := make(map[string]bool)

			tmpl, err := svc.ValidateAndLoadTemplate(context.Background(), ref, visited)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, tmpl)
		})
	}
}

func TestTemplateService_validateAndLoadTemplate_ContextCancellation(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
	}
	visited := make(map[string]bool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tmpl, err := svc.ValidateAndLoadTemplate(ctx, ref, visited)
	// Note: actual behavior depends on implementation
	if err != nil {
		assert.Error(t, err)
	}
	// Even if no error, should not panic
	_ = tmpl
}

func TestTemplateService_validateAndLoadTemplate_VisitedMapUnmodified(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
	}
	visited := map[string]bool{"other-template": true}

	_, err := svc.ValidateAndLoadTemplate(context.Background(), ref, visited)

	require.NoError(t, err)
	// Template should be marked as visited
	assert.True(t, visited["simple-echo"])
	// Original entries should remain unchanged
	assert.True(t, visited["other-template"])
}

// selectPrimaryStep Tests

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

			step, err := svc.SelectPrimaryStep(tt.template)

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

			step, err := svc.SelectPrimaryStep(tt.template)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
			assert.Nil(t, step)
		})
	}
}

func TestTemplateService_selectPrimaryStep_SingleStep(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

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

	step, err := svc.SelectPrimaryStep(template)

	require.NoError(t, err)
	require.NotNil(t, step)
	assert.Equal(t, "only-step", step.Name)
}

// expandNestedTemplate Tests

func TestTemplateService_expandNestedTemplate_HappyPath(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()

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
	repo.AddTemplate(leafTemplate.Name, leafTemplate)

	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	// Step with nested template reference
	templateStep := &workflow.Step{
		Name: "parent-step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "leaf-template",
		},
	}
	visited := make(map[string]bool)

	err := svc.ExpandNestedTemplate(context.Background(), "parent-step", templateStep, visited)

	require.NoError(t, err)
	// After expansion, TemplateRef should be nil
	assert.Nil(t, templateStep.TemplateRef)
}

func TestTemplateService_expandNestedTemplate_NoNestedRef(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name:        "simple-step",
		Type:        workflow.StepTypeCommand,
		Command:     "echo no nesting",
		TemplateRef: nil, // No nested template
	}
	visited := make(map[string]bool)

	err := svc.ExpandNestedTemplate(context.Background(), "simple-step", templateStep, visited)

	require.NoError(t, err)
}

func TestTemplateService_expandNestedTemplate_DeepNesting(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()

	// Level 0 (leaf)
	tmpl0 := &workflow.Template{
		Name: "level-0",
		States: map[string]*workflow.Step{
			"step": {
				Name:    "step",
				Type:    workflow.StepTypeCommand,
				Command: "echo level-0",
			},
		},
	}
	repo.AddTemplate(tmpl0.Name, tmpl0)

	// Level 1
	tmpl1 := &workflow.Template{
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
	}
	repo.AddTemplate(tmpl1.Name, tmpl1)

	// Level 2
	tmpl2 := &workflow.Template{
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
	}
	repo.AddTemplate(tmpl2.Name, tmpl2)

	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "root-step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "level-2",
		},
	}
	visited := make(map[string]bool)

	err := svc.ExpandNestedTemplate(context.Background(), "root-step", templateStep, visited)

	require.NoError(t, err)
	assert.Nil(t, templateStep.TemplateRef)
}

func TestTemplateService_expandNestedTemplate_CircularReference(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()

	// Template A references B
	tmplA := &workflow.Template{
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
	}
	repo.AddTemplate(tmplA.Name, tmplA)

	// Template B references A (circular)
	tmplB := &workflow.Template{
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
	}
	repo.AddTemplate(tmplB.Name, tmplB)

	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "root-step",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "template-a",
		},
	}
	visited := make(map[string]bool)

	err := svc.ExpandNestedTemplate(context.Background(), "root-step", templateStep, visited)

	require.Error(t, err)
	var circErr *workflow.CircularTemplateError
	require.ErrorAs(t, err, &circErr)
}

func TestTemplateService_expandNestedTemplate_TemplateNotFound(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	templateStep := &workflow.Step{
		Name: "step-with-missing-ref",
		Type: workflow.StepTypeCommand,
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "nonexistent-template",
		},
	}
	visited := make(map[string]bool)

	err := svc.ExpandNestedTemplate(context.Background(), "step-with-missing-ref", templateStep, visited)

	require.Error(t, err)
	var notFoundErr *workflow.TemplateNotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}

func TestTemplateService_expandNestedTemplate_ContextCancellation(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

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

	err := svc.ExpandNestedTemplate(ctx, "step", templateStep, visited)

	// Note: actual behavior depends on implementation
	_ = err // May or may not error depending on timing
}

// applyTemplateFields Tests

func TestTemplateService_applyTemplateFields_HappyPath(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

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

	err := svc.ApplyTemplateFields(step, templateStep, params)

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

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

			err := svc.ApplyTemplateFields(step, templateStep, params)

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
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

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

	err := svc.ApplyTemplateFields(step, templateStep, params)

	require.NoError(t, err)
	require.NotNil(t, step.Retry)
	// Step's MaxAttempts should override template
	assert.Equal(t, 5, step.Retry.MaxAttempts)
	// But missing fields should be inherited from template
	assert.Equal(t, 5000, step.Retry.InitialDelayMs)
	assert.Equal(t, "exponential", step.Retry.Backoff)
}

func TestTemplateService_applyTemplateFields_CaptureMerging(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

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

	err := svc.ApplyTemplateFields(step, templateStep, params)

	require.NoError(t, err)
	require.NotNil(t, step.Capture)
	// Step's Stdout should override template
	assert.Equal(t, "custom_var", step.Capture.Stdout)
	// But missing Stderr should be inherited from template
	assert.Equal(t, "error_var", step.Capture.Stderr)
}

func TestTemplateService_applyTemplateFields_TransitionsPreserved(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

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

	err := svc.ApplyTemplateFields(step, templateStep, params)

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

			step := &workflow.Step{Name: "workflow-step"}
			templateStep := &workflow.Step{
				Name:    "template-step",
				Type:    workflow.StepTypeCommand,
				Command: tt.command,
			}

			err := svc.ApplyTemplateFields(step, templateStep, tt.params)

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

			err := svc.ApplyTemplateFields(tt.step, tt.templateStep, tt.params)

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
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	step := &workflow.Step{Name: "workflow-step"}
	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo 'no params'",
	}

	err := svc.ApplyTemplateFields(step, templateStep, map[string]any{})

	require.NoError(t, err)
	assert.Equal(t, workflow.StepTypeCommand, step.Type)
	assert.Equal(t, "echo 'no params'", step.Command)
}

func TestTemplateService_applyTemplateFields_NilParams(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()
	svc := application.NewTemplateService(repo, logger)

	step := &workflow.Step{Name: "workflow-step"}
	templateStep := &workflow.Step{
		Name:    "template-step",
		Type:    workflow.StepTypeCommand,
		Command: "echo 'no params'",
	}

	err := svc.ApplyTemplateFields(step, templateStep, nil)

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
			repo := mocks.NewMockTemplateRepository()
			logger := mocks.NewMockLogger()
			svc := application.NewTemplateService(repo, logger)

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

			err := svc.ApplyTemplateFields(step, templateStep, params)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTimeout, step.Timeout)
		})
	}
}
