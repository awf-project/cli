package acp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/pkg/display"
)

// SecretMasker masks sensitive values in text output before emission.
// Consumer-defined interface satisfied by logger.SecretMasker.
type SecretMasker interface {
	MaskText(text string, env map[string]string) string
}

// synthesizeToolID returns a stable per-step tool identifier for seenTools dedup.
// If the event carries its own ID, that is used verbatim. Otherwise the tool name
// is combined with stepID to produce a stable ID across streaming chunks of the same
// tool call (issue #4 fix). As a last resort, seq gives a unique but non-stable ID
// so multi-chunk dedup will not work, but nothing panics.
func synthesizeToolID(stepID, eventID, eventName string, seq uint64) string {
	if eventID != "" {
		return eventID
	}
	if eventName != "" {
		return fmt.Sprintf("%s-tool-%s", stepID, eventName)
	}
	return fmt.Sprintf("%s-tool-%d", stepID, seq)
}

// Renderer converts a DisplayEvent stream into ACP SessionUpdate emissions via
// application.SessionUpdateEmitter. It is instantiated per workflow step — the
// seenTools dedup index never leaks across steps (per-step isolation invariant,
// D-row from plan; sharing would misclassify first-sighting tool_call vs subsequent
// tool_call_update variants).
//
// The renderer is bound to a single ACP session: sessionID routes every emitted
// update to the correct session, while stepID scopes tool-ID synthesis and dedup.
type Renderer struct {
	// sessionID and stepID are immutable after NewRenderer returns; they are read
	// outside mu (e.g. in synthesizeToolID and when building fields) and must stay
	// immutable for that to be data-race-free. Do not reuse a Renderer across steps.
	sessionID string
	stepID    string
	emitter   application.SessionUpdateEmitter
	masker    SecretMasker
	logger    *slog.Logger
	env       map[string]string
	mu        sync.Mutex
	seq       uint64
	seenTools map[string]struct{}
}

// NewRenderer creates a Renderer bound to one ACP session and one workflow step.
// masker may be nil (no redaction); env is the source of secret values to mask.
func NewRenderer(sessionID, stepID string, emitter application.SessionUpdateEmitter, masker SecretMasker, logger *slog.Logger, env map[string]string) *Renderer {
	return &Renderer{
		sessionID: sessionID,
		stepID:    stepID,
		emitter:   emitter,
		masker:    masker,
		logger:    logger,
		env:       env,
		seenTools: make(map[string]struct{}),
	}
}

// Render converts one DisplayEvent into an ACP SessionUpdate and emits it via the
// emitter. The discriminator (kind) and field shapes match the ACP wire protocol:
//   - text  → "agent_message_chunk" with content {type:text, text}
//   - reasoning → "agent_thought_chunk" with content {type:text, text}
//   - tool use → "tool_call" (first sighting) / "tool_call_update" (subsequent)
//     with {toolCallId, title, rawInput:{text}}
//
// Concurrency: the mutex guards only seq allocation and seenTools. MaskText and
// EmitSessionUpdate run OUTSIDE the lock so a slow peer does not serialize concurrent
// callers (invariant verified by TestRenderer_SlowEmitterDoesNotSerializeCallers).
func (r *Renderer) Render(ctx context.Context, event *display.DisplayEvent) error {
	if event == nil {
		if r.logger != nil {
			r.logger.Warn("acp renderer: nil event dropped", "step", r.stepID)
		}
		return nil
	}

	var (
		kind     string
		toolID   string
		rawText  string
		toolName string
		isTool   bool
		valid    bool
	)

	r.mu.Lock()
	r.seq++
	seq := r.seq

	// Switch on event.Kind (normalized discriminator) rather than event.Type (raw
	// provider string). Kind is set by every provider's parser and is the canonical
	// field for rendering decisions.
	switch event.Kind {
	case display.EventText:
		kind, rawText, valid = "agent_message_chunk", event.Text, true

	case display.EventReasoning:
		kind, rawText, valid = "agent_thought_chunk", event.Text, true

	case display.EventToolUse:
		toolID = synthesizeToolID(r.stepID, event.ID, event.Name, seq)
		kind = "tool_call"
		if _, seen := r.seenTools[toolID]; seen {
			kind = "tool_call_update"
		} else {
			r.seenTools[toolID] = struct{}{}
		}
		rawText, toolName, isTool, valid = event.Arg, event.Name, true, true
	}
	r.mu.Unlock()

	if !valid {
		return nil
	}

	// MaskText is applied outside the mutex: env is read-only after construction and
	// keeping slow string work off the lock preserves the no-serialization invariant.
	content := r.mask(rawText)

	var fields map[string]any
	if isTool {
		fields = map[string]any{
			"seq":        seq,
			"toolCallId": toolID,
			"title":      toolName,
			"rawInput":   map[string]any{"text": content},
		}
	} else {
		fields = map[string]any{
			"seq":     seq,
			"content": map[string]any{"type": "text", "text": content},
		}
	}

	return r.emitter.EmitSessionUpdate(ctx, r.sessionID, kind, fields)
}

// mask redacts secrets in text using the configured masker. A nil masker is a no-op.
func (r *Renderer) mask(text string) string {
	if r.masker == nil {
		return text
	}
	return r.masker.MaskText(text, r.env)
}

// RenderFunc returns a closure that satisfies agents.DisplayEventRenderer.
// Each event in the slice is rendered independently; an emit error is logged and the
// batch continues. If ctx is cancelled before an event is processed, the batch stops.
func (r *Renderer) RenderFunc(ctx context.Context) agents.DisplayEventRenderer {
	return func(events []agents.DisplayEvent) {
		for i := range events {
			if ctx.Err() != nil {
				return
			}
			if err := r.Render(ctx, &events[i]); err != nil && r.logger != nil {
				r.logger.Warn("acp render failed", "step", r.stepID, "err", err.Error())
			}
		}
	}
}
