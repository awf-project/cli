package ports

import (
	"context"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// HistoryStore defines the contract for persisting workflow execution history.
type HistoryStore interface {
	Record(ctx context.Context, record *workflow.ExecutionRecord) error
	List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error)
	GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error)
	Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
	Close() error
}
