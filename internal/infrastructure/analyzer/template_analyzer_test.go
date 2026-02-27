package analyzer_test

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/analyzer"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInterpolationAnalyzer(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()
	assert.NotNil(t, a)
}

func TestInterpolationAnalyzer_ImplementsInterface(t *testing.T) {
	var _ workflow.TemplateAnalyzer = analyzer.NewInterpolationAnalyzer()
}

func TestExtractReferences(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     []workflow.TemplateReference
		wantErr  bool
	}{
		{
			name:     "empty template",
			template: "",
			want:     nil,
		},
		{
			name:     "no references",
			template: "plain text without templates",
			want:     nil,
		},
		{
			name:     "single input reference",
			template: "Hello {{inputs.name}}!",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeInputs, Namespace: "inputs", Path: "name", Raw: "{{inputs.name}}"},
			},
		},
		{
			name:     "state with property",
			template: "Result: {{states.build.output}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeStates, Namespace: "states", Path: "build", Property: "output", Raw: "{{states.build.output}}"},
			},
		},
		{
			name:     "workflow metadata",
			template: "Workflow {{workflow.id}} - {{workflow.name}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeWorkflow, Namespace: "workflow", Path: "id", Raw: "{{workflow.id}}"},
				{Type: workflow.TypeWorkflow, Namespace: "workflow", Path: "name", Raw: "{{workflow.name}}"},
			},
		},
		{
			name:     "env variable",
			template: "Path: {{env.PATH}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeEnv, Namespace: "env", Path: "PATH", Raw: "{{env.PATH}}"},
			},
		},
		{
			name:     "error in hook",
			template: "Error: {{error.message}} ({{error.type}})",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeError, Namespace: "error", Path: "message", Raw: "{{error.message}}"},
				{Type: workflow.TypeError, Namespace: "error", Path: "type", Raw: "{{error.type}}"},
			},
		},
		{
			name:     "context info",
			template: "Running in {{context.working_dir}} as {{context.user}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeContext, Namespace: "context", Path: "working_dir", Raw: "{{context.working_dir}}"},
				{Type: workflow.TypeContext, Namespace: "context", Path: "user", Raw: "{{context.user}}"},
			},
		},
		{
			name:     "unknown namespace",
			template: "Unknown: {{custom.value}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeUnknown, Namespace: "custom", Path: "value", Raw: "{{custom.value}}"},
			},
		},
		{
			name:     "multiple reference types",
			template: "{{inputs.a}} and {{states.b.output}} with {{env.C}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeInputs, Namespace: "inputs", Path: "a", Raw: "{{inputs.a}}"},
				{Type: workflow.TypeStates, Namespace: "states", Path: "b", Property: "output", Raw: "{{states.b.output}}"},
				{Type: workflow.TypeEnv, Namespace: "env", Path: "C", Raw: "{{env.C}}"},
			},
		},
		{
			name:     "state exit_code property",
			template: "Exit: {{states.test.exit_code}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeStates, Namespace: "states", Path: "test", Property: "exit_code", Raw: "{{states.test.exit_code}}"},
			},
		},
		{
			name:     "state stderr property",
			template: "Stderr: {{states.build.stderr}}",
			want: []workflow.TemplateReference{
				{Type: workflow.TypeStates, Namespace: "states", Path: "build", Property: "stderr", Raw: "{{states.build.stderr}}"},
			},
		},
	}

	a := analyzer.NewInterpolationAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.ExtractReferences(tt.template)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, len(tt.want), len(got), "reference count mismatch")

			for i, want := range tt.want {
				assert.Equal(t, want.Type, got[i].Type, "Type mismatch at index %d", i)
				assert.Equal(t, want.Namespace, got[i].Namespace, "Namespace mismatch at index %d", i)
				assert.Equal(t, want.Path, got[i].Path, "Path mismatch at index %d", i)
				assert.Equal(t, want.Property, got[i].Property, "Property mismatch at index %d", i)
				assert.Equal(t, want.Raw, got[i].Raw, "Raw mismatch at index %d", i)
			}
		})
	}
}

func TestConvertReferenceType(t *testing.T) {
	tests := []struct {
		name  string
		input interpolation.ReferenceType
		want  workflow.ReferenceType
	}{
		{"inputs", interpolation.TypeInputs, workflow.TypeInputs},
		{"states", interpolation.TypeStates, workflow.TypeStates},
		{"workflow", interpolation.TypeWorkflow, workflow.TypeWorkflow},
		{"env", interpolation.TypeEnv, workflow.TypeEnv},
		{"error", interpolation.TypeError, workflow.TypeError},
		{"context", interpolation.TypeContext, workflow.TypeContext},
		{"unknown", interpolation.TypeUnknown, workflow.TypeUnknown},
	}

	a := analyzer.NewInterpolationAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use ExtractReferences with a template that produces this type
			template := "{{" + string(tt.input) + ".test}}"
			refs, err := a.ExtractReferences(template)
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, tt.want, refs[0].Type)
		})
	}
}

func TestConvertReferenceType_DefaultCase(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()

	// Test with a completely unknown namespace
	refs, err := a.ExtractReferences("{{foobar.value}}")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, workflow.TypeUnknown, refs[0].Type)
}

func TestExtractReferences_EdgeCases(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()

	t.Run("incomplete template", func(t *testing.T) {
		refs, err := a.ExtractReferences("Start {{inputs.test")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("empty braces", func(t *testing.T) {
		refs, err := a.ExtractReferences("Empty {{}} here")
		require.NoError(t, err)
		assert.Empty(t, refs)
	})

	t.Run("whitespace in braces", func(t *testing.T) {
		refs, err := a.ExtractReferences("Value {{ inputs.name }}")
		require.NoError(t, err)
		require.Len(t, refs, 1)
		assert.Equal(t, workflow.TypeInputs, refs[0].Type)
	})

	t.Run("single segment namespace", func(t *testing.T) {
		refs, err := a.ExtractReferences("{{single}}")
		require.NoError(t, err)
		require.Len(t, refs, 1)
		assert.Equal(t, workflow.TypeUnknown, refs[0].Type)
	})
}

func TestExtractReferences_AllWorkflowProperties(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()

	for prop := range workflow.ValidWorkflowProperties {
		t.Run("workflow_"+prop, func(t *testing.T) {
			refs, err := a.ExtractReferences("{{workflow." + prop + "}}")
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, workflow.TypeWorkflow, refs[0].Type)
			assert.Equal(t, prop, refs[0].Path)
		})
	}
}

func TestExtractReferences_AllStateProperties(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()

	for prop := range workflow.ValidStateProperties {
		t.Run("state_"+prop, func(t *testing.T) {
			refs, err := a.ExtractReferences("{{states.step." + prop + "}}")
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, workflow.TypeStates, refs[0].Type)
			assert.Equal(t, "step", refs[0].Path)
			assert.Equal(t, prop, refs[0].Property)
		})
	}
}

func TestExtractReferences_AllErrorProperties(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()

	for prop := range workflow.ValidErrorProperties {
		t.Run("error_"+prop, func(t *testing.T) {
			refs, err := a.ExtractReferences("{{error." + prop + "}}")
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, workflow.TypeError, refs[0].Type)
			assert.Equal(t, prop, refs[0].Path)
		})
	}
}

func TestExtractReferences_AllContextProperties(t *testing.T) {
	a := analyzer.NewInterpolationAnalyzer()

	for prop := range workflow.ValidContextProperties {
		t.Run("context_"+prop, func(t *testing.T) {
			refs, err := a.ExtractReferences("{{context." + prop + "}}")
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, workflow.TypeContext, refs[0].Type)
			assert.Equal(t, prop, refs[0].Path)
		})
	}
}
