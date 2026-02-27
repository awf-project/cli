package interpolation_test

import (
	"testing"

	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidStateProperties_IncludesJSON verifies that the JSON field
// is included in ValidStateProperties for F065.
func TestValidStateProperties_IncludesJSON(t *testing.T) {
	assert.True(t, interpolation.ValidStateProperties["JSON"],
		"JSON field must be in ValidStateProperties for F065")
}

// TestValidStateProperties_AllFields verifies all expected fields
// including the new JSON field are present.
func TestValidStateProperties_AllFields(t *testing.T) {
	expectedFields := []string{
		"Output",
		"Stderr",
		"ExitCode",
		"Status",
		"Response",
		"TokensUsed",
		"JSON", // F065: new field for explicit JSON output
	}

	for _, field := range expectedFields {
		assert.True(t, interpolation.ValidStateProperties[field],
			"ValidStateProperties must include %s", field)
	}

	// Verify exact count - no extra fields
	assert.Equal(t, len(expectedFields), len(interpolation.ValidStateProperties),
		"ValidStateProperties should have exactly %d fields", len(expectedFields))
}

// TestValidStateProperties_NoLowercaseJSON verifies that lowercase
// "json" is not accepted (Go naming conventions require uppercase).
func TestValidStateProperties_NoLowercaseJSON(t *testing.T) {
	assert.False(t, interpolation.ValidStateProperties["json"],
		"lowercase 'json' should not be in ValidStateProperties")
	assert.False(t, interpolation.ValidStateProperties["Json"],
		"mixed-case 'Json' should not be in ValidStateProperties")
}

// TestExtractReferences_JSONField tests that JSON field references
// are correctly parsed from templates.
func TestExtractReferences_JSONField(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		wantType     interpolation.ReferenceType
		wantPath     string
		wantProperty string
	}{
		{
			name:         "JSON field reference",
			template:     "{{states.agent_step.JSON}}",
			wantType:     interpolation.TypeStates,
			wantPath:     "agent_step",
			wantProperty: "JSON",
		},
		{
			name:         "JSON field with nested access",
			template:     "{{states.agent_step.JSON.name}}",
			wantType:     interpolation.TypeStates,
			wantPath:     "agent_step",
			wantProperty: "JSON",
		},
		{
			name:         "leading dot syntax",
			template:     "{{.states.agent_step.JSON}}",
			wantType:     interpolation.TypeStates,
			wantPath:     "agent_step",
			wantProperty: "JSON",
		},
		{
			name:         "leading dot with nested access",
			template:     "{{.states.agent_step.JSON.field}}",
			wantType:     interpolation.TypeStates,
			wantPath:     "agent_step",
			wantProperty: "JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			require.Len(t, refs, 1, "should extract exactly one reference")

			assert.Equal(t, tt.wantType, refs[0].Type)
			assert.Equal(t, tt.wantPath, refs[0].Path)
			assert.Equal(t, tt.wantProperty, refs[0].Property)
		})
	}
}

// TestExtractReferences_JSONFieldMultiple tests multiple JSON field
// references in the same template.
func TestExtractReferences_JSONFieldMultiple(t *testing.T) {
	template := "user: {{.states.step1.JSON.name}}, count: {{.states.step2.JSON.count}}"

	refs, err := interpolation.ExtractReferences(template)
	require.NoError(t, err)
	require.Len(t, refs, 2)

	// First reference
	assert.Equal(t, interpolation.TypeStates, refs[0].Type)
	assert.Equal(t, "step1", refs[0].Path)
	assert.Equal(t, "JSON", refs[0].Property)

	// Second reference
	assert.Equal(t, interpolation.TypeStates, refs[1].Type)
	assert.Equal(t, "step2", refs[1].Path)
	assert.Equal(t, "JSON", refs[1].Property)
}

// TestExtractReferences_JSONvsResponse verifies that JSON and Response
// are treated as separate properties (both valid).
func TestExtractReferences_JSONvsResponse(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		wantProperty string
	}{
		{
			name:         "Response field",
			template:     "{{.states.step.Response.field}}",
			wantProperty: "Response",
		},
		{
			name:         "JSON field",
			template:     "{{.states.step.JSON.field}}",
			wantProperty: "JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)
			require.NoError(t, err)
			require.Len(t, refs, 1)
			assert.Equal(t, tt.wantProperty, refs[0].Property)

			// Verify both are valid
			assert.True(t, interpolation.ValidStateProperties[tt.wantProperty])
		})
	}
}

// TestParseReference_JSONField tests low-level parsing of JSON field references.
func TestParseReference_JSONField(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantType     interpolation.ReferenceType
		wantNS       string
		wantPath     string
		wantProperty string
	}{
		{
			name:         "states.step.JSON",
			path:         "states.step.JSON",
			wantType:     interpolation.TypeStates,
			wantNS:       "states",
			wantPath:     "step",
			wantProperty: "JSON",
		},
		{
			name:         "leading dot stripped",
			path:         ".states.step.JSON",
			wantType:     interpolation.TypeStates,
			wantNS:       "states",
			wantPath:     "step",
			wantProperty: "JSON",
		},
		{
			name:         "with nested field access",
			path:         "states.agent_step.JSON.name",
			wantType:     interpolation.TypeStates,
			wantNS:       "states",
			wantPath:     "agent_step",
			wantProperty: "JSON", // Property is always the first field after step name
		},
		{
			name:         "deeply nested JSON access",
			path:         "states.step.JSON.level1.level2.value",
			wantType:     interpolation.TypeStates,
			wantNS:       "states",
			wantPath:     "step",
			wantProperty: "JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := interpolation.ParseReference(tt.path)

			assert.Equal(t, tt.wantType, ref.Type)
			assert.Equal(t, tt.wantNS, ref.Namespace)
			assert.Equal(t, tt.wantPath, ref.Path)
			assert.Equal(t, tt.wantProperty, ref.Property)
		})
	}
}

// TestExtractReferences_JSONFieldEdgeCases tests edge cases for JSON field parsing.
func TestExtractReferences_JSONFieldEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantLen  int
		wantErr  bool
	}{
		{
			name:     "JSON in plain text (not a reference)",
			template: "this JSON should be parsed",
			wantLen:  0,
		},
		{
			name:     "JSON without states namespace",
			template: "{{JSON.field}}",
			wantLen:  1,
			// Should be parsed as unknown type
		},
		{
			name:     "lowercase json should still parse (but won't validate)",
			template: "{{.states.step.json}}",
			wantLen:  1,
		},
		{
			name:     "JSON in different namespaces",
			template: "{{.states.step.JSON}} and {{.inputs.JSON}}",
			wantLen:  2,
		},
		{
			name:     "empty JSON reference",
			template: "{{.states..JSON}}",
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := interpolation.ExtractReferences(tt.template)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, refs, tt.wantLen)
		})
	}
}

// TestExtractReferences_RealWorldJSONUsage tests realistic JSON field usage
// patterns from actual workflows.
func TestExtractReferences_RealWorldJSONUsage(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantLen  int
	}{
		{
			name:     "shell command with JSON field",
			template: `curl -X POST -d '{"name": "{{.states.agent_step.JSON.name}}"}' https://api.example.com`,
			wantLen:  1,
		},
		{
			name:     "multiline script with JSON access",
			template: "#!/bin/bash\nNAME={{.states.agent_step.JSON.name}}\nCOUNT={{.states.agent_step.JSON.count}}\necho \"Processing $NAME with $COUNT items\"",
			wantLen:  2,
		},
		{
			name:     "JSON output construction from JSON field",
			template: `{"workflow": "{{.workflow.Name}}", "result": {"name": "{{.states.agent_step.JSON.name}}", "status": "{{.states.agent_step.JSON.status}}"}}`,
			wantLen:  3,
		},
		{
			name:     "conditional based on JSON field",
			template: `{{if .states.agent_step.JSON.enabled}}ENABLED=1{{else}}ENABLED=0{{end}}`,
			wantLen:  3, // "if", "else", "end" are each parsed as separate references
		},
		{
			name:     "combining JSON and Response fields",
			template: "explicit: {{.states.step1.JSON.value}}, auto: {{.states.step2.Response.value}}",
			wantLen:  2,
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

// TestCategorizeNamespace_JSONNotANamespace verifies that "JSON" itself
// is not a namespace, only a property of states.
func TestCategorizeNamespace_JSONNotANamespace(t *testing.T) {
	refType := interpolation.CategorizeNamespace("JSON")
	assert.Equal(t, interpolation.TypeUnknown, refType,
		"JSON should not be recognized as a namespace")

	refType = interpolation.CategorizeNamespace("json")
	assert.Equal(t, interpolation.TypeUnknown, refType,
		"json (lowercase) should not be recognized as a namespace")
}
