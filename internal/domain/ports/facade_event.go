package ports

import "time"

// EventKind classifies a facade Event.
//
// The taxonomy covers three groups:
//
//  1. Run lifecycle (EventRunStarted, EventRunCompleted) — emitted once per
//     top-level run at start and end of the execution loop.
//
//  2. Step lifecycle (EventStepStarted, EventStepCompleted,
//     EventStepCallWorkflowStarted, EventStepCallWorkflowCompleted) — emitted
//     around each step, including nested sub-workflow dispatch.
//
//  3. Exchange content (EventMessageUser, EventMessageAssistant, EventToolCall,
//     EventToolResult) — emitted for each message or tool invocation produced by
//     an agent during a step's exchange.
//
// Two additional kinds are synthesized by the facade bridge rather than sourced
// from the underlying transcript:
//
//   - EventInputRequired — the workflow is paused awaiting user input via Respond.
//   - EventWorkflowCompleted / EventWorkflowFailed — terminal sentinels that signal
//     the end of the Events() stream; the channel is closed immediately after.
//
// EventKindUnknown is the zero value and the fail-closed fallback for any unrecognized
// or uninitialized event type (NFR-007).
type EventKind uint8

const (
	// EventKindUnknown is the zero value and the fail-closed fallback for any
	// unrecognized or uninitialized event type (NFR-007). Because it is iota (= 0),
	// an accidental zero-init of an Event struct will always produce this kind rather
	// than a valid observable event — callers must treat it as "unknown/drop" rather
	// than as a real event. Consumers that switch on EventKind should always include
	// a default case that handles EventKindUnknown.
	EventKindUnknown               EventKind = iota
	EventRunStarted                          // run lifecycle: execution loop started
	EventRunCompleted                        // run lifecycle: execution loop finished
	EventStepStarted                         // step lifecycle: step entered
	EventStepCompleted                       // step lifecycle: step exited
	EventStepCallWorkflowStarted             // step lifecycle: sub-workflow dispatch started
	EventStepCallWorkflowCompleted           // step lifecycle: sub-workflow dispatch finished
	EventMessageUser                         // exchange content: user message produced
	EventMessageAssistant                    // exchange content: assistant message produced
	EventToolCall                            // exchange content: tool invocation issued
	EventToolResult                          // exchange content: tool invocation result received
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

// Event is a projection wrapper emitted on RunSession.Events().
// Seq, RunID, ParentRunID are reused verbatim from the source event —
// no independent sequence numbering is introduced (A2, FR-006, D3).
//
// Payload type by Kind:
//
//	Kind                          Payload type           Notes
//	──────────────────────────── ────────────────────── ───────────────────────────────
//	EventStepStarted              *EnrichedStepPayload
//	EventStepCompleted            *EnrichedStepPayload   HadOutput/Output/Stderr valid
//	EventStepCallWorkflowStarted  *EnrichedStepPayload
//	EventStepCallWorkflowCompleted *EnrichedStepPayload
//	EventRunStarted               nil
//	EventRunCompleted             nil
//	EventMessageUser              *EnrichedMessagePayload
//	EventMessageAssistant         *EnrichedMessagePayload
//	EventToolCall                 nil  (raw transcript payload; may be non-nil in future)
//	EventToolResult               nil  (raw transcript payload; may be non-nil in future)
//	EventInputRequired            *EnrichedInputRequest
//	EventWorkflowCompleted        nil  (success: no error payload needed)
//	EventWorkflowFailed           *EnrichedTerminal      Error field carries the reason
//	EventKindUnknown              nil  (drop: unrecognized or zero-init)
//
// Zero-value safety: a zero-value Event has Kind == EventKindUnknown (iota = 0), which is the
// fail-closed sentinel. Accidental zero-init therefore produces an "unknown" event rather than
// a valid observable event. Consumers must always handle the EventKindUnknown case.
type Event struct {
	Seq         uint64
	Kind        EventKind
	RunID       string
	ParentRunID string
	Payload     any
	Timestamp   time.Time
}

// EnrichedStepPayload carries step metadata for EventStepStarted, EventStepCompleted,
// EventStepCallWorkflowStarted, and EventStepCallWorkflowCompleted.
type EnrichedStepPayload struct {
	StepName   string
	Error      string
	DurationMs int64
	// HadOutput reports whether the step produced any human-visible stdout/stderr.
	// It is meaningful only on EventStepCompleted (false on EventStepStarted). The CLI
	// renderer uses it to reproduce the legacy F037 success-feedback behavior: a step
	// that completed with no output gets an explicit "<step>: completed successfully"
	// line, whereas a step that already printed output does not (its output is the
	// feedback). It is derived from the step state at emit time, not from the event
	// stream, because shell-step stdout is streamed out-of-band via the output writers
	// and never appears as a facade event.
	HadOutput bool
	// Output and Stderr carry the step's captured stdout/stderr at completion time.
	// Like HadOutput they are derived from the step state at emit time (shell-step
	// stdout is streamed out-of-band via the output writers and is not otherwise
	// recoverable from the event stream). They are meaningful only on
	// EventStepCompleted and let event-only consumers (TUI monitoring, SSE) display
	// per-step output without polling an ExecutionContext. The CLI renderer ignores
	// them (it streams stdout directly through the output writers).
	Output string
	Stderr string
}

// EnrichedMessagePayload carries message content for both EventMessageUser and
// EventMessageAssistant. The Kind field on the enclosing Event distinguishes which
// participant produced the message.
type EnrichedMessagePayload struct {
	Content string
}

// EnrichedInputRequest carries the prompt for the bridge-synthesized EventInputRequired.
type EnrichedInputRequest struct {
	Prompt string
}

// EnrichedTerminal carries the terminal error for EventWorkflowFailed.
// It is nil/unused for EventWorkflowCompleted (success requires no error payload).
type EnrichedTerminal struct {
	Error string
}
