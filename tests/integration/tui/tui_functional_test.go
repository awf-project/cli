//go:build integration

// Feature: F088
package tui_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/tui"
)

func TestTUIModel_WorkflowExecutionLifecycle(t *testing.T) {
	wf := &workflow.Workflow{Name: "deploy-app", Description: "Deploy to production"}

	// The TUI's sole execution entry point is the WorkflowFacade. Wire a fake facade
	// (returning a live RunSession) into the bridge exactly as production does via
	// command.go/SetFacade — no legacy ExecutionContext or Done channel is involved.
	facade := &fakeFacade{session: newFakeRunSession("exec-abc123")}
	bridge := tui.NewBridge(&fakeLister{wf: wf}, nil)
	bridge.SetFacade(facade)
	m := tui.NewWithBridge(bridge, context.Background(), "")

	resizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updated, _ := m.Update(resizeMsg)
	m = updated.(tui.Model)

	updated, _ = m.Update(tui.WorkflowsLoadedMsg{Workflows: []*workflow.Workflow{wf}})
	m = updated.(tui.Model)

	output := m.View().Content
	assert.Contains(t, output, "1:Workflows", "tab bar should show workflows tab")
	assert.Contains(t, output, "2:Monitoring", "tab bar should show monitoring tab")

	// Switch to monitoring tab.
	updated, _ = m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m = updated.(tui.Model)

	// Launch through the facade — this is exactly what LaunchWorkflowMsg triggers in
	// production. The resulting ExecutionStartedMsg carries the live RunSession.
	startedMsg := bridge.RunWorkflowViaFacade(context.Background(), wf.Name, nil)()
	started, ok := startedMsg.(tui.ExecutionStartedMsg)
	require.True(t, ok, "facade launch must yield ExecutionStartedMsg, got %T", startedMsg)
	require.NotNil(t, started.Session, "facade-driven start must carry a live RunSession")

	updated, _ = m.Update(started)
	m = updated.(tui.Model)

	// In production the session's terminal event drives ExecutionFinishedMsg via
	// StartEventLoop; deliver it directly here to finalize the monitoring tab.
	updated, _ = m.Update(tui.ExecutionFinishedMsg{Err: nil})
	m = updated.(tui.Model)

	output = m.View().Content
	assert.NotEmpty(t, output, "view should render after facade-driven execution lifecycle")
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

// --- Facade test doubles: the TUI's only execution path is the WorkflowFacade ---

// fakeRunSession is a minimal ports.RunSession with no live events; the execution
// lifecycle is asserted at the model/message boundary, not by streaming events.
type fakeRunSession struct {
	id     string
	events chan ports.Event
}

func newFakeRunSession(id string) *fakeRunSession {
	ch := make(chan ports.Event)
	close(ch) // StartEventLoop ranges over a closed channel and exits cleanly
	return &fakeRunSession{id: id, events: ch}
}

func (s *fakeRunSession) ID() string                        { return s.id }
func (s *fakeRunSession) Events() <-chan ports.Event        { return s.events }
func (s *fakeRunSession) Respond(ports.InputResponse) error { return nil }
func (s *fakeRunSession) Err() error                        { return nil }
func (s *fakeRunSession) Close() error                      { return nil }

// fakeFacade is a minimal ports.WorkflowFacade whose Run returns the wired session.
type fakeFacade struct {
	session ports.RunSession
}

func (f *fakeFacade) List(context.Context) ([]ports.WorkflowSummary, error) { return nil, nil }

func (f *fakeFacade) Validate(context.Context, ports.RunRequest) (ports.ValidationReport, error) { //nolint:gocritic // interface contract: RunRequest passed by value
	return ports.ValidationReport{}, nil
}

func (f *fakeFacade) Status(context.Context, string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

func (f *fakeFacade) History(context.Context, ports.HistoryFilter) ([]ports.RunRecord, error) {
	return nil, nil
}

func (f *fakeFacade) Run(_ context.Context, _ ports.RunRequest) (ports.RunSession, error) { //nolint:gocritic // interface contract: RunRequest passed by value
	return f.session, nil
}

func (f *fakeFacade) Resume(_ context.Context, _ ports.ResumeRequest) (ports.RunSession, error) { //nolint:gocritic // interface contract: ResumeRequest passed by value
	return f.session, nil
}

// fakeLister is a minimal tui.WorkflowLister so RunWorkflowViaFacade can load the
// workflow definition for the monitoring tab's step tree.
type fakeLister struct {
	wf *workflow.Workflow
}

func (l *fakeLister) ListAllWorkflows(context.Context) ([]workflow.WorkflowEntry, error) {
	return []workflow.WorkflowEntry{{Name: l.wf.Name}}, nil
}

func (l *fakeLister) GetWorkflow(context.Context, string) (*workflow.Workflow, error) {
	return l.wf, nil
}

func (l *fakeLister) ValidateWorkflow(context.Context, string) error { return nil }
