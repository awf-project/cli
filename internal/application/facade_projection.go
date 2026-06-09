package application

import (
	"fmt"

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
	return out, nil
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

func validateContentBlocks(payload any) error {
	blocks, ok := payload.([]transcript.ContentBlock)
	if !ok {
		return nil
	}
	for i := range blocks {
		if !transcript.ValidBlockType(blocks[i].Type) {
			return domainerrors.NewSystemError(
				domainerrors.ErrorCodeSystemInternalUnmapped,
				fmt.Sprintf("unknown block type: %s", blocks[i].Type),
				map[string]any{"block_type": string(blocks[i].Type)},
				nil,
			)
		}
	}
	return nil
}
