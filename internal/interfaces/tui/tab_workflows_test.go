package tui

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// --- workflowItem ---

func TestWorkflowItem_Title_ReturnsWorkflowName(t *testing.T) {
	entry := workflow.WorkflowEntry{Name: "my-workflow", Source: "local"}
	wf := &workflow.Workflow{Name: "my-workflow", Description: "A description"}
	item := workflowItem{wf: wf, entry: entry}

	assert.Equal(t, "my-workflow", item.Title())
}

func TestWorkflowItem_Description_WithEntryFields_ShowsDescriptionSourceVersion(t *testing.T) {
	entry := workflow.WorkflowEntry{
		Name:        "my-workflow",
		Description: "A description",
		Source:      "local",
		Version:     "1.0.0",
	}
	wf := &workflow.Workflow{
		Name:  "my-workflow",
		Steps: map[string]*workflow.Step{"step-1": {}, "step-2": {}},
	}
	item := workflowItem{wf: wf, entry: entry}

	desc := item.Description()

	assert.Contains(t, desc, "A description")
	assert.Contains(t, desc, "local")
	assert.Contains(t, desc, "v1.0.0")
}

func TestWorkflowItem_Description_WithNoEntryFields_FallsBackToStepCount(t *testing.T) {
	entry := workflow.WorkflowEntry{Name: "empty-wf"}
	wf := &workflow.Workflow{
		Name:  "empty-wf",
		Steps: map[string]*workflow.Step{"step-1": {}, "step-2": {}},
	}
	item := workflowItem{wf: wf, entry: entry}

	assert.Contains(t, item.Description(), "2 steps")
}

func TestWorkflowItem_Description_WithNoEntryFieldsAndNoWf_ReturnsEmpty(t *testing.T) {
	entry := workflow.WorkflowEntry{Name: "orphan"}
	item := workflowItem{wf: nil, entry: entry}

	assert.Empty(t, item.Description())
}

func TestWorkflowItem_FilterValue_CombinesNameDescriptionSource(t *testing.T) {
	entry := workflow.WorkflowEntry{
		Name:        "deploy-app",
		Description: "Deploys the application",
		Source:      "pack",
	}
	item := workflowItem{entry: entry}

	fv := item.FilterValue()

	assert.Contains(t, fv, "deploy-app")
	assert.Contains(t, fv, "Deploys the application")
	assert.Contains(t, fv, "pack")
}

func TestWorkflowItem_FilterValue_EmptyFields_DoesNotPanic(t *testing.T) {
	entry := workflow.WorkflowEntry{Name: "simple"}
	item := workflowItem{entry: entry}

	require.NotPanics(t, func() {
		_ = item.FilterValue()
	})
	assert.Contains(t, item.FilterValue(), "simple")
}

// --- WorkflowsTab construction ---

func TestNewWorkflowsTab_ReturnsZeroValue(t *testing.T) {
	tab := newWorkflowsTab()

	assert.Zero(t, tab.width, "width should be zero-initialized")
	assert.Zero(t, tab.height, "height should be zero-initialized")
	assert.Nil(t, tab.entries, "entries should be nil initially")
	assert.Nil(t, tab.validationResult, "validationResult should be nil initially")
	assert.Nil(t, tab.bridge, "bridge should be nil initially")
}

func TestNewWorkflowsTab_ListIsInitialized(t *testing.T) {
	tab := newWorkflowsTab()

	// The list must have been created (not zero-value list.Model).
	// Calling View() without panicking is the observable indicator.
	require.NotPanics(t, func() {
		_ = tab.View()
	})
}

// --- Init ---

func TestWorkflowsTab_Init_ReturnsNil(t *testing.T) {
	tab := newWorkflowsTab()

	cmd := tab.Init()

	assert.Nil(t, cmd, "Init() should return nil per Bubbletea convention")
}

// --- setWorkflows ---

func TestWorkflowsTab_SetWorkflows_PopulatesEntriesField(t *testing.T) {
	tab := newWorkflowsTab()
	entries := []workflow.WorkflowEntry{
		{Name: "wf-a", Description: "First", Source: "local"},
		{Name: "wf-b", Description: "Second", Source: "local"},
	}
	wfs := []*workflow.Workflow{
		{Name: "wf-a"},
		{Name: "wf-b"},
	}

	tab.setWorkflows(entries, wfs)

	assert.Equal(t, entries, tab.entries)
}

func TestWorkflowsTab_SetWorkflows_PopulatesListItems(t *testing.T) {
	tab := newWorkflowsTab()
	entries := []workflow.WorkflowEntry{
		{Name: "wf-a", Description: "First", Source: "local"},
		{Name: "wf-b", Description: "Second", Source: "local"},
		{Name: "wf-c", Description: "Third", Source: "local"},
	}
	wfs := []*workflow.Workflow{
		{Name: "wf-a"},
		{Name: "wf-b"},
		{Name: "wf-c"},
	}

	tab.setWorkflows(entries, wfs)

	assert.Len(t, tab.list.Items(), 3)
}

func TestWorkflowsTab_SetWorkflows_EmptySlice_ClearsList(t *testing.T) {
	tab := newWorkflowsTab()
	tab.setWorkflows(
		[]workflow.WorkflowEntry{{Name: "wf-a", Source: "local"}},
		[]*workflow.Workflow{{Name: "wf-a"}},
	)
	tab.setWorkflows([]workflow.WorkflowEntry{}, []*workflow.Workflow{})

	assert.Empty(t, tab.entries)
	assert.Empty(t, tab.list.Items())
}

func TestWorkflowsTab_SetWorkflows_NilSlice_SetsEmptyEntries(t *testing.T) {
	tab := newWorkflowsTab()

	tab.setWorkflows(nil, nil)

	assert.Nil(t, tab.entries)
	assert.Empty(t, tab.list.Items())
}

func TestWorkflowsTab_SetWorkflows_MapsWorkflowsByName(t *testing.T) {
	tab := newWorkflowsTab()
	entries := []workflow.WorkflowEntry{
		{Name: "wf-a", Source: "local"},
		{Name: "wf-b", Source: "pack"},
	}
	wfs := []*workflow.Workflow{
		{Name: "wf-b", Steps: map[string]*workflow.Step{"s1": {}}},
		{Name: "wf-a", Steps: map[string]*workflow.Step{"s1": {}, "s2": {}}},
	}

	tab.setWorkflows(entries, wfs)

	items := tab.list.Items()
	require.Len(t, items, 2)
	// First entry is wf-a which has 2 steps; its Description falls back to step count
	// since WorkflowEntry has no Description/Version set.
	itemA, ok := items[0].(workflowItem)
	require.True(t, ok)
	assert.Equal(t, "wf-a", itemA.entry.Name)
	assert.NotNil(t, itemA.wf, "wf-a should be resolved from workflows slice")
}

// --- Update: window size ---

func TestWorkflowsTab_Update_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	tab := newWorkflowsTab()

	updatedTab, cmd := tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	assert.Equal(t, 80, updatedTab.width)
	assert.Equal(t, 24, updatedTab.height)
	assert.Nil(t, cmd, "WindowSizeMsg should return nil command")
}

func TestWorkflowsTab_Update_WindowSizeMsg_SetsListSize(t *testing.T) {
	tab := newWorkflowsTab()

	updatedTab, _ := tab.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	assert.Equal(t, 100, updatedTab.list.Width())
	// Height is reduced by the tab bar separator rows.
	assert.Greater(t, updatedTab.list.Height(), 0)
}

func TestWorkflowsTab_Update_MultipleWindowSizeMsg_TracksDimensions(t *testing.T) {
	tab := newWorkflowsTab()

	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Equal(t, 120, tab.width, "should track latest width")
	assert.Equal(t, 40, tab.height, "should track latest height")
}

func TestWorkflowsTab_Update_ZeroSizeWindowMsg_SetsZeroDimensions(t *testing.T) {
	tab := newWorkflowsTab()
	tab.width = 80
	tab.height = 24

	updatedTab, cmd := tab.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	assert.Equal(t, 0, updatedTab.width)
	assert.Equal(t, 0, updatedTab.height)
	assert.Nil(t, cmd)
}

// --- Update: ValidationResultMsg ---

func TestWorkflowsTab_Update_ValidationResultMsg_Success_SetsOverlay(t *testing.T) {
	tab := newWorkflowsTab()

	updatedTab, cmd := tab.Update(ValidationResultMsg{Name: "my-wf", Success: true})

	require.NotNil(t, updatedTab.validationResult)
	assert.Equal(t, "Validation passed", *updatedTab.validationResult)
	assert.Nil(t, cmd)
}

func TestWorkflowsTab_Update_ValidationResultMsg_Failure_SetsErrorOverlay(t *testing.T) {
	tab := newWorkflowsTab()

	updatedTab, cmd := tab.Update(ValidationResultMsg{
		Name:    "bad-wf",
		Success: false,
		Error:   "step 'missing' not found",
	})

	require.NotNil(t, updatedTab.validationResult)
	assert.Contains(t, *updatedTab.validationResult, "Validation failed")
	assert.Contains(t, *updatedTab.validationResult, "step 'missing' not found")
	assert.Nil(t, cmd)
}

// --- Update: Enter key ---

func TestWorkflowsTab_Update_EnterKey_WithNoItems_ReturnsNoCmd(t *testing.T) {
	tab := newWorkflowsTab()

	updatedTab, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Nil(t, cmd, "Enter on empty list should return nil cmd")
	assert.Nil(t, updatedTab.validationResult)
}

func TestWorkflowsTab_Update_EnterKey_WithSelectedItem_EmitsLaunchWorkflowMsg(t *testing.T) {
	tab := newWorkflowsTab()
	wf := &workflow.Workflow{Name: "target-wf", Description: "Test"}
	tab.setWorkflows(
		[]workflow.WorkflowEntry{{Name: "target-wf", Source: "local"}},
		[]*workflow.Workflow{wf},
	)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	_, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.NotNil(t, cmd, "Enter on a selected item must return a command")
	msg := cmd()
	launch, ok := msg.(LaunchWorkflowMsg)
	require.True(t, ok, "command must emit LaunchWorkflowMsg, got %T", msg)
	assert.Equal(t, "target-wf", launch.Workflow.Name)
}

// --- Update: 'v' key (validate) ---

func TestWorkflowsTab_Update_VKey_WithNoItems_ReturnsNoCmd(t *testing.T) {
	tab := newWorkflowsTab()

	_, cmd := tab.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})

	assert.Nil(t, cmd)
}

func TestWorkflowsTab_Update_VKey_WithNoBridge_ReturnsNoCmd(t *testing.T) {
	tab := newWorkflowsTab()
	tab.setWorkflows(
		[]workflow.WorkflowEntry{{Name: "wf-a", Description: "Test", Source: "local"}},
		[]*workflow.Workflow{{Name: "wf-a", Description: "Test"}},
	)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	_, cmd := tab.Update(tea.KeyPressMsg{Code: 'v', Text: "v"})

	// No bridge configured — must return nil without panicking.
	assert.Nil(t, cmd)
}

// --- Update: Escape key ---

func TestWorkflowsTab_Update_EscapeKey_WithOverlay_DismissesOverlay(t *testing.T) {
	tab := newWorkflowsTab()
	s := "Validation passed"
	tab.validationResult = &s

	updatedTab, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	assert.Nil(t, updatedTab.validationResult, "Escape must dismiss the validation overlay")
	assert.Nil(t, cmd)
}

func TestWorkflowsTab_Update_EscapeKey_WithNoOverlay_DelegatestoList(t *testing.T) {
	tab := newWorkflowsTab()

	// No overlay — escape is forwarded to the list (which handles filter clearing).
	// Must not panic and should return the tab in a valid state.
	require.NotPanics(t, func() {
		_, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	})
}

// --- View ---

func TestWorkflowsTab_View_ReturnsString(t *testing.T) {
	tab := newWorkflowsTab()

	view := tab.View()

	assert.IsType(t, "", view)
}

func TestWorkflowsTab_View_WithNoWorkflows_ShowsNoWorkflowsFound(t *testing.T) {
	tab := newWorkflowsTab()
	tab.setWorkflows([]workflow.WorkflowEntry{}, []*workflow.Workflow{})

	view := tab.View()

	assert.Contains(t, view, "No workflows found")
}

func TestWorkflowsTab_View_WithWorkflows_ShowsWorkflowNames(t *testing.T) {
	tab := newWorkflowsTab()
	tab.setWorkflows(
		[]workflow.WorkflowEntry{
			{Name: "workflow-alpha", Source: "local"},
			{Name: "workflow-beta", Source: "local"},
		},
		[]*workflow.Workflow{
			{Name: "workflow-alpha", Description: "First"},
			{Name: "workflow-beta", Description: "Second"},
		},
	)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := tab.View()

	assert.Contains(t, view, "workflow-alpha")
	assert.Contains(t, view, "workflow-beta")
}

func TestWorkflowsTab_View_WithValidationOverlay_ShowsOverlay(t *testing.T) {
	tab := newWorkflowsTab()
	s := "Validation passed"
	tab.validationResult = &s

	view := tab.View()

	assert.Contains(t, view, "Validation passed")
	assert.Contains(t, view, "Escape")
}

func TestWorkflowsTab_View_WithValidationError_ShowsErrorMessage(t *testing.T) {
	tab := newWorkflowsTab()
	s := "Validation failed: step 'deploy' references unknown state"
	tab.validationResult = &s

	view := tab.View()

	assert.Contains(t, view, "Validation failed")
}

func TestWorkflowsTab_View_WithZeroDimensions_DoesNotPanic(t *testing.T) {
	tab := newWorkflowsTab()
	tab.setWorkflows(
		[]workflow.WorkflowEntry{{Name: "test-workflow", Description: "Test", Source: "local"}},
		[]*workflow.Workflow{{Name: "test-workflow", Description: "Test"}},
	)

	require.NotPanics(t, func() {
		_ = tab.View()
	})
}

func TestWorkflowsTab_View_WithSingleWorkflow_RendersNonEmpty(t *testing.T) {
	tab := newWorkflowsTab()
	tab.setWorkflows(
		[]workflow.WorkflowEntry{{Name: "single-workflow", Description: "Only workflow", Source: "local"}},
		[]*workflow.Workflow{{Name: "single-workflow", Description: "Only workflow"}},
	)
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := tab.View()

	assert.NotEmpty(t, view)
}

func TestWorkflowsTab_View_WithManyWorkflows_DoesNotPanic(t *testing.T) {
	tab := newWorkflowsTab()
	tab, _ = tab.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	entries := make([]workflow.WorkflowEntry, 100)
	wfs := make([]*workflow.Workflow, 100)
	for i := range wfs {
		name := fmt.Sprintf("workflow-%d", i)
		entries[i] = workflow.WorkflowEntry{Name: name, Source: "local"}
		wfs[i] = &workflow.Workflow{
			Name:        name,
			Description: "Auto-generated workflow",
		}
	}
	tab.setWorkflows(entries, wfs)

	require.NotPanics(t, func() {
		_ = tab.View()
	})
	assert.NotEmpty(t, tab.View())
}

// --- Spinner ---

func TestWorkflowsTab_Spinner_ShownDuringValidation(t *testing.T) {
	tab := newWorkflowsTab()
	tab.validating = true

	view := tab.View()
	assert.Contains(t, view, "Validating")
}

func TestWorkflowsTab_Update_ValidationResult_ClearsValidating(t *testing.T) {
	tab := newWorkflowsTab()
	tab.validating = true

	updated, _ := tab.Update(ValidationResultMsg{Name: "wf", Success: true})
	assert.False(t, updated.validating)
}

// --- workflowItem satisfies list.DefaultItem interface ---

func TestWorkflowItem_ImplementsListItem(t *testing.T) {
	entry := workflow.WorkflowEntry{Name: "check", Source: "local"}
	item := workflowItem{entry: entry}
	var _ list.Item = item
}

func TestWorkflowItem_ImplementsListDefaultItem(t *testing.T) {
	entry := workflow.WorkflowEntry{Name: "check", Source: "local"}
	item := workflowItem{entry: entry}
	var _ list.DefaultItem = item
}
