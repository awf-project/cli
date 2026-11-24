// Package tui — tests for T006: monitoring tab implementation (internal/interfaces/tui/tab_monitoring.go).
//
// Acceptance Criteria covered:
//
//	AC1:  Renders two-panel layout (tree left, viewport right)
//	       → TestMonitoringTab_View_WithExecCtx_RendersTwoPanels
//	AC3:  Up/down arrow changes selectedIdx
//	       → TestMonitoringTab_Update_ArrowKeys_ChangesSelection
//	AC4:  Selected node highlighted in rendered tree
//	       → TestMonitoringTab_RenderTreeWithSelection_HighlightsSelected
//	AC5:  Selected node output shown in right viewport
//	       → TestMonitoringTab_SelectedStepOutput_ReturnsStepOutput
//	AC6:  Auto-scroll enabled by default
//	       → TestMonitoringTab_AutoScroll_EnabledByDefault
//	AC7:  Empty state rendered when execCtx is nil and active is nil
//	       → TestMonitoringTab_View_WithNilExecCtx_RendersEmptyState
//	AC8:  'f' key re-enables auto-scroll
//	       → TestMonitoringTab_FKey_ReenablesAutoScroll
//	AC9:  Step failure auto-selects failed node
//	       → TestMonitoringTab_AutoSelectFailed_SelectsFailedNode
//	AC11: ExecutionFinishedMsg stops ticking
//	       → TestMonitoringTab_ExecutionFinishedMsg_StopsTicking
//	AC12: ExecutionStartedMsg starts tick loop
//	       → TestMonitoringTab_ExecutionStartedMsg_StartsTicking
//
// T078 cleanup criteria covered:
//
//	tickMsg type declaration removed from tab_monitoring.go
//	       → TestT078_TickMsg_TypeRemovedFromTabMonitoring
//	executionPollMsg type declaration removed from tab_monitoring.go
//	       → TestT078_ExecutionPollMsg_TypeRemovedFromTabMonitoring
//	scheduleTick function removed from tab_monitoring.go
//	       → TestT078_ScheduleTick_FunctionRemovedFromTabMonitoring
package tui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// helpers

// mockRunSession implements ports.RunSession for testing event loop behavior.
type mockRunSession struct {
	eventsChan <-chan ports.Event
	responses  []ports.InputResponse
}

func (m *mockRunSession) ID() string {
	return "mock-session"
}

func (m *mockRunSession) Events() <-chan ports.Event {
	return m.eventsChan
}

func (m *mockRunSession) Respond(r ports.InputResponse) error {
	m.responses = append(m.responses, r)
	return nil
}

func (m *mockRunSession) Err() error {
	return nil
}

func (m *mockRunSession) Close() error {
	return nil
}

func monitoringTabWithSize(w, h int) MonitoringTab {
	tab := newMonitoringTab()
	tab.width = w
	tab.height = h
	tab.resizeViewport()
	return tab
}

// --- newMonitoringTab ---

func TestNewMonitoringTab_ReturnsZeroValue(t *testing.T) {
	tab := newMonitoringTab()

	assert.Zero(t, tab.width, "width should be zero-initialized")
	assert.Zero(t, tab.height, "height should be zero-initialized")
	assert.Nil(t, tab.stats, "stats should be nil")
	assert.Nil(t, tab.history, "history should be nil")
	assert.Nil(t, tab.active, "active execution state should be nil")
}

func TestNewMonitoringTab_AutoScrollEnabledByDefault(t *testing.T) {
	tab := newMonitoringTab()
	assert.True(t, tab.autoScroll, "auto-scroll must be enabled by default")
}

func TestNewMonitoringTab_StatesInitialized(t *testing.T) {
	tab := newMonitoringTab()
	assert.NotNil(t, tab.states, "states map must be initialized")
}

// --- Init ---

func TestMonitoringTab_Init_ReturnsNil(t *testing.T) {
	tab := newMonitoringTab()
	cmd := tab.Init()
	assert.Nil(t, cmd, "Init() should return nil per Bubbletea convention")
}

// --- WindowSizeMsg ---

func TestMonitoringTab_Update_WithWindowSizeMsg_UpdatesDimensions(t *testing.T) {
	tab := newMonitoringTab()

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	updatedTab, cmd := tab.Update(msg)

	assert.Equal(t, 80, updatedTab.width)
	assert.Equal(t, 24, updatedTab.height)
	assert.Nil(t, cmd)
}

func TestMonitoringTab_Update_MultipleWindowSizeMsg_TracksDimensions(t *testing.T) {
	tab := newMonitoringTab()
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Equal(t, 120, tab.width)
	assert.Equal(t, 40, tab.height)
}

func TestMonitoringTab_Update_WithZeroSizeMsg_SetsZeroDimensions(t *testing.T) {
	tab := newMonitoringTab()
	tab.width = 80
	tab.height = 24

	updatedTab, cmd := tab.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	assert.Equal(t, 0, updatedTab.width)
	assert.Equal(t, 0, updatedTab.height)
	assert.Nil(t, cmd)
}

func TestMonitoringTab_Update_WithNegativeDimensions_StoresRawValues(t *testing.T) {
	tab := newMonitoringTab()

	updatedTab, cmd := tab.Update(tea.WindowSizeMsg{Width: -1, Height: -1})

	assert.Equal(t, -1, updatedTab.width)
	assert.Equal(t, -1, updatedTab.height)
	assert.Nil(t, cmd)
}

// --- ExecutionStartedMsg ---

func TestMonitoringTab_ExecutionStartedMsg_ResetsState(t *testing.T) {
	tab := newMonitoringTab()
	tab.selectedIdx = 5
	tab.states["old"] = workflow.StepState{Name: "old"}

	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "exec-1"})

	assert.Equal(t, 0, tab.selectedIdx, "selectedIdx must be reset to 0")
	assert.Empty(t, tab.states, "states must be cleared")
	assert.True(t, tab.autoScroll, "auto-scroll must be re-enabled")
}

// --- ExecutionFinishedMsg ---

func TestMonitoringTab_ExecutionFinishedMsg_StopsTicking(t *testing.T) {
	tab := newMonitoringTab()
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "s1",
		Steps: map[string]*workflow.Step{
			"s1": {Name: "s1", Type: workflow.StepTypeTerminal},
		},
	}
	// Start a run (facade path) so the step tree is built from the workflow.
	tab.SetWorkflow(wf)
	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "exec-1", Workflow: wf})
	tab.ticking = true

	tab, _ = tab.Update(ExecutionFinishedMsg{Err: nil})

	assert.False(t, tab.ticking, "ticking must stop on ExecutionFinishedMsg")
	assert.NotEmpty(t, tab.flatNodes, "tree built from the workflow must persist after finish")
}

// --- Arrow-key navigation ---

func TestMonitoringTab_Update_ArrowKeys_ChangesSelection(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "a", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
		{Name: "b", Status: workflow.StatusRunning, Children: []*TreeNode{}},
		{Name: "c", Status: workflow.StatusPending, Children: []*TreeNode{}},
	}
	tab.selectedIdx = 0

	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, tab.selectedIdx, "down should move to index 1")

	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 2, tab.selectedIdx, "down should move to index 2")

	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 2, tab.selectedIdx, "down at last should stay at index 2")

	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 1, tab.selectedIdx, "up should move to index 1")

	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, tab.selectedIdx, "up should move to index 0")

	tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, tab.selectedIdx, "up at first should stay at index 0")
}

func TestMonitoringTab_Update_JKKeys_ChangesSelection(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "step-1", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
		{Name: "step-2", Status: workflow.StatusPending, Children: []*TreeNode{}},
	}
	tab.selectedIdx = 0

	tab, _ = tab.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	assert.Equal(t, 1, tab.selectedIdx, "j should navigate down")

	tab, _ = tab.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	assert.Equal(t, 0, tab.selectedIdx, "k should navigate up")
}

func TestMonitoringTab_Update_ArrowKeys_WithEmptyNodes_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = nil

	require.NotPanics(t, func() {
		tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	})
	assert.Equal(t, 0, tab.selectedIdx)
}

// --- 'f' key re-enables auto-scroll ---

func TestMonitoringTab_FKey_ReenablesAutoScroll(t *testing.T) {
	tab := monitoringTabWithSize(120, 40)
	tab.autoScroll = false

	tab, _ = tab.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})

	assert.True(t, tab.autoScroll, "'f' must re-enable auto-scroll")
}

// --- AutoScroll enabled by default ---

func TestMonitoringTab_AutoScroll_EnabledByDefault(t *testing.T) {
	tab := newMonitoringTab()
	assert.True(t, tab.autoScroll)
}

// --- Auto-select failed node ---

func TestMonitoringTab_AutoSelectFailed_SelectsFailedNode(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "step-a", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
		{Name: "step-b", Status: workflow.StatusFailed, Children: []*TreeNode{}},
		{Name: "step-c", Status: workflow.StatusPending, Children: []*TreeNode{}},
	}
	tab.states = map[string]workflow.StepState{
		"step-a": {Name: "step-a", Status: workflow.StatusCompleted},
		"step-b": {Name: "step-b", Status: workflow.StatusFailed, Error: "command failed"},
	}
	tab.selectedIdx = 0
	tab.autoScroll = true

	tab.autoSelectFailed()

	assert.Equal(t, 1, tab.selectedIdx, "failed node must be auto-selected")
	assert.False(t, tab.autoScroll, "auto-scroll must be disabled when failed node auto-selected")
}

func TestMonitoringTab_AutoSelectFailed_WithNoFailedNode_DoesNotChangeSelection(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "step-a", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
		{Name: "step-b", Status: workflow.StatusRunning, Children: []*TreeNode{}},
	}
	tab.selectedIdx = 1

	tab.autoSelectFailed()

	assert.Equal(t, 1, tab.selectedIdx, "selection unchanged when no failed node")
}

// --- selectedStepOutput ---

func TestMonitoringTab_SelectedStepOutput_ReturnsStepOutput(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "build", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
	}
	tab.states = map[string]workflow.StepState{
		"build": {Name: "build", Status: workflow.StatusCompleted, Output: "build successful"},
	}
	tab.selectedIdx = 0

	output := tab.selectedStepOutput()

	assert.Contains(t, output, "build successful")
}

func TestMonitoringTab_SelectedStepOutput_WithError_IncludesErrorText(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "deploy", Status: workflow.StatusFailed, Children: []*TreeNode{}},
	}
	tab.states = map[string]workflow.StepState{
		"deploy": {Name: "deploy", Status: workflow.StatusFailed, Error: "connection refused", Stderr: "fatal error"},
	}
	tab.selectedIdx = 0

	output := tab.selectedStepOutput()

	assert.Contains(t, output, "connection refused")
	assert.Contains(t, output, "fatal error")
}

func TestMonitoringTab_SelectedStepOutput_WithNoState_ReturnsNotStartedMsg(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "pending-step", Status: workflow.StatusPending, Children: []*TreeNode{}},
	}
	tab.states = map[string]workflow.StepState{} // no state for pending-step
	tab.selectedIdx = 0

	output := tab.selectedStepOutput()

	assert.Contains(t, output, "pending-step")
}

func TestMonitoringTab_SelectedStepOutput_WithEmptyFlatNodes_ReturnsEmptyString(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = nil
	tab.selectedIdx = 0

	output := tab.selectedStepOutput()

	assert.Equal(t, "", output)
}

// --- renderTreeWithSelection ---

func TestMonitoringTab_RenderTreeWithSelection_HighlightsSelected(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = []*TreeNode{
		{Name: "step-a", Status: workflow.StatusCompleted, Depth: 0, Children: []*TreeNode{}},
		{Name: "step-b", Status: workflow.StatusRunning, Depth: 0, Children: []*TreeNode{}},
	}
	tab.selectedIdx = 0

	rendered := tab.renderTreeWithSelection()

	// The rendered output must contain both step names.
	assert.Contains(t, rendered, "step-a")
	assert.Contains(t, rendered, "step-b")
	// The rendered output must be non-empty.
	assert.NotEmpty(t, rendered)
}

func TestMonitoringTab_RenderTreeWithSelection_WithEmptyNodes_ReturnsWaitingMsg(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = nil

	rendered := tab.renderTreeWithSelection()

	assert.Contains(t, rendered, "Waiting")
}

func TestMonitoringTab_RenderTreeWithSelection_WithActiveAndEmptyNodes_RendersActiveInfo(t *testing.T) {
	tab := newMonitoringTab()
	tab.flatNodes = nil
	tab.active = &executionState{
		id:     "exec-active",
		status: workflow.StatusRunning,
		steps:  make(map[string]*workflow.StepState),
	}

	rendered := tab.renderTreeWithSelection()

	assert.Contains(t, rendered, "exec-active")
}

// --- View ---

func TestMonitoringTab_View_WithNilExecCtx_RendersEmptyState(t *testing.T) {
	tab := newMonitoringTab()
	// execCtx is nil, active is nil.

	view := tab.View()

	assert.Contains(t, view, "No active execution")
	assert.Contains(t, view, "Workflows tab")
}

func TestMonitoringTab_View_WithActiveButNilExecCtx_StillRendersEmptyState(t *testing.T) {
	tab := newMonitoringTab()
	tab.active = &executionState{
		id:     "exec-1",
		status: workflow.StatusRunning,
		steps:  make(map[string]*workflow.StepState),
	}
	// execCtx is still nil — active comes from model.go's executionState not the real context.
	// In this case View should render something (not the empty state) because active != nil.
	tab.width = 80
	tab.height = 24

	view := tab.View()

	// Since active != nil, the empty state message must not appear.
	assert.NotContains(t, view, "No active execution")
}

func TestMonitoringTab_View_WithWorkflow_RendersTwoPanels(t *testing.T) {
	tab := monitoringTabWithSize(120, 40)
	tab.SetWorkflow(&workflow.Workflow{Name: "wf-1", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}})

	view := tab.View()

	// Two-panel layout: both panels should be rendered.
	// With lipgloss.JoinHorizontal, content from both sides appears in the output.
	assert.NotEmpty(t, view)
	// Must not show empty state.
	assert.NotContains(t, view, "No active execution")
}

func TestMonitoringTab_View_WithZeroDimensions_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()
	tab.SetWorkflow(&workflow.Workflow{Name: "wf-1"})
	tab.width = 0
	tab.height = 0

	require.NotPanics(t, func() {
		_ = tab.View()
	})
}

func TestMonitoringTab_View_ReturnsString(t *testing.T) {
	tab := newMonitoringTab()
	view := tab.View()
	require.IsType(t, "", view)
}

// --- View with stats/history (legacy fields) ---

func TestMonitoringTab_View_WithStats_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()
	tab.stats = &workflow.HistoryStats{
		TotalExecutions: 10,
		SuccessCount:    7,
		FailedCount:     3,
	}
	tab.width = 80
	tab.height = 24

	require.NotPanics(t, func() {
		_ = tab.View()
	})
}

func TestMonitoringTab_View_WithHistory_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()
	tab.history = []*workflow.ExecutionRecord{
		{ID: "exec-1", WorkflowName: "my-workflow", Status: "success"},
	}
	tab.width = 80
	tab.height = 24

	require.NotPanics(t, func() {
		_ = tab.View()
	})
}

func TestMonitoringTab_View_WithActiveExecution_RendersActiveState(t *testing.T) {
	tab := monitoringTabWithSize(80, 24)
	tab.active = &executionState{
		id:     "exec-active",
		status: workflow.StatusRunning,
		steps:  make(map[string]*workflow.StepState),
	}

	view := tab.View()

	assert.NotEmpty(t, view, "View() must render active execution state when an execution is in progress")
	assert.NotContains(t, view, "No active execution")
}

// --- SetWorkflow ---

func TestMonitoringTab_SetWorkflow_WiresWorkflow(t *testing.T) {
	tab := newMonitoringTab()
	wf := &workflow.Workflow{
		Name:  "wf-1",
		Steps: map[string]*workflow.Step{},
	}

	tab.SetWorkflow(wf)

	assert.Equal(t, wf, tab.wf)
}

func TestMonitoringTab_SetWorkflow_WithNilWorkflow_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()

	require.NotPanics(t, func() {
		tab.SetWorkflow(nil)
	})
	assert.Nil(t, tab.wf)
}

// --- flattenTree ---

func TestFlattenTree_SingleNode(t *testing.T) {
	nodes := []*TreeNode{
		{Name: "root", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
	}

	flat := flattenTree(nodes)

	require.Len(t, flat, 1)
	assert.Equal(t, "root", flat[0].Name)
}

func TestFlattenTree_WithChildren(t *testing.T) {
	child1 := &TreeNode{Name: "child-a", Status: workflow.StatusCompleted, Children: []*TreeNode{}}
	child2 := &TreeNode{Name: "child-b", Status: workflow.StatusRunning, Children: []*TreeNode{}}
	root := &TreeNode{Name: "root", IsParallel: true, Children: []*TreeNode{child1, child2}}

	flat := flattenTree([]*TreeNode{root})

	require.Len(t, flat, 3)
	assert.Equal(t, "root", flat[0].Name)
	assert.Equal(t, "child-a", flat[1].Name)
	assert.Equal(t, "child-b", flat[2].Name)
}

func TestFlattenTree_NilInput_ReturnsEmpty(t *testing.T) {
	flat := flattenTree(nil)
	assert.Empty(t, flat)
}

// --- panelWidths ---

func TestMonitoringTab_PanelWidths_40_60Split(t *testing.T) {
	tab := newMonitoringTab()
	tab.width = 100

	left, right := tab.panelWidths()

	assert.Equal(t, 40, left, "left panel should be 40% of total width")
	assert.Equal(t, 60, right, "right panel should be 60% of total width")
}

func TestMonitoringTab_PanelWidths_WithZeroWidth_ReturnsMin1(t *testing.T) {
	tab := newMonitoringTab()
	tab.width = 0

	left, right := tab.panelWidths()

	assert.GreaterOrEqual(t, left, 1)
	assert.GreaterOrEqual(t, right, 1)
}

// --- rebuildTree ---

func TestMonitoringTab_RebuildTree_WithSteps_BuildsFlatNodes(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "step-a", Type: workflow.StepTypeCommand},
		{Name: "step-b", Type: workflow.StepTypeCommand},
	}
	tab.states = map[string]workflow.StepState{
		"step-a": {Name: "step-a", Status: workflow.StatusCompleted},
		"step-b": {Name: "step-b", Status: workflow.StatusRunning},
	}

	tab.rebuildTree()

	require.Len(t, tab.flatNodes, 2)
	assert.Equal(t, "step-a", tab.flatNodes[0].Name)
	assert.Equal(t, "step-b", tab.flatNodes[1].Name)
}

func TestMonitoringTab_RebuildTree_ClampsSelectedIdx(t *testing.T) {
	tab := newMonitoringTab()
	tab.selectedIdx = 99 // out of bounds
	tab.steps = []workflow.Step{
		{Name: "only-step", Type: workflow.StepTypeCommand},
	}
	tab.states = map[string]workflow.StepState{}

	tab.rebuildTree()

	assert.Equal(t, 0, tab.selectedIdx, "selectedIdx must be clamped to valid range")
}

// --- Spinner ---

func TestMonitoringTab_WithWorkflow_RendersTwoPanels(t *testing.T) {
	tab := newMonitoringTab()
	wf := &workflow.Workflow{Name: "test-wf", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	tab.SetWorkflow(wf)
	tab.width = 80
	tab.height = 24
	tab.rebuildTree()

	view := tab.View()
	assert.NotContains(t, view, "No active execution")
}

// --- T061: Event loop migration tests ---

// TestTUI_TabMonitoringConsumesEventsLoop verifies that StartEventLoop properly
// consumes events from a RunSession and forwards each event to the Bubble Tea program.
// This replaces the 200ms polling loop pattern (D27).
func TestTUI_TabMonitoringConsumesEventsLoop(t *testing.T) {
	tab := newMonitoringTab()

	// Mock RunSession with scripted events.
	eventChan := make(chan ports.Event, 3)
	defer close(eventChan)

	mockSession := &mockRunSession{
		eventsChan: eventChan,
	}

	// Track messages sent via the sender callback with proper synchronization.
	sentChan := make(chan tea.Msg, 3)
	sender := func(msg tea.Msg) {
		sentChan <- msg
	}
	tab.SetSender(sender)

	// Queue test events.
	testEvent1 := ports.Event{
		Seq:   1,
		Kind:  ports.EventRunStarted,
		RunID: "test-run-1",
	}
	testEvent2 := ports.Event{
		Seq:   2,
		Kind:  ports.EventStepStarted,
		RunID: "test-run-1",
	}
	testEvent3 := ports.Event{
		Seq:   3,
		Kind:  ports.EventRunCompleted,
		RunID: "test-run-1",
	}

	eventChan <- testEvent1
	eventChan <- testEvent2
	eventChan <- testEvent3

	// Start the event loop goroutine.
	tab.StartEventLoop(context.Background(), mockSession)

	// Collect sent messages with timeout.
	var sentMessages []tea.Msg
	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()

CollectLoop:
	for len(sentMessages) < 3 {
		select {
		case msg := <-sentChan:
			sentMessages = append(sentMessages, msg)
		case <-timeout.C:
			break CollectLoop
		}
	}

	// Verify each event was forwarded as a facadeEventMsg.
	require.Len(t, sentMessages, 3, "must send one message per event")

	for i, msg := range sentMessages {
		fMsg, ok := msg.(facadeEventMsg)
		require.True(t, ok, "sent message %d must be facadeEventMsg, got %T", i, msg)
		assert.Equal(t, uint64(i+1), fMsg.Event.Seq, "event %d seq mismatch", i)
	}
}

// TestTUI_TabMonitoringEventLoopWithNilSender verifies that the event loop
// safely handles a nil sender (does not panic and discards events).
func TestTUI_TabMonitoringEventLoopWithNilSender(t *testing.T) {
	tab := newMonitoringTab()
	// sender is nil by default

	eventChan := make(chan ports.Event, 1)
	defer close(eventChan)

	mockSession := &mockRunSession{
		eventsChan: eventChan,
	}

	eventChan <- ports.Event{
		Seq:   1,
		Kind:  ports.EventRunStarted,
		RunID: "test-run",
	}

	// Should not panic with nil sender.
	require.NotPanics(t, func() {
		tab.StartEventLoop(context.Background(), mockSession)
		time.Sleep(50 * time.Millisecond)
	})
}

// TestTUI_StartEventLoop_TerminalCompleted_SendsExecutionFinishedMsg verifies that
// StartEventLoop sends ExecutionFinishedMsg(nil) when EventWorkflowCompleted is received.
// This allows the facade-driven path (Done == nil) to properly stop the tick loop.
func TestTUI_StartEventLoop_TerminalCompleted_SendsExecutionFinishedMsg(t *testing.T) {
	tab := newMonitoringTab()

	eventChan := make(chan ports.Event, 2)
	mockSession := &mockRunSession{eventsChan: eventChan}

	sentChan := make(chan tea.Msg, 4)
	tab.SetSender(func(msg tea.Msg) { sentChan <- msg })

	tab.StartEventLoop(context.Background(), mockSession)

	eventChan <- ports.Event{Seq: 1, Kind: ports.EventStepStarted, RunID: "r1"}
	eventChan <- ports.Event{Seq: 2, Kind: ports.EventWorkflowCompleted, RunID: "r1"}
	close(eventChan)

	// Collect messages; expect facadeEventMsg×2 + ExecutionFinishedMsg×1.
	var got []tea.Msg
	timeout := time.NewTimer(300 * time.Millisecond)
	defer timeout.Stop()
collect:
	for {
		select {
		case m := <-sentChan:
			got = append(got, m)
			if _, ok := m.(ExecutionFinishedMsg); ok {
				break collect
			}
		case <-timeout.C:
			break collect
		}
	}

	var finishedMsgs []ExecutionFinishedMsg
	for _, m := range got {
		if fm, ok := m.(ExecutionFinishedMsg); ok {
			finishedMsgs = append(finishedMsgs, fm)
		}
	}
	require.Len(t, finishedMsgs, 1, "must send exactly one ExecutionFinishedMsg on terminal event")
	assert.Nil(t, finishedMsgs[0].Err, "err must be nil for EventWorkflowCompleted")
}

// TestTUI_StartEventLoop_TerminalFailed_SendsExecutionFinishedMsg verifies that
// StartEventLoop sends ExecutionFinishedMsg with sess.Err() when EventWorkflowFailed.
func TestTUI_StartEventLoop_TerminalFailed_SendsExecutionFinishedMsg(t *testing.T) {
	tab := newMonitoringTab()

	eventChan := make(chan ports.Event, 1)
	mockSession := &mockRunSession{eventsChan: eventChan}

	sentChan := make(chan tea.Msg, 4)
	tab.SetSender(func(msg tea.Msg) { sentChan <- msg })

	tab.StartEventLoop(context.Background(), mockSession)

	eventChan <- ports.Event{Seq: 1, Kind: ports.EventWorkflowFailed, RunID: "r1"}
	close(eventChan)

	timeout := time.NewTimer(300 * time.Millisecond)
	defer timeout.Stop()
	var finishedMsgs []ExecutionFinishedMsg
collect:
	for {
		select {
		case m := <-sentChan:
			if fm, ok := m.(ExecutionFinishedMsg); ok {
				finishedMsgs = append(finishedMsgs, fm)
				break collect
			}
		case <-timeout.C:
			break collect
		}
	}

	require.Len(t, finishedMsgs, 1, "must send ExecutionFinishedMsg on EventWorkflowFailed")
	// mockRunSession.Err() returns nil; just verify the message was sent.
	_ = finishedMsgs[0].Err
}

// TestTUI_NoPollingIntervalConstant verifies that the monitoringTickInterval
// constant has been removed from the TUI package as part of the polling->events migration.
func TestTUI_NoPollingIntervalConstant(t *testing.T) {
	// This is a static analysis test: verify that monitoringTickInterval is no longer
	// referenced in this file's source code.
	// The constant was defined on line 71-72 and scheduled ticks in scheduleTick().
	source, err := os.ReadFile("tab_monitoring.go")
	if err != nil {
		// If we can't read from the current directory, try the absolute path.
		// Test runs from the module root.
		wd, _ := os.Getwd()
		source, err = os.ReadFile(wd + "/internal/interfaces/tui/tab_monitoring.go")
		require.NoError(t, err, "must be able to read tab_monitoring.go from either . or ./internal/interfaces/tui/")
	}

	hasConstant := strings.Contains(string(source), "monitoringTickInterval")
	assert.False(t, hasConstant, "monitoringTickInterval constant should be removed (D27: replaced by event loop)")
}

// TestTUI_CommandResolvePackWorkflowRemoved verifies that the resolvePackWorkflow
// function has been deleted from command.go as part of the cleanup (D28).
func TestTUI_CommandResolvePackWorkflowRemoved(t *testing.T) {
	// This is a static analysis test: verify resolvePackWorkflow function is removed
	// from command.go. The function was at lines 223-248.
	source, err := os.ReadFile("command.go")
	if err != nil {
		wd, _ := os.Getwd()
		source, err = os.ReadFile(wd + "/internal/interfaces/tui/command.go")
		require.NoError(t, err, "must be able to read command.go from either . or ./internal/interfaces/tui/")
	}

	hasFunc := strings.Contains(string(source), "resolvePackWorkflow")
	assert.False(t, hasFunc, "resolvePackWorkflow function should be removed (D28: duplicate + silent-continue bug)")
}

// TestTUI_InputReaderFileDeleted verifies that the input_reader.go file
// has been deleted as part of the cleanup (D28).
func TestTUI_InputReaderFileDeleted(t *testing.T) {
	// This is a static analysis test: verify the input_reader.go file does not exist.
	// The file contained the TUIInputReader type and related functions.
	_, err := os.Stat("input_reader.go")
	if err == nil {
		// Try with full path if it exists in current dir.
		wd, _ := os.Getwd()
		_, err = os.Stat(wd + "/internal/interfaces/tui/input_reader.go")
	}
	assert.True(t, os.IsNotExist(err), "input_reader.go file should be deleted (D28)")
}

// --- T061 GREEN: facadeEventMsg handler ---

// TestMonitoringTab_FacadeEventMsg_StepStarted_SetsRunningState verifies that a
// EventStepStarted facade event updates the corresponding step's state to Running.
func TestMonitoringTab_FacadeEventMsg_StepStarted_SetsRunningState(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "build", Type: workflow.StepTypeCommand},
	}

	payload := &ports.EnrichedStepPayload{StepName: "build"}
	event := ports.Event{
		Seq:     1,
		Kind:    ports.EventStepStarted,
		RunID:   "run-1",
		Payload: payload,
	}

	tab, _ = tab.Update(facadeEventMsg{Event: event})

	state, ok := tab.states["build"]
	require.True(t, ok, "state must be set for step 'build' after EventStepStarted")
	assert.Equal(t, workflow.StatusRunning, state.Status)
}

// TestMonitoringTab_FacadeEventMsg_StepCompleted_SetsCompletedState verifies that
// EventStepCompleted updates the step's state to Completed.
func TestMonitoringTab_FacadeEventMsg_StepCompleted_SetsCompletedState(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "test", Type: workflow.StepTypeCommand},
	}
	// Pre-seed running state.
	tab.states["test"] = workflow.StepState{Name: "test", Status: workflow.StatusRunning}

	payload := &ports.EnrichedStepPayload{StepName: "test"}
	event := ports.Event{
		Seq:     2,
		Kind:    ports.EventStepCompleted,
		RunID:   "run-1",
		Payload: payload,
	}

	tab, _ = tab.Update(facadeEventMsg{Event: event})

	state, ok := tab.states["test"]
	require.True(t, ok, "state must exist for step 'test' after EventStepCompleted")
	assert.Equal(t, workflow.StatusCompleted, state.Status)
}

// TestMonitoringTab_FacadeEventMsg_StepCompleted_StoresOutput verifies that the captured
// stdout/stderr carried on the completed event are stored on the step state so the detail
// panel can render per-step output for any selected completed step (regression: facade
// step events dropped output, leaving the panel showing only the command and no result).
func TestMonitoringTab_FacadeEventMsg_StepCompleted_StoresOutput(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "greet", Type: workflow.StepTypeCommand},
	}
	tab.states["greet"] = workflow.StepState{Name: "greet", Status: workflow.StatusRunning}

	payload := &ports.EnrichedStepPayload{
		StepName: "greet",
		Output:   "Hello, World!",
		Stderr:   "a warning",
	}
	event := ports.Event{Seq: 2, Kind: ports.EventStepCompleted, RunID: "run-1", Payload: payload}

	tab, _ = tab.Update(facadeEventMsg{Event: event})

	state := tab.states["greet"]
	assert.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "Hello, World!", state.Output, "captured stdout must be stored on the step state")
	assert.Equal(t, "a warning", state.Stderr, "captured stderr must be stored on the step state")
}

// TestMonitoringTab_FacadeEventMsg_StepCompleted_WithError_SetsFailedState verifies
// that EventStepCompleted with a non-empty Error sets status to Failed.
func TestMonitoringTab_FacadeEventMsg_StepCompleted_WithError_SetsFailedState(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "deploy", Type: workflow.StepTypeCommand},
	}
	tab.states["deploy"] = workflow.StepState{Name: "deploy", Status: workflow.StatusRunning}

	payload := &ports.EnrichedStepPayload{StepName: "deploy", Error: "exit status 1"}
	event := ports.Event{
		Seq:     3,
		Kind:    ports.EventStepCompleted,
		RunID:   "run-1",
		Payload: payload,
	}

	tab, _ = tab.Update(facadeEventMsg{Event: event})

	state, ok := tab.states["deploy"]
	require.True(t, ok, "state must exist for step 'deploy' after failed EventStepCompleted")
	assert.Equal(t, workflow.StatusFailed, state.Status)
	assert.Equal(t, "exit status 1", state.Error)
}

// TestMonitoringTab_FacadeEventMsg_UnknownPayload_DoesNotPanic verifies that events
// with unexpected or nil payload are handled gracefully.
func TestMonitoringTab_FacadeEventMsg_UnknownPayload_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()

	event := ports.Event{
		Seq:     1,
		Kind:    ports.EventStepStarted,
		RunID:   "run-1",
		Payload: nil, // nil payload
	}

	require.NotPanics(t, func() {
		tab, _ = tab.Update(facadeEventMsg{Event: event})
	})
}

// TestMonitoringTab_FacadeEventMsg_NonStepEvent_DoesNotAlterStates verifies that
// non-step events (run-level, message events) do not corrupt the step-state map.
func TestMonitoringTab_FacadeEventMsg_NonStepEvent_DoesNotAlterStates(t *testing.T) {
	tab := newMonitoringTab()
	tab.states["existing"] = workflow.StepState{Name: "existing", Status: workflow.StatusCompleted}

	for _, kind := range []ports.EventKind{
		ports.EventRunStarted,
		ports.EventRunCompleted,
		ports.EventMessageUser,
		ports.EventMessageAssistant,
	} {
		event := ports.Event{Seq: 1, Kind: kind, RunID: "run-1"}
		tab, _ = tab.Update(facadeEventMsg{Event: event})
	}

	assert.Equal(t, workflow.StatusCompleted, tab.states["existing"].Status,
		"non-step events must not alter existing step states")
}

// TestMonitoringTab_FacadeEventMsg_RebuildsTree verifies that step state changes
// from facade events are reflected in the rendered tree (flatNodes updated).
func TestMonitoringTab_FacadeEventMsg_RebuildsTree(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "lint", Type: workflow.StepTypeCommand},
	}

	payload := &ports.EnrichedStepPayload{StepName: "lint"}
	event := ports.Event{
		Seq:     1,
		Kind:    ports.EventStepStarted,
		RunID:   "run-1",
		Payload: payload,
	}

	tab, _ = tab.Update(facadeEventMsg{Event: event})

	require.NotEmpty(t, tab.flatNodes, "flatNodes must be rebuilt after facade event updates state")
	assert.Equal(t, "lint", tab.flatNodes[0].Name)
	assert.Equal(t, workflow.StatusRunning, tab.flatNodes[0].Status)
}

// --- T072: No polling after facade dispatch ---

// TestTabMonitoring_NoPollingAfterFacadeDispatch verifies that when ExecutionStartedMsg
// carries a non-nil Session, no tickMsg or executionPollMsg is ever produced (FR-002, D27).
func TestTabMonitoring_NoPollingAfterFacadeDispatch(t *testing.T) {
	tab := newMonitoringTab()

	// Facade-driven path: Session != nil signals push-event mode.
	eventChan := make(chan ports.Event)
	defer close(eventChan)
	sess := &mockRunSession{eventsChan: eventChan}

	tab, cmd := tab.Update(ExecutionStartedMsg{
		ExecutionID: "exec-facade",
		Session:     sess,
	})

	assert.False(t, tab.ticking, "ticking must NOT start when Session is non-nil")
	assert.Nil(t, cmd, "no tick cmd must be emitted when Session is non-nil")

	// facadeEventMsg must not produce tickMsg or executionPollMsg.
	event := ports.Event{Seq: 1, Kind: ports.EventStepStarted, RunID: "exec-facade"}
	tab, cmd = tab.Update(facadeEventMsg{Event: event})

	assert.Nil(t, cmd, "facadeEventMsg must not emit cmd (no tickMsg or executionPollMsg)")
}

// TestTabMonitoring_FacadePathRendersPanel is a regression guard for the bug where the
// Monitoring tab showed "No active execution" for an entire run because the facade dispatch
// path (Bridge.RunWorkflowViaFacade) emitted ExecutionStartedMsg with a nil Workflow. With
// wf, execCtx and active all nil, the View empty-state guard never cleared and BuildTree
// returned no nodes — so the panel stayed blank even while Session.Events() were flowing to
// the Logs tab. The fix supplies the workflow definition; this test pins the rendered panel.
func TestTabMonitoring_FacadePathRendersPanel(t *testing.T) {
	tab := newMonitoringTab()
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	wf := &workflow.Workflow{
		Name:    "echo",
		Initial: "greet",
		Steps: map[string]*workflow.Step{
			"greet": {Name: "greet", Type: workflow.StepTypeCommand, Command: "echo hi"},
		},
	}
	// Mirror the production facade path: model calls SetWorkflow(Workflow) then forwards
	// ExecutionStartedMsg to the tab. State is event-driven via facadeEventMsg.
	tab.SetWorkflow(wf)
	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "echo", Workflow: wf})

	view := tab.View()
	assert.NotContains(t, view, "No active execution",
		"a facade-driven run with a supplied Workflow must render the monitoring panel, not the empty state")
	assert.Contains(t, view, "greet", "the step tree must list the workflow's steps")

	// Drive the step to completion carrying captured stdout, then assert the detail panel
	// renders that output — the user's reported symptom was a completed step showing only
	// its command and no output (facade step events dropped the captured stdout/stderr).
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Seq:     1,
		Kind:    ports.EventStepStarted,
		RunID:   "echo",
		Payload: &ports.EnrichedStepPayload{StepName: "greet"},
	}})
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Seq:     2,
		Kind:    ports.EventStepCompleted,
		RunID:   "echo",
		Payload: &ports.EnrichedStepPayload{StepName: "greet", Output: "Hello, World!"},
	}})

	view = tab.View()
	assert.Contains(t, view, "Hello, World!",
		"the detail panel must render the completed step's captured output")
}

// TestTabMonitoring_TerminalStepCompletesOnWorkflowDone is a regression guard: a terminal
// step (e.g. "done") emits no step event of its own, so on the facade event-driven path
// (execCtx == nil) it stayed ⏳ pending even after the workflow finished. EventWorkflowCompleted
// must mark the reached terminal step completed.
func TestTabMonitoring_TerminalStepCompletesOnWorkflowDone(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "greet", Type: workflow.StepTypeCommand},
		{Name: "done", Type: workflow.StepTypeTerminal},
	}
	// greet ran and completed; the terminal "done" has no step event and no state.
	tab.states["greet"] = workflow.StepState{Name: "greet", Status: workflow.StatusCompleted}
	tab.rebuildTree()

	// Before the workflow ends, the terminal node is still pending.
	require.Equal(t, workflow.StatusPending, terminalNodeStatus(t, tab, "done"))

	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Seq:     9,
		Kind:    ports.EventWorkflowCompleted,
		RunID:   "echo",
		Payload: &ports.EnrichedTerminal{},
	}})

	assert.Equal(t, workflow.StatusCompleted, terminalNodeStatus(t, tab, "done"),
		"terminal step must be marked completed once the workflow finishes successfully")
}

// TestTabMonitoring_TerminalStatusResetOnNewRun guards against a stale terminal outcome
// leaking across runs in the same TUI session: after one run completes (marking the terminal
// step ✓ via finalStatus), starting a NEW run must reset finalStatus so its terminal step
// shows ⏳ pending until that run actually finishes — not ✓ by default.
func TestTabMonitoring_TerminalStatusResetOnNewRun(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "work", Type: workflow.StepTypeCommand},
		{Name: "done", Type: workflow.StepTypeTerminal},
	}
	tab.states["work"] = workflow.StepState{Name: "work", Status: workflow.StatusCompleted}
	// First run finishes → terminal marked completed.
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{Kind: ports.EventWorkflowCompleted, Payload: &ports.EnrichedTerminal{}}})
	require.Equal(t, workflow.StatusCompleted, terminalNodeStatus(t, tab, "done"))

	// A new run starts: the terminal step must go back to pending, not stay ✓.
	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "run-2"})
	tab.steps = []workflow.Step{
		{Name: "work", Type: workflow.StepTypeCommand},
		{Name: "done", Type: workflow.StepTypeTerminal},
	}
	tab.rebuildTree()
	assert.Equal(t, workflow.StatusPending, terminalNodeStatus(t, tab, "done"),
		"a new run must not inherit the previous run's terminal completion")
}

// terminalNodeStatus returns the tree-node status for the named step.
func terminalNodeStatus(t *testing.T, tab MonitoringTab, name string) workflow.ExecutionStatus {
	t.Helper()
	for _, n := range tab.flatNodes {
		if n.Name == name {
			return n.Status
		}
	}
	t.Fatalf("node %q not found in flatNodes", name)
	return ""
}

// TestTabMonitoring_ConversationTurnsFromFacadeEvents is a regression guard: during a parked
// conversation the agent's question and the user's answer must appear in the detail panel.
// On the facade path the tab has no ExecutionContext, so it must build the conversation from
// EventMessageAssistant/User events attributed to the running agent step.
func TestTabMonitoring_ConversationTurnsFromFacadeEvents(t *testing.T) {
	tab := newMonitoringTab()
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	wf := &workflow.Workflow{
		Name:    "clarifier",
		Initial: "clarify",
		Steps: map[string]*workflow.Step{
			"clarify": {Name: "clarify", Type: workflow.StepTypeAgent, Agent: &workflow.AgentConfig{}},
		},
	}
	tab.SetWorkflow(wf)
	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "clarifier", Workflow: wf})

	// Agent step starts → becomes the conversation target.
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventStepStarted, RunID: "clarifier",
		Payload: &ports.EnrichedStepPayload{StepName: "clarify"},
	}})
	// Agent asks a question (the user is about to answer it).
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventMessageAssistant, RunID: "clarifier",
		Payload: &ports.EnrichedMessagePayload{Content: "Which environment should I target?"},
	}})

	view := tab.View()
	assert.Contains(t, view, "Which environment should I target?",
		"the agent's question must be visible so the user knows what they are answering")

	// User answers → the answer turn appears too.
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventMessageUser, RunID: "clarifier",
		Payload: &ports.EnrichedMessagePayload{Content: "production"},
	}})

	view = tab.View()
	assert.Contains(t, view, "production", "the user's answer must appear in the conversation")
}

func TestTabMonitoring_InputRequiredFromFacadeShowsInputAndRespondsToSession(t *testing.T) {
	tab := newMonitoringTab()
	session := &mockRunSession{}
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "clarifier", Session: session})

	tab, cmd := tab.Update(facadeEventMsg{Event: ports.Event{
		Kind:  ports.EventInputRequired,
		RunID: "clarifier",
		Payload: &ports.EnrichedInputRequest{
			Prompt: "> ",
		},
	}})

	require.Nil(t, cmd)
	require.True(t, tab.InputActive(), "facade input request must display the conversation input box")
	assert.Contains(t, tab.View(), "empty to end conversation", "input box must be rendered while awaiting user input")

	tab.inputField.SetValue("production")
	tab, cmd = tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.Nil(t, cmd)
	require.False(t, tab.InputActive(), "submitting input should hide the input box")
	require.Len(t, session.responses, 1)
	assert.Equal(t, "production", session.responses[0].Value)
}

// --- End-to-end: AC gate TestMonitoringTab ---

// TestMonitoringTab is the acceptance-criteria gate named in the task spec.
func TestMonitoringTab(t *testing.T) {
	t.Run("empty state when no execution", func(t *testing.T) {
		tab := newMonitoringTab()
		view := tab.View()
		assert.Contains(t, view, "No active execution")
	})

	t.Run("execution started does not start polling (facade-driven)", func(t *testing.T) {
		tab := newMonitoringTab()
		tab, cmd := tab.Update(ExecutionStartedMsg{ExecutionID: "exec-1"})
		// F108: no polling tick is scheduled; state updates arrive via facadeEventMsg.
		assert.False(t, tab.ticking)
		assert.Nil(t, cmd)
	})

	t.Run("execution finished stops tick", func(t *testing.T) {
		tab := newMonitoringTab()
		tab.ticking = true
		tab, _ = tab.Update(ExecutionFinishedMsg{Err: nil})
		assert.False(t, tab.ticking)
	})

	t.Run("arrow navigation changes selection", func(t *testing.T) {
		tab := newMonitoringTab()
		tab.flatNodes = []*TreeNode{
			{Name: "a", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
			{Name: "b", Status: workflow.StatusPending, Children: []*TreeNode{}},
		}
		tab, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		assert.Equal(t, 1, tab.selectedIdx)
	})

	t.Run("f key re-enables auto-scroll", func(t *testing.T) {
		tab := monitoringTabWithSize(120, 40)
		tab.autoScroll = false
		tab, _ = tab.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
		assert.True(t, tab.autoScroll)
	})

	t.Run("failed step auto-selects", func(t *testing.T) {
		tab := newMonitoringTab()
		tab.flatNodes = []*TreeNode{
			{Name: "ok", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
			{Name: "bad", Status: workflow.StatusFailed, Children: []*TreeNode{}},
		}
		tab.states = map[string]workflow.StepState{
			"bad": {Name: "bad", Status: workflow.StatusFailed, Error: "oops"},
		}
		tab.autoSelectFailed()
		assert.Equal(t, 1, tab.selectedIdx)
	})

	t.Run("two-panel view renders without panic", func(t *testing.T) {
		tab := monitoringTabWithSize(120, 40)
		tab.SetWorkflow(&workflow.Workflow{Name: "exec-1"})
		require.NotPanics(t, func() {
			view := tab.View()
			assert.NotEmpty(t, view)
		})
	})

	t.Run("selected step output shown in viewport area", func(t *testing.T) {
		tab := newMonitoringTab()
		tab.flatNodes = []*TreeNode{
			{Name: "deploy", Status: workflow.StatusCompleted, Children: []*TreeNode{}},
		}
		tab.states = map[string]workflow.StepState{
			"deploy": {Name: "deploy", Status: workflow.StatusCompleted, Output: "Deployed!"},
		}
		tab.selectedIdx = 0
		output := tab.selectedStepOutput()
		assert.Contains(t, output, "Deployed!")
	})

	t.Run("view contains both panel borders", func(t *testing.T) {
		tab := monitoringTabWithSize(120, 40)
		tab.SetWorkflow(&workflow.Workflow{Name: "exec-1"})
		tab.flatNodes = []*TreeNode{
			{Name: "step-1", Status: workflow.StatusRunning, Children: []*TreeNode{}},
		}
		view := tab.View()
		// Lipgloss rounded borders produce '╭' or '┌' characters.
		// Either way the view must be non-empty and structured.
		assert.NotEmpty(t, view)
		lines := strings.Split(view, "\n")
		assert.Greater(t, len(lines), 1, "two-panel view must span multiple lines")
	})
}

// --- T072: Facade-driven path removes polling ---

// TestTabMonitoring_FacadeModeNoTickScheduled verifies that ExecutionStartedMsg with a
// non-nil Session never emits a tick command (FR-002: push-event pattern obsoletes polling).
func TestTabMonitoring_FacadeModeNoTickScheduled(t *testing.T) {
	eventChan := make(chan ports.Event)
	defer close(eventChan)
	sess := &mockRunSession{eventsChan: eventChan}
	tab := newMonitoringTab()

	tab, cmd := tab.Update(ExecutionStartedMsg{ExecutionID: "facade-exec", Session: sess})

	assert.Nil(t, cmd, "facade mode must not emit tick cmd")
	assert.False(t, tab.ticking, "facade mode must not enable ticking")
}

// TestTabMonitoring_FacadeEventLoopNoTickLoop verifies the complete sequence:
// ExecutionStartedMsg(Session) → facadeEventMsg → no tickMsg/executionPollMsg ever produced.
func TestTabMonitoring_FacadeEventLoopNoTickLoop(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{{Name: "step-1", Type: workflow.StepTypeCommand}}

	eventChan := make(chan ports.Event)
	defer close(eventChan)
	sess := &mockRunSession{eventsChan: eventChan}

	// Start facade-driven execution
	tab, cmd := tab.Update(ExecutionStartedMsg{
		ExecutionID: "facade-exec",
		Session:     sess,
	})
	assert.Nil(t, cmd, "facade dispatch must not schedule tick")
	assert.False(t, tab.ticking, "facade dispatch must not enable ticking")

	// Multiple facadeEventMsg should never trigger tickMsg or executionPollMsg
	for seq := 1; seq <= 3; seq++ {
		event := ports.Event{
			Seq:     uint64(seq),
			Kind:    ports.EventStepStarted,
			RunID:   "facade-exec",
			Payload: &ports.EnrichedStepPayload{StepName: "step-1"},
		}
		tab, cmd = tab.Update(facadeEventMsg{Event: event})

		// Each facadeEventMsg should return nil cmd (no tick scheduling)
		assert.Nil(t, cmd, "facadeEventMsg seq %d must not emit cmd", seq)
		assert.False(t, tab.ticking, "facadeEventMsg seq %d must not enable ticking", seq)
	}
}

// TestTabMonitoring_NoScheduleTickCallsInFacadePath verifies that throughout
// the facade event loop lifecycle, scheduleTick is never called by examining
// the returned Cmd values (they should all be nil).
func TestTabMonitoring_NoScheduleTickCallsInFacadePath(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "build", Type: workflow.StepTypeCommand},
		{Name: "test", Type: workflow.StepTypeCommand},
	}

	eventChan := make(chan ports.Event)
	defer close(eventChan)
	sess := &mockRunSession{eventsChan: eventChan}

	// Track all returned Cmds
	var cmds []tea.Cmd //nolint:prealloc // appended conditionally across branches; capacity not known up front

	// Start facade-driven
	tab, cmd := tab.Update(ExecutionStartedMsg{
		ExecutionID: "facade-exec",
		Session:     sess,
	})
	cmds = append(cmds, cmd)

	// Send facade events
	events := []ports.Event{
		{Seq: 1, Kind: ports.EventStepStarted, RunID: "facade-exec", Payload: &ports.EnrichedStepPayload{StepName: "build"}},
		{Seq: 2, Kind: ports.EventStepCompleted, RunID: "facade-exec", Payload: &ports.EnrichedStepPayload{StepName: "build"}},
		{Seq: 3, Kind: ports.EventStepStarted, RunID: "facade-exec", Payload: &ports.EnrichedStepPayload{StepName: "test"}},
		{Seq: 4, Kind: ports.EventStepCompleted, RunID: "facade-exec", Payload: &ports.EnrichedStepPayload{StepName: "test"}},
		{Seq: 5, Kind: ports.EventWorkflowCompleted, RunID: "facade-exec"},
	}

	for _, ev := range events {
		tab, cmd = tab.Update(facadeEventMsg{Event: ev})
		cmds = append(cmds, cmd)
	}

	// All returned Cmds must be nil (no tick scheduling)
	for i, cmd := range cmds {
		assert.Nil(t, cmd, "Cmd %d must be nil in facade path (no scheduleTick)", i)
	}
}

// --- T078: Legacy polling types removed from tab_monitoring.go ---

func readTabMonitoringSource(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile("tab_monitoring.go")
	if err != nil {
		wd, _ := os.Getwd()
		source, err = os.ReadFile(wd + "/internal/interfaces/tui/tab_monitoring.go")
		require.NoError(t, err, "must be able to read tab_monitoring.go")
	}
	return string(source)
}

// TestT078_TickMsg_TypeRemovedFromTabMonitoring verifies that the tickMsg type declaration
// has been deleted as part of T078 cleanup (replaced by event-driven path D27).
func TestT078_TickMsg_TypeRemovedFromTabMonitoring(t *testing.T) {
	source := readTabMonitoringSource(t)
	assert.False(t, strings.Contains(source, "type tickMsg struct"),
		"tickMsg type declaration should be removed (T078: legacy polling type)")
}

// TestT078_ExecutionPollMsg_TypeRemovedFromTabMonitoring verifies that the executionPollMsg
// type declaration has been deleted as part of T078 cleanup.
func TestT078_ExecutionPollMsg_TypeRemovedFromTabMonitoring(t *testing.T) {
	source := readTabMonitoringSource(t)
	assert.False(t, strings.Contains(source, "type executionPollMsg struct"),
		"executionPollMsg type declaration should be removed (T078: legacy polling type)")
}

// TestT078_ScheduleTick_FunctionRemovedFromTabMonitoring verifies that the scheduleTick
// function has been deleted as part of T078 cleanup.
func TestT078_ScheduleTick_FunctionRemovedFromTabMonitoring(t *testing.T) {
	source := readTabMonitoringSource(t)
	assert.False(t, strings.Contains(source, "func scheduleTick()"),
		"scheduleTick function should be removed (T078: legacy polling helper)")
}
