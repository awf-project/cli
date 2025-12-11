package store_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/store"
)

// Compile-time check that BadgerHistoryStore implements HistoryStore
func TestBadgerHistoryStore_ImplementsInterface(t *testing.T) {
	var _ ports.HistoryStore = (*store.BadgerHistoryStore)(nil)
}

func TestBadgerHistoryStore_New(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid path creates store",
			path:    t.TempDir(),
			wantErr: false,
		},
		{
			name:    "nested path creates directories",
			path:    t.TempDir() + "/nested/history",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, err := store.NewBadgerHistoryStore(tc.path)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errSubstr != "" {
					assert.Contains(t, err.Error(), tc.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, store)
			defer func() { _ = store.Close() }()
		})
	}
}

func TestBadgerHistoryStore_Record(t *testing.T) {
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
			wantErr: false, // empty workflow name should be allowed
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_Record_MultipleRecords(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_List_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	records, err := s.List(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestBadgerHistoryStore_List_FilterByWorkflowName(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_List_FilterByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_List_FilterByDateRange(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Create records at different times
	times := []time.Time{
		now.Add(-7 * 24 * time.Hour),  // 7 days ago
		now.Add(-3 * 24 * time.Hour),  // 3 days ago
		now.Add(-24 * time.Hour),      // 1 day ago
		now.Add(-1 * time.Hour),       // 1 hour ago
		now,                           // now
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

func TestBadgerHistoryStore_List_Limit(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_List_CombinedFilters(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_List_OrderByCompletedAt(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_GetStats_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_GetStats_AllStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

	// Average: (1000+2000+3000+500+1500+250) / 6 = 1375
	expectedAvg := int64(1375)
	assert.Equal(t, expectedAvg, stats.AvgDurationMs)
}

func TestBadgerHistoryStore_GetStats_WithFilter(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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
		name           string
		filter         *workflow.HistoryFilter
		expectedTotal  int
		expectedAvgMs  int64
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

func TestBadgerHistoryStore_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_Cleanup_NoOldRecords(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_Cleanup_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	deleted, err := s.Cleanup(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestBadgerHistoryStore_Close(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
	require.NoError(t, err)

	err = s.Close()
	assert.NoError(t, err)

	// Operations after close should fail gracefully
	ctx := context.Background()
	_, err = s.List(ctx, &workflow.HistoryFilter{})
	assert.Error(t, err, "operations after close should fail")
}

func TestBadgerHistoryStore_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	baseTime := time.Now()
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

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
			err := s.Record(ctx, record)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify all records were written
	records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, goroutines)
}

func TestBadgerHistoryStore_ConcurrentReadsWrites(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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
			_ = s.Record(ctx, record)
		}(i)
	}

	// Readers
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			records, err := s.List(ctx, &workflow.HistoryFilter{Limit: 100})
			assert.NoError(t, err)
			assert.NotNil(t, records)
		}()
	}

	wg.Wait()
}

func TestBadgerHistoryStore_NilFilter(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	now := time.Now()

	// Write data and close
	func() {
		s, err := store.NewBadgerHistoryStore(tmpDir)
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
		s, err := store.NewBadgerHistoryStore(tmpDir)
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

func TestBadgerHistoryStore_RecordPreservesAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.NewBadgerHistoryStore(tmpDir)
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
