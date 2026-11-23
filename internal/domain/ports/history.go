package ports

import (
	"context"
	"errors"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// ErrHistoryRecordConflict reports reuse of a record ID or workflow attempt
// identity with different persisted data. Callers use errors.Is to tell it
// apart from a transient write failure.
var ErrHistoryRecordConflict = errors.New("history record conflict")

// HistoryStore defines the contract for persisting workflow execution history.
type HistoryStore interface {
	Record(ctx context.Context, record *workflow.ExecutionRecord) error
	List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error)
	GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error)
	Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
	Close() error
}
