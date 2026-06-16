package application

import (
	"context"
	"errors"
	"strings"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// Resolver consolidates pack-based and repository-based workflow resolution into
// a single canonical entry point for the application layer.
// The three prior implementations (cli/pack_resolver.go, tui/command.go:resolvePackWorkflow,
// subworkflow_executor.go:SplitCallWorkflowName) are replaced here; coexistence is
// preserved until T060–T063 migrate each interface.
type Resolver struct {
	packDiscoverer ports.PackDiscoverer
	repo           ports.WorkflowRepository
}

func NewResolver(packDiscoverer ports.PackDiscoverer, repo ports.WorkflowRepository) *Resolver {
	return &Resolver{
		packDiscoverer: packDiscoverer,
		repo:           repo,
	}
}

// Resolve parses a canonical "pack/workflow" identifier and loads the corresponding workflow.
// Empty identifier and missing "/" segment return USER.FACADE.* structured errors (declared by T055).
func (r *Resolver) Resolve(ctx context.Context, identifier string) (*workflow.Workflow, error) {
	if identifier == "" {
		return nil, domainerrors.NewUserError(
			domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
			"workflow identifier is empty",
			nil,
			nil,
		)
	}

	parts := strings.SplitN(identifier, "/", 2)
	var packName, workflowName string
	if len(parts) < 2 {
		// Bare workflow name (no pack separator): resolve by name against the
		// repository, identical to the explicit "*/name" wildcard. This is the
		// single-core canonicalization — every interface (CLI, TUI, HTTP, ACP) may
		// pass a bare local name and it is owned here, not duplicated per interface.
		packName, workflowName = "*", identifier
	} else {
		packName, workflowName = parts[0], parts[1]
		// A separator is present but a segment is empty ("pack/" or "/wf"): the
		// identifier is genuinely malformed (distinct from a bare name).
		if packName == "" || workflowName == "" {
			return nil, domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeIdentifierMalformed,
				"workflow identifier has an empty segment: "+identifier,
				map[string]any{"identifier": identifier},
				nil,
			)
		}
	}

	// "*" is the wildcard pack meaning "resolve by workflow name only" (used by the CLI
	// commands that take a bare local name). The pack discoverer rejects "*" as an invalid
	// pack name, so skip it and resolve straight from the repository.
	if packName != "*" {
		wf, err := r.packDiscoverer.LoadWorkflow(ctx, packName, workflowName)
		if err != nil {
			return nil, err
		}
		if wf != nil {
			return wf, nil
		}
	}

	// Pack not declared in discoverer (or wildcard "*"); fall back to repository.
	wf, err := r.repo.Load(ctx, workflowName)
	// Distinguish "file absent" from "file present but broken": a genuine missing-file error
	// collapses into the friendly facade not-found below, but any other load/parse error
	// (a malformed or semantically-invalid workflow that exists) must surface as-is —
	// otherwise `awf validate` reports "workflow not found" for a file that is present.
	if err != nil {
		var se *domainerrors.StructuredError
		if !errors.As(err, &se) || se.Code != domainerrors.ErrorCodeUserInputMissingFile {
			return nil, err
		}
		// missing file → fall through to the friendly not-found handling (wf is nil)
	}
	if wf == nil {
		// Use pack name to distinguish: "*" means user searched by workflow name only.
		if packName == "*" {
			return nil, domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
				"workflow not found: "+workflowName,
				map[string]any{"workflow": workflowName},
				nil,
			)
		}
		return nil, domainerrors.NewUserError(
			domainerrors.ErrorCodeUserFacadePackNotFound,
			"pack not found: "+packName,
			map[string]any{"pack": packName, "workflow": workflowName},
			nil,
		)
	}

	return wf, nil
}
