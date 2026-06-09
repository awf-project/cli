package application

import (
	"context"
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
	if len(parts) < 2 {
		return nil, domainerrors.NewUserError(
			domainerrors.ErrorCodeUserFacadeIdentifierMalformed,
			"workflow identifier must contain a pack separator: "+identifier,
			map[string]any{"identifier": identifier},
			nil,
		)
	}

	packName, workflowName := parts[0], parts[1]

	wf, err := r.packDiscoverer.LoadWorkflow(ctx, packName, workflowName)
	if err != nil {
		return nil, err
	}
	if wf != nil {
		return wf, nil
	}

	// Pack not declared in discoverer; fall back to repository.
	wf, err = r.repo.Load(ctx, workflowName)
	if err != nil || wf == nil {
		// Use pack name to distinguish: "*" means user searched by workflow name only.
		if packName == "*" {
			return nil, domainerrors.NewUserError(
				domainerrors.ErrorCodeUserFacadeWorkflowNotFound,
				"workflow not found: "+workflowName,
				map[string]any{"workflow": workflowName},
				err,
			)
		}
		return nil, domainerrors.NewUserError(
			domainerrors.ErrorCodeUserFacadePackNotFound,
			"pack not found: "+packName,
			map[string]any{"pack": packName, "workflow": workflowName},
			err,
		)
	}

	return wf, nil
}
