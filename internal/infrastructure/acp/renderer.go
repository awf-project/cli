package acp

import (
	"context"
	"fmt"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/pkg/display"
)

// SecretMasker masks sensitive values in text output before emission.
// Consumer-defined interface satisfied by logger.SecretMasker.
type SecretMasker interface {
	MaskText(text string, env map[string]string) string
}

// ACPRenderer converts a DisplayEvent stream into ACP Message variants.
// It is instantiated per workflow step — the seenTools dedup index never leaks across steps.
type ACPRenderer struct {
	stepID    string
	sender    Sender
	masker    SecretMasker
	logger    ports.Logger
	env       map[string]string
	mu        sync.Mutex
	seq       uint64
	seenTools map[string]struct{}
}

// NewACPRenderer creates a renderer bound to a single workflow step.
func NewACPRenderer(stepID string, sender Sender, masker SecretMasker, logger ports.Logger, env map[string]string) *ACPRenderer {
	return &ACPRenderer{
		stepID:    stepID,
		sender:    sender,
		masker:    masker,
		logger:    logger,
		env:       env,
		seenTools: make(map[string]struct{}),
	}
}

// Render converts one DisplayEvent into a Message and forwards it via the Sender.
// ctx carries the workflow's cancellation signal and is propagated to Sender.Send
// so emission stops when the ACP peer disconnects. event is taken by pointer to avoid
// copying the ~112-byte struct on every event; Render does not retain it.
//
// Concurrency: the mutex is held only to allocate a monotonic seq number and consult
// seenTools. Sender.Send is called OUTSIDE the lock so a slow peer does not serialize
// all concurrent Render callers. Seq monotonicity is preserved (each goroutine gets a
// unique seq before releasing the lock); emission order is not guaranteed when multiple
// goroutines race — use a single-threaded caller when strict ordering is required.
func (r *ACPRenderer) Render(ctx context.Context, event *display.DisplayEvent) error {
	if event == nil {
		r.logger.Warn("acp renderer: nil event dropped", "step", r.stepID)
		return nil
	}

	// Build the message skeleton under the lock (seq allocation + seenTools update only).
	// MaskText is called OUTSIDE the lock: masker and env are immutable after construction
	// so there is no race on them, and moving the call out avoids holding the mutex during
	// a potentially non-trivial string scan.
	// Release the lock before calling MaskText and Sender.Send to avoid serializing slow I/O.
	type msgSkeleton struct {
		msgType  MessageType
		seq      uint64
		rawText  string // unmasked text to pass to MaskText after unlock
		toolID   string
		toolName string
	}

	var (
		sk    msgSkeleton
		valid bool
	)

	r.mu.Lock()
	r.seq++
	seq := r.seq

	// Switch on event.Kind (normalized discriminator) rather than event.Type (raw
	// provider string). Kind is set by every provider's parser and is the canonical
	// field for rendering decisions; Type is provider-specific and cannot be reliably
	// compared across providers (M-4 fix).
	switch event.Kind {
	case display.EventText:
		sk = msgSkeleton{msgType: MsgAgentMessageChunk, seq: seq, rawText: event.Text}
		valid = true

	case display.EventReasoning:
		sk = msgSkeleton{msgType: MsgAgentThoughtChunk, seq: seq, rawText: event.Text}
		valid = true

	case display.EventToolUse:
		toolID := event.ID
		if toolID == "" {
			// Synthesize a STABLE ID so that successive streaming chunks from the same
			// tool are correctly classified as MsgToolCallUpdate rather than MsgToolCall.
			// Using seq would produce a unique ID per event (every event looks like a
			// first sighting). Using the tool name makes the ID stable across all chunks
			// belonging to the same tool invocation within this step (issue #4 fix).
			// Fallback to seq only when the name is also absent — seq at least prevents
			// a panic and gives a unique string, though multi-chunk dedup won't work in
			// that degenerate case.
			if event.Name != "" {
				toolID = fmt.Sprintf("%s-tool-%s", r.stepID, event.Name)
			} else {
				toolID = fmt.Sprintf("%s-tool-%d", r.stepID, seq)
			}
		}

		msgType := MsgToolCall
		if _, seen := r.seenTools[toolID]; seen {
			msgType = MsgToolCallUpdate
		} else {
			r.seenTools[toolID] = struct{}{}
		}

		sk = msgSkeleton{msgType: msgType, seq: seq, rawText: event.Arg, toolID: toolID, toolName: event.Name}
		valid = true
	}
	r.mu.Unlock()

	if !valid {
		return nil
	}

	// MaskText and Sender.Send run outside the lock: env is read-only after construction.
	msg := Message{
		Type:    sk.msgType,
		StepID:  r.stepID,
		Seq:     sk.seq,
		Content: r.masker.MaskText(sk.rawText, r.env),
		ToolID:  sk.toolID,
		Tool:    sk.toolName,
	}
	return r.sender.Send(ctx, msg)
}

// RenderFunc returns a closure that satisfies agents.DisplayEventRenderer.
// Each event in the slice is rendered independently; a Send error is logged and the batch continues.
// If ctx is cancelled before an event is processed, the batch stops early.
func (r *ACPRenderer) RenderFunc(ctx context.Context) agents.DisplayEventRenderer {
	return func(events []agents.DisplayEvent) {
		for i := range events {
			if ctx.Err() != nil {
				return
			}
			if err := r.Render(ctx, &events[i]); err != nil {
				r.logger.Warn("acp render failed", "step", r.stepID, "err", err.Error())
			}
		}
	}
}
