package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
)

// WorkflowHandlers exposes workflow read operations (list, get, validate) via HTTP.
//
// All operations route through the common application facade rather than legacy Bridge
// ports (F108 read-path consolidation): list and validate go through ports.WorkflowFacade,
// and the single-workflow fetch goes through the focused ports.WorkflowReader. Both are
// satisfied by the same application.Adapter, so every HTTP read shares the one execution
// core the CLI/TUI/ACP interfaces also drive through.
type WorkflowHandlers struct {
	facade ports.WorkflowFacade
	reader ports.WorkflowReader
}

// NewWorkflowHandlers creates a WorkflowHandlers bound to the facade and reader ports.
// Either may be nil (e.g. a server constructed without WithFacade/WithWorkflowReader); the
// handlers degrade to 503 rather than panicking, mirroring the execution handlers.
func NewWorkflowHandlers(facade ports.WorkflowFacade, reader ports.WorkflowReader) *WorkflowHandlers {
	return &WorkflowHandlers{facade: facade, reader: reader}
}

func (h *WorkflowHandlers) List(ctx context.Context, _ *struct{}) (*ListWorkflowsOutput, error) {
	if h.facade == nil {
		return nil, huma.Error503ServiceUnavailable("workflow listing is temporarily unavailable")
	}
	summaries, err := h.facade.List(ctx)
	if err != nil {
		slog.Error("list workflows: internal error", slog.Any("error", err))
		return nil, huma.Error500InternalServerError("failed to list workflows")
	}
	apiSummaries := make([]WorkflowSummary, 0, len(summaries))
	for _, s := range summaries {
		apiSummaries = append(apiSummaries, WorkflowSummary{
			Name:        s.Name,
			Scope:       s.Scope,
			Workflow:    s.Workflow,
			Version:     s.Version,
			Description: s.Description,
		})
	}
	out := &ListWorkflowsOutput{}
	out.Body.Body = listWorkflowsBody{Workflows: apiSummaries}
	return out, nil
}

func (h *WorkflowHandlers) Get(ctx context.Context, in *GetWorkflowInput) (*GetWorkflowOutput, error) {
	if h.reader == nil {
		return nil, huma.Error503ServiceUnavailable("workflow retrieval is temporarily unavailable")
	}
	id := recomposeIdentifier(in.Scope, in.Name)
	wf, err := h.reader.GetWorkflow(ctx, id)
	if err != nil {
		// Return 404 only for genuine "file not found" errors so that YAML parse errors,
		// permission failures, and other internal errors do not masquerade as missing
		// workflows. Log internals and return 500 for anything that is not a missing-file
		// domain error.
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
	if h.facade == nil || h.reader == nil {
		return nil, huma.Error503ServiceUnavailable("workflow validation is temporarily unavailable")
	}
	id := recomposeIdentifier(in.Scope, in.Name)

	// Probe existence first so a missing workflow returns 404 rather than 200 with a
	// synthetic validation error, or 500 from a resolver not-found. This preserves the
	// pre-facade two-step semantics (existence → validity) precisely.
	if _, getErr := h.reader.GetWorkflow(ctx, id); getErr != nil {
		var se *domainerrors.StructuredError
		if errors.As(getErr, &se) && se.Code == domainerrors.ErrorCodeUserInputMissingFile {
			return nil, huma.Error404NotFound(fmt.Sprintf("workflow not found: %s", id))
		}
		slog.Error("validate workflow: internal error", slog.String("id", id), slog.Any("error", getErr))
		return nil, huma.Error500InternalServerError("failed to validate workflow")
	}

	report, err := h.facade.Validate(ctx, ports.RunRequest{Identifier: id})
	if err != nil {
		slog.Error("validate workflow: internal error", slog.String("id", id), slog.Any("error", err))
		return nil, huma.Error500InternalServerError("failed to validate workflow")
	}
	if !report.Valid {
		// M-5: return 422 Unprocessable Entity for a workflow that fails validation.
		// This distinguishes a "well-formed request that produced validation errors" (422)
		// from a "server-side processing failure" (500). 200 would be misleading because
		// the resource exists but is structurally invalid.
		msgs := make([]string, 0, len(report.Errors))
		for _, ve := range report.Errors {
			msgs = append(msgs, ve.Message)
		}
		return nil, huma.Error422UnprocessableEntity(strings.Join(msgs, "; "))
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
