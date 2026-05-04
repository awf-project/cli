package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// PackDiscoverer discovers installed workflow packs and loads individual pack workflows.
type PackDiscoverer interface {
	DiscoverWorkflows(ctx context.Context) ([]workflow.WorkflowEntry, error)
	LoadWorkflow(ctx context.Context, packName, workflowName string) (*workflow.Workflow, error)
}
