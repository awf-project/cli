package api

import (
	"context"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
)

// ExecutionHandlers exposes execution lifecycle operations via HTTP.
type ExecutionHandlers struct {
	b *Bridge
}

// NewExecutionHandlers creates an ExecutionHandlers bound to the given Bridge.
func NewExecutionHandlers(b *Bridge) *ExecutionHandlers {
	return &ExecutionHandlers{b: b}
}

func (h *ExecutionHandlers) Run(ctx context.Context, in *RunWorkflowInput) (*RunWorkflowOutput, error) {
	id := recomposeIdentifier(in.Scope, in.Name)
	wf, err := h.b.workflows.GetWorkflow(ctx, id)
	if err != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", id))
	}
	execID, _, err := h.b.StartExecution(ctx, wf, in.Body.Inputs)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(fmt.Sprintf("failed to start execution: %s", err))
	}
	out := &RunWorkflowOutput{}
	out.Body.Body = runWorkflowBody{ExecutionID: execID, Status: "accepted"}
	return out, nil
}

func (h *ExecutionHandlers) List(_ context.Context, _ *struct{}) (*ListExecutionsOutput, error) {
	active := h.b.ListExecutions()
	bodies := make([]executionBody, 0, len(active))
	for _, ae := range active {
		bodies = append(bodies, activeExecutionToBody(ae))
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
	out.Body.Body = activeExecutionToBody(ae)
	return out, nil
}

func (h *ExecutionHandlers) Cancel(_ context.Context, in *CancelExecutionInput) (*struct{}, error) {
	_ = h.b.CancelExecution(in.ID) //nolint:errcheck // idempotent: 204 regardless of whether execution exists
	return nil, nil
}

func (h *ExecutionHandlers) Resume(ctx context.Context, in *ResumeExecutionInput) (*RunWorkflowOutput, error) {
	if h.b.resumer == nil {
		return nil, huma.Error422UnprocessableEntity("resume is not available: no resumer configured")
	}
	execCtx, err := h.b.resumer.Resume(ctx, in.ID, in.Body.InputOverrides, in.Body.FromStep)
	if err != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("execution not found or cannot be resumed: %s", in.ID))
	}
	id := h.b.TrackResumedExecution(execCtx)
	out := &RunWorkflowOutput{}
	out.Body.Body = runWorkflowBody{ExecutionID: id, Status: "accepted"}
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
