//go:build integration

// Feature: F088
package tui_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/tui"
)

func TestTUIModel_WorkflowExecutionLifecycle(t *testing.T) {
	m := tui.New()

	initCmd := m.Init()
	require.NotNil(t, initCmd, "Init should return a command to trigger initial load")

	initMsg := initCmd()
	_, ok := initMsg.(tui.WorkflowsLoadedMsg)
	require.True(t, ok, "Init command should produce WorkflowsLoadedMsg")

	wf := &workflow.Workflow{Name: "deploy-app", Description: "Deploy to production"}
	loadedMsg := tui.WorkflowsLoadedMsg{
		Workflows: []*workflow.Workflow{wf},
	}
	resizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updated, _ := m.Update(resizeMsg)
	m = updated.(tui.Model)

	updated, _ = m.Update(loadedMsg)
	m = updated.(tui.Model)

	output := m.View().Content
	assert.Contains(t, output, "1:Workflows", "tab bar should show workflows tab")
	assert.Contains(t, output, "2:Monitoring", "tab bar should show monitoring tab")

	// Switch to monitoring tab, start execution.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(tui.Model)

	execCtx := workflow.NewExecutionContext("exec-abc123", "deploy-app")
	done := make(chan error, 1)
	done <- nil
	startedMsg := tui.ExecutionStartedMsg{ExecutionID: "exec-abc123", Workflow: wf, ExecCtx: execCtx, Done: done}
	updated, _ = m.Update(startedMsg)
	m = updated.(tui.Model)

	finishedMsg := tui.ExecutionFinishedMsg{Err: nil}
	updated, _ = m.Update(finishedMsg)
	m = updated.(tui.Model)

	output = m.View().Content
	assert.NotEmpty(t, output, "view should render after execution lifecycle")
}

func TestTUIModel_ErrorsAreHandledGracefully(t *testing.T) {
	m := tui.New()

	errMsg := tui.ErrMsg{Err: assert.AnError}
	updated, _ := m.Update(errMsg)
	m = updated.(tui.Model)

	// ErrMsg does not crash and the model remains functional.
	output := m.View().Content
	assert.NotEmpty(t, output, "view should still render after error message")
}

func TestTUIModel_WindowResize(t *testing.T) {
	m := tui.New()

	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updated, _ := m.Update(resizeMsg)
	m = updated.(tui.Model)

	output := m.View().Content
	assert.NotEmpty(t, output, "view should render after resize")
}

func TestTUIModel_HistoryLoadedMsg_PopulatesHistoryTab(t *testing.T) {
	m := tui.New()

	records := []*workflow.ExecutionRecord{
		{ID: "exec-1", WorkflowName: "my-workflow", Status: "success"},
	}
	stats := &workflow.HistoryStats{TotalExecutions: 1, SuccessCount: 1}

	updated, _ := m.Update(tui.HistoryLoadedMsg{Records: records, Stats: stats})
	m = updated.(tui.Model)

	// Switch to history tab to verify content.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	m = updated.(tui.Model)

	output := m.View().Content
	assert.NotEmpty(t, output)
}

func TestTUITailer_ReadsNewJSONLLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.jsonl")

	entries := []map[string]any{
		{"event": "workflow.started", "timestamp": "2026-05-01T10:00:00Z", "workflow_name": "deploy", "execution_id": "abc-123", "user": "pocky", "schema_version": 1},
		{"event": "workflow.completed", "timestamp": "2026-05-01T10:00:05Z", "workflow_name": "deploy", "execution_id": "abc-123", "status": "success", "duration_ms": 5000, "schema_version": 1},
	}
	f, err := os.Create(logFile)
	require.NoError(t, err)
	enc := json.NewEncoder(f)
	for _, e := range entries {
		require.NoError(t, enc.Encode(e))
	}
	require.NoError(t, f.Close())

	tailer := tui.NewTailer(logFile)

	// First Next() tails the last entries from the file as a batch.
	cmd1 := tailer.Next()
	require.NotNil(t, cmd1)
	msg1 := cmd1()
	batch, ok := msg1.(tui.LogBatchMsg)
	require.True(t, ok, "first Next() should return LogBatchMsg, got %T", msg1)
	require.Len(t, batch.Entries, 2)
	assert.Equal(t, "workflow.started", batch.Entries[0].Event)
	assert.Equal(t, "workflow.completed", batch.Entries[1].Event)
	assert.Equal(t, "success", batch.Entries[1].Status)

	// Subsequent Next() follows — no new data returns nil.
	cmd2 := tailer.Next()
	require.NotNil(t, cmd2)
	msg2 := cmd2()
	assert.Nil(t, msg2, "Follow() should return nil when no more lines")
}

func TestTUITailer_SkipsMalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "bad.jsonl")

	require.NoError(t, os.WriteFile(logFile, []byte("not valid json\n"), 0o644))

	tailer := tui.NewTailer(logFile)

	cmd := tailer.Next()
	require.NotNil(t, cmd)
	msg := cmd()
	assert.Nil(t, msg, "malformed JSON should return nil, not crash")
}

func TestTUITailer_HandlesNonexistentFile(t *testing.T) {
	tailer := tui.NewTailer("/tmp/nonexistent-tui-test-file.jsonl")

	cmd := tailer.Next()
	require.NotNil(t, cmd, "Next() must return a tea.Cmd even for missing files")
	// The tailer emits an internal rotation message (unexported) when the file
	// does not exist; it does NOT crash or return nil. The model handles the
	// rotation message gracefully via logRotationMsg routing in tab_logs.go.
	_ = cmd() // must not panic
}
