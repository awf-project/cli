package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// StateStore defines the contract for persisting workflow execution state.
type StateStore interface {
	Save(ctx context.Context, state *workflow.ExecutionContext) error
	Load(ctx context.Context, workflowID string) (*workflow.ExecutionContext, error)
	Delete(ctx context.Context, workflowID string) error
	List(ctx context.Context) ([]string, error)
}
