package tui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestModel_InitWithoutBridgeEmitsEmptyWorkflows(t *testing.T) {
	cmd := New().Init()
	require.NotNil(t, cmd)
	assert.Equal(t, WorkflowsLoadedMsg{}, cmd())
}

func TestModel_TabSwitching(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want Tab
	}{
		{"starts on workflows", nil, TabWorkflows},
		{"digit 2 opens history", []string{"2"}, TabHistory},
		{"digit 1 returns to workflows", []string{"2", "1"}, TabWorkflows},
		{"tab moves forward", []string{"tab"}, TabHistory},
		{"tab wraps around", []string{"tab", "tab"}, TabWorkflows},
		{"shift+tab wraps backwards", []string{"shift+tab"}, TabHistory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loadedModel(t, 100, 30)
			for _, k := range tt.keys {
				m, _ = send(m, press(k))
			}
			assert.Equal(t, tt.want, m.activeTab)
		})
	}
}

func TestModel_QuitKey(t *testing.T) {
	m := loadedModel(t, 100, 30)
	_, cmd := send(m, press("q"))
	assert.True(t, isQuit(cmd))
}

func TestModel_TextEntryOwnsKeys(t *testing.T) {
	m := loadedModel(t, 100, 30)

	m, _ = send(m, press("/"))
	for _, k := range []string{"q", "2", "o"} {
		var cmd tea.Cmd
		m, cmd = send(m, press(k))
		assert.False(t, isQuit(cmd), "typing %q in the filter must not quit", k)
	}

	assert.Equal(t, TabWorkflows, m.activeTab, "digits typed in the filter must not switch tabs")
	assert.Equal(t, "q2o", m.workflows.filter.Value())
}

func TestModel_CtrlC(t *testing.T) {
	t.Run("quits when idle", func(t *testing.T) {
		_, cmd := send(loadedModel(t, 100, 30), press("ctrl+c"))
		assert.True(t, isQuit(cmd))
	})
	t.Run("cancels a running run instead of quitting", func(t *testing.T) {
		sess := &cancellableSession{}
		m := startedRun(t, loadedModel(t, 100, 30), sess)

		m, cmd := send(m, press("ctrl+c"))

		assert.False(t, isQuit(cmd), "ctrl+c must not quit while a run is in flight")
		assert.Equal(t, []string{"cancelled"}, sess.cancelReasons)
		assert.Equal(t, modeRun, m.mode)
		assert.Contains(t, plain(m), "cancelling")
	})
	t.Run("quits from the run view once the run ended", func(t *testing.T) {
		m := startedRun(t, loadedModel(t, 100, 30), &mockRunSession{})
		m, _ = send(m, ExecutionFinishedMsg{})

		_, cmd := send(m, press("ctrl+c"))

		assert.True(t, isQuit(cmd))
	})
}

func TestModel_RenderFillsTheTerminalExactly(t *testing.T) {
	sizes := [][2]int{{80, 24}, {110, 30}, {200, 50}}
	views := map[string]func(*testing.T, Model) Model{
		"workflows": func(_ *testing.T, m Model) Model { return m },
		"history":   func(_ *testing.T, m Model) Model { m, _ = send(m, press("2")); return m },
		"form": func(_ *testing.T, m Model) Model {
			m, _ = send(m, press("x"))
			return m
		},
		"run": func(t *testing.T, m Model) Model { return startedRun(t, m, &mockRunSession{}) },
		"history detail": func(t *testing.T, m Model) Model {
			b := NewBridge(nil, nil)
			b.SetStateReader(&fakeStates{ec: sampleExecution(time.Now())})
			m.bridge = b
			m, _ = send(m, press("2"))
			m, _ = send(m, press("down"))
			return openDetail(t, m)
		},
	}
	for name, setup := range views {
		for _, sz := range sizes {
			t.Run(name, func(t *testing.T) {
				m := setup(t, loadedModel(t, sz[0], sz[1]))

				out := lines(plain(m))

				require.Len(t, out, sz[1], "%s at %dx%d must fill every row", name, sz[0], sz[1])
				for i, l := range out {
					assert.Equal(t, sz[0], ansiWidth(l), "%s row %d at %dx%d", name, i, sz[0], sz[1])
				}
			})
		}
	}
}

// ansiWidth is the number of terminal cells of s (wide runes such as emoji count twice).
func ansiWidth(s string) int { return lipgloss.Width(ansi.Strip(s)) }

func TestModel_TerminalTooSmall(t *testing.T) {
	for _, sz := range [][2]int{{79, 24}, {80, 23}, {40, 10}} {
		m := New()
		m, _ = send(m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})

		assert.Contains(t, plain(m), "Terminal too small", "%dx%d", sz[0], sz[1])
	}
}

func TestModel_RendersAtMinimumSizeBeforeTheFirstWindowSizeMsg(t *testing.T) {
	out := lines(plain(New()))

	require.Len(t, out, minTermHeight)
	for i, l := range out {
		assert.Equal(t, minTermWidth, ansiWidth(l), "row %d", i)
	}
	assert.Contains(t, plain(New()), "1 Workflows")
}

func TestModel_TabBarShowsCounts(t *testing.T) {
	out := plain(loadedModel(t, 100, 30))

	assert.Contains(t, out, "1 Workflows (3)")
	assert.Contains(t, out, "2 History (3)")
	assert.Contains(t, out, "~/project", "the working directory is shown in the header")
}

func TestModel_RunLifecycle(t *testing.T) {
	m := loadedModel(t, 110, 30)

	t.Run("launching without a backend explains why", func(t *testing.T) {
		got, cmd := send(m, LaunchWorkflowMsg{Workflow: sampleWorkflow()})
		assert.Nil(t, cmd)
		assert.Contains(t, plain(got), labelNoBackend)
		assert.Equal(t, modeTabs, got.mode, "a failed start leaves the user on the list")
	})

	t.Run("a started run opens the full-frame view", func(t *testing.T) {
		got := startedRun(t, m, &mockRunSession{})

		assert.Equal(t, modeRun, got.mode)
		out := plain(got)
		assert.Contains(t, out, "deploy")
		assert.Contains(t, out, "Steps")
		assert.Contains(t, out, "pick a step")
		assert.NotContains(t, out, "1 Workflows", "the tab bar is replaced by the run header")
	})

	t.Run("esc returns to the tabs and o reopens the run", func(t *testing.T) {
		got := startedRun(t, m, &mockRunSession{})

		got, _ = send(got, press("esc"))
		assert.Equal(t, modeTabs, got.mode)
		assert.True(t, got.run.Running(), "leaving the view does not stop the run")

		got, _ = send(got, press("o"))
		assert.Equal(t, modeRun, got.mode)
	})

	t.Run("o without a run does nothing", func(t *testing.T) {
		got, _ := send(m, press("o"))
		assert.Equal(t, modeTabs, got.mode)
	})

	t.Run("a finished run reports its outcome in the header", func(t *testing.T) {
		got := startedRun(t, m, &mockRunSession{})
		got, _ = send(got, facadeEventMsg{Event: ports.Event{Kind: ports.EventWorkflowCompleted}})
		got, _ = send(got, ExecutionFinishedMsg{})

		assert.Contains(t, plain(got), "done")
		assert.False(t, got.run.Running())
	})
}

func TestModel_InputRequestPullsUserToTheRun(t *testing.T) {
	t.Run("facade event", func(t *testing.T) {
		m := startedRun(t, loadedModel(t, 110, 30), &mockRunSession{})
		m, _ = send(m, press("esc"))

		m, _ = send(m, facadeEventMsg{Event: ports.Event{
			Kind:    ports.EventInputRequired,
			Payload: &ports.EnrichedInputRequest{Prompt: "> "},
		}})

		assert.Equal(t, modeRun, m.mode)
		assert.True(t, m.run.InputActive())
		assert.Contains(t, plain(m), "Your message")
	})
	t.Run("legacy input request", func(t *testing.T) {
		m, _ := send(loadedModel(t, 110, 30), InputRequestedMsg{})
		assert.Equal(t, modeRun, m.mode)
	})
}

func TestModel_ErrorFlashIsClearedByNextKey(t *testing.T) {
	m, _ := send(loadedModel(t, 110, 30), ErrMsg{Err: errors.New("boom")})
	assert.Contains(t, plain(m), "boom")

	m, _ = send(m, press("down"))

	assert.NotContains(t, plain(m), "boom")
}

func TestModel_ValidationResultFlash(t *testing.T) {
	m := loadedModel(t, 110, 30)

	ok, _ := send(m, ValidationResultMsg{Name: "deploy", Success: true})
	bad, _ := send(m, ValidationResultMsg{Name: "deploy", Error: "unknown step"})

	assert.Contains(t, plain(ok), "deploy is valid")
	assert.Contains(t, plain(bad), "deploy: unknown step")
}

func TestModel_ClockAdvancesOnlyWhileRunning(t *testing.T) {
	m := loadedModel(t, 110, 30)

	idle, cmd := send(m, clockMsg{})
	assert.Equal(t, 0, idle.tick)
	assert.Nil(t, cmd, "an idle model stops the clock")

	running := startedRun(t, m, &mockRunSession{})
	running, cmd = send(running, clockMsg{})
	assert.Equal(t, 1, running.tick)
	assert.Equal(t, 1, running.run.tick, "the run screen shares the frame")
	assert.NotNil(t, cmd, "a running model re-arms the clock")
}

func TestModel_RefreshWithoutBridgeIsHarmless(t *testing.T) {
	m := loadedModel(t, 110, 30)

	got, cmd := send(m, refreshMsg{})

	assert.Equal(t, len(m.workflows.entries), len(got.workflows.entries))
	if cmd != nil {
		assert.NotPanics(t, func() { cmd() })
	}
}

func TestModel_ValidateWithoutBridgeSaysSo(t *testing.T) {
	m := loadedModel(t, 110, 30)

	m, _ = send(m, press("v"))
	m, _ = send(m, m.workflowsValidateMsg(t))

	assert.Contains(t, plain(m), labelNoBackend)
}

// workflowsValidateMsg runs the command produced by the "v" key to obtain its message.
func (m Model) workflowsValidateMsg(t *testing.T) tea.Msg {
	t.Helper()
	_, cmd := m.workflows.updateList(press("v"))
	require.NotNil(t, cmd)
	return cmd()
}

func TestModel_HistoryLoadFeedsBothScreens(t *testing.T) {
	m := loadedModel(t, 110, 30)

	assert.Len(t, m.history.records, 3)
	assert.Len(t, m.workflows.records, 3)
	assert.True(t, m.history.records[0].StartedAt.After(m.history.records[1].StartedAt), "history is newest first")
	_ = workflow.StatusPending // keep the domain import used if assertions change
}

func TestModel_HistoryDetail_OpensWithThePersistedStepsOfTheSelectedRun(t *testing.T) {
	states := &fakeStates{ec: sampleExecution(time.Now())}
	m := historyModel(t, states)

	m = openDetail(t, m)

	out := plain(m)
	for _, want := range []string{"✓ 1 build", "✗ 2 test", "exit status 2", "FAIL: TestX", "API_KEY ***", "back"} {
		assert.Contains(t, out, want)
	}
	assert.Equal(t, []string{"wf-state-aaa"}, states.loadedIDs, "the state is read under the record's workflow ID")
	assert.True(t, m.history.InDetail())
}

func TestModel_HistoryDetail_ShowsLoadingBeforeTheReplyArrives(t *testing.T) {
	m := historyModel(t, &fakeStates{ec: sampleExecution(time.Now())})

	m, cmd := send(m, press("enter"))

	require.NotNil(t, cmd)
	assert.Contains(t, plain(m), labelDetailLoading)
}

func TestModel_HistoryDetail_OwnsTheKeysExceptQuit(t *testing.T) {
	m := openDetail(t, historyModel(t, &fakeStates{ec: sampleExecution(time.Now())}))

	for _, k := range []string{"tab", "shift+tab", "1", "2", "o"} {
		m, _ = send(m, press(k))
		assert.Equal(t, TabHistory, m.activeTab, "%q must not leave the open detail", k)
		assert.True(t, m.history.InDetail(), "%q must not close the detail", k)
	}

	_, cmd := send(m, press("q"))
	assert.True(t, isQuit(cmd), "q still quits")

	m, _ = send(m, press("esc"))
	assert.False(t, m.history.InDetail())
	m, _ = send(m, press("1"))
	assert.Equal(t, TabWorkflows, m.activeTab, "navigation works again once the detail is closed")
}

func TestModel_HistoryDetail_WithoutAStateStoreExplainsWhy(t *testing.T) {
	m := loadedModel(t, 110, 30) // no bridge
	m, _ = send(m, press("2"))
	m, _ = send(m, press("down"))

	m = openDetail(t, m)

	assert.Contains(t, plain(m), labelDetailNoStore)
}

func TestModel_HistoryDetail_MissingStateExplainsWhy(t *testing.T) {
	m := openDetail(t, historyModel(t, &fakeStates{})) // Load returns (nil, nil)
	assert.Contains(t, plain(m), labelDetailGone)
}

func TestModel_HistoryDetail_ReadFailureIsShown(t *testing.T) {
	m := openDetail(t, historyModel(t, &fakeStates{err: errors.New("permission denied")}))
	assert.Contains(t, plain(m), "permission denied")
}

func TestModel_HistoryDetail_ReloadReadsTheStateAgain(t *testing.T) {
	states := &fakeStates{ec: sampleExecution(time.Now())}
	m := openDetail(t, historyModel(t, states))

	m, cmd := send(m, press("r"))
	m = settle(m, cmd)

	assert.Len(t, states.loadedIDs, 2)
	assert.Contains(t, plain(m), "exit status 2")
}

func TestModel_HistoryDetail_CopyShowsConfirmationInTheHeader(t *testing.T) {
	m := openDetail(t, historyModel(t, &fakeStates{ec: sampleExecution(time.Now())}))

	m, flash := send(m, flashMsg{Text: "copied 12 bytes"})

	assert.Nil(t, flash)
	assert.Contains(t, plain(m), "copied 12 bytes")
}

func TestModel_HistoryDetail_MouseWheelScrollsTheOutput(t *testing.T) {
	m := openDetail(t, historyModel(t, &fakeStates{ec: sampleExecution(time.Now())}))
	before := plain(m)

	m, _ = send(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	assert.NotEqual(t, before, plain(m), "the output moved")
	assert.Equal(t, wheelStep, m.history.open.offset)
}

func TestModel_HistoryDetail_ResizeKeepsTheDetailUsable(t *testing.T) {
	m := openDetail(t, historyModel(t, &fakeStates{ec: sampleExecution(time.Now())}))
	m, _ = send(m, press("end"))

	m, _ = send(m, tea.WindowSizeMsg{Width: 160, Height: 60})

	out := lines(plain(m))
	require.Len(t, out, 60)
	assert.Contains(t, plain(m), "exit status 2")
}
