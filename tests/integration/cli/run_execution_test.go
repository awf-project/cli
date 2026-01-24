//go:build integration

package cli_test

// C015 T008: Execution tests extracted from run_test.go
// Thread-safety: All tests use thread-safe patterns (no os.Chdir)
// Tests: 9 functions, ~300 lines
// Scope: Single step execution, history store wiring, concurrent workflows

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// Single Step Execution Tests
// =============================================================================

func TestRunCommand_SingleStep_WorkflowNotFound(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create .awf directory but no workflow

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "nonexistent", "--step=mystep"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunCommand_SingleStep_StepNotFound(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a workflow without the requested step
	workflowContent := `name: test
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test", "--step=nonexistent"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunCommand_SingleStep_Success(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create a simple workflow
	workflowContent := `name: test
version: "1.0.0"
states:
  initial: greet
  greet:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "test.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "test", "--step=greet"})

	err := cmd.Execute()
	// Should succeed (once implemented)
	require.NoError(t, err)
}

func TestRunCommand_SingleStep_WithInputs(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create a workflow with inputs
	workflowContent := `name: input-test
version: "1.0.0"
inputs:
  - name: message
    type: string
states:
  initial: show
  show:
    type: step
    command: echo "{{.inputs.message}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "input-test.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "input-test", "--step=show", "--input=message=test-value"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestRunCommand_SingleStep_WithMocks(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create a workflow where process depends on fetch output
	workflowContent := `name: mock-test
version: "1.0.0"
states:
  initial: fetch
  fetch:
    type: step
    command: curl http://api
    on_success: process
  process:
    type: step
    command: echo "{{.states.fetch.Output}}"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mock-test.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Execute the process step with mocked fetch output
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run", "mock-test",
		"--step=process",
		"--mock=states.fetch.output=mocked-data",
	})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestRunCommand_SingleStep_TerminalStepError(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create storage directories (states and history for Badger)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Create a workflow with a terminal step
	workflowContent := `name: terminal-test
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "terminal-test.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Try to execute a terminal step (should error)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "terminal-test", "--step=done"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "terminal")
}

// =============================================================================
// SQLite History Store Tests
// =============================================================================

// TestRunCommand_SQLiteHistoryStore_Wiring tests that workflows correctly use SQLiteHistoryStore
// This validates the T004 CLI wiring update from BadgerDB to SQLite
func TestRunCommand_SQLiteHistoryStore_Wiring(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "workflow execution uses SQLite history store",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := setupTestDir(t)

			// Setup workflow and directories
			require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

			workflowContent := `name: sqlite-test
version: "1.0.0"
states:
  initial: echo
  echo:
    type: step
    command: echo "testing sqlite"
    on_success: done
  done:
    type: terminal
`
			createTestWorkflow(t, tmpDir, "sqlite-test.yaml", workflowContent)

			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "sqlite-test"})

			err := cmd.Execute()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)

				// Verify SQLite database file was created (not Badger directory)
				historyDBPath := filepath.Join(tmpDir, "history.db")
				_, statErr := os.Stat(historyDBPath)
				assert.NoError(t, statErr, "SQLite history.db should exist after workflow execution")

				// Verify no Badger directory was created
				badgerPath := filepath.Join(tmpDir, "history")
				_, badgerErr := os.Stat(badgerPath)
				assert.True(t, os.IsNotExist(badgerErr), "Badger history directory should NOT exist")
			}
		})
	}
}

// TestRunCommand_SingleStep_SQLiteHistory verifies single step execution uses SQLite
func TestRunCommand_SingleStep_SQLiteHistory(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Setup
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	workflowContent := `name: step-test
version: "1.0.0"
states:
  initial: greet
  greet:
    type: step
    command: echo hello
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "step-test.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "step-test", "--step=greet"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify SQLite database was used
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, statErr := os.Stat(historyDBPath)
	assert.NoError(t, statErr, "SQLite history.db should exist after single step execution")
}

// =============================================================================
// Concurrent Workflow Execution Tests
// =============================================================================

// TestRunCommand_ConcurrentWorkflows validates that bug-48 is fixed
// Multiple workflows should be able to run concurrently without lock errors
func TestRunCommand_ConcurrentWorkflows(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Setup workflow and directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755))

	// Create a workflow that takes a bit of time
	workflowContent := `name: concurrent-test
version: "1.0.0"
states:
  initial: work
  work:
    type: step
    command: echo "workflow running" && sleep 0.1
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "concurrent-test.yaml", workflowContent)

	// Run multiple workflows concurrently
	const numConcurrent = 3
	errChan := make(chan error, numConcurrent)
	doneChan := make(chan struct{}, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(workerID int) {
			// Each goroutine needs its own command instance
			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "concurrent-test"})

			err := cmd.Execute()
			if err != nil {
				errChan <- fmt.Errorf("worker %d failed: %w (output: %s)", workerID, err, out.String())
			}
			doneChan <- struct{}{}
		}(i)
	}

	// Wait for all workers to complete
	for i := 0; i < numConcurrent; i++ {
		<-doneChan
	}
	close(errChan)

	// Check if any worker failed
	// Preallocate for potential errors
	errors := make([]error, 0, numConcurrent)
	for err := range errChan {
		errors = append(errors, err)
	}

	// All concurrent executions should succeed (bug-48 fix validation)
	assert.Empty(t, errors, "concurrent workflow executions should not fail with lock errors")

	// Verify history.db exists and is a valid SQLite file
	historyDBPath := filepath.Join(tmpDir, "history.db")
	info, err := os.Stat(historyDBPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, "history.db should have content")
}
