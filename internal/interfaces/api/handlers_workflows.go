package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/danielgtaylor/huma/v2"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
)

// WorkflowHandlers exposes workflow read operations (list, get, validate) via
// HTTP. It is bound to a Bridge which holds the WorkflowLister adapter to the
// application service layer.
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
		slog.Error("list workflows: internal error", slog.Any("error", err))
		return nil, huma.Error500InternalServerError("failed to list workflows")
	}
	summaries := make([]WorkflowSummary, 0, len(entries))
	for _, e := range entries {
		summaries = append(summaries, WorkflowSummary{
			Name:        e.Name,
			Scope:       e.Scope,
			Workflow:    e.Workflow,
			Version:     e.Version,
			Description: e.Description,
		})
	}
	out := &ListWorkflowsOutput{}
	out.Body.Body = listWorkflowsBody{Workflows: summaries}
	return out, nil
}

func (h *WorkflowHandlers) Get(ctx context.Context, in *GetWorkflowInput) (*GetWorkflowOutput, error) {
	id := recomposeIdentifier(in.Scope, in.Name)
	wf, err := h.b.workflows.GetWorkflow(ctx, id)
	if err != nil {
		// Return 404 only for genuine "file not found" errors so that YAML
		// parse errors, permission failures, and other internal errors do not
		// masquerade as missing workflows. Log internals and return 500 for
		// anything that is not a missing-file domain error.
		var se *domainerrors.StructuredError
		if errors.As(err, &se) && se.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", id))
		}
		slog.Error("get workflow: internal error", slog.String("id", id), slog.Any("error", err))
		return nil, huma.Error500InternalServerError("failed to load workflow")
	}
	out := &GetWorkflowOutput{}
	out.Body.Body = wf
	return out, nil
}

func (h *WorkflowHandlers) Validate(ctx context.Context, in *ValidateWorkflowInput) (*ValidateWorkflowOutput, error) {
	id := recomposeIdentifier(in.Scope, in.Name)

	// Probe existence first so a missing workflow returns 404 rather than 200
	// with a synthetic validation error, which would be misleading to callers.
	if _, getErr := h.b.workflows.GetWorkflow(ctx, id); getErr != nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", id))
	}

	err := h.b.workflows.ValidateWorkflow(ctx, id)
	if err != nil {
		// M-5: return 422 Unprocessable Entity for a workflow that fails validation.
		// This distinguishes a "well-formed request that produced validation errors"
		// (422) from a "server-side processing failure" (500). 200 would be misleading
		// because the resource exists but is structurally invalid.
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	out := &ValidateWorkflowOutput{}
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
		Path:        "/api/workflows/{scope}/{name}",
		OperationID: "get-workflow",
		Tags:        []string{"Workflows"},
	}, h.Get)

	huma.Register(api, huma.Operation{
		Method:      "POST",
		Path:        "/api/workflows/{scope}/{name}/validate",
		OperationID: "validate-workflow",
		Tags:        []string{"Workflows"},
	}, h.Validate)
}
