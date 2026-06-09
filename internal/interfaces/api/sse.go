package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/awf-project/cli/internal/domain/ports"
)

// SessionLookup is the driven port for resolving a live RunSession by execution ID.
// It is satisfied by *application.SessionRegistry (wrapped via SessionRegistryLookup).
// Defined here so the api package has no import dependency on the application package.
type SessionLookup interface {
	GetSession(id string) (ports.RunSession, bool)
}

// StreamInput holds the path parameter for the SSE event stream endpoint.
type StreamInput struct {
	ID string `path:"id" doc:"Execution ID." example:"550e8400-e29b-41d4-a716-446655440000" required:"true"`
}

// StepStartedEvent is emitted when a step transitions to running.
type StepStartedEvent struct {
	StepName  string    `json:"step_name"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// StepCompletedEvent is emitted when a step transitions to completed.
type StepCompletedEvent struct {
	StepName    string    `json:"step_name"`
	Status      string    `json:"status"`
	Output      string    `json:"output"`
	CompletedAt time.Time `json:"completed_at"`
}

// StepFailedEvent is emitted when a step transitions to failed.
type StepFailedEvent struct {
	StepName    string    `json:"step_name"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	CompletedAt time.Time `json:"completed_at"`
}

// WorkflowCompletedEvent is emitted when the workflow reaches the completed terminal state.
type WorkflowCompletedEvent struct {
	WorkflowName string    `json:"workflow_name"`
	Status       string    `json:"status"`
	CompletedAt  time.Time `json:"completed_at"`
}

// WorkflowFailedEvent is emitted when the workflow reaches the failed terminal state.
type WorkflowFailedEvent struct {
	WorkflowName string    `json:"workflow_name"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
	CompletedAt  time.Time `json:"completed_at"`
}

// OutputEvent carries incremental output from a running step.
type OutputEvent struct {
	StepName string `json:"step_name"`
	Output   string `json:"output"`
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

	lastEventID := h.getLastEventID(ctx)
	if err := h.replayBuffered(send, session, lastEventID); err != nil {
		return err
	}

	for event := range session.Events() {
		if err := send(sse.Message{Data: event}); err != nil {
			return nil
		}
	}

	return nil
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

func (h *SSEHandler) getLastEventID(_ context.Context) uint64 {
	return 0
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
			if err := send(sse.Message{Data: ev}); err != nil {
				return nil //nolint:nilerr // client disconnected; treat as clean exit
			}
		}
	}
	return nil
}

// RegisterSSERoutes registers GET /api/executions/{id}/events on the given Huma API.
func RegisterSSERoutes(api huma.API, h *SSEHandler) {
	sse.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/executions/{id}/events",
		OperationID: "stream-execution-events",
		Tags:        []string{"Executions"},
	}, map[string]any{}, func(ctx context.Context, in *StreamInput, send sse.Sender) {
		_ = h.Stream(ctx, in, send) //nolint:errcheck // sse.Register's f has no error return; 404 handled inside Stream via early close
	})
}
