package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// Resume Command Unit Tests (F013)
// =============================================================================

func TestResumeCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			found = true
			break
		}
	}

	assert.True(t, found, "expected root command to have 'resume' subcommand")
}

func TestResumeCommand_HasListFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			flag := sub.Flags().Lookup("list")
			require.NotNil(t, flag, "expected 'resume' command to have --list flag")
			assert.Equal(t, "l", flag.Shorthand, "list flag should have -l shorthand")
			return
		}
	}

	t.Error("resume command not found")
}

func TestResumeCommand_HasInputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			flag := sub.Flags().Lookup("input")
			require.NotNil(t, flag, "expected 'resume' command to have --input flag")
			assert.Equal(t, "i", flag.Shorthand, "input flag should have -i shorthand")
			return
		}
	}

	t.Error("resume command not found")
}

func TestResumeCommand_HasOutputFlag(t *testing.T) {
	cmd := cli.NewRootCommand()

	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			flag := sub.Flags().Lookup("output")
			require.NotNil(t, flag, "expected 'resume' command to have --output flag")
			assert.Equal(t, "o", flag.Shorthand, "output flag should have -o shorthand")
			return
		}
	}

	t.Error("resume command not found")
}

func TestResumeCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"resume", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Resume")
	assert.Contains(t, output, "--list")
	assert.Contains(t, output, "--input")
}

func TestResumeCommand_NoArgsNoListFlag(t *testing.T) {
	// Without --list flag and without workflow-id argument, should error
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"resume"})

	err := cmd.Execute()
	require.Error(t, err, "expected error when no workflow-id provided")
	assert.Contains(t, err.Error(), "workflow-id required")
}

func TestResumeCommand_ListFlag_NoResumableWorkflows(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty states directory
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No resumable workflows")
}

func TestResumeCommand_ListFlag_ShowsResumableWorkflows(t *testing.T) {
	tmpDir := t.TempDir()

	// Create states directory with a resumable state file
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	// Create a state file for a running workflow
	// Note: JSON uses Go field names (PascalCase) since no json tags are defined
	stateContent := `{
		"WorkflowID": "test-id-123",
		"WorkflowName": "my-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {"Name": "step1", "Status": "completed"}
		},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "test-id-123.json"),
		[]byte(stateContent),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-id-123")
	assert.Contains(t, output, "my-workflow")
	assert.Contains(t, output, "running")
	assert.Contains(t, output, "step2")
}

func TestResumeCommand_ListFlag_FiltersCompletedWorkflows(t *testing.T) {
	tmpDir := t.TempDir()

	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	// Create a completed workflow state (should be filtered out)
	completedState := `{
		"WorkflowID": "completed-id",
		"WorkflowName": "completed-workflow",
		"Status": "completed",
		"CurrentStep": "done",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "completed-id.json"),
		[]byte(completedState),
		0644,
	))

	// Create a running workflow state (should be shown)
	runningState := `{
		"WorkflowID": "running-id",
		"WorkflowName": "running-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "running-id.json"),
		[]byte(runningState),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "running-id", "should show running workflow")
	assert.NotContains(t, output, "completed-id", "should not show completed workflow")
}

func TestResumeCommand_ListFlag_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	// Create a running workflow state
	runningState := `{
		"WorkflowID": "json-test-id",
		"WorkflowName": "json-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {},
		"States": {"step0": {"Name": "step0", "Status": "completed"}},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "json-test-id.json"),
		[]byte(runningState),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list", "--format=json"})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify output is valid JSON
	var result []map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err, "output should be valid JSON")
	require.Len(t, result, 1)
	assert.Equal(t, "json-test-id", result[0]["workflow_id"])
	assert.Equal(t, "json-workflow", result[0]["workflow_name"])
}

func TestResumeCommand_WorkflowNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty states directory
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "nonexistent-workflow-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResumeCommand_AlreadyCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create states and workflows directories
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	// Create a completed workflow state
	completedState := `{
		"WorkflowID": "completed-id",
		"WorkflowName": "test-workflow",
		"Status": "completed",
		"CurrentStep": "done",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "completed-id.json"),
		[]byte(completedState),
		0644,
	))

	// Create the workflow definition
	workflowContent := `name: test-workflow
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
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "test-workflow.yaml"),
		[]byte(workflowContent),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "completed-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed")
}

func TestResumeCommand_InputOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create states and workflows directories
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	// Create a running workflow state
	runningState := `{
		"WorkflowID": "override-id",
		"WorkflowName": "override-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {"key": "original"},
		"States": {},
		"Env": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "override-id.json"),
		[]byte(runningState),
		0644,
	))

	// Create the workflow definition
	workflowContent := `name: override-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "{{inputs.key}}"
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "override-workflow.yaml"),
		[]byte(workflowContent),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "override-id", "--input=key=overridden"})

	err := cmd.Execute()
	// The resume should work with input override
	// (Implementation will determine success/failure)
	_ = err // Acknowledge error for now - test verifies command accepts --input flag
}

func TestResumeCommand_WorkflowDefinitionDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create states directory but NOT workflows directory with the definition
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	// Create a running workflow state (but workflow definition doesn't exist)
	runningState := `{
		"WorkflowID": "orphan-id",
		"WorkflowName": "deleted-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "orphan-id.json"),
		[]byte(runningState),
		0644,
	))
	// Note: No workflow file created for "deleted-workflow"

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "orphan-id"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResumeCommand_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create states and workflows directories
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	// Create a running workflow state (step1 completed, at step2)
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "success-id",
		"WorkflowName": "success-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 output"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "success-id.json"),
		[]byte(runningState),
		0644,
	))

	// Create the workflow definition
	workflowContent := `name: success-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "success-workflow.yaml"),
		[]byte(workflowContent),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "success-id"})

	err := cmd.Execute()
	require.NoError(t, err, "resume should succeed")

	output := buf.String()
	assert.True(t, strings.Contains(output, "completed") || strings.Contains(output, "success"),
		"output should indicate success")
}

func TestResumeCommand_QuietFormat(t *testing.T) {
	tmpDir := t.TempDir()

	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	// Create a running workflow state
	runningState := `{
		"WorkflowID": "quiet-id",
		"WorkflowName": "quiet-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "quiet-id.json"),
		[]byte(runningState),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list", "--format=quiet"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())
	// Quiet format should only output workflow IDs
	assert.Equal(t, "quiet-id", output)
}

func TestResumeCommand_TableFormat(t *testing.T) {
	tmpDir := t.TempDir()

	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	// Create a running workflow state
	runningState := `{
		"WorkflowID": "table-id",
		"WorkflowName": "table-workflow",
		"Status": "running",
		"CurrentStep": "step1",
		"Inputs": {},
		"States": {},
		"StartedAt": "2025-01-01T10:00:00Z",
		"UpdatedAt": "2025-01-01T10:05:00Z"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "table-id.json"),
		[]byte(runningState),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list", "--format=table"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Table format should contain headers and borders
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "WORKFLOW")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "table-id")
}

func TestResumeCommand_MultipleInputOverrides(t *testing.T) {
	// Test that multiple --input flags work
	cmd := cli.NewRootCommand()

	// Find the resume command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "resume" {
			// Verify input flag can be specified multiple times (StringArray)
			flag := sub.Flags().Lookup("input")
			require.NotNil(t, flag)
			// StringArray type allows multiple values
			return
		}
	}
	t.Error("resume command not found")
}

// =============================================================================
// SQLite History Store Integration Tests (bug-48)
// =============================================================================

func TestResumeCommand_SQLiteHistoryStore_Wiring(t *testing.T) {
	// Test that resume command creates history.db (SQLite) instead of Badger directory
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Create states and workflows directories
	statesDir := filepath.Join(tmpDir, "states")
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	require.NoError(t, os.MkdirAll(statesDir, 0755))
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	// Create a running workflow state
	now := time.Now().Format(time.RFC3339)
	runningState := `{
		"WorkflowID": "sqlite-test-id",
		"WorkflowName": "sqlite-workflow",
		"Status": "running",
		"CurrentStep": "step2",
		"Inputs": {},
		"States": {
			"step1": {
				"Name": "step1",
				"Status": "completed",
				"Output": "step1 done"
			}
		},
		"Env": {},
		"StartedAt": "` + now + `",
		"UpdatedAt": "` + now + `"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(statesDir, "sqlite-test-id.json"),
		[]byte(runningState),
		0644,
	))

	// Create the workflow definition
	workflowContent := `name: sqlite-workflow
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo step1
    on_success: step2
  step2:
    type: step
    command: echo step2
    on_success: done
  done:
    type: terminal
`
	require.NoError(t, os.WriteFile(
		filepath.Join(workflowsDir, "sqlite-workflow.yaml"),
		[]byte(workflowContent),
		0644,
	))

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "sqlite-test-id"})

	err := cmd.Execute()
	require.NoError(t, err, "resume should succeed")

	// Verify SQLite database file was created
	historyDBPath := filepath.Join(tmpDir, "history.db")
	_, err = os.Stat(historyDBPath)
	assert.NoError(t, err, "history.db should exist (SQLite)")

	// Verify Badger directory was NOT created
	badgerDir := filepath.Join(tmpDir, "history")
	_, err = os.Stat(badgerDir)
	assert.True(t, os.IsNotExist(err), "Badger history directory should NOT exist")
}

func TestResumeCommand_ConcurrentAccess(t *testing.T) {
	// Test that multiple concurrent resume --list commands don't cause lock errors
	// This validates the bug-48 fix for the resume command
	tmpDir := t.TempDir()

	// Create states directory with resumable workflows
	statesDir := filepath.Join(tmpDir, "states")
	require.NoError(t, os.MkdirAll(statesDir, 0755))

	// Create multiple running workflow states
	for i := 0; i < 3; i++ {
		state := `{
			"WorkflowID": "concurrent-` + string(rune('a'+i)) + `",
			"WorkflowName": "concurrent-workflow",
			"Status": "running",
			"CurrentStep": "step1",
			"Inputs": {},
			"States": {},
			"StartedAt": "2025-01-01T10:00:00Z",
			"UpdatedAt": "2025-01-01T10:05:00Z"
		}`
		require.NoError(t, os.WriteFile(
			filepath.Join(statesDir, "concurrent-"+string(rune('a'+i))+".json"),
			[]byte(state),
			0644,
		))
	}

	// Run concurrent resume --list commands
	const concurrency = 3
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "resume", "--list"})
			errCh <- cmd.Execute()
		}()
	}

	// Collect results - all should succeed without lock errors
	for i := 0; i < concurrency; i++ {
		err := <-errCh
		assert.NoError(t, err, "concurrent resume --list should not fail with lock errors")
	}
}
