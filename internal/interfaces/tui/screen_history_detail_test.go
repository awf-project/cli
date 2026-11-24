package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// openHistoryScreen returns a sized screen with the "deploy" success record's detail open
// and loaded with sampleExecution.
func openHistoryScreen(t *testing.T) HistoryScreen {
	t.Helper()
	s := loadedHistory()
	s.SetSize(100, 27)
	s.sel = 1 // newest first: lint, deploy (success), deploy (failed)
	s, cmd := s.Update(press("enter"))
	require.NotNil(t, cmd)
	require.True(t, s.InDetail())
	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: buildRunDetail(sampleExecution(time.Now()))})
	return s
}

func TestHistoryDetail_EnterOpensAndRequestsTheLoad(t *testing.T) {
	s := loadedHistory()
	s.sel = 1

	s, cmd := s.Update(press("enter"))

	require.True(t, s.InDetail())
	assert.True(t, s.open.loading)
	require.NotNil(t, cmd)
	req, ok := cmd().(loadRunDetailMsg)
	require.True(t, ok)
	assert.Equal(t, "run-aaaaaaaa11", req.Record.ID)
}

func TestHistoryDetail_EnterOnEmptyListDoesNothing(t *testing.T) {
	s := newHistoryScreen()

	s, cmd := s.Update(press("enter"))

	assert.False(t, s.InDetail())
	assert.Nil(t, cmd)
}

func TestHistoryDetail_LoadedPreselectsTheFirstFailedStep(t *testing.T) {
	s := openHistoryScreen(t)

	step, ok := s.open.selected()

	require.True(t, ok)
	assert.Equal(t, "test", step.State.Name, "the failure is on screen without any key press")
	assert.False(t, s.open.loading)
}

func TestHistoryDetail_LoadedWithoutFailureSelectsTheFirstStep(t *testing.T) {
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.SetStepState("a", workflow.StepState{Name: "a", Status: workflow.StatusCompleted, StartedAt: time.Now()})
	s := loadedHistory()
	s.sel = 1
	s, _ = s.Update(press("enter"))

	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: buildRunDetail(ec)})

	assert.Equal(t, 0, s.open.sel)
}

func TestHistoryDetail_StaleReplyForAnotherRecordIsIgnored(t *testing.T) {
	s := loadedHistory()
	s.sel = 1
	s, _ = s.Update(press("enter"))

	s, _ = s.Update(RunDetailLoadedMsg{RecordID: "some-other-record", Detail: buildRunDetail(sampleExecution(time.Now()))})

	assert.True(t, s.open.loading, "a reply for a record the user left must not fill this one")
	assert.Nil(t, s.open.detail)
}

func TestHistoryDetail_ReplyAfterClosingIsIgnored(t *testing.T) {
	s := loadedHistory()
	s, _ = s.Update(press("enter"))
	id := s.open.record.ID
	s, _ = s.Update(press("esc"))

	assert.NotPanics(t, func() {
		s, _ = s.Update(RunDetailLoadedMsg{RecordID: id, Detail: &RunDetail{}})
	})
	assert.False(t, s.InDetail())
}

func TestHistoryDetail_LoadErrorIsShown(t *testing.T) {
	s := loadedHistory()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))

	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Err: labelDetailGone})

	out := ansi.Strip(s.view(100, 27))
	assert.Contains(t, out, labelDetailGone)
	assert.False(t, s.open.loading)
}

func TestHistoryDetail_EscClosesAndKeepsTheListSelection(t *testing.T) {
	s := openHistoryScreen(t)

	s, _ = s.Update(press("esc"))

	assert.False(t, s.InDetail())
	assert.Equal(t, 1, s.sel)
}

func TestHistoryDetail_StepNavigationStopsAtTheEdgesAndResetsScroll(t *testing.T) {
	s := openHistoryScreen(t)
	s.open.sel, s.open.offset = 0, 0

	s, _ = s.Update(press("up"))
	assert.Equal(t, 0, s.open.sel)

	s, _ = s.Update(press("down"))
	s, _ = s.Update(press("pgdown"))
	require.Positive(t, s.open.offset)
	s, _ = s.Update(press("down"))
	assert.Equal(t, 2, s.open.sel)
	assert.Zero(t, s.open.offset, "picking another step shows it from its first line")

	s, _ = s.Update(press("down"))
	assert.Equal(t, 2, s.open.sel, "stops at the last step")
}

func TestHistoryDetail_Scrolling(t *testing.T) {
	s := openHistoryScreen(t) // "test" selected: 48 lines of content
	rows := outputRows(s.h)
	total := len(s.open.lines())
	require.Greater(t, total, rows, "the fixture must overflow the panel")

	s, _ = s.Update(press("pgdown"))
	assert.Equal(t, rows-1, s.open.offset, "a page leaves one line of context")

	s, _ = s.Update(press("end"))
	assert.Equal(t, total-rows, s.open.offset, "end shows the last line, never past it")

	s, _ = s.Update(press("pgdown"))
	assert.Equal(t, total-rows, s.open.offset, "cannot scroll past the end")

	s, _ = s.Update(press("home"))
	assert.Zero(t, s.open.offset)

	s, _ = s.Update(press("pgup"))
	assert.Zero(t, s.open.offset, "cannot scroll before the start")
}

func TestHistoryDetail_MouseWheelScrolls(t *testing.T) {
	s := openHistoryScreen(t)

	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	assert.Equal(t, wheelStep, s.open.offset)

	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	assert.Zero(t, s.open.offset)

	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	assert.Zero(t, s.open.offset, "wheel up at the top stays at the top")
}

func TestHistoryDetail_MouseWheelOnTheListDoesNothing(t *testing.T) {
	s := loadedHistory()
	assert.NotPanics(t, func() { s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown}) }) //nolint:errcheck // only checking it does not panic
}

func TestHistoryDetail_ViewShowsStepsOutputAndMaskedInputs(t *testing.T) {
	s := openHistoryScreen(t)

	out := ansi.Strip(s.view(100, 27))

	for _, want := range []string{
		"deploy", "run-aaaa", // summary line
		"✓ 1 build", "5.0s", "✗ 2 test", "3.0s", "○ 3 done", "pending", // steps
		"Output · test", "exit status 2", "FAIL: TestX", "test output line", // selected step
		"env prod", "API_KEY ***", // inputs
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "s3cr3t", "secret inputs are never drawn")
	assert.Contains(t, out, "(1–", "an overflowing output shows its scroll position")
}

func TestHistoryDetail_SwitchingStepShowsItsOutput(t *testing.T) {
	s := openHistoryScreen(t)
	s, _ = s.Update(press("up")) // build

	out := ansi.Strip(s.view(100, 27))

	assert.Contains(t, out, "Output · build")
	assert.Contains(t, out, "compiled ok")
	assert.NotContains(t, out, "exit status 2")
}

func TestHistoryDetail_StepsWithoutOutputExplainWhy(t *testing.T) {
	tests := []struct {
		name   string
		status workflow.ExecutionStatus
		want   string
		absent string
	}{
		{"never ran", workflow.StatusPending, labelDetailDidNotRun, labelRunPending},
		{"run stopped mid-step", workflow.StatusRunning, labelDetailInterrupted, labelRunRunning},
		{"completed quietly", workflow.StatusCompleted, labelDetailNoOutput, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := workflow.NewExecutionContext("wf-1", "deploy")
			ec.SetStepState("quiet", workflow.StepState{Name: "quiet", Status: tt.status, StartedAt: time.Now()})
			s := loadedHistory()
			s.SetSize(100, 27)
			s, _ = s.Update(press("enter"))
			s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: buildRunDetail(ec)})

			out := ansi.Strip(s.view(100, 27))

			assert.Contains(t, out, tt.want)
			if tt.absent != "" {
				assert.NotContains(t, out, tt.absent, "the live-run placeholder is wrong for a past run")
			}
		})
	}
}

func TestHistoryDetail_TruncatedOutputIsFlagged(t *testing.T) {
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.SetStepState("big", workflow.StepState{Name: "big", Status: workflow.StatusCompleted, Output: "head", Truncated: true, StartedAt: time.Now()})
	s := loadedHistory()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: buildRunDetail(ec)})

	assert.Contains(t, ansi.Strip(s.view(100, 27)), labelDetailTruncated)
}

func TestHistoryDetail_RunWithNoSavedStepsSaysSo(t *testing.T) {
	s := loadedHistory()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: &RunDetail{}})

	out := ansi.Strip(s.view(100, 27))
	assert.Contains(t, out, labelDetailNoSteps)
	assert.Contains(t, out, labelDetailNoStepsHint, "the full explanation fits in the wider Output box")
}

func TestHistoryDetail_ConversationStepIsShownAsAChat(t *testing.T) {
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.SetStepState("talk", workflow.StepState{
		Name: "talk", Status: workflow.StatusCompleted, StartedAt: time.Now(),
		Conversation: &workflow.ConversationState{Turns: []workflow.Turn{
			{Role: workflow.TurnRoleUser, Content: "hello"},
			{Role: workflow.TurnRoleAssistant, Content: "hi there"},
		}},
	})
	s := loadedHistory()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: buildRunDetail(ec)})

	out := ansi.Strip(s.view(100, 27))

	assert.Contains(t, out, labelRunUser)
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, labelRunAgent)
	assert.Contains(t, out, "hi there")
}

func TestHistoryDetail_CopyAndReload(t *testing.T) {
	s := openHistoryScreen(t)

	t.Run("copy reports what was copied", func(t *testing.T) {
		_, cmd := s.Update(press("y"))
		require.NotNil(t, cmd)
		assert.Contains(t, s.open.copyText(), "exit status 2")
		assert.Contains(t, s.open.copyText(), "FAIL: TestX")
	})
	t.Run("nothing to copy is reported", func(t *testing.T) {
		empty := openHistoryScreen(t)
		empty.open.sel = 2
		_, cmd := empty.Update(press("y"))
		require.NotNil(t, cmd)
		assert.Equal(t, flashMsg{Text: labelRunNothingToCopy}, cmd())
	})
	t.Run("reload requests the load again", func(t *testing.T) {
		s2, cmd := s.Update(press("r"))
		assert.True(t, s2.open.loading)
		require.NotNil(t, cmd)
		_, ok := cmd().(loadRunDetailMsg)
		assert.True(t, ok)
	})
}

func TestHistoryDetail_KeyBars(t *testing.T) {
	list := loadedHistory()
	assert.Contains(t, ansi.Strip(list.keys(120)), "step detail", "the list advertises enter")

	s := openHistoryScreen(t)
	bar := ansi.Strip(s.keys(120))
	for _, want := range []string{"pick a step", "scroll", "copy", "reload", "back"} {
		assert.Contains(t, bar, want)
	}
}

func TestHistoryDetail_ViewFillsExactWidths(t *testing.T) {
	s := openHistoryScreen(t)
	for _, w := range []int{80, 100, 150} {
		for _, l := range lines(s.view(w, 27)) {
			assert.Equal(t, w, ansiWidth(l), "width %d", w)
		}
	}
}

func TestHistoryDetail_ResizeReclampsTheScrollPosition(t *testing.T) {
	s := openHistoryScreen(t)
	s, _ = s.Update(press("end"))
	require.Positive(t, s.open.offset)

	s.SetSize(100, 60) // a much taller panel fits more lines at once

	assert.LessOrEqual(t, s.open.offset, max(0, len(s.open.lines())-outputRows(60)))
}

func TestHistoryDetail_LongLinesAreClippedToTheBox(t *testing.T) {
	ec := workflow.NewExecutionContext("wf-1", "deploy")
	ec.SetStepState("wide", workflow.StepState{Name: "wide", Status: workflow.StatusCompleted, Output: strings.Repeat("x", 500), StartedAt: time.Now()})
	s := loadedHistory()
	s.SetSize(100, 27)
	s, _ = s.Update(press("enter"))
	s, _ = s.Update(RunDetailLoadedMsg{RecordID: s.open.record.ID, Detail: buildRunDetail(ec)})

	for _, l := range lines(s.view(100, 27)) {
		assert.Equal(t, 100, ansiWidth(l))
	}
}
