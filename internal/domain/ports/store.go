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

// ResumeLeaseStore serializes the complete load-and-resume lifecycle for a
// workflow across independent StateStore instances and processes. The lease
// must remain held until resumed execution finishes so another caller cannot
// treat a newly allocated running attempt as a stale interrupted attempt.
type ResumeLeaseStore interface {
	AcquireResumeLease(ctx context.Context, workflowID string) (release func() error, err error)
}
