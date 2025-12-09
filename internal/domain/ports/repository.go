package ports

import (
	"context"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// WorkflowRepository defines the contract for loading workflow definitions.
type WorkflowRepository interface {
	Load(ctx context.Context, name string) (*workflow.Workflow, error)
	List(ctx context.Context) ([]string, error)
	Exists(ctx context.Context, name string) (bool, error)
}
