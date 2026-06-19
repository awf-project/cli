package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/awf-project/cli/internal/domain/ports"
)

// sseEventData is the JSON payload for each SSE frame.
// The Kind field uses the string representation so the wire format is human-readable
// rather than emitting the raw uint8 constant value.
type sseEventData struct {
	Kind   string `json:"kind"`
	RunID  string `json:"run_id"`
	Seq    uint64 `json:"seq,omitempty"`
	Status string `json:"status,omitempty"`
	Step   string `json:"step,omitempty"`
	Error  string `json:"error,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

func eventDataFromEvent(ev ports.Event) sseEventData { //nolint:gocritic // Event is the public facade value type
	data := sseEventData{Kind: ev.Kind.String(), RunID: ev.RunID, Seq: ev.Seq}

	switch ev.Kind {
	case ports.EventStepStarted, ports.EventStepCompleted, ports.EventStepCallWorkflowStarted, ports.EventStepCallWorkflowCompleted:
		if payload, ok := ev.Payload.(*ports.EnrichedStepPayload); ok {
			data.Step = payload.StepName
			switch {
			case payload.Error != "":
				data.Error = payload.Error
				data.Status = "failed"
				if ev.Kind == ports.EventStepCompleted {
					data.Kind = "step.failed"
				}
			case ev.Kind == ports.EventStepCompleted || ev.Kind == ports.EventStepCallWorkflowCompleted:
				data.Status = "completed"
			default:
				data.Status = "running"
			}
		}
	case ports.EventWorkflowCompleted:
		data.Status = "completed"
	case ports.EventWorkflowFailed:
		data.Status = "failed"
		if payload, ok := ev.Payload.(*ports.EnrichedTerminal); ok {
			data.Error = payload.Error
			if payload.Error == "cancelled" {
				data.Status = "cancelled"
			}
		}
	case ports.EventInputRequired:
		if payload, ok := ev.Payload.(*ports.EnrichedInputRequest); ok && payload != nil {
			data.Prompt = payload.Prompt
		}
	case ports.EventKindUnknown,
		ports.EventRunStarted,
		ports.EventRunCompleted,
		ports.EventMessageUser,
		ports.EventMessageAssistant,
		ports.EventToolCall,
		ports.EventToolResult:
	}

	return data
}

// SessionLookup is the driven port for resolving a live RunSession by execution ID.
// It is satisfied by *application.SessionRegistry (wrapped via SessionRegistryLookup).
// Defined here so the api package has no import dependency on the application package.
type SessionLookup interface {
	GetSession(id string) (ports.RunSession, bool)
}

// StreamInput holds the path parameter and optional reconnection header for the SSE
// event stream endpoint. LastEventID is parsed from the standard "Last-Event-ID" request
// header (RFC 8895 §9.1) and used to replay buffered events on reconnect.
type StreamInput struct {
	ID          string `path:"id" doc:"Execution ID." example:"550e8400-e29b-41d4-a716-446655440000" required:"true"`
	LastEventID string `header:"Last-Event-ID" doc:"Resume from this event sequence; replay buffered events from this seq onward."`
}

// SSEHandler streams workflow execution events over Server-Sent Events.
type SSEHandler struct {
	b        *Bridge
	wg       *sync.WaitGroup
	sessions SessionLookup
}

// NewSSEHandler creates an SSEHandler bound to the given Bridge and WaitGroup.
func NewSSEHandler(b *Bridge, wg *sync.WaitGroup) *SSEHandler {
	return &SSEHandler{b: b, wg: wg}
}

// SetSessionLookup wires the session registry into the handler so getSession can
// resolve live RunSessions by ID. Must be called before the first request is served.
func (h *SSEHandler) SetSessionLookup(sl SessionLookup) {
	h.sessions = sl
}

// Stream consumes RunSession.Events() and emits typed SSE events.
// Supports Last-Event-ID header for reconnection with replay from buffered events.
// Returns huma.Error404NotFound when the execution ID is unknown.
// Exits cleanly on terminal state or ctx.Done().
func (h *SSEHandler) Stream(ctx context.Context, in *StreamInput, send sse.Sender) error {
	h.wg.Add(1)
	defer h.wg.Done()

	session, err := h.getSession(in.ID)
	if err != nil {
		return huma.Error404NotFound(fmt.Sprintf("execution not found: %s", in.ID))
	}

	lastEventID := h.getLastEventID(in)
	if err := h.replayBuffered(send, session, lastEventID); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-session.Events():
			if !ok {
				return nil
			}
			d := eventDataFromEvent(event)
			if err := send(sse.Message{Data: d}); err != nil {
				return nil //nolint:nilerr // client disconnected; treat as clean exit
			}
		}
	}
}

// getSession resolves a live RunSession by ID. Returns a descriptive error when
// the session registry is not configured or the ID is unknown — never (nil, nil).
func (h *SSEHandler) getSession(id string) (ports.RunSession, error) {
	if h.sessions == nil {
		return nil, fmt.Errorf("session registry not configured")
	}
	session, ok := h.sessions.GetSession(id)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return session, nil
}

// getLastEventID parses the "Last-Event-ID" header from the stream input and returns the
// uint64 sequence number. Returns 0 when the header is absent or contains a non-numeric
// value (treated as "no prior position" — full stream from the beginning, no replay).
// The seq numbers emitted in sseEventData.Seq are the values to pass back here on reconnect.
func (h *SSEHandler) getLastEventID(in *StreamInput) uint64 {
	if in == nil || in.LastEventID == "" {
		return 0
	}
	seq, err := strconv.ParseUint(in.LastEventID, 10, 64)
	if err != nil {
		// Non-numeric Last-Event-ID (e.g. a legacy string ID from another system):
		// treat as absent so we do not replay from a garbage position.
		return 0
	}
	return seq
}

// replayBuffered sends buffered events with Seq >= fromSeq to the SSE sender.
// When fromSeq == 0 (no Last-Event-ID header), no replay is performed.
// Overflow (requested seq evicted from bounded buffer) is silently skipped per
// spec edge case — bounded replay buffer; oldest events are dropped on overflow.
func (h *SSEHandler) replayBuffered(send sse.Sender, session ports.RunSession, fromSeq uint64) error {
	// Replay is only meaningful when a Last-Event-ID was provided.
	// The replayFromSeq method lives on *application.RunSession; at this interface
	// boundary we only have ports.RunSession. Type-assert optionally so the handler
	// works with any RunSession implementation (including mocks in tests).
	if fromSeq == 0 {
		return nil
	}
	type replayProvider interface {
		ReplayFromSeq(seq uint64) []ports.Event
	}
	if rp, ok := session.(replayProvider); ok {
		for _, ev := range rp.ReplayFromSeq(fromSeq) {
			// Send the same sseEventData shape as the live path (line ~125). Sending the
			// raw ports.Event here used an unregistered Go type, so huma logged
			// "unknown event type" and dumped a stack trace for every replayed frame.
			d := eventDataFromEvent(ev)
			if err := send(sse.Message{Data: d}); err != nil {
				return nil //nolint:nilerr // client disconnected; treat as clean exit
			}
		}
	}
	return nil
}

// sseMessageTypes maps SSE event names to a sample value of the Go type sent for that
// event. huma resolves a frame's event name by looking up the Go type of msg.Data in
// this registry; an empty registry makes huma log "unknown event type" and dump a stack
// trace on EVERY frame (see huma sse.go send()). All live and replayed frames share
// sseEventData — the event kind travels in its JSON `kind` field — so the type is
// registered under the default "message" event name (huma omits the redundant
// "event: message" line, keeping the wire format `data: {...}`).
func sseMessageTypes() map[string]any {
	return map[string]any{"message": sseEventData{}}
}

// RegisterSSERoutes registers GET /api/executions/{id}/events on the given Huma API.
func RegisterSSERoutes(api huma.API, h *SSEHandler) {
	sse.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/executions/{id}/events",
		OperationID: "stream-execution-events",
		Tags:        []string{"Executions"},
	}, sseMessageTypes(), func(ctx context.Context, in *StreamInput, send sse.Sender) {
		_ = h.Stream(ctx, in, send) //nolint:errcheck // sse.Register's f has no error return; 404 handled inside Stream via early close
	})
}

// ProjectEventToSSEFrame serializes ev to the raw SSE frame bytes used by the HTTP
// streaming endpoint and the facade conformance projector.
// Wire format: "event: <kind>\ndata: <json>\n\n"
func ProjectEventToSSEFrame(ev ports.Event) []byte { //nolint:gocritic // hugeParam: public API used by value in conformance tests; signature cannot change
	d := eventDataFromEvent(ev)
	data, _ := json.Marshal(d) //nolint:errcheck // controlled struct, cannot fail
	return fmt.Appendf(nil, "event: %s\ndata: %s\n\n", d.Kind, data)
}
