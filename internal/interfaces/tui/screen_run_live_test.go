package tui

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// cancellableSession is a RunSession that can publish a terminal cancelled event,
// like application.RunSession.
type cancellableSession struct {
	mockRunSession
	cancelReasons []string
	closed        bool
}

func (c *cancellableSession) CancelWithEvent(reason string) error {
	c.cancelReasons = append(c.cancelReasons, reason)
	return nil
}

func (c *cancellableSession) Close() error {
	c.closed = true
	return nil
}

// closeOnlySession has no CancelWithEvent, so cancelling must fall back to Close.
type closeOnlySession struct {
	mockRunSession
	closed bool
}

func (c *closeOnlySession) Close() error {
	c.closed = true
	return nil
}

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func stepOutputEvent(step, stream, text string) facadeEventMsg {
	return facadeEventMsg{Event: ports.Event{
		Kind:    ports.EventStepOutput,
		Payload: &ports.EnrichedStepOutput{StepName: step, Stream: stream, Text: text},
	}}
}

// runningScreen returns a sized screen with one command step already started.
func runningScreen(t *testing.T) RunScreen {
	t.Helper()
	tab := monitoringTabWithSize(120, 40)
	tab.SetWorkflow(sampleWorkflow()) // build -> test -> done
	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "deploy", Workflow: tab.wf, Session: &mockRunSession{}})
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind:    ports.EventStepStarted,
		Payload: &ports.EnrichedStepPayload{StepName: "build"},
	}})
	return tab
}

func TestLiveOutput_Add(t *testing.T) {
	tests := []struct {
		name       string
		chunks     [][2]string // stream, text
		wantStdout string
		wantStderr string
	}{
		{"concatenates raw chunks of one stream", [][2]string{{ports.StepOutputStdout, "he"}, {ports.StepOutputStdout, "llo\nwor"}, {ports.StepOutputStdout, "ld"}}, "hello\nworld", ""},
		{"keeps streams apart", [][2]string{{ports.StepOutputStdout, "out"}, {ports.StepOutputStderr, "err"}}, "out", "err"},
		{"unknown stream is treated as stdout, never lost", [][2]string{{"weird", "x"}}, "x", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l liveOutput
			for _, c := range tt.chunks {
				l.add(c[0], c[1])
			}
			assert.Equal(t, tt.wantStdout, l.stdout)
			assert.Equal(t, tt.wantStderr, l.stderr)
			assert.False(t, l.truncated)
		})
	}
}

func TestLiveOutput_CapKeepsTailAndValidUTF8(t *testing.T) {
	var l liveOutput
	// "é" is 2 bytes; an odd cap boundary would split it if the cut were naive.
	l.add(ports.StepOutputStdout, strings.Repeat("é", maxLiveOutputBytes))

	assert.True(t, l.truncated)
	assert.LessOrEqual(t, len(l.stdout), maxLiveOutputBytes)
	assert.True(t, utf8.ValidString(l.stdout), "cut must not split a multi-byte rune")
	assert.True(t, strings.HasSuffix(l.stdout, "é"), "the newest bytes are kept")
}

func TestRunScreen_StepOutputShownWhileRunningThenReplacedOnCompletion(t *testing.T) {
	tab := runningScreen(t)

	tab, _ = tab.Update(stepOutputEvent("build", ports.StepOutputStdout, "compiling a\ncompiling b\n"))
	tab, _ = tab.Update(stepOutputEvent("build", ports.StepOutputStderr, "warning: unused\n"))

	content := tab.vp.GetContent()
	assert.Contains(t, content, labelRunOutputLive)
	assert.Contains(t, content, "compiling b")
	assert.Contains(t, content, labelRunStderrLive)
	assert.Contains(t, content, "warning: unused")
	assert.NotContains(t, content, labelRunRunning, "live output replaces the running placeholder")

	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind:    ports.EventStepCompleted,
		Payload: &ports.EnrichedStepPayload{StepName: "build", Output: "final output", Stderr: ""},
	}})

	content = tab.vp.GetContent()
	assert.Contains(t, content, "final output")
	assert.NotContains(t, content, "compiling b", "authoritative output replaces streamed output")
	assert.NotContains(t, content, labelRunOutputLive)
	assert.NotContains(t, tab.live, "build", "live buffer is released on completion")
}

func TestRunScreen_StepOutputForOtherStepDoesNotRerenderPanel(t *testing.T) {
	tab := runningScreen(t)
	before := tab.vp.GetContent()

	tab, _ = tab.Update(stepOutputEvent("background", ports.StepOutputStdout, "noise\n"))

	assert.Equal(t, before, tab.vp.GetContent(), "chunks of an unselected step must not touch the panel")
	require.Contains(t, tab.live, "background", "the chunk is still buffered for when the step is selected")
	assert.Equal(t, "noise\n", tab.live["background"].stdout)
}

func TestRunScreen_StepOutputIgnoresMalformedPayloads(t *testing.T) {
	tab := runningScreen(t)
	for name, payload := range map[string]any{
		"nil payload":     nil,
		"wrong type":      &ports.EnrichedStepPayload{StepName: "build"},
		"empty step name": &ports.EnrichedStepOutput{Text: "x"},
		"empty text":      &ports.EnrichedStepOutput{StepName: "build"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				tab.Update(facadeEventMsg{Event: ports.Event{Kind: ports.EventStepOutput, Payload: payload}}) //nolint:errcheck // result irrelevant: only checking it does not panic
			})
		})
	}
	assert.Empty(t, tab.live)
}

func TestRunScreen_LongLiveOutputShowsOnlyTail(t *testing.T) {
	tab := runningScreen(t)
	var sb strings.Builder
	for i := 0; i < maxLiveLines+30; i++ {
		sb.WriteString("line\n")
	}
	sb.WriteString("LAST\n")

	tab, _ = tab.Update(stepOutputEvent("build", ports.StepOutputStdout, sb.String()))

	content := tab.vp.GetContent()
	assert.Contains(t, content, "LAST")
	assert.Contains(t, content, "earlier lines")
}

func TestRunScreen_StartAndFinishRunToggleRunning(t *testing.T) {
	tab := runningScreen(t)
	assert.True(t, tab.running)
	assert.True(t, tab.showSpinner || len(tab.states) > 0)

	tab, _ = tab.Update(ExecutionFinishedMsg{})

	assert.False(t, tab.running)
	assert.False(t, tab.showSpinner)
	assert.Nil(t, tab.activeSession)
}

func TestRunScreen_StartWithoutSessionIsNotRunning(t *testing.T) {
	tab := monitoringTabWithSize(100, 30)

	tab, _ = tab.Update(ExecutionStartedMsg{ExecutionID: "x"})

	assert.False(t, tab.running, "no session means nothing is in flight")
}

func TestRunScreen_FinishMarksReachedTerminalStepFromOutcome(t *testing.T) {
	tab := monitoringTabWithSize(100, 30)
	tab.steps = []workflow.Step{
		{Name: "build", Type: workflow.StepTypeCommand, Command: "make"},
		{Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
	}
	tab.wf = &workflow.Workflow{Name: "wf"}
	tab, _ = tab.Update(ExecutionStartedMsg{Workflow: tab.wf, Session: &mockRunSession{}})
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{Kind: ports.EventWorkflowCompleted}})

	tab, _ = tab.Update(ExecutionFinishedMsg{})

	var done *TreeNode
	for _, n := range tab.flatNodes {
		if n.Name == "done" {
			done = n
		}
	}
	require.NotNil(t, done)
	assert.Equal(t, workflow.StatusCompleted, done.Status, "terminal step shows the workflow outcome once the run ended")
}

func TestRunScreen_CancelUsesCancelWithEventWhenAvailable(t *testing.T) {
	tab := runningScreen(t)
	sess := &cancellableSession{}
	tab.activeSession = sess

	tab, cmd := tab.Update(keyPress('c'))

	assert.Nil(t, cmd)
	assert.Equal(t, []string{"cancelled"}, sess.cancelReasons)
	assert.False(t, sess.closed, "a cancellable session is cancelled, not closed: the terminal event follows")
	assert.True(t, tab.cancelling)
	assert.True(t, tab.running, "the run stays in flight until the terminal event arrives")
	assert.Contains(t, ansi.Strip(tab.outputBox(80, 10)), labelRunCancelling)

	// A second press while cancelling is a no-op.
	tab, _ = tab.Update(keyPress('c'))
	assert.Len(t, sess.cancelReasons, 1)
}

func TestRunScreen_CancelFallsBackToCloseAndFinishesImmediately(t *testing.T) {
	tab := runningScreen(t)
	sess := &closeOnlySession{}
	tab.activeSession = sess

	tab, _ = tab.Update(keyPress('c'))

	assert.True(t, sess.closed)
	assert.False(t, tab.running, "no terminal event will arrive after a direct Close")
	assert.False(t, tab.cancelling)
}

func TestRunScreen_CancelWhenIdleDoesNothing(t *testing.T) {
	tab := monitoringTabWithSize(100, 30)

	tab, cmd := tab.Update(keyPress('c'))

	assert.Nil(t, cmd)
	assert.False(t, tab.cancelling)
}

func TestRunScreen_RerunEmitsLastLaunchOnlyWhenIdle(t *testing.T) {
	wf := &workflow.Workflow{Name: "deploy"}
	launch := LaunchWorkflowMsg{Workflow: wf, Inputs: map[string]any{"env": "prod"}}

	t.Run("idle with a recorded launch", func(t *testing.T) {
		tab := monitoringTabWithSize(100, 30)
		tab.SetLaunch(launch)

		_, cmd := tab.Update(keyPress('r'))

		require.NotNil(t, cmd)
		assert.Equal(t, launch, cmd())
	})
	t.Run("running run is not restarted", func(t *testing.T) {
		tab := runningScreen(t)
		tab.SetLaunch(launch)

		_, cmd := tab.Update(keyPress('r'))

		assert.Nil(t, cmd)
	})
	t.Run("nothing to repeat", func(t *testing.T) {
		tab := monitoringTabWithSize(100, 30)

		_, cmd := tab.Update(keyPress('r'))

		assert.Nil(t, cmd)
	})
}

func TestRunScreen_CopyBuildsPlainTextOfSelectedStep(t *testing.T) {
	tests := []struct {
		name  string
		state workflow.StepState
		live  *liveOutput
		want  string
	}{
		{"completed step: error, stderr then stdout", workflow.StepState{Name: "build", Status: workflow.StatusFailed, Error: "exit 2", Stderr: "boom", Output: "partial"}, nil, "exit 2\nboom\npartial"},
		{"running step copies live output", workflow.StepState{Name: "build", Status: workflow.StatusRunning}, &liveOutput{stdout: "so far\n"}, "so far"},
		{"live output is ignored once completed", workflow.StepState{Name: "build", Status: workflow.StatusCompleted, Output: "final"}, &liveOutput{stdout: "stale"}, "final"},
		{"empty step copies nothing", workflow.StepState{Name: "build", Status: workflow.StatusPending}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tab := runningScreen(t)
			tab.states["build"] = tt.state
			tab.live = map[string]*liveOutput{}
			if tt.live != nil {
				tab.live["build"] = tt.live
			}
			tab.rebuildTree()

			assert.Equal(t, tt.want, tab.copyText())
		})
	}
}

func TestRunScreen_CopyKeyReportsOutcome(t *testing.T) {
	t.Run("with output returns a clipboard command", func(t *testing.T) {
		tab := runningScreen(t)
		tab.states["build"] = workflow.StepState{Name: "build", Status: workflow.StatusCompleted, Output: "abc"}

		tab, cmd := tab.Update(keyPress('y'))

		assert.NotNil(t, cmd)
		assert.Contains(t, tab.notice, "3 bytes")
		assert.Contains(t, ansi.Strip(tab.outputBox(80, 10)), "copied 3 bytes")
	})
	t.Run("without output reports nothing to copy", func(t *testing.T) {
		tab := runningScreen(t)

		tab, cmd := tab.Update(keyPress('y'))

		assert.Nil(t, cmd)
		assert.Equal(t, labelRunNothingToCopy, tab.notice)
	})
	t.Run("next key clears the notice", func(t *testing.T) {
		tab := runningScreen(t)
		tab, _ = tab.Update(keyPress('y'))

		tab, _ = tab.Update(keyPress('f'))

		assert.Empty(t, tab.notice)
	})
}

func TestRunScreen_ActionKeysAreTypedTextWhileInputIsActive(t *testing.T) {
	tab := runningScreen(t)
	sess := &cancellableSession{}
	tab.activeSession = sess
	tab.beginInputRequest()
	require.True(t, tab.InputActive())

	for _, r := range "cry" {
		tab, _ = tab.Update(keyPress(r))
	}

	assert.Empty(t, sess.cancelReasons, "'c' while typing a message must not cancel the run")
	assert.False(t, tab.cancelling)
	assert.Equal(t, "cry", tab.inputField.Value())
}

func TestRunScreen_StartResetsLiveStateFromPreviousRun(t *testing.T) {
	tab := runningScreen(t)
	tab, _ = tab.Update(stepOutputEvent("build", ports.StepOutputStdout, "old\n"))
	tab.cancelling = true
	tab.notice = "stale"

	tab, _ = tab.Update(ExecutionStartedMsg{Workflow: tab.wf, Session: &mockRunSession{}})

	assert.Empty(t, tab.live)
	assert.False(t, tab.cancelling)
	assert.Empty(t, tab.notice)
}

func TestCancelSession(t *testing.T) {
	assert.True(t, cancelSession(nil), "nil session has nothing to wait for")

	c := &cancellableSession{}
	assert.False(t, cancelSession(c))
	assert.Equal(t, []string{"cancelled"}, c.cancelReasons)

	o := &closeOnlySession{}
	assert.True(t, cancelSession(o))
	assert.True(t, o.closed)
}

func TestRunScreen_StepContentView(t *testing.T) {
	tests := []struct {
		name   string
		states map[string]workflow.StepState
		want   []string
		absent []string
	}{
		{
			"completed step shows its input and output",
			map[string]workflow.StepState{"build": {Name: "build", Status: workflow.StatusCompleted, Output: "build successful"}},
			[]string{"$ make", "build successful", labelRunOutput},
			nil,
		},
		{
			"failed step shows error and stderr",
			map[string]workflow.StepState{"build": {Name: "build", Status: workflow.StatusFailed, Error: "connection refused", Stderr: "fatal error"}},
			[]string{"connection refused", "fatal error", labelRunError, labelRunStderr},
			[]string{labelRunOutput},
		},
		{
			"step that has not started yet waits",
			map[string]workflow.StepState{},
			[]string{labelRunWaiting},
			[]string{labelRunOutput},
		},
		{
			"running step without output says so",
			map[string]workflow.StepState{"build": {Name: "build", Status: workflow.StatusRunning}},
			[]string{labelRunRunning},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tab := runningScreen(t)
			tab.states = tt.states
			tab.live = map[string]*liveOutput{}

			out := ansi.Strip(tab.renderSelectedStepChat("build"))

			for _, w := range tt.want {
				assert.Contains(t, out, w)
			}
			for _, a := range tt.absent {
				assert.NotContains(t, out, a)
			}
		})
	}
}

func TestRunScreen_StepsBox(t *testing.T) {
	tab := runningScreen(t)
	tab.begin["build"] = time.Now().Add(-90 * time.Second)
	tab.end["build"] = tab.begin["build"].Add(65 * time.Second)
	tab.states["build"] = workflow.StepState{Name: "build", Status: workflow.StatusCompleted}
	tab.inputs = map[string]any{"env": "prod", "API_KEY": "s3cr3t"}
	tab.rebuildTree()

	out := ansi.Strip(tab.stepsBox(38, 20))

	assert.Contains(t, out, "✓ 1 build")
	assert.Contains(t, out, "1m05s", "a finished step shows its duration")
	assert.Contains(t, out, "3 done")
	assert.Contains(t, out, "pending", "steps that have not started are labeled")
	assert.Contains(t, out, "env prod")
	assert.Contains(t, out, "API_KEY ***", "secret inputs are masked")
	assert.NotContains(t, out, "s3cr3t")
}

func TestRunScreen_StepsBoxBeforeAnyStepShowsAStartingHint(t *testing.T) {
	tab := monitoringTabWithSize(100, 30)
	tab.showSpinner = true

	assert.Contains(t, ansi.Strip(tab.stepsBox(38, 10)), labelRunStarting)
}

func TestRunScreen_HeaderStatusText(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*RunScreen)
		want  string
	}{
		{"running", func(t *RunScreen) {}, "running"},
		{"waiting for the user", func(t *RunScreen) { t.inputActive = true }, "waiting for your reply"},
		{"cancelling", func(t *RunScreen) { t.cancelling = true }, "cancelling"},
		{"completed", func(t *RunScreen) { t.running = false; t.finalStatus = workflow.StatusCompleted }, "done"},
		{"failed", func(t *RunScreen) { t.running = false; t.finalStatus = workflow.StatusFailed }, "failed"},
		{"cancelled by the user", func(t *RunScreen) { t.running = false; t.finalStatus = workflow.StatusFailed; t.cancelledRun = true }, "cancelled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tab := runningScreen(t)
			tt.setup(&tab)
			assert.Contains(t, ansi.Strip(tab.statusText()), tt.want)
		})
	}
	idle := newRunScreen()
	assert.Empty(t, idle.statusText())
}

func TestRunScreen_CancelledRunIsReportedAsCancelled(t *testing.T) {
	tab := runningScreen(t)
	tab.activeSession = &cancellableSession{}
	tab.RequestCancel()
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{Kind: ports.EventWorkflowFailed}})

	tab, _ = tab.Update(ExecutionFinishedMsg{})

	assert.Contains(t, ansi.Strip(tab.statusText()), "cancelled")
	assert.NotContains(t, ansi.Strip(tab.statusText()), "failed")
}

func TestRunScreen_FollowModeSelectsTheStepThatStarts(t *testing.T) {
	tab := runningScreen(t)
	require.Equal(t, "build", tab.selectedStepName())

	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventStepStarted, Payload: &ports.EnrichedStepPayload{StepName: "test"},
	}})

	assert.Equal(t, "test", tab.selectedStepName(), "the cursor follows the running step")
}

func TestRunScreen_PickingAStepStopsFollowing(t *testing.T) {
	tab := runningScreen(t)
	tab.autoScroll = false // the user scrolled away

	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventStepStarted, Payload: &ports.EnrichedStepPayload{StepName: "test"},
	}})

	assert.Equal(t, "build", tab.selectedStepName(), "a user who left follow mode keeps their selection")
}

func TestRunScreen_StepDurationsComeFromEventTimestamps(t *testing.T) {
	tab := runningScreen(t)
	start := time.Now().Add(-10 * time.Second)
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventStepStarted, Timestamp: start, Payload: &ports.EnrichedStepPayload{StepName: "test"},
	}})
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventStepCompleted, Timestamp: start.Add(4 * time.Second), Payload: &ports.EnrichedStepPayload{StepName: "test"},
	}})

	assert.Equal(t, 4*time.Second, tab.stepDuration("test"))
	assert.Zero(t, tab.stepDuration("never-ran"))
}

func TestRunScreen_KeyBarDependsOnState(t *testing.T) {
	running := runningScreen(t)
	assert.Contains(t, ansi.Strip(running.keys(120)), "cancel run")
	assert.NotContains(t, ansi.Strip(running.keys(120)), "run again")

	running, _ = running.Update(ExecutionFinishedMsg{})
	assert.Contains(t, ansi.Strip(running.keys(120)), "run again")
	assert.Contains(t, ansi.Strip(running.keys(120)), "copy")

	running.inputActive = true
	assert.Contains(t, ansi.Strip(running.keys(120)), "reply")
}

// failRun delivers the terminal failure event the way the facade does.
func failRun(tab RunScreen, message string) RunScreen {
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventWorkflowFailed, Payload: &ports.EnrichedTerminal{Error: message},
	}})
	return tab
}

func TestRunScreen_WorkflowErrorNotTiedToAStepIsShown(t *testing.T) {
	tab := runningScreen(t) // build is running, nothing failed
	tab = failRun(tab, "invalid input: env is required")
	tab, _ = tab.Update(ExecutionFinishedMsg{})

	out := ansi.Strip(tab.renderSelectedStepChat("build"))

	assert.Contains(t, out, labelRunFailure)
	assert.Contains(t, out, "invalid input: env is required")
}

func TestRunScreen_WorkflowErrorIsShownUnderTheFailedStepOnly(t *testing.T) {
	tab := runningScreen(t)
	tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{
		Kind: ports.EventStepCompleted, Payload: &ports.EnrichedStepPayload{StepName: "build", Error: "exit status 2"},
	}})
	tab = failRun(tab, "step build failed")
	tab, _ = tab.Update(ExecutionFinishedMsg{})

	assert.Contains(t, ansi.Strip(tab.renderSelectedStepChat("build")), "step build failed", "shown under the step that failed")
	assert.NotContains(t, ansi.Strip(tab.renderSelectedStepChat("test")), labelRunFailure, "not repeated under unrelated steps")
}

func TestRunScreen_WorkflowErrorIsHiddenWhenItShouldNotBeShown(t *testing.T) {
	tests := []struct {
		name  string
		setup func(RunScreen) RunScreen
	}{
		{"run still in flight", func(tab RunScreen) RunScreen { return failRun(tab, "boom") }},
		{"run cancelled by the user", func(tab RunScreen) RunScreen {
			tab.activeSession = &cancellableSession{}
			tab.RequestCancel()
			tab = failRun(tab, "cancelled")
			tab, _ = tab.Update(ExecutionFinishedMsg{})
			return tab
		}},
		{"run succeeded", func(tab RunScreen) RunScreen {
			tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{Kind: ports.EventWorkflowCompleted}})
			tab.runErr = "stale"
			tab, _ = tab.Update(ExecutionFinishedMsg{})
			return tab
		}},
		{"failure without a message", func(tab RunScreen) RunScreen {
			tab = failRun(tab, "")
			tab, _ = tab.Update(ExecutionFinishedMsg{})
			return tab
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tab := tt.setup(runningScreen(t))
			assert.NotContains(t, ansi.Strip(tab.renderSelectedStepChat("build")), labelRunFailure)
		})
	}
}

func TestRunScreen_SessionErrorIsTheFallbackWhenTheEventHadNone(t *testing.T) {
	t.Run("used when the terminal event carried no message", func(t *testing.T) {
		tab := failRun(runningScreen(t), "")
		tab, _ = tab.Update(ExecutionFinishedMsg{Err: errors.New("session failed: disk full")})

		assert.Contains(t, ansi.Strip(tab.renderSelectedStepChat("build")), "session failed: disk full")
	})
	t.Run("the event message wins over the session error", func(t *testing.T) {
		tab := failRun(runningScreen(t), "the real reason")
		tab, _ = tab.Update(ExecutionFinishedMsg{Err: errors.New("generic")})

		out := ansi.Strip(tab.renderSelectedStepChat("build"))
		assert.Contains(t, out, "the real reason")
		assert.NotContains(t, out, "generic")
	})
}

func TestRunScreen_WorkflowErrorIsClearedByTheNextRun(t *testing.T) {
	tab := failRun(runningScreen(t), "old failure")
	tab, _ = tab.Update(ExecutionFinishedMsg{})

	tab, _ = tab.Update(ExecutionStartedMsg{Workflow: tab.wf, Session: &mockRunSession{}})

	assert.Empty(t, tab.runErr)
}

func TestRunScreen_StepsNeverReachedAreSkippedOnceTheRunIsOver(t *testing.T) {
	tab := runningScreen(t) // build running, test and done pending

	running := ansi.Strip(tab.stepsBox(38, 20))
	assert.Contains(t, running, "○ 2 test")
	assert.Contains(t, running, "pending")
	assert.NotContains(t, running, "skipped", "while the run is in flight a pending step is simply pending")

	tab = failRun(tab, "boom")
	tab, _ = tab.Update(ExecutionFinishedMsg{})
	over := ansi.Strip(tab.stepsBox(38, 20))

	assert.Contains(t, over, "– 2 test")
	assert.Contains(t, over, "skipped")
	assert.NotContains(t, over, "pending")
}

func TestRunScreen_NothingIsSkippedBeforeAnyRunStarted(t *testing.T) {
	tab := monitoringTabWithSize(100, 30)
	tab.SetWorkflow(sampleWorkflow())
	tab.rebuildTree()

	out := ansi.Strip(tab.stepsBox(38, 20))

	assert.NotContains(t, out, "skipped")
	assert.Contains(t, out, "pending")
}

func TestTerminalMatches(t *testing.T) {
	tests := []struct {
		declared workflow.TerminalStatus
		outcome  workflow.ExecutionStatus
		want     bool
	}{
		{workflow.TerminalSuccess, workflow.StatusCompleted, true},
		{workflow.TerminalSuccess, workflow.StatusFailed, false},
		{workflow.TerminalFailure, workflow.StatusFailed, true},
		{workflow.TerminalFailure, workflow.StatusCompleted, false},
		{"", workflow.StatusCompleted, true},
		{"", workflow.StatusFailed, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, terminalMatches(tt.declared, tt.outcome), "%q vs %q", tt.declared, tt.outcome)
	}
}

func TestRunScreen_OnlyTheTerminalMatchingTheOutcomeIsMarked(t *testing.T) {
	// The execution tree only lists the default path, so it normally holds one terminal;
	// the steps are set directly to exercise every kind of terminal at once.
	steps := []workflow.Step{
		{Name: "work", Type: workflow.StepTypeCommand, Command: "true"},
		{Name: "ok", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		{Name: "bad", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
		{Name: "unknown", Type: workflow.StepTypeTerminal},
	}
	statusOf := func(tab RunScreen, name string) workflow.ExecutionStatus {
		for _, n := range tab.flatNodes {
			if n.Name == name {
				return n.Status
			}
		}
		t.Fatalf("no node %q", name)
		return ""
	}
	run := func(event ports.EventKind) RunScreen {
		tab := monitoringTabWithSize(100, 30)
		tab.steps = steps
		tab.wf = &workflow.Workflow{Name: "wf"}
		tab, _ = tab.Update(ExecutionStartedMsg{Workflow: tab.wf, Session: &mockRunSession{}})
		tab, _ = tab.Update(facadeEventMsg{Event: ports.Event{Kind: event}})
		tab, _ = tab.Update(ExecutionFinishedMsg{})
		return tab
	}

	failed := run(ports.EventWorkflowFailed)
	assert.Equal(t, workflow.StatusFailed, statusOf(failed, "bad"))
	assert.Equal(t, workflow.StatusPending, statusOf(failed, "ok"), "a failed run did not reach the success terminal")
	assert.Equal(t, workflow.StatusFailed, statusOf(failed, "unknown"), "a terminal without an outcome is still inferred")

	done := run(ports.EventWorkflowCompleted)
	assert.Equal(t, workflow.StatusCompleted, statusOf(done, "ok"))
	assert.Equal(t, workflow.StatusPending, statusOf(done, "bad"), "a successful run did not reach the failure terminal")
}
