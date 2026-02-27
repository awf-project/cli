package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// StepExecutor defines the contract for executing a single workflow step.
// This is used by parallel execution to delegate individual branch execution.
type StepExecutor interface {
	// ExecuteStep runs a single step and returns the result.
	// The step is looked up by name from the workflow.
	ExecuteStep(
		ctx context.Context,
		wf *workflow.Workflow,
		stepName string,
		execCtx *workflow.ExecutionContext,
	) (*workflow.BranchResult, error)
}
