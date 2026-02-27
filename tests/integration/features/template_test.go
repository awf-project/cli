//go:build integration

package features_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const templatesFixturePath = "../../fixtures/templates"

func TestTemplateRepository_LoadFromFixtures_Integration(t *testing.T) {
	repo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})

	tests := []struct {
		name           string
		templateName   string
		expectedParams int
		hasRequired    bool
	}{
		{
			name:           "simple-echo template",
			templateName:   "simple-echo",
			expectedParams: 2,
			hasRequired:    true,
		},
		{
			name:           "ai-analyze template",
			templateName:   "ai-analyze",
			expectedParams: 3,
			hasRequired:    true,
		},
		{
			name:           "no-params template",
			templateName:   "no-params",
			expectedParams: 0,
			hasRequired:    false,
		},
		{
			name:           "multi-state template",
			templateName:   "multi-state",
			expectedParams: 1,
			hasRequired:    true,
		},
		{
			name:           "all-required template",
			templateName:   "all-required",
			expectedParams: 3,
			hasRequired:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			tmpl, err := repo.GetTemplate(ctx, tt.templateName)
			require.NoError(t, err)
			require.NotNil(t, tmpl)

			assert.Equal(t, tt.templateName, tmpl.Name)
			assert.Len(t, tmpl.Parameters, tt.expectedParams)

			if tt.hasRequired {
				required := tmpl.GetRequiredParams()
				assert.NotEmpty(t, required)
			}

			// Validate template is well-formed
			err = tmpl.Validate()
			require.NoError(t, err)
		})
	}
}

func TestTemplateRepository_ListTemplates_Integration(t *testing.T) {
	repo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	ctx := context.Background()

	names, err := repo.ListTemplates(ctx)
	require.NoError(t, err)

	// Should find our test fixtures
	expectedTemplates := []string{
		"simple-echo",
		"ai-analyze",
		"no-params",
		"multi-state",
		"all-required",
	}

	for _, expected := range expectedTemplates {
		assert.Contains(t, names, expected, "should list template: %s", expected)
	}
}

func TestTemplateRepository_Exists_Integration(t *testing.T) {
	repo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	ctx := context.Background()

	assert.True(t, repo.Exists(ctx, "simple-echo"))
	assert.True(t, repo.Exists(ctx, "ai-analyze"))
	assert.False(t, repo.Exists(ctx, "nonexistent-template"))
}

func TestTemplateService_ExpandWorkflow_Integration(t *testing.T) {
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	logger := &mockLogger{}

	svc := application.NewTemplateService(templateRepo, logger)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				OnSuccess: "done",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "simple-echo",
					Parameters: map[string]any{
						"message": "Integration Test Message",
					},
				},
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

	ctx := context.Background()
	err := svc.ExpandWorkflow(ctx, wf)
	require.NoError(t, err)

	// Verify expansion
	step := wf.Steps["step1"]
	require.NotNil(t, step)

	assert.Equal(t, workflow.StepTypeCommand, step.Type)
	assert.Contains(t, step.Command, "Integration Test Message")
	assert.Contains(t, step.Command, "[INFO]") // default prefix
	assert.Nil(t, step.TemplateRef)
}

func TestTemplateService_ExpandWorkflow_OverrideDefaults_Integration(t *testing.T) {
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	logger := &mockLogger{}

	svc := application.NewTemplateService(templateRepo, logger)

	wf := &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "analyze",
		Steps: map[string]*workflow.Step{
			"analyze": {
				Name:      "analyze",
				OnSuccess: "done",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "ai-analyze",
					Parameters: map[string]any{
						"prompt": "Analyze the following code",
						"model":  "gemini", // override default "claude"
					},
				},
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal},
			"error": {Name: "error", Type: workflow.StepTypeTerminal},
		},
	}

	ctx := context.Background()
	err := svc.ExpandWorkflow(ctx, wf)
	require.NoError(t, err)

	step := wf.Steps["analyze"]
	assert.Contains(t, step.Command, "gemini")
	assert.NotContains(t, step.Command, "claude")
	assert.Contains(t, step.Command, "Analyze the following code")
}

func TestTemplateService_ValidateTemplateRef_Integration(t *testing.T) {
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	logger := &mockLogger{}

	svc := application.NewTemplateService(templateRepo, logger)
	ctx := context.Background()

	t.Run("valid reference", func(t *testing.T) {
		ref := &workflow.WorkflowTemplateRef{
			TemplateName: "simple-echo",
			Parameters: map[string]any{
				"message": "test",
			},
		}
		err := svc.ValidateTemplateRef(ctx, ref)
		require.NoError(t, err)
	})

	t.Run("missing required parameter", func(t *testing.T) {
		ref := &workflow.WorkflowTemplateRef{
			TemplateName: "all-required",
			Parameters: map[string]any{
				"source": "src", // missing "destination" and "format"
			},
		}
		err := svc.ValidateTemplateRef(ctx, ref)
		require.Error(t, err)

		var missingErr *workflow.MissingParameterError
		require.ErrorAs(t, err, &missingErr)
	})

	t.Run("template not found", func(t *testing.T) {
		ref := &workflow.WorkflowTemplateRef{
			TemplateName: "nonexistent",
			Parameters:   map[string]any{},
		}
		err := svc.ValidateTemplateRef(ctx, ref)
		require.Error(t, err)

		var notFoundErr *repository.TemplateNotFoundError
		require.ErrorAs(t, err, &notFoundErr)
	})
}

func TestWorkflowWithTemplate_Execution_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "output.log")

	// Create workflow that uses a template
	wfYAML := `name: template-workflow
version: "1.0.0"
states:
  initial: echo_step
  echo_step:
    type: command
    command: 'echo "Hello from template workflow" > ` + logFile + `'
    on_success: done
  done:
    type: terminal
    status: success
`
	err := os.WriteFile(filepath.Join(tmpDir, "template-wf.yaml"), []byte(wfYAML), 0o644)
	require.NoError(t, err)

	repo := repository.NewYAMLRepository(tmpDir)
	store := newMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	wfSvc := application.NewWorkflowService(repo, store, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, store, logger, resolver, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "template-wf", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify output
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, "Hello from template workflow\n", string(data))
}

func TestTemplateService_CircularReference_Integration(t *testing.T) {
	// The circular-a and circular-b fixtures reference each other
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	logger := &mockLogger{}

	svc := application.NewTemplateService(templateRepo, logger)

	wf := &workflow.Workflow{
		Name:    "circular-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "circular-a",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	ctx := context.Background()
	err := svc.ExpandWorkflow(ctx, wf)

	// Should detect circular reference
	require.Error(t, err)

	var circularErr *workflow.CircularTemplateError
	require.ErrorAs(t, err, &circularErr)
}

func TestTemplateService_InvalidTemplate_Integration(t *testing.T) {
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})

	t.Run("invalid syntax", func(t *testing.T) {
		ctx := context.Background()
		_, err := templateRepo.GetTemplate(ctx, "invalid-syntax")
		require.Error(t, err)
	})

	t.Run("missing name", func(t *testing.T) {
		ctx := context.Background()
		_, err := templateRepo.GetTemplate(ctx, "invalid-missing-name")
		require.Error(t, err)
	})
}

func TestMultipleTemplates_SameWorkflow_Integration(t *testing.T) {
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	logger := &mockLogger{}

	svc := application.NewTemplateService(templateRepo, logger)

	wf := &workflow.Workflow{
		Name:    "multi-template-workflow",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				OnSuccess: "step2",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "simple-echo",
					Parameters: map[string]any{
						"message": "First template",
					},
				},
			},
			"step2": {
				Name:      "step2",
				OnSuccess: "step3",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "ai-analyze",
					Parameters: map[string]any{
						"prompt": "Analyze step1 output",
					},
				},
			},
			"step3": {
				Name:      "step3",
				OnSuccess: "done",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "no-params",
					Parameters:   map[string]any{},
				},
			},
			"done":  {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			"error": {Name: "error", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		},
	}

	ctx := context.Background()
	err := svc.ExpandWorkflow(ctx, wf)
	require.NoError(t, err)

	// All template refs should be expanded
	assert.Nil(t, wf.Steps["step1"].TemplateRef)
	assert.Nil(t, wf.Steps["step2"].TemplateRef)
	assert.Nil(t, wf.Steps["step3"].TemplateRef)

	// All steps should have commands
	assert.NotEmpty(t, wf.Steps["step1"].Command)
	assert.NotEmpty(t, wf.Steps["step2"].Command)
	assert.NotEmpty(t, wf.Steps["step3"].Command)

	// Verify parameter substitution
	assert.Contains(t, wf.Steps["step1"].Command, "First template")
	assert.Contains(t, wf.Steps["step2"].Command, "Analyze step1 output")
}

func TestTemplateService_ComplexParameters_Integration(t *testing.T) {
	templateRepo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	logger := &mockLogger{}

	svc := application.NewTemplateService(templateRepo, logger)

	t.Run("all required params provided", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test",
			Initial: "convert",
			Steps: map[string]*workflow.Step{
				"convert": {
					Name:      "convert",
					OnSuccess: "done",
					TemplateRef: &workflow.WorkflowTemplateRef{
						TemplateName: "all-required",
						Parameters: map[string]any{
							"source":      "/path/to/source.txt",
							"destination": "/path/to/dest.txt",
							"format":      "json",
						},
					},
				},
				"done": {Name: "done", Type: workflow.StepTypeTerminal},
			},
		}

		ctx := context.Background()
		err := svc.ExpandWorkflow(ctx, wf)
		require.NoError(t, err)

		assert.Contains(t, wf.Steps["convert"].Command, "/path/to/source.txt")
		assert.Contains(t, wf.Steps["convert"].Command, "/path/to/dest.txt")
		assert.Contains(t, wf.Steps["convert"].Command, "json")
	})

	t.Run("missing one required param", func(t *testing.T) {
		wf := &workflow.Workflow{
			Name:    "test",
			Initial: "convert",
			Steps: map[string]*workflow.Step{
				"convert": {
					Name:      "convert",
					OnSuccess: "done",
					TemplateRef: &workflow.WorkflowTemplateRef{
						TemplateName: "all-required",
						Parameters: map[string]any{
							"source": "/path/to/source.txt",
							"format": "json",
							// missing "destination"
						},
					},
				},
				"done": {Name: "done", Type: workflow.StepTypeTerminal},
			},
		}

		ctx := context.Background()
		err := svc.ExpandWorkflow(ctx, wf)
		require.Error(t, err)

		var missingErr *workflow.MissingParameterError
		require.ErrorAs(t, err, &missingErr)
		assert.Equal(t, "destination", missingErr.ParameterName)
	})
}

func TestTemplateRepository_PathPriority_Integration(t *testing.T) {
	// Create temp directory with override template
	tmpDir := t.TempDir()
	overrideContent := `
name: simple-echo
parameters:
  - name: message
    required: true
states:
  initial: echo
  echo:
    type: command
    command: "OVERRIDE: {{parameters.message}}"
`
	err := os.WriteFile(filepath.Join(tmpDir, "simple-echo.yaml"), []byte(overrideContent), 0o644)
	require.NoError(t, err)

	// First path takes priority
	repo := repository.NewYAMLTemplateRepository([]string{tmpDir, templatesFixturePath})

	ctx := context.Background()
	tmpl, err := repo.GetTemplate(ctx, "simple-echo")
	require.NoError(t, err)

	// Should get the override version (only 1 param, not 2)
	assert.Len(t, tmpl.Parameters, 1)

	// Expand and check command
	logger := &mockLogger{}
	svc := application.NewTemplateService(repo, logger)

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "simple-echo",
					Parameters: map[string]any{
						"message": "test",
					},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err = svc.ExpandWorkflow(ctx, wf)
	require.NoError(t, err)

	assert.Contains(t, wf.Steps["step"].Command, "OVERRIDE:")
}

func TestTemplateRepository_Caching_Integration(t *testing.T) {
	repo := repository.NewYAMLTemplateRepository([]string{templatesFixturePath})
	ctx := context.Background()

	// First load
	tmpl1, err := repo.GetTemplate(ctx, "simple-echo")
	require.NoError(t, err)

	// Second load (should be cached)
	tmpl2, err := repo.GetTemplate(ctx, "simple-echo")
	require.NoError(t, err)

	// Should be the same pointer
	assert.Same(t, tmpl1, tmpl2)
}
