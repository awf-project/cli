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

func newAIAnalyzeTemplate() *workflow.Template {
	return &workflow.Template{
		Name: "ai-analyze",
		Parameters: []workflow.TemplateParam{
			{Name: "prompt", Required: true},
			{Name: "model", Required: false, Default: "claude"},
			{Name: "timeout", Required: false, Default: "120s"},
		},
		States: map[string]*workflow.Step{
			"analyze": {
				Name:    "analyze",
				Type:    workflow.StepTypeCommand,
				Command: "{{parameters.model}} -c '{{parameters.prompt}}'",
				Timeout: 120,
				Capture: &workflow.CaptureConfig{
					Stdout: "analysis",
				},
			},
		},
	}
}

func newWorkflowWithTemplateRef(templateName string, params map[string]any) *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test-workflow",
		Initial: "templated-step",
		Steps: map[string]*workflow.Step{
			"templated-step": {
				Name:      "templated-step",
				OnSuccess: "done",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: templateName,
					Parameters:   params,
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
}

func TestNewTemplateService(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)
	require.NotNil(t, svc)
}

func TestTemplateService_ExpandWorkflow_Simple(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := newWorkflowWithTemplateRef("simple-echo", map[string]any{
		"message": "Hello World",
	})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// After expansion, the step should have template content merged in
	step := wf.Steps["templated-step"]
	require.NotNil(t, step)

	// Type should be set from template
	assert.Equal(t, workflow.StepTypeCommand, step.Type)

	// Command should have parameters substituted
	assert.Contains(t, step.Command, "Hello World")
	assert.Contains(t, step.Command, "[INFO]") // default value

	// TemplateRef should be cleared after expansion
	assert.Nil(t, step.TemplateRef)

	// Transitions should be preserved from workflow
	assert.Equal(t, "done", step.OnSuccess)
	assert.Equal(t, "error", step.OnFailure)
}

func TestTemplateService_ExpandWorkflow_OverrideDefaults(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := newWorkflowWithTemplateRef("simple-echo", map[string]any{
		"message": "Custom Message",
		"prefix":  "[CUSTOM]", // override default
	})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	step := wf.Steps["templated-step"]
	assert.Contains(t, step.Command, "Custom Message")
	assert.Contains(t, step.Command, "[CUSTOM]")
	assert.NotContains(t, step.Command, "[INFO]")
}

func TestTemplateService_ExpandWorkflow_AIAnalyzeTemplate(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newAIAnalyzeTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := newWorkflowWithTemplateRef("ai-analyze", map[string]any{
		"prompt": "Analyze this code",
		"model":  "gemini",
	})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	step := wf.Steps["templated-step"]

	// Command should use provided model (not default)
	assert.Contains(t, step.Command, "gemini")
	assert.Contains(t, step.Command, "Analyze this code")

	// Timeout should be copied from template
	assert.Equal(t, 120, step.Timeout)

	// Capture should be copied from template
	require.NotNil(t, step.Capture)
	assert.Equal(t, "analysis", step.Capture.Stdout)
}

func TestTemplateService_ExpandWorkflow_NoTemplateRefs(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Workflow with no template references
	wf := &workflow.Workflow{
		Name:    "no-templates",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// Workflow should remain unchanged
	assert.Equal(t, "echo hello", wf.Steps["start"].Command)
}

func TestTemplateService_ExpandWorkflow_MultipleTemplateRefs(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	tmpl = newAIAnalyzeTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := &workflow.Workflow{
		Name:    "multi-template",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				OnSuccess: "step2",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "simple-echo",
					Parameters:   map[string]any{"message": "First"},
				},
			},
			"step2": {
				Name:      "step2",
				OnSuccess: "done",
				OnFailure: "error",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "ai-analyze",
					Parameters:   map[string]any{"prompt": "Analyze"},
				},
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// Both steps should be expanded
	assert.Contains(t, wf.Steps["step1"].Command, "First")
	assert.Nil(t, wf.Steps["step1"].TemplateRef)

	assert.Contains(t, wf.Steps["step2"].Command, "Analyze")
	assert.Nil(t, wf.Steps["step2"].TemplateRef)
}

func TestTemplateService_ExpandWorkflow_PreservesStepDir(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name:       "with-dir",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "make build",
				Dir:     "/template/dir",
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Step with its own dir should keep it
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Dir:       "/workflow/dir", // Step's dir should take precedence
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "with-dir",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// Workflow step dir should be preserved
	assert.Equal(t, "/workflow/dir", wf.Steps["step"].Dir)
}

func TestTemplateService_ExpandWorkflow_InheritsTemplateDir(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name:       "with-dir",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "make build",
				Dir:     "/template/dir",
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Step without dir should inherit from template
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Dir:       "", // No dir specified
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "with-dir",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// Should inherit template dir
	assert.Equal(t, "/template/dir", wf.Steps["step"].Dir)
}

func TestTemplateService_ExpandWorkflow_TemplateNotFound(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := newWorkflowWithTemplateRef("nonexistent", map[string]any{})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.Error(t, err)

	var notFound *workflow.TemplateNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, "nonexistent", notFound.TemplateName)
}

func TestTemplateService_ExpandWorkflow_MissingRequiredParam(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Missing required "message" parameter
	wf := newWorkflowWithTemplateRef("simple-echo", map[string]any{
		"prefix": "[CUSTOM]", // only provide optional param
	})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.Error(t, err)

	var missingParam *workflow.MissingParameterError
	require.ErrorAs(t, err, &missingParam)
	assert.Equal(t, "simple-echo", missingParam.TemplateName)
	assert.Equal(t, "message", missingParam.ParameterName)
}

func TestTemplateService_ExpandWorkflow_MissingAllRequiredParams(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name: "multi-required",
		Parameters: []workflow.TemplateParam{
			{Name: "param1", Required: true},
			{Name: "param2", Required: true},
			{Name: "param3", Required: true},
		},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{parameters.param1}} {{parameters.param2}} {{parameters.param3}}",
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// No parameters provided
	wf := newWorkflowWithTemplateRef("multi-required", map[string]any{})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.Error(t, err)

	var missingParam *workflow.MissingParameterError
	require.ErrorAs(t, err, &missingParam)
}

func TestTemplateService_ExpandWorkflow_CircularReference(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()

	// Create templates that reference each other
	tmplA := &workflow.Template{
		Name:       "circular-a",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name: "run",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "circular-b",
					Parameters:   map[string]any{},
				},
			},
		},
	}
	tmplB := &workflow.Template{
		Name:       "circular-b",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name: "run",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "circular-a",
					Parameters:   map[string]any{},
				},
			},
		},
	}
	repo.AddTemplate(tmplA.Name, tmplA)
	repo.AddTemplate(tmplB.Name, tmplB)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := newWorkflowWithTemplateRef("circular-a", map[string]any{})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.Error(t, err)

	var circularErr *workflow.CircularTemplateError
	require.ErrorAs(t, err, &circularErr)
	assert.NotEmpty(t, circularErr.Chain)
}

func TestTemplateService_ExpandWorkflow_SelfReference(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()

	// Template that references itself
	tmpl := &workflow.Template{
		Name:       "self-ref",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name: "run",
				Type: workflow.StepTypeCommand,
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "self-ref",
					Parameters:   map[string]any{},
				},
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := newWorkflowWithTemplateRef("self-ref", map[string]any{})

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.Error(t, err)

	var circularErr *workflow.CircularTemplateError
	require.ErrorAs(t, err, &circularErr)
}

func TestTemplateService_ValidateTemplateRef_Valid(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
		Parameters: map[string]any{
			"message": "Test message",
		},
	}

	err := svc.ValidateTemplateRef(context.Background(), ref)
	require.NoError(t, err)
}

func TestTemplateService_ValidateTemplateRef_WithDefaults(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newAIAnalyzeTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Only provide required param, defaults should be used
	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "ai-analyze",
		Parameters: map[string]any{
			"prompt": "Analyze code",
		},
	}

	err := svc.ValidateTemplateRef(context.Background(), ref)
	require.NoError(t, err)
}

func TestTemplateService_ValidateTemplateRef_TemplateNotFound(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "nonexistent",
		Parameters:   map[string]any{},
	}

	err := svc.ValidateTemplateRef(context.Background(), ref)
	require.Error(t, err)

	var notFound *workflow.TemplateNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestTemplateService_ValidateTemplateRef_MissingRequired(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
		Parameters:   map[string]any{}, // missing required "message"
	}

	err := svc.ValidateTemplateRef(context.Background(), ref)
	require.Error(t, err)

	var missingParam *workflow.MissingParameterError
	require.ErrorAs(t, err, &missingParam)
}

func TestTemplateService_ValidateTemplateRef_ExtraParams(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Provide extra parameters not defined in template
	ref := &workflow.WorkflowTemplateRef{
		TemplateName: "simple-echo",
		Parameters: map[string]any{
			"message": "Test",
			"extra":   "not defined",
			"another": 123,
		},
	}

	// Extra params should be ignored (no error)
	err := svc.ValidateTemplateRef(context.Background(), ref)
	require.NoError(t, err)
}

func TestTemplateService_ParameterSubstitution(t *testing.T) {
	tests := []struct {
		name     string
		template string
		params   map[string]any
		want     string
	}{
		{
			name:     "simple string substitution",
			template: "echo {{parameters.message}}",
			params:   map[string]any{"message": "hello"},
			want:     "echo hello",
		},
		{
			name:     "multiple substitutions",
			template: "{{parameters.cmd}} {{parameters.arg1}} {{parameters.arg2}}",
			params:   map[string]any{"cmd": "echo", "arg1": "foo", "arg2": "bar"},
			want:     "echo foo bar",
		},
		{
			name:     "numeric value",
			template: "sleep {{parameters.seconds}}",
			params:   map[string]any{"seconds": 30},
			want:     "sleep 30",
		},
		{
			name:     "boolean value",
			template: "run --verbose={{parameters.verbose}}",
			params:   map[string]any{"verbose": true},
			want:     "run --verbose=true",
		},
		{
			name:     "float value",
			template: "threshold {{parameters.ratio}}",
			params:   map[string]any{"ratio": 0.75},
			want:     "threshold 0.75",
		},
		{
			name:     "no substitutions needed",
			template: "echo static text",
			params:   map[string]any{},
			want:     "echo static text",
		},
		{
			name:     "parameter in middle of string",
			template: "prefix-{{parameters.middle}}-suffix",
			params:   map[string]any{"middle": "center"},
			want:     "prefix-center-suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMockTemplateRepository()

			// Build params list from map
			params := make([]workflow.TemplateParam, 0, len(tt.params))
			for name := range tt.params {
				params = append(params, workflow.TemplateParam{Name: name, Required: true})
			}

			tmpl := &workflow.Template{
				Name:       "test",
				Parameters: params,
				States: map[string]*workflow.Step{
					"run": {
						Name:    "run",
						Type:    workflow.StepTypeCommand,
						Command: tt.template,
					},
				},
			}
			repo.AddTemplate(tmpl.Name, tmpl)
			logger := mocks.NewMockLogger()

			svc := application.NewTemplateService(repo, logger)

			wf := newWorkflowWithTemplateRef("test", tt.params)

			err := svc.ExpandWorkflow(context.Background(), wf)
			require.NoError(t, err)

			assert.Equal(t, tt.want, wf.Steps["templated-step"].Command)
		})
	}
}

func TestTemplateService_ExpandWorkflow_InheritsRetry(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name:       "with-retry",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "flaky-command",
				Retry: &workflow.RetryConfig{
					MaxAttempts:    3,
					InitialDelayMs: 1000,
					Backoff:        "exponential",
				},
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "with-retry",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	require.NotNil(t, wf.Steps["step"].Retry)
	assert.Equal(t, 3, wf.Steps["step"].Retry.MaxAttempts)
	assert.Equal(t, "exponential", wf.Steps["step"].Retry.Backoff)
}

func TestTemplateService_ExpandWorkflow_InheritsCapture(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name:       "with-capture",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "get-data",
				Capture: &workflow.CaptureConfig{
					Stdout:  "output",
					Stderr:  "errors",
					MaxSize: "10MB",
				},
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "with-capture",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	require.NotNil(t, wf.Steps["step"].Capture)
	assert.Equal(t, "output", wf.Steps["step"].Capture.Stdout)
	assert.Equal(t, "errors", wf.Steps["step"].Capture.Stderr)
}

func TestTemplateService_ExpandWorkflow_InheritsTimeout(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name:       "with-timeout",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "long-running",
				Timeout: 300,
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Step without timeout
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Timeout:   0, // No timeout specified
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "with-timeout",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	assert.Equal(t, 300, wf.Steps["step"].Timeout)
}

func TestTemplateService_ExpandWorkflow_StepTimeoutOverridesTemplate(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := &workflow.Template{
		Name:       "with-timeout",
		Parameters: []workflow.TemplateParam{},
		States: map[string]*workflow.Step{
			"run": {
				Name:    "run",
				Type:    workflow.StepTypeCommand,
				Command: "long-running",
				Timeout: 300,
			},
		},
	}
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Step with its own timeout
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:      "step",
				Timeout:   60, // Step's timeout should take precedence
				OnSuccess: "done",
				TemplateRef: &workflow.WorkflowTemplateRef{
					TemplateName: "with-timeout",
					Parameters:   map[string]any{},
				},
			},
			"done": {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// Step timeout should be preserved
	assert.Equal(t, 60, wf.Steps["step"].Timeout)
}

func TestTemplateService_ExpandWorkflow_NilWorkflow(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Should handle nil gracefully
	err := svc.ExpandWorkflow(context.Background(), nil)

	// Either no error (does nothing) or a clear error
	// The implementation should decide - testing both valid behaviors
	_ = err
}

func TestTemplateService_ExpandWorkflow_EmptySteps(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := &workflow.Workflow{
		Name:    "empty",
		Initial: "start",
		Steps:   map[string]*workflow.Step{},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)
}

func TestTemplateService_ExpandWorkflow_NilTemplateRef(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:        "step",
				Type:        workflow.StepTypeCommand,
				Command:     "echo test",
				TemplateRef: nil, // Explicitly nil
			},
		},
	}

	err := svc.ExpandWorkflow(context.Background(), wf)
	require.NoError(t, err)

	// Step should remain unchanged
	assert.Equal(t, "echo test", wf.Steps["step"].Command)
}

func TestTemplateService_ExpandWorkflow_ContextCanceled(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	tmpl := newSimpleEchoTemplate()
	repo.AddTemplate(tmpl.Name, tmpl)
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	wf := newWorkflowWithTemplateRef("simple-echo", map[string]any{
		"message": "test",
	})

	// Context cancellation behavior depends on implementation
	// Just verify it doesn't panic
	_ = svc.ExpandWorkflow(ctx, wf)
}

func TestTemplateService_ImplementsExpectedBehavior(t *testing.T) {
	repo := mocks.NewMockTemplateRepository()
	logger := mocks.NewMockLogger()

	svc := application.NewTemplateService(repo, logger)

	// Verify service can be used with the expected interface
	type TemplateExpander interface {
		ExpandWorkflow(ctx context.Context, wf *workflow.Workflow) error
		ValidateTemplateRef(ctx context.Context, ref *workflow.WorkflowTemplateRef) error
	}

	var _ TemplateExpander = svc
}
