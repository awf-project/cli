package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// ParallelExecutor defines the contract for executing parallel branches.
type ParallelExecutor interface {
	// Execute runs multiple branches concurrently according to the given strategy.
	// It respects the MaxConcurrent limit via semaphore and applies the strategy
	// to determine overall success/failure.
	Execute(
		ctx context.Context,
		wf *workflow.Workflow,
		branches []string,
		config workflow.ParallelConfig,
		execCtx *workflow.ExecutionContext,
		stepExecutor StepExecutor,
	) (*workflow.ParallelResult, error)
}
