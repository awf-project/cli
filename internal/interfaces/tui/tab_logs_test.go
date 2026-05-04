package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogsTab(t *testing.T) {
	path := "/tmp/logs.jsonl"
	tab := newLogsTab(path)

	assert.Equal(t, path, tab.path)
	require.NotNil(t, tab.tailer)
	assert.Empty(t, tab.entries)
	assert.Equal(t, 0, tab.width)
	assert.Equal(t, 0, tab.height)
	assert.True(t, tab.autoScroll, "auto-scroll should be enabled by default")
	assert.True(t, tab.watching, "watching should be true when path is set")
}

func TestNewLogsTab_NoPath(t *testing.T) {
	tab := newLogsTab("")
	assert.False(t, tab.watching, "watching should be false when no path is given")
}

func TestLogsTab_Init_WithPath(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	jsonLine := `{"event":"workflow.started","timestamp":"2026-01-01T10:00:00Z","workflow_name":"test"}`
	err := os.WriteFile(logFile, []byte(jsonLine+"\n"), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	cmd := tab.Init()

	require.NotNil(t, cmd, "Init should return a Cmd when path is configured")
}

func TestLogsTab_Init_NoPath(t *testing.T) {
	tab := newLogsTab("")
	cmd := tab.Init()
	assert.Nil(t, cmd, "Init should return nil when no path is configured")
}

func TestLogsTab_Update_WindowSizeMsg(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}

	updatedTab, cmd := tab.Update(msg)

	assert.Equal(t, 120, updatedTab.width)
	assert.Equal(t, 40, updatedTab.height)
	assert.Nil(t, cmd)
}

func TestLogsTab_Update_LogLineMsg(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)

	entry := LogEntry{
		Timestamp:    "2026-01-01T10:00:00Z",
		Event:        "workflow.started",
		WorkflowName: "deploy",
		ExecutionID:  "abc-123",
	}
	msg := LogLineMsg{Entry: entry}

	updatedTab, _ := tab.Update(msg)

	assert.Len(t, updatedTab.entries, 1)
	assert.Equal(t, entry, updatedTab.entries[0])
}

func TestLogsTab_Update_MultipleLogLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)

	entries := []LogEntry{
		{Timestamp: "2026-01-01T10:00:00Z", Event: "workflow.started", WorkflowName: "deploy"},
		{Timestamp: "2026-01-01T10:00:05Z", Event: "workflow.completed", WorkflowName: "deploy", Status: "success"},
		{Timestamp: "2026-01-01T10:01:00Z", Event: "workflow.started", WorkflowName: "test"},
	}

	for _, entry := range entries {
		msg := LogLineMsg{Entry: entry}
		tab, _ = tab.Update(msg)
	}

	assert.Len(t, tab.entries, 3)
	assert.Equal(t, "deploy", tab.entries[0].WorkflowName)
	assert.Equal(t, "success", tab.entries[1].Status)
	assert.Equal(t, "test", tab.entries[2].WorkflowName)
}

func TestLogsTab_Update_MaxEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)

	// Insert maxLogEntries + 10 entries; oldest should be trimmed.
	for i := range maxLogEntries + 10 {
		entry := LogEntry{Event: "workflow.started", WorkflowName: "entry"}
		if i == maxLogEntries+9 {
			entry.WorkflowName = "last"
		}
		tab, _ = tab.Update(LogLineMsg{Entry: entry})
	}

	assert.Len(t, tab.entries, maxLogEntries)
	assert.Equal(t, "last", tab.entries[maxLogEntries-1].WorkflowName)
}

func TestLogsTab_Update_TickMsg_Watching(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	_, cmd := tab.Update(logsTickMsg{})

	// Should trigger another Next() call (non-nil cmd).
	assert.NotNil(t, cmd)
}

func TestLogsTab_Update_TickMsg_NotWatching(t *testing.T) {
	tab := newLogsTab("")
	_, cmd := tab.Update(logsTickMsg{})
	// No path — no cmd.
	assert.Nil(t, cmd)
}

func TestLogsTab_Update_RotationMsg(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	updatedTab, cmd := tab.Update(logRotationMsg{path: logFile})

	assert.NotEmpty(t, updatedTab.rotationNotice, "rotation notice should be set")
	_ = cmd // tick loop is self-sustaining; rotation just updates the notice
}

func TestLogsTab_Update_FKey_ReenablesAutoScroll(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	tab.autoScroll = false // simulate user scrolled up

	updatedTab, _ := tab.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})

	assert.True(t, updatedTab.autoScroll, "pressing 'f' should re-enable auto-scroll")
}

// ---------------------------------------------------------------------------
// View tests
// ---------------------------------------------------------------------------

func TestLogsTab_View_EmptyNoPath(t *testing.T) {
	tab := newLogsTab("")
	output := tab.View()

	assert.Contains(t, output, "No AWF audit log found")
	assert.Contains(t, output, "audit.jsonl")
}

func TestLogsTab_View_EmptyWithPath(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	// Window size must be set for the header separator to render.
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	output := tab.View()

	// With a path but no entries, should show header + waiting message.
	assert.Contains(t, output, logFile)
	assert.Contains(t, output, "Waiting for log entries")
}

func TestLogsTab_View_WithEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	tab.entries = []LogEntry{
		{Timestamp: "2026-01-01T10:00:00Z", Event: "workflow.started", WorkflowName: "deploy"},
		{Timestamp: "2026-01-01T10:00:05Z", Event: "workflow.completed", WorkflowName: "deploy", Status: "success", DurationMs: 5000},
	}
	tab.viewport.SetContent(tab.renderEntries())

	output := tab.View()

	assert.Contains(t, output, "2026-01-01T10:00:00Z")
	assert.Contains(t, output, "deploy")
	assert.Contains(t, output, "2026-01-01T10:00:05Z")
	assert.Contains(t, output, "success")
}

func TestLogsTab_ViewFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")
	err := os.WriteFile(logFile, []byte(""), 0o644)
	require.NoError(t, err)

	tab := newLogsTab(logFile)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	tab.entries = []LogEntry{
		{Timestamp: "2026-01-01T10:00:00Z", Event: "workflow.started", WorkflowName: "deploy"},
	}
	tab.viewport.SetContent(tab.renderEntries())

	output := tab.View()

	assert.Contains(t, output, "2026-01-01T10:00:00Z")
	assert.Contains(t, output, "started")
	assert.Contains(t, output, "deploy")
}

// ---------------------------------------------------------------------------
// renderEntries / formatLogEntry tests
// ---------------------------------------------------------------------------

func TestRenderEntries_Empty(t *testing.T) {
	tab := newLogsTab("")
	out := tab.renderEntries()
	assert.Equal(t, "", out)
}

func TestFormatLogEntry(t *testing.T) {
	tests := []struct {
		name     string
		entry    LogEntry
		contains []string
	}{
		{
			name:     "workflow started",
			entry:    LogEntry{Timestamp: "2026-01-01T10:00:00Z", Event: "workflow.started", WorkflowName: "deploy"},
			contains: []string{"2026-01-01T10:00:00Z", "started", "deploy"},
		},
		{
			name:     "workflow completed with status",
			entry:    LogEntry{Timestamp: "2026-01-01T10:00:05Z", Event: "workflow.completed", WorkflowName: "deploy", Status: "success", DurationMs: 5000},
			contains: []string{"2026-01-01T10:00:05Z", "completed", "deploy", "success", "5000ms"},
		},
		{
			name:     "workflow completed with error",
			entry:    LogEntry{Timestamp: "2026-01-01T10:00:05Z", Event: "workflow.completed", WorkflowName: "deploy", Status: "success", Error: "step failed"},
			contains: []string{"completed", "deploy", "step failed"},
		},
		{
			name:     "no timestamp",
			entry:    LogEntry{Event: "workflow.started", WorkflowName: "test"},
			contains: []string{"—", "started", "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := formatLogEntry(&tt.entry)
			for _, want := range tt.contains {
				assert.Contains(t, out, want)
			}
		})
	}
}

func TestStyledEvent(t *testing.T) {
	for _, event := range []string{"workflow.started", "workflow.completed", "unknown", ""} {
		out := styledEvent(event)
		if event != "" {
			assert.NotEmpty(t, out)
		}
	}
}
