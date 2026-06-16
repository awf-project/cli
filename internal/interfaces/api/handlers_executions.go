package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	"github.com/awf-project/cli/internal/application"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
)

// ExecutionHandlers exposes execution lifecycle operations via HTTP.
type ExecutionHandlers struct {
	b      *Bridge
	facade ports.WorkflowFacade
	reg    *application.SessionRegistry
}

func (h *ExecutionHandlers) SetFacade(f ports.WorkflowFacade) { h.facade = f }

func (h *ExecutionHandlers) SetSessionRegistry(reg *application.SessionRegistry) { h.reg = reg }

// runViaFacade drives the single-core facade path (F108). It calls facade.Run to obtain
// a live RunSession, tracks lightweight metadata in the Bridge for List/Get/Cancel,
// and returns 202 with the session's own ID as the execution ID. The session's ID() is
// authoritative — it is the key under which SessionRegistry.Get(execID) resolves later.
//
// FR-003: the session is registered into the shared SessionRegistry synchronously inside
// facade.Run → Adapter.newSession, before this function returns. The Adapter was wired
// with the same *SessionRegistry as WithRegistryImpl (B2 fix), so a subscriber connecting
// right after /run MUST find the session without any additional Add call here.
func (h *ExecutionHandlers) runViaFacade(ctx context.Context, in *RunWorkflowInput) (*RunWorkflowOutput, error) {
	id := recomposeIdentifier(in.Scope, in.Name)
	// The workflow runs asynchronously and MUST outlive this HTTP request: the handler
	// returns 202 immediately, which cancels the request context. Detaching execution
	// from the request lifetime (while preserving request-scoped values) prevents the run
	// from being killed the moment the 202 response is written. Teardown is owned by the
	// session (Cancel / server shutdown), not the request.
	runCtx := context.WithoutCancel(ctx)
	session, err := h.facade.Run(runCtx, ports.RunRequest{
		Identifier: id,
		Inputs:     in.Body.Inputs,
	})
	if err != nil {
		// Resolver rejections (empty/unknown identifier) surface as not-found USER errors;
		// everything else is an unprocessable request. Mirrors the legacy 404/422 split.
		if errors.Is(err, ports.ErrInvalidRequest) || errors.Is(err, ports.ErrRunNotFound) {
			return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", id))
		}
		var se *domainerrors.StructuredError
		if errors.As(err, &se) && se.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", id))
		}
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("failed to start execution: %s", err))
	}

	// Ensure the session is in the registry. The Adapter's newSession pre-registers the
	// session synchronously (B2 fix), so Add returns ErrSessionExists on the production
	// path — that is expected and silently ignored. Non-Adapter facades (test doubles,
	// future alternate implementations) may not pre-register, so this try-Add guarantees
	// FR-003 compliance regardless of facade implementation.
	if addErr := h.reg.Add(session); addErr != nil && !errors.Is(addErr, ports.ErrSessionExists) {
		slog.Warn("session registry add failed", slog.String("id", session.ID()), slog.Any("error", addErr))
	}

	// Track metadata in the Bridge so List/Get/Cancel keep working (A4 / R5). The
	// RunSession lives in the registry; activeExecutions only holds metadata.
	h.b.TrackFacadeSession(session, id)

	out := &RunWorkflowOutput{}
	out.Body.Body = runWorkflowBody{ExecutionID: session.ID(), Status: "accepted"}
	return out, nil
}

// NewExecutionHandlers creates an ExecutionHandlers bound to the given Bridge.
func NewExecutionHandlers(b *Bridge) *ExecutionHandlers {
	return &ExecutionHandlers{b: b}
}

func (h *ExecutionHandlers) Run(ctx context.Context, in *RunWorkflowInput) (*RunWorkflowOutput, error) {
	if h.facade == nil || h.reg == nil {
		// Return a generic 503 rather than exposing internal configuration detail to clients.
		return nil, huma.Error503ServiceUnavailable("workflow execution is temporarily unavailable")
	}
	return h.runViaFacade(ctx, in)
}

func (h *ExecutionHandlers) List(_ context.Context, _ *struct{}) (*ListExecutionsOutput, error) {
	active := h.b.ListExecutions()
	bodies := make([]executionBody, 0, len(active))
	for _, ae := range active {
		bodies = append(bodies, h.executionBodyFor(ae))
	}
	out := &ListExecutionsOutput{}
	out.Body.Body = listExecutionsBody{Executions: bodies}
	return out, nil
}

func (h *ExecutionHandlers) Get(_ context.Context, in *GetExecutionInput) (*ExecutionOutput, error) {
	ae, ok := h.b.GetExecution(in.ID)
	if !ok {
		return nil, huma.Error404NotFound(fmt.Sprintf("execution not found: %s", in.ID))
	}
	out := &ExecutionOutput{}
	out.Body.Body = h.executionBodyFor(ae)
	return out, nil
}

// executionBodyFor renders the response body for one execution. A legacy execution that
// carries an ExecutionContext is rendered directly from it. A facade-tracked session (no
// ExecutionContext, because the HTTP facade uses a NopRecorder) reports the track-time
// StartedAt and overlays live status from the session's non-consuming replay-buffer
// snapshot — so GET reflects running → completed/failed without competing with the SSE
// consumer for the live event channel. The closed session stays in the registry and keeps
// its buffer, so a finished run still reports its terminal status.
func (h *ExecutionHandlers) executionBodyFor(ae *ActiveExecution) executionBody {
	if ae.ExecutionContext != nil {
		return activeExecutionToBody(ae)
	}
	body := executionBody{
		ExecutionID:  ae.ExecutionID,
		WorkflowName: ae.WorkflowName,
		Status:       "running",
		StartedAt:    ae.StartedAt,
		UpdatedAt:    ae.StartedAt,
	}
	if h.reg == nil {
		return body
	}
	sess, ok := h.reg.Get(ae.ExecutionID)
	if !ok {
		return body
	}
	snapper, ok := sess.(interface{ StatusSnapshot() ports.RunStatus })
	if !ok {
		return body
	}
	snap := snapper.StatusSnapshot()
	if snap.Status != "" {
		// RunStatus.Status is ports.RunState; string conversion is safe here because
		// executionBody.Status is a plain string for JSON serialization purposes.
		body.Status = string(snap.Status)
	}
	if snap.CurrentStep != "" {
		body.CurrentStep = snap.CurrentStep
	}
	if !snap.CompletedAt.IsZero() {
		body.UpdatedAt = snap.CompletedAt
	}
	return body
}

func (h *ExecutionHandlers) Cancel(_ context.Context, in *CancelExecutionInput) (*struct{}, error) {
	_ = h.b.CancelExecution(in.ID) //nolint:errcheck // idempotent: 204 regardless of whether execution exists
	return nil, nil
}

func (h *ExecutionHandlers) Resume(ctx context.Context, in *ResumeExecutionInput) (*RunWorkflowOutput, error) {
	if h.facade == nil || h.reg == nil {
		// Return a generic 503 rather than exposing internal configuration detail to clients.
		return nil, huma.Error503ServiceUnavailable("workflow execution is temporarily unavailable")
	}
	return h.resumeViaFacade(ctx, in)
}

// resumeViaFacade drives the single-core facade path for Resume (F108). It calls
// facade.Resume with inputOverrides and fromStep so no power-user parameters are lost,
// registers the returned RunSession in the SessionRegistry synchronously (FR-003), and
// tracks metadata in the Bridge for List/Get/Cancel compatibility (A4 / R5).
func (h *ExecutionHandlers) resumeViaFacade(ctx context.Context, in *ResumeExecutionInput) (*RunWorkflowOutput, error) {
	// Resume runs asynchronously and MUST outlive the request — see runViaFacade.
	runCtx := context.WithoutCancel(ctx)
	session, err := h.facade.Resume(runCtx, ports.ResumeRequest{
		RunID:          in.ID,
		InputOverrides: in.Body.InputOverrides,
		FromStep:       in.Body.FromStep,
	})
	if err != nil {
		if errors.Is(err, ports.ErrRunNotFound) || errors.Is(err, application.ErrExecutionNotFound) {
			return nil, huma.Error404NotFound(fmt.Sprintf("execution not found: %s", in.ID))
		}
		slog.Error("resume via facade: internal error", slog.String("id", in.ID), slog.Any("error", err))
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("cannot resume execution: %s", err))
	}

	// Ensure the session is in the registry (same try-Add pattern as runViaFacade).
	// The Adapter pre-registers on the production path; non-Adapter facades do not.
	if addErr := h.reg.Add(session); addErr != nil && !errors.Is(addErr, ports.ErrSessionExists) {
		slog.Warn("session registry add failed (resume)", slog.String("id", session.ID()), slog.Any("error", addErr))
	}

	// Track metadata in the Bridge so List/Get/Cancel keep working (A4 / R5).
	// The resumed session keeps the original runID as its session ID so lookups resolve.
	h.b.TrackFacadeSession(session, in.ID)

	out := &RunWorkflowOutput{}
	out.Body.Body = runWorkflowBody{ExecutionID: session.ID(), Status: "accepted"}
	return out, nil
}

func activeExecutionToBody(ae *ActiveExecution) executionBody {
	body := executionBody{
		ExecutionID:  ae.ExecutionID,
		WorkflowName: ae.WorkflowName,
	}
	if ae.ExecutionContext != nil {
		body.Status = ae.ExecutionContext.GetStatus().String()
		body.CurrentStep = ae.ExecutionContext.GetCurrentStep()
		body.StartedAt = ae.ExecutionContext.StartedAt // set once in constructor, immutable
		body.UpdatedAt = ae.ExecutionContext.GetUpdatedAt()
	}
	return body
}

// RegisterExecutionRoutes mounts the execution lifecycle routes on the given Huma API.
func RegisterExecutionRoutes(api huma.API, h *ExecutionHandlers) {
	huma.Register(api, huma.Operation{
		Method:        "POST",
		Path:          "/api/workflows/{scope}/{name}/run",
		OperationID:   "run-workflow",
		Tags:          []string{"Executions"},
		DefaultStatus: 202,
	}, h.Run)

	huma.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/executions",
		OperationID: "list-executions",
		Tags:        []string{"Executions"},
	}, h.List)

	huma.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/executions/{id}",
		OperationID: "get-execution",
		Tags:        []string{"Executions"},
	}, h.Get)

	huma.Register(api, huma.Operation{
		Method:        "DELETE",
		Path:          "/api/executions/{id}",
		OperationID:   "cancel-execution",
		Tags:          []string{"Executions"},
		DefaultStatus: 204,
	}, h.Cancel)

	huma.Register(api, huma.Operation{
		Method:      "POST",
		Path:        "/api/executions/{id}/resume",
		OperationID: "resume-execution",
		Tags:        []string{"Executions"},
	}, h.Resume)
}
