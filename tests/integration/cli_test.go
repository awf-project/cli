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
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "input-test.yaml"), []byte(wfYAML), 0644)

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
