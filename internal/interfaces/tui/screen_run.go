package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// RunScreen displays a live (or just finished) workflow run: the step tree on the left
// and the selected step's input, conversation and output on the right.
//
// The screen is split across three files by responsibility:
//   - screen_run.go         state, Update, View and layout
//   - screen_run_events.go  facade event consumption and live step output
//   - screen_run_render.go  pure rendering of steps, conversations and the tree
//
// State updates are event-driven: StartEventLoop consumes ports.RunSession.Events() and
// forwards each facade event into the update loop as a facadeEventMsg.
type RunScreen struct {
	width  int
	height int

	// Execution state for tree rendering.
	wf     *workflow.Workflow // running workflow (for step ordering in BuildTree)
	steps  []workflow.Step    // ordered step slice derived from wf
	states map[string]workflow.StepState
	// finalStatus carries the workflow's terminal outcome on the facade (event-driven)
	// path. Terminal steps never emit step events, so rebuildTree uses this to mark the
	// reached terminal step completed/failed once the workflow ends.
	finalStatus workflow.ExecutionStatus

	// live holds the stdout/stderr streamed by ports.EventStepOutput for steps that have
	// not completed yet. A completed step's authoritative output replaces its live entry.
	live map[string]*liveOutput

	// Tree navigation.
	flatNodes   []*TreeNode
	selectedIdx int

	// Output viewport.
	vp         viewport.Model
	autoScroll bool

	// running is true while a run is in flight (between ExecutionStartedMsg and
	// ExecutionFinishedMsg); the status bar and tree inference both depend on it.
	// It never drives a timer: updates are pushed by events, not polled.
	running bool

	// Spinner shown while waiting for the first facade event after ExecutionStartedMsg.
	spinner     spinner.Model
	showSpinner bool

	// lastLaunch remembers the most recent launch request so the run can be repeated.
	lastLaunch LaunchWorkflowMsg
	// cancelling is true once the user asked to cancel and the terminal event is pending.
	cancelling bool
	// cancelledRun records that the run that just ended was cancelled by the user, so the
	// header can say "cancelled" instead of "failed".
	cancelledRun bool
	// runErr is why the workflow failed when that is not tied to a step: the message of the
	// terminal event, or the session's error when the event carried none.
	runErr string
	// notice is a transient one-line message (copy confirmation) cleared on the next key.
	notice string

	// runID is the facade session identifier, shown in the header.
	runID string
	// startedAt/endedAt bound the run for the elapsed clock in the header.
	startedAt time.Time
	endedAt   time.Time
	// begin/end record when each step started and finished, for per-step durations.
	begin map[string]time.Time
	end   map[string]time.Time
	// inputs are the values the run was launched with (shown masked in the Steps box).
	inputs map[string]any
	// tick is the animation frame, advanced by the Model while a run is in flight.
	tick int

	// Conversation input.
	inputReader   *TUIInputReader
	activeSession ports.RunSession
	inputField    textinput.Model
	inputActive   bool
	// agentStep names the currently-running agent/conversation step. Facade message events
	// (EventMessageUser/Assistant) carry no step name, so they are attributed to this step
	// to build its conversation view on the event-driven path.
	agentStep string

	// senderRef forwards tea.Msg values from the event goroutine into the Bubble Tea loop.
	// It is a POINTER holder so it survives the value copy tea.NewProgram makes of the Model:
	// SetSender is called AFTER the program is created (p.Send is only available then), and
	// only a shared-pointer field (like inputReader) reaches the program's model copy. A bare
	// func field set post-NewProgram would be lost on the copy.
	senderRef *msgSenderRef
}

// msgSenderRef holds the Bubble Tea message dispatcher behind a pointer (see senderRef).
type msgSenderRef struct{ fn func(tea.Msg) }

func newRunScreen() RunScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorRunning)

	ti := textinput.New()
	ti.Placeholder = labelRunInputPlaceholder
	ti.CharLimit = 4096

	return RunScreen{
		vp:         viewport.New(),
		autoScroll: true,
		states:     make(map[string]workflow.StepState),
		live:       make(map[string]*liveOutput),
		begin:      make(map[string]time.Time),
		end:        make(map[string]time.Time),
		spinner:    sp,
		inputField: ti,
		senderRef:  &msgSenderRef{},
	}
}

// Init implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t RunScreen) Init() tea.Cmd {
	return nil
}

// Update implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t RunScreen) Update(msg tea.Msg) (RunScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.resizeViewport()
		if t.inputActive {
			t.resizeInputField()
		}
		return t, nil

	case ExecutionStartedMsg:
		t.startRun(msg)
		return t, nil

	case ExecutionFinishedMsg:
		if t.runErr == "" && msg.Err != nil {
			t.runErr = msg.Err.Error()
		}
		t.finishRun()
		return t, nil

	case facadeEventMsg:
		t.showSpinner = false
		t.applyFacadeEvent(msg.Event)
		if msg.Event.Kind == ports.EventInputRequired {
			t.beginInputRequest()
		}
		return t, nil

	case spinner.TickMsg:
		if t.showSpinner {
			var cmd tea.Cmd
			t.spinner, cmd = t.spinner.Update(msg)
			return t, cmd
		}
		return t, nil

	case InputRequestedMsg:
		t.beginInputRequest()
		return t, nil

	case tea.KeyPressMsg:
		if next, cmd, handled := t.handleKey(msg); handled {
			return next, cmd
		}
	}

	// Delegate viewport scroll events (page up/down, mouse wheel) to the viewport.
	var vpCmd tea.Cmd
	t.vp, vpCmd = viewportAutoScroll(t.vp, &t.autoScroll, msg)
	return t, vpCmd
}

// startRun resets the screen for a new run. State updates then arrive through
// facadeEventMsg (see StartEventLoop); there is no polling fallback.
func (t *RunScreen) startRun(msg ExecutionStartedMsg) {
	t.selectedIdx = 0
	t.states = make(map[string]workflow.StepState)
	t.live = make(map[string]*liveOutput)
	t.begin = make(map[string]time.Time)
	t.end = make(map[string]time.Time)
	t.runID = ""
	if msg.Session != nil {
		t.runID = msg.Session.ID()
	}
	t.inputs = t.lastLaunch.Inputs
	t.startedAt = time.Now()
	t.endedAt = time.Time{}
	t.finalStatus = "" // clear any previous run's outcome so terminal steps start pending
	t.activeSession = msg.Session
	t.agentStep = ""
	t.autoScroll = true
	t.cancelling = false
	t.cancelledRun = false
	t.runErr = ""
	t.notice = ""
	t.inputActive = false
	t.inputField.Reset()
	t.inputField.Blur()
	t.running = msg.Session != nil
	t.showSpinner = msg.Session != nil
	t.rebuildTree()
	t.resizeViewport()
	t.updateViewportContent()
}

// finishRun clears the in-flight indicators and re-renders so the terminal outcome
// (applied by applyFacadeEvent on the terminal event) is reflected in the tree.
func (t *RunScreen) finishRun() {
	if t.running {
		t.endedAt = time.Now()
	}
	if t.cancelling {
		t.cancelledRun = true
	}
	t.running = false
	t.showSpinner = false
	t.cancelling = false
	t.activeSession = nil
	t.inputActive = false
	t.inputField.Blur()
	t.resizeViewport()
	t.rebuildTree()
	t.updateViewportContent()
}

// handleKey processes a key press. handled is false when the key should fall through to
// the viewport (page up/down and the like).
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t RunScreen) handleKey(msg tea.KeyPressMsg) (RunScreen, tea.Cmd, bool) {
	if t.inputActive {
		next, cmd := t.handleInputKey(msg)
		return next, cmd, true
	}
	t.notice = ""

	switch {
	case key.Matches(msg, keyUp):
		t.moveSelection(-1)
		return t, nil, true

	case key.Matches(msg, keyDown):
		t.moveSelection(1)
		return t, nil, true

	case key.Matches(msg, keyFollow):
		t.autoScroll = true
		t.vp.GotoBottom()
		return t, nil, true

	case key.Matches(msg, keyCancel):
		t.RequestCancel()
		return t, nil, true

	case key.Matches(msg, keyRerun):
		if t.running || t.lastLaunch.Workflow == nil {
			return t, nil, true
		}
		launch := t.lastLaunch
		return t, func() tea.Msg { return launch }, true

	case key.Matches(msg, keyCopy):
		text := t.copyText()
		if text == "" {
			t.notice = labelRunNothingToCopy
			return t, nil, true
		}
		t.notice = fmt.Sprintf(labelRunCopiedFmt, len(text))
		return t, tea.SetClipboard(text), true
	}
	return t, nil, false
}

// handleInputKey routes keys while the conversation text field is focused.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t RunScreen) handleInputKey(msg tea.KeyPressMsg) (RunScreen, tea.Cmd) {
	switch {
	case key.Matches(msg, keySelect):
		text := t.inputField.Value()
		t.inputField.Reset()
		t.inputActive = false
		t.inputField.Blur()
		t.resizeViewport()
		if t.activeSession != nil {
			if err := t.activeSession.Respond(ports.InputResponse{Value: text}); err != nil {
				return t, nil
			}
		} else if t.inputReader != nil {
			t.inputReader.Respond(text)
		}
		t.updateViewportContent()
		return t, nil
	case msg.Code == tea.KeyUp:
		t.moveSelection(-1)
		return t, nil
	case msg.Code == tea.KeyDown:
		t.moveSelection(1)
		return t, nil
	}
	var cmd tea.Cmd
	t.inputField, cmd = t.inputField.Update(msg)
	return t, cmd
}

// moveSelection moves the tree cursor by delta and refreshes the output panel.
func (t *RunScreen) moveSelection(delta int) {
	next := t.selectedIdx + delta
	if next < 0 || next >= len(t.flatNodes) {
		return
	}
	t.selectedIdx = next
	t.updateViewportContent()
}

// cancelSession asks the run to stop. It mirrors the HTTP bridge: a session that can
// publish a terminal "cancelled" event is cancelled that way; otherwise it is closed.
// It reports whether the session was closed directly (no terminal event will follow).
func cancelSession(sess ports.RunSession) (closedDirectly bool) {
	if sess == nil {
		return true
	}
	if c, ok := sess.(interface{ CancelWithEvent(reason string) error }); ok {
		_ = c.CancelWithEvent("cancelled") //nolint:errcheck // best-effort: the terminal event reports the outcome
		return false
	}
	_ = sess.Close() //nolint:errcheck // Close is idempotent; the run is being abandoned
	return true
}

// RequestCancel asks the running run to stop. It does nothing when no run is in flight
// or a cancellation is already pending.
func (t *RunScreen) RequestCancel() {
	if !t.running || t.cancelling {
		return
	}
	t.cancelling = true
	if cancelSession(t.activeSession) {
		// The session was closed directly, so no terminal event will arrive.
		t.finishRun()
	}
}

// SetWorkflow wires the running workflow so the screen can order steps and render the
// execution tree. Called by the Model when it receives ExecutionStartedMsg.
func (t *RunScreen) SetWorkflow(wf *workflow.Workflow) {
	t.wf = wf
	if wf != nil {
		t.steps = workflow.ExecutionOrder(wf)
	}
}

// SetLaunch records the launch request so the run can be repeated with `r`.
func (t *RunScreen) SetLaunch(msg LaunchWorkflowMsg) {
	t.lastLaunch = msg
}

// SetInputReader wires the TUIInputReader so the screen can respond to conversation
// input requests and route user replies back to the blocked ReadInput call.
func (t *RunScreen) SetInputReader(r *TUIInputReader) {
	t.inputReader = r
}

// SetSender wires the tea.Msg dispatcher so StartEventLoop can forward facade events
// into the Bubble Tea update loop. Call this after tea.NewProgram, passing p.Send.
func (t *RunScreen) SetSender(send func(tea.Msg)) {
	if t.senderRef == nil {
		t.senderRef = &msgSenderRef{}
	}
	t.senderRef.fn = send
}

// InputActive reports whether the conversation text input is focused.
func (t RunScreen) InputActive() bool { //nolint:gocritic // read-only
	return t.inputActive
}

func (t *RunScreen) beginInputRequest() {
	t.inputActive = true
	t.resizeInputField()
	t.resizeViewport()
	_ = t.inputField.Focus()
	t.autoSelectRunning()
	t.updateViewportContent()
}

// SetTick sets the animation frame (spinner glyph, elapsed clock refresh).
func (t *RunScreen) SetTick(n int) { t.tick = n }

// Running reports whether a run is in flight.
func (t *RunScreen) Running() bool { return t.running }

// HasRun reports whether the screen has a run (live or finished) to show.
func (t *RunScreen) HasRun() bool { return t.wf != nil }

// View renders the run body at the screen's own size.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t RunScreen) View() string {
	if t.wf == nil && !t.inputActive {
		return EmptyStateView("📡", labelRunEmptyTitle, labelRunEmptySubtitle)
	}
	if t.width <= 0 || t.height <= 0 {
		return StyleEmptyState.Render(labelRunWaitingDimensions)
	}
	return t.view(t.width, t.height)
}

// title is the header title: the workflow name and the run identifier.
func (t *RunScreen) title() string {
	name := labelRunOutputTitle
	if t.wf != nil {
		name = t.wf.Name
	}
	if t.runID == "" {
		return name
	}
	return name + " " + sDim.Render(fmt.Sprintf(labelRunIDFmt, shortID(t.runID)))
}

func (t *RunScreen) elapsed() time.Duration {
	if t.startedAt.IsZero() {
		return 0
	}
	if !t.endedAt.IsZero() {
		return t.endedAt.Sub(t.startedAt)
	}
	return time.Since(t.startedAt)
}

// statusText is the right side of the header: outcome or live state with the clock.
func (t *RunScreen) statusText() string {
	clock := fmtClock(t.elapsed())
	switch {
	case t.running && t.inputActive:
		return sRun.Render(fmt.Sprintf(labelRunWaitingReplyFmt, clock))
	case t.running && t.cancelling:
		return sRun.Render(fmt.Sprintf(labelRunCancellingFmt, clock))
	case t.running:
		return sRun.Render(fmt.Sprintf(labelRunRunningFmt, spinnerFrame(t.tick), clock))
	case t.finalStatus == workflow.StatusCompleted:
		return sOK.Render(fmt.Sprintf(labelRunDoneFmt, clock))
	case t.cancelledRun:
		return sDim.Render(fmt.Sprintf(labelRunCancelledFmt, clock))
	case t.finalStatus == workflow.StatusFailed:
		return sBad.Render(fmt.Sprintf(labelRunFailedFmt, clock))
	}
	return ""
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinnerFrame(tick int) string {
	return spinnerFrames[((tick%len(spinnerFrames))+len(spinnerFrames))%len(spinnerFrames)]
}

// keys is the key bar for the current state.
func (t *RunScreen) keys(w int) string {
	if t.inputActive {
		return keyBar(w, keyHint{"enter", labelKeyReply}, keyHint{"esc", labelKeyBackRunContinues})
	}
	items := []keyHint{{"↑↓", labelKeyPickStep}, {"f", labelKeyFollow}, {"pgup/dn", labelKeyScroll}}
	if t.running {
		items = append(items, keyHint{"c", labelKeyCancelRun})
	} else {
		items = append(items, keyHint{"r", labelKeyRerun}, keyHint{"y", labelKeyCopy})
	}
	items = append(items, keyHint{"esc", labelKeyBackRunContinues})
	return keyBar(w, items...)
}

// view draws the progress line and the Steps and Output boxes in a w x h area.
func (t *RunScreen) view(w, h int) string {
	total := max(1, len(t.steps))
	done := 0
	for _, n := range t.flatNodes {
		if n.Status == workflow.StatusCompleted {
			done++
		}
	}
	label := fmt.Sprintf(labelRunProgressFmt, done, len(t.steps)) + "  " + sDim.Render(fmt.Sprintf("%d%%", done*100/total))
	barW := max(10, w-lipgloss.Width(label)-2)
	progress := progressBar(barW, float64(done)/float64(total)) + "  " + label

	lw, rw := t.panelWidths()
	bh := max(h-1, 3)
	out := t.outputBox(rw, bh)
	return progress + "\n" + joinH(t.stepsBox(lw, bh), out, 1)
}

// stepsBox lists the steps with their status, number and duration, then the run inputs.
func (t *RunScreen) stepsBox(w, h int) string {
	inner := w - 4
	lines := make([]string, 0, h)
	over := t.runIsOver()
	for i, n := range t.flatNodes {
		icon := statusIcon(string(n.Status))
		right := sDim.Render(labelRunStepPending)
		switch n.Status { //nolint:exhaustive // other statuses fall through to the pending label
		case workflow.StatusPending:
			if over {
				// The run ended without ever reaching this step.
				icon, right = statusIcon("skipped"), sDim.Render(labelRunStepSkipped)
			}
		case workflow.StatusRunning:
			icon = sRun.Render(spinnerFrame(t.tick))
			right = sRun.Render(fmtDur(time.Since(t.begin[n.Name])))
		case workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled:
			right = sDim.Render(fmtDur(t.stepDuration(n.Name)))
		}
		line := row(fmt.Sprintf("%s%s %d %s", strings.Repeat("  ", n.Depth), icon, i+1, n.Name), right, inner)
		if i == t.selectedIdx {
			line = selRow(line, inner, cSelBg, true)
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		msg := labelRunWaitingStart
		if t.showSpinner {
			msg = spinnerFrame(t.tick) + " " + labelRunStarting
		}
		lines = append(lines, sDim.Render(msg))
	}

	keys := sortedKeys(t.inputs)
	room := h - 2 - len(keys) - 1
	for len(lines) < room {
		lines = append(lines, "")
	}
	if len(keys) > 0 {
		lines = append(lines, sDim.Render(labelRunInputsTitle))
		for _, k := range keys {
			lines = append(lines, sPurple.Render(k)+" "+maskValue(k, fmt.Sprint(t.inputs[k])))
		}
	}
	return box(labelRunStepsTitle, lines, w, h, false)
}

// runIsOver reports whether a run was started and has ended.
func (t *RunScreen) runIsOver() bool { return !t.running && !t.startedAt.IsZero() }

func (t *RunScreen) stepDuration(name string) time.Duration {
	b, ok := t.begin[name]
	if !ok {
		return 0
	}
	e, ok := t.end[name]
	if !ok {
		return 0
	}
	return e.Sub(b)
}

// outputBox shows the selected step's input, conversation and output, with the reply field
// underneath while the run waits for the user.
func (t *RunScreen) outputBox(w, h int) string {
	title := labelRunOutputTitle
	if name := t.selectedStepName(); name != "" {
		title = fmt.Sprintf("%s · %s", labelRunOutputTitle, name)
		if live := t.live[name]; live != nil && t.running {
			title += " " + labelRunLiveSuffix
		}
	}
	switch {
	case t.cancelling && t.running:
		title += " · " + labelRunCancelling
	case t.notice != "":
		title += " · " + t.notice
	}

	if !t.inputActive {
		return box(title, strings.Split(t.vp.View(), "\n"), w, h, true)
	}
	const inputRows = 3
	top := box(title, strings.Split(t.vp.View(), "\n"), w, max(h-inputRows, 3), false)
	reply := box(labelRunReplyTitle, []string{t.inputField.View()}, w, inputRows, true)
	return top + "\n" + reply
}

// resizeInputField sets the textinput width to fill the output box.
func (t *RunScreen) resizeInputField() {
	_, rw := t.panelWidths()
	t.inputField.SetWidth(max(rw-6, 1))
}

// resizeViewport updates the viewport dimensions to the output box's inner area.
func (t *RunScreen) resizeViewport() {
	_, rw := t.panelWidths()
	h := t.panelHeight()
	if t.inputActive {
		h = max(h-3, 3)
	}
	t.vp.SetWidth(max(rw-4, 1))
	t.vp.SetHeight(max(h-2, 1))
}

// panelWidths returns the Steps and Output box widths.
func (t RunScreen) panelWidths() (int, int) { //nolint:gocritic // value receiver: read-only
	left := max(min(38, t.width/3), 1)
	return left, max(t.width-left-1, 1)
}

// panelHeight returns the height of the two boxes (the progress line sits above them).
func (t RunScreen) panelHeight() int { //nolint:gocritic // value receiver: read-only
	return max(t.height-1, 3)
}

// RenderEventsToTUIMsgs projects facade events to their TUI tea.Msg representation.
// It backs the facade conformance golden (tests/fixtures/facade/tui-tea-msg.golden).
func RenderEventsToTUIMsgs(events []ports.Event) []byte {
	var b strings.Builder
	for _, ev := range events {
		fmt.Fprintf(&b, "facadeEventMsg{Seq:%d Kind:%q RunID:%q}\n",
			ev.Seq, ev.Kind.String(), ev.RunID)
	}
	return []byte(b.String())
}
