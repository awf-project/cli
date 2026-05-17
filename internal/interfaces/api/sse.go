package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/sse"

	"github.com/awf-project/cli/internal/domain/workflow"
)

const (
	apiPollInterval = 200 * time.Millisecond
	eventOutput     = "output"
)

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

// eventRegistry maps audit-event constant strings to SSE event struct types.
// huma/sse uses Go reflect to derive the event name from the struct type at send time.
var eventRegistry = map[string]any{
	workflow.EventStepStarted:       StepStartedEvent{},
	workflow.EventStepCompleted:     StepCompletedEvent{},
	workflow.EventStepFailed:        StepFailedEvent{},
	workflow.EventWorkflowCompleted: WorkflowCompletedEvent{},
	workflow.EventWorkflowFailed:    WorkflowFailedEvent{},
	eventOutput:                     OutputEvent{},
}

// SSEHandler streams workflow execution events over Server-Sent Events.
type SSEHandler struct {
	b  *Bridge
	wg *sync.WaitGroup
}

// NewSSEHandler creates an SSEHandler bound to the given Bridge and WaitGroup.
func NewSSEHandler(b *Bridge, wg *sync.WaitGroup) *SSEHandler {
	return &SSEHandler{b: b, wg: wg}
}

// emitStepEvent sends the appropriate typed SSE event for a step status.
//
//nolint:gocritic // hugeParam: StepState passed by value intentionally; callers hold map values not pointers
func emitStepEvent(send sse.Sender, name string, state workflow.StepState) error {
	switch state.Status {
	case workflow.StatusRunning:
		return send(sse.Message{Data: StepStartedEvent{
			StepName:  name,
			Status:    string(state.Status),
			StartedAt: state.StartedAt,
		}})
	case workflow.StatusCompleted:
		return send(sse.Message{Data: StepCompletedEvent{
			StepName:    name,
			Status:      string(state.Status),
			Output:      state.Output,
			CompletedAt: state.CompletedAt,
		}})
	case workflow.StatusFailed:
		return send(sse.Message{Data: StepFailedEvent{
			StepName:    name,
			Status:      string(state.Status),
			Error:       state.Error,
			CompletedAt: state.CompletedAt,
		}})
	default:
		// StatusPending and StatusCancelled produce no step event.
		return nil
	}
}

// Stream polls the ExecutionContext every apiPollInterval and emits typed SSE
// events for each step state transition. Returns huma.Error404NotFound when the
// execution ID is unknown. Exits cleanly on terminal workflow state or ctx.Done().
func (h *SSEHandler) Stream(ctx context.Context, in *StreamInput, send sse.Sender) error {
	active, ok := h.b.GetExecution(in.ID)
	if !ok {
		return huma.Error404NotFound(fmt.Sprintf("execution not found: %s", in.ID))
	}

	h.wg.Add(1)
	defer h.wg.Done()

	ticker := time.NewTicker(apiPollInterval)
	defer ticker.Stop()

	prev := make(map[string]workflow.ExecutionStatus)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			execCtx := active.ExecutionContext
			states := execCtx.GetAllStepStates()

			for name := range states { //nolint:gocritic // rangeValCopy: StepState is 272 bytes; map lookup copies once vs range copying per iteration
				st := states[name]
				if prev[name] == st.Status {
					continue
				}
				if err := emitStepEvent(send, name, st); err != nil {
					return nil
				}
				prev[name] = st.Status
			}

			workflowStatus := execCtx.GetStatus()
			switch workflowStatus {
			case workflow.StatusCompleted:
				if err := send(sse.Message{Data: WorkflowCompletedEvent{
					WorkflowName: execCtx.WorkflowName,
					Status:       string(workflowStatus),
					CompletedAt:  execCtx.GetCompletedAt(),
				}}); err != nil {
					return nil
				}
				return nil
			case workflow.StatusFailed, workflow.StatusCancelled:
				if err := send(sse.Message{Data: WorkflowFailedEvent{
					WorkflowName: execCtx.WorkflowName,
					Status:       string(workflowStatus),
					CompletedAt:  execCtx.GetCompletedAt(),
				}}); err != nil {
					return nil
				}
				return nil
			default:
				// StatusPending and StatusRunning: no terminal event yet, continue polling.
			}
		}
	}
}

// RegisterSSERoutes registers GET /api/executions/{id}/events on the given Huma API.
func RegisterSSERoutes(api huma.API, h *SSEHandler) {
	sse.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/executions/{id}/events",
		OperationID: "stream-execution-events",
		Tags:        []string{"Executions"},
	}, eventRegistry, func(ctx context.Context, in *StreamInput, send sse.Sender) {
		_ = h.Stream(ctx, in, send) //nolint:errcheck // sse.Register's f has no error return; 404 handled inside Stream via early close
	})
}
