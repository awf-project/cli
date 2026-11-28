package tui

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// maxLiveOutputBytes caps the live output kept per step and stream. Beyond it the
// oldest bytes are dropped: the user is following the tail, and the complete output
// arrives with the step's completion event anyway.
const maxLiveOutputBytes = 256 << 10

// liveOutput accumulates the stdout/stderr chunks streamed by ports.EventStepOutput for
// one step. Chunks are raw (they may end mid-line), so they are concatenated verbatim.
type liveOutput struct {
	stdout    string
	stderr    string
	truncated bool
}

// add appends text to the named stream, dropping the oldest bytes past the cap. Unknown
// stream names are treated as stdout so no output is ever silently lost.
func (l *liveOutput) add(stream, text string) {
	if stream == ports.StepOutputStderr {
		l.stderr = l.appendCapped(l.stderr, text)
		return
	}
	l.stdout = l.appendCapped(l.stdout, text)
}

func (l *liveOutput) appendCapped(current, text string) string {
	current += text
	if len(current) <= maxLiveOutputBytes {
		return current
	}
	l.truncated = true
	cut := len(current) - maxLiveOutputBytes
	// Do not start in the middle of a multi-byte rune.
	for cut < len(current) && !utf8.RuneStart(current[cut]) {
		cut++
	}
	return current[cut:]
}

// StartEventLoop spawns a goroutine that ranges over sess.Events() and forwards each
// event to the Bubble Tea program as a facadeEventMsg.
//
// Terminal events (EventWorkflowCompleted, EventWorkflowFailed) are forwarded as
// facadeEventMsg first, then an ExecutionFinishedMsg is sent to finalize the screen
// (clears the in-flight indicators and renders the final state).
//
// ctx must be derived from the Bubble Tea program's lifetime (or a parent cancelled when
// the TUI stops). When ctx is done the goroutine exits without calling the sender,
// preventing dropped messages after the program has halted.
func (t *RunScreen) StartEventLoop(ctx context.Context, sess ports.RunSession) {
	ref := t.senderRef
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-sess.Events():
				if !ok {
					return
				}
				if ref == nil || ref.fn == nil {
					continue
				}
				ref.fn(facadeEventMsg{Event: e})
				switch e.Kind { //nolint:exhaustive // only terminal events require a follow-up message
				case ports.EventWorkflowCompleted:
					ref.fn(ExecutionFinishedMsg{Err: nil})
				case ports.EventWorkflowFailed:
					ref.fn(ExecutionFinishedMsg{Err: sess.Err()})
				}
			}
		}
	}()
}

// applyFacadeEvent translates a facade ports.Event into a state update and refreshes the
// tree and output panel. Events that do not affect the screen are ignored.
//
//nolint:gocritic // hugeParam: Event is part of the ports contract; pointer indirection would couple the TUI to *ports.Event
func (t *RunScreen) applyFacadeEvent(ev ports.Event) {
	switch ev.Kind { //nolint:exhaustive // only the kinds that drive screen state are handled; the rest are intentionally ignored
	case ports.EventStepStarted:
		t.applyStepStarted(ev)

	case ports.EventStepOutput:
		t.applyStepOutput(ev)

	case ports.EventMessageUser:
		if p, ok := ev.Payload.(*ports.EnrichedMessagePayload); ok && p != nil {
			t.appendConversationTurn(workflow.TurnRoleUser, p.Content)
		}

	case ports.EventMessageAssistant:
		if p, ok := ev.Payload.(*ports.EnrichedMessagePayload); ok && p != nil {
			t.appendConversationTurn(workflow.TurnRoleAssistant, p.Content)
		}

	case ports.EventStepCompleted:
		t.applyStepCompleted(ev)

	case ports.EventWorkflowCompleted:
		// Terminal steps never emit step events, so the reached terminal node stays
		// pending until the workflow ends. Record the outcome so rebuildTree marks it.
		t.finalStatus = workflow.StatusCompleted
		t.rebuildTree()
		t.updateViewportContent()

	case ports.EventWorkflowFailed:
		t.finalStatus = workflow.StatusFailed
		if term, ok := ev.Payload.(*ports.EnrichedTerminal); ok && term != nil && term.Error != "" {
			t.runErr = term.Error
		}
		t.rebuildTree()
		t.updateViewportContent()
	}
}

//nolint:gocritic // hugeParam: see applyFacadeEvent
func (t *RunScreen) applyStepStarted(ev ports.Event) {
	payload, ok := ev.Payload.(*ports.EnrichedStepPayload)
	if !ok || payload == nil || payload.StepName == "" {
		return
	}
	existing := t.states[payload.StepName]
	existing.Name = payload.StepName
	existing.Status = workflow.StatusRunning
	t.states[payload.StepName] = existing
	t.begin[payload.StepName] = eventTime(ev)
	// Track the running agent/conversation step so subsequent message events
	// (which carry no step name) attach to the right step's conversation view.
	if step, found := t.findStep(payload.StepName); found && step.Agent != nil {
		t.agentStep = payload.StepName
	}
	t.rebuildTree()
	if t.autoScroll {
		t.followStep(payload.StepName)
	}
	t.updateViewportContent()
}

// followStep moves the tree cursor onto the named step, if it is in the tree.
func (t *RunScreen) followStep(name string) {
	for i, n := range t.flatNodes {
		if n.Name == name {
			t.selectedIdx = i
			return
		}
	}
}

// applyStepOutput appends a live output chunk. The panel is only re-rendered when the
// chunk belongs to the step on screen, which keeps chatty background steps cheap.
//
//nolint:gocritic // hugeParam: see applyFacadeEvent
func (t *RunScreen) applyStepOutput(ev ports.Event) {
	payload, ok := ev.Payload.(*ports.EnrichedStepOutput)
	if !ok || payload == nil || payload.StepName == "" || payload.Text == "" {
		return
	}
	out := t.live[payload.StepName]
	if out == nil {
		out = &liveOutput{}
		t.live[payload.StepName] = out
	}
	out.add(payload.Stream, payload.Text)

	if t.selectedStepName() == payload.StepName {
		t.updateViewportContent()
	}
}

//nolint:gocritic // hugeParam: see applyFacadeEvent
func (t *RunScreen) applyStepCompleted(ev ports.Event) {
	payload, ok := ev.Payload.(*ports.EnrichedStepPayload)
	if !ok || payload == nil || payload.StepName == "" {
		return
	}
	existing := t.states[payload.StepName]
	existing.Name = payload.StepName
	// The completed event carries the captured output/stderr (derived from step state at
	// emit time). It is authoritative, so it replaces whatever streamed live.
	existing.Output = payload.Output
	existing.Stderr = payload.Stderr
	if payload.Error != "" {
		existing.Status = workflow.StatusFailed
		existing.Error = payload.Error
	} else {
		existing.Status = workflow.StatusCompleted
	}
	t.states[payload.StepName] = existing
	t.end[payload.StepName] = eventTime(ev)
	delete(t.live, payload.StepName)
	t.rebuildTree()
	t.updateViewportContent()
	t.autoSelectFailed()
}

// appendConversationTurn attaches a conversation turn (the agent's question or the user's
// answer) to the running agent step's state, building the ConversationState the output
// panel renders. Facade message events carry no step name, so turns are attributed to
// agentStep (the last agent/conversation step that started). The running step is
// auto-selected so the live conversation is what the user sees while the workflow is
// parked for input.
func (t *RunScreen) appendConversationTurn(role workflow.TurnRole, content string) {
	name := t.agentStep
	if name == "" {
		return
	}
	st := t.states[name]
	st.Name = name
	if st.Conversation == nil {
		st.Conversation = &workflow.ConversationState{}
	}
	st.Conversation.Turns = append(st.Conversation.Turns, workflow.Turn{Role: role, Content: content})
	st.Conversation.TotalTurns = len(st.Conversation.Turns)
	t.states[name] = st
	t.autoSelectRunning()
	t.rebuildTree()
	t.updateViewportContent()
}

// autoSelectRunning switches selectedIdx to the first running node.
func (t *RunScreen) autoSelectRunning() {
	for i, node := range t.flatNodes {
		if node.Status == workflow.StatusRunning {
			t.selectedIdx = i
			return
		}
	}
}

// autoSelectFailed switches selectedIdx to the first failed node, if any.
func (t *RunScreen) autoSelectFailed() {
	for i, node := range t.flatNodes {
		if node.Status == workflow.StatusFailed {
			t.selectedIdx = i
			t.autoScroll = false
			t.updateViewportContent()
			return
		}
	}
}

// eventTime returns the event's timestamp, or now when the producer did not set one.
//
//nolint:gocritic // hugeParam: see applyFacadeEvent
func eventTime(ev ports.Event) time.Time {
	if ev.Timestamp.IsZero() {
		return time.Now()
	}
	return ev.Timestamp
}

// selectedStepName returns the name of the step under the tree cursor, or "".
func (t *RunScreen) selectedStepName() string {
	if t.selectedIdx < 0 || t.selectedIdx >= len(t.flatNodes) {
		return ""
	}
	return t.flatNodes[t.selectedIdx].Name
}
