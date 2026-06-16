package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

const (
	defaultHistoryLimit = 20
	retentionPeriod     = 30 * 24 * time.Hour // 30 days
)

// ErrRecordNotFound is returned by GetByID when no execution record matches the
// requested ID. Callers should test with errors.Is(err, ErrRecordNotFound).
var ErrRecordNotFound = errors.New("execution record not found")

// HistoryService provides business logic for workflow execution history.
type HistoryService struct {
	store  ports.HistoryStore
	logger ports.Logger
}

// NewHistoryService constructs a HistoryService backed by the given store and
// logger. Either dependency may be nil; nil-store calls degrade gracefully.
func NewHistoryService(store ports.HistoryStore, logger ports.Logger) *HistoryService {
	return &HistoryService{
		store:  store,
		logger: logger,
	}
}

// Record persists a single execution record to the history store.
// Returns nil without error when the store is not configured (degraded mode).
func (s *HistoryService) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.Record(ctx, record)
}

// List returns execution records matching the given filter.
// Returns an error when the store is not configured so callers can distinguish
// "no records" from "store unavailable".
func (s *HistoryService) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	if s.store == nil {
		return nil, fmt.Errorf("list history records: history store not configured")
	}
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

// GetByID returns the execution record with the given ID.
// Returns (nil, nil) when the store is not configured (degraded mode).
// Returns ErrRecordNotFound when no record matches.
// The HistoryStore port does not expose an ID-scoped lookup, so this method
// performs a full List scan; the O(N) cost is centralized here so callers do
// not duplicate the scan logic.
func (s *HistoryService) GetByID(ctx context.Context, id string) (*workflow.ExecutionRecord, error) {
	if s.store == nil {
		return nil, nil
	}

	// Fetch all records without a workflow-name or status constraint so that the
	// scan covers every execution. A zero Limit is treated as "no limit" here;
	// we pass a large sentinel that is unlikely to be reached in practice while
	// still bounding memory usage.
	const scanLimit = 10_000
	filter := &workflow.HistoryFilter{Limit: scanLimit}

	records, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get execution record by id %q: %w", id, err)
	}

	for _, r := range records {
		if r.ID == id {
			return r, nil
		}
	}

	return nil, fmt.Errorf("get execution record by id %q: %w", id, ErrRecordNotFound)
}

// GetStats returns aggregated execution statistics for the given filter.
// Returns (nil, nil) when the store is not configured (degraded mode).
func (s *HistoryService) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	if s.store == nil {
		return nil, nil
	}
	stats, err := s.store.GetStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get history stats: %w", err)
	}
	return stats, nil
}
