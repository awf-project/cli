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

	"github.com/awf-project/cli/internal/domain/workflow"
)

// stepInputText returns a short human-readable description of a step's input
// for display in the chat-style conversation view.
func stepInputText(step *workflow.Step) string {
	switch {
	case step.Agent != nil:
		if step.Agent.Prompt != "" {
			return step.Agent.Prompt
		}
		if step.Agent.PromptFile != "" {
			return "📄 " + step.Agent.PromptFile
		}
		return ""
	case step.Command != "":
		return "$ " + step.Command
	case step.ScriptFile != "":
		return "📄 " + step.ScriptFile
	case step.Operation != "":
		text := step.Operation
		if len(step.OperationInputs) > 0 {
			parts := make([]string, 0, len(step.OperationInputs))
			for k, v := range step.OperationInputs {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			text += " (" + strings.Join(parts, ", ") + ")"
		}
		return text
	case step.Type == workflow.StepTypeParallel:
		return "branches: " + strings.Join(step.Branches, ", ")
	case step.Type == workflow.StepTypeTerminal:
		if step.Message != "" {
			return step.Message
		}
		return string(step.Status)
	default:
		return ""
	}
}

// renderTruncated returns lines capped at maxLines, appending a muted
// "… (N more lines)" indicator when the input exceeds the limit.
func renderTruncated(lines []string, maxLines int) []string {
	if len(lines) <= maxLines {
		return lines
	}
	result := make([]string, maxLines+1)
	copy(result, lines[:maxLines])
	result[maxLines] = lipgloss.NewStyle().Foreground(ColorMuted).Render(
		fmt.Sprintf("… (%d more lines)", len(lines)-maxLines),
	)
	return result
}

// monitoringTickInterval defines the polling interval for execution state updates.
const monitoringTickInterval = 200 * time.Millisecond

// tickMsg is an internal message that drives the execution state polling loop.
type tickMsg struct{}

// executionPollMsg carries a fresh snapshot of step states after a poll.
type executionPollMsg struct {
	States map[string]workflow.StepState
}

// Lipgloss styles for the monitoring tab.
var (
	monitoringSelectedStyle = StyleSelectedRow

	monitoringEmptyStyle = StyleEmptyState
)

// MonitoringTab displays live execution metrics and history for the TUI monitoring view.
// It renders a two-panel layout: execution tree (left) and a log viewport (right).
// A 200 ms tick loop polls the active ExecutionContext when an execution is running.
type MonitoringTab struct {
	width  int
	height int

	// Legacy fields preserved for model.go and model_test.go compatibility.
	stats   *workflow.HistoryStats
	history []*workflow.ExecutionRecord
	active  *executionState

	// Execution state for tree rendering.
	execCtx *workflow.ExecutionContext // nil when no execution is active
	wf      *workflow.Workflow         // running workflow (for step ordering in BuildTree)
	steps   []workflow.Step            // ordered step slice derived from wf
	states  map[string]workflow.StepState

	// Tree navigation.
	flatNodes   []*TreeNode
	selectedIdx int

	// Streaming output from execution.
	stream     *StreamBuffer
	lastStream int // byte offset already rendered

	// Log viewport.
	vp         viewport.Model
	autoScroll bool

	// Tick management.
	ticking bool

	// Spinner shown while waiting for execCtx to be wired after ExecutionStartedMsg.
	spinner     spinner.Model
	showSpinner bool

	// Conversation input and streaming display.
	inputReader   *TUIInputReader
	inputField    textinput.Model
	inputActive   bool
	convBuf       *strings.Builder
	convStep      string
	convTurnCount int
}

func newMonitoringTab() MonitoringTab {
	vp := viewport.New()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(ColorRunning)

	ti := textinput.New()
	ti.Placeholder = "Type your message (empty to end conversation)..."
	ti.CharLimit = 4096

	return MonitoringTab{
		vp:         vp,
		autoScroll: true,
		states:     make(map[string]workflow.StepState),
		spinner:    sp,
		inputField: ti,
		convBuf:    &strings.Builder{},
	}
}

// Init implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t MonitoringTab) Init() tea.Cmd {
	return nil
}

// Update implements the Bubbletea sub-model convention.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t MonitoringTab) Update(msg tea.Msg) (MonitoringTab, tea.Cmd) { //nolint:cyclop,gocognit // Monitoring tab handles many message types by design; conversation input adds necessary branching
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
		t.selectedIdx = 0
		t.states = make(map[string]workflow.StepState)
		t.autoScroll = true
		t.showSpinner = false
		t.convReset()
		t.rebuildTree()
		if !t.ticking {
			t.ticking = true
			return t, scheduleTick()
		}
		return t, nil

	case ExecutionFinishedMsg:
		t.ticking = false
		t.showSpinner = false
		if t.execCtx != nil {
			t.states = t.execCtx.GetAllStepStates()
			t.rebuildTree()
			t.updateViewportContent()
		}
		return t, nil

	case tickMsg:
		if !t.ticking {
			return t, nil
		}
		// Poll execution context directly — GetAllStepStates is thread-safe.
		if t.execCtx != nil {
			states := t.execCtx.GetAllStepStates()
			return t, func() tea.Msg {
				return executionPollMsg{States: states}
			}
		}
		return t, scheduleTick()

	case executionPollMsg:
		t.showSpinner = false
		t.states = msg.States
		t.rebuildTree()
		t.updateViewportContent()
		t.autoSelectFailed()

		if t.ticking {
			return t, scheduleTick()
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
		t.inputActive = true
		t.resizeInputField()
		t.resizeViewport()
		_ = t.inputField.Focus()
		t.autoSelectRunning()
		// Direct state read — the poll may not have fired yet after SetStepState.
		if t.execCtx != nil {
			t.states = t.execCtx.GetAllStepStates()
		}
		// Only append new turns during an active conversation; never restart
		// tracking (convStart resets convTurnCount, duplicating user messages
		// already rendered by convAppendUser).
		if t.convStep != "" {
			if state, ok := t.states[t.convStep]; ok && state.Conversation != nil {
				t.convAppendNewTurns(state.Conversation)
			}
		} else if len(t.flatNodes) > 0 && t.selectedIdx >= 0 && t.selectedIdx < len(t.flatNodes) {
			name := t.flatNodes[t.selectedIdx].Name
			if state, ok := t.states[name]; ok && state.Conversation != nil {
				t.convStart(name, state.Conversation)
			}
		}
		t.convApplyToViewport()
		return t, nil

	case tea.KeyPressMsg:
		if t.inputActive {
			switch {
			case key.Matches(msg, keySelect) && t.inputReader != nil:
				text := t.inputField.Value()
				t.inputField.Reset()
				t.inputActive = false
				t.inputField.Blur()
				t.resizeViewport()
				if t.stream != nil {
					t.stream.Reset()
				}
				t.convApplyToViewport()
				t.inputReader.Respond(text)
				return t, nil
			case msg.Code == tea.KeyUp:
				if t.selectedIdx > 0 {
					t.selectedIdx--
					t.updateViewportContent()
				}
				return t, nil
			case msg.Code == tea.KeyDown:
				if t.selectedIdx < len(t.flatNodes)-1 {
					t.selectedIdx++
					t.updateViewportContent()
				}
				return t, nil
			}
			var cmd tea.Cmd
			t.inputField, cmd = t.inputField.Update(msg)
			return t, cmd
		}

		switch {
		case key.Matches(msg, keyUp):
			if t.selectedIdx > 0 {
				t.selectedIdx--
				t.updateViewportContent()
			}
			return t, nil

		case key.Matches(msg, keyDown):
			if t.selectedIdx < len(t.flatNodes)-1 {
				t.selectedIdx++
				t.updateViewportContent()
			}
			return t, nil

		case key.Matches(msg, keyFollow):
			t.autoScroll = true
			t.vp.GotoBottom()
			return t, nil
		}
	}

	// Delegate viewport scroll events (page up/down, mouse wheel) to the viewport.
	var vpCmd tea.Cmd
	t.vp, vpCmd = viewportAutoScroll(t.vp, &t.autoScroll, msg)
	return t, vpCmd
}

// SetExecCtx wires the active ExecutionContext so the tick loop can poll it.
// Called by the Model when it receives ExecutionStartedMsg with a real context.
func (t *MonitoringTab) SetExecCtx(ctx *workflow.ExecutionContext, wf *workflow.Workflow) {
	t.execCtx = ctx
	t.wf = wf
	if wf != nil {
		t.steps = orderedSteps(wf)
	}
}

// SetStream wires the streaming output buffer for live viewport display.
func (t *MonitoringTab) SetStream(s *StreamBuffer) {
	t.stream = s
	t.lastStream = 0
}

// SetInputReader wires the TUIInputReader so the tab can respond to conversation
// input requests and route user replies back to the blocked ReadInput call.
func (t *MonitoringTab) SetInputReader(r *TUIInputReader) {
	t.inputReader = r
}

// InputActive reports whether the conversation text input is focused.
func (t MonitoringTab) InputActive() bool { //nolint:gocritic // read-only
	return t.inputActive
}

// View renders the monitoring tab.
//
//nolint:gocritic // Bubbletea convention: value receivers return a new model on each update
func (t MonitoringTab) View() string {
	if t.execCtx == nil && t.active == nil && t.wf == nil {
		return EmptyStateView("📡", "No active execution", "Launch a workflow from the Workflows tab.")
	}

	if t.width <= 0 || t.height <= 0 {
		return monitoringEmptyStyle.Render("Waiting for terminal dimensions…")
	}

	leftWidth, rightWidth := t.panelWidths()
	fullHeight := t.panelHeight()

	// Progress calculation.
	completed := 0
	total := len(t.flatNodes)
	for _, node := range t.flatNodes {
		switch node.Status { //nolint:exhaustive // only counting terminal statuses
		case workflow.StatusCompleted, workflow.StatusFailed, workflow.StatusCancelled:
			completed++
		}
	}
	treeTitle := fmt.Sprintf("Execution (%d/%d)", completed, total)

	logTitle := "Output"
	if len(t.flatNodes) > 0 && t.selectedIdx >= 0 && t.selectedIdx < len(t.flatNodes) {
		selected := t.flatNodes[t.selectedIdx]
		logTitle = selected.Name + " " + StatusBadge(selected.Status)
	}

	treeContent := t.renderTreeWithSelection()
	leftPanel := Panel(treeTitle, treeContent, leftWidth, fullHeight, true)

	if t.inputActive {
		// Right panel is shorter; input field fills the remaining space.
		// Input renders 3 lines (border-top + content + border-bottom).
		// Panel border adds 2 to the Height value, so: rightHeight + 2 + 3 = fullHeight + 2.
		const inputRows = 3
		rightHeight := fullHeight - inputRows
		if rightHeight < 3 {
			rightHeight = 3
		}
		rightPanel := Panel(logTitle, t.vp.View(), rightWidth, rightHeight, false)
		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimaryLight).
			Width(rightWidth - 2)
		rightColumn := lipgloss.JoinVertical(lipgloss.Left, rightPanel, inputStyle.Render(t.inputField.View()))
		return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightColumn)
	}

	rightPanel := Panel(logTitle, t.vp.View(), rightWidth, fullHeight, false)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

// scheduleTick returns a tea.Cmd that fires a tickMsg after monitoringTickInterval.
func scheduleTick() tea.Cmd {
	return tea.Tick(monitoringTickInterval, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// rebuildTree reconstructs flatNodes from the current steps and states.
func (t *MonitoringTab) rebuildTree() {
	nodes := BuildTree(t.steps, t.states)
	t.flatNodes = flattenTree(nodes)

	// Infer terminal step status from execution context when not tracked in states.
	if t.execCtx != nil && !t.ticking {
		for _, node := range t.flatNodes {
			if _, hasState := t.states[node.Name]; hasState {
				continue
			}
			if step, ok := t.findStep(node.Name); ok && step.Type == workflow.StepTypeTerminal {
				switch t.execCtx.Status {
				case workflow.StatusCompleted:
					node.Status = workflow.StatusCompleted
				case workflow.StatusFailed:
					node.Status = workflow.StatusFailed
				default:
					node.Status = t.execCtx.Status
				}
			}
		}
	}

	// Clamp selectedIdx within new bounds.
	if len(t.flatNodes) == 0 {
		t.selectedIdx = 0
	} else if t.selectedIdx >= len(t.flatNodes) {
		t.selectedIdx = len(t.flatNodes) - 1
	}
}

// findStep returns the step definition by name.
func (t *MonitoringTab) findStep(name string) (*workflow.Step, bool) {
	for i := range t.steps {
		if t.steps[i].Name == name {
			return &t.steps[i], true
		}
	}
	return nil, false
}

// updateViewportContent refreshes the right-panel viewport with the selected step's chat view.
func (t *MonitoringTab) updateViewportContent() {
	if len(t.flatNodes) == 0 || t.selectedIdx < 0 || t.selectedIdx >= len(t.flatNodes) {
		t.vp.SetContent("")
		return
	}

	name := t.flatNodes[t.selectedIdx].Name
	state, ok := t.states[name]

	// Auto-start or continue incremental conversation rendering.
	if ok && state.Conversation != nil && len(state.Conversation.Turns) > 0 {
		if name != t.convStep {
			t.convStart(name, state.Conversation)
		} else {
			t.convAppendNewTurns(state.Conversation)
		}
		t.convApplyToViewport()
		return
	}

	// Non-conversation step: full rebuild.
	if t.convStep != "" {
		t.convReset()
	}
	content := t.renderSelectedStepChat(name)
	t.vp.SetContent(content)
	if t.autoScroll {
		t.vp.GotoBottom()
	}
}

// renderSelectedStepChat builds the chat view for a given step.
func (t *MonitoringTab) renderSelectedStepChat(name string) string {
	step, _ := t.findStep(name)
	state, hasState := t.states[name]

	if !hasState && step != nil && step.Type == workflow.StepTypeTerminal && t.execCtx != nil && !t.ticking {
		state = workflow.StepState{
			Name:   name,
			Status: t.execCtx.Status,
		}
		hasState = true
	}

	if t.ticking && t.stream != nil && t.stream.Len() > 0 && hasState && state.Status == workflow.StatusRunning {
		return t.renderStepBlock(step, &state, hasState) + "\n" + t.stream.String()
	}

	return t.renderStepBlock(step, &state, hasState)
}

// --- Conversation streaming helpers ---

// convReset clears the conversation buffer when switching away from a conversation step.
func (t *MonitoringTab) convReset() {
	t.convBuf.Reset()
	t.convStep = ""
	t.convTurnCount = 0
}

// convStart initializes incremental conversation rendering for the given step.
func (t *MonitoringTab) convStart(stepName string, conv *workflow.ConversationState) {
	t.convReset()
	t.convStep = stepName
	t.convAppendNewTurns(conv)
}

// convAppendNewTurns renders only turns that haven't been rendered yet and appends to the buffer.
func (t *MonitoringTab) convAppendNewTurns(conv *workflow.ConversationState) {
	inputStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimaryLight)
	outputStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	dividerStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	for i := t.convTurnCount; i < len(conv.Turns); i++ {
		turn := &conv.Turns[i]
		switch turn.Role {
		case workflow.TurnRoleSystem:
			// skip
		case workflow.TurnRoleUser:
			if t.convBuf.Len() > 0 {
				t.convBuf.WriteString(dividerStyle.Render("────────────────────────────") + "\n\n")
			}
			t.convBuf.WriteString(inputStyle.Render("▶ User:") + "\n")
			for _, l := range strings.Split(turn.Content, "\n") {
				t.convBuf.WriteString("  " + l + "\n")
			}
			t.convBuf.WriteString("\n")
		case workflow.TurnRoleAssistant:
			t.convBuf.WriteString(outputStyle.Render("◀ Agent:") + "\n")
			// Prefer filtered stream content (human-readable) over raw turn
			// content which may contain unparsed JSONL from the provider CLI.
			displayText := turn.Content
			if t.stream != nil && t.stream.Len() > 0 {
				displayText = strings.TrimRight(t.stream.String(), "\n")
				t.stream.Reset()
			}
			for _, l := range strings.Split(displayText, "\n") {
				t.convBuf.WriteString("  " + l + "\n")
			}
			t.convBuf.WriteString("\n")
		}
	}
	t.convTurnCount = len(conv.Turns)
}

// convApplyToViewport sets the viewport content from the conversation buffer.
// During agent execution, live streaming output is shown as the in-progress response.
func (t *MonitoringTab) convApplyToViewport() {
	dimStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	outputStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)

	content := t.convBuf.String()

	if !t.inputActive && t.ticking {
		if t.stream != nil && t.stream.Len() > 0 {
			content += outputStyle.Render("◀ Agent:") + "\n"
			for _, l := range strings.Split(t.stream.String(), "\n") {
				content += "  " + l + "\n"
			}
		} else {
			content += dimStyle.Render("  "+t.spinner.View()+" Agent is thinking...") + "\n"
		}
	}

	t.vp.SetContent(content)
	if t.autoScroll {
		t.vp.GotoBottom()
	}
}

// renderStepBlock builds the chat-style view for a single step.
//
//nolint:cyclop,gocognit // Chat rendering traverses all step types and output variants; complexity is inherent.
func (t *MonitoringTab) renderStepBlock(step *workflow.Step, state *workflow.StepState, hasState bool) string {
	inputStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimaryLight)
	outputStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	errorLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	stderrLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorWarning)
	dimStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	var sb strings.Builder

	// Conversation-mode agent steps: render turns as a chat thread.
	if hasState && state.Conversation != nil && len(state.Conversation.Turns) > 0 {
		dividerLine := lipgloss.NewStyle().Foreground(ColorMuted).Render("────────────────────────────")
		needsDivider := false
		for _, turn := range state.Conversation.Turns {
			switch turn.Role {
			case workflow.TurnRoleSystem:
				continue
			case workflow.TurnRoleUser:
				if needsDivider {
					sb.WriteString(dividerLine + "\n\n")
				}
				sb.WriteString(inputStyle.Render("▶ User:") + "\n")
				for _, l := range strings.Split(turn.Content, "\n") {
					sb.WriteString("  " + l + "\n")
				}
			case workflow.TurnRoleAssistant:
				sb.WriteString(outputStyle.Render("◀ Agent:") + "\n")
				for _, l := range strings.Split(turn.Content, "\n") {
					sb.WriteString("  " + l + "\n")
				}
			}
			sb.WriteString("\n")
			needsDivider = true
		}
		if state.Status == workflow.StatusRunning {
			lastTurn := state.Conversation.Turns[len(state.Conversation.Turns)-1]
			if lastTurn.Role == workflow.TurnRoleUser {
				sb.WriteString(dimStyle.Render("  "+t.spinner.View()+" Agent is thinking...") + "\n")
			}
		}
		return sb.String()
	}

	// Non-conversation steps: show input then output.
	if step != nil {
		if inputText := stepInputText(step); inputText != "" {
			sb.WriteString(inputStyle.Render("▶ Input:") + "\n")
			for _, l := range strings.Split(inputText, "\n") {
				sb.WriteString("  " + l + "\n")
			}
			sb.WriteString("\n")
		}
	}

	if hasState {
		if state.Error != "" {
			sb.WriteString(errorLabel.Render("✗ Error:") + "\n")
			sb.WriteString("  " + state.Error + "\n\n")
		}
		if state.Stderr != "" {
			sb.WriteString(stderrLabel.Render("⚠ Stderr:") + "\n")
			for _, l := range renderTruncated(strings.Split(state.Stderr, "\n"), 20) {
				sb.WriteString("  " + l + "\n")
			}
			sb.WriteString("\n")
		}
		if state.Output != "" {
			sb.WriteString(outputStyle.Render("◀ Output:") + "\n")
			for _, l := range renderTruncated(strings.Split(state.Output, "\n"), 50) {
				sb.WriteString("  " + l + "\n")
			}
			sb.WriteString("\n")
		}
		if state.Output == "" && state.Error == "" && state.Stderr == "" {
			switch state.Status { //nolint:exhaustive // only running/pending need placeholder text
			case workflow.StatusRunning:
				sb.WriteString(dimStyle.Render("  ⟳ Running…") + "\n")
			case workflow.StatusPending:
				sb.WriteString(dimStyle.Render("  ⏳ Pending") + "\n")
			}
		}
	} else {
		sb.WriteString(dimStyle.Render("  ⏳ Waiting…") + "\n")
	}

	return sb.String()
}

// autoSelectRunning switches selectedIdx to the first running node.
func (t *MonitoringTab) autoSelectRunning() {
	for i, node := range t.flatNodes {
		if node.Status == workflow.StatusRunning {
			t.selectedIdx = i
			return
		}
	}
}

// autoSelectFailed switches selectedIdx to the first failed node, if any.
func (t *MonitoringTab) autoSelectFailed() {
	for i, node := range t.flatNodes {
		if node.Status == workflow.StatusFailed {
			t.selectedIdx = i
			t.autoScroll = false
			t.updateViewportContent()
			return
		}
	}
}

// selectedStepOutput returns the relevant output text for the currently selected node.
func (t MonitoringTab) selectedStepOutput() string { //nolint:gocritic // value receiver: read-only view
	if len(t.flatNodes) == 0 || t.selectedIdx < 0 || t.selectedIdx >= len(t.flatNodes) {
		return ""
	}
	name := t.flatNodes[t.selectedIdx].Name
	state, ok := t.states[name]
	if !ok {
		return StyleEmptyState.Render(fmt.Sprintf("Step %q has not started yet.", name))
	}

	errorLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	stderrLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorWarning)

	var sb strings.Builder
	if state.Error != "" {
		fmt.Fprintf(&sb, "%s %s\n\n", errorLabel.Render("Error:"), state.Error)
	}
	if state.Stderr != "" {
		fmt.Fprintf(&sb, "%s\n%s\n\n", stderrLabel.Render("Stderr:"), state.Stderr)
	}
	if state.Output != "" {
		fmt.Fprintf(&sb, "Output:\n%s", state.Output)
	}
	if sb.Len() == 0 {
		return fmt.Sprintf("Step %q: %s", name, StatusBadge(state.Status))
	}
	return sb.String()
}

// renderTreeWithSelection renders the execution tree with the selected line highlighted.
func (t MonitoringTab) renderTreeWithSelection() string { //nolint:gocritic // value receiver: read-only view
	if len(t.flatNodes) == 0 {
		if t.active != nil {
			return fmt.Sprintf("Active: %s (%s)\n", t.active.id, t.active.status)
		}
		return "Waiting for execution to start…"
	}

	lines := strings.Split(strings.TrimRight(RenderTree(treesFromFlat(t.flatNodes)), "\n"), "\n")
	var sb strings.Builder
	for i, line := range lines {
		if i < len(t.flatNodes) && i == t.selectedIdx {
			sb.WriteString(monitoringSelectedStyle.Render(line))
		} else {
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// resizeInputField sets the textinput width to fill the right panel.
func (t *MonitoringTab) resizeInputField() {
	_, rightWidth := t.panelWidths()
	// Border (2) + padding inside border (2) = 4 chars consumed by the container.
	w := rightWidth - 4
	if w < 1 {
		w = 1
	}
	t.inputField.SetWidth(w)
}

// resizeViewport updates the viewport dimensions based on the current terminal size.
func (t *MonitoringTab) resizeViewport() {
	_, rightWidth := t.panelWidths()
	panelH := t.panelHeight()
	if t.inputActive {
		panelH -= 3 // input field steals from right panel height
		if panelH < 3 {
			panelH = 3
		}
	}
	// Border (2) + panel title row (1) + horizontal padding (2 via Padding(0,1) on width only).
	vpWidth := rightWidth - 4
	vpHeight := panelH - 3
	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	t.vp.SetWidth(vpWidth)
	t.vp.SetHeight(vpHeight)
}

// panelWidths returns (leftWidth, rightWidth) based on 40/60 split.
func (t MonitoringTab) panelWidths() (int, int) { //nolint:gocritic // value receiver: read-only
	left := (t.width * 40) / 100
	right := t.width - left
	if left < 1 {
		left = 1
	}
	if right < 1 {
		right = 1
	}
	return left, right
}

// panelHeight returns the available height for the content panels.
func (t MonitoringTab) panelHeight() int { //nolint:gocritic // value receiver: read-only
	// Reserve 4 lines: tab bar (1) + separator (1) + help bar (1) + margins (1).
	h := t.height - 4
	if h < 1 {
		h = 1
	}
	return h
}

// flattenTree converts a tree of TreeNodes into a flat slice for keyboard navigation.
func flattenTree(nodes []*TreeNode) []*TreeNode {
	result := make([]*TreeNode, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
		result = append(result, flattenTree(node.Children)...)
	}
	return result
}

// treesFromFlat reconstructs a minimal top-level slice from a flat list for RenderTree.
// Since flatNodes already mirrors the tree structure, we extract root-level nodes only.
func treesFromFlat(flat []*TreeNode) []*TreeNode {
	var roots []*TreeNode
	for _, n := range flat {
		if n.Depth == 0 {
			roots = append(roots, n)
		}
	}
	return roots
}

// orderedSteps returns steps in execution order by walking the workflow graph
// from Initial, following OnSuccess / default transitions.
func orderedSteps(wf *workflow.Workflow) []workflow.Step {
	if wf == nil || len(wf.Steps) == 0 || wf.Initial == "" {
		return nil
	}

	visited := make(map[string]bool, len(wf.Steps))
	steps := make([]workflow.Step, 0, len(wf.Steps))
	current := wf.Initial

	for current != "" && !visited[current] {
		step, ok := wf.Steps[current]
		if !ok {
			break
		}
		visited[current] = true
		steps = append(steps, *step)

		if step.Type == workflow.StepTypeTerminal {
			break
		}
		current = nextStepName(step)
	}

	return steps
}

// nextStepName resolves the default next step for graph traversal.
// Checks Transitions for a default fallback first, then falls back to OnSuccess.
func nextStepName(step *workflow.Step) string {
	for _, tr := range step.Transitions {
		if tr.When == "" {
			return tr.Goto
		}
	}
	return step.OnSuccess
}
