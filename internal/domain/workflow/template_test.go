package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Template Validation Tests
// =============================================================================

func TestTemplate_Validate(t *testing.T) {
	tests := []struct {
		name     string
		template *workflow.Template
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid template with required param",
			template: &workflow.Template{
				Name: "test-template",
				Parameters: []workflow.TemplateParam{
					{Name: "message", Required: true},
				},
				States: map[string]*workflow.Step{
					"echo": {
						Name:    "echo",
						Type:    workflow.StepTypeCommand,
						Command: "echo {{parameters.message}}",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid template with default param",
			template: &workflow.Template{
				Name: "test-template",
				Parameters: []workflow.TemplateParam{
					{Name: "message", Required: false, Default: "hello"},
				},
				States: map[string]*workflow.Step{
					"echo": {
						Name:    "echo",
						Type:    workflow.StepTypeCommand,
						Command: "echo {{parameters.message}}",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid template with multiple params",
			template: &workflow.Template{
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
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid template with no params",
			template: &workflow.Template{
				Name:       "no-params",
				Parameters: []workflow.TemplateParam{},
				States: map[string]*workflow.Step{
					"run": {
						Name:    "run",
						Type:    workflow.StepTypeCommand,
						Command: "echo static",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - empty name",
			template: &workflow.Template{
				Name:       "",
				Parameters: []workflow.TemplateParam{},
				States: map[string]*workflow.Step{
					"run": {
						Name:    "run",
						Type:    workflow.StepTypeCommand,
						Command: "echo test",
					},
				},
			},
			wantErr: true,
			errMsg:  "template name is required",
		},
		{
			name: "invalid - no states",
			template: &workflow.Template{
				Name:       "empty-states",
				Parameters: []workflow.TemplateParam{},
				States:     map[string]*workflow.Step{},
			},
			wantErr: true,
			errMsg:  "template must define at least one state",
		},
		{
			name: "invalid - nil states",
			template: &workflow.Template{
				Name:       "nil-states",
				Parameters: []workflow.TemplateParam{},
				States:     nil,
			},
			wantErr: true,
			errMsg:  "template must define at least one state",
		},
		{
			name: "invalid - empty parameter name",
			template: &workflow.Template{
				Name: "bad-param",
				Parameters: []workflow.TemplateParam{
					{Name: "", Required: true},
				},
				States: map[string]*workflow.Step{
					"run": {
						Name:    "run",
						Type:    workflow.StepTypeCommand,
						Command: "echo test",
					},
				},
			},
			wantErr: true,
			errMsg:  "parameter name is required",
		},
		{
			name: "invalid - duplicate parameter names",
			template: &workflow.Template{
				Name: "duplicate-params",
				Parameters: []workflow.TemplateParam{
					{Name: "message", Required: true},
					{Name: "message", Required: false, Default: "dup"},
				},
				States: map[string]*workflow.Step{
					"run": {
						Name:    "run",
						Type:    workflow.StepTypeCommand,
						Command: "echo {{parameters.message}}",
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate parameter name: message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.template.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// GetRequiredParams Tests
// =============================================================================

func TestTemplate_GetRequiredParams(t *testing.T) {
	tests := []struct {
		name     string
		params   []workflow.TemplateParam
		expected []string
	}{
		{
			name:     "no params",
			params:   []workflow.TemplateParam{},
			expected: nil,
		},
		{
			name: "all optional",
			params: []workflow.TemplateParam{
				{Name: "opt1", Required: false, Default: "a"},
				{Name: "opt2", Required: false, Default: "b"},
			},
			expected: nil,
		},
		{
			name: "all required",
			params: []workflow.TemplateParam{
				{Name: "req1", Required: true},
				{Name: "req2", Required: true},
			},
			expected: []string{"req1", "req2"},
		},
		{
			name: "mixed required and optional",
			params: []workflow.TemplateParam{
				{Name: "prompt", Required: true},
				{Name: "model", Required: false, Default: "claude"},
				{Name: "output", Required: true},
				{Name: "timeout", Required: false, Default: "60s"},
			},
			expected: []string{"prompt", "output"},
		},
		{
			name: "single required",
			params: []workflow.TemplateParam{
				{Name: "message", Required: true},
			},
			expected: []string{"message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &workflow.Template{
				Name:       "test",
				Parameters: tt.params,
				States: map[string]*workflow.Step{
					"dummy": {Name: "dummy", Type: workflow.StepTypeCommand, Command: "echo"},
				},
			}

			result := tmpl.GetRequiredParams()

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// =============================================================================
// GetDefaultValues Tests
// =============================================================================

func TestTemplate_GetDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		params   []workflow.TemplateParam
		expected map[string]any
	}{
		{
			name:     "no params",
			params:   []workflow.TemplateParam{},
			expected: map[string]any{},
		},
		{
			name: "all required (no defaults)",
			params: []workflow.TemplateParam{
				{Name: "req1", Required: true},
				{Name: "req2", Required: true},
			},
			expected: map[string]any{},
		},
		{
			name: "all with defaults",
			params: []workflow.TemplateParam{
				{Name: "model", Required: false, Default: "claude"},
				{Name: "timeout", Required: false, Default: 120},
			},
			expected: map[string]any{
				"model":   "claude",
				"timeout": 120,
			},
		},
		{
			name: "mixed defaults and required",
			params: []workflow.TemplateParam{
				{Name: "prompt", Required: true},
				{Name: "model", Required: false, Default: "claude"},
				{Name: "output", Required: true},
				{Name: "timeout", Required: false, Default: "60s"},
			},
			expected: map[string]any{
				"model":   "claude",
				"timeout": "60s",
			},
		},
		{
			name: "various default types",
			params: []workflow.TemplateParam{
				{Name: "str", Default: "string-value"},
				{Name: "num", Default: 42},
				{Name: "float", Default: 3.14},
				{Name: "bool", Default: true},
			},
			expected: map[string]any{
				"str":   "string-value",
				"num":   42,
				"float": 3.14,
				"bool":  true,
			},
		},
		{
			name: "nil default is not included",
			params: []workflow.TemplateParam{
				{Name: "with_default", Default: "value"},
				{Name: "nil_default", Default: nil},
			},
			expected: map[string]any{
				"with_default": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &workflow.Template{
				Name:       "test",
				Parameters: tt.params,
				States: map[string]*workflow.Step{
					"dummy": {Name: "dummy", Type: workflow.StepTypeCommand, Command: "echo"},
				},
			}

			result := tmpl.GetDefaultValues()

			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// WorkflowTemplateRef Tests
// =============================================================================

func TestWorkflowTemplateRef(t *testing.T) {
	tests := []struct {
		name string
		ref  *workflow.WorkflowTemplateRef
	}{
		{
			name: "simple ref with no params",
			ref: &workflow.WorkflowTemplateRef{
				TemplateName: "simple-echo",
				Parameters:   nil,
			},
		},
		{
			name: "ref with string params",
			ref: &workflow.WorkflowTemplateRef{
				TemplateName: "ai-analyze",
				Parameters: map[string]any{
					"prompt": "Analyze this code",
					"model":  "gemini",
				},
			},
		},
		{
			name: "ref with mixed param types",
			ref: &workflow.WorkflowTemplateRef{
				TemplateName: "complex-template",
				Parameters: map[string]any{
					"message": "hello",
					"count":   5,
					"enabled": true,
					"ratio":   0.75,
				},
			},
		},
		{
			name: "ref with template interpolation in params",
			ref: &workflow.WorkflowTemplateRef{
				TemplateName: "ai-analyze",
				Parameters: map[string]any{
					"prompt": "Analyze: {{states.extract.output}}",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.ref)
			assert.NotEmpty(t, tt.ref.TemplateName)
		})
	}
}

// =============================================================================
// Template Entity Construction Tests
// =============================================================================

func TestTemplate_Construction(t *testing.T) {
	// Verify complete template construction from F017 spec
	tmpl := &workflow.Template{
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

	err := tmpl.Validate()
	require.NoError(t, err)

	// Check required params
	required := tmpl.GetRequiredParams()
	assert.Equal(t, []string{"prompt"}, required)

	// Check defaults
	defaults := tmpl.GetDefaultValues()
	assert.Equal(t, "claude", defaults["model"])
	assert.Equal(t, "120s", defaults["timeout"])
	assert.NotContains(t, defaults, "prompt")
}

// =============================================================================
// Step with TemplateRef Tests
// =============================================================================

func TestStep_WithTemplateRef(t *testing.T) {
	step := &workflow.Step{
		Name:      "code_analysis",
		OnSuccess: "format",
		OnFailure: "error",
		TemplateRef: &workflow.WorkflowTemplateRef{
			TemplateName: "ai-analyze",
			Parameters: map[string]any{
				"prompt": "Analyze this code: {{states.extract.output}}",
				"model":  "gemini",
			},
		},
	}

	require.NotNil(t, step.TemplateRef)
	assert.Equal(t, "ai-analyze", step.TemplateRef.TemplateName)
	assert.Equal(t, "gemini", step.TemplateRef.Parameters["model"])
	assert.Contains(t, step.TemplateRef.Parameters["prompt"].(string), "{{states.extract.output}}")
}

func TestStep_WithoutTemplateRef(t *testing.T) {
	step := &workflow.Step{
		Name:      "normal-step",
		Type:      workflow.StepTypeCommand,
		Command:   "echo hello",
		OnSuccess: "next",
	}

	assert.Nil(t, step.TemplateRef)
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

func TestTemplate_EdgeCases(t *testing.T) {
	t.Run("template with multiple states", func(t *testing.T) {
		tmpl := &workflow.Template{
			Name:       "multi-state",
			Parameters: []workflow.TemplateParam{},
			States: map[string]*workflow.Step{
				"step1": {Name: "step1", Type: workflow.StepTypeCommand, Command: "echo 1"},
				"step2": {Name: "step2", Type: workflow.StepTypeCommand, Command: "echo 2"},
				"step3": {Name: "step3", Type: workflow.StepTypeCommand, Command: "echo 3"},
			},
		}

		err := tmpl.Validate()
		require.NoError(t, err)
		assert.Len(t, tmpl.States, 3)
	})

	t.Run("template parameter with empty string default", func(t *testing.T) {
		tmpl := &workflow.Template{
			Name: "empty-default",
			Parameters: []workflow.TemplateParam{
				{Name: "prefix", Required: false, Default: ""},
			},
			States: map[string]*workflow.Step{
				"run": {Name: "run", Type: workflow.StepTypeCommand, Command: "echo"},
			},
		}

		err := tmpl.Validate()
		require.NoError(t, err)

		// Empty string is a valid default, different from nil
		defaults := tmpl.GetDefaultValues()
		assert.Contains(t, defaults, "prefix")
		assert.Equal(t, "", defaults["prefix"])
	})

	t.Run("template parameter with zero numeric default", func(t *testing.T) {
		tmpl := &workflow.Template{
			Name: "zero-default",
			Parameters: []workflow.TemplateParam{
				{Name: "count", Required: false, Default: 0},
			},
			States: map[string]*workflow.Step{
				"run": {Name: "run", Type: workflow.StepTypeCommand, Command: "echo"},
			},
		}

		err := tmpl.Validate()
		require.NoError(t, err)

		defaults := tmpl.GetDefaultValues()
		assert.Contains(t, defaults, "count")
		assert.Equal(t, 0, defaults["count"])
	})

	t.Run("template parameter with false boolean default", func(t *testing.T) {
		tmpl := &workflow.Template{
			Name: "false-default",
			Parameters: []workflow.TemplateParam{
				{Name: "enabled", Required: false, Default: false},
			},
			States: map[string]*workflow.Step{
				"run": {Name: "run", Type: workflow.StepTypeCommand, Command: "echo"},
			},
		}

		err := tmpl.Validate()
		require.NoError(t, err)

		defaults := tmpl.GetDefaultValues()
		assert.Contains(t, defaults, "enabled")
		assert.Equal(t, false, defaults["enabled"])
	})
}

// =============================================================================
// Template Param Ordering Tests
// =============================================================================

func TestTemplate_ParamOrdering(t *testing.T) {
	// Verify that GetRequiredParams returns params in order of definition
	tmpl := &workflow.Template{
		Name: "ordered-params",
		Parameters: []workflow.TemplateParam{
			{Name: "alpha", Required: true},
			{Name: "beta", Required: false},
			{Name: "gamma", Required: true},
			{Name: "delta", Required: false},
			{Name: "epsilon", Required: true},
		},
		States: map[string]*workflow.Step{
			"run": {Name: "run", Type: workflow.StepTypeCommand, Command: "echo"},
		},
	}

	required := tmpl.GetRequiredParams()
	expected := []string{"alpha", "gamma", "epsilon"}
	assert.Equal(t, expected, required)
}
