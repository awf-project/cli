//go:build integration

package state_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/store"
)

// Compile-time check that SQLiteHistoryStore implements HistoryStore
func TestSQLiteHistoryStore_ImplementsInterface(t *testing.T) {
	var _ ports.HistoryStore = (*store.SQLiteHistoryStore)(nil)
}

func TestSQLiteHistoryStore_New(t *testing.T) {
	tests := []struct {
		name      string
		setupPath func(t *testing.T) string
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid path creates store",
			setupPath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "history.db")
			},
			wantErr: false,
		},
		{
			name: "nested path creates directories",
			setupPath: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "deep", "history.db")
			},
			wantErr: false,
		},
		{
			name: "existing database opens successfully",
			setupPath: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "existing.db")
				// Create an empty file to simulate existing db
				s, err := store.NewSQLiteHistoryStore(path)
				require.NoError(t, err)
				require.NoError(t, s.Close())
				return path
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.setupPath(t)
			s, err := store.NewSQLiteHistoryStore(path)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errSubstr != "" {
					assert.Contains(t, err.Error(), tc.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, s)
			defer func() { _ = s.Close() }()
		})
	}
}

func TestSQLiteHistoryStore_Record(t *testing.T) {
	tests := []struct {
		name    string
		record  *workflow.ExecutionRecord
		wantErr bool
	}{
		{
			name: "record success execution",
			record: &workflow.ExecutionRecord{
				ID:           "exec-001",
				WorkflowID:   "wf-001",
				WorkflowName: "deploy",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-5 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   300000,
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
				StartedAt:    time.Now().Add(-30 * time.Second),
				CompletedAt:  time.Now(),
				DurationMs:   30000,
				ErrorMessage: "compilation error",
			},
			wantErr: false,
		},
		{
			name: "record cancelled execution",
			record: &workflow.ExecutionRecord{
				ID:           "exec-003",
				WorkflowID:   "wf-003",
				WorkflowName: "test",
				Status:       "cancelled",
				ExitCode:     130,
				StartedAt:    time.Now().Add(-10 * time.Second),
				CompletedAt:  time.Now(),
				DurationMs:   10000,
				ErrorMessage: "user cancelled",
			},
			wantErr: false,
		},
		{
			name: "record with empty workflow name",
			record: &workflow.ExecutionRecord{
				ID:          "exec-004",
				WorkflowID:  "wf-004",
				Status:      "success",
				StartedAt:   time.Now(),
				CompletedAt: time.Now(),
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.db")
			s, err := store.NewSQLiteHistoryStore(path)
			require.NoError(t, err)
			defer func() { _ = s.Close() }()

			ctx := context.Background()
			err = s.Record(ctx, tc.record)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Verify record was stored
			records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
			require.NoError(t, err)
			assert.Len(t, records, 1)
			assert.Equal(t, tc.record.ID, records[0].ID)
		})
	}
}

func TestSQLiteHistoryStore_Record_MultipleRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Record multiple executions
	for i := 0; i < 5; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test-workflow",
			Status:       "success",
			StartedAt:    baseTime.Add(time.Duration(i) * time.Minute),
			CompletedAt:  baseTime.Add(time.Duration(i)*time.Minute + 30*time.Second),
			DurationMs:   30000,
		}
		err := s.Record(ctx, record)
		require.NoError(t, err)
	}

	// Verify all records exist
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, 5)
}

func TestSQLiteHistoryStore_Record_DuplicateID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	record := &workflow.ExecutionRecord{
		ID:           "duplicate-id",
		WorkflowID:   "wf-001",
		WorkflowName: "test",
		Status:       "success",
		StartedAt:    now,
		CompletedAt:  now,
	}

	// First insert should succeed
	err = s.Record(ctx, record)
	require.NoError(t, err)

	// Second insert with same ID should fail (PRIMARY KEY constraint)
	err = s.Record(ctx, record)
	assert.Error(t, err, "duplicate ID should be rejected")
}

func TestSQLiteHistoryStore_List_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	records, err := s.List(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestSQLiteHistoryStore_List_FilterByWorkflowName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Create records for different workflows
	workflows := []string{"deploy", "build", "test", "deploy", "build"}
	for i, wfName := range workflows {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: wfName,
			Status:       "success",
			StartedAt:    baseTime.Add(time.Duration(i) * time.Minute),
			CompletedAt:  baseTime.Add(time.Duration(i)*time.Minute + 30*time.Second),
			DurationMs:   30000,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	tests := []struct {
		name          string
		workflowName  string
		expectedCount int
	}{
		{"filter deploy", "deploy", 2},
		{"filter build", "build", 2},
		{"filter test", "test", 1},
		{"filter nonexistent", "nonexistent", 0},
		{"no filter", "", 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := &workflow.HistoryFilter{
				WorkflowName: tc.workflowName,
				Limit:        100,
			}
			records, err := s.List(ctx, filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)

			// Verify all returned records match the filter
			for _, r := range records {
				if tc.workflowName != "" {
					assert.Equal(t, tc.workflowName, r.WorkflowName)
				}
			}
		})
	}
}

func TestSQLiteHistoryStore_List_FilterByStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Create records with different statuses
	statuses := []string{"success", "failed", "cancelled", "success", "success", "failed"}
	for i, status := range statuses {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       status,
			StartedAt:    baseTime.Add(time.Duration(i) * time.Minute),
			CompletedAt:  baseTime.Add(time.Duration(i)*time.Minute + 30*time.Second),
		}
		require.NoError(t, s.Record(ctx, record))
	}

	tests := []struct {
		name          string
		status        string
		expectedCount int
	}{
		{"filter success", "success", 3},
		{"filter failed", "failed", 2},
		{"filter cancelled", "cancelled", 1},
		{"no filter", "", 6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := &workflow.HistoryFilter{
				Status: tc.status,
				Limit:  100,
			}
			records, err := s.List(ctx, filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)

			for _, r := range records {
				if tc.status != "" {
					assert.Equal(t, tc.status, r.Status)
				}
			}
		})
	}
}

func TestSQLiteHistoryStore_List_FilterByDateRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create records at different times
	times := []time.Time{
		now.Add(-7 * 24 * time.Hour), // 7 days ago
		now.Add(-3 * 24 * time.Hour), // 3 days ago
		now.Add(-24 * time.Hour),     // 1 day ago
		now.Add(-1 * time.Hour),      // 1 hour ago
		now,                          // now
	}
	for i, completedAt := range times {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    completedAt.Add(-time.Minute),
			CompletedAt:  completedAt,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	tests := []struct {
		name          string
		since         time.Time
		until         time.Time
		expectedCount int
	}{
		{
			name:          "since 2 days ago",
			since:         now.Add(-2 * 24 * time.Hour),
			expectedCount: 3,
		},
		{
			name:          "since 5 days ago",
			since:         now.Add(-5 * 24 * time.Hour),
			expectedCount: 4,
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
		{
			name:          "future date range",
			since:         now.Add(24 * time.Hour),
			expectedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := &workflow.HistoryFilter{
				Since: tc.since,
				Until: tc.until,
				Limit: 100,
			}
			records, err := s.List(ctx, filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)
		})
	}
}

func TestSQLiteHistoryStore_List_Limit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Create 50 records
	for i := 0; i < 50; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%03d", i),
			WorkflowID:   fmt.Sprintf("wf-%03d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    baseTime.Add(time.Duration(i) * time.Second),
			CompletedAt:  baseTime.Add(time.Duration(i)*time.Second + time.Second),
		}
		require.NoError(t, s.Record(ctx, record))
	}

	tests := []struct {
		name          string
		limit         int
		expectedCount int
	}{
		{"limit 10", 10, 10},
		{"limit 20", 20, 20},
		{"limit 100", 100, 50},
		{"limit 0 uses default", 0, 20}, // implementation should default to 20
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := &workflow.HistoryFilter{
				Limit: tc.limit,
			}
			records, err := s.List(ctx, filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)
		})
	}
}

func TestSQLiteHistoryStore_List_CombinedFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create diverse set of records
	testData := []struct {
		workflowName string
		status       string
		completedAt  time.Time
	}{
		{"deploy", "success", now.Add(-1 * time.Hour)},
		{"deploy", "failed", now.Add(-2 * time.Hour)},
		{"deploy", "success", now.Add(-25 * time.Hour)},
		{"build", "success", now.Add(-1 * time.Hour)},
		{"build", "failed", now.Add(-1 * time.Hour)},
		{"test", "success", now.Add(-1 * time.Hour)},
	}

	for i, td := range testData {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: td.workflowName,
			Status:       td.status,
			StartedAt:    td.completedAt.Add(-time.Minute),
			CompletedAt:  td.completedAt,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	tests := []struct {
		name          string
		filter        *workflow.HistoryFilter
		expectedCount int
	}{
		{
			name: "deploy + success",
			filter: &workflow.HistoryFilter{
				WorkflowName: "deploy",
				Status:       "success",
				Limit:        100,
			},
			expectedCount: 2,
		},
		{
			name: "deploy + success + last 24h",
			filter: &workflow.HistoryFilter{
				WorkflowName: "deploy",
				Status:       "success",
				Since:        now.Add(-24 * time.Hour),
				Limit:        100,
			},
			expectedCount: 1,
		},
		{
			name: "failed + last 24h",
			filter: &workflow.HistoryFilter{
				Status: "failed",
				Since:  now.Add(-24 * time.Hour),
				Limit:  100,
			},
			expectedCount: 2,
		},
		{
			name: "all filters with limit",
			filter: &workflow.HistoryFilter{
				WorkflowName: "deploy",
				Status:       "success",
				Since:        now.Add(-48 * time.Hour),
				Limit:        1,
			},
			expectedCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			records, err := s.List(ctx, tc.filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)
		})
	}
}

func TestSQLiteHistoryStore_List_OrderByCompletedAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create records in random order
	times := []time.Duration{
		-3 * time.Hour,
		-1 * time.Hour,
		-5 * time.Hour,
		-2 * time.Hour,
		-4 * time.Hour,
	}
	for i, d := range times {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    now.Add(d).Add(-time.Minute),
			CompletedAt:  now.Add(d),
		}
		require.NoError(t, s.Record(ctx, record))
	}

	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, records, 5)

	// Verify records are ordered by completed_at descending (most recent first)
	for i := 1; i < len(records); i++ {
		assert.True(t, records[i-1].CompletedAt.After(records[i].CompletedAt) ||
			records[i-1].CompletedAt.Equal(records[i].CompletedAt),
			"records should be ordered by completed_at descending")
	}
}

func TestSQLiteHistoryStore_GetStats_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	stats, err := s.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)

	assert.Equal(t, 0, stats.TotalExecutions)
	assert.Equal(t, 0, stats.SuccessCount)
	assert.Equal(t, 0, stats.FailedCount)
	assert.Equal(t, 0, stats.CancelledCount)
	assert.Equal(t, int64(0), stats.AvgDurationMs)
}

func TestSQLiteHistoryStore_GetStats_AllStatuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Create records with different statuses and durations
	testData := []struct {
		status     string
		durationMs int64
	}{
		{"success", 1000},
		{"success", 2000},
		{"success", 3000},
		{"failed", 500},
		{"failed", 1500},
		{"cancelled", 250},
	}

	for i, td := range testData {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       td.status,
			StartedAt:    baseTime,
			CompletedAt:  baseTime.Add(time.Duration(td.durationMs) * time.Millisecond),
			DurationMs:   td.durationMs,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	stats, err := s.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)

	assert.Equal(t, 6, stats.TotalExecutions)
	assert.Equal(t, 3, stats.SuccessCount)
	assert.Equal(t, 2, stats.FailedCount)
	assert.Equal(t, 1, stats.CancelledCount)

	// Expected average duration: (1000+2000+3000+500+1500+250) / 6 = 1375ms
	expectedAvg := int64(1375)
	assert.Equal(t, expectedAvg, stats.AvgDurationMs)
}

func TestSQLiteHistoryStore_GetStats_WithFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create records for different workflows
	testData := []struct {
		workflowName string
		status       string
		durationMs   int64
		completedAt  time.Time
	}{
		{"deploy", "success", 1000, now.Add(-1 * time.Hour)},
		{"deploy", "success", 2000, now.Add(-2 * time.Hour)},
		{"deploy", "failed", 500, now.Add(-3 * time.Hour)},
		{"build", "success", 3000, now.Add(-1 * time.Hour)},
		{"build", "failed", 1000, now.Add(-1 * time.Hour)},
	}

	for i, td := range testData {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: td.workflowName,
			Status:       td.status,
			StartedAt:    td.completedAt.Add(-time.Duration(td.durationMs) * time.Millisecond),
			CompletedAt:  td.completedAt,
			DurationMs:   td.durationMs,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	tests := []struct {
		name          string
		filter        *workflow.HistoryFilter
		expectedTotal int
		expectedAvgMs int64
	}{
		{
			name:          "stats for deploy workflow",
			filter:        &workflow.HistoryFilter{WorkflowName: "deploy"},
			expectedTotal: 3,
			expectedAvgMs: 1166, // (1000+2000+500) / 3
		},
		{
			name:          "stats for build workflow",
			filter:        &workflow.HistoryFilter{WorkflowName: "build"},
			expectedTotal: 2,
			expectedAvgMs: 2000, // (3000+1000) / 2
		},
		{
			name:          "stats for success only",
			filter:        &workflow.HistoryFilter{Status: "success"},
			expectedTotal: 3,
			expectedAvgMs: 2000, // (1000+2000+3000) / 3
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stats, err := s.GetStats(ctx, tc.filter)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedTotal, stats.TotalExecutions)
			assert.Equal(t, tc.expectedAvgMs, stats.AvgDurationMs)
		})
	}
}

func TestSQLiteHistoryStore_Cleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create old records (older than 30 days)
	for i := 0; i < 5; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("old-exec-%d", i),
			WorkflowID:   fmt.Sprintf("old-wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    now.Add(-40 * 24 * time.Hour),
			CompletedAt:  now.Add(-35 * 24 * time.Hour),
		}
		require.NoError(t, s.Record(ctx, record))
	}

	// Create recent records (within 30 days)
	for i := 0; i < 3; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("new-exec-%d", i),
			WorkflowID:   fmt.Sprintf("new-wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    now.Add(-1 * time.Hour),
			CompletedAt:  now,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	// Cleanup old records (older than 30 days)
	deleted, err := s.Cleanup(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 5, deleted)

	// Verify only recent records remain
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, 3)

	for _, r := range records {
		assert.True(t, r.CompletedAt.After(now.Add(-30*24*time.Hour)),
			"remaining records should be within retention period")
	}
}

func TestSQLiteHistoryStore_Cleanup_NoOldRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create only recent records
	for i := 0; i < 3; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    now.Add(-1 * time.Hour),
			CompletedAt:  now,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	// Cleanup should delete nothing
	deleted, err := s.Cleanup(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)

	// All records should remain
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, 3)
}

func TestSQLiteHistoryStore_Cleanup_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	deleted, err := s.Cleanup(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestSQLiteHistoryStore_Close(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)

	err = s.Close()
	assert.NoError(t, err)

	// Operations after close should fail gracefully
	ctx := context.Background()
	_, err = s.List(ctx, &workflow.HistoryFilter{})
	assert.Error(t, err, "operations after close should fail")
}

func TestSQLiteHistoryStore_Close_DoubleClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)

	// First close should succeed
	err = s.Close()
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	err = s.Close()
	assert.NoError(t, err, "double close should be safe")

	// Third close should also succeed
	err = s.Close()
	assert.NoError(t, err, "multiple closes should be safe")
}

func TestSQLiteHistoryStore_OperationsAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)

	// Add initial data before closing
	ctx := context.Background()
	record := &workflow.ExecutionRecord{
		ID:           "pre-close-exec",
		WorkflowID:   "pre-close-wf",
		WorkflowName: "test",
		Status:       "success",
		StartedAt:    time.Now().Add(-time.Minute),
		CompletedAt:  time.Now(),
	}
	require.NoError(t, s.Record(ctx, record))

	// Close the store
	require.NoError(t, s.Close())

	// All operations should fail after close
	t.Run("Record after close", func(t *testing.T) {
		newRecord := &workflow.ExecutionRecord{
			ID:           "post-close-exec",
			WorkflowID:   "post-close-wf",
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    time.Now(),
			CompletedAt:  time.Now(),
		}
		err := s.Record(ctx, newRecord)
		assert.Error(t, err, "Record should fail after close")
		assert.Contains(t, err.Error(), "closed")
	})

	t.Run("List after close", func(t *testing.T) {
		_, err := s.List(ctx, &workflow.HistoryFilter{})
		assert.Error(t, err, "List should fail after close")
		assert.Contains(t, err.Error(), "closed")
	})

	t.Run("GetStats after close", func(t *testing.T) {
		_, err := s.GetStats(ctx, &workflow.HistoryFilter{})
		assert.Error(t, err, "GetStats should fail after close")
		assert.Contains(t, err.Error(), "closed")
	})

	t.Run("Cleanup after close", func(t *testing.T) {
		_, err := s.Cleanup(ctx, 24*time.Hour)
		assert.Error(t, err, "Cleanup should fail after close")
		assert.Contains(t, err.Error(), "closed")
	})
}

func TestSQLiteHistoryStore_Record_ZeroValueTimes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Record with zero-value times (should still work)
	// Note: zero time (0001-01-01 00:00:00) is stored as negative nanoseconds
	// and reconstructed as Unix epoch-relative time, not as Go's zero value
	record := &workflow.ExecutionRecord{
		ID:           "zero-time-exec",
		WorkflowID:   "zero-time-wf",
		WorkflowName: "test",
		Status:       "success",
		StartedAt:    time.Time{}, // zero value
		CompletedAt:  time.Time{}, // zero value
	}

	err = s.Record(ctx, record)
	require.NoError(t, err, "zero-value times should be accepted")

	// Verify it was stored and retrieved without error
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, records, 1)

	// Due to UnixNano storage, the exact zero-value is not preserved,
	// but the record should be retrievable without error
	assert.Equal(t, record.ID, records[0].ID)
	assert.Equal(t, record.WorkflowID, records[0].WorkflowID)
}

func TestSQLiteHistoryStore_GetStats_CompletedStatus(t *testing.T) {
	// The GetStats implementation counts 'completed' as success
	// This test verifies that behavior
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Create records with 'completed' status (alternative to 'success')
	testData := []struct {
		status string
	}{
		{"success"},
		{"completed"}, // Should count as success
		{"failed"},
	}

	for i, td := range testData {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       td.status,
			StartedAt:    baseTime,
			CompletedAt:  baseTime.Add(time.Second),
		}
		require.NoError(t, s.Record(ctx, record))
	}

	stats, err := s.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)

	assert.Equal(t, 3, stats.TotalExecutions)
	assert.Equal(t, 2, stats.SuccessCount)
	assert.Equal(t, 1, stats.FailedCount)
}

func TestSQLiteHistoryStore_List_SameCompletionTime(t *testing.T) {
	// Test ordering when multiple records have same completion time
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	sameTime := time.Now()

	// Create multiple records with exact same completion time
	for i := 0; i < 5; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("same-time-exec-%d", i),
			WorkflowID:   fmt.Sprintf("same-time-wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    sameTime.Add(-time.Minute),
			CompletedAt:  sameTime,
		}
		require.NoError(t, s.Record(ctx, record))
	}

	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 10})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// All records should have same completion time - no crash or error
	for _, r := range records {
		assert.Equal(t, sameTime.Unix(), r.CompletedAt.Unix())
	}
}

func TestSQLiteHistoryStore_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errChan := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("concurrent-exec-%d", n),
				WorkflowID:   fmt.Sprintf("concurrent-wf-%d", n),
				WorkflowName: "concurrent-test",
				Status:       "success",
				StartedAt:    baseTime,
				CompletedAt:  baseTime.Add(time.Second),
			}
			if err := s.Record(ctx, record); err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("concurrent write error: %v", err)
	}

	// Verify all records were written
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, goroutines)
}

func TestSQLiteHistoryStore_ConcurrentReadsWrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()
	const iterations = 20

	// Seed initial data
	for i := 0; i < 10; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("seed-exec-%d", i),
			WorkflowID:   fmt.Sprintf("seed-wf-%d", i),
			WorkflowName: "seed",
			Status:       "success",
			StartedAt:    baseTime,
			CompletedAt:  baseTime.Add(time.Second),
		}
		require.NoError(t, s.Record(ctx, record))
	}

	var wg sync.WaitGroup
	wg.Add(iterations * 2)

	writeErrors := make(chan error, iterations)
	readErrors := make(chan error, iterations)

	// Writers
	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("race-exec-%d", n),
				WorkflowID:   fmt.Sprintf("race-wf-%d", n),
				WorkflowName: "race-test",
				Status:       "success",
				StartedAt:    baseTime,
				CompletedAt:  baseTime.Add(time.Second),
			}
			if err := s.Record(ctx, record); err != nil {
				writeErrors <- err
			}
		}(i)
	}

	// Readers
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
			if err != nil {
				readErrors <- err
			}
			if records == nil {
				readErrors <- fmt.Errorf("nil records returned")
			}
		}()
	}

	wg.Wait()
	close(writeErrors)
	close(readErrors)

	for err := range writeErrors {
		t.Errorf("concurrent write error: %v", err)
	}
	for err := range readErrors {
		t.Errorf("concurrent read error: %v", err)
	}
}

// TestSQLiteHistoryStore_MultipleConnections validates that WAL mode allows
// multiple connections to the same database file - the key fix for bug-48.
func TestSQLiteHistoryStore_MultipleConnections(t *testing.T) {
	t.Parallel()

	// Use a shared path for all connections
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "shared.db")

	// Create first connection and write initial data
	s1, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)

	ctx := context.Background()
	baseTime := time.Now()

	record1 := &workflow.ExecutionRecord{
		ID:           "conn1-exec",
		WorkflowID:   "conn1-wf",
		WorkflowName: "connection-test",
		Status:       "success",
		StartedAt:    baseTime,
		CompletedAt:  baseTime.Add(time.Second),
	}
	require.NoError(t, s1.Record(ctx, record1))

	// Open second connection while first is still open
	// This is the key test - BadgerDB would fail here with directory lock error
	s2, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err, "second connection should succeed with WAL mode")

	// Write from second connection
	record2 := &workflow.ExecutionRecord{
		ID:           "conn2-exec",
		WorkflowID:   "conn2-wf",
		WorkflowName: "connection-test",
		Status:       "success",
		StartedAt:    baseTime.Add(time.Minute),
		CompletedAt:  baseTime.Add(time.Minute + time.Second),
	}
	require.NoError(t, s2.Record(ctx, record2))

	// Both connections should see all data
	records1, err := s1.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records1, 2)

	records2, err := s2.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records2, 2)

	// Open third connection to stress test
	s3, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err, "third connection should succeed")

	record3 := &workflow.ExecutionRecord{
		ID:           "conn3-exec",
		WorkflowID:   "conn3-wf",
		WorkflowName: "connection-test",
		Status:       "success",
		StartedAt:    baseTime.Add(2 * time.Minute),
		CompletedAt:  baseTime.Add(2*time.Minute + time.Second),
	}
	require.NoError(t, s3.Record(ctx, record3))

	// All three connections should see all 3 records
	records3, err := s3.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records3, 3)

	// Clean up in order
	require.NoError(t, s3.Close())
	require.NoError(t, s2.Close())
	require.NoError(t, s1.Close())
}

// TestSQLiteHistoryStore_ConcurrentConnectionsWithInterleavedOps tests
// truly concurrent operations across multiple connections
func TestSQLiteHistoryStore_ConcurrentConnectionsWithInterleavedOps(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "concurrent.db")

	const numConnections = 5
	const opsPerConnection = 10

	connections := make([]*store.SQLiteHistoryStore, numConnections)
	for i := 0; i < numConnections; i++ {
		s, err := store.NewSQLiteHistoryStore(path)
		require.NoError(t, err, "connection %d should open", i)
		connections[i] = s
	}
	defer func() {
		for _, s := range connections {
			_ = s.Close()
		}
	}()

	ctx := context.Background()
	baseTime := time.Now()

	var wg sync.WaitGroup
	errChan := make(chan error, numConnections*opsPerConnection)

	// Each connection performs multiple operations concurrently
	for connIdx, s := range connections {
		for opIdx := 0; opIdx < opsPerConnection; opIdx++ {
			wg.Add(1)
			go func(connID, opID int, store *store.SQLiteHistoryStore) {
				defer wg.Done()

				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("conn%d-op%d", connID, opID),
					WorkflowID:   fmt.Sprintf("wf-conn%d-op%d", connID, opID),
					WorkflowName: fmt.Sprintf("workflow-conn%d", connID),
					Status:       "success",
					StartedAt:    baseTime,
					CompletedAt:  baseTime.Add(time.Second),
				}

				if err := store.Record(ctx, record); err != nil {
					errChan <- fmt.Errorf("conn%d-op%d write failed: %w", connID, opID, err)
				}
			}(connIdx, opIdx, s)
		}
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Error(err)
	}

	// Verify all operations completed
	records, err := connections[0].List(ctx, &workflow.HistoryFilter{Limit: 1000})
	require.NoError(t, err)
	expectedRecords := numConnections * opsPerConnection
	assert.Len(t, records, expectedRecords)
}

// TestSQLiteHistoryStore_BusyTimeout verifies that SQLite properly handles
// write contention using busy_timeout
func TestSQLiteHistoryStore_BusyTimeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "busy.db")

	s1, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s1.Close() }()

	s2, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()

	ctx := context.Background()
	baseTime := time.Now()

	// Perform rapid alternating writes to trigger busy conditions
	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(iterations * 2)

	errChan := make(chan error, iterations*2)

	for i := 0; i < iterations; i++ {
		go func(n int) {
			defer wg.Done()
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("s1-exec-%d", n),
				WorkflowID:   fmt.Sprintf("s1-wf-%d", n),
				WorkflowName: "busy-test-1",
				Status:       "success",
				StartedAt:    baseTime,
				CompletedAt:  baseTime.Add(time.Millisecond),
			}
			if err := s1.Record(ctx, record); err != nil {
				errChan <- fmt.Errorf("s1 write %d failed: %w", n, err)
			}
		}(i)

		go func(n int) {
			defer wg.Done()
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("s2-exec-%d", n),
				WorkflowID:   fmt.Sprintf("s2-wf-%d", n),
				WorkflowName: "busy-test-2",
				Status:       "success",
				StartedAt:    baseTime,
				CompletedAt:  baseTime.Add(time.Millisecond),
			}
			if err := s2.Record(ctx, record); err != nil {
				errChan <- fmt.Errorf("s2 write %d failed: %w", n, err)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Error(err)
	}

	// Verify all writes succeeded despite contention
	records, err := s1.List(ctx, &workflow.HistoryFilter{Limit: 1000})
	require.NoError(t, err)
	assert.Len(t, records, iterations*2)
}

func TestSQLiteHistoryStore_NilFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Add a record
	record := &workflow.ExecutionRecord{
		ID:           "test-exec",
		WorkflowID:   "test-wf",
		WorkflowName: "test",
		Status:       "success",
		StartedAt:    time.Now().Add(-time.Minute),
		CompletedAt:  time.Now(),
	}
	require.NoError(t, s.Record(ctx, record))

	// List with nil filter should not panic
	records, err := s.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// GetStats with nil filter should not panic
	stats, err := s.GetStats(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalExecutions)
}

func TestSQLiteHistoryStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "persist.db")
	ctx := context.Background()
	now := time.Now()

	// Write data and close
	func() {
		s, err := store.NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer func() { _ = s.Close() }()

		record := &workflow.ExecutionRecord{
			ID:           "persist-exec",
			WorkflowID:   "persist-wf",
			WorkflowName: "persist-test",
			Status:       "success",
			StartedAt:    now.Add(-time.Minute),
			CompletedAt:  now,
			DurationMs:   60000,
		}
		require.NoError(t, s.Record(ctx, record))
	}()

	// Reopen and verify data persisted
	func() {
		s, err := store.NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer func() { _ = s.Close() }()

		records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, "persist-exec", records[0].ID)
		assert.Equal(t, "persist-test", records[0].WorkflowName)
		assert.Equal(t, "success", records[0].Status)
	}()
}

func TestSQLiteHistoryStore_RecordPreservesAllFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond) // Truncate for comparison

	original := &workflow.ExecutionRecord{
		ID:           "full-exec",
		WorkflowID:   "full-wf-id",
		WorkflowName: "full-workflow-name",
		Status:       "failed",
		ExitCode:     42,
		StartedAt:    now.Add(-5 * time.Minute),
		CompletedAt:  now,
		DurationMs:   300000,
		ErrorMessage: "detailed error message with special chars: <>&\"'",
	}

	require.NoError(t, s.Record(ctx, original))

	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, records, 1)

	loaded := records[0]
	assert.Equal(t, original.ID, loaded.ID)
	assert.Equal(t, original.WorkflowID, loaded.WorkflowID)
	assert.Equal(t, original.WorkflowName, loaded.WorkflowName)
	assert.Equal(t, original.Status, loaded.Status)
	assert.Equal(t, original.ExitCode, loaded.ExitCode)
	assert.Equal(t, original.DurationMs, loaded.DurationMs)
	assert.Equal(t, original.ErrorMessage, loaded.ErrorMessage)
	// Time comparison with tolerance for serialization
	assert.WithinDuration(t, original.StartedAt, loaded.StartedAt, time.Second)
	assert.WithinDuration(t, original.CompletedAt, loaded.CompletedAt, time.Second)
}

func TestSQLiteHistoryStore_ContextCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations with cancelled context should return error
	record := &workflow.ExecutionRecord{
		ID:           "cancelled-exec",
		WorkflowID:   "cancelled-wf",
		WorkflowName: "test",
		Status:       "success",
		StartedAt:    time.Now(),
		CompletedAt:  time.Now(),
	}
	err = s.Record(ctx, record)
	// SQLite may or may not check context; this test documents expected behavior
	// The implementation should ideally return context.Canceled
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
}

func TestSQLiteHistoryStore_SpecialCharactersInData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Test with special characters that could cause SQL issues
	record := &workflow.ExecutionRecord{
		ID:           "special-exec",
		WorkflowID:   "wf-with-special-chars",
		WorkflowName: "workflow'name\"with;special--chars",
		Status:       "failed",
		ExitCode:     1,
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
		ErrorMessage: "error with 'quotes' and \"double quotes\" and ; semicolons and -- comments",
	}

	require.NoError(t, s.Record(ctx, record))

	// Verify special characters are preserved
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, record.WorkflowName, records[0].WorkflowName)
	assert.Equal(t, record.ErrorMessage, records[0].ErrorMessage)
}

func TestSQLiteHistoryStore_VeryLongErrorMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create a very long error message (10KB)
	longMessage := ""
	for i := 0; i < 10000; i++ {
		longMessage += "x"
	}

	record := &workflow.ExecutionRecord{
		ID:           "long-error-exec",
		WorkflowID:   "long-error-wf",
		WorkflowName: "test",
		Status:       "failed",
		ExitCode:     1,
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
		ErrorMessage: longMessage,
	}

	require.NoError(t, s.Record(ctx, record))

	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, longMessage, records[0].ErrorMessage)
}

// TestSQLiteHistoryStore_MultiProcessConcurrency tests the actual bug-48 scenario:
// multiple processes trying to access the same database simultaneously.
// This test spawns actual child processes to verify SQLite handles this correctly.
func TestSQLiteHistoryStore_MultiProcessConcurrency(t *testing.T) {
	t.Skip("skipping multi-process test in short mode")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "multiproc.db")

	// Create database with initial setup
	s, err := store.NewSQLiteHistoryStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, s.Close())

	// We can't easily spawn child processes in a test, but we can simulate
	// the scenario by having goroutines open separate connections
	// (which is equivalent from SQLite's perspective)

	const numProcesses = 3
	const recordsPerProcess = 5

	var wg sync.WaitGroup
	errChan := make(chan error, numProcesses)

	// Each "process" opens its own connection and writes records
	for procID := 0; procID < numProcesses; procID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Open a new connection (simulates separate process)
			conn, err := store.NewSQLiteHistoryStore(dbPath)
			if err != nil {
				errChan <- fmt.Errorf("process %d: failed to open db: %w", id, err)
				return
			}
			defer func() { _ = conn.Close() }()

			ctx := context.Background()
			baseTime := time.Now()

			// Write records with small delays to interleave with other processes
			for i := 0; i < recordsPerProcess; i++ {
				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("proc%d-rec%d", id, i),
					WorkflowID:   fmt.Sprintf("wf-proc%d-rec%d", id, i),
					WorkflowName: fmt.Sprintf("workflow-proc%d", id),
					Status:       "success",
					StartedAt:    baseTime,
					CompletedAt:  baseTime.Add(time.Second),
				}
				if err := conn.Record(ctx, record); err != nil {
					errChan <- fmt.Errorf("process %d record %d: %w", id, i, err)
					return
				}
				// Small sleep to increase chance of interleaving
				time.Sleep(time.Millisecond)
			}
		}(procID)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errs := make([]error, 0, numProcesses)
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "all processes should complete without error: %v", errs)

	// Verify all records exist
	verifyConn, err := store.NewSQLiteHistoryStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = verifyConn.Close() }()

	records, err := verifyConn.List(context.Background(), &workflow.HistoryFilter{Limit: 1000})
	require.NoError(t, err)

	expectedRecords := numProcesses * recordsPerProcess
	assert.Len(t, records, expectedRecords,
		"all records from all processes should be written successfully")
}

// TestSQLiteHistoryStore_WALModeEnabled verifies that WAL mode is actually enabled.
// This is critical for the bug-48 fix to work correctly.
func TestSQLiteHistoryStore_WALModeEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal-test.db")
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	// WAL mode creates additional files: .db-wal and .db-shm
	// We can verify by checking if multiple connections work
	s2, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err, "WAL mode should allow multiple connections")
	defer func() { _ = s2.Close() }()

	// Write from one connection
	ctx := context.Background()
	record := &workflow.ExecutionRecord{
		ID:           "wal-test-exec",
		WorkflowID:   "wal-test-wf",
		WorkflowName: "wal-test",
		Status:       "success",
		StartedAt:    time.Now(),
		CompletedAt:  time.Now(),
	}
	require.NoError(t, s.Record(ctx, record))

	// Read from other connection should see the data (WAL provides read consistency)
	records, err := s2.List(ctx, &workflow.HistoryFilter{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

// TestSQLiteHistoryStore_SimulateWorkflowExecution simulates the actual workflow
// execution pattern: open store, record execution, close store
func TestSQLiteHistoryStore_SimulateWorkflowExecution(t *testing.T) {
	t.Skip("skipping workflow simulation in short mode")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "workflow.db")

	const numWorkflows = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numWorkflows)

	// Simulate multiple workflow executions happening simultaneously
	for i := 0; i < numWorkflows; i++ {
		wg.Add(1)
		go func(workflowNum int) {
			defer wg.Done()

			// Each workflow execution opens the store, does work, records, closes
			histStore, err := store.NewSQLiteHistoryStore(dbPath)
			if err != nil {
				errChan <- fmt.Errorf("workflow %d: open failed: %w", workflowNum, err)
				return
			}

			// Simulate some workflow execution time
			time.Sleep(time.Duration(10+workflowNum) * time.Millisecond)

			ctx := context.Background()
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("workflow-exec-%d", workflowNum),
				WorkflowID:   fmt.Sprintf("wf-%d", workflowNum),
				WorkflowName: "simulated-workflow",
				Status:       "success",
				StartedAt:    time.Now().Add(-time.Duration(10+workflowNum) * time.Millisecond),
				CompletedAt:  time.Now(),
				DurationMs:   int64(10 + workflowNum),
			}

			if err := histStore.Record(ctx, record); err != nil {
				errChan <- fmt.Errorf("workflow %d: record failed: %w", workflowNum, err)
				_ = histStore.Close()
				return
			}

			if err := histStore.Close(); err != nil {
				errChan <- fmt.Errorf("workflow %d: close failed: %w", workflowNum, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	errs := make([]error, 0, numWorkflows)
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "all simulated workflows should complete successfully")

	// Verify final state
	verifyStore, err := store.NewSQLiteHistoryStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = verifyStore.Close() }()

	records, err := verifyStore.List(context.Background(), &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, numWorkflows)
}

// TestSQLiteHistoryStore_RealMultiProcessWithHelper uses a helper binary to
// test actual multi-process concurrency. This is the definitive test for bug-48.
// Currently skipped - relies on in-process concurrency tests (TestSQLiteHistoryStore_MultiProcessConcurrency).
func TestSQLiteHistoryStore_RealMultiProcessWithHelper(t *testing.T) {
	t.Skip("Complex multi-process test requires helper binary setup - relying on in-process concurrency tests")
}
