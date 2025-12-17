//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/pkg/interpolation"
)

// historyMockStateStore for history integration tests
type historyMockStateStore struct {
	states map[string]*workflow.ExecutionContext
}

func newHistoryMockStateStore() *historyMockStateStore {
	return &historyMockStateStore{states: make(map[string]*workflow.ExecutionContext)}
}

func (m *historyMockStateStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	m.states[state.WorkflowID] = state
	return nil
}

func (m *historyMockStateStore) Load(ctx context.Context, id string) (*workflow.ExecutionContext, error) {
	if state, ok := m.states[id]; ok {
		return state, nil
	}
	return nil, nil
}

func (m *historyMockStateStore) Delete(ctx context.Context, id string) error {
	delete(m.states, id)
	return nil
}

func (m *historyMockStateStore) List(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.states))
	for id := range m.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// historyMockLogger for history integration tests
type historyMockLogger struct{}

func (m *historyMockLogger) Debug(msg string, fields ...any) {}
func (m *historyMockLogger) Info(msg string, fields ...any)  {}
func (m *historyMockLogger) Warn(msg string, fields ...any)  {}
func (m *historyMockLogger) Error(msg string, fields ...any) {}
func (m *historyMockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

func TestHistoryStore_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	// Create and use history store
	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Record several executions
	for i := 0; i < 5; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test-workflow",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    now.Add(-time.Duration(i) * time.Minute),
			CompletedAt:  now.Add(-time.Duration(i)*time.Minute + 30*time.Second),
			DurationMs:   30000,
		}
		err := historyStore.Record(ctx, record)
		require.NoError(t, err)
	}

	// List and verify
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// Get stats
	stats, err := historyStore.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Equal(t, 5, stats.TotalExecutions)
	assert.Equal(t, 5, stats.SuccessCount)
}

func TestHistoryStore_FilterByWorkflowName_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Record executions for different workflows
	workflows := []string{"deploy", "build", "test", "deploy", "build"}
	for i, wfName := range workflows {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: wfName,
			Status:       "success",
			StartedAt:    now.Add(-time.Duration(i) * time.Minute),
			CompletedAt:  now.Add(-time.Duration(i)*time.Minute + 30*time.Second),
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	// Filter by workflow name
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{
		WorkflowName: "deploy",
		Limit:        100,
	})
	require.NoError(t, err)
	assert.Len(t, records, 2)

	for _, r := range records {
		assert.Equal(t, "deploy", r.WorkflowName)
	}
}

func TestHistoryStore_FilterByStatus_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Record executions with different statuses
	statuses := []string{"success", "failed", "cancelled", "success", "success", "failed"}
	for i, status := range statuses {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       status,
			StartedAt:    now.Add(-time.Duration(i) * time.Minute),
			CompletedAt:  now.Add(-time.Duration(i)*time.Minute + 30*time.Second),
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	// Filter by status
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{
		Status: "failed",
		Limit:  100,
	})
	require.NoError(t, err)
	assert.Len(t, records, 2)

	for _, r := range records {
		assert.Equal(t, "failed", r.Status)
	}

	// Get stats
	stats, err := historyStore.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Equal(t, 6, stats.TotalExecutions)
	assert.Equal(t, 3, stats.SuccessCount)
	assert.Equal(t, 2, stats.FailedCount)
	assert.Equal(t, 1, stats.CancelledCount)
}

func TestHistoryStore_FilterByDateRange_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Record executions at different times
	times := []time.Duration{
		-7 * 24 * time.Hour, // 7 days ago
		-3 * 24 * time.Hour, // 3 days ago
		-24 * time.Hour,     // 1 day ago
		-1 * time.Hour,      // 1 hour ago
		0,                   // now
	}
	for i, offset := range times {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: "test",
			Status:       "success",
			StartedAt:    now.Add(offset).Add(-time.Minute),
			CompletedAt:  now.Add(offset),
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	// Filter since 2 days ago
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{
		Since: now.Add(-2 * 24 * time.Hour),
		Limit: 100,
	})
	require.NoError(t, err)
	assert.Len(t, records, 3)

	for _, r := range records {
		assert.True(t, r.CompletedAt.After(now.Add(-2*24*time.Hour)))
	}
}

func TestHistoryStore_Cleanup_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Record old executions (older than 30 days)
	for i := 0; i < 3; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("old-exec-%d", i),
			WorkflowID:   fmt.Sprintf("old-wf-%d", i),
			WorkflowName: "old-workflow",
			Status:       "success",
			StartedAt:    now.Add(-40 * 24 * time.Hour),
			CompletedAt:  now.Add(-35 * 24 * time.Hour),
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	// Record recent executions
	for i := 0; i < 2; i++ {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("new-exec-%d", i),
			WorkflowID:   fmt.Sprintf("new-wf-%d", i),
			WorkflowName: "new-workflow",
			Status:       "success",
			StartedAt:    now.Add(-1 * time.Hour),
			CompletedAt:  now,
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	// Verify all records exist before cleanup
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// Cleanup old records
	deleted, err := historyStore.Cleanup(ctx, 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 3, deleted)

	// Verify only recent records remain
	records, err = historyStore.List(ctx, &workflow.HistoryFilter{Limit: 100})
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestHistoryStore_Persistence_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")
	ctx := context.Background()
	now := time.Now()

	// First session: write data
	func() {
		historyStore, err := store.NewSQLiteHistoryStore(historyPath)
		require.NoError(t, err)
		defer historyStore.Close()

		for i := 0; i < 3; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("persist-exec-%d", i),
				WorkflowID:   fmt.Sprintf("persist-wf-%d", i),
				WorkflowName: "persist-test",
				Status:       "success",
				StartedAt:    now.Add(-time.Duration(i) * time.Minute),
				CompletedAt:  now.Add(-time.Duration(i)*time.Minute + 30*time.Second),
				DurationMs:   30000,
			}
			require.NoError(t, historyStore.Record(ctx, record))
		}
	}()

	// Verify data file exists
	_, err := os.Stat(historyPath)
	require.NoError(t, err)

	// Second session: read data
	func() {
		historyStore, err := store.NewSQLiteHistoryStore(historyPath)
		require.NoError(t, err)
		defer historyStore.Close()

		records, err := historyStore.List(ctx, &workflow.HistoryFilter{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, records, 3)

		// Verify data integrity
		for _, r := range records {
			assert.Contains(t, r.ID, "persist-exec-")
			assert.Equal(t, "persist-test", r.WorkflowName)
			assert.Equal(t, "success", r.Status)
		}
	}()
}

func TestHistoryService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	logger := &historyMockLogger{}
	historySvc := application.NewHistoryService(historyStore, logger)
	defer historySvc.Close()

	ctx := context.Background()
	now := time.Now()

	// Record through service
	record := &workflow.ExecutionRecord{
		ID:           "svc-exec-001",
		WorkflowID:   "svc-wf-001",
		WorkflowName: "service-test",
		Status:       "success",
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
		DurationMs:   60000,
	}
	err = historySvc.Record(ctx, record)
	require.NoError(t, err)

	// List through service (should apply default limit of 20)
	records, err := historySvc.List(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Len(t, records, 1)

	// Get stats through service
	stats, err := historySvc.GetStats(ctx, &workflow.HistoryFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TotalExecutions)
	assert.Equal(t, 1, stats.SuccessCount)
}

func TestHistoryService_CleanupOnInit_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")
	ctx := context.Background()
	now := time.Now()

	// First session: write old data
	func() {
		historyStore, err := store.NewSQLiteHistoryStore(historyPath)
		require.NoError(t, err)
		defer historyStore.Close()

		// Record old execution
		record := &workflow.ExecutionRecord{
			ID:           "old-exec",
			WorkflowID:   "old-wf",
			WorkflowName: "old-test",
			Status:       "success",
			StartedAt:    now.Add(-40 * 24 * time.Hour),
			CompletedAt:  now.Add(-35 * 24 * time.Hour),
		}
		require.NoError(t, historyStore.Record(ctx, record))

		// Record recent execution
		recentRecord := &workflow.ExecutionRecord{
			ID:           "new-exec",
			WorkflowID:   "new-wf",
			WorkflowName: "new-test",
			Status:       "success",
			StartedAt:    now.Add(-1 * time.Hour),
			CompletedAt:  now,
		}
		require.NoError(t, historyStore.Record(ctx, recentRecord))
	}()

	// Second session: service cleanup on init
	func() {
		historyStore, err := store.NewSQLiteHistoryStore(historyPath)
		require.NoError(t, err)
		defer historyStore.Close()

		logger := &historyMockLogger{}
		historySvc := application.NewHistoryService(historyStore, logger)
		defer historySvc.Close()

		// Run cleanup
		deleted, err := historySvc.Cleanup(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, deleted)

		// Verify only recent records remain
		records, err := historySvc.List(ctx, &workflow.HistoryFilter{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "new-exec", records[0].ID)
	}()
}

func TestWorkflowExecution_RecordsHistory_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	workflowDir := filepath.Join(tmpDir, "workflows")
	historyPath := filepath.Join(tmpDir, "history.db")

	// Create workflow directory and file
	require.NoError(t, os.MkdirAll(workflowDir, 0755))

	wfYAML := `name: history-test
version: "1.0.0"
states:
  initial: step1
  step1:
    type: step
    command: echo "hello"
    on_success: done
  done:
    type: terminal
`
	err := os.WriteFile(filepath.Join(workflowDir, "history-test.yaml"), []byte(wfYAML), 0644)
	require.NoError(t, err)

	// Setup components
	repo := repository.NewYAMLRepository(workflowDir)
	stateStore := newHistoryMockStateStore()
	exec := executor.NewShellExecutor()
	logger := &historyMockLogger{}
	resolver := interpolation.NewTemplateResolver()

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()
	historySvc := application.NewHistoryService(historyStore, logger)

	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionService(wfSvc, exec, parallelExec, stateStore, logger, resolver, historySvc)

	// Execute workflow
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execCtx, err := execSvc.Run(ctx, "history-test", nil)
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify history was recorded
	filter := &workflow.HistoryFilter{Limit: 10}
	records, err := historyStore.List(context.Background(), filter)
	require.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, execCtx.WorkflowID, records[0].ID)
	assert.Equal(t, "completed", records[0].Status)
}

func TestHistoryStore_CombinedFilters_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Create diverse test data
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
		require.NoError(t, historyStore.Record(ctx, record))
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			records, err := historyStore.List(ctx, tc.filter)
			require.NoError(t, err)
			assert.Len(t, records, tc.expectedCount)
		})
	}
}

func TestHistoryStore_StatsWithFilters_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history.db")

	historyStore, err := store.NewSQLiteHistoryStore(historyPath)
	require.NoError(t, err)
	defer historyStore.Close()

	ctx := context.Background()
	now := time.Now()

	// Create test data with different workflows and durations
	testData := []struct {
		workflowName string
		status       string
		durationMs   int64
	}{
		{"deploy", "success", 1000},
		{"deploy", "success", 2000},
		{"deploy", "failed", 500},
		{"build", "success", 3000},
		{"build", "failed", 1000},
	}

	for i, td := range testData {
		record := &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			WorkflowName: td.workflowName,
			Status:       td.status,
			StartedAt:    now.Add(-time.Duration(td.durationMs) * time.Millisecond),
			CompletedAt:  now,
			DurationMs:   td.durationMs,
		}
		require.NoError(t, historyStore.Record(ctx, record))
	}

	tests := []struct {
		name            string
		filter          *workflow.HistoryFilter
		expectedTotal   int
		expectedSuccess int
		expectedFailed  int
	}{
		{
			name:            "all workflows",
			filter:          &workflow.HistoryFilter{},
			expectedTotal:   5,
			expectedSuccess: 3,
			expectedFailed:  2,
		},
		{
			name:            "deploy only",
			filter:          &workflow.HistoryFilter{WorkflowName: "deploy"},
			expectedTotal:   3,
			expectedSuccess: 2,
			expectedFailed:  1,
		},
		{
			name:            "build only",
			filter:          &workflow.HistoryFilter{WorkflowName: "build"},
			expectedTotal:   2,
			expectedSuccess: 1,
			expectedFailed:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stats, err := historyStore.GetStats(ctx, tc.filter)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedTotal, stats.TotalExecutions)
			assert.Equal(t, tc.expectedSuccess, stats.SuccessCount)
			assert.Equal(t, tc.expectedFailed, stats.FailedCount)
		})
	}
}
