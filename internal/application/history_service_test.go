package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// mockHistoryStore implements ports.HistoryStore for testing
type mockHistoryStore struct {
	records     []*workflow.ExecutionRecord
	recordErr   error
	listErr     error
	statsErr    error
	cleanupErr  error
	closeErr    error
	cleanupAge  time.Duration
	cleanedCount int
}

func newMockHistoryStore() *mockHistoryStore {
	return &mockHistoryStore{
		records: make([]*workflow.ExecutionRecord, 0),
	}
}

func (m *mockHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	if m.recordErr != nil {
		return m.recordErr
	}
	m.records = append(m.records, record)
	return nil
}

func (m *mockHistoryStore) List(ctx context.Context, filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	if filter == nil {
		filter = &workflow.HistoryFilter{}
	}

	result := make([]*workflow.ExecutionRecord, 0)
	for _, r := range m.records {
		if filter.WorkflowName != "" && r.WorkflowName != filter.WorkflowName {
			continue
		}
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		if !filter.Since.IsZero() && r.CompletedAt.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && r.CompletedAt.After(filter.Until) {
			continue
		}
		result = append(result, r)
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 20
	}
	if len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (m *mockHistoryStore) GetStats(ctx context.Context, filter *workflow.HistoryFilter) (*workflow.HistoryStats, error) {
	if m.statsErr != nil {
		return nil, m.statsErr
	}

	records, _ := m.List(ctx, filter)
	stats := &workflow.HistoryStats{
		TotalExecutions: len(records),
	}

	var totalDuration int64
	for _, r := range records {
		switch r.Status {
		case "success":
			stats.SuccessCount++
		case "failed":
			stats.FailedCount++
		case "cancelled":
			stats.CancelledCount++
		}
		totalDuration += r.DurationMs
	}

	if stats.TotalExecutions > 0 {
		stats.AvgDurationMs = totalDuration / int64(stats.TotalExecutions)
	}

	return stats, nil
}

func (m *mockHistoryStore) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	if m.cleanupErr != nil {
		return 0, m.cleanupErr
	}
	m.cleanupAge = olderThan
	m.cleanedCount = 0

	cutoff := time.Now().Add(-olderThan)
	newRecords := make([]*workflow.ExecutionRecord, 0)
	for _, r := range m.records {
		if r.CompletedAt.After(cutoff) {
			newRecords = append(newRecords, r)
		} else {
			m.cleanedCount++
		}
	}
	m.records = newRecords
	return m.cleanedCount, nil
}

func (m *mockHistoryStore) Close() error {
	return m.closeErr
}

// mockHistoryLogger for testing
type mockHistoryLogger struct {
	warnCalls  []string
	errorCalls []string
}

func (m *mockHistoryLogger) Debug(msg string, fields ...any) {}
func (m *mockHistoryLogger) Info(msg string, fields ...any)  {}
func (m *mockHistoryLogger) Warn(msg string, fields ...any) {
	m.warnCalls = append(m.warnCalls, msg)
}
func (m *mockHistoryLogger) Error(msg string, fields ...any) {
	m.errorCalls = append(m.errorCalls, msg)
}
func (m *mockHistoryLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestNewHistoryService(t *testing.T) {
	store := newMockHistoryStore()
	logger := &mockHistoryLogger{}

	svc := application.NewHistoryService(store, logger)
	require.NotNil(t, svc)
}

func TestHistoryService_Record(t *testing.T) {
	tests := []struct {
		name      string
		record    *workflow.ExecutionRecord
		storeErr  error
		wantErr   bool
	}{
		{
			name: "record success execution",
			record: &workflow.ExecutionRecord{
				ID:           "exec-001",
				WorkflowID:   "wf-001",
				WorkflowName: "deploy",
				Status:       "success",
				StartedAt:    time.Now().Add(-time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			},
			wantErr: false,
		},
		{
			name: "record failed execution",
			record: &workflow.ExecutionRecord{
				ID:           "exec-002",
				WorkflowID:   "wf-002",
				WorkflowName: "build",
				Status:       "failed",
				ExitCode:     1,
				ErrorMessage: "build failed",
				StartedAt:    time.Now().Add(-30 * time.Second),
				CompletedAt:  time.Now(),
				DurationMs:   30000,
			},
			wantErr: false,
		},
		{
			name: "store error",
			record: &workflow.ExecutionRecord{
				ID:     "exec-003",
				Status: "success",
			},
			storeErr: errors.New("database error"),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockHistoryStore()
			store.recordErr = tc.storeErr
			logger := &mockHistoryLogger{}

			svc := application.NewHistoryService(store, logger)
			ctx := context.Background()

			err := svc.Record(ctx, tc.record)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Len(t, store.records, 1)
			assert.Equal(t, tc.record.ID, store.records[0].ID)
		})
	}
}

func TestHistoryService_List(t *testing.T) {
	tests := []struct {
		name          string
		setupRecords  []*workflow.ExecutionRecord
		filter        *workflow.HistoryFilter
		storeErr      error
		expectedCount int
		wantErr       bool
	}{
		{
			name:          "list empty",
			setupRecords:  nil,
			filter:        &workflow.HistoryFilter{},
			expectedCount: 0,
		},
		{
			name: "list all",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", WorkflowName: "a", Status: "success", CompletedAt: time.Now()},
				{ID: "2", WorkflowName: "b", Status: "success", CompletedAt: time.Now()},
				{ID: "3", WorkflowName: "c", Status: "success", CompletedAt: time.Now()},
			},
			filter:        &workflow.HistoryFilter{Limit: 100},
			expectedCount: 3,
		},
		{
			name: "filter by workflow name",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", WorkflowName: "deploy", Status: "success", CompletedAt: time.Now()},
				{ID: "2", WorkflowName: "build", Status: "success", CompletedAt: time.Now()},
				{ID: "3", WorkflowName: "deploy", Status: "success", CompletedAt: time.Now()},
			},
			filter:        &workflow.HistoryFilter{WorkflowName: "deploy", Limit: 100},
			expectedCount: 2,
		},
		{
			name: "filter by status",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", WorkflowName: "test", Status: "success", CompletedAt: time.Now()},
				{ID: "2", WorkflowName: "test", Status: "failed", CompletedAt: time.Now()},
				{ID: "3", WorkflowName: "test", Status: "success", CompletedAt: time.Now()},
			},
			filter:        &workflow.HistoryFilter{Status: "failed", Limit: 100},
			expectedCount: 1,
		},
		{
			name: "default limit applied when zero",
			setupRecords: func() []*workflow.ExecutionRecord {
				records := make([]*workflow.ExecutionRecord, 50)
				for i := 0; i < 50; i++ {
					records[i] = &workflow.ExecutionRecord{
						ID:           string(rune(i)),
						WorkflowName: "test",
						Status:       "success",
						CompletedAt:  time.Now(),
					}
				}
				return records
			}(),
			filter:        &workflow.HistoryFilter{Limit: 0}, // should default to 20
			expectedCount: 20,
		},
		{
			name: "store error",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", Status: "success", CompletedAt: time.Now()},
			},
			filter:   &workflow.HistoryFilter{},
			storeErr: errors.New("database error"),
			wantErr:  true,
		},
		{
			name: "nil filter uses defaults",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", WorkflowName: "test", Status: "success", CompletedAt: time.Now()},
			},
			filter:        nil,
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockHistoryStore()
			store.records = tc.setupRecords
			store.listErr = tc.storeErr
			logger := &mockHistoryLogger{}

			svc := application.NewHistoryService(store, logger)
			ctx := context.Background()

			records, err := svc.List(ctx, tc.filter)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)
		})
	}
}

func TestHistoryService_List_AppliesDefaultLimit(t *testing.T) {
	store := newMockHistoryStore()
	// Add 30 records
	for i := 0; i < 30; i++ {
		store.records = append(store.records, &workflow.ExecutionRecord{
			ID:          string(rune('a' + i)),
			Status:      "success",
			CompletedAt: time.Now(),
		})
	}
	logger := &mockHistoryLogger{}

	svc := application.NewHistoryService(store, logger)
	ctx := context.Background()

	// Filter with Limit=0 should apply default of 20
	filter := &workflow.HistoryFilter{Limit: 0}
	records, err := svc.List(ctx, filter)

	require.NoError(t, err)
	assert.Len(t, records, 20, "should apply default limit of 20 when Limit is 0")
}

func TestHistoryService_GetStats(t *testing.T) {
	tests := []struct {
		name           string
		setupRecords   []*workflow.ExecutionRecord
		filter         *workflow.HistoryFilter
		storeErr       error
		expectedTotal  int
		expectedSuccess int
		expectedFailed int
		wantErr        bool
	}{
		{
			name:          "stats for empty store",
			setupRecords:  nil,
			filter:        &workflow.HistoryFilter{},
			expectedTotal: 0,
		},
		{
			name: "stats with mixed statuses",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", Status: "success", DurationMs: 1000, CompletedAt: time.Now()},
				{ID: "2", Status: "success", DurationMs: 2000, CompletedAt: time.Now()},
				{ID: "3", Status: "failed", DurationMs: 500, CompletedAt: time.Now()},
				{ID: "4", Status: "cancelled", DurationMs: 250, CompletedAt: time.Now()},
			},
			filter:          &workflow.HistoryFilter{Limit: 100},
			expectedTotal:   4,
			expectedSuccess: 2,
			expectedFailed:  1,
		},
		{
			name: "stats filtered by workflow",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "1", WorkflowName: "deploy", Status: "success", DurationMs: 1000, CompletedAt: time.Now()},
				{ID: "2", WorkflowName: "build", Status: "success", DurationMs: 2000, CompletedAt: time.Now()},
				{ID: "3", WorkflowName: "deploy", Status: "failed", DurationMs: 500, CompletedAt: time.Now()},
			},
			filter:          &workflow.HistoryFilter{WorkflowName: "deploy", Limit: 100},
			expectedTotal:   2,
			expectedSuccess: 1,
			expectedFailed:  1,
		},
		{
			name:     "store error",
			storeErr: errors.New("database error"),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockHistoryStore()
			store.records = tc.setupRecords
			store.statsErr = tc.storeErr
			logger := &mockHistoryLogger{}

			svc := application.NewHistoryService(store, logger)
			ctx := context.Background()

			stats, err := svc.GetStats(ctx, tc.filter)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedTotal, stats.TotalExecutions)
			assert.Equal(t, tc.expectedSuccess, stats.SuccessCount)
			assert.Equal(t, tc.expectedFailed, stats.FailedCount)
		})
	}
}

func TestHistoryService_Cleanup(t *testing.T) {
	tests := []struct {
		name           string
		setupRecords   []*workflow.ExecutionRecord
		storeErr       error
		expectedDeleted int
		wantErr        bool
	}{
		{
			name: "cleanup old records",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "old1", CompletedAt: time.Now().Add(-40 * 24 * time.Hour)},
				{ID: "old2", CompletedAt: time.Now().Add(-35 * 24 * time.Hour)},
				{ID: "new1", CompletedAt: time.Now().Add(-1 * time.Hour)},
			},
			expectedDeleted: 2,
		},
		{
			name: "cleanup with no old records",
			setupRecords: []*workflow.ExecutionRecord{
				{ID: "new1", CompletedAt: time.Now().Add(-1 * time.Hour)},
				{ID: "new2", CompletedAt: time.Now()},
			},
			expectedDeleted: 0,
		},
		{
			name:           "cleanup empty store",
			setupRecords:   nil,
			expectedDeleted: 0,
		},
		{
			name:     "store error",
			storeErr: errors.New("database error"),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockHistoryStore()
			store.records = tc.setupRecords
			store.cleanupErr = tc.storeErr
			logger := &mockHistoryLogger{}

			svc := application.NewHistoryService(store, logger)
			ctx := context.Background()

			deleted, err := svc.Cleanup(ctx)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDeleted, deleted)

			// Verify cleanup was called with 30 days
			assert.Equal(t, 30*24*time.Hour, store.cleanupAge)
		})
	}
}

func TestHistoryService_Cleanup_Uses30DayRetention(t *testing.T) {
	store := newMockHistoryStore()
	logger := &mockHistoryLogger{}

	svc := application.NewHistoryService(store, logger)
	ctx := context.Background()

	_, err := svc.Cleanup(ctx)
	require.NoError(t, err)

	// Verify the cleanup was called with exactly 30 days
	expectedDuration := 30 * 24 * time.Hour
	assert.Equal(t, expectedDuration, store.cleanupAge,
		"Cleanup should use 30-day retention period")
}

func TestHistoryService_Close(t *testing.T) {
	tests := []struct {
		name     string
		closeErr error
		wantErr  bool
	}{
		{
			name:    "close success",
			wantErr: false,
		},
		{
			name:     "close error",
			closeErr: errors.New("close failed"),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMockHistoryStore()
			store.closeErr = tc.closeErr
			logger := &mockHistoryLogger{}

			svc := application.NewHistoryService(store, logger)
			err := svc.Close()

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestHistoryService_List_FilterDateRange(t *testing.T) {
	now := time.Now()
	store := newMockHistoryStore()
	store.records = []*workflow.ExecutionRecord{
		{ID: "1", WorkflowName: "test", Status: "success", CompletedAt: now.Add(-7 * 24 * time.Hour)},
		{ID: "2", WorkflowName: "test", Status: "success", CompletedAt: now.Add(-3 * 24 * time.Hour)},
		{ID: "3", WorkflowName: "test", Status: "success", CompletedAt: now.Add(-1 * 24 * time.Hour)},
		{ID: "4", WorkflowName: "test", Status: "success", CompletedAt: now.Add(-1 * time.Hour)},
	}
	logger := &mockHistoryLogger{}

	svc := application.NewHistoryService(store, logger)
	ctx := context.Background()

	tests := []struct {
		name          string
		since         time.Time
		until         time.Time
		expectedCount int
	}{
		{
			name:          "since 2 days ago",
			since:         now.Add(-2 * 24 * time.Hour),
			expectedCount: 2,
		},
		{
			name:          "until 2 days ago",
			until:         now.Add(-2 * 24 * time.Hour),
			expectedCount: 2,
		},
		{
			name:          "between 5 and 2 days ago",
			since:         now.Add(-5 * 24 * time.Hour),
			until:         now.Add(-2 * 24 * time.Hour),
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := &workflow.HistoryFilter{
				Since: tc.since,
				Until: tc.until,
				Limit: 100,
			}
			records, err := svc.List(ctx, filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)
		})
	}
}

func TestHistoryService_GetStats_CalculatesAverage(t *testing.T) {
	store := newMockHistoryStore()
	store.records = []*workflow.ExecutionRecord{
		{ID: "1", Status: "success", DurationMs: 1000, CompletedAt: time.Now()},
		{ID: "2", Status: "success", DurationMs: 2000, CompletedAt: time.Now()},
		{ID: "3", Status: "failed", DurationMs: 3000, CompletedAt: time.Now()},
	}
	logger := &mockHistoryLogger{}

	svc := application.NewHistoryService(store, logger)
	ctx := context.Background()

	stats, err := svc.GetStats(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)

	// Average: (1000 + 2000 + 3000) / 3 = 2000
	assert.Equal(t, int64(2000), stats.AvgDurationMs)
	assert.Equal(t, 3, stats.TotalExecutions)
	assert.Equal(t, 2, stats.SuccessCount)
	assert.Equal(t, 1, stats.FailedCount)
}
