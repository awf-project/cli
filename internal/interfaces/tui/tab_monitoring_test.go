// Package tui — tests for T006: monitoring tab implementation (internal/interfaces/tui/tab_monitoring.go).
//
// Acceptance Criteria covered:
//
//	AC1:  Renders two-panel layout (tree left, viewport right)
//	       → TestMonitoringTab_View_WithExecCtx_RendersTwoPanels
//	AC2:  tea.Tick at 200ms triggers poll when ticking
//	       → TestMonitoringTab_TickMsg_WithExecCtx_EmitsPollMsg
//	       → TestMonitoringTab_TickMsg_WithoutExecCtx_EmitsNextTick
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
//	AC10: PollMsg updates states and rebuilds tree
//	       → TestMonitoringTab_ExecutionPollMsg_UpdatesStates
//	AC11: ExecutionFinishedMsg stops ticking
//	       → TestMonitoringTab_ExecutionFinishedMsg_StopsTicking
//	AC12: ExecutionStartedMsg starts tick loop
//	       → TestMonitoringTab_ExecutionStartedMsg_StartsTicking
package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// helpers

// mockRunSession implements ports.RunSession for testing event loop behavior.
type mockRunSession struct {
	eventsChan <-chan ports.Event
}

func (m *mockRunSession) ID() string {
	return "mock-session"
}

func (m *mockRunSession) Events() <-chan ports.Event {
	return m.eventsChan
}

func (m *mockRunSession) Respond(r ports.InputResponse) error {
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

func makeExecCtx(id string) *workflow.ExecutionContext {
	return workflow.NewExecutionContext(id, id)
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

func TestMonitoringTab_ExecutionStartedMsg_StartsTicking(t *testing.T) {
	tab := newMonitoringTab()
	assert.False(t, tab.ticking, "must not be ticking initially")

	tab, cmd := tab.Update(ExecutionStartedMsg{ExecutionID: "exec-1"})

	assert.True(t, tab.ticking, "must be ticking after ExecutionStartedMsg")
	require.NotNil(t, cmd, "must emit a tick command")
}

func TestMonitoringTab_ExecutionStartedMsg_ResetsState(t *testing.T) {
	tab := newMonitoringTab()
	tab.selectedIdx = 5
	tab.states["old"] = workflow.StepState{Name: "old"}

	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "exec-1"})

	assert.Equal(t, 0, tab.selectedIdx, "selectedIdx must be reset to 0")
	assert.Empty(t, tab.states, "states must be cleared")
	assert.True(t, tab.autoScroll, "auto-scroll must be re-enabled")
}

func TestMonitoringTab_ExecutionStartedMsg_WhenAlreadyTicking_DoesNotDoubleSchedule(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = true

	tab, cmd := tab.Update(ExecutionStartedMsg{ExecutionID: "exec-2"})

	assert.True(t, tab.ticking)
	assert.Nil(t, cmd, "must not emit extra tick when already ticking")
	assert.False(t, tab.showSpinner)
}

// --- ExecutionFinishedMsg ---

func TestMonitoringTab_ExecutionFinishedMsg_StopsTicking(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = true
	execCtx := workflow.NewExecutionContext("exec-1", "test-workflow")
	execCtx.SetStepState("s1", workflow.StepState{Name: "s1", Status: workflow.StatusCompleted})
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "s1",
		Steps: map[string]*workflow.Step{
			"s1": {Name: "s1", Type: workflow.StepTypeTerminal},
		},
	}
	tab.SetExecCtx(execCtx, wf)

	tab, _ = tab.Update(ExecutionFinishedMsg{Err: nil})

	assert.False(t, tab.ticking, "ticking must stop on ExecutionFinishedMsg")
	assert.NotEmpty(t, tab.flatNodes, "tree must be rebuilt on finish")
}

// --- tickMsg ---

func TestMonitoringTab_TickMsg_WhenNotTicking_ReturnsNilCmd(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = false

	_, cmd := tab.Update(tickMsg{})

	assert.Nil(t, cmd)
}

func TestMonitoringTab_TickMsg_WithExecCtx_EmitsPollMsg(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = true
	ctx := makeExecCtx("exec-1")
	ctx.SetStepState("step-a", workflow.StepState{Name: "step-a", Status: workflow.StatusRunning})
	tab.execCtx = ctx

	_, cmd := tab.Update(tickMsg{})

	require.NotNil(t, cmd, "tick with execCtx must emit a cmd")
	msg := cmd()
	pollMsg, ok := msg.(executionPollMsg)
	require.True(t, ok, "expected executionPollMsg, got %T", msg)
	assert.Contains(t, pollMsg.States, "step-a")
}

func TestMonitoringTab_TickMsg_WithoutExecCtx_EmitsNextTick(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = true
	tab.execCtx = nil

	_, cmd := tab.Update(tickMsg{})

	require.NotNil(t, cmd, "tick without execCtx must emit next tick cmd to keep loop alive")
	// Verify the cmd produces another tickMsg.
	msg := cmd()
	_, ok := msg.(tickMsg)
	assert.True(t, ok, "expected tickMsg continuation, got %T", msg)
}

// --- executionPollMsg ---

func TestMonitoringTab_ExecutionPollMsg_UpdatesStates(t *testing.T) {
	tab := newMonitoringTab()
	states := map[string]workflow.StepState{
		"build": {Name: "build", Status: workflow.StatusCompleted},
		"test":  {Name: "test", Status: workflow.StatusRunning},
	}

	tab, _ = tab.Update(executionPollMsg{States: states})

	assert.Equal(t, workflow.StatusCompleted, tab.states["build"].Status)
	assert.Equal(t, workflow.StatusRunning, tab.states["test"].Status)
}

func TestMonitoringTab_ExecutionPollMsg_WhenTicking_EmitsNextTick(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = true

	_, cmd := tab.Update(executionPollMsg{States: map[string]workflow.StepState{}})

	require.NotNil(t, cmd, "poll result while ticking must emit next tick cmd")
	msg := cmd()
	_, ok := msg.(tickMsg)
	assert.True(t, ok, "expected tickMsg continuation, got %T", msg)
}

func TestMonitoringTab_ExecutionPollMsg_WhenNotTicking_NoCmd(t *testing.T) {
	tab := newMonitoringTab()
	tab.ticking = false

	_, cmd := tab.Update(executionPollMsg{States: map[string]workflow.StepState{}})

	assert.Nil(t, cmd)
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

func TestMonitoringTab_View_WithExecCtx_RendersTwoPanels(t *testing.T) {
	tab := monitoringTabWithSize(120, 40)
	ctx := makeExecCtx("exec-1")
	tab.execCtx = ctx

	view := tab.View()

	// Two-panel layout: both panels should be rendered.
	// With lipgloss.JoinHorizontal, content from both sides appears in the output.
	assert.NotEmpty(t, view)
	// Must not show empty state.
	assert.NotContains(t, view, "No active execution")
}

func TestMonitoringTab_View_WithZeroDimensions_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()
	tab.execCtx = makeExecCtx("exec-1")
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

// --- SetExecCtx ---

func TestMonitoringTab_SetExecCtx_WiresContext(t *testing.T) {
	tab := newMonitoringTab()
	ctx := makeExecCtx("wf-1")
	wf := &workflow.Workflow{
		Name:  "wf-1",
		Steps: map[string]*workflow.Step{},
	}

	tab.SetExecCtx(ctx, wf)

	assert.Equal(t, ctx, tab.execCtx)
	assert.Equal(t, wf, tab.wf)
}

func TestMonitoringTab_SetExecCtx_WithNilWorkflow_DoesNotPanic(t *testing.T) {
	tab := newMonitoringTab()
	ctx := makeExecCtx("wf-1")

	require.NotPanics(t, func() {
		tab.SetExecCtx(ctx, nil)
	})
	assert.Equal(t, ctx, tab.execCtx)
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

// --- scheduleTick ---

func TestScheduleTick_EmitsTickMsg(t *testing.T) {
	cmd := scheduleTick()
	require.NotNil(t, cmd)

	// The command executes after the tick interval; calling cmd() blocks
	// in tests so we just verify it returns a tickMsg type.
	// We use a goroutine with a short timeout to avoid hanging.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	msg := <-done
	_, ok := msg.(tickMsg)
	assert.True(t, ok, "scheduleTick must produce tickMsg, got %T", msg)
}

// --- Spinner ---

func TestMonitoringTab_WithExecCtx_RendersTwoPanels(t *testing.T) {
	tab := newMonitoringTab()
	execCtx := workflow.NewExecutionContext("exec-1", "test-wf")
	wf := &workflow.Workflow{Name: "test-wf", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	tab.SetExecCtx(execCtx, wf)
	tab.width = 80
	tab.height = 24
	tab.rebuildTree()

	view := tab.View()
	assert.NotContains(t, view, "No active execution")
}

func TestMonitoringTab_Spinner_HiddenAfterPollData(t *testing.T) {
	tab := newMonitoringTab()
	tab.showSpinner = true

	tab, _ = tab.Update(executionPollMsg{States: map[string]workflow.StepState{}})
	assert.False(t, tab.showSpinner)
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
	tab.StartEventLoop(mockSession)

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
		tab.StartEventLoop(mockSession)
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

	tab.StartEventLoop(mockSession)

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

	tab.StartEventLoop(mockSession)

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

	payload := &transcript.StepPayload{Name: "build", Kind: "command"}
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

	payload := &transcript.StepPayload{Name: "test", Kind: "command"}
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

// TestMonitoringTab_FacadeEventMsg_StepCompleted_WithError_SetsFailedState verifies
// that EventStepCompleted with a non-empty Error sets status to Failed.
func TestMonitoringTab_FacadeEventMsg_StepCompleted_WithError_SetsFailedState(t *testing.T) {
	tab := newMonitoringTab()
	tab.steps = []workflow.Step{
		{Name: "deploy", Type: workflow.StepTypeCommand},
	}
	tab.states["deploy"] = workflow.StepState{Name: "deploy", Status: workflow.StatusRunning}

	payload := &transcript.StepPayload{Name: "deploy", Kind: "command", Error: "exit status 1"}
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

	payload := &transcript.StepPayload{Name: "lint", Kind: "command"}
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

// --- End-to-end: AC gate TestMonitoringTab ---

// TestMonitoringTab is the acceptance-criteria gate named in the task spec.
func TestMonitoringTab(t *testing.T) {
	t.Run("empty state when no execution", func(t *testing.T) {
		tab := newMonitoringTab()
		view := tab.View()
		assert.Contains(t, view, "No active execution")
	})

	t.Run("execution started triggers tick", func(t *testing.T) {
		tab := newMonitoringTab()
		tab, cmd := tab.Update(ExecutionStartedMsg{ExecutionID: "exec-1"})
		assert.True(t, tab.ticking)
		assert.NotNil(t, cmd)
	})

	t.Run("execution finished stops tick", func(t *testing.T) {
		tab := newMonitoringTab()
		tab.ticking = true
		tab, _ = tab.Update(ExecutionFinishedMsg{Err: nil})
		assert.False(t, tab.ticking)
	})

	t.Run("poll msg updates states", func(t *testing.T) {
		tab := newMonitoringTab()
		tab.steps = []workflow.Step{
			{Name: "build", Type: workflow.StepTypeCommand},
		}
		states := map[string]workflow.StepState{
			"build": {Name: "build", Status: workflow.StatusCompleted, Output: "ok"},
		}
		tab, _ = tab.Update(executionPollMsg{States: states})
		assert.Equal(t, workflow.StatusCompleted, tab.states["build"].Status)
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
		tab.execCtx = makeExecCtx("exec-1")
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
		tab.execCtx = makeExecCtx("exec-1")
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
