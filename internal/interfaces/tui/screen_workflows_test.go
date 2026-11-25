package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

func loadedWorkflows() WorkflowsScreen {
	s := newWorkflowsScreen()
	s.setWorkflows(sampleEntries(), []*workflow.Workflow{sampleWorkflow()})
	s.setHistory(sampleRecords(time.Now()))
	return s
}

func names(entries []workflow.WorkflowEntry) []string {
	out := make([]string, len(entries))
	for i := range entries {
		out[i] = entries[i].Name
	}
	return out
}

func TestWorkflowsScreen_FilterMatchesNameAndDescriptionCaseInsensitively(t *testing.T) {
	tests := []struct {
		needle string
		want   []string
	}{
		{"", []string{"deploy", "lint", "my-pack/release"}},
		{"DEP", []string{"deploy"}},
		{"release", []string{"my-pack/release"}},
		{"build and", []string{"deploy"}}, // description
		{"  lint  ", []string{"lint"}},    // surrounding spaces ignored
		{"zzz", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.needle, func(t *testing.T) {
			s := loadedWorkflows()
			s.filter.SetValue(tt.needle)
			assert.Equal(t, tt.want, names(s.visible()))
		})
	}
}

func TestWorkflowsScreen_Navigation(t *testing.T) {
	s := loadedWorkflows()

	for _, step := range []struct {
		key  string
		want int
	}{
		{"down", 1}, {"j", 2}, {"down", 2}, // clamps at the end
		{"up", 1}, {"k", 0}, {"up", 0}, // clamps at the start
		{"G", 2}, {"g", 0}, {"end", 2}, {"home", 0},
	} {
		s, _ = s.Update(press(step.key))
		assert.Equal(t, step.want, s.sel, "after %q", step.key)
	}
}

func TestWorkflowsScreen_FilterModeFlow(t *testing.T) {
	s := loadedWorkflows()

	s, _ = s.Update(press("/"))
	require.True(t, s.filtering)
	assert.True(t, s.InputActive())
	for _, r := range "lint" {
		s, _ = s.Update(press(string(r)))
	}
	assert.Equal(t, []string{"lint"}, names(s.visible()))

	s, _ = s.Update(press("enter"))
	assert.False(t, s.filtering, "enter leaves filter mode")
	assert.Equal(t, []string{"lint"}, names(s.visible()), "enter keeps the filter applied")

	s, _ = s.Update(press("esc"))
	assert.Len(t, s.visible(), 3, "esc outside filter mode clears an applied filter")

	s, _ = s.Update(press("/"))
	s, _ = s.Update(press("l"))
	s, _ = s.Update(press("esc"))
	assert.False(t, s.filtering)
	assert.Empty(t, s.filter.Value(), "esc in filter mode clears and leaves it")
}

func TestWorkflowsScreen_SelectionClampsWhenFilterShrinksTheList(t *testing.T) {
	s := loadedWorkflows()
	s.sel = 2

	s, _ = s.Update(press("/"))
	s, _ = s.Update(press("l"))
	s, _ = s.Update(press("i"))

	assert.Equal(t, 0, s.sel, "typing resets the cursor to the first match")
	e, ok := s.selected()
	require.True(t, ok)
	assert.Equal(t, "lint", e.Name)
}

func TestWorkflowsScreen_RunWithoutInputsLaunchesImmediately(t *testing.T) {
	s := loadedWorkflows()
	s.sel = 1 // lint: not loaded, so it has no inputs

	s, cmd := s.Update(press("x"))

	require.NotNil(t, cmd)
	launch, ok := cmd().(LaunchWorkflowMsg)
	require.True(t, ok)
	assert.Equal(t, "lint", launch.Workflow.Name, "an unloaded workflow still launches by name")
	assert.Nil(t, launch.Inputs)
	assert.Nil(t, s.form)
}

func TestWorkflowsScreen_RunWithInputsOpensFormThenSubmits(t *testing.T) {
	s := loadedWorkflows()

	s, _ = s.Update(press("x"))
	require.NotNil(t, s.form, "a workflow with inputs asks for them first")
	assert.True(t, s.InputActive())
	assert.Contains(t, ansi.Strip(lines(s.popup())[0]), "Run deploy")

	s.form.fields[0].SetValue("prod") // replace the prefilled "staging" default
	s, _ = s.Update(press("enter"))
	var cmd tea.Cmd
	s, cmd = s.Update(press("enter"))

	require.NotNil(t, cmd)
	launch, ok := cmd().(LaunchWorkflowMsg)
	require.True(t, ok)
	assert.Equal(t, "prod", launch.Inputs["env"])
	assert.Nil(t, s.form, "the form closes on submit")
}

func TestWorkflowsScreen_FormCancelReturnsToTheList(t *testing.T) {
	s := loadedWorkflows()
	s, _ = s.Update(press("x"))
	require.NotNil(t, s.form)

	s, cmd := s.Update(press("esc"))

	assert.Nil(t, s.form)
	assert.Nil(t, cmd)
	assert.False(t, s.InputActive())
}

func TestWorkflowsScreen_RunOnEmptyListDoesNothing(t *testing.T) {
	s := newWorkflowsScreen()
	_, cmd := s.Update(press("x"))
	assert.Nil(t, cmd)
}

func TestWorkflowsScreen_ValidateAndRefreshEmitRequests(t *testing.T) {
	s := loadedWorkflows()

	s, cmd := s.Update(press("v"))
	require.NotNil(t, cmd)
	assert.Equal(t, validateRequestMsg{Name: "deploy"}, cmd())
	assert.True(t, s.validating)

	s, _ = s.Update(ValidationResultMsg{Name: "deploy", Success: true})
	assert.False(t, s.validating)

	_, cmd = s.Update(press("r"))
	require.NotNil(t, cmd)
	assert.Equal(t, refreshMsg{}, cmd())
}

func TestWorkflowsScreen_LastRunsAreNewestFirstAndPerWorkflow(t *testing.T) {
	s := loadedWorkflows()

	runs := s.lastRuns("deploy", 5)

	require.Len(t, runs, 2)
	assert.Equal(t, "success", runs[0].Status, "the most recent deploy run comes first")
	assert.Len(t, s.lastRuns("deploy", 1), 1, "the limit is honored")
	assert.Empty(t, s.lastRuns("unknown", 3))
}

func TestWorkflowsScreen_DetailShowsInputsStepsAndRuns(t *testing.T) {
	s := loadedWorkflows()

	out := ansi.Strip(s.detailBox(70, 24))

	for _, want := range []string{
		"deploy", "Build and deploy the app", "local · v1.2.0", "env=staging",
		"1 build · command", "3 done · terminal", "Last runs", "12.4s",
	} {
		assert.Contains(t, out, want)
	}
	assert.Contains(t, out, "API_KEY=***", "secret defaults are masked")
}

func TestWorkflowsScreen_DetailStates(t *testing.T) {
	t.Run("nothing selected", func(t *testing.T) {
		empty := newWorkflowsScreen()
		assert.Contains(t, ansi.Strip(empty.detailBox(60, 10)), "Select a workflow.")
	})
	t.Run("definition could not be loaded", func(t *testing.T) {
		s := loadedWorkflows()
		s.sel = 1
		assert.Contains(t, ansi.Strip(s.detailBox(60, 10)), "could not be read")
	})
	t.Run("no runs yet", func(t *testing.T) {
		s := loadedWorkflows()
		s.setHistory(nil)
		assert.Contains(t, ansi.Strip(s.detailBox(60, 24)), "no runs yet")
	})
}

func TestWorkflowsScreen_ListShowsLastRunIconAndAge(t *testing.T) {
	s := loadedWorkflows()
	out := ansi.Strip(s.listBox(44, 12))

	assert.Contains(t, out, "deploy")
	assert.Contains(t, out, "3 h ago")
	assert.Contains(t, out, "⊘ lint", "a cancelled last run shows the cancelled icon")
	assert.Contains(t, out, "never")
	assert.Contains(t, out, "3 workflows")
	assert.Contains(t, out, "1/3")
}

func TestWorkflowsScreen_ListScrollsToKeepTheSelectionVisible(t *testing.T) {
	s := newWorkflowsScreen()
	var entries []workflow.WorkflowEntry
	for i := 0; i < 30; i++ {
		entries = append(entries, workflow.WorkflowEntry{Name: "wf-" + string(rune('a'+i%26)) + string(rune('0'+i/26))})
	}
	s.setWorkflows(entries, nil)
	s.sel = 25

	out := ansi.Strip(s.listBox(44, 12))

	assert.Contains(t, out, entries[25].Name, "the selected row must be drawn")
	assert.NotContains(t, out, entries[0].Name, "rows scrolled past are not drawn")
	assert.Contains(t, out, "26/30")
}

func TestWorkflowsScreen_KeysBarFollowsTheMode(t *testing.T) {
	s := loadedWorkflows()
	assert.Contains(t, ansi.Strip(s.keys(100)), "validate")

	s, _ = s.Update(press("/"))
	assert.Contains(t, ansi.Strip(s.keys(100)), "apply")

	s, _ = s.Update(press("esc"))
	s, _ = s.Update(press("x"))
	assert.Contains(t, ansi.Strip(s.keys(100)), "next / run")
}

func TestWorkflowsScreen_EmptyStateRenders(t *testing.T) {
	s := newWorkflowsScreen()
	out := ansi.Strip(s.listBox(44, 12))
	assert.Contains(t, out, "no workflows")
	assert.Contains(t, out, "0/0")
}
