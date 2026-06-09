package ports

import "time"

// EventKind classifies a facade Event. It covers the 10 F106 transcript.EventType
// values, the bridge-synthesized EventInputRequired, and the two terminal kinds.
// EventKindUnknown is the fail-closed fallback for unrecognized event types.
type EventKind uint8

const (
	EventKindUnknown               EventKind = iota
	EventRunStarted                          // maps to transcript.EventTypeRunStarted
	EventRunCompleted                        // maps to transcript.EventTypeRunCompleted
	EventStepStarted                         // maps to transcript.EventTypeStepStarted
	EventStepCompleted                       // maps to transcript.EventTypeStepCompleted
	EventStepCallWorkflowStarted             // maps to transcript.EventTypeStepCallWorkflowStarted
	EventStepCallWorkflowCompleted           // maps to transcript.EventTypeStepCallWorkflowCompleted
	EventMessageUser                         // maps to transcript.EventTypeMessageUser
	EventMessageAssistant                    // maps to transcript.EventTypeMessageAssistant
	EventToolCall                            // maps to transcript.EventTypeToolCall
	EventToolResult                          // maps to transcript.EventTypeToolResult
	EventInputRequired                       // bridge-synthesized: workflow awaits user input
	EventWorkflowCompleted                   // terminal: workflow finished successfully
	EventWorkflowFailed                      // terminal: workflow finished with error
)

func (k EventKind) String() string {
	switch k {
	case EventRunStarted:
		return "run.started"
	case EventRunCompleted:
		return "run.completed"
	case EventStepStarted:
		return "step.started"
	case EventStepCompleted:
		return "step.completed"
	case EventStepCallWorkflowStarted:
		return "step.call_workflow.started"
	case EventStepCallWorkflowCompleted:
		return "step.call_workflow.completed"
	case EventMessageUser:
		return "message.user"
	case EventMessageAssistant:
		return "message.assistant"
	case EventToolCall:
		return "tool.call"
	case EventToolResult:
		return "tool.result"
	case EventInputRequired:
		return "input.required"
	case EventWorkflowCompleted:
		return "workflow.completed"
	case EventWorkflowFailed:
		return "workflow.failed"
	default:
		return "unknown"
	}
}

// Event is a projection wrapper over transcript.ExchangeEvent.
// Seq, RunID, ParentRunID, and Payload are reused verbatim from the source event —
// no independent sequence numbering is introduced (A2, FR-006, D3).
// Payload carries *transcript.MessagePayload, *transcript.ToolPayload,
// *transcript.StepPayload, or []transcript.ContentBlock depending on Kind.
type Event struct {
	Seq         uint64
	Kind        EventKind
	RunID       string
	ParentRunID string
	Payload     any
	Timestamp   time.Time
}
