//go:build integration

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusCommand_PersistedWorkflowAttemptLineage proves the facade-wired
// public status command selects the greatest persisted attempt for a stable
// workflow ID, even when that attempt has an older timestamp than prior rows.
func TestStatusCommand_PersistedWorkflowAttemptLineage(t *testing.T) {
	storageDir := t.TempDir()
	const workflowID = "stable-workflow"

	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(storageDir, "history.db"))
	require.NoError(t, err)

	newerAttempt := &workflow.ExecutionRecord{
		ID:           "history-row-01",
		WorkflowID:   workflowID,
		Attempt:      1,
		WorkflowName: "deploy",
		Status:       "success",
		StartedAt:    time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC),
		CompletedAt:  time.Date(2026, 6, 2, 10, 1, 0, 0, time.UTC),
	}
	greatestAttempt := &workflow.ExecutionRecord{
		ID:            "history-row-25",
		WorkflowID:    workflowID,
		Attempt:       25,
		ResumedFromID: "history-row-24", // Retention may remove the predecessor row.
		WorkflowName:  "deploy",
		Status:        "failed",
		StartedAt:     time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		CompletedAt:   time.Date(2026, 6, 1, 10, 1, 0, 0, time.UTC),
	}
	require.NoError(t, historyStore.Record(context.Background(), newerAttempt))
	require.NoError(t, historyStore.Record(context.Background(), greatestAttempt))
	require.NoError(t, historyStore.Close())

	cmd, cleanup := cli.NewRootCommandAutoFacade()
	t.Cleanup(cleanup)
	output := new(bytes.Buffer)
	cmd.SetOut(output)
	cmd.SetErr(output)
	cmd.SetArgs([]string{"--storage=" + storageDir, "--format=json", "status", workflowID})

	require.NoError(t, cmd.ExecuteContext(context.Background()))
	var status struct {
		WorkflowID string `json:"workflow_id"`
		Status     string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &status))
	assert.Equal(t, workflowID, status.WorkflowID)
	assert.Equal(t, "failed", status.Status)
}

func TestStatusCommand_NoArgs(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow ID provided")
	}
}

func TestStatusCommand_NotFound(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "nonexistent-id"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent workflow ID")
	}

	output := buf.String()
	errOutput := buf.String()
	combined := output + errOutput
	if !strings.Contains(combined, "not found") && err != nil && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' message, got output: %s, err: %v", combined, err)
	}
}

func TestStatusCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "status" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'status' subcommand")
	}
}

func TestStatusCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"status", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "workflow") {
		t.Errorf("expected help text about workflow, got: %s", output)
	}
}
