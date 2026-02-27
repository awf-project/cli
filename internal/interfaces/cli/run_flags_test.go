package cli_test

// C015 T007: Flag parsing tests extracted from run_test.go
// Thread-safety: All tests use thread-safe patterns (no os.Chdir)
// Tests: 25 functions, ~900 lines
// Scope: Command structure, flag parsing, help output, workflow help (F035)

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow name provided")
	}
}

func TestRunCommand_WorkflowNotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestRunCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'run' subcommand")
	}
}

func TestRunCommand_HasInputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("input")
			if flag == nil {
				t.Error("expected 'run' command to have --input flag")
			}
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Execute a workflow") {
		t.Errorf("expected help text, got: %s", output)
	}
}

func TestRunCommand_HasOutputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("output")
			if flag == nil {
				t.Error("expected 'run' command to have --output flag")
			}
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_InvalidOutputMode(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "test-workflow", "--output=invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid output mode")
	}
	if !strings.Contains(err.Error(), "invalid output mode") {
		t.Errorf("expected 'invalid output mode' error, got: %v", err)
	}
}

func TestRunCommand_HasStepFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("step")
			require.NotNil(t, flag, "expected 'run' command to have --step flag")
			assert.Equal(t, "s", flag.Shorthand, "step flag should have -s shorthand")
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_HasMockFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("mock")
			require.NotNil(t, flag, "expected 'run' command to have --mock flag")
			assert.Equal(t, "m", flag.Shorthand, "mock flag should have -m shorthand")
			return
		}
	}

	t.Error("run command not found")
}

func TestRunCommand_StepFlagInHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--step")
	assert.Contains(t, output, "Execute only this step")
}

func TestRunCommand_MockFlagInHelp(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "--mock")
	assert.Contains(t, output, "Mock state value")
}

// TestRunCommand_HasDryRunFlag tests that --dry-run flag is recognized
func TestRunCommand_HasDryRunFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("dry-run")
			require.NotNil(t, flag, "expected 'run' command to have --dry-run flag")
			assert.Equal(t, "false", flag.DefValue, "dry-run should default to false")
			return
		}
	}

	t.Error("run command not found")
}

// TestRunCommand_HasInteractiveFlag tests that --interactive flag is recognized
func TestRunCommand_HasInteractiveFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("interactive")
			require.NotNil(t, flag, "expected 'run' command to have --interactive flag")
			assert.Equal(t, "false", flag.DefValue, "interactive should default to false")
			return
		}
	}

	t.Error("run command not found")
}

// TestRunCommand_HasBreakpointFlag tests that --breakpoint flag is recognized
func TestRunCommand_HasBreakpointFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "run" {
			flag := sub.Flags().Lookup("breakpoint")
			require.NotNil(t, flag, "expected 'run' command to have --breakpoint flag")
			return
		}
	}

	t.Error("run command not found")
}

// TestRunCommand_WorkflowsFlag_EmptyDirectory tests run command behavior with
// an empty workflow directory using thread-safe patterns.
// This test uses setupTestDir(t) which internally uses t.TempDir() and t.Setenv().
func TestRunCommand_WorkflowsFlag_EmptyDirectory(t *testing.T) {
	// Uses thread-safe pattern: setupTestDir creates isolated test environment
	tmpDir := setupTestDir(t)

	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Request a workflow that doesn't exist in the empty test directory
	cmd.SetArgs([]string{"run", "nonexistent-workflow"})

	err := cmd.Execute()

	// Should fail because no workflow exists
	require.Error(t, err)

	// Verify the error relates to workflow not found (not a panic or unrelated error)
	// The exact error message may vary, but it should indicate the workflow wasn't found
	errStr := err.Error()
	assert.True(t,
		strings.Contains(errStr, "workflow") || strings.Contains(errStr, "not found") || strings.Contains(errStr, "load"),
		"expected workflow-related error, got: %s (tmpDir: %s)", errStr, tmpDir)
}

func TestParseMockFlags(t *testing.T) {
	tests := []struct {
		name        string
		flags       []string
		want        map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name:  "empty flags returns empty map",
			flags: []string{},
			want:  map[string]string{},
		},
		{
			name:  "single mock flag",
			flags: []string{"states.fetch.output=hello"},
			want:  map[string]string{"states.fetch.output": "hello"},
		},
		{
			name:  "multiple mock flags",
			flags: []string{"states.fetch.output=data", "states.process.output=result"},
			want: map[string]string{
				"states.fetch.output":   "data",
				"states.process.output": "result",
			},
		},
		{
			name:  "mock flag with value containing equals sign",
			flags: []string{"states.step.output=key=value"},
			want:  map[string]string{"states.step.output": "key=value"},
		},
		{
			name:  "mock flag with empty value",
			flags: []string{"states.step.output="},
			want:  map[string]string{"states.step.output": ""},
		},
		{
			name:  "mock flag with json value",
			flags: []string{`states.api.output={"key":"value"}`},
			want:  map[string]string{"states.api.output": `{"key":"value"}`},
		},
		{
			name:        "invalid format without equals",
			flags:       []string{"states.step.output"},
			wantErr:     true,
			errContains: "invalid mock format",
		},
		{
			name:        "empty key",
			flags:       []string{"=value"},
			wantErr:     true,
			errContains: "mock key cannot be empty",
		},
		{
			name:  "whitespace trimmed from key and value",
			flags: []string{"  states.step.output  =  value  "},
			want:  map[string]string{"states.step.output": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cli.ParseMockFlags(tt.flags)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestRunCommand_WorkflowHelp_WithInputs tests that `awf run <workflow> --help`
// displays workflow-specific help including input parameters (US1)
func TestRunCommand_WorkflowHelp_WithInputs(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with multiple inputs of varying types

	workflowContent := `name: commit
version: "1.0.0"
description: Create a git commit with the specified message
inputs:
  - name: branch
    type: string
    required: true
    description: The branch to commit to
  - name: message
    type: string
    required: false
    default: "Auto-commit"
    description: The commit message
  - name: verbose
    type: boolean
    required: false
    default: false
states:
  initial: commit
  commit:
    type: step
    command: git commit -m "{{inputs.message}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "commit.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "commit", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show workflow description (US4)
	assert.Contains(t, output, "Create a git commit", "should display workflow description")

	// Should show Input Parameters header
	assert.Contains(t, output, "Input Parameters", "should display input parameters header")

	// Should show each input name
	assert.Contains(t, output, "branch", "should display 'branch' input")
	assert.Contains(t, output, "message", "should display 'message' input")
	assert.Contains(t, output, "verbose", "should display 'verbose' input")

	// Should show input types (FR-002)
	assert.Contains(t, output, "string", "should display string type")
	assert.Contains(t, output, "boolean", "should display boolean type")

	// Should show required/optional status (FR-002)
	assert.Contains(t, output, "yes", "should indicate required inputs")
	assert.Contains(t, output, "no", "should indicate optional inputs")

	// Should show descriptions (US2)
	assert.Contains(t, output, "The branch to commit to", "should display input description")
	assert.Contains(t, output, "The commit message", "should display input description")

	// Should show default values (US3)
	assert.Contains(t, output, "Auto-commit", "should display default value")

	// Should also include standard command flags (FR-003)
	assert.Contains(t, output, "--input", "should include standard --input flag")
}

// TestRunCommand_WorkflowHelp_NoInputs tests that `awf run <workflow> --help`
// shows appropriate message when workflow has no inputs (US1)
func TestRunCommand_WorkflowHelp_NoInputs(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with no inputs

	workflowContent := `name: deploy
version: "1.0.0"
states:
  initial: deploy
  deploy:
    type: step
    command: echo "deploying..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "deploy.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "deploy", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show "No input parameters" message
	assert.Contains(t, output, "No input parameters", "should indicate no inputs defined")

	// Should still include standard command flags
	assert.Contains(t, output, "--input", "should include standard --input flag")
}

// TestRunCommand_WorkflowHelp_WorkflowNotFound tests error handling for non-existent workflow (US1)
func TestRunCommand_WorkflowHelp_WorkflowNotFound(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create .awf directory but no workflow

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "unknown-workflow", "--help"})

	err := cmd.Execute()
	// Help should not return an error, but display error message in output
	require.NoError(t, err)

	// Should show error message about workflow not found
	combinedOutput := out.String() + errOut.String()
	assert.Contains(t, combinedOutput, "not found", "should display workflow not found error")
	assert.Contains(t, combinedOutput, "unknown-workflow", "should mention the workflow name")

	// Should fall back to default help after error
	assert.Contains(t, out.String(), "Execute a workflow", "should show default help text")
}

// TestRunCommand_WorkflowHelp_WithDescription tests workflow description display (US4)
func TestRunCommand_WorkflowHelp_WithDescription(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with a description

	workflowContent := `name: analyze
version: "1.0.0"
description: Analyze code quality and generate a comprehensive report
inputs:
  - name: target
    type: string
    required: true
states:
  initial: analyze
  analyze:
    type: step
    command: echo "analyzing..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "analyze.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "analyze", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Description should appear at the top (before inputs)
	assert.Contains(t, output, "Description:", "should have description label")
	assert.Contains(t, output, "Analyze code quality", "should display workflow description")

	// Description should appear before Input Parameters
	descIdx := strings.Index(output, "Analyze code quality")
	inputIdx := strings.Index(output, "Input Parameters")
	assert.Less(t, descIdx, inputIdx, "description should appear before input parameters")
}

// TestRunCommand_WorkflowHelp_WithoutDescription tests that help works without description (US4)
func TestRunCommand_WorkflowHelp_WithoutDescription(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow without a description

	workflowContent := `name: simple
version: "1.0.0"
inputs:
  - name: file
    type: string
    required: true
states:
  initial: process
  process:
    type: step
    command: echo "processing..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "simple.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "simple", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should NOT have "Description:" label when no description
	assert.NotContains(t, output, "Description:", "should not show description label when no description")

	// Should still show inputs
	assert.Contains(t, output, "file", "should display input name")
	assert.Contains(t, output, "Input Parameters", "should show input parameters section")
}

// TestRunCommand_WorkflowHelp_InputWithoutDescription tests "No description" placeholder (US2)
func TestRunCommand_WorkflowHelp_InputWithoutDescription(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with inputs that have no descriptions

	workflowContent := `name: nodesc
version: "1.0.0"
inputs:
  - name: param1
    type: string
    required: true
  - name: param2
    type: string
    required: false
    description: This one has a description
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "nodesc.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "nodesc", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show "No description" placeholder for inputs without description
	assert.Contains(t, output, "No description", "should show 'No description' placeholder")

	// Should still show the actual description for the input that has one
	assert.Contains(t, output, "This one has a description", "should display input with description")
}

// TestRunCommand_WorkflowHelp_DefaultValues tests default value display (US3)
func TestRunCommand_WorkflowHelp_DefaultValues(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with various default values

	workflowContent := `name: defaults
version: "1.0.0"
inputs:
  - name: timeout
    type: integer
    required: false
    default: 30
    description: Timeout in seconds
  - name: format
    type: string
    required: false
    default: json
    description: Output format
  - name: optional_no_default
    type: string
    required: false
    description: Optional without default
  - name: required_input
    type: string
    required: true
    description: Required input
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "defaults.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "defaults", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show default values
	assert.Contains(t, output, "30", "should display numeric default value")
	assert.Contains(t, output, "json", "should display string default value")

	// Should show "-" or empty for inputs without defaults
	// The input "optional_no_default" should have "-" in the DEFAULT column
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "optional_no_default") {
			assert.Contains(t, line, "-", "should show '-' for optional input without default")
			break
		}
	}
}

// TestRunCommand_WorkflowHelp_NoWorkflowArg tests that default help is shown without workflow arg
func TestRunCommand_WorkflowHelp_NoWorkflowArg(t *testing.T) {
	cmd := cli.NewRootCommand()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"run", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should show default Cobra help for run command (FR-003)
	assert.Contains(t, output, "Execute a workflow", "should show run command description")
	assert.Contains(t, output, "--input", "should show --input flag")
	assert.Contains(t, output, "--output", "should show --output flag")
	assert.Contains(t, output, "--step", "should show --step flag")
	assert.Contains(t, output, "--dry-run", "should show --dry-run flag")
}

// TestRunCommand_WorkflowHelp_HelpTakesPrecedence tests that --help prevents execution (FR-005)
func TestRunCommand_WorkflowHelp_HelpTakesPrecedence(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow that would fail if executed

	workflowContent := `name: would-fail
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
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "would-fail.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Note: Missing required input and using a failing command, but --help should prevent execution
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "would-fail", "--help"})

	err := cmd.Execute()
	// Should not error - help takes precedence
	require.NoError(t, err)

	output := out.String()

	// Should show help content, not execution error
	assert.Contains(t, output, "required_param", "should display input in help")
	assert.NotContains(t, output, "exit 1", "should not show command output")
}

// TestRunCommand_WorkflowHelp_TableFormat tests help output format is aligned (NFR-002)
func TestRunCommand_WorkflowHelp_TableFormat(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with inputs

	workflowContent := `name: formatted
version: "1.0.0"
inputs:
  - name: short
    type: string
    required: true
    description: Short
  - name: very_long_parameter_name
    type: boolean
    required: false
    description: A very long description that should still be readable
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "formatted.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "formatted", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should have table header with column names
	assert.Contains(t, output, "NAME", "should have NAME column header")
	assert.Contains(t, output, "TYPE", "should have TYPE column header")
	assert.Contains(t, output, "REQUIRED", "should have REQUIRED column header")
	assert.Contains(t, output, "DEFAULT", "should have DEFAULT column header")
	assert.Contains(t, output, "DESCRIPTION", "should have DESCRIPTION column header")
}

// TestRunCommand_WorkflowHelp_AllInputTypes tests different input types display correctly
func TestRunCommand_WorkflowHelp_AllInputTypes(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow with all input types

	workflowContent := `name: alltypes
version: "1.0.0"
inputs:
  - name: string_input
    type: string
    required: true
  - name: integer_input
    type: integer
    required: false
    default: 42
  - name: boolean_input
    type: boolean
    required: false
    default: true
  - name: number_input
    type: number
    required: false
    default: 3.14
states:
  initial: run
  run:
    type: step
    command: echo "running..."
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "alltypes.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "alltypes", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	// Should display all type names
	assert.Contains(t, output, "string", "should display string type")
	assert.Contains(t, output, "integer", "should display integer type")
	assert.Contains(t, output, "boolean", "should display boolean type")
	assert.Contains(t, output, "number", "should display number type")
}
