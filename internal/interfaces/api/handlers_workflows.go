package api

import (
	"context"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
)

// WorkflowHandlers exposes workflow read operations via HTTP.
type WorkflowHandlers struct {
	b *Bridge
}

// NewWorkflowHandlers creates a WorkflowHandlers bound to the given Bridge.
func NewWorkflowHandlers(b *Bridge) *WorkflowHandlers {
	return &WorkflowHandlers{b: b}
}

func (h *WorkflowHandlers) List(ctx context.Context, _ *struct{}) (*ListWorkflowsOutput, error) {
	entries, err := h.b.workflows.ListAllWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]WorkflowSummary, 0, len(entries))
	for _, e := range entries {
		summaries = append(summaries, WorkflowSummary{
			Name:        e.Name,
			Version:     e.Version,
			Description: e.Description,
		})
	}
	out := &ListWorkflowsOutput{}
	out.Body.Body = listWorkflowsBody{Workflows: summaries}
	return out, nil
}

func (h *WorkflowHandlers) Get(ctx context.Context, in *GetWorkflowInput) (*GetWorkflowOutput, error) {
	wf, err := h.b.workflows.GetWorkflow(ctx, in.Name)
	if err != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", in.Name))
	}
	out := &GetWorkflowOutput{}
	out.Body.Body = wf
	return out, nil
}

func (h *WorkflowHandlers) Validate(ctx context.Context, in *ValidateWorkflowInput) (*ValidateWorkflowOutput, error) {
	out := &ValidateWorkflowOutput{}
	err := h.b.workflows.ValidateWorkflow(ctx, in.Name)
	if err != nil {
		out.Body.Body = validateWorkflowBody{Errors: []string{err.Error()}}
	}
	return out, nil
}

// RegisterWorkflowRoutes mounts the three workflow read routes on the given Huma API.
func RegisterWorkflowRoutes(api huma.API, h *WorkflowHandlers) {
	huma.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/workflows",
		OperationID: "list-workflows",
		Tags:        []string{"Workflows"},
	}, h.List)

	huma.Register(api, huma.Operation{
		Method:      "GET",
		Path:        "/api/workflows/{name}",
		OperationID: "get-workflow",
		Tags:        []string{"Workflows"},
	}, h.Get)

	huma.Register(api, huma.Operation{
		Method:      "POST",
		Path:        "/api/workflows/{name}/validate",
		OperationID: "validate-workflow",
		Tags:        []string{"Workflows"},
	}, h.Validate)
}
