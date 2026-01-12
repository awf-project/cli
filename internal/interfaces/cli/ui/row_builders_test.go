package ui

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// formatInputRow Tests (C004 - Component T008)
// Feature: C004 - Extract row builders to reduce writeValidationResultTable complexity
// =============================================================================

func TestFormatInputRow_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		input          InputInfo
		wantName       string
		wantType       string
		wantRequired   string
		wantDefaultVal string
	}{
		{
			name: "required input with default",
			input: InputInfo{
				Name:     "env",
				Type:     "string",
				Required: true,
				Default:  "production",
			},
			wantName:       "env",
			wantType:       "string",
			wantRequired:   "yes",
			wantDefaultVal: "production",
		},
		{
			name: "optional input without default",
			input: InputInfo{
				Name:     "region",
				Type:     "string",
				Required: false,
				Default:  "",
			},
			wantName:       "region",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "-",
		},
		{
			name: "required input without default",
			input: InputInfo{
				Name:     "api_key",
				Type:     "string",
				Required: true,
				Default:  "",
			},
			wantName:       "api_key",
			wantType:       "string",
			wantRequired:   "yes",
			wantDefaultVal: "-",
		},
		{
			name: "optional input with default",
			input: InputInfo{
				Name:     "timeout",
				Type:     "number",
				Required: false,
				Default:  "30s",
			},
			wantName:       "timeout",
			wantType:       "number",
			wantRequired:   "no",
			wantDefaultVal: "30s",
		},
		{
			name: "boolean type required",
			input: InputInfo{
				Name:     "verbose",
				Type:     "boolean",
				Required: true,
				Default:  "false",
			},
			wantName:       "verbose",
			wantType:       "boolean",
			wantRequired:   "yes",
			wantDefaultVal: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, typ, required, defaultVal := formatInputRow(tt.input)

			assert.Equal(t, tt.wantName, name, "name should match")
			assert.Equal(t, tt.wantType, typ, "type should match")
			assert.Equal(t, tt.wantRequired, required, "required should match")
			assert.Equal(t, tt.wantDefaultVal, defaultVal, "defaultVal should match")
		})
	}
}

func TestFormatInputRow_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		input          InputInfo
		wantName       string
		wantType       string
		wantRequired   string
		wantDefaultVal string
	}{
		{
			name: "empty name",
			input: InputInfo{
				Name:     "",
				Type:     "string",
				Required: false,
				Default:  "",
			},
			wantName:       "",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "-",
		},
		{
			name: "empty type",
			input: InputInfo{
				Name:     "input",
				Type:     "",
				Required: true,
				Default:  "",
			},
			wantName:       "input",
			wantType:       "",
			wantRequired:   "yes",
			wantDefaultVal: "-",
		},
		{
			name: "whitespace only default",
			input: InputInfo{
				Name:     "spacing",
				Type:     "string",
				Required: false,
				Default:  "   ",
			},
			wantName:       "spacing",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "   ",
		},
		{
			name: "default value with special characters",
			input: InputInfo{
				Name:     "command",
				Type:     "string",
				Required: false,
				Default:  "echo 'hello world' | grep hello",
			},
			wantName:       "command",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "echo 'hello world' | grep hello",
		},
		{
			name: "default value with newlines",
			input: InputInfo{
				Name:     "script",
				Type:     "string",
				Required: false,
				Default:  "line1\nline2\nline3",
			},
			wantName:       "script",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "line1\nline2\nline3",
		},
		{
			name: "very long name",
			input: InputInfo{
				Name:     "very_long_input_name_that_might_cause_formatting_issues",
				Type:     "string",
				Required: true,
				Default:  "value",
			},
			wantName:       "very_long_input_name_that_might_cause_formatting_issues",
			wantType:       "string",
			wantRequired:   "yes",
			wantDefaultVal: "value",
		},
		{
			name: "very long default value",
			input: InputInfo{
				Name:     "path",
				Type:     "string",
				Required: false,
				Default:  "/very/long/path/to/some/directory/structure/that/is/deeply/nested",
			},
			wantName:       "path",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "/very/long/path/to/some/directory/structure/that/is/deeply/nested",
		},
		{
			name: "unicode in name and default",
			input: InputInfo{
				Name:     "名前",
				Type:     "string",
				Required: false,
				Default:  "デフォルト値",
			},
			wantName:       "名前",
			wantType:       "string",
			wantRequired:   "no",
			wantDefaultVal: "デフォルト値",
		},
		{
			name: "zero value default for number",
			input: InputInfo{
				Name:     "count",
				Type:     "number",
				Required: false,
				Default:  "0",
			},
			wantName:       "count",
			wantType:       "number",
			wantRequired:   "no",
			wantDefaultVal: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, typ, required, defaultVal := formatInputRow(tt.input)

			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantType, typ)
			assert.Equal(t, tt.wantRequired, required)
			assert.Equal(t, tt.wantDefaultVal, defaultVal)
		})
	}
}

func TestFormatInputRow_RequiredMapping(t *testing.T) {
	// Test that Required bool maps correctly to yes/no
	tests := []struct {
		name     string
		required bool
		want     string
	}{
		{"true maps to yes", true, "yes"},
		{"false maps to no", false, "no"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := InputInfo{
				Name:     "test",
				Type:     "string",
				Required: tt.required,
				Default:  "value",
			}

			_, _, required, _ := formatInputRow(input)

			assert.Equal(t, tt.want, required)
		})
	}
}

func TestFormatInputRow_DefaultValueDashSubstitution(t *testing.T) {
	// Test that empty default values are replaced with "-"
	tests := []struct {
		name string
		def  string
		want string
	}{
		{"empty string becomes dash", "", "-"},
		{"non-empty string unchanged", "value", "value"},
		{"dash string unchanged", "-", "-"},
		{"space string unchanged", " ", " "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := InputInfo{
				Name:     "test",
				Type:     "string",
				Required: false,
				Default:  tt.def,
			}

			_, _, _, defaultVal := formatInputRow(input)

			assert.Equal(t, tt.want, defaultVal)
		})
	}
}

// =============================================================================
// formatStepRow Tests (C004 - Component T008)
// =============================================================================

func TestFormatStepRow_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		step     StepSummary
		wantName string
		wantType string
		wantNext string
	}{
		{
			name: "step with next step",
			step: StepSummary{
				Name: "build",
				Type: "step",
				Next: "test",
			},
			wantName: "build",
			wantType: "step",
			wantNext: "test",
		},
		{
			name: "terminal step without next",
			step: StepSummary{
				Name: "deploy",
				Type: "step",
				Next: "",
			},
			wantName: "deploy",
			wantType: "step",
			wantNext: "(terminal)",
		},
		{
			name: "parallel step",
			step: StepSummary{
				Name: "test_parallel",
				Type: "parallel",
				Next: "deploy",
			},
			wantName: "test_parallel",
			wantType: "parallel",
			wantNext: "deploy",
		},
		{
			name: "loop step",
			step: StepSummary{
				Name: "process_items",
				Type: "loop",
				Next: "finalize",
			},
			wantName: "process_items",
			wantType: "loop",
			wantNext: "finalize",
		},
		{
			name: "choice step",
			step: StepSummary{
				Name: "branch",
				Type: "choice",
				Next: "conditional_step",
			},
			wantName: "branch",
			wantType: "choice",
			wantNext: "conditional_step",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, typ, next := formatStepRow(tt.step)

			assert.Equal(t, tt.wantName, name, "name should match")
			assert.Equal(t, tt.wantType, typ, "type should match")
			assert.Equal(t, tt.wantNext, next, "next should match")
		})
	}
}

func TestFormatStepRow_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		step     StepSummary
		wantName string
		wantType string
		wantNext string
	}{
		{
			name: "empty name",
			step: StepSummary{
				Name: "",
				Type: "step",
				Next: "next",
			},
			wantName: "",
			wantType: "step",
			wantNext: "next",
		},
		{
			name: "empty type",
			step: StepSummary{
				Name: "step1",
				Type: "",
				Next: "next",
			},
			wantName: "step1",
			wantType: "",
			wantNext: "next",
		},
		{
			name: "empty next becomes terminal",
			step: StepSummary{
				Name: "final",
				Type: "step",
				Next: "",
			},
			wantName: "final",
			wantType: "step",
			wantNext: "(terminal)",
		},
		{
			name: "whitespace only next",
			step: StepSummary{
				Name: "step1",
				Type: "step",
				Next: "   ",
			},
			wantName: "step1",
			wantType: "step",
			wantNext: "   ",
		},
		{
			name: "very long step name",
			step: StepSummary{
				Name: "very_long_step_name_that_might_cause_table_formatting_issues",
				Type: "step",
				Next: "next",
			},
			wantName: "very_long_step_name_that_might_cause_table_formatting_issues",
			wantType: "step",
			wantNext: "next",
		},
		{
			name: "very long next step name",
			step: StepSummary{
				Name: "step1",
				Type: "step",
				Next: "very_long_next_step_name_that_might_cause_issues",
			},
			wantName: "step1",
			wantType: "step",
			wantNext: "very_long_next_step_name_that_might_cause_issues",
		},
		{
			name: "unicode in all fields",
			step: StepSummary{
				Name: "ステップ",
				Type: "並列",
				Next: "次へ",
			},
			wantName: "ステップ",
			wantType: "並列",
			wantNext: "次へ",
		},
		{
			name: "special characters in step name",
			step: StepSummary{
				Name: "step-1_with.special@chars",
				Type: "step",
				Next: "step-2",
			},
			wantName: "step-1_with.special@chars",
			wantType: "step",
			wantNext: "step-2",
		},
		{
			name: "terminal keyword as next",
			step: StepSummary{
				Name: "step1",
				Type: "step",
				Next: "(terminal)",
			},
			wantName: "step1",
			wantType: "step",
			wantNext: "(terminal)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, typ, next := formatStepRow(tt.step)

			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantType, typ)
			assert.Equal(t, tt.wantNext, next)
		})
	}
}

func TestFormatStepRow_TerminalHandling(t *testing.T) {
	// Test that empty Next field is replaced with "(terminal)"
	tests := []struct {
		name string
		next string
		want string
	}{
		{"empty string becomes terminal", "", "(terminal)"},
		{"non-empty string unchanged", "next_step", "next_step"},
		{"terminal marker unchanged", "(terminal)", "(terminal)"},
		{"whitespace not treated as empty", " ", " "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := StepSummary{
				Name: "test",
				Type: "step",
				Next: tt.next,
			}

			_, _, next := formatStepRow(step)

			assert.Equal(t, tt.want, next)
		})
	}
}

// =============================================================================
// renderStatusHeader Tests (C004 - Component T008)
// =============================================================================

func TestRenderStatusHeader_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
		valid        bool
		wantContains []string
	}{
		{
			name:         "valid workflow",
			workflowName: "deploy-prod",
			valid:        true,
			wantContains: []string{"Workflow:", "deploy-prod", "Status:", "valid"},
		},
		{
			name:         "invalid workflow",
			workflowName: "test-workflow",
			valid:        false,
			wantContains: []string{"Workflow:", "test-workflow", "Status:", "invalid"},
		},
		{
			name:         "simple workflow name valid",
			workflowName: "build",
			valid:        true,
			wantContains: []string{"Workflow:", "build", "Status:", "valid"},
		},
		{
			name:         "simple workflow name invalid",
			workflowName: "lint",
			valid:        false,
			wantContains: []string{"Workflow:", "lint", "Status:", "invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			tw := newTableWriter(buf, 15, 10, 10, 20)

			renderStatusHeader(tw, tt.workflowName, tt.valid)

			output := buf.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain %q", want)
			}
		})
	}
}

func TestRenderStatusHeader_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
		valid        bool
		wantContains []string
	}{
		{
			name:         "empty workflow name",
			workflowName: "",
			valid:        true,
			wantContains: []string{"Workflow:", "Status:", "valid"},
		},
		{
			name:         "very long workflow name",
			workflowName: "very-long-workflow-name-that-exceeds-normal-formatting-width",
			valid:        false,
			wantContains: []string{"Workflow:", "very-long-workflow-name-that-exceeds-normal-formatting-width", "Status:", "invalid"},
		},
		{
			name:         "workflow name with spaces",
			workflowName: "my workflow",
			valid:        true,
			wantContains: []string{"Workflow:", "my workflow", "Status:", "valid"},
		},
		{
			name:         "workflow name with special chars",
			workflowName: "deploy-prod@v1.2.3",
			valid:        false,
			wantContains: []string{"Workflow:", "deploy-prod@v1.2.3", "Status:", "invalid"},
		},
		{
			name:         "unicode workflow name",
			workflowName: "ワークフロー",
			valid:        true,
			wantContains: []string{"Workflow:", "ワークフロー", "Status:", "valid"},
		},
		{
			name:         "workflow name with path separators",
			workflowName: "ci/cd/deploy",
			valid:        true,
			wantContains: []string{"Workflow:", "ci/cd/deploy", "Status:", "valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			tw := newTableWriter(buf, 15, 10, 10, 20)

			renderStatusHeader(tw, tt.workflowName, tt.valid)

			output := buf.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestRenderStatusHeader_ValidStatusMapping(t *testing.T) {
	// Test that valid bool maps correctly to "valid"/"invalid" status
	tests := []struct {
		name       string
		valid      bool
		wantStatus string
	}{
		{"true maps to valid", true, "valid"},
		{"false maps to invalid", false, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			tw := newTableWriter(buf, 15, 10, 10, 20)

			renderStatusHeader(tw, "test-workflow", tt.valid)

			output := buf.String()
			assert.Contains(t, output, "Status: "+tt.wantStatus)
		})
	}
}

func TestRenderStatusHeader_Formatting(t *testing.T) {
	// Test that header uses fullWidthSeparator and fullWidthRow
	buf := new(bytes.Buffer)
	tw := newTableWriter(buf, 15, 10, 10, 20)

	renderStatusHeader(tw, "test", true)

	output := buf.String()

	// Should contain separator lines (+ and -)
	assert.Contains(t, output, "+", "should have separator border")
	assert.Contains(t, output, "-", "should have separator line")

	// Should be a single full-width row
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	nonEmptyLines := 0
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) > 0 {
			nonEmptyLines++
		}
	}

	// Expect: separator line + content row + separator line (at least 3 lines)
	assert.GreaterOrEqual(t, nonEmptyLines, 3, "should have separator + content + separator")
}

func TestRenderStatusHeader_Integration(t *testing.T) {
	// Integration test: verify header can be rendered multiple times without issues
	buf := new(bytes.Buffer)
	tw := newTableWriter(buf, 15, 10, 10, 20)

	// Render first header
	renderStatusHeader(tw, "workflow1", true)
	firstOutput := buf.String()

	// Reset buffer
	buf.Reset()

	// Render second header
	renderStatusHeader(tw, "workflow2", false)
	secondOutput := buf.String()

	// Both should contain their respective content
	assert.Contains(t, firstOutput, "workflow1")
	assert.Contains(t, firstOutput, "valid")
	assert.Contains(t, secondOutput, "workflow2")
	assert.Contains(t, secondOutput, "invalid")

	// Outputs should be different
	assert.NotEqual(t, firstOutput, secondOutput)
}

// =============================================================================
// Cross-Function Integration Tests (C004 - Component T008)
// =============================================================================

func TestRowBuilders_IntegrationWithValidationTable(t *testing.T) {
	// Test that all three helpers work together to build a complete validation table

	// Create sample data
	inputs := []InputInfo{
		{Name: "env", Type: "string", Required: true, Default: "dev"},
		{Name: "region", Type: "string", Required: false, Default: ""},
	}

	steps := []StepSummary{
		{Name: "build", Type: "step", Next: "test"},
		{Name: "test", Type: "step", Next: ""},
	}

	// Format input rows
	for _, inp := range inputs {
		name, typ, required, def := formatInputRow(inp)
		require.NotEmpty(t, name, "input name should not be empty")
		require.NotEmpty(t, typ, "input type should not be empty")
		require.NotEmpty(t, required, "required field should not be empty")
		require.NotEmpty(t, def, "default field should not be empty")
	}

	// Format step rows
	for _, step := range steps {
		name, typ, next := formatStepRow(step)
		require.NotEmpty(t, name, "step name should not be empty")
		require.NotEmpty(t, typ, "step type should not be empty")
		require.NotEmpty(t, next, "next field should not be empty")
	}

	// Render status header
	buf := new(bytes.Buffer)
	tw := newTableWriter(buf, 15, 10, 10, 20)
	renderStatusHeader(tw, "test-workflow", true)

	output := buf.String()
	require.NotEmpty(t, output, "status header should produce output")
}

func TestRowBuilders_ConsistentBehaviorAcrossHelpers(t *testing.T) {
	// Verify that all helpers handle empty/nil inputs consistently

	// formatInputRow with minimal data
	name1, _, req1, def1 := formatInputRow(InputInfo{})
	assert.Equal(t, "", name1, "empty input should have empty name")
	assert.Equal(t, "no", req1, "empty input should default to not required")
	assert.Equal(t, "-", def1, "empty input should have dash as default")

	// formatStepRow with minimal data
	name2, _, next2 := formatStepRow(StepSummary{})
	assert.Equal(t, "", name2, "empty step should have empty name")
	assert.Equal(t, "(terminal)", next2, "empty step should be terminal")

	// renderStatusHeader should not panic with empty workflow
	buf := new(bytes.Buffer)
	tw := newTableWriter(buf, 15, 10, 10, 20)
	require.NotPanics(t, func() {
		renderStatusHeader(tw, "", true)
	}, "renderStatusHeader should not panic with empty workflow name")
}
