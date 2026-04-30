// Feature: F087
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

func TestFullIDDisplay_MultipleRecords(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	records := []struct {
		id           string
		workflowID   string
		workflowName string
		status       string
	}{
		{
			id:           "550e8400-e29b-41d4-a716-446655440000",
			workflowID:   "wf-staging-deploy-eu-west-1-primary",
			workflowName: "deploy-staging-eu-west-1",
			status:       "success",
		},
		{
			id:           "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
			workflowID:   "wf-prod-rollback-us-east-2-canary-v2",
			workflowName: "production-rollback-us-east-2-canary",
			status:       "failed",
		},
		{
			id:           "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			workflowID:   "wf-ci",
			workflowName: "ci",
			status:       "success",
		},
	}

	for i, r := range records {
		err = historyStore.Record(ctx, &workflow.ExecutionRecord{
			ID:           r.id,
			WorkflowID:   r.workflowID,
			WorkflowName: r.workflowName,
			Status:       r.status,
			StartedAt:    now.Add(-time.Duration(len(records)-i) * 10 * time.Minute),
			CompletedAt:  now.Add(-time.Duration(len(records)-i-1) * 10 * time.Minute),
			DurationMs:   600000,
		})
		require.NoError(t, err)
	}
	require.NoError(t, historyStore.Close())

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	for _, r := range records {
		assert.Contains(t, output, r.id, "execution ID must appear in full")
		assert.Contains(t, output, r.workflowName, "workflow name must appear in full")
	}
	assert.NotContains(t, output, "...", "no values should be truncated")
}

func TestFullIDDisplay_TabwriterAlignment(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	// Short and long IDs to verify tabwriter aligns columns
	err = historyStore.Record(ctx, &workflow.ExecutionRecord{
		ID:           "short-id",
		WorkflowID:   "wf-1",
		WorkflowName: "ci",
		Status:       "success",
		StartedAt:    now.Add(-2 * time.Minute),
		CompletedAt:  now.Add(-time.Minute),
		DurationMs:   60000,
	})
	require.NoError(t, err)

	err = historyStore.Record(ctx, &workflow.ExecutionRecord{
		ID:           "550e8400-e29b-41d4-a716-446655440000",
		WorkflowID:   "wf-prod-deploy-multi-region-failover-v3",
		WorkflowName: "deploy-production-multi-region-failover",
		Status:       "failed",
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
		DurationMs:   60000,
		ErrorMessage: "timeout",
	})
	require.NoError(t, err)
	require.NoError(t, historyStore.Close())

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "history"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 4, "header + separator + 2 data rows")

	assert.Contains(t, lines[0], "ID")
	assert.Contains(t, lines[0], "WORKFLOW")
	assert.Contains(t, lines[0], "STATUS")

	assert.Contains(t, output, "short-id")
	assert.Contains(t, output, "550e8400-e29b-41d4-a716-446655440000")
	assert.Contains(t, output, "deploy-production-multi-region-failover")
}

func TestFullIDDisplay_JSONPreservesFullValues(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now()

	fullUUID := "550e8400-e29b-41d4-a716-446655440000"
	fullWorkflowID := "wf-staging-deploy-eu-west-1-primary-v2"
	fullWorkflowName := "deploy-staging-eu-west-1-primary"

	err = historyStore.Record(ctx, &workflow.ExecutionRecord{
		ID:           fullUUID,
		WorkflowID:   fullWorkflowID,
		WorkflowName: fullWorkflowName,
		Status:       "success",
		StartedAt:    now.Add(-5 * time.Minute),
		CompletedAt:  now,
		DurationMs:   300000,
	})
	require.NoError(t, err)
	require.NoError(t, historyStore.Close())

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "--format=json", "history"})

	err = cmd.Execute()
	require.NoError(t, err)

	var records []map[string]interface{}
	err = json.Unmarshal(out.Bytes(), &records)
	require.NoError(t, err)
	require.Len(t, records, 1)

	assert.Equal(t, fullUUID, records[0]["id"])
	assert.Equal(t, fullWorkflowID, records[0]["workflow_id"])
	assert.Equal(t, fullWorkflowName, records[0]["workflow_name"])
}
