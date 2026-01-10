package application

import (
	"context"
	"fmt"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

const (
	defaultHistoryLimit = 20
	retentionPeriod     = 30 * 24 * time.Hour // 30 days
)

// HistoryService provides business logic for workflow execution history.
type HistoryService struct {
	store  ports.HistoryStore
	logger ports.Logger
}

// NewHistoryService creates a new history service.
func NewHistoryService(store ports.HistoryStore, logger ports.Logger) *HistoryService {
	return &HistoryService{
		store:  store,
		logger: logger,
	}
}

// Record stores a workflow execution record.
func (s *HistoryService) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	return s.store.Record(ctx, record)
}

// List retrieves execution records matching the filter criteria.
// Applies default limit of 20 if not specified.
func (s *HistoryService) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	// Apply default limit if not specified
	if filter.Limit == 0 {
		filter.Limit = defaultHistoryLimit
	}
	records, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list history records: %w", err)
	}
	return records, nil
}

// GetStats returns aggregated statistics for executions matching the filter.
func (s *HistoryService) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	stats, err := s.store.GetStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get history stats: %w", err)
	}
	return stats, nil
}

// Cleanup removes execution records older than 30 days.
// Returns the number of records deleted.
func (s *HistoryService) Cleanup(ctx context.Context) (int, error) {
	count, err := s.store.Cleanup(ctx, retentionPeriod)
	if err != nil {
		return 0, fmt.Errorf("cleanup history: %w", err)
	}
	return count, nil
}

// Close gracefully shuts down the history store.
func (s *HistoryService) Close() error {
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("close history store: %w", err)
	}
	return nil
}
