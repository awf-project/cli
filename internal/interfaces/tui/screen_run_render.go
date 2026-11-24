package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// Line caps for completed-step output; live output is already bounded by maxLiveOutputBytes.
const (
	maxStderrLines = 20
	maxOutputLines = 50
	maxLiveLines   = 200
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
		if strings.HasPrefix(step.Operation, "notifier.") {
			return step.Operation
		}
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
		fmt.Sprintf(labelRunMoreLinesFmt, len(lines)-maxLines),
	)
	return result
}

// tailLines returns the last maxLines lines of text, prefixed by a muted marker when
// earlier lines were cut. Live output follows the tail, so the newest lines matter most.
func tailLines(text string, maxLines int) []string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= maxLines {
		return lines
	}
	marker := lipgloss.NewStyle().Foreground(ColorMuted).Render(
		fmt.Sprintf(labelRunEarlierLinesFmt, len(lines)-maxLines),
	)
	return append([]string{marker}, lines[len(lines)-maxLines:]...)
}

// writeIndented writes lines under the current block, indented by two spaces.
func writeIndented(sb *strings.Builder, lines []string) {
	for _, l := range lines {
		sb.WriteString("  ")
		sb.WriteString(l)
		sb.WriteString("\n")
	}
}

// rebuildTree reconstructs flatNodes from the current steps and states.
func (t *RunScreen) rebuildTree() {
	nodes := BuildTree(t.steps, t.states)
	t.flatNodes = flattenTree(nodes)

	// Infer terminal step status from the workflow outcome when not tracked in states.
	// Source it from finalStatus (set by applyFacadeEvent on the terminal event);
	// terminal steps never emit their own step events.
	if outcome := t.finalStatus; outcome != "" && !t.running {
		for _, node := range t.flatNodes {
			if _, hasState := t.states[node.Name]; hasState {
				continue
			}
			if step, ok := t.findStep(node.Name); ok && step.Type == workflow.StepTypeTerminal {
				node.Status = outcome
			}
		}
	}

	// Clamp selectedIdx within new bounds.
	switch {
	case len(t.flatNodes) == 0:
		t.selectedIdx = 0
	case t.selectedIdx >= len(t.flatNodes):
		t.selectedIdx = len(t.flatNodes) - 1
	}
}

// findStep returns the step definition by name.
func (t *RunScreen) findStep(name string) (*workflow.Step, bool) {
	for i := range t.steps {
		if t.steps[i].Name == name {
			return &t.steps[i], true
		}
	}
	return nil, false
}

// updateViewportContent refreshes the right-panel viewport with the selected step's view.
func (t *RunScreen) updateViewportContent() {
	name := t.selectedStepName()
	if name == "" {
		t.vp.SetContent("")
		return
	}
	t.vp.SetContent(t.renderSelectedStepChat(name))
	if t.autoScroll {
		t.vp.GotoBottom()
	}
}

// renderSelectedStepChat builds the chat view for a given step.
func (t *RunScreen) renderSelectedStepChat(name string) string {
	step, _ := t.findStep(name)
	state, hasState := t.states[name]

	if !hasState && step != nil && step.Type == workflow.StepTypeTerminal && t.finalStatus != "" && !t.running {
		state = workflow.StepState{Name: name, Status: t.finalStatus}
		hasState = true
	}
	return t.renderStepBlock(step, &state, hasState)
}

// renderStepBlock builds the chat-style view for a single step.
func (t *RunScreen) renderStepBlock(step *workflow.Step, state *workflow.StepState, hasState bool) string {
	var sb strings.Builder

	// Conversation-mode agent steps: render turns as a chat thread.
	if hasState && state.Conversation != nil && len(state.Conversation.Turns) > 0 {
		writeConversation(&sb, state, "  "+t.spinner.View()+" "+labelRunAgentThinking)
		return sb.String()
	}

	// Non-conversation steps: show input then output.
	if step != nil {
		if inputText := stepInputText(step); inputText != "" {
			sb.WriteString(StyleBlockInput.Render(labelRunInput))
			sb.WriteString("\n")
			writeIndented(&sb, strings.Split(inputText, "\n"))
			sb.WriteString("\n")
		}
	}

	if !hasState {
		sb.WriteString(StyleBlockDim.Render("  " + labelRunWaiting))
		sb.WriteString("\n")
		return sb.String()
	}
	writeStepResult(&sb, state, t.live[state.Name])
	return sb.String()
}

// writeConversation renders the conversation turns of an agent step. thinking is the
// line shown while the agent is answering (empty for a run that already ended).
func writeConversation(sb *strings.Builder, state *workflow.StepState, thinking string) {
	divider := StyleBlockDim.Render("────────────────────────────")
	needsDivider := false
	for _, turn := range state.Conversation.Turns {
		switch turn.Role {
		case workflow.TurnRoleSystem:
			continue
		case workflow.TurnRoleUser:
			if needsDivider {
				sb.WriteString(divider)
				sb.WriteString("\n\n")
			}
			sb.WriteString(StyleBlockInput.Render(labelRunUser))
			sb.WriteString("\n")
			writeIndented(sb, strings.Split(turn.Content, "\n"))
		case workflow.TurnRoleAssistant:
			sb.WriteString(StyleBlockOutput.Render(labelRunAgent))
			sb.WriteString("\n")
			writeIndented(sb, strings.Split(turn.Content, "\n"))
		}
		sb.WriteString("\n")
		needsDivider = true
	}
	if state.Status == workflow.StatusRunning && thinking != "" {
		last := state.Conversation.Turns[len(state.Conversation.Turns)-1]
		if last.Role == workflow.TurnRoleUser {
			sb.WriteString(StyleBlockDim.Render(thinking))
			sb.WriteString("\n")
		}
	}
}

// writeStepResult renders a step's error, stderr and output. While the step is still
// running, whatever it streamed so far (live, may be nil) is shown in place of the
// not yet captured output.
func writeStepResult(sb *strings.Builder, state *workflow.StepState, live *liveOutput) {
	if state.Error != "" {
		sb.WriteString(StyleBlockError.Render(labelRunError))
		sb.WriteString("\n  ")
		sb.WriteString(state.Error)
		sb.WriteString("\n\n")
	}

	stdout, stderr := state.Output, state.Stderr
	streaming := state.Status == workflow.StatusRunning && live != nil
	if streaming {
		stdout, stderr = live.stdout, live.stderr
	}

	if stderr != "" {
		label := labelRunStderr
		lines := renderTruncated(strings.Split(stderr, "\n"), maxStderrLines)
		if streaming {
			label, lines = labelRunStderrLive, tailLines(stderr, maxLiveLines)
		}
		sb.WriteString(StyleBlockWarn.Render(label))
		sb.WriteString("\n")
		writeIndented(sb, lines)
		sb.WriteString("\n")
	}
	if stdout != "" {
		label := labelRunOutput
		lines := renderTruncated(strings.Split(stdout, "\n"), maxOutputLines)
		if streaming {
			label, lines = labelRunOutputLive, tailLines(stdout, maxLiveLines)
		}
		sb.WriteString(StyleBlockOutput.Render(label))
		sb.WriteString("\n")
		writeIndented(sb, lines)
		sb.WriteString("\n")
	}
	if streaming && live.truncated {
		sb.WriteString(StyleBlockDim.Render("  " + labelRunLiveTruncated))
		sb.WriteString("\n")
	}

	if stdout == "" && stderr == "" && state.Error == "" {
		switch state.Status { //nolint:exhaustive // only running/pending need placeholder text
		case workflow.StatusRunning:
			sb.WriteString(StyleBlockDim.Render("  " + labelRunRunning))
			sb.WriteString("\n")
		case workflow.StatusPending:
			sb.WriteString(StyleBlockDim.Render("  " + labelRunPending))
			sb.WriteString("\n")
		}
	}
}

// copyText returns the plain-text result of the selected step for the clipboard.
func (t *RunScreen) copyText() string {
	name := t.selectedStepName()
	state, ok := t.states[name]
	if !ok {
		return ""
	}
	return stepCopyText(&state, t.live[name])
}

// stepCopyText returns a step's error, stderr and stdout as plain text, using live output
// while the step is still running.
func stepCopyText(state *workflow.StepState, live *liveOutput) string {
	stdout, stderr := state.Output, state.Stderr
	if live != nil && state.Status == workflow.StatusRunning {
		stdout, stderr = live.stdout, live.stderr
	}

	var parts []string
	if state.Error != "" {
		parts = append(parts, state.Error)
	}
	if stderr != "" {
		parts = append(parts, stderr)
	}
	if stdout != "" {
		parts = append(parts, stdout)
	}
	return strings.TrimRight(strings.Join(parts, "\n"), "\n")
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
