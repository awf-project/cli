package interpolation_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractReferences_NamespaceTypes(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantType interpolation.ReferenceType
		wantNS   string
		wantPath string
		wantProp string
		wantRaw  string
	}{
		{
			name:     "inputs",
			template: "echo {{inputs.name}}",
			wantType: interpolation.TypeInputs,
			wantNS:   "inputs",
			wantPath: "name",
			wantRaw:  "{{inputs.name}}",
		},
		{
			name:     "states with property",
			template: "result: {{states.build.Output}}",
			wantType: interpolation.TypeStates,
			wantNS:   "states",
			wantPath: "build",
			wantProp: "Output",
			wantRaw:  "{{states.build.Output}}",
		},
		{
			name:     "workflow",
			template: "Workflow: {{workflow.name}}",
			wantType: interpolation.TypeWorkflow,
			wantPath: "name",
		},
		{
			name:     "env",
			template: "token: {{env.API_TOKEN}}",
			wantType: interpolation.TypeEnv,
			wantPath: "API_TOKEN",
		},
		{
			name:     "error",
			template: "Error: {{error.message}}",
			wantType: interpolation.TypeError,
			wantPath: "message",
		},
		{
			name:     "context",
			template: "Dir: {{context.working_dir}}",
			wantType: interpolation.TypeContext,
			wantPath: "working_dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, tt.wantType, refs[0].Type)
			if tt.wantNS != "" {
				assert.Equal(t, tt.wantNS, refs[0].Namespace)
			}
			if tt.wantPath != "" {
				assert.Equal(t, tt.wantPath, refs[0].Path)
			}
			if tt.wantProp != "" {
				assert.Equal(t, tt.wantProp, refs[0].Property)
			}
			if tt.wantRaw != "" {
				assert.Equal(t, tt.wantRaw, refs[0].Raw)
			}
		})
	}
}

func TestExtractReferences_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		wantLen   int
		wantEmpty bool
		checkType bool
		wantType  interpolation.ReferenceType
		checkPath bool
		path0     string
		path1     string
	}{
		{
			name:      "multiple references",
			template:  "{{inputs.name}} ran {{states.build.output}} in {{workflow.name}}",
			wantLen:   3,
			wantEmpty: false,
		},
		{
			name:      "no references",
			template:  "plain text without templates",
			wantEmpty: true,
		},
		{
			name:      "unknown namespace",
			template:  "{{unknown.field}}",
			wantLen:   1,
			checkType: true,
			wantType:  interpolation.TypeUnknown,
		},
		{
			name:      "empty template",
			template:  "",
			wantEmpty: true,
		},
		{
			name:     "whitespace in template",
			template: "{{ inputs.name }}",
			wantLen:  1,
		},
		{
			name:     "nested braces",
			template: "echo {{{inputs.name}}}",
			wantLen:  1,
		},
		{
			name:      "unmatched open brace",
			template:  "echo {{inputs.name",
			wantEmpty: true,
		},
		{
			name:      "unmatched close brace",
			template:  "echo inputs.name}}",
			wantEmpty: true,
		},
		{
			name:      "adjacent references",
			template:  "{{inputs.a}}{{inputs.b}}",
			wantLen:   2,
			checkPath: true,
			path0:     "a",
			path1:     "b",
		},
		{
			name:     "multiline",
			template: "line1: {{inputs.name}}\nline2: {{states.step1.output}}\nline3: {{workflow.id}}",
			wantLen:  3,
		},
		{
			name:     "duplicates",
			template: "{{inputs.name}} and {{inputs.name}}",
			wantLen:  2,
		},
		{
			name:      "empty braces",
			template:  "echo {{}}",
			wantEmpty: true,
		},
		{
			name:      "single segment",
			template:  "{{invalid}}",
			checkType: true,
			wantType:  interpolation.TypeUnknown,
		},
		{
			name:      "deep nested path",
			template:  "{{states.step.output.nested.deep}}",
			wantLen:   1,
			checkType: true,
			wantType:  interpolation.TypeStates,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)

			if tt.wantEmpty {
				assert.Empty(t, refs)
				return
			}

			if tt.wantLen > 0 {
				require.Len(t, refs, tt.wantLen)
			}

			if tt.checkType && len(refs) > 0 {
				assert.Equal(t, tt.wantType, refs[0].Type)
			}

			if tt.checkPath && len(refs) >= 2 {
				assert.Equal(t, tt.path0, refs[0].Path)
				assert.Equal(t, tt.path1, refs[1].Path)
			}
		})
	}
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

func TestExtractReferences_AllProperties(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		property  string
		wantType  interpolation.ReferenceType
	}{
		{"state Output", "states.step1", "Output", interpolation.TypeStates},
		{"state Stderr", "states.step1", "Stderr", interpolation.TypeStates},
		{"state ExitCode", "states.step1", "ExitCode", interpolation.TypeStates},
		{"state Status", "states.step1", "Status", interpolation.TypeStates},
		{"workflow id", "workflow", "id", interpolation.TypeWorkflow},
		{"workflow name", "workflow", "name", interpolation.TypeWorkflow},
		{"workflow current_state", "workflow", "current_state", interpolation.TypeWorkflow},
		{"workflow started_at", "workflow", "started_at", interpolation.TypeWorkflow},
		{"workflow duration", "workflow", "duration", interpolation.TypeWorkflow},
		{"error message", "error", "message", interpolation.TypeError},
		{"error state", "error", "state", interpolation.TypeError},
		{"error exit_code", "error", "exit_code", interpolation.TypeError},
		{"error type", "error", "type", interpolation.TypeError},
		{"context working_dir", "context", "working_dir", interpolation.TypeContext},
		{"context user", "context", "user", interpolation.TypeContext},
		{"context hostname", "context", "hostname", interpolation.TypeContext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := "{{" + tt.namespace + "." + tt.property + "}}"
			refs, err := interpolation.ExtractReferences(template)

			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, tt.wantType, refs[0].Type)
		})
	}
}

func TestParseReference(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType interpolation.ReferenceType
		wantNS   string
		wantPath string
		wantProp string
	}{
		{
			name:     "inputs",
			path:     "inputs.name",
			wantType: interpolation.TypeInputs,
			wantNS:   "inputs",
			wantPath: "name",
		},
		{
			name:     "states with Output",
			path:     "states.build.Output",
			wantType: interpolation.TypeStates,
			wantNS:   "states",
			wantPath: "build",
			wantProp: "Output",
		},
		{
			name:     "states with Stderr",
			path:     "states.compile.Stderr",
			wantType: interpolation.TypeStates,
			wantPath: "compile",
			wantProp: "Stderr",
		},
		{
			name:     "states with ExitCode",
			path:     "states.validate.ExitCode",
			wantType: interpolation.TypeStates,
			wantPath: "validate",
			wantProp: "ExitCode",
		},
		{
			name:     "states with Status",
			path:     "states.deploy.Status",
			wantType: interpolation.TypeStates,
			wantPath: "deploy",
			wantProp: "Status",
		},
		{
			name:     "workflow",
			path:     "workflow.duration",
			wantType: interpolation.TypeWorkflow,
			wantPath: "duration",
		},
		{
			name:     "env",
			path:     "env.HOME",
			wantType: interpolation.TypeEnv,
			wantPath: "HOME",
		},
		{
			name:     "error",
			path:     "error.message",
			wantType: interpolation.TypeError,
			wantPath: "message",
		},
		{
			name:     "context",
			path:     "context.working_dir",
			wantType: interpolation.TypeContext,
			wantPath: "working_dir",
		},
		{
			name:     "empty path",
			path:     "",
			wantType: interpolation.TypeUnknown,
		},
		{
			name:     "single segment",
			path:     "invalid",
			wantType: interpolation.TypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := interpolation.ParseReference(tt.path)
			assert.Equal(t, tt.wantType, ref.Type)
			if tt.wantNS != "" {
				assert.Equal(t, tt.wantNS, ref.Namespace)
			}
			if tt.wantPath != "" {
				assert.Equal(t, tt.wantPath, ref.Path)
			}
			if tt.wantProp != "" {
				assert.Equal(t, tt.wantProp, ref.Property)
			}
		})
	}
}

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

func TestValidationMaps(t *testing.T) {
	tests := []struct {
		name           string
		validMap       map[string]bool
		expectedKeys   []string
		invalidKeys    []string
		forbiddenLower []string
	}{
		{
			name:           "ValidWorkflowProperties",
			validMap:       interpolation.ValidWorkflowProperties,
			expectedKeys:   []string{"ID", "Name", "CurrentState", "StartedAt", "Duration"},
			invalidKeys:    []string{"invalid", ""},
			forbiddenLower: []string{"id", "name", "current_state", "started_at", "duration"},
		},
		{
			name:           "ValidStateProperties",
			validMap:       interpolation.ValidStateProperties,
			expectedKeys:   []string{"Output", "Stderr", "ExitCode", "Status", "Response", "TokensUsed", "JSON"},
			invalidKeys:    []string{"stdout", "result", ""},
			forbiddenLower: []string{"output", "stderr", "exit_code", "status", "response", "tokens", "json"},
		},
		{
			name:           "ValidErrorProperties",
			validMap:       interpolation.ValidErrorProperties,
			expectedKeys:   []string{"Message", "State", "ExitCode", "Type"},
			invalidKeys:    []string{"code", ""},
			forbiddenLower: []string{"message", "state", "exit_code", "type"},
		},
		{
			name:           "ValidContextProperties",
			validMap:       interpolation.ValidContextProperties,
			expectedKeys:   []string{"WorkingDir", "User", "Hostname"},
			invalidKeys:    []string{"cwd", ""},
			forbiddenLower: []string{"working_dir", "user", "hostname"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range tt.expectedKeys {
				assert.True(t, tt.validMap[key])
			}
			for _, key := range tt.invalidKeys {
				assert.False(t, tt.validMap[key])
			}
			for _, key := range tt.forbiddenLower {
				assert.False(t, tt.validMap[key])
			}
		})
	}
}

func TestExtractReferences_RealWorld(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantLen  int
		wantRaw  string
	}{
		{
			name:     "preserves raw simple",
			template: "{{inputs.name}}",
			wantLen:  1,
			wantRaw:  "{{inputs.name}}",
		},
		{
			name:     "preserves raw with surrounding",
			template: "echo {{inputs.name}} done",
			wantLen:  1,
			wantRaw:  "{{inputs.name}}",
		},
		{
			name:     "shell command with input",
			template: "curl -X POST -d '{\"name\": \"{{inputs.name}}\"}' https://api.example.com",
			wantLen:  1,
		},
		{
			name:     "multiline script",
			template: "#!/bin/bash\nNAME={{inputs.name}}\necho \"Result: {{states.process.output}}\"",
			wantLen:  2,
		},
		{
			name:     "json output",
			template: `{"workflow": "{{workflow.name}}", "result": "{{states.final.output}}"}`,
			wantLen:  2,
		},
		{
			name:     "error handler",
			template: "echo 'Error in {{error.state}}: {{error.message}}' >> {{context.working_dir}}/error.log",
			wantLen:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			assert.Len(t, refs, tt.wantLen)
			if tt.wantRaw != "" && len(refs) > 0 {
				assert.Equal(t, tt.wantRaw, refs[0].Raw)
			}
		})
	}
}

func TestParseReference_LeadingDot(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantType interpolation.ReferenceType
		wantNS   string
		wantPath string
		wantProp string
	}{
		{
			name:     "states",
			path:     ".states.build.Output",
			wantType: interpolation.TypeStates,
			wantNS:   "states",
			wantPath: "build",
			wantProp: "Output",
		},
		{
			name:     "inputs",
			path:     ".inputs.pr_base",
			wantType: interpolation.TypeInputs,
			wantNS:   "inputs",
			wantPath: "pr_base",
		},
		{
			name:     "workflow",
			path:     ".workflow.Duration",
			wantType: interpolation.TypeWorkflow,
			wantNS:   "workflow",
			wantPath: "Duration",
		},
		{
			name:     "error",
			path:     ".error.Message",
			wantType: interpolation.TypeError,
			wantNS:   "error",
			wantPath: "Message",
		},
		{
			name:     "loop",
			path:     ".loop.Index",
			wantType: interpolation.TypeLoop,
			wantNS:   "loop",
			wantPath: "Index",
		},
		{
			name:     "env",
			path:     ".env.HOME",
			wantType: interpolation.TypeEnv,
			wantNS:   "env",
			wantPath: "HOME",
		},
		{
			name:     "context",
			path:     ".context.working_dir",
			wantType: interpolation.TypeContext,
			wantNS:   "context",
			wantPath: "working_dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := interpolation.ParseReference(tt.path)
			assert.Equal(t, tt.wantType, ref.Type)
			assert.Equal(t, tt.wantNS, ref.Namespace)
			assert.Equal(t, tt.wantPath, ref.Path)
			if tt.wantProp != "" {
				assert.Equal(t, tt.wantProp, ref.Property)
			}
		})
	}
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
	template := "{{.states.step1.Output}} and {{states.step2.Output}}"
	refs, err := interpolation.ExtractReferences(template)

	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, interpolation.TypeStates, refs[0].Type)
	assert.Equal(t, interpolation.TypeStates, refs[1].Type)
	assert.Equal(t, "step1", refs[0].Path)
	assert.Equal(t, "step2", refs[1].Path)
}

func TestExtractReferences_RealWorldLeadingDot(t *testing.T) {
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

func TestValidationMaps_Comprehensive(t *testing.T) {
	type mapSpec struct {
		validMap   map[string]bool
		required   []string
		invalid    []string
		deprecated []string
	}

	specs := map[string]mapSpec{
		"ValidWorkflowProperties": {
			validMap:   interpolation.ValidWorkflowProperties,
			required:   []string{"ID", "Name", "CurrentState", "StartedAt", "Duration"},
			invalid:    []string{"invalid", ""},
			deprecated: []string{"id", "name", "current_state", "started_at", "duration"},
		},
		"ValidStateProperties": {
			validMap:   interpolation.ValidStateProperties,
			required:   []string{"Output", "Stderr", "ExitCode", "Status", "Response", "TokensUsed", "JSON"},
			invalid:    []string{"stdout", "result", ""},
			deprecated: []string{"output", "stderr", "exit_code", "status", "response", "tokensused", "json"},
		},
		"ValidErrorProperties": {
			validMap:   interpolation.ValidErrorProperties,
			required:   []string{"Message", "State", "ExitCode", "Type"},
			invalid:    []string{"code", ""},
			deprecated: []string{"message", "state", "exit_code", "type"},
		},
		"ValidContextProperties": {
			validMap:   interpolation.ValidContextProperties,
			required:   []string{"WorkingDir", "User", "Hostname"},
			invalid:    []string{"cwd", ""},
			deprecated: []string{"working_dir", "user", "hostname"},
		},
	}

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			for _, key := range spec.required {
				assert.True(t, spec.validMap[key])
			}

			for _, key := range spec.invalid {
				assert.False(t, spec.validMap[key])
			}

			for _, key := range spec.deprecated {
				assert.False(t, spec.validMap[key])
			}

			assert.Equal(t, len(spec.required), len(spec.validMap))
		})
	}
}
