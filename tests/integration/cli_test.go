//go:build integration

package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestCLI_List_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Use fixtures directory
	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "valid-simple")
}

func TestCLI_Validate_Valid_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "valid-simple"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "valid")
}

func TestCLI_Validate_Invalid_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "invalid-missing-name"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestCLI_Run_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create temp directory for state storage
	tmpDir := t.TempDir()
	statesDir := filepath.Join(tmpDir, "states")

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "valid-simple", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "Workflow ID:")

	// Verify state was saved
	files, _ := os.ReadDir(statesDir)
	assert.NotEmpty(t, files, "expected state file to be created")
}

func TestCLI_Status_NotFound_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "nonexistent-id", "--storage", tmpDir})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCLI_Version_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "awf version")
}

func TestCLI_Help_Integration(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "AI Workflow CLI")
	assert.Contains(t, output, "run")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "status")
	assert.Contains(t, output, "validate")
}

func TestCLI_GlobalFlags_Integration(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"verbose", []string{"--verbose", "version"}},
		{"quiet", []string{"--quiet", "version"}},
		{"no-color", []string{"--no-color", "version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			require.NoError(t, err)
		})
	}
}

func TestCLI_Run_WithInputs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a workflow that uses inputs
	wfYAML := `name: input-test
version: "1.0.0"
inputs:
  - name: message
    type: string
    required: false
    default: "default message"
states:
  initial: echo
  echo:
    type: step
    command: echo "received input"
    on_success: done
  done:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "input-test.yaml"), []byte(wfYAML), 0o644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "input-test", "--input", "message=hello", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "completed")
}

func TestCLI_ExitCodes_Integration(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectError  bool
		errorContain string
	}{
		{
			name:        "success - version",
			args:        []string{"version"},
			expectError: false,
		},
		{
			name:         "user error - missing workflow",
			args:         []string{"run", "nonexistent"},
			expectError:  true,
			errorContain: "not found",
		},
		{
			name:         "user error - no args to run",
			args:         []string{"run"},
			expectError:  true,
			errorContain: "accepts 1 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContain != "" {
					assert.True(t, strings.Contains(err.Error(), tt.errorContain) ||
						strings.Contains(buf.String(), tt.errorContain),
						"expected error to contain %q", tt.errorContain)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCLI_Run_FailingCommand_Integration tests workflow with a failing command
func TestCLI_Run_FailingCommand_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a workflow with a failing command
	wfYAML := `name: failing-test
version: "1.0.0"
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
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "failing-test.yaml"), []byte(wfYAML), 0o644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "failing-test", "--storage", tmpDir})

	err := cmd.Execute()
	// Workflow should complete (reached terminal state) even though command failed
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "error") // Should end in error state
}

// TestCLI_Validate_InvalidStrategy_Integration tests validation of invalid parallel strategy
func TestCLI_Validate_InvalidStrategy_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a workflow with invalid parallel strategy
	// Note: YAML uses "parallel" field, not "branches"
	wfYAML := `name: invalid-strategy
version: "1.0.0"
states:
  initial: parallel_step
  parallel_step:
    type: parallel
    parallel:
      - step1
      - step2
    strategy: invalid_strategy_value
    on_success: done
  step1:
    type: step
    command: echo step1
    on_success: done
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "invalid-strategy.yaml"), []byte(wfYAML), 0o644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"validate", "invalid-strategy"})

	err := cmd.Execute()
	assert.Error(t, err, "should fail validation with invalid strategy")
	assert.Contains(t, err.Error(), "invalid parallel strategy")
}

// TestCLI_Run_OutputModes_Integration tests different output modes
func TestCLI_Run_OutputModes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a simple workflow
	wfYAML := `name: output-test
version: "1.0.0"
states:
  initial: echo
  echo:
    type: step
    command: echo "test output"
    on_success: done
  done:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "output-test.yaml"), []byte(wfYAML), 0o644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tests := []struct {
		name          string
		args          []string
		expectOutput  string
		expectMissing string
	}{
		{
			name:         "default output",
			args:         []string{"run", "output-test", "--storage", tmpDir},
			expectOutput: "completed",
		},
		{
			name:          "quiet mode",
			args:          []string{"run", "output-test", "--storage", tmpDir, "--quiet"},
			expectMissing: "Step:",
		},
		{
			name:         "verbose mode",
			args:         []string{"run", "output-test", "--storage", tmpDir, "--verbose"},
			expectOutput: "completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use unique storage per test to avoid conflicts
			testDir := filepath.Join(tmpDir, tt.name)
			os.MkdirAll(testDir, 0o755)

			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			// Replace storage path in args
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if arg == tmpDir {
					args[i] = testDir
				}
			}
			cmd.SetArgs(args)

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()
			if tt.expectOutput != "" {
				assert.Contains(t, output, tt.expectOutput)
			}
			if tt.expectMissing != "" {
				assert.NotContains(t, output, tt.expectMissing)
			}
		})
	}
}

// TestCLI_Run_MultiStep_Integration tests a workflow with multiple steps
func TestCLI_Run_MultiStep_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a multi-step workflow
	wfYAML := `name: multi-step
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "step 1"
    on_success: step2
    on_failure: error
  step2:
    type: step
    command: echo "step 2"
    on_success: step3
    on_failure: error
  step3:
    type: step
    command: echo "step 3"
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "multi-step.yaml"), []byte(wfYAML), 0o644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"run", "multi-step", "--storage", tmpDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "done") // Should reach 'done' terminal state
}

// TestCLI_Run_StepSuccessFeedback_Integration tests F037 success feedback for silent steps
func TestCLI_Run_StepSuccessFeedback_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a workflow with a step that produces no output
	wfYAML := `name: silent-step
version: "1.0.0"
states:
  initial: silent
  silent:
    type: step
    command: "true"
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0o755)
	os.WriteFile(filepath.Join(wfDir, "silent-step.yaml"), []byte(wfYAML), 0o644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	t.Run("shows success message for silent step", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "default")
		os.MkdirAll(testDir, 0o755)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"run", "silent-step", "--storage", testDir})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "silent: completed successfully", "should show success feedback for step with no output")
		assert.Contains(t, output, "\u2713", "should contain checkmark")
	})

	t.Run("quiet mode hides success message", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "quiet")
		os.MkdirAll(testDir, 0o755)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"run", "silent-step", "--storage", testDir, "--quiet"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Quiet mode should hide step success feedback, not the workflow completion message
		assert.NotContains(t, output, "silent: completed successfully", "quiet mode should hide step success feedback")
	})

	t.Run("step with output does not show extra success message", func(t *testing.T) {
		// Create a workflow with a step that has output
		wfYAMLWithOutput := `name: output-step
version: "1.0.0"
states:
  initial: echo
  echo:
    type: step
    command: echo "hello"
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
`
		os.WriteFile(filepath.Join(wfDir, "output-step.yaml"), []byte(wfYAMLWithOutput), 0o644)

		testDir := filepath.Join(tmpDir, "output")
		os.MkdirAll(testDir, 0o755)

		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		// Use buffered mode to see step output
		cmd.SetArgs([]string{"run", "output-step", "--storage", testDir, "--output", "buffered"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		// Should not show success feedback for step that had output
		assert.NotContains(t, output, "echo: completed successfully", "step with output should not show extra success message")
	})
}
