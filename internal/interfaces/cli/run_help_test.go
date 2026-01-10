package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// =============================================================================
// RenderWorkflowHelp Tests (US1, US2, US3, US4)
// =============================================================================

func TestRenderWorkflowHelp_WorkflowWithMultipleInputs(t *testing.T) {
	// US1: View workflow input arguments
	wf := &workflow.Workflow{
		Name:        "deploy",
		Description: "Deploy application to target environment",
		Inputs: []workflow.Input{
			{
				Name:        "branch",
				Type:        "string",
				Required:    true,
				Description: "Git branch to deploy",
			},
			{
				Name:        "environment",
				Type:        "string",
				Required:    false,
				Default:     "staging",
				Description: "Target deployment environment",
			},
			{
				Name:        "dry_run",
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Simulate deployment without making changes",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Verify all inputs are shown
	assert.Contains(t, output, "branch")
	assert.Contains(t, output, "environment")
	assert.Contains(t, output, "dry_run")
	// Verify types are shown
	assert.Contains(t, output, "string")
	assert.Contains(t, output, "boolean")
	// Verify descriptions are shown
	assert.Contains(t, output, "Git branch to deploy")
	assert.Contains(t, output, "Target deployment environment")
}

func TestRenderWorkflowHelp_WorkflowWithNoInputs(t *testing.T) {
	// US1: Workflow with no inputs should show appropriate message
	wf := &workflow.Workflow{
		Name:        "minimal",
		Description: "A workflow with no inputs",
		Inputs:      []workflow.Input{},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should show message about no inputs
	assert.Contains(t, output, "No input parameters")
}

func TestRenderWorkflowHelp_WorkflowWithDescription(t *testing.T) {
	// US4: Show workflow description
	wf := &workflow.Workflow{
		Name:        "analyze-code",
		Description: "Analyze source code for issues and generate a report",
		Inputs: []workflow.Input{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: "Path to source code",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Workflow description should appear
	assert.Contains(t, output, "Analyze source code for issues and generate a report")
}

func TestRenderWorkflowHelp_WorkflowWithoutDescription(t *testing.T) {
	// US4: Workflow without description should not show description section
	wf := &workflow.Workflow{
		Name:        "simple",
		Description: "", // No description
		Inputs: []workflow.Input{
			{
				Name:     "file",
				Type:     "string",
				Required: true,
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should still show inputs even without workflow description
	assert.Contains(t, output, "file")
	// Should NOT contain description section header when no description
	assert.NotContains(t, output, "Description:")
}

func TestRenderWorkflowHelp_InputWithDescription(t *testing.T) {
	// US2: Display input descriptions
	wf := &workflow.Workflow{
		Name: "test-wf",
		Inputs: []workflow.Input{
			{
				Name:        "greeting",
				Type:        "string",
				Required:    false,
				Default:     "hello",
				Description: "Greeting message to display",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Input description should be shown
	assert.Contains(t, output, "Greeting message to display")
}

func TestRenderWorkflowHelp_InputWithoutDescription(t *testing.T) {
	// US2: Inputs missing description should show placeholder
	wf := &workflow.Workflow{
		Name: "test-wf",
		Inputs: []workflow.Input{
			{
				Name:        "undocumented",
				Type:        "string",
				Required:    true,
				Description: "", // No description
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should show "No description" placeholder for inputs without description
	assert.Contains(t, output, "No description")
}

func TestRenderWorkflowHelp_OptionalInputWithDefault(t *testing.T) {
	// US3: Display default values for optional inputs
	wf := &workflow.Workflow{
		Name: "test-wf",
		Inputs: []workflow.Input{
			{
				Name:        "timeout",
				Type:        "integer",
				Required:    false,
				Default:     30,
				Description: "Request timeout in seconds",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should show default value
	assert.Contains(t, output, "30")
}

func TestRenderWorkflowHelp_OptionalInputWithoutDefault(t *testing.T) {
	// US3: Optional input without default should not show default value
	wf := &workflow.Workflow{
		Name: "test-wf",
		Inputs: []workflow.Input{
			{
				Name:        "optional_field",
				Type:        "string",
				Required:    false,
				Default:     nil, // No default
				Description: "An optional field",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should show "-" for no default
	assert.Contains(t, output, "optional_field")
	assert.Contains(t, output, "-")
}

func TestRenderWorkflowHelp_TableFormat(t *testing.T) {
	// FR-003: Help output should follow Cobra help conventions with table format
	tests := []struct {
		name        string
		workflow    *workflow.Workflow
		wantHeaders []string
	}{
		{
			name: "full workflow",
			workflow: &workflow.Workflow{
				Name:        "complete",
				Description: "A complete workflow",
				Inputs: []workflow.Input{
					{Name: "branch", Type: "string", Required: true, Description: "Git branch"},
					{Name: "verbose", Type: "boolean", Required: false, Default: false, Description: "Verbose mode"},
					{Name: "count", Type: "integer", Required: false, Default: 10, Description: "Count"},
				},
			},
			wantHeaders: []string{"NAME", "TYPE", "REQUIRED", "DEFAULT", "DESCRIPTION"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := &cobra.Command{}
			cmd.SetOut(buf)

			err := RenderWorkflowHelp(cmd, tt.workflow, buf, true)
			require.NoError(t, err, "should render help without error")

			output := buf.String()
			// Should contain table headers
			for _, header := range tt.wantHeaders {
				assert.Contains(t, output, header)
			}
		})
	}
}

func TestRenderWorkflowHelp_AllInputTypes(t *testing.T) {
	// FR-002: Each input must show type (string/integer/boolean)
	wf := &workflow.Workflow{
		Name: "multi-type",
		Inputs: []workflow.Input{
			{Name: "name", Type: "string", Required: true, Description: "Name"},
			{Name: "count", Type: "integer", Required: true, Description: "Count"},
			{Name: "enabled", Type: "boolean", Required: false, Default: true, Description: "Enabled flag"},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should contain all three types
	assert.Contains(t, output, "string")
	assert.Contains(t, output, "integer")
	assert.Contains(t, output, "boolean")
}

func TestRenderWorkflowHelp_RequiredAndOptionalInputs(t *testing.T) {
	// FR-002: Each input must show required/optional status
	wf := &workflow.Workflow{
		Name: "mixed-required",
		Inputs: []workflow.Input{
			{Name: "required_field", Type: "string", Required: true, Description: "Required input"},
			{Name: "optional_field", Type: "string", Required: false, Description: "Optional input"},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should show both "yes" and "no" for required status
	assert.Contains(t, output, "yes")
	assert.Contains(t, output, "no")
}

func TestRenderWorkflowHelp_80ColumnTerminal(t *testing.T) {
	// NFR-002: Help output must be readable in 80-column terminals
	wf := &workflow.Workflow{
		Name:        "wide-content",
		Description: "A workflow with a very long description that should wrap properly in narrow terminal windows to ensure readability",
		Inputs: []workflow.Input{
			{
				Name:        "very_long_input_name_that_might_cause_wrapping",
				Type:        "string",
				Required:    true,
				Description: "This is a very long description that could potentially cause line wrapping issues in narrow terminal windows",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	// Should render without panic - content should be present
	assert.Contains(t, output, "very_long_input_name")
}

func TestRenderWorkflowHelp_ColorEnabled(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "color-test",
		Inputs: []workflow.Input{
			{Name: "input", Type: "string", Required: true, Description: "Test input"},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	// Test with color enabled (noColor: false)
	err := RenderWorkflowHelp(cmd, wf, buf, false)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	assert.Contains(t, output, "input")
}

func TestRenderWorkflowHelp_ColorDisabled(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "no-color-test",
		Inputs: []workflow.Input{
			{Name: "input", Type: "string", Required: true, Description: "Test input"},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	// Test with color disabled (noColor: true)
	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should render help without error")

	output := buf.String()
	assert.Contains(t, output, "input")
}

// =============================================================================
// workflowToInputInfos Tests
// =============================================================================

func TestWorkflowToInputInfos_EmptyInputs(t *testing.T) {
	wf := &workflow.Workflow{
		Name:   "empty-inputs",
		Inputs: []workflow.Input{},
	}

	result := workflowToInputInfos(wf)

	require.NotNil(t, result, "should return empty slice, not nil")
	assert.Len(t, result, 0)
}

func TestWorkflowToInputInfos_SingleInput(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "single-input",
		Inputs: []workflow.Input{
			{
				Name:        "file",
				Type:        "string",
				Required:    true,
				Default:     nil,
				Description: "File to process",
			},
		},
	}

	result := workflowToInputInfos(wf)

	require.Len(t, result, 1)
	assert.Equal(t, "file", result[0].Name)
	assert.Equal(t, "string", result[0].Type)
	assert.True(t, result[0].Required)
	assert.Equal(t, "File to process", result[0].Description)
}

func TestWorkflowToInputInfos_MultipleInputs(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "multi-inputs",
		Inputs: []workflow.Input{
			{Name: "input1", Type: "string", Required: true, Description: "First input"},
			{Name: "input2", Type: "integer", Required: false, Default: 10, Description: "Second input"},
			{Name: "input3", Type: "boolean", Required: false, Default: true, Description: "Third input"},
		},
	}

	result := workflowToInputInfos(wf)

	require.Len(t, result, 3)
	assert.Equal(t, "input1", result[0].Name)
	assert.Equal(t, "input2", result[1].Name)
	assert.Equal(t, "input3", result[2].Name)
}

func TestWorkflowToInputInfos_DefaultValueConversion(t *testing.T) {
	tests := []struct {
		name        string
		input       workflow.Input
		wantDefault string
	}{
		{
			name: "string default",
			input: workflow.Input{
				Name:    "str",
				Type:    "string",
				Default: "hello",
			},
			wantDefault: "hello",
		},
		{
			name: "integer default",
			input: workflow.Input{
				Name:    "num",
				Type:    "integer",
				Default: 42,
			},
			wantDefault: "42",
		},
		{
			name: "boolean true default",
			input: workflow.Input{
				Name:    "flag",
				Type:    "boolean",
				Default: true,
			},
			wantDefault: "true",
		},
		{
			name: "boolean false default",
			input: workflow.Input{
				Name:    "flag",
				Type:    "boolean",
				Default: false,
			},
			wantDefault: "false",
		},
		{
			name: "nil default",
			input: workflow.Input{
				Name:    "empty",
				Type:    "string",
				Default: nil,
			},
			wantDefault: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:   "test",
				Inputs: []workflow.Input{tt.input},
			}

			result := workflowToInputInfos(wf)

			require.Len(t, result, 1)
			assert.Equal(t, tt.wantDefault, result[0].Default)
		})
	}
}

func TestWorkflowToInputInfos_PreservesOrder(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "ordered",
		Inputs: []workflow.Input{
			{Name: "first"},
			{Name: "second"},
			{Name: "third"},
			{Name: "fourth"},
		},
	}

	result := workflowToInputInfos(wf)

	require.Len(t, result, 4)
	assert.Equal(t, "first", result[0].Name)
	assert.Equal(t, "second", result[1].Name)
	assert.Equal(t, "third", result[2].Name)
	assert.Equal(t, "fourth", result[3].Name)
}

// =============================================================================
// formatInputsTable Tests
// =============================================================================

func TestFormatInputsTable_EmptyInputs(t *testing.T) {
	inputs := []ui.InputInfo{}
	buf := new(bytes.Buffer)

	err := formatInputsTable(inputs, buf, true)
	require.NoError(t, err, "should handle empty inputs without error")

	output := buf.String()
	assert.Contains(t, output, "No input parameters")
}

func TestFormatInputsTable_SingleInput(t *testing.T) {
	inputs := []ui.InputInfo{
		{
			Name:        "branch",
			Type:        "string",
			Required:    true,
			Default:     "",
			Description: "Git branch name",
		},
	}
	buf := new(bytes.Buffer)

	err := formatInputsTable(inputs, buf, true)
	require.NoError(t, err, "should format single input without error")

	output := buf.String()
	assert.Contains(t, output, "branch")
	assert.Contains(t, output, "string")
	assert.Contains(t, output, "yes")
	assert.Contains(t, output, "Git branch name")
}

func TestFormatInputsTable_MultipleInputs(t *testing.T) {
	inputs := []ui.InputInfo{
		{Name: "branch", Type: "string", Required: true, Default: "", Description: "Branch name"},
		{Name: "count", Type: "integer", Required: false, Default: "10", Description: "Item count"},
		{Name: "verbose", Type: "boolean", Required: false, Default: "false", Description: "Verbose mode"},
	}
	buf := new(bytes.Buffer)

	err := formatInputsTable(inputs, buf, true)
	require.NoError(t, err, "should format multiple inputs without error")

	output := buf.String()
	// Should contain all inputs
	assert.Contains(t, output, "branch")
	assert.Contains(t, output, "count")
	assert.Contains(t, output, "verbose")
	// Should contain headers
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "TYPE")
}

func TestFormatInputsTable_AlignedColumns(t *testing.T) {
	inputs := []ui.InputInfo{
		{Name: "a", Type: "string", Required: true, Default: "", Description: "Short"},
		{Name: "very_long_name", Type: "boolean", Required: false, Default: "true", Description: "A much longer description"},
	}
	buf := new(bytes.Buffer)

	err := formatInputsTable(inputs, buf, true)
	require.NoError(t, err, "should format with proper alignment")

	output := buf.String()
	// Both names should be present - alignment is handled by tabwriter
	assert.Contains(t, output, "a")
	assert.Contains(t, output, "very_long_name")
}

// =============================================================================
// formatDefaultValue Tests
// =============================================================================

func TestFormatDefaultValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "empty string",
			value: "",
			want:  "-",
		},
		{
			name:  "non-empty string value",
			value: "hello",
			want:  "hello",
		},
		{
			name:  "numeric value",
			value: "42",
			want:  "42",
		},
		{
			name:  "boolean value",
			value: "true",
			want:  "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDefaultValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// formatRequired Tests
// =============================================================================

func TestFormatRequired(t *testing.T) {
	tests := []struct {
		name     string
		required bool
		want     string
	}{
		{
			name:     "required input",
			required: true,
			want:     "yes",
		},
		{
			name:     "optional input",
			required: false,
			want:     "no",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRequired(tt.required)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// formatDescription Tests
// =============================================================================

func TestFormatDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "empty description",
			description: "",
			want:        "No description",
		},
		{
			name:        "non-empty description",
			description: "A helpful description",
			want:        "A helpful description",
		},
		{
			name:        "whitespace only description",
			description: "   ",
			want:        "No description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDescription(tt.description)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Edge Cases and Boundary Conditions
// =============================================================================

func TestRenderWorkflowHelp_NilWorkflow(t *testing.T) {
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	// This tests defensive coding - should not panic and should return error
	err := RenderWorkflowHelp(cmd, nil, buf, true)
	require.Error(t, err, "should return error for nil workflow")
}

func TestRenderWorkflowHelp_SpecialCharactersInDescription(t *testing.T) {
	wf := &workflow.Workflow{
		Name:        "special-chars",
		Description: "Workflow with special chars: <>&\"'",
		Inputs: []workflow.Input{
			{
				Name:        "path",
				Type:        "string",
				Required:    true,
				Description: `Path with "quotes" and <brackets>`,
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should handle special characters without error")

	output := buf.String()
	// Special characters should be preserved in output
	assert.Contains(t, output, "path")
}

func TestRenderWorkflowHelp_UnicodeContent(t *testing.T) {
	wf := &workflow.Workflow{
		Name:        "unicode-wf",
		Description: "Workflow with unicode: Hello World",
		Inputs: []workflow.Input{
			{
				Name:        "message",
				Type:        "string",
				Required:    false,
				Default:     "Hello",
				Description: "Unicode supported input",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should handle unicode content without error")

	output := buf.String()
	assert.Contains(t, output, "message")
}

func TestRenderWorkflowHelp_VeryLongInputName(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "long-names",
		Inputs: []workflow.Input{
			{
				Name:        "this_is_a_very_long_input_name_that_might_cause_formatting_issues_in_the_table",
				Type:        "string",
				Required:    true,
				Description: "Input with very long name",
			},
		},
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should handle long input names without error")

	output := buf.String()
	// Long names should be present (may be truncated)
	assert.Contains(t, output, "this_is_a_very_long")
}

func TestRenderWorkflowHelp_ManyInputs(t *testing.T) {
	// Create workflow with many inputs to test table rendering
	inputs := make([]workflow.Input, 20)
	for i := 0; i < 20; i++ {
		inputs[i] = workflow.Input{
			Name:        "input_" + string(rune('a'+i)),
			Type:        "string",
			Required:    i%2 == 0,
			Description: "Input number " + string(rune('A'+i)),
		}
	}

	wf := &workflow.Workflow{
		Name:        "many-inputs",
		Description: "Workflow with many inputs",
		Inputs:      inputs,
	}

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)

	err := RenderWorkflowHelp(cmd, wf, buf, true)
	require.NoError(t, err, "should handle many inputs without error")

	output := buf.String()
	// First and last inputs should be present
	assert.Contains(t, output, "input_a")
	assert.Contains(t, output, "input_t") // 20th input (a-t)
}

// =============================================================================
// Integration with ui.InputInfo
// =============================================================================

func TestWorkflowToInputInfos_MapsAllFields(t *testing.T) {
	input := workflow.Input{
		Name:        "complete_input",
		Type:        "string",
		Required:    true,
		Default:     "default_value",
		Description: "Complete input with all fields",
	}

	wf := &workflow.Workflow{
		Name:   "test",
		Inputs: []workflow.Input{input},
	}

	result := workflowToInputInfos(wf)

	require.Len(t, result, 1)
	info := result[0]
	assert.Equal(t, "complete_input", info.Name)
	assert.Equal(t, "string", info.Type)
	assert.True(t, info.Required)
	assert.Equal(t, "default_value", info.Default)
	assert.Equal(t, "Complete input with all fields", info.Description)
}

// =============================================================================
// Table-driven comprehensive tests
// =============================================================================

func TestRenderWorkflowHelp_Scenarios(t *testing.T) {
	tests := []struct {
		name     string
		workflow *workflow.Workflow
		noColor  bool
		wantErr  bool
		wantOut  []string // expected substrings in output
	}{
		{
			name: "standard workflow",
			workflow: &workflow.Workflow{
				Name:        "standard",
				Description: "Standard workflow",
				Inputs: []workflow.Input{
					{Name: "input1", Type: "string", Required: true, Description: "First"},
				},
			},
			noColor: true,
			wantErr: false,
			wantOut: []string{"input1", "string", "First"},
		},
		{
			name: "workflow with all input types",
			workflow: &workflow.Workflow{
				Name: "all-types",
				Inputs: []workflow.Input{
					{Name: "str", Type: "string", Required: false, Default: "default"},
					{Name: "num", Type: "integer", Required: false, Default: 100},
					{Name: "bool", Type: "boolean", Required: false, Default: true},
				},
			},
			noColor: true,
			wantErr: false,
			wantOut: []string{"str", "num", "bool", "string", "integer", "boolean"},
		},
		{
			name: "empty workflow",
			workflow: &workflow.Workflow{
				Name:   "empty",
				Inputs: nil,
			},
			noColor: true,
			wantErr: false,
			wantOut: []string{"No input parameters"},
		},
		{
			name: "workflow with only required inputs",
			workflow: &workflow.Workflow{
				Name: "all-required",
				Inputs: []workflow.Input{
					{Name: "a", Type: "string", Required: true},
					{Name: "b", Type: "string", Required: true},
				},
			},
			noColor: true,
			wantErr: false,
			wantOut: []string{"a", "b", "yes"},
		},
		{
			name: "workflow with only optional inputs",
			workflow: &workflow.Workflow{
				Name: "all-optional",
				Inputs: []workflow.Input{
					{Name: "x", Type: "string", Required: false, Default: "val1"},
					{Name: "y", Type: "integer", Required: false, Default: 0},
				},
			},
			noColor: false, // With color
			wantErr: false,
			wantOut: []string{"x", "y", "no"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			cmd := &cobra.Command{}
			cmd.SetOut(buf)

			err := RenderWorkflowHelp(cmd, tt.workflow, buf, tt.noColor)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				output := buf.String()
				for _, want := range tt.wantOut {
					assert.Contains(t, output, want)
				}
			}
		})
	}
}
