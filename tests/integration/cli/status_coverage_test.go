//go:build integration

// Component: T005
// Feature: C028
package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/internal/testutil"
)

// setupStatusTest creates a test directory with optional execution state
func setupStatusTest(t *testing.T, workflowID string, createState bool) (string, *workflow.ExecutionContext) {
	t.Helper()
	dir := testutil.SetupTestDir(t)

	if !createState {
		return dir, nil
	}

	// Create execution state for positive tests
	ctx := &workflow.ExecutionContext{
		WorkflowID:   workflowID,
		WorkflowName: "test-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		Inputs:       map[string]any{"test": "value"},
		States: map[string]workflow.StepState{
			"start": {
				Status:      workflow.StatusCompleted,
				Output:      "test output",
				Stderr:      "",
				ExitCode:    0,
				StartedAt:   time.Now().Add(-1 * time.Minute),
				CompletedAt: time.Now().Add(-30 * time.Second),
			},
		},
		Env:       map[string]string{},
		StartedAt: time.Now().Add(-2 * time.Minute),
		UpdatedAt: time.Now().Add(-30 * time.Second),
	}

	// Save state to storage
	// runStatus uses: cfg.StoragePath + "/states"
	// So if we pass --storage dir/.awf, it looks for dir/.awf/states
	storagePath := filepath.Join(dir, ".awf")
	stateStore := store.NewJSONStore(filepath.Join(storagePath, "states"))
	err := stateStore.Save(context.TODO(), ctx)
	require.NoError(t, err, "failed to create test state")

	return dir, ctx
}

// TestRunStatus_NotFound_TextError tests missing workflow execution with text error output
func TestRunStatus_NotFound_TextError(t *testing.T) {
	// Arrange: create test directory without execution state
	dir, _ := setupStatusTest(t, "nonexistent-id", false)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "nonexistent-id", "--storage", filepath.Join(dir, ".awf")})

	// Act: execute status command
	err := cmd.Execute()

	// Assert: command fails with not found error
	require.Error(t, err, "status should fail for nonexistent workflow")
	assert.Contains(t, err.Error(), "workflow execution not found", "error should indicate workflow not found")
	assert.Contains(t, err.Error(), "nonexistent-id", "error should include workflow ID")
}

// TestRunStatus_NotFound_JSONError tests missing workflow execution with JSON error output
func TestRunStatus_NotFound_JSONError(t *testing.T) {
	// Arrange: create test directory without execution state
	dir, _ := setupStatusTest(t, "nonexistent-id", false)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "nonexistent-id", "--format", "json", "--storage", filepath.Join(dir, ".awf")})

	// Act: execute status command
	err := cmd.Execute()

	// Assert: JSON format writes error via WriteError
	output := buf.String()

	// Parse JSON output
	if output != "" {
		var result map[string]interface{}
		jsonErr := json.Unmarshal([]byte(output), &result)
		require.NoError(t, jsonErr, "output should be valid JSON: %s", output)

		// Verify error is in JSON output (WriteError format: {code, error})
		assert.NotNil(t, result["error"], "JSON should contain error field")
		errorMsg, ok := result["error"].(string)
		require.True(t, ok, "error field should be string")
		assert.Contains(t, errorMsg, "workflow execution not found", "error message should indicate not found")
		assert.Contains(t, errorMsg, "nonexistent-id", "error message should include workflow ID")
	} else {
		// If no JSON output, command should have returned error
		require.Error(t, err, "status should fail for nonexistent workflow")
	}
}

// TestRunStatus_AllFormats tests successful status retrieval in all output formats
func TestRunStatus_AllFormats(t *testing.T) {
	// Test table: verify all output formats work correctly
	tests := []struct {
		name       string
		format     string
		expectJSON bool
	}{
		{
			name:       "text format",
			format:     "text",
			expectJSON: false,
		},
		{
			name:       "json format",
			format:     "json",
			expectJSON: true,
		},
		{
			name:       "quiet format",
			format:     "quiet",
			expectJSON: false,
		},
		{
			name:       "table format",
			format:     "table",
			expectJSON: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create execution state for test
			workflowID := "test-workflow-" + tt.format
			dir, execCtx := setupStatusTest(t, workflowID, true)

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			args := []string{"status", workflowID, "--storage", filepath.Join(dir, ".awf")}
			if tt.format != "text" {
				args = append(args, "--format", tt.format)
			}
			cmd.SetArgs(args)

			// Act: execute status command
			err := cmd.Execute()

			// Assert: command succeeds
			require.NoError(t, err, "status should succeed for existing workflow")
			output := buf.String()
			assert.NotEmpty(t, output, "output should not be empty")

			// Verify format-specific output
			if tt.expectJSON {
				// JSON format: parse and verify structure
				var result map[string]interface{}
				jsonErr := json.Unmarshal([]byte(output), &result)
				require.NoError(t, jsonErr, "output should be valid JSON: %s", output)

				// Verify JSON contains workflow information
				assert.Equal(t, workflowID, result["workflow_id"], "JSON should contain workflow ID")
				assert.Equal(t, execCtx.WorkflowName, result["workflow_name"], "JSON should contain workflow name")
				assert.NotNil(t, result["status"], "JSON should contain status")
			} else {
				// Quiet format should be minimal - just status
				if tt.format == "quiet" {
					// Quiet format outputs just the status
					lines := strings.Split(strings.TrimSpace(output), "\n")
					assert.LessOrEqual(t, len(lines), 3, "quiet format should be minimal")
					assert.Contains(t, output, string(execCtx.Status), "quiet output should show status")
				}

				// Text format should show full workflow ID
				if tt.format == "text" {
					assert.Contains(t, output, workflowID, "text output should contain full workflow ID")
					assert.Contains(t, output, string(execCtx.Status), "text output should show workflow status")
				}

				// Table format truncates long IDs, so check for workflow name instead
				if tt.format == "table" {
					assert.Contains(t, output, execCtx.WorkflowName, "table output should contain workflow name")
					assert.Contains(t, output, string(execCtx.Status), "table output should show workflow status")
				}
			}
		})
	}
}

// TestRunStatus_VerboseFlag tests verbose flag produces additional output
func TestRunStatus_VerboseFlag(t *testing.T) {
	// Arrange: create execution state with multiple steps
	workflowID := "test-workflow-verbose"
	dir, execCtx := setupStatusTest(t, workflowID, true)

	// Test without verbose flag
	t.Run("without verbose", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"status", workflowID, "--storage", filepath.Join(dir, ".awf")})

		err := cmd.Execute()
		require.NoError(t, err, "status should succeed")

		output := buf.String()
		assert.Contains(t, output, workflowID, "output should contain workflow ID")
		assert.Contains(t, output, string(execCtx.Status), "output should contain status")

		// Without verbose, output should be more concise
		normalLineCount := len(strings.Split(strings.TrimSpace(output), "\n"))

		// Store for comparison
		t.Setenv("NORMAL_LINE_COUNT", string(rune(normalLineCount)))
	})

	// Test with verbose flag
	t.Run("with verbose", func(t *testing.T) {
		cmd := cli.NewRootCommand()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"status", workflowID, "--storage", filepath.Join(dir, ".awf"), "--verbose"})

		err := cmd.Execute()
		require.NoError(t, err, "status should succeed")

		output := buf.String()
		assert.Contains(t, output, workflowID, "output should contain workflow ID")
		assert.Contains(t, output, string(execCtx.Status), "output should contain status")

		// With verbose, should show additional details
		// Based on status.go displayStatus function with verbose=true:
		// - Shows step details (states map)
		// - Shows timestamps
		// - Shows environment variables if present

		// Verify verbose shows step information
		assert.Contains(t, output, "start", "verbose output should show step names")

		// Verbose output should have more lines than normal
		verboseLineCount := len(strings.Split(strings.TrimSpace(output), "\n"))
		assert.Greater(t, verboseLineCount, 3, "verbose output should have multiple lines of detail")
	})

	// Compare outputs - verbose should have more information
	t.Run("verbose comparison", func(t *testing.T) {
		// Run both and compare line counts
		cmdNormal := cli.NewRootCommand()
		bufNormal := &bytes.Buffer{}
		cmdNormal.SetOut(bufNormal)
		cmdNormal.SetErr(bufNormal)
		cmdNormal.SetArgs([]string{"status", workflowID, "--storage", filepath.Join(dir, ".awf")})
		err := cmdNormal.Execute()
		require.NoError(t, err)

		cmdVerbose := cli.NewRootCommand()
		bufVerbose := &bytes.Buffer{}
		cmdVerbose.SetOut(bufVerbose)
		cmdVerbose.SetErr(bufVerbose)
		cmdVerbose.SetArgs([]string{"status", workflowID, "--storage", filepath.Join(dir, ".awf"), "--verbose"})
		err = cmdVerbose.Execute()
		require.NoError(t, err)

		normalLines := len(strings.Split(strings.TrimSpace(bufNormal.String()), "\n"))
		verboseLines := len(strings.Split(strings.TrimSpace(bufVerbose.String()), "\n"))

		// Verbose should have more output
		assert.GreaterOrEqual(t, verboseLines, normalLines,
			"verbose output should have at least as many lines as normal output")
	})
}

// TestRunStatus_CompletedWorkflow tests status display for completed workflow
func TestRunStatus_CompletedWorkflow(t *testing.T) {
	// Arrange: create execution state with completed status
	workflowID := "test-workflow-completed"
	dir := testutil.SetupTestDir(t)

	ctx := &workflow.ExecutionContext{
		WorkflowID:   workflowID,
		WorkflowName: "completed-workflow",
		Status:       workflow.StatusCompleted,
		CurrentStep:  "done",
		Inputs:       map[string]any{"test": "value"},
		States: map[string]workflow.StepState{
			"start": {
				Status:      workflow.StatusCompleted,
				Output:      "step output",
				ExitCode:    0,
				StartedAt:   time.Now().Add(-5 * time.Minute),
				CompletedAt: time.Now().Add(-4 * time.Minute),
			},
		},
		Env:         map[string]string{},
		StartedAt:   time.Now().Add(-6 * time.Minute),
		UpdatedAt:   time.Now().Add(-4 * time.Minute),
		CompletedAt: time.Now().Add(-4 * time.Minute),
	}

	storagePath := filepath.Join(dir, ".awf")
	stateStore := store.NewJSONStore(filepath.Join(storagePath, "states"))
	err := stateStore.Save(context.TODO(), ctx)
	require.NoError(t, err, "failed to create test state")

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", workflowID, "--storage", storagePath})

	// Act: execute status command
	err = cmd.Execute()

	// Assert: command succeeds and shows completed status
	require.NoError(t, err, "status should succeed for completed workflow")
	output := buf.String()
	assert.Contains(t, output, string(workflow.StatusCompleted), "output should show completed status")
	assert.Contains(t, output, workflowID, "output should contain workflow ID")
}

// TestRunStatus_StateLoadError tests error handling when state file is corrupted
func TestRunStatus_StateLoadError(t *testing.T) {
	// Arrange: create test directory with invalid JSON state file
	dir := testutil.SetupTestDir(t)
	storagePath := filepath.Join(dir, ".awf")
	statesDir := filepath.Join(storagePath, "states")

	// Create states directory
	err := os.MkdirAll(statesDir, 0o750)
	require.NoError(t, err)

	// Write corrupted JSON file
	workflowID := "corrupted-state"
	stateFile := filepath.Join(statesDir, workflowID+".json")
	err = os.WriteFile(stateFile, []byte("{invalid json"), 0o600)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", workflowID, "--storage", storagePath})

	// Act: execute status command
	err = cmd.Execute()

	// Assert: command fails with load error
	require.Error(t, err, "status should fail for corrupted state file")
	assert.Contains(t, err.Error(), "failed to load state", "error should indicate state load failure")
}

// TestRunStatus_EmptyStepsMap tests workflow with no step states
func TestRunStatus_EmptyStepsMap(t *testing.T) {
	// Arrange: create execution state with empty States map
	workflowID := "test-workflow-no-steps"
	dir := testutil.SetupTestDir(t)

	ctx := &workflow.ExecutionContext{
		WorkflowID:   workflowID,
		WorkflowName: "no-steps-workflow",
		Status:       workflow.StatusRunning,
		CurrentStep:  "start",
		Inputs:       map[string]any{},
		States:       map[string]workflow.StepState{}, // Empty states
		Env:          map[string]string{},
		StartedAt:    time.Now().Add(-1 * time.Minute),
		UpdatedAt:    time.Now(),
	}

	storagePath := filepath.Join(dir, ".awf")
	stateStore := store.NewJSONStore(filepath.Join(storagePath, "states"))
	err := stateStore.Save(context.TODO(), ctx)
	require.NoError(t, err, "failed to create test state")

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", workflowID, "--storage", storagePath})

	// Act: execute status command
	err = cmd.Execute()

	// Assert: command succeeds even with no steps
	require.NoError(t, err, "status should succeed for workflow with no step states")
	output := buf.String()
	assert.Contains(t, output, workflowID, "output should contain workflow ID")
	assert.Contains(t, output, string(workflow.StatusRunning), "output should show running status")
}
