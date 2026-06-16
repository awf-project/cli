package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielgtaylor/huma/v2"

	"github.com/awf-project/cli/internal/domain/ports"
)

// RespondInput holds the path parameter for the respond endpoint.
type RespondInput struct {
	ID   string `path:"id" doc:"Execution ID." example:"550e8400-e29b-41d4-a716-446655440000" required:"true"`
	Body struct {
		Response ports.InputResponse `json:"response" doc:"User input response."`
	}
}

// RespondHandler handles user input submission for running workflows.
type RespondHandler struct {
	facade   ports.WorkflowFacade
	sessions SessionLookup
}

// NewRespondHandler creates a RespondHandler bound to the given facade.
func NewRespondHandler(facade ports.WorkflowFacade) *RespondHandler {
	return &RespondHandler{facade: facade}
}

// SetSessionLookup wires the session registry into the handler so getSession can
// resolve live RunSessions by ID. Must be called before the first request is served.
func (h *RespondHandler) SetSessionLookup(sl SessionLookup) {
	h.sessions = sl
}

// Respond delegates to the session's Respond method to deliver user input.
func (h *RespondHandler) Respond(_ context.Context, in *RespondInput) (*struct{}, error) {
	session, err := h.getSession(in.ID)
	if err != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("execution not found: %s", in.ID))
	}

	if err := session.Respond(in.Body.Response); err != nil {
		// ErrSessionClosed and ErrDuplicateResponse are plain sentinels (not StructuredError)
		// emitted directly by the session implementation. They indicate a lifecycle conflict
		// rather than a malformed request, so 409 Conflict is more accurate than 422.
		if errors.Is(err, ports.ErrSessionClosed) || errors.Is(err, ports.ErrDuplicateResponse) {
			return nil, huma.Error409Conflict(fmt.Sprintf("cannot respond: %s", err))
		}
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("failed to send response: %s", err))
	}

	return nil, nil
}

// getSession resolves a live RunSession by ID. Returns a descriptive error when
// the session registry is not configured or the ID is unknown — never (nil, nil).
func (h *RespondHandler) getSession(id string) (ports.RunSession, error) {
	if h.sessions == nil {
		return nil, fmt.Errorf("session registry not configured")
	}
	session, ok := h.sessions.GetSession(id)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return session, nil
}

// RegisterRespondRoutes registers POST /runs/{id}/respond on the given Huma API.
func RegisterRespondRoutes(api huma.API, h *RespondHandler) {
	huma.Register(api, huma.Operation{
		Method:        "POST",
		Path:          "/api/executions/{id}/respond",
		OperationID:   "respond-to-input",
		Tags:          []string{"Executions"},
		DefaultStatus: 204,
	}, h.Respond)
}
