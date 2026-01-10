package interpolation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// ExtractReferences Tests - Basic Cases
// =============================================================================

func TestExtractReferences_SingleInput(t *testing.T) {
	refs, err := interpolation.ExtractReferences("echo {{inputs.name}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeInputs, refs[0].Type)
	assert.Equal(t, "inputs", refs[0].Namespace)
	assert.Equal(t, "name", refs[0].Path)
	assert.Equal(t, "{{inputs.name}}", refs[0].Raw)
}

func TestExtractReferences_StateOutput(t *testing.T) {
	refs, err := interpolation.ExtractReferences("result: {{states.build.Output}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeStates, refs[0].Type)
	assert.Equal(t, "states", refs[0].Namespace)
	assert.Equal(t, "build", refs[0].Path)
	assert.Equal(t, "Output", refs[0].Property)
	assert.Equal(t, "{{states.build.Output}}", refs[0].Raw)
}

func TestExtractReferences_WorkflowProperty(t *testing.T) {
	refs, err := interpolation.ExtractReferences("Workflow: {{workflow.name}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeWorkflow, refs[0].Type)
	assert.Equal(t, "name", refs[0].Path)
}

func TestExtractReferences_EnvVariable(t *testing.T) {
	refs, err := interpolation.ExtractReferences("token: {{env.API_TOKEN}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeEnv, refs[0].Type)
	assert.Equal(t, "API_TOKEN", refs[0].Path)
}

func TestExtractReferences_ErrorInHook(t *testing.T) {
	refs, err := interpolation.ExtractReferences("Error: {{error.message}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeError, refs[0].Type)
	assert.Equal(t, "message", refs[0].Path)
}

func TestExtractReferences_ContextProperty(t *testing.T) {
	refs, err := interpolation.ExtractReferences("Dir: {{context.working_dir}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeContext, refs[0].Type)
	assert.Equal(t, "working_dir", refs[0].Path)
}

func TestExtractReferences_MultipleReferences(t *testing.T) {
	template := "{{inputs.name}} ran {{states.build.output}} in {{workflow.name}}"
	refs, err := interpolation.ExtractReferences(template)

	require.NoError(t, err)
	require.Len(t, refs, 3)

	types := make(map[interpolation.ReferenceType]bool)
	for _, ref := range refs {
		types[ref.Type] = true
	}
	assert.True(t, types[interpolation.TypeInputs])
	assert.True(t, types[interpolation.TypeStates])
	assert.True(t, types[interpolation.TypeWorkflow])
}

func TestExtractReferences_NoReferences(t *testing.T) {
	refs, err := interpolation.ExtractReferences("plain text without templates")

	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestExtractReferences_UnknownNamespace(t *testing.T) {
	refs, err := interpolation.ExtractReferences("{{unknown.field}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeUnknown, refs[0].Type)
}

func TestExtractReferences_EmptyTemplate(t *testing.T) {
	refs, err := interpolation.ExtractReferences("")

	require.NoError(t, err)
	assert.Empty(t, refs)
}

// =============================================================================
// ExtractReferences Tests - Edge Cases
// =============================================================================

func TestExtractReferences_WhitespaceInTemplate(t *testing.T) {
	// Templates should not have internal whitespace
	refs, err := interpolation.ExtractReferences("{{ inputs.name }}")

	require.NoError(t, err)
	// Should either parse trimmed or recognize as-is
	// Implementation decides behavior for whitespace
	require.Len(t, refs, 1)
}

func TestExtractReferences_NestedBraces(t *testing.T) {
	// Edge case: nested or escaped braces
	refs, err := interpolation.ExtractReferences("echo {{{inputs.name}}}")

	require.NoError(t, err)
	// Should extract the valid reference within
	require.GreaterOrEqual(t, len(refs), 1)
}

func TestExtractReferences_UnmatchedOpenBrace(t *testing.T) {
	// Unmatched braces should not cause errors, just no extraction
	refs, err := interpolation.ExtractReferences("echo {{inputs.name")

	require.NoError(t, err)
	// Could be 0 (incomplete pattern) or 1 (lenient parsing)
	// Implementation decides behavior
	assert.Empty(t, refs, "incomplete template pattern should not extract")
}

func TestExtractReferences_UnmatchedCloseBrace(t *testing.T) {
	refs, err := interpolation.ExtractReferences("echo inputs.name}}")

	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestExtractReferences_AdjacentReferences(t *testing.T) {
	refs, err := interpolation.ExtractReferences("{{inputs.a}}{{inputs.b}}")

	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, "a", refs[0].Path)
	assert.Equal(t, "b", refs[1].Path)
}

func TestExtractReferences_ReferencesOnMultipleLines(t *testing.T) {
	template := `line1: {{inputs.name}}
line2: {{states.step1.output}}
line3: {{workflow.id}}`

	refs, err := interpolation.ExtractReferences(template)

	require.NoError(t, err)
	require.Len(t, refs, 3)
}

func TestExtractReferences_DuplicateReferences(t *testing.T) {
	// Same reference used twice
	refs, err := interpolation.ExtractReferences("{{inputs.name}} and {{inputs.name}}")

	require.NoError(t, err)
	require.Len(t, refs, 2) // Should return both occurrences
}

func TestExtractReferences_EmptyBraces(t *testing.T) {
	refs, err := interpolation.ExtractReferences("echo {{}}")

	require.NoError(t, err)
	// Empty braces shouldn't be a valid reference
	// Could produce an error or empty result
	assert.Empty(t, refs)
}

func TestExtractReferences_SingleDotPath(t *testing.T) {
	// Single segment path (no namespace)
	refs, err := interpolation.ExtractReferences("{{invalid}}")

	require.NoError(t, err)
	// Should handle gracefully - either error or TypeUnknown
	if len(refs) > 0 {
		assert.Equal(t, interpolation.TypeUnknown, refs[0].Type)
	}
}

func TestExtractReferences_DeepNestedPath(t *testing.T) {
	// More than 3 segments
	refs, err := interpolation.ExtractReferences("{{states.step.output.nested.deep}}")

	require.NoError(t, err)
	require.Len(t, refs, 1)
	// Should still parse with TypeStates
	assert.Equal(t, interpolation.TypeStates, refs[0].Type)
}

func TestExtractReferences_SpecialCharactersInPath(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantLen  int
	}{
		{"underscores", "{{inputs.my_var_name}}", 1},
		{"numbers", "{{inputs.var123}}", 1},
		{"hyphen in step name", "{{states.my-step.output}}", 1},
		{"leading underscore", "{{inputs._private}}", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			assert.Len(t, refs, tt.wantLen)
		})
	}
}

// =============================================================================
// ExtractReferences Tests - All State Properties
// =============================================================================

func TestExtractReferences_AllStateProperties(t *testing.T) {
	tests := []struct {
		property string
	}{
		{"Output"},
		{"Stderr"},
		{"ExitCode"},
		{"Status"},
	}

	for _, tt := range tests {
		t.Run(tt.property, func(t *testing.T) {
			template := "{{states.step1." + tt.property + "}}"
			refs, err := interpolation.ExtractReferences(template)

			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, interpolation.TypeStates, refs[0].Type)
			assert.Equal(t, tt.property, refs[0].Property)
		})
	}
}

// =============================================================================
// ExtractReferences Tests - All Workflow Properties
// =============================================================================

func TestExtractReferences_AllWorkflowProperties(t *testing.T) {
	tests := []struct {
		property string
	}{
		{"id"},
		{"name"},
		{"current_state"},
		{"started_at"},
		{"duration"},
	}

	for _, tt := range tests {
		t.Run(tt.property, func(t *testing.T) {
			template := "{{workflow." + tt.property + "}}"
			refs, err := interpolation.ExtractReferences(template)

			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, interpolation.TypeWorkflow, refs[0].Type)
			assert.Equal(t, tt.property, refs[0].Path)
		})
	}
}

// =============================================================================
// ExtractReferences Tests - All Error Properties
// =============================================================================

func TestExtractReferences_AllErrorProperties(t *testing.T) {
	tests := []struct {
		property string
	}{
		{"message"},
		{"state"},
		{"exit_code"},
		{"type"},
	}

	for _, tt := range tests {
		t.Run(tt.property, func(t *testing.T) {
			template := "{{error." + tt.property + "}}"
			refs, err := interpolation.ExtractReferences(template)

			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, interpolation.TypeError, refs[0].Type)
			assert.Equal(t, tt.property, refs[0].Path)
		})
	}
}

// =============================================================================
// ExtractReferences Tests - All Context Properties
// =============================================================================

func TestExtractReferences_AllContextProperties(t *testing.T) {
	tests := []struct {
		property string
	}{
		{"working_dir"},
		{"user"},
		{"hostname"},
	}

	for _, tt := range tests {
		t.Run(tt.property, func(t *testing.T) {
			template := "{{context." + tt.property + "}}"
			refs, err := interpolation.ExtractReferences(template)

			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, interpolation.TypeContext, refs[0].Type)
			assert.Equal(t, tt.property, refs[0].Path)
		})
	}
}

// =============================================================================
// ParseReference Tests
// =============================================================================

func TestParseReference_InputsSimple(t *testing.T) {
	ref := interpolation.ParseReference("inputs.name")

	assert.Equal(t, interpolation.TypeInputs, ref.Type)
	assert.Equal(t, "inputs", ref.Namespace)
	assert.Equal(t, "name", ref.Path)
}

func TestParseReference_StatesWithProperty(t *testing.T) {
	ref := interpolation.ParseReference("states.build.Output")

	assert.Equal(t, interpolation.TypeStates, ref.Type)
	assert.Equal(t, "states", ref.Namespace)
	assert.Equal(t, "build", ref.Path)
	assert.Equal(t, "Output", ref.Property)
}

func TestParseReference_StatesWithStderr(t *testing.T) {
	ref := interpolation.ParseReference("states.compile.Stderr")

	assert.Equal(t, interpolation.TypeStates, ref.Type)
	assert.Equal(t, "compile", ref.Path)
	assert.Equal(t, "Stderr", ref.Property)
}

func TestParseReference_WorkflowDuration(t *testing.T) {
	ref := interpolation.ParseReference("workflow.duration")

	assert.Equal(t, interpolation.TypeWorkflow, ref.Type)
	assert.Equal(t, "duration", ref.Path)
}

func TestParseReference_EnvVariable(t *testing.T) {
	ref := interpolation.ParseReference("env.HOME")

	assert.Equal(t, interpolation.TypeEnv, ref.Type)
	assert.Equal(t, "HOME", ref.Path)
}

func TestParseReference_ErrorMessage(t *testing.T) {
	ref := interpolation.ParseReference("error.message")

	assert.Equal(t, interpolation.TypeError, ref.Type)
	assert.Equal(t, "message", ref.Path)
}

func TestParseReference_ContextWorkingDir(t *testing.T) {
	ref := interpolation.ParseReference("context.working_dir")

	assert.Equal(t, interpolation.TypeContext, ref.Type)
	assert.Equal(t, "working_dir", ref.Path)
}

func TestParseReference_EmptyPath(t *testing.T) {
	ref := interpolation.ParseReference("")

	assert.Equal(t, interpolation.TypeUnknown, ref.Type)
}

func TestParseReference_SingleSegment(t *testing.T) {
	ref := interpolation.ParseReference("invalid")

	assert.Equal(t, interpolation.TypeUnknown, ref.Type)
}

func TestParseReference_StatesWithExitCode(t *testing.T) {
	ref := interpolation.ParseReference("states.validate.ExitCode")

	assert.Equal(t, interpolation.TypeStates, ref.Type)
	assert.Equal(t, "validate", ref.Path)
	assert.Equal(t, "ExitCode", ref.Property)
}

func TestParseReference_StatesWithStatus(t *testing.T) {
	ref := interpolation.ParseReference("states.deploy.Status")

	assert.Equal(t, interpolation.TypeStates, ref.Type)
	assert.Equal(t, "deploy", ref.Path)
	assert.Equal(t, "Status", ref.Property)
}

// =============================================================================
// CategorizeNamespace Tests
// =============================================================================

func TestCategorizeNamespace(t *testing.T) {
	tests := []struct {
		namespace string
		want      interpolation.ReferenceType
	}{
		{"inputs", interpolation.TypeInputs},
		{"states", interpolation.TypeStates},
		{"workflow", interpolation.TypeWorkflow},
		{"env", interpolation.TypeEnv},
		{"error", interpolation.TypeError},
		{"context", interpolation.TypeContext},
		{"unknown", interpolation.TypeUnknown},
		{"", interpolation.TypeUnknown},
		{"INPUTS", interpolation.TypeUnknown}, // case-sensitive
		{"Inputs", interpolation.TypeUnknown}, // case-sensitive
		{"input", interpolation.TypeUnknown},  // typo
		{"state", interpolation.TypeUnknown},  // typo
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := interpolation.CategorizeNamespace(tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// ValidProperties Tests (ensure maps are populated correctly)
// =============================================================================

func TestValidWorkflowProperties(t *testing.T) {
	expected := []string{"id", "name", "current_state", "started_at", "duration"}
	for _, prop := range expected {
		assert.True(t, interpolation.ValidWorkflowProperties[prop],
			"expected %q to be a valid workflow property", prop)
	}
	// Also verify invalid properties return false
	assert.False(t, interpolation.ValidWorkflowProperties["invalid"])
	assert.False(t, interpolation.ValidWorkflowProperties[""])
}

func TestValidStateProperties(t *testing.T) {
	expected := []string{"Output", "Stderr", "ExitCode", "Status"}
	for _, prop := range expected {
		assert.True(t, interpolation.ValidStateProperties[prop],
			"expected %q to be a valid state property", prop)
	}
	// Also verify invalid properties return false
	assert.False(t, interpolation.ValidStateProperties["stdout"]) // common mistake
	assert.False(t, interpolation.ValidStateProperties["result"])
	assert.False(t, interpolation.ValidStateProperties[""])
	// F050: Verify lowercase keys are now invalid (breaking change)
	assert.False(t, interpolation.ValidStateProperties["output"])
	assert.False(t, interpolation.ValidStateProperties["stderr"])
	assert.False(t, interpolation.ValidStateProperties["exit_code"])
	assert.False(t, interpolation.ValidStateProperties["status"])
}

func TestValidErrorProperties(t *testing.T) {
	expected := []string{"message", "state", "exit_code", "type"}
	for _, prop := range expected {
		assert.True(t, interpolation.ValidErrorProperties[prop],
			"expected %q to be a valid error property", prop)
	}
	assert.False(t, interpolation.ValidErrorProperties["code"])
	assert.False(t, interpolation.ValidErrorProperties[""])
}

func TestValidContextProperties(t *testing.T) {
	expected := []string{"working_dir", "user", "hostname"}
	for _, prop := range expected {
		assert.True(t, interpolation.ValidContextProperties[prop],
			"expected %q to be a valid context property", prop)
	}
	assert.False(t, interpolation.ValidContextProperties["cwd"])
	assert.False(t, interpolation.ValidContextProperties[""])
}

// =============================================================================
// Reference Struct Tests
// =============================================================================

func TestReference_RawFieldPreservesOriginal(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantRaw  string
	}{
		{"simple input", "{{inputs.name}}", "{{inputs.name}}"},
		{"state with property", "{{states.build.output}}", "{{states.build.output}}"},
		{"env var", "{{env.HOME}}", "{{env.HOME}}"},
		{"with surrounding text", "echo {{inputs.name}} done", "{{inputs.name}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, tt.wantRaw, refs[0].Raw)
		})
	}
}

// =============================================================================
// Complex Template Tests
// =============================================================================

func TestExtractReferences_RealWorldTemplates(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantLen  int
	}{
		{
			name:     "shell command with input",
			template: "curl -X POST -d '{\"name\": \"{{inputs.name}}\"}' https://api.example.com",
			wantLen:  1,
		},
		{
			name:     "multiline script",
			template: "#!/bin/bash\nNAME={{inputs.name}}\necho \"Processing $NAME\"\necho \"Result: {{states.process.output}}\"",
			wantLen:  2,
		},
		{
			name:     "json output",
			template: `{"workflow": "{{workflow.name}}", "result": "{{states.final.output}}", "env": "{{env.ENVIRONMENT}}"}`,
			wantLen:  3,
		},
		{
			name:     "error handler",
			template: "echo 'Error in {{error.state}}: {{error.message}}' >> {{context.working_dir}}/error.log",
			wantLen:  3,
		},
		{
			name:     "all namespace types",
			template: "{{inputs.a}} {{states.b.output}} {{workflow.id}} {{env.HOME}} {{error.message}} {{context.user}}",
			wantLen:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			assert.Len(t, refs, tt.wantLen)
		})
	}
}

// =============================================================================
// Table-Driven Comprehensive Test
// =============================================================================

func TestExtractReferences_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		wantCount int
		wantTypes []interpolation.ReferenceType
	}{
		{
			name:      "input only",
			template:  "{{inputs.foo}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeInputs},
		},
		{
			name:      "states only",
			template:  "{{states.step.output}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeStates},
		},
		{
			name:      "workflow only",
			template:  "{{workflow.name}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeWorkflow},
		},
		{
			name:      "env only",
			template:  "{{env.PATH}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeEnv},
		},
		{
			name:      "error only",
			template:  "{{error.message}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeError},
		},
		{
			name:      "context only",
			template:  "{{context.working_dir}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeContext},
		},
		{
			name:      "unknown namespace",
			template:  "{{custom.field}}",
			wantCount: 1,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeUnknown},
		},
		{
			name:      "mixed types",
			template:  "{{inputs.name}} {{states.step.output}}",
			wantCount: 2,
			wantTypes: []interpolation.ReferenceType{interpolation.TypeInputs, interpolation.TypeStates},
		},
		{
			name:      "no templates",
			template:  "plain text",
			wantCount: 0,
			wantTypes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)

			require.NoError(t, err)
			assert.Len(t, refs, tt.wantCount)

			if tt.wantTypes != nil {
				extractedTypes := make([]interpolation.ReferenceType, len(refs))
				for i, ref := range refs {
					extractedTypes[i] = ref.Type
				}
				assert.ElementsMatch(t, tt.wantTypes, extractedTypes)
			}
		})
	}
}

// =============================================================================
// Leading Dot Syntax Tests (Go template compatibility)
// =============================================================================

func TestParseReference_LeadingDotStates(t *testing.T) {
	// Go template syntax: {{.states.step.Output}} should work like {{states.step.Output}}
	ref := interpolation.ParseReference(".states.build.Output")

	assert.Equal(t, interpolation.TypeStates, ref.Type)
	assert.Equal(t, "states", ref.Namespace)
	assert.Equal(t, "build", ref.Path)
	assert.Equal(t, "Output", ref.Property)
}

func TestParseReference_LeadingDotInputs(t *testing.T) {
	ref := interpolation.ParseReference(".inputs.pr_base")

	assert.Equal(t, interpolation.TypeInputs, ref.Type)
	assert.Equal(t, "inputs", ref.Namespace)
	assert.Equal(t, "pr_base", ref.Path)
}

func TestParseReference_LeadingDotWorkflow(t *testing.T) {
	ref := interpolation.ParseReference(".workflow.Duration")

	assert.Equal(t, interpolation.TypeWorkflow, ref.Type)
	assert.Equal(t, "workflow", ref.Namespace)
	assert.Equal(t, "Duration", ref.Path)
}

func TestParseReference_LeadingDotError(t *testing.T) {
	ref := interpolation.ParseReference(".error.Message")

	assert.Equal(t, interpolation.TypeError, ref.Type)
	assert.Equal(t, "error", ref.Namespace)
	assert.Equal(t, "Message", ref.Path)
}

func TestParseReference_LeadingDotLoop(t *testing.T) {
	ref := interpolation.ParseReference(".loop.Index")

	assert.Equal(t, interpolation.TypeLoop, ref.Type)
	assert.Equal(t, "loop", ref.Namespace)
	assert.Equal(t, "Index", ref.Path)
}

func TestParseReference_LeadingDotEnv(t *testing.T) {
	ref := interpolation.ParseReference(".env.HOME")

	assert.Equal(t, interpolation.TypeEnv, ref.Type)
	assert.Equal(t, "env", ref.Namespace)
	assert.Equal(t, "HOME", ref.Path)
}

func TestParseReference_LeadingDotContext(t *testing.T) {
	ref := interpolation.ParseReference(".context.working_dir")

	assert.Equal(t, interpolation.TypeContext, ref.Type)
	assert.Equal(t, "context", ref.Namespace)
	assert.Equal(t, "working_dir", ref.Path)
}

func TestExtractReferences_LeadingDotSyntax(t *testing.T) {
	// Full extraction test with leading dot in braces
	tests := []struct {
		name     string
		template string
		wantType interpolation.ReferenceType
		wantPath string
	}{
		{
			name:     "states with leading dot",
			template: "{{.states.step.Output}}",
			wantType: interpolation.TypeStates,
			wantPath: "step",
		},
		{
			name:     "inputs with leading dot",
			template: "{{.inputs.name}}",
			wantType: interpolation.TypeInputs,
			wantPath: "name",
		},
		{
			name:     "workflow with leading dot",
			template: "{{.workflow.Duration}}",
			wantType: interpolation.TypeWorkflow,
			wantPath: "Duration",
		},
		{
			name:     "error with leading dot",
			template: "{{.error.Message}}",
			wantType: interpolation.TypeError,
			wantPath: "Message",
		},
		{
			name:     "env with leading dot",
			template: "{{.env.API_KEY}}",
			wantType: interpolation.TypeEnv,
			wantPath: "API_KEY",
		},
		{
			name:     "loop with leading dot",
			template: "{{.loop.Index}}",
			wantType: interpolation.TypeLoop,
			wantPath: "Index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, tt.wantType, refs[0].Type)
			assert.Equal(t, tt.wantPath, refs[0].Path)
		})
	}
}

func TestExtractReferences_MixedLeadingDotSyntax(t *testing.T) {
	// Test mixing both syntaxes in the same template
	template := "{{.states.step1.Output}} and {{states.step2.Output}}"
	refs, err := interpolation.ExtractReferences(template)

	require.NoError(t, err)
	require.Len(t, refs, 2)
	// Both should be TypeStates
	assert.Equal(t, interpolation.TypeStates, refs[0].Type)
	assert.Equal(t, interpolation.TypeStates, refs[1].Type)
	// Paths should be correctly extracted
	assert.Equal(t, "step1", refs[0].Path)
	assert.Equal(t, "step2", refs[1].Path)
}

func TestExtractReferences_RealWorldLeadingDot(t *testing.T) {
	// Test based on actual user workflow that was failing
	template := `git commit -m "$(cat << 'COMMITEOF'
      {{.states.generate_commit.Output}}
      COMMITEOF
      )"`

	refs, err := interpolation.ExtractReferences(template)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, interpolation.TypeStates, refs[0].Type)
	assert.Equal(t, "generate_commit", refs[0].Path)
	assert.Equal(t, "Output", refs[0].Property)
}
