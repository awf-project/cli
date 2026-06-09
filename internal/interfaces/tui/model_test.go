package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// --- Tab constants ---

func TestTab_Constants_HaveCorrectIotaValues(t *testing.T) {
	assert.Equal(t, Tab(0), TabWorkflows, "TabWorkflows must be iota 0")
	assert.Equal(t, Tab(1), TabMonitoring, "TabMonitoring must be iota 1")
	assert.Equal(t, Tab(2), TabHistory, "TabHistory must be iota 2")
	assert.Equal(t, Tab(3), TabExternalLogs, "TabExternalLogs must be iota 3")
}

// --- New ---

func TestNew_ReturnsZeroValueModel(t *testing.T) {
	m := New()

	assert.Zero(t, m.width, "width should be zero-initialized")
	assert.Zero(t, m.height, "height should be zero-initialized")
	assert.Equal(t, TabWorkflows, m.activeTab, "should start on workflows tab")
	assert.Nil(t, m.bridge, "bridge should be nil when not provided")
}

func TestModel_ImplementsTeaModel(t *testing.T) {
	m := New()
	var _ tea.Model = m
}

// --- Init ---

func TestModel_Init_ReturnsCommandToLoadWorkflows(t *testing.T) {
	m := New()

	cmd := m.Init()

	require.NotNil(t, cmd, "Init should return a command to load workflows")
}

func TestModel_Init_WithNoBridge_ReturnsCommandEmittingWorkflowsLoadedMsg(t *testing.T) {
	m := New()

	cmd := m.Init()
	require.NotNil(t, cmd)

	msg := cmd()
	_, ok := msg.(WorkflowsLoadedMsg)
	assert.True(t, ok, "Init without bridge should emit WorkflowsLoadedMsg")
}

// --- Update: tab switching via numeric keys ---

func TestModel_Update_Key1_SwitchesToWorkflowsTab(t *testing.T) {
	m := New()
	m.activeTab = TabHistory // start elsewhere

	result, _ := m.Update(tea.KeyPressMsg{Code: '1', Text: "1"})

	updated := result.(Model)
	assert.Equal(t, TabWorkflows, updated.activeTab)
}

func TestModel_Update_Key2_SwitchesToMonitoringTab(t *testing.T) {
	m := New()

	result, _ := m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})

	updated := result.(Model)
	assert.Equal(t, TabMonitoring, updated.activeTab)
}

func TestModel_Update_Key3_SwitchesToHistoryTab(t *testing.T) {
	m := New()

	result, _ := m.Update(tea.KeyPressMsg{Code: '3', Text: "3"})

	updated := result.(Model)
	assert.Equal(t, TabHistory, updated.activeTab)
}

func TestModel_Update_Key4_SwitchesToLogsTab(t *testing.T) {
	m := New()

	result, _ := m.Update(tea.KeyPressMsg{Code: '4', Text: "4"})

	updated := result.(Model)
	assert.Equal(t, TabExternalLogs, updated.activeTab)
}

func TestModel_Update_KeyQ_ReturnsQuitCmd(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})

	require.NotNil(t, cmd, "pressing 'q' must return a quit command")
	// Verify it is a quit command by executing it and checking the message type.
	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

func TestModel_Update_CtrlC_ReturnsQuitCmd(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	require.NotNil(t, cmd, "pressing ctrl+c must return a quit command")
	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
}

// --- Update: window size ---

func TestModel_Update_WindowSizeMsg_StoresWidthAndHeight(t *testing.T) {
	m := New()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	updated := result.(Model)
	assert.Equal(t, 120, updated.width, "should update width on window resize")
	assert.Equal(t, 40, updated.height, "should update height on window resize")
}

func TestModel_Update_WindowSizeMsg_PropagatesZeroDimensions(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 24

	result, _ := m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	updated := result.(Model)
	assert.Equal(t, 0, updated.width)
	assert.Equal(t, 0, updated.height)
}

// --- Update: domain messages ---

func TestModel_Update_WorkflowsLoadedMsg_PopulatesWorkflowsTab(t *testing.T) {
	m := New()
	workflows := []*workflow.Workflow{
		{Name: "workflow-1"},
		{Name: "workflow-2"},
	}
	entries := []workflow.WorkflowEntry{
		{Name: "workflow-1", Source: "local"},
		{Name: "workflow-2", Source: "local"},
	}

	result, _ := m.Update(WorkflowsLoadedMsg{Workflows: workflows, Entries: entries})

	updated := result.(Model)
	assert.Len(t, updated.tabWorkflows.entries, 2)
}

func TestModel_Update_HistoryLoadedMsg_PopulatesHistoryTab(t *testing.T) {
	m := New()
	records := []*workflow.ExecutionRecord{
		{ID: "exec-1", WorkflowName: "wf1", Status: "success"},
	}
	stats := &workflow.HistoryStats{TotalExecutions: 1, SuccessCount: 1}

	result, _ := m.Update(HistoryLoadedMsg{Records: records, Stats: stats})

	updated := result.(Model)
	assert.Len(t, updated.tabHistory.history, 1)
	assert.Equal(t, stats, updated.tabHistory.stats)
	// Also propagated to monitoring tab.
	assert.Len(t, updated.tabMonitoring.history, 1)
	assert.Equal(t, stats, updated.tabMonitoring.stats)
}

func TestModel_Update_ExecutionStartedMsg_SetsExecCtxAndWorkflowOnMonitoringTab(t *testing.T) {
	m := New()
	wf := &workflow.Workflow{Name: "test-wf", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	execCtx := workflow.NewExecutionContext("exec-abc", "test-wf")
	done := make(chan error, 1)

	result, _ := m.Update(ExecutionStartedMsg{ExecutionID: "exec-abc", Workflow: wf, ExecCtx: execCtx, Done: done})

	updated := result.(Model)
	assert.Equal(t, wf, updated.tabMonitoring.wf)
	assert.Equal(t, execCtx, updated.tabMonitoring.execCtx)
	assert.True(t, updated.tabMonitoring.ticking)
}

func TestModel_Update_ExecutionFinishedMsg_StopsTicking(t *testing.T) {
	m := New()
	m.tabMonitoring.ticking = true
	wf := &workflow.Workflow{
		Name:    "test-wf",
		Initial: "s1",
		Steps:   map[string]*workflow.Step{"s1": {Name: "s1", Type: workflow.StepTypeTerminal}},
	}
	execCtx := workflow.NewExecutionContext("exec-1", "test-wf")
	execCtx.SetStepState("s1", workflow.StepState{Name: "s1", Status: workflow.StatusCompleted})
	m.tabMonitoring.SetExecCtx(execCtx, wf)

	result, _ := m.Update(ExecutionFinishedMsg{Err: nil})

	updated := result.(Model)
	assert.False(t, updated.tabMonitoring.ticking)
	assert.NotEmpty(t, updated.tabMonitoring.flatNodes)
}

func TestModel_Update_ExecutionFinishedMsg_WithError_SetsLastErr(t *testing.T) {
	m := New()
	m.tabMonitoring.ticking = true

	result, _ := m.Update(ExecutionFinishedMsg{Err: errors.New("step failed")})

	updated := result.(Model)
	assert.Equal(t, "step failed", updated.lastErr)
	assert.False(t, updated.tabMonitoring.ticking)
}

func TestModel_Update_ErrMsg_DoesNotPanic(t *testing.T) {
	m := New()

	require.NotPanics(t, func() {
		_, _ = m.Update(ErrMsg{Err: nil})
	})
}

func TestModel_Update_LogLineMsg_AppendsToLogsTab(t *testing.T) {
	m := New()
	entry := LogEntry{Timestamp: "2026-01-01T00:00:00Z", Event: "workflow.started", WorkflowName: "deploy"}

	result, _ := m.Update(LogLineMsg{Entry: entry})

	updated := result.(Model)
	require.Len(t, updated.tabLogs.entries, 1)
	assert.Equal(t, entry, updated.tabLogs.entries[0])
}

func TestModel_Update_LogLineMsg_CapsAtMaxLogEntries(t *testing.T) {
	m := New()
	// Pre-fill with maxLogEntries entries.
	m.tabLogs.entries = make([]LogEntry, maxLogEntries)
	for i := range m.tabLogs.entries {
		m.tabLogs.entries[i] = LogEntry{Event: "workflow.started", WorkflowName: "old"}
	}

	newEntry := LogEntry{Event: "workflow.completed", WorkflowName: "new"}
	result, _ := m.Update(LogLineMsg{Entry: newEntry})

	updated := result.(Model)
	assert.Len(t, updated.tabLogs.entries, maxLogEntries, "entries must not exceed maxLogEntries")
	assert.Equal(t, newEntry, updated.tabLogs.entries[len(updated.tabLogs.entries)-1])
}

// --- View ---

func TestModel_View_ReturnsNonEmptyString(t *testing.T) {
	m := New()

	output := m.View().Content

	assert.NotEmpty(t, output)
	assert.IsType(t, "", output)
}

func TestModel_View_RendersAllFourTabLabels(t *testing.T) {
	m := New()

	output := m.View().Content

	assert.Contains(t, output, "1:Workflows")
	assert.Contains(t, output, "2:Monitoring")
	assert.Contains(t, output, "3:History")
	assert.Contains(t, output, "4:Logs")
}

func TestModel_View_ActiveTabIsRendered(t *testing.T) {
	tests := []struct {
		name      string
		activeTab Tab
		label     string
	}{
		{"workflows tab", TabWorkflows, "1:Workflows"},
		{"monitoring tab", TabMonitoring, "2:Monitoring"},
		{"history tab", TabHistory, "3:History"},
		{"logs tab", TabExternalLogs, "4:Logs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			m.activeTab = tt.activeTab

			output := m.View().Content

			assert.Contains(t, output, tt.label)
		})
	}
}

func TestModel_View_TabSwitching_ChangesContentArea(t *testing.T) {
	m := New()
	// Place distinct content in each tab.
	m.tabWorkflows.entries = []workflow.WorkflowEntry{{Name: "my-distinct-workflow", Source: "local"}}

	m.activeTab = TabWorkflows
	workflowsView := m.View().Content

	m.activeTab = TabMonitoring
	monitoringView := m.View().Content

	// Both views contain the tab bar, but the content area should differ.
	assert.NotEqual(t, workflowsView, monitoringView)
}

// --- T061: facadeEventMsg routing ---

// TestModel_Update_FacadeEventMsg_RoutesToMonitoringTabRegardlessOfActiveTab verifies
// that facadeEventMsg is always forwarded to the monitoring tab regardless of which
// tab is currently active. This mirrors the tickMsg / executionPollMsg routing pattern.
func TestModel_Update_FacadeEventMsg_RoutesToMonitoringTabRegardlessOfActiveTab(t *testing.T) {
	tabs := []struct {
		name string
		tab  Tab
	}{
		{"workflows", TabWorkflows},
		{"history", TabHistory},
		{"logs", TabExternalLogs},
		{"monitoring", TabMonitoring},
	}

	for _, tt := range tabs {
		t.Run(tt.name+" active", func(t *testing.T) {
			m := New()
			m.activeTab = tt.tab
			m.tabMonitoring.steps = []workflow.Step{
				{Name: "step-a", Type: workflow.StepTypeCommand},
			}

			payload := &transcript.StepPayload{Name: "step-a", Kind: "command"}
			event := ports.Event{
				Seq:     1,
				Kind:    ports.EventStepStarted,
				Payload: payload,
			}

			result, _ := m.Update(facadeEventMsg{Event: event})

			updated := result.(Model)
			state, ok := updated.tabMonitoring.states["step-a"]
			require.True(t, ok, "monitoring tab must receive facade event when %s tab is active", tt.name)
			assert.Equal(t, workflow.StatusRunning, state.Status)
		})
	}
}

// TestModel_Update_ExecutionStartedMsg_WithSession_StartsEventLoop verifies that
// when ExecutionStartedMsg carries a non-nil RunSession, the monitoring tab's
// event-loop goroutine is started and events are forwarded to the program.
func TestModel_Update_ExecutionStartedMsg_WithSession_StartsEventLoop(t *testing.T) {
	m := New()

	eventChan := make(chan ports.Event, 1)
	defer close(eventChan)

	sentChan := make(chan tea.Msg, 1)
	m.tabMonitoring.SetSender(func(msg tea.Msg) {
		sentChan <- msg
	})

	mockSess := &mockRunSession{eventsChan: eventChan}
	wf := &workflow.Workflow{Name: "wf", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	execCtx := workflow.NewExecutionContext("exec-1", "wf")
	done := make(chan error, 1)

	m.Update(ExecutionStartedMsg{
		ExecutionID: "exec-1",
		Workflow:    wf,
		ExecCtx:     execCtx,
		Done:        done,
		Session:     mockSess,
	})

	// Send an event; the goroutine must forward it via sender.
	testEvent := ports.Event{Seq: 1, Kind: ports.EventRunStarted, RunID: "exec-1"}
	eventChan <- testEvent

	select {
	case msg := <-sentChan:
		fMsg, ok := msg.(facadeEventMsg)
		require.True(t, ok, "event loop must forward events as facadeEventMsg, got %T", msg)
		assert.Equal(t, uint64(1), fMsg.Event.Seq)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("event loop did not forward event within 200ms")
	}
}

// TestModel_Update_ExecutionStartedMsg_WithNilDone_DoesNotPanic verifies that the
// facade-driven path (RunWorkflowViaFacade) can emit an ExecutionStartedMsg with a
// nil Done channel without causing a panic or blocking the Bubble Tea event loop.
// ExecutionFinishedMsg will arrive via StartEventLoop when a terminal event is received.
func TestModel_Update_ExecutionStartedMsg_WithNilDone_DoesNotPanic(t *testing.T) {
	m := New()

	eventChan := make(chan ports.Event, 1)
	defer close(eventChan)

	sentChan := make(chan tea.Msg, 2)
	m.tabMonitoring.SetSender(func(msg tea.Msg) {
		sentChan <- msg
	})

	mockSess := &mockRunSession{eventsChan: eventChan}
	wf := &workflow.Workflow{Name: "wf", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}

	require.NotPanics(t, func() {
		_, cmd := m.Update(ExecutionStartedMsg{
			ExecutionID: "exec-1",
			Workflow:    wf,
			Session:     mockSess,
			// Done is intentionally nil — facade-driven path
		})
		// cmd must not be nil (at minimum the tick command from the monitoring tab)
		assert.NotNil(t, cmd, "monCmd must be returned even when Done is nil")
	})
}

// TestModel_Update_ExecutionStartedMsg_WithNilDone_FinishesViaEventLoop verifies the
// full flow: nil Done + Session → StartEventLoop → terminal event → ExecutionFinishedMsg.
func TestModel_Update_ExecutionStartedMsg_WithNilDone_FinishesViaEventLoop(t *testing.T) {
	m := New()

	eventChan := make(chan ports.Event, 2)
	finishedChan := make(chan ExecutionFinishedMsg, 1)

	// Wire sender: capture ExecutionFinishedMsg sent by the event-loop goroutine.
	m.tabMonitoring.SetSender(func(msg tea.Msg) {
		if fm, ok := msg.(ExecutionFinishedMsg); ok {
			finishedChan <- fm
		}
	})

	mockSess := &mockRunSession{eventsChan: eventChan}
	wf := &workflow.Workflow{Name: "wf", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}

	m.Update(ExecutionStartedMsg{
		ExecutionID: "exec-1",
		Workflow:    wf,
		Session:     mockSess,
	})

	// The event-loop goroutine is now running; send a terminal event.
	eventChan <- ports.Event{Seq: 1, Kind: ports.EventWorkflowCompleted, RunID: "exec-1"}
	close(eventChan)

	select {
	case fm := <-finishedChan:
		assert.Nil(t, fm.Err, "ExecutionFinishedMsg.Err must be nil for completed workflow")
	case <-time.After(300 * time.Millisecond):
		t.Fatal("StartEventLoop did not send ExecutionFinishedMsg within 300ms")
	}
}

// --- Help toggle ---

func TestModel_Update_HelpToggle_TogglesShowFullHelp(t *testing.T) {
	m := New()
	require.False(t, m.showFullHelp, "showFullHelp must start false")

	result, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	updated := result.(Model)
	assert.True(t, updated.showFullHelp, "pressing '?' should enable full help")
	assert.True(t, updated.help.ShowAll)

	result2, _ := updated.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	toggled := result2.(Model)
	assert.False(t, toggled.showFullHelp, "pressing '?' again should disable full help")
	assert.False(t, toggled.help.ShowAll)
}

func TestModel_View_ShowsHelpBar(t *testing.T) {
	m := New()

	output := m.View().Content

	// The short help bar must contain at least the quit key hint.
	assert.Contains(t, output, "q", "View should include help bar with quit keybinding")
}

func TestModel_ActiveHelpKeys_ReturnsCorrectKeyMap(t *testing.T) {
	tests := []struct {
		name      string
		activeTab Tab
		wantType  string
	}{
		{"workflows", TabWorkflows, "workflowsHelpKeys"},
		{"monitoring", TabMonitoring, "monitoringHelpKeys"},
		{"history", TabHistory, "historyHelpKeys"},
		{"logs", TabExternalLogs, "logsHelpKeys"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			m.activeTab = tt.activeTab
			km := m.activeHelpKeys()
			// Each help.KeyMap must return at least one short binding.
			assert.NotEmpty(t, km.ShortHelp(), "ShortHelp for %s should be non-empty", tt.name)
			// Each help.KeyMap must return at least one full help group.
			assert.NotEmpty(t, km.FullHelp(), "FullHelp for %s should be non-empty", tt.name)
		})
	}
}
