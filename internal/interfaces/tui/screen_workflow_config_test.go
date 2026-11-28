package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// openConfigScreen returns a sized workflows screen with "deploy"'s config open and loaded.
func openConfigScreen(t *testing.T) WorkflowsScreen {
	t.Helper()
	items, err := parseWorkflowConfig([]byte(sampleYAML))
	require.NoError(t, err)
	s := loadedWorkflows()
	s.SetSize(100, 27)
	s, cmd := s.Update(press("enter"))
	require.NotNil(t, cmd)
	require.True(t, s.InConfig())
	s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "deploy", Path: "/proj/.awf/workflows/deploy.yaml", Items: items})
	return s
}

func TestWorkflowConfig_EnterOpensAndRequestsTheSource(t *testing.T) {
	s := loadedWorkflows()

	s, cmd := s.Update(press("enter"))

	require.True(t, s.InConfig())
	assert.True(t, s.cfg.loading)
	require.NotNil(t, cmd)
	assert.Equal(t, loadWorkflowSourceMsg{Name: "deploy"}, cmd())
	assert.Nil(t, s.form, "enter no longer runs the workflow")
}

func TestWorkflowConfig_EnterOnAnEmptyListDoesNothing(t *testing.T) {
	s := newWorkflowsScreen()
	s, cmd := s.Update(press("enter"))
	assert.False(t, s.InConfig())
	assert.Nil(t, cmd)
}

func TestWorkflowConfig_XStillRunsFromTheList(t *testing.T) {
	s := loadedWorkflows()
	s.sel = 1 // lint: no inputs

	_, cmd := s.Update(press("x"))

	require.NotNil(t, cmd)
	_, ok := cmd().(LaunchWorkflowMsg)
	assert.True(t, ok)
}

func TestWorkflowConfig_LoadedShowsTheFirstBlockAndAppliesReplies(t *testing.T) {
	s := openConfigScreen(t)

	item, ok := s.cfg.selected()

	require.True(t, ok)
	assert.Equal(t, labelConfigGeneral, item.Name)
	assert.False(t, s.cfg.loading)
	assert.Equal(t, "/proj/.awf/workflows/deploy.yaml", s.cfg.path)
}

func TestWorkflowConfig_StaleReplyForAnotherWorkflowIsIgnored(t *testing.T) {
	s := loadedWorkflows()
	s, _ = s.Update(press("enter"))

	s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "lint", Items: []ConfigItem{{Name: "x"}}})

	assert.True(t, s.cfg.loading, "a reply for a workflow the user left must not fill this view")
	assert.Empty(t, s.cfg.items)
}

func TestWorkflowConfig_ReplyAfterClosingIsIgnored(t *testing.T) {
	s := loadedWorkflows()
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(press("esc"))

	assert.NotPanics(t, func() {
		s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "deploy", Items: []ConfigItem{{Name: "x"}}})
	})
	assert.False(t, s.InConfig())
}

func TestWorkflowConfig_StepNavigationResetsScrollAndStopsAtTheEdges(t *testing.T) {
	s := openConfigScreen(t)

	s, _ = s.Update(press("up"))
	assert.Equal(t, 0, s.cfg.sel)

	for i := 0; i < 6; i++ {
		s, _ = s.Update(press("down"))
	}
	assert.Equal(t, 3, s.cfg.sel, "general + three states: the last index is 3")

	s.cfg.offset = 1
	s, _ = s.Update(press("up"))
	assert.Zero(t, s.cfg.offset, "another block is shown from its first line")
}

func TestWorkflowConfig_Scrolling(t *testing.T) {
	long := ConfigItem{Name: "big", Lines: make([]string, 60)}
	for i := range long.Lines {
		long.Lines[i] = "line: value"
	}
	s := loadedWorkflows()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "deploy", Items: []ConfigItem{long}})
	rows := outputRows(s.h)

	s, _ = s.Update(press("pgdown"))
	assert.Equal(t, rows-1, s.cfg.offset)
	s, _ = s.Update(press("end"))
	assert.Equal(t, 60-rows, s.cfg.offset, "end shows the last line, never past it")
	s, _ = s.Update(press("pgdown"))
	assert.Equal(t, 60-rows, s.cfg.offset)
	s, _ = s.Update(press("home"))
	assert.Zero(t, s.cfg.offset)
	s, _ = s.Update(press("pgup"))
	assert.Zero(t, s.cfg.offset)

	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	assert.Equal(t, wheelStep, s.cfg.offset)
	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	assert.Zero(t, s.cfg.offset)
}

func TestWorkflowConfig_WheelOnTheListDoesNothing(t *testing.T) {
	s := loadedWorkflows()
	assert.NotPanics(t, func() { s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown}) }) //nolint:errcheck // only checking it does not panic
}

func TestWorkflowConfig_ViewShowsStepsAndTheSelectedYAML(t *testing.T) {
	s := openConfigScreen(t)
	s, _ = s.Update(press("down")) // build

	out := ansi.Strip(s.view(100, 27))

	for _, want := range []string{
		"deploy", "/proj/.awf/workflows/deploy.yaml", "local · v1.2.0", // summary
		"general", "1 build", "2 test", "3 done", "terminal", // steps, general unnumbered
		"build · step", "command: make build", "# compile", "on_success: test", // selected block
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "command: make test", "other blocks are not shown")
}

func TestWorkflowConfig_GeneralIsNotNumbered(t *testing.T) {
	s := openConfigScreen(t)
	out := ansi.Strip(s.view(100, 27))
	assert.Contains(t, out, "general")
	assert.NotContains(t, out, "0 general")
	assert.NotContains(t, out, "1 general")
}

func TestWorkflowConfig_LoadingAndErrorStates(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		s := loadedWorkflows()
		s.SetSize(100, 27)
		s, _ = s.Update(press("enter"))
		assert.Contains(t, ansi.Strip(s.view(100, 27)), labelConfigLoading)
	})
	t.Run("error", func(t *testing.T) {
		s := loadedWorkflows()
		s.SetSize(100, 27)
		s, _ = s.Update(press("enter"))
		s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "deploy", Err: labelConfigNoSource})
		assert.Contains(t, flatText(s.view(100, 27)), labelConfigNoSource, "the whole message is readable, wrapped")
		assert.False(t, s.cfg.loading)
	})
}

func TestWorkflowConfig_RunFromTheConfigViewAsksForInputs(t *testing.T) {
	s := openConfigScreen(t)

	s, _ = s.Update(press("x"))

	require.NotNil(t, s.form, "deploy declares inputs, so x opens the form over the config view")
	assert.True(t, s.InConfig(), "the config stays open behind the form")
	s, _ = s.Update(press("esc"))
	assert.Nil(t, s.form)
	assert.True(t, s.InConfig(), "cancelling the form returns to the config, not the list")
}

func TestWorkflowConfig_RunWithoutInputsLaunchesTheOpenedWorkflow(t *testing.T) {
	s := loadedWorkflows()
	s.sel = 1 // lint
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))

	_, cmd := s.Update(press("x"))

	launch, ok := cmd().(LaunchWorkflowMsg)
	require.True(t, ok)
	assert.Equal(t, "lint", launch.Workflow.Name)
}

func TestWorkflowConfig_CopyReloadAndClose(t *testing.T) {
	s := openConfigScreen(t)

	t.Run("copy puts the selected block on the clipboard", func(t *testing.T) {
		assert.Contains(t, s.cfg.copyText(), "name: deploy")
		_, cmd := s.Update(press("y"))
		assert.NotNil(t, cmd)
	})
	t.Run("copy with nothing selected says so", func(t *testing.T) {
		empty := loadedWorkflows()
		empty, _ = empty.Update(press("enter"))
		_, cmd := empty.Update(press("y"))
		require.NotNil(t, cmd)
		assert.Equal(t, flashMsg{Text: labelRunNothingToCopy}, cmd())
	})
	t.Run("R reloads", func(t *testing.T) {
		s2, cmd := s.Update(press("R"))
		assert.True(t, s2.cfg.loading)
		require.NotNil(t, cmd)
		assert.Equal(t, loadWorkflowSourceMsg{Name: "deploy"}, cmd())
	})
	t.Run("esc closes and keeps the list selection", func(t *testing.T) {
		s2, _ := s.Update(press("esc"))
		assert.False(t, s2.InConfig())
		assert.Equal(t, 0, s2.sel)
	})
}

func TestWorkflowConfig_KeyBars(t *testing.T) {
	list := loadedWorkflows()
	assert.Contains(t, ansi.Strip(list.keys(140)), "config", "the list advertises enter")

	open := openConfigScreen(t)
	bar := ansi.Strip(open.keys(140))
	for _, want := range []string{"pick a step", "scroll", "run", "copy", "reload", "back"} {
		assert.Contains(t, bar, want)
	}
}

func TestWorkflowConfig_ViewFillsExactWidthsAndClipsLongLines(t *testing.T) {
	s := loadedWorkflows()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "deploy", Items: []ConfigItem{
		{Name: "wide", Kind: "step", Lines: []string{"command: " + strings.Repeat("x", 400)}},
	}})

	for _, w := range []int{80, 100, 150} {
		for _, l := range lines(s.view(w, 27)) {
			assert.Equal(t, w, ansiWidth(l), "width %d", w)
		}
	}
}

func TestWorkflowConfig_ResizeReclampsTheScroll(t *testing.T) {
	s := loadedWorkflows()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	long := ConfigItem{Name: "big", Lines: make([]string, 80)}
	s, _ = s.Update(WorkflowSourceLoadedMsg{Name: "deploy", Items: []ConfigItem{long}})
	s, _ = s.Update(press("end"))
	require.Positive(t, s.cfg.offset)

	s.SetSize(100, 70)

	assert.LessOrEqual(t, s.cfg.offset, max(0, 80-outputRows(70)))
}

func TestModel_WorkflowConfigEndToEnd(t *testing.T) {
	src := &fakeSources{data: []byte(sampleYAML), path: "/proj/.awf/workflows/deploy.yaml"}
	m := openConfig(t, configModel(t, src))

	out := plain(m)

	assert.Equal(t, []string{"deploy"}, src.requested)
	for _, want := range []string{"general", "1 build", "name: deploy", "pick a step", "back"} {
		assert.Contains(t, out, want)
	}
	assert.True(t, m.workflows.InConfig())
}

func TestModel_WorkflowConfigOwnsTheKeysExceptQuit(t *testing.T) {
	m := openConfig(t, configModel(t, &fakeSources{data: []byte(sampleYAML)}))

	for _, k := range []string{"tab", "shift+tab", "1", "2", "o", "h"} {
		m, _ = send(m, press(k))
		assert.Equal(t, TabWorkflows, m.activeTab, "%q must not leave the open config", k)
		assert.True(t, m.workflows.InConfig(), "%q must not close the config", k)
	}
	_, cmd := send(m, press("q"))
	assert.True(t, isQuit(cmd))

	m, _ = send(m, press("esc"))
	assert.False(t, m.workflows.InConfig())
	m, _ = send(m, press("2"))
	assert.Equal(t, TabHistory, m.activeTab, "navigation works again once the config is closed")
}

func TestModel_WorkflowConfigWithoutABridgeExplainsWhy(t *testing.T) {
	m := loadedModel(t, 110, 30)

	m = openConfig(t, m)

	assert.Contains(t, plain(m), labelConfigNoReader)
}

func TestModel_WorkflowConfigErrorsAreShown(t *testing.T) {
	t.Run("pack workflow", func(t *testing.T) {
		m := openConfig(t, configModel(t, &fakeSources{err: ErrSourceNotFound}))
		assert.Contains(t, flatText(plain(m)), labelConfigNoSource)
	})
	t.Run("invalid yaml", func(t *testing.T) {
		m := openConfig(t, configModel(t, &fakeSources{data: []byte("a: [oops")}))
		assert.Contains(t, plain(m), "invalid YAML")
	})
	t.Run("read failure", func(t *testing.T) {
		m := openConfig(t, configModel(t, &fakeSources{err: errors.New("permission denied")}))
		assert.Contains(t, plain(m), "permission denied")
	})
}

func TestModel_WorkflowConfigRunStartsThroughTheUsualPath(t *testing.T) {
	m := openConfig(t, configModel(t, &fakeSources{data: []byte(sampleYAML)}))

	m, _ = send(m, press("x"))

	assert.NotNil(t, m.workflows.form, "x in the config asks for the workflow's inputs")
	_ = workflow.StatusPending
}
