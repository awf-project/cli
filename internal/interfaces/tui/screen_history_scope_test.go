package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func scopedRecords() []*workflow.ExecutionRecord {
	now := time.Now()
	return []*workflow.ExecutionRecord{
		{ID: "r1", WorkflowName: "deploy", Status: "success", StartedAt: now.Add(-1 * time.Hour)},
		{ID: "r2", WorkflowName: "deploy-prod", Status: "success", StartedAt: now.Add(-2 * time.Hour)},
		{ID: "r3", WorkflowName: "deploy", Status: "failed", StartedAt: now.Add(-3 * time.Hour)},
		{ID: "r4", WorkflowName: "lint", Status: "success", StartedAt: now.Add(-4 * time.Hour)},
	}
}

func TestHistoryScope_IsAnExactMatchNotASubstring(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords(scopedRecords(), nil)

	s.setWorkflowScope("deploy")

	assert.Equal(t, []string{"r1", "r3"}, runIDs(s.visible()), "deploy-prod must not leak into deploy")
}

func TestHistoryScope_CombinesWithStatusAndTextFilters(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords(scopedRecords(), nil)
	s.setWorkflowScope("deploy")

	s, _ = s.Update(press("s")) // success only
	assert.Equal(t, []string{"r1"}, runIDs(s.visible()))

	s, _ = s.Update(press("s")) // failed only
	assert.Equal(t, []string{"r3"}, runIDs(s.visible()))
}

func TestHistoryScope_EscClearsTheTextFilterThenTheScope(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords(scopedRecords(), nil)
	s.setWorkflowScope("deploy")
	s.filter.SetValue("dep")

	s, _ = s.Update(press("esc"))
	assert.Empty(t, s.filter.Value())
	assert.Equal(t, "deploy", s.scope, "the first esc only clears the text filter")

	s, _ = s.Update(press("esc"))
	assert.Empty(t, s.scope)
	assert.Len(t, s.visible(), 4, "the second esc shows every workflow again")
}

func TestHistoryScope_SettingItResetsTheViewState(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords(scopedRecords(), nil)
	s.filter.SetValue("lint")
	s.sel = 0
	s, _ = s.Update(press("enter")) // open the only match's detail
	require.True(t, s.InDetail())

	s.setWorkflowScope("deploy")

	assert.False(t, s.InDetail(), "a stale detail is closed")
	assert.Zero(t, s.sel)
	assert.Empty(t, s.filter.Value(), "a leftover text filter would hide the workflow's runs")
	assert.False(t, s.filtering)
}

func TestHistoryScope_TitleAndKeysShowTheScope(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords(scopedRecords(), nil)

	assert.Equal(t, labelHistoryTitle, s.title())
	assert.NotContains(t, ansi.Strip(s.keys(120)), "all workflows")

	s.setWorkflowScope("deploy")

	assert.Equal(t, "History · deploy", s.title())
	assert.Contains(t, ansi.Strip(s.listBox(50, 12)), "History · deploy")
	assert.Contains(t, ansi.Strip(s.keys(120)), "all workflows", "the way out is advertised")
}

func TestHistoryScope_ScopeWithNoRunsSaysSo(t *testing.T) {
	s := newHistoryScreen()
	s.setRecords(scopedRecords(), nil)

	s.setWorkflowScope("never-run")

	assert.Empty(t, s.visible())
	assert.Contains(t, ansi.Strip(s.listBox(50, 12)), labelHistoryNone)
}

func TestWorkflowsScreen_HKeyAsksForTheWorkflowsHistory(t *testing.T) {
	s := loadedWorkflows()
	s.sel = 2

	_, cmd := s.Update(press("h"))

	require.NotNil(t, cmd)
	assert.Equal(t, showHistoryMsg{Workflow: "my-pack/release"}, cmd())
	assert.Contains(t, ansi.Strip(s.keys(140)), "history")
}

func TestWorkflowsScreen_HKeyOnAnEmptyListDoesNothing(t *testing.T) {
	s := newWorkflowsScreen()
	_, cmd := s.Update(press("h"))
	assert.Nil(t, cmd)
}

func TestModel_HKeyOpensHistoryOnThatWorkflow(t *testing.T) {
	m := loadedModel(t, 110, 30) // deploy is selected first

	m, cmd := send(m, press("h"))
	require.NotNil(t, cmd)
	m, _ = send(m, cmd())

	assert.Equal(t, TabHistory, m.activeTab)
	out := plain(m)
	assert.Contains(t, out, "History · deploy")
	assert.NotContains(t, out, "lint", "other workflows' runs are hidden")
}

func TestModel_TypingHInTheFilterDoesNotJumpToHistory(t *testing.T) {
	m := loadedModel(t, 110, 30)
	m, _ = send(m, press("/"))

	m, _ = send(m, press("h"))

	assert.Equal(t, TabWorkflows, m.activeTab, "h is text while the filter is focused")
	assert.Equal(t, "h", m.workflows.filter.Value())
}

func TestModel_RerunFromHistoryStartsTheRunWithTheSavedInputs(t *testing.T) {
	states := &fakeStates{ec: sampleExecution(time.Now())}
	m := openDetail(t, historyModel(t, states))

	m, cmd := send(m, press("r"))
	require.NotNil(t, cmd)
	launch, ok := cmd().(LaunchWorkflowMsg)
	require.True(t, ok)
	m, start := send(m, launch)

	assert.Contains(t, plain(m), "starting deploy…", "the launch goes through the same path as a run started from the list")
	assert.NotNil(t, start, "the bridge is asked to start the run")
	assert.Equal(t, "prod", m.run.lastLaunch.Inputs["env"], "the launch is remembered so the run view can repeat it")
}
