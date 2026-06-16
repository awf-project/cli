package application

import (
	"fmt"
	"strings"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

// ProjectEvent maps a transcript.ExchangeEvent to a ports.Event.
// Unknown event types fail closed to EventKindUnknown + non-fatal error (NFR-007).
// Pure function — no I/O, no state, no provider branching (D10).
func ProjectEvent(ev transcript.ExchangeEvent) (ports.Event, error) { //nolint:gocritic // hugeParam: signature is fixed by the ports contract; callers pass by value
	out := ports.Event{
		Seq:         ev.Seq,
		Kind:        ports.EventKindUnknown,
		RunID:       ev.RunID,
		ParentRunID: ev.ParentRunID,
		Payload:     ev.Payload,
		Timestamp:   ev.Timestamp,
	}
	kind, err := mapEventKind(ev.Type)
	if err != nil {
		return out, err
	}
	if err := validateContentBlocks(ev.Payload); err != nil {
		return out, err
	}
	out.Kind = kind
	out.Payload = projectPayload(ev.Payload)
	return out, nil
}

// projectPayload maps a transcript payload to the corresponding Enriched* struct.
//
// Step payloads carry the step name and error verbatim. DurationMs is NOT derivable
// from a single StepPayload (the typed payload has no timestamp field; A1) so it is
// left zero here and computed by the caller that owns the started→completed timestamp
// deltas; a zero value is an explicit "not yet computed" signal, never silently hidden.
//
// Message payloads concatenate their text ContentBlocks into a single Content string;
// non-text blocks (thinking, tool_use, …) contribute no text and are skipped.
//
// Unknown payload types: the caller (ProjectEvent) already guards against unknown
// EventTypes via mapEventKind; by the time projectPayload is reached the type is
// known and the payload is well-typed. An unexpected dynamic type therefore indicates
// a programming error (a new transcript payload type added without updating this
// switch). Rather than silently passing the raw value through — which would expose
// internal transcript types to consumers that expect only ports.Enriched* types —
// we return nil. The nil payload with a valid Kind is interpretable (callers must
// handle nil payloads per the Event table in facade_event.go) and surfaces the
// mapping gap at the consumer rather than hiding it.
func projectPayload(payload any) any {
	switch p := payload.(type) {
	case *transcript.StepPayload:
		hadOutput, ok := p.Result.(bool)
		return &ports.EnrichedStepPayload{
			StepName:  p.Name,
			Error:     p.Error,
			HadOutput: ok && hadOutput,
			Output:    p.Output,
			Stderr:    p.Stderr,
		}
	case *transcript.MessagePayload:
		return &ports.EnrichedMessagePayload{Content: concatTextBlocks(p.Blocks)}
	default:
		// Unrecognized payload type: return nil rather than leaking an internal
		// transcript type to facade consumers. This is a fail-closed policy consistent
		// with mapEventKind returning EventKindUnknown for unrecognized event types.
		// If a new transcript payload type is added, add a matching case here.
		return nil
	}
}

// concatTextBlocks concatenates the Text of every BlockTypeText block in order. Non-text
// blocks contribute nothing. Concatenation is verbatim (no separator) so a multi-block
// assistant message reads as one continuous string.
func concatTextBlocks(blocks []transcript.ContentBlock) string {
	var b strings.Builder
	for i := range blocks {
		if blocks[i].Type == transcript.BlockTypeText {
			b.WriteString(blocks[i].Text)
		}
	}
	return b.String()
}

func mapEventKind(et transcript.EventType) (ports.EventKind, error) {
	switch et {
	case transcript.EventTypeRunStarted:
		return ports.EventRunStarted, nil
	case transcript.EventTypeRunCompleted:
		return ports.EventRunCompleted, nil
	case transcript.EventTypeStepStarted:
		return ports.EventStepStarted, nil
	case transcript.EventTypeStepCompleted:
		return ports.EventStepCompleted, nil
	case transcript.EventTypeStepCallWorkflowStarted:
		return ports.EventStepCallWorkflowStarted, nil
	case transcript.EventTypeStepCallWorkflowCompleted:
		return ports.EventStepCallWorkflowCompleted, nil
	case transcript.EventTypeMessageUser:
		return ports.EventMessageUser, nil
	case transcript.EventTypeMessageAssistant:
		return ports.EventMessageAssistant, nil
	case transcript.EventTypeToolCall:
		return ports.EventToolCall, nil
	case transcript.EventTypeToolResult:
		return ports.EventToolResult, nil
	default:
		return ports.EventKindUnknown, domainerrors.NewSystemError(
			domainerrors.ErrorCodeSystemInternalUnmapped,
			fmt.Sprintf("unknown event type: %s", et),
			map[string]any{"event_type": string(et)},
			nil,
		)
	}
}

// validateContentBlocks validates the ContentBlocks inside a *transcript.MessagePayload.
// The previous implementation asserted payload.([]transcript.ContentBlock), which is always
// false — payloads are *transcript.MessagePayload, not raw block slices (M3). This version
// correctly unwraps the message and validates each block's type.
func validateContentBlocks(payload any) error {
	msg, ok := payload.(*transcript.MessagePayload)
	if !ok {
		return nil
	}
	for i := range msg.Blocks {
		if !transcript.ValidBlockType(msg.Blocks[i].Type) {
			return domainerrors.NewSystemError(
				domainerrors.ErrorCodeSystemInternalUnmapped,
				fmt.Sprintf("unknown block type: %s", msg.Blocks[i].Type),
				map[string]any{"block_type": string(msg.Blocks[i].Type)},
				nil,
			)
		}
	}
	return nil
}
