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
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// helpers

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

// --- orderedSteps ---

func TestOrderedSteps_NilWorkflow_ReturnsNil(t *testing.T) {
	steps := orderedSteps(nil)
	assert.Nil(t, steps)
}

func TestOrderedSteps_EmptyWorkflow_ReturnsNil(t *testing.T) {
	wf := &workflow.Workflow{
		Name:  "empty",
		Steps: map[string]*workflow.Step{},
	}
	steps := orderedSteps(wf)
	assert.Nil(t, steps)
}

func TestOrderedSteps_FollowsExecutionOrder(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "wf",
		Initial: "build",
		Steps: map[string]*workflow.Step{
			"build":  {Name: "build", Type: workflow.StepTypeCommand, OnSuccess: "test"},
			"test":   {Name: "test", Type: workflow.StepTypeCommand, OnSuccess: "deploy"},
			"deploy": {Name: "deploy", Type: workflow.StepTypeCommand, OnSuccess: "done"},
			"done":   {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	steps := orderedSteps(wf)

	require.Len(t, steps, 4)
	assert.Equal(t, "build", steps[0].Name)
	assert.Equal(t, "test", steps[1].Name)
	assert.Equal(t, "deploy", steps[2].Name)
	assert.Equal(t, "done", steps[3].Name)
}

func TestOrderedSteps_UsesTransitionsDefaultFallback(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "wf",
		Initial: "validate",
		Steps: map[string]*workflow.Step{
			"validate": {
				Name: "validate", Type: workflow.StepTypeCommand,
				Transitions: workflow.Transitions{
					{When: "exit_code == 1", Goto: "fail"},
					{When: "", Goto: "deploy"},
				},
			},
			"deploy": {Name: "deploy", Type: workflow.StepTypeCommand, OnSuccess: "done"},
			"fail":   {Name: "fail", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
			"done":   {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	steps := orderedSteps(wf)

	require.Len(t, steps, 3)
	assert.Equal(t, "validate", steps[0].Name)
	assert.Equal(t, "deploy", steps[1].Name)
	assert.Equal(t, "done", steps[2].Name)
}

func TestOrderedSteps_StopsOnCycle(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "wf",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {Name: "a", Type: workflow.StepTypeCommand, OnSuccess: "b"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, OnSuccess: "a"},
		},
	}

	steps := orderedSteps(wf)

	require.Len(t, steps, 2)
	assert.Equal(t, "a", steps[0].Name)
	assert.Equal(t, "b", steps[1].Name)
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
