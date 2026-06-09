package ports

import (
	"context"
	"errors"
)

var (
	ErrInvalidRequest    = errors.New("invalid request")
	ErrSessionClosed     = errors.New("session closed")
	ErrDuplicateResponse = errors.New("duplicate response")
	ErrSessionExists     = errors.New("session already exists")
)

// WorkflowFacade is the primary port for workflow orchestration.
// Driving port — called by interface layer (CLI, API, MQ).
type WorkflowFacade interface {
	List(ctx context.Context) ([]WorkflowSummary, error)
	Validate(ctx context.Context, req RunRequest) (ValidationReport, error)
	Status(ctx context.Context, runID string) (RunStatus, error)
	History(ctx context.Context, filter HistoryFilter) ([]RunRecord, error)
	Run(ctx context.Context, req RunRequest) (RunSession, error)
	Resume(ctx context.Context, runID string) (RunSession, error)
}

// RunSession represents an active workflow execution.
// Close is idempotent: multiple calls return the same nil/error without panicking.
type RunSession interface {
	ID() string
	Events() <-chan Event
	Respond(InputResponse) error
	Err() error
	Close() error
}
