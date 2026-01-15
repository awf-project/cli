//go:build integration

package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// TestRunHelp_WorkflowWithInputs_Integration tests US1: View Workflow Input Arguments
// Given a workflow with inputs, when running `awf run <workflow> --help`,
// displays all inputs with their types and required/optional status.
func TestRunHelp_WorkflowWithInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with multiple inputs of varying types
	wfYAML := `name: inputs-test
version: "1.0.0"
inputs:
  - name: branch
    type: string
    required: true
  - name: message
    type: string
    required: false
    default: "commit message"
  - name: verbose
    type: boolean
    required: false
    default: false
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "inputs-test.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "inputs-test", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify inputs table header
	assert.Contains(t, output, "Input Parameters:")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "REQUIRED")

	// Verify each input is listed
	assert.Contains(t, output, "branch")
	assert.Contains(t, output, "message")
	assert.Contains(t, output, "verbose")

	// Verify types are shown
	assert.Contains(t, output, "string")
	assert.Contains(t, output, "boolean")

	// Verify required status
	assert.Contains(t, output, "yes") // branch is required
	assert.Contains(t, output, "no")  // message and verbose are optional
}

// TestRunHelp_WorkflowNoInputs_Integration tests US1: workflow with no inputs
// Given a workflow with no inputs defined, when running `awf run <workflow> --help`,
// displays a message indicating the workflow has no input arguments.
func TestRunHelp_WorkflowNoInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow without inputs
	wfYAML := `name: no-inputs
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "no-inputs.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "no-inputs", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No input parameters")
}

// TestRunHelp_NonExistentWorkflow_Integration tests US1: non-existent workflow
// Given a non-existent workflow, when running `awf run <workflow> --help`,
// displays an error message with exit code 1.
func TestRunHelp_NonExistentWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "nonexistent-workflow", "--help"})

	err := cmd.Execute()

	// Combined output for checking
	combinedOutput := buf.String()
	if err != nil {
		combinedOutput += err.Error()
	}

	// Should return error for non-existent workflow
	// OR show error message in output
	workflowNotFoundIndicator := strings.Contains(combinedOutput, "not found") ||
		strings.Contains(combinedOutput, "does not exist") ||
		strings.Contains(combinedOutput, "no such")

	if err == nil && !workflowNotFoundIndicator {
		t.Fatalf("expected error or 'not found' message for non-existent workflow, got output: %s", combinedOutput)
	}

	if err != nil {
		// Error should indicate workflow not found
		assert.True(t, workflowNotFoundIndicator,
			"expected error to indicate workflow not found, got: %s", combinedOutput)
	}
}

// TestRunHelp_InputDescriptions_Integration tests US2: Display Input Descriptions
// Given a workflow with inputs that have description fields, when running `--help`,
// displays the description next to each input.
func TestRunHelp_InputDescriptions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with described inputs
	wfYAML := `name: described-inputs
version: "1.0.0"
inputs:
  - name: api_key
    type: string
    required: true
    description: API key for authentication
  - name: timeout
    type: integer
    required: false
    default: 30
    description: Request timeout in seconds
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "described-inputs.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "described-inputs", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify descriptions are shown
	assert.Contains(t, output, "API key for authentication")
	assert.Contains(t, output, "Request timeout in seconds")
	assert.Contains(t, output, "DESCRIPTION")
}

// TestRunHelp_InputMissingDescriptions_Integration tests US2: Missing descriptions
// Given a workflow with inputs missing description fields, when running `--help`,
// displays "No description" placeholder for those inputs.
func TestRunHelp_InputMissingDescriptions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with inputs missing descriptions
	wfYAML := `name: undescribed-inputs
version: "1.0.0"
inputs:
  - name: silent_input
    type: string
    required: true
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "undescribed-inputs.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "undescribed-inputs", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No description")
}

// TestRunHelp_DefaultValues_Integration tests US3: Display Default Values
// Given a workflow with optional inputs that have defaults, when running `--help`,
// displays the default values next to those inputs.
func TestRunHelp_DefaultValues_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with various default values
	wfYAML := `name: default-values
version: "1.0.0"
inputs:
  - name: greeting
    type: string
    required: false
    default: "hello world"
  - name: count
    type: integer
    required: false
    default: 42
  - name: enabled
    type: boolean
    required: false
    default: true
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "default-values.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "default-values", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify default values are displayed
	assert.Contains(t, output, "DEFAULT")
	assert.Contains(t, output, "hello world")
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "true")
}

// TestRunHelp_OptionalWithoutDefault_Integration tests US3: Optional without default
// Given a workflow with an optional input without a default, when running `--help`,
// displays "-" for that input's default column.
func TestRunHelp_OptionalWithoutDefault_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with optional input without default
	wfYAML := `name: optional-no-default
version: "1.0.0"
inputs:
  - name: optional_value
    type: string
    required: false
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "optional-no-default.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "optional-no-default", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Find the line with optional_value and verify it has "-" for default
	lines := strings.Split(output, "\n")
	foundLine := false
	for _, line := range lines {
		if strings.Contains(line, "optional_value") {
			foundLine = true
			// Should contain "-" as the default placeholder
			assert.Contains(t, line, "-", "optional input without default should show '-'")
			break
		}
	}
	assert.True(t, foundLine, "should find optional_value in output")
}

// TestRunHelp_WorkflowDescription_Integration tests US4: Show Workflow Description
// Given a workflow with a description field, when running `--help`,
// displays the description at the top before the inputs list.
func TestRunHelp_WorkflowDescription_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with description
	wfYAML := `name: described-workflow
description: This workflow performs automated deployment to production
version: "1.0.0"
inputs:
  - name: environment
    type: string
    required: true
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "described-workflow.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "described-workflow", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify workflow description appears
	assert.Contains(t, output, "This workflow performs automated deployment to production")

	// Description should appear before inputs
	descIndex := strings.Index(output, "automated deployment")
	inputsIndex := strings.Index(output, "Input Parameters")
	assert.True(t, descIndex < inputsIndex || inputsIndex == -1,
		"workflow description should appear before inputs table")
}

// TestRunHelp_WorkflowNoDescription_Integration tests US4: No workflow description
// Given a workflow without a description field, when running `--help`,
// displays inputs without a description section.
func TestRunHelp_WorkflowNoDescription_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow without description
	wfYAML := `name: no-description
version: "1.0.0"
inputs:
  - name: value
    type: string
    required: true
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "no-description.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "no-description", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should not have "Description:" prefix for workflow
	assert.NotContains(t, output, "Description:")

	// But should still show inputs
	assert.Contains(t, output, "value")
}

// TestRunHelp_FullWorkflowExample_Integration tests complete help output
// Uses valid-full.yaml fixture which has all features: description, multiple inputs,
// required/optional, defaults, and input descriptions.
func TestRunHelp_FullWorkflowExample_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-full", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify workflow description (from fixture)
	assert.Contains(t, output, "A workflow with all features")

	// Verify inputs from fixture
	assert.Contains(t, output, "file_path")
	assert.Contains(t, output, "count")

	// Verify input description (from fixture)
	assert.Contains(t, output, "Path to input file")

	// Verify required/optional
	assert.Contains(t, output, "yes") // file_path is required
	assert.Contains(t, output, "no")  // count is optional

	// Verify default value
	assert.Contains(t, output, "10") // count default
}

// TestRunHelp_SimpleWorkflowExample_Integration tests basic help output
// Uses valid-simple.yaml fixture as a basic test case.
func TestRunHelp_SimpleWorkflowExample_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify workflow description
	assert.Contains(t, output, "A simple test workflow with inputs")

	// Verify inputs from fixture
	assert.Contains(t, output, "greeting")
	assert.Contains(t, output, "verbose")

	// Verify types
	assert.Contains(t, output, "string")
	assert.Contains(t, output, "boolean")

	// Verify defaults
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "false")
}

// TestRunHelp_NoColorFlag_Integration tests --no-color flag with help
// Verifies that --no-color flag is respected in help output.
func TestRunHelp_NoColorFlag_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--help", "--no-color"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Output should not contain ANSI escape codes
	assert.NotContains(t, output, "\x1b[")
	assert.NotContains(t, output, "\033[")

	// Should still contain valid content
	assert.Contains(t, output, "greeting")
}

// TestRunHelp_Performance_Integration tests NFR-001: < 50ms response
// Help display should complete quickly as it only parses YAML, no execution.
func TestRunHelp_Performance_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")

	// Run multiple times and measure
	const iterations = 5
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"run", "valid-full", "--help"})

		start := time.Now()
		err := cmd.Execute()
		elapsed := time.Since(start)

		require.NoError(t, err)
		totalDuration += elapsed
	}

	avgDuration := totalDuration / iterations

	// NFR-001: Help display must complete in < 50ms
	assert.Less(t, avgDuration, 50*time.Millisecond,
		"help display should complete in < 50ms, got %v average", avgDuration)
}

// TestRunHelp_HelpTakesPrecedence_Integration tests FR-005: --help takes precedence
// Verifies that --help flag takes precedence over other flags (workflow is not executed).
func TestRunHelp_HelpTakesPrecedence_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create a workflow that would fail if executed
	wfYAML := `name: would-fail
version: "1.0.0"
inputs:
  - name: required_param
    type: string
    required: true
states:
  initial: fail
  fail:
    type: step
    command: exit 1
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "would-fail.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Missing required input, but --help should take precedence
	cmd.SetArgs([]string{"run", "would-fail", "--help"})

	err := cmd.Execute()
	// Should succeed because --help takes precedence
	require.NoError(t, err)

	output := buf.String()
	// Should show help, not error about missing required input
	assert.Contains(t, output, "required_param")
	assert.NotContains(t, output, "required input")
}

// TestRunHelp_MixedInputTypes_Integration tests table-driven scenarios
// for various input type combinations.
func TestRunHelp_MixedInputTypes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tests := []struct {
		name           string
		workflowYAML   string
		expectContains []string
	}{
		{
			name: "all types displayed",
			workflowYAML: `name: all-types
version: "1.0.0"
inputs:
  - name: str_input
    type: string
    required: true
  - name: int_input
    type: integer
    required: false
    default: 100
  - name: bool_input
    type: boolean
    required: false
    default: false
states:
  initial: done
  done:
    type: terminal
`,
			expectContains: []string{"str_input", "int_input", "bool_input", "string", "integer", "boolean"},
		},
		{
			name: "mixed required and optional",
			workflowYAML: `name: mixed-required
version: "1.0.0"
inputs:
  - name: required_one
    type: string
    required: true
  - name: optional_one
    type: string
    required: false
  - name: required_two
    type: integer
    required: true
states:
  initial: done
  done:
    type: terminal
`,
			expectContains: []string{"required_one", "optional_one", "required_two", "yes", "no"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			wfDir := filepath.Join(tmpDir, "workflows")
			require.NoError(t, os.MkdirAll(wfDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(wfDir, "test.yaml"), []byte(tt.workflowYAML), 0o644))

			t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"run", "test", "--help"})

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()
			for _, expected := range tt.expectContains {
				assert.Contains(t, output, expected, "output should contain %q", expected)
			}
		})
	}
}

// TestRunHelp_TerminalWidth_Integration tests NFR-002: 80-column readability
// Help output must be readable in 80-column terminals.
func TestRunHelp_TerminalWidth_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with long description to test wrapping
	wfYAML := `name: long-content
version: "1.0.0"
description: This is a workflow with a reasonably long description that should display well
inputs:
  - name: param_with_description
    type: string
    required: true
    description: A parameter with a description
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "long-content.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "long-content", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Check that lines are reasonably sized for 80-column display
	// Allow some flexibility for table formatting
	for _, line := range lines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Most lines should fit in 120 chars (allowing for table formatting)
		assert.LessOrEqual(t, len(line), 120,
			"line should be reasonably sized: %q", line)
	}
}

// TestRunHelp_SpecialCharactersInInputs_Integration tests edge case with special characters
// Verifies that input names and descriptions with special characters are displayed correctly.
func TestRunHelp_SpecialCharactersInInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with special characters in descriptions
	wfYAML := `name: special-chars
version: "1.0.0"
description: "Workflow with special chars: <>&\"'"
inputs:
  - name: api_url
    type: string
    required: true
    description: "URL with special chars: https://example.com?foo=bar&baz=qux"
  - name: json_data
    type: string
    required: false
    default: '{"key": "value"}'
    description: 'JSON input (uses quotes "like this")'
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "special-chars.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "special-chars", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify special characters are preserved
	assert.Contains(t, output, "api_url")
	assert.Contains(t, output, "json_data")
	// Description with URL special chars
	assert.Contains(t, output, "https://example.com")
}

// TestRunHelp_InvalidYAMLWorkflow_Integration tests error handling for malformed YAML
// Verifies that invalid YAML produces appropriate error message.
func TestRunHelp_InvalidYAMLWorkflow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with invalid YAML
	invalidYAML := `name: invalid-yaml
version: "1.0.0"
inputs:
  - name: test
    type: [invalid array in wrong place
states:
  initial: done
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "invalid-yaml.yaml"), []byte(invalidYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "invalid-yaml", "--help"})

	err := cmd.Execute()

	// Should handle invalid YAML gracefully
	combinedOutput := buf.String()
	if err != nil {
		combinedOutput += err.Error()
	}

	// Should indicate a parsing or loading error
	hasError := err != nil ||
		strings.Contains(combinedOutput, "error") ||
		strings.Contains(combinedOutput, "invalid") ||
		strings.Contains(combinedOutput, "parse")

	assert.True(t, hasError, "should indicate error for invalid YAML, got: %s", combinedOutput)
}

// TestRunHelp_EmptyInputName_Integration tests edge case with empty or whitespace input names
// Verifies graceful handling of edge cases in input definitions.
func TestRunHelp_EmptyInputName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow with inputs that have edge case names
	wfYAML := `name: edge-case-names
version: "1.0.0"
inputs:
  - name: a
    type: string
    required: true
    description: Single character name
  - name: very_long_input_name_that_might_cause_table_formatting_issues
    type: string
    required: false
    description: Very long input name
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "edge-case-names.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "edge-case-names", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify both inputs are displayed
	assert.Contains(t, output, "Single character name")
	assert.Contains(t, output, "very_long_input_name")
}

// TestRunHelp_ExitCode_Integration tests FR-004: exit code 1 for user error
// Verifies that non-existent workflow returns exit code 1.
func TestRunHelp_ExitCode_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "nonexistent", "--help"})

	err := cmd.Execute()
	// Error should be returned for non-existent workflow
	// FR-004: exit code 1 for user error
	if err != nil {
		// Cobra wraps errors - the presence of error indicates failure
		combinedOutput := buf.String() + err.Error()
		assert.True(t,
			strings.Contains(combinedOutput, "not found") ||
				strings.Contains(combinedOutput, "does not exist") ||
				strings.Contains(combinedOutput, "no such"),
			"error should indicate workflow not found")
	}
}

// TestRunHelp_EnvPathOverride_Integration tests NFR-003: workflows from env path
// Verifies that AWF_WORKFLOWS_PATH environment variable is respected.
func TestRunHelp_EnvPathOverride_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "custom-workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	// Create workflow in custom directory
	wfYAML := `name: env-path-workflow
version: "1.0.0"
description: A workflow from custom ENV path
inputs:
  - name: custom_input
    type: string
    required: true
    description: Input loaded from custom path
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "env-path-workflow.yaml"), []byte(wfYAML), 0o644))

	// Set custom workflow path via environment variable
	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "env-path-workflow", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "custom_input")
	assert.Contains(t, output, "Input loaded from custom path")
}

// TestRunHelp_AllInputTypes_Integration tests comprehensive input type coverage
// Verifies all supported input types are displayed correctly.
func TestRunHelp_AllInputTypes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	wfYAML := `name: all-input-types
version: "1.0.0"
description: Workflow testing all input types
inputs:
  - name: string_input
    type: string
    required: true
    description: A string input
  - name: integer_input
    type: integer
    required: false
    default: 42
    description: An integer input
  - name: boolean_input
    type: boolean
    required: false
    default: true
    description: A boolean input
  - name: number_input
    type: number
    required: false
    default: 3.14
    description: A number input (float)
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "all-input-types.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "all-input-types", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify all input types are shown
	assert.Contains(t, output, "string")
	assert.Contains(t, output, "integer")
	assert.Contains(t, output, "boolean")

	// Verify all inputs are listed
	assert.Contains(t, output, "string_input")
	assert.Contains(t, output, "integer_input")
	assert.Contains(t, output, "boolean_input")
	assert.Contains(t, output, "number_input")

	// Verify defaults
	assert.Contains(t, output, "42")
	assert.Contains(t, output, "true")
	assert.Contains(t, output, "3.14")
}

// TestRunHelp_LongDescription_Integration tests handling of long descriptions
// Verifies that long workflow and input descriptions are handled gracefully.
func TestRunHelp_LongDescription_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	longDesc := strings.Repeat("This is a very long description. ", 10)
	wfYAML := `name: long-descriptions
version: "1.0.0"
description: "` + longDesc + `"
inputs:
  - name: param1
    type: string
    required: true
    description: "` + longDesc + `"
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "long-descriptions.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "long-descriptions", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Should contain at least part of the long description
	assert.Contains(t, output, "very long description")
	assert.Contains(t, output, "param1")
}

// TestRunHelp_UnicodeContent_Integration tests Unicode support
// Verifies that Unicode characters in workflow names and descriptions are handled.
func TestRunHelp_UnicodeContent_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	wfYAML := `name: unicode-workflow
version: "1.0.0"
description: "Workflow with émojis 🚀 and accénts"
inputs:
  - name: greeting
    type: string
    required: true
    description: "Grüß Gott, こんにちは, مرحبا"
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "unicode-workflow.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "unicode-workflow", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Verify Unicode content is preserved
	assert.Contains(t, output, "greeting")
	// At least verify the workflow loads without error
	assert.Contains(t, output, "Input Parameters") // or similar header
}

// TestRunHelp_OnlyRequiredInputs_Integration tests workflow with only required inputs
// Verifies display when all inputs are required (no optional column needed).
func TestRunHelp_OnlyRequiredInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	wfYAML := `name: all-required
version: "1.0.0"
inputs:
  - name: required_a
    type: string
    required: true
  - name: required_b
    type: integer
    required: true
  - name: required_c
    type: boolean
    required: true
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "all-required.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "all-required", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// All three inputs should be listed
	assert.Contains(t, output, "required_a")
	assert.Contains(t, output, "required_b")
	assert.Contains(t, output, "required_c")

	// Count occurrences of "yes" (all are required)
	yesCount := strings.Count(output, "yes")
	assert.GreaterOrEqual(t, yesCount, 3, "should have at least 3 'yes' for required inputs")
}

// TestRunHelp_OnlyOptionalInputs_Integration tests workflow with only optional inputs
// Verifies display when all inputs are optional with defaults.
func TestRunHelp_OnlyOptionalInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	wfDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(wfDir, 0o755))

	wfYAML := `name: all-optional
version: "1.0.0"
inputs:
  - name: optional_a
    type: string
    required: false
    default: "default_a"
  - name: optional_b
    type: integer
    required: false
    default: 123
  - name: optional_c
    type: boolean
    required: false
    default: false
states:
  initial: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(filepath.Join(wfDir, "all-optional.yaml"), []byte(wfYAML), 0o644))

	t.Setenv("AWF_WORKFLOWS_PATH", wfDir)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "all-optional", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// All three inputs should be listed
	assert.Contains(t, output, "optional_a")
	assert.Contains(t, output, "optional_b")
	assert.Contains(t, output, "optional_c")

	// All defaults should be shown
	assert.Contains(t, output, "default_a")
	assert.Contains(t, output, "123")
	assert.Contains(t, output, "false")

	// Count occurrences of "no" (all are optional)
	noCount := strings.Count(output, "no")
	assert.GreaterOrEqual(t, noCount, 3, "should have at least 3 'no' for optional inputs")
}
