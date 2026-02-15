package application

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
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

func NewHistoryService(store ports.HistoryStore, logger ports.Logger) *HistoryService {
	return &HistoryService{
		store:  store,
		logger: logger,
	}
}

func (s *HistoryService) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	return s.store.Record(ctx, record)
}

func (s *HistoryService) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	if filter.Limit == 0 {
		filter.Limit = defaultHistoryLimit
	}
	records, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list history records: %w", err)
	}
	return records, nil
}

func (s *HistoryService) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	stats, err := s.store.GetStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get history stats: %w", err)
	}
	return stats, nil
}

func (s *HistoryService) Cleanup(ctx context.Context) (int, error) {
	count, err := s.store.Cleanup(ctx, retentionPeriod)
	if err != nil {
		return 0, fmt.Errorf("cleanup history: %w", err)
	}
	return count, nil
}

func (s *HistoryService) Close() error {
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("close history store: %w", err)
	}
	return nil
}
