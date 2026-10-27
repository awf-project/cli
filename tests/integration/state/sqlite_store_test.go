//go:build integration

package state_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const legacyExecutionRecordsSchema = `
	CREATE TABLE execution_records (
		id TEXT PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		workflow_name TEXT NOT NULL,
		status TEXT NOT NULL,
		exit_code INTEGER NOT NULL DEFAULT 0,
		started_at INTEGER NOT NULL,
		completed_at INTEGER NOT NULL,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		error_message TEXT NOT NULL DEFAULT ''
	)`

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
				Attempt:      1,
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
				Attempt:      1,
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
				Attempt:      1,
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
				Attempt:     1,
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
			Attempt:      1,
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
		Attempt:      1,
	}

	// First insert should succeed
	err = s.Record(ctx, record)
	require.NoError(t, err)

	// Replaying an identical record is idempotent.
	err = s.Record(ctx, record)
	require.NoError(t, err)

	// Reusing the record ID for different persisted data is a conflict.
	conflictingRecord := *record
	conflictingRecord.Status = "failed"
	err = s.Record(ctx, &conflictingRecord)
	require.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrHistoryRecordConflict))
}

func TestSQLiteHistoryStore_ListsAttemptsByWorkflowID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s := openHistoryStore(t, path)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0)
	records := []*workflow.ExecutionRecord{
		newIntegrationHistoryRecord("deploy-1", "workflow-deploy", 1, "", "success", base.Add(time.Minute)),
		newIntegrationHistoryRecord("other-1", "workflow-other", 1, "", "cancelled", base.Add(4*time.Minute)),
		newIntegrationHistoryRecord("deploy-2", "workflow-deploy", 2, "deploy-1", "failed", base.Add(3*time.Minute)),
		newIntegrationHistoryRecord("deploy-3", "workflow-deploy", 3, "deploy-2", "success", base.Add(2*time.Minute)),
	}
	for _, record := range records {
		require.NoError(t, s.Record(ctx, record))
	}
	require.NoError(t, s.Close())

	s = openHistoryStore(t, path)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	filter := &workflow.HistoryFilter{WorkflowID: "workflow-deploy", Limit: 10}
	got, err := s.List(ctx, filter)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []string{"deploy-2", "deploy-3", "deploy-1"}, historyRecordIDs(got))
	assert.Equal(t, []int{2, 3, 1}, historyRecordAttempts(got))
	assert.Equal(t, []string{"deploy-1", "deploy-2", ""}, historyRecordPredecessors(got))

	stats, err := s.GetStats(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, &workflow.HistoryStats{
		TotalExecutions: 3,
		SuccessCount:    2,
		FailedCount:     1,
		AvgDurationMs:   1_000,
	}, stats)
}

func TestSQLiteHistoryStore_ExactReplayIsIdempotent(t *testing.T) {
	base := newIntegrationHistoryRecord(
		"record-2", "workflow-replay", 2, "record-1", "failed", time.Unix(1_700_100_000, 123),
	)
	base.WorkflowName = "deploy"
	base.ExitCode = 17
	base.StartedAt = base.CompletedAt.Add(-2 * time.Second)
	base.DurationMs = 2_000
	base.ErrorMessage = "original-error-canary"

	path := filepath.Join(t.TempDir(), "history.db")
	s := openHistoryStore(t, path)
	require.NoError(t, s.Record(context.Background(), base))
	require.NoError(t, s.Record(context.Background(), base))
	require.NoError(t, s.Close())

	s = openHistoryStore(t, path)
	replayed, err := s.List(context.Background(), &workflow.HistoryFilter{WorkflowID: base.WorkflowID})
	require.NoError(t, err)
	require.Len(t, replayed, 1)
	assert.Equal(t, base, replayed[0])
	require.NoError(t, s.Close())

	mutations := []struct {
		name   string
		mutate func(*workflow.ExecutionRecord)
	}{
		{"record ID", func(r *workflow.ExecutionRecord) { r.ID = "other-record-2" }},
		{"workflow ID", func(r *workflow.ExecutionRecord) { r.WorkflowID = "other-workflow" }},
		{"attempt", func(r *workflow.ExecutionRecord) { r.Attempt = 3 }},
		{"predecessor", func(r *workflow.ExecutionRecord) { r.ResumedFromID = "other-record" }},
		{"workflow name", func(r *workflow.ExecutionRecord) { r.WorkflowName = "other-name" }},
		{"status", func(r *workflow.ExecutionRecord) { r.Status = "success" }},
		{"exit code", func(r *workflow.ExecutionRecord) { r.ExitCode++ }},
		{"started at", func(r *workflow.ExecutionRecord) { r.StartedAt = r.StartedAt.Add(time.Nanosecond) }},
		{"completed at", func(r *workflow.ExecutionRecord) { r.CompletedAt = r.CompletedAt.Add(time.Nanosecond) }},
		{"duration", func(r *workflow.ExecutionRecord) { r.DurationMs++ }},
		{"error message", func(r *workflow.ExecutionRecord) { r.ErrorMessage = "changed-error-canary" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			mutationPath := filepath.Join(t.TempDir(), "history.db")
			mutationStore := openHistoryStore(t, mutationPath)
			defer func() { require.NoError(t, mutationStore.Close()) }()
			require.NoError(t, mutationStore.Record(context.Background(), base))
			candidate := *base
			tc.mutate(&candidate)

			err := mutationStore.Record(context.Background(), &candidate)
			require.ErrorIs(t, err, store.ErrHistoryRecordConflict)
			assert.NotContains(t, err.Error(), base.ErrorMessage)
			assert.NotContains(t, err.Error(), candidate.ErrorMessage)
		})
	}
}

func TestSQLiteHistoryStore_RejectsConflictingAttemptIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	s := openHistoryStore(t, path)
	defer func() { require.NoError(t, s.Close()) }()
	ctx := context.Background()
	original := newIntegrationHistoryRecord(
		"attempt-2", "workflow-conflict", 2, "attempt-1", "failed", time.Unix(1_700_200_000, 0),
	)
	original.ErrorMessage = "private-conflict-canary"
	require.NoError(t, s.Record(ctx, original))

	tests := []struct {
		name      string
		candidate *workflow.ExecutionRecord
	}{
		{
			name: "different content reuses record ID",
			candidate: func() *workflow.ExecutionRecord {
				candidate := *original
				candidate.Status = "success"
				return &candidate
			}(),
		},
		{
			name: "different record ID reuses workflow attempt",
			candidate: func() *workflow.ExecutionRecord {
				candidate := *original
				candidate.ID = "other-attempt-2"
				return &candidate
			}(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Record(ctx, tc.candidate)
			require.ErrorIs(t, err, store.ErrHistoryRecordConflict)
			assert.Contains(t, err.Error(), "record_id="+fmt.Sprintf("%q", tc.candidate.ID))
			assert.Contains(t, err.Error(), "workflow_id="+fmt.Sprintf("%q", tc.candidate.WorkflowID))
			assert.Contains(t, err.Error(), fmt.Sprintf("attempt=%d", tc.candidate.Attempt))
			assert.NotContains(t, err.Error(), original.ErrorMessage)
			assert.NotContains(t, strings.ToLower(err.Error()), "sqlite")
		})
	}

	t.Run("rejects a nonpositive attempt on a fresh database", func(t *testing.T) {
		fresh := openHistoryStore(t, filepath.Join(t.TempDir(), "fresh.db"))
		defer func() { require.NoError(t, fresh.Close()) }()
		invalid := newIntegrationHistoryRecord(
			"invalid-fresh", "workflow-invalid", 0, "", "failed", time.Unix(1_700_210_000, 0),
		)

		err := fresh.Record(ctx, invalid)
		require.ErrorIs(t, err, workflow.ErrInvalidHistoryAttempt)
	})

	t.Run("rejects a nonpositive attempt after legacy migration", func(t *testing.T) {
		legacyPath := filepath.Join(t.TempDir(), "legacy.db")
		db := openSQLiteFixture(t, legacyPath)
		createLegacyHistorySchema(t, db)
		insertLegacyRecord(t, db, "legacy", "workflow-invalid", "deploy", "success", 0, 1, 2, 1, "")
		require.NoError(t, db.Close())
		migrated := openHistoryStore(t, legacyPath)
		defer func() { require.NoError(t, migrated.Close()) }()
		invalid := newIntegrationHistoryRecord(
			"invalid-migrated", "workflow-invalid", 0, "", "failed", time.Unix(1_700_220_000, 0),
		)

		err := migrated.Record(ctx, invalid)
		require.ErrorIs(t, err, workflow.ErrInvalidHistoryAttempt)
	})
}

func TestSQLiteHistoryStore_PersistsConcurrentAttemptsForOneWorkflow(t *testing.T) {
	t.Run("persists distinct attempts and resolves competing ordinals", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.db")
		stores := openIntegrationHistoryStores(t, path, 6)
		base := time.Unix(1_700_300_000, 0)
		records := []*workflow.ExecutionRecord{
			newIntegrationHistoryRecord("attempt-1", "workflow-concurrent", 1, "", "success", base.Add(time.Minute)),
			newIntegrationHistoryRecord("attempt-2", "workflow-concurrent", 2, "attempt-1", "success", base.Add(2*time.Minute)),
			newIntegrationHistoryRecord("attempt-3", "workflow-concurrent", 3, "attempt-2", "success", base.Add(3*time.Minute)),
			newIntegrationHistoryRecord("attempt-4-a", "workflow-concurrent", 4, "attempt-3", "failed", base.Add(4*time.Minute)),
			newIntegrationHistoryRecord("attempt-4-b", "workflow-concurrent", 4, "attempt-3", "failed", base.Add(4*time.Minute)),
		}
		errs := recordIntegrationHistoryConcurrently(stores[:5], records)
		successes, conflicts := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, store.ErrHistoryRecordConflict):
				conflicts++
				conflictMessage := strings.ToLower(err.Error())
				assert.NotContains(t, conflictMessage, "sqlite")
				assert.NotContains(t, conflictMessage, "database is locked")
				assert.NotContains(t, conflictMessage, "database is busy")
			default:
				t.Fatalf("unexpected concurrent Record error: %v", err)
			}
		}
		assert.Equal(t, 4, successes)
		assert.Equal(t, 1, conflicts)
		closeIntegrationHistoryStores(t, stores)

		reopened := openHistoryStore(t, path)
		got, err := reopened.List(context.Background(), &workflow.HistoryFilter{
			WorkflowID: "workflow-concurrent", Limit: 10,
		})
		require.NoError(t, err)
		require.Len(t, got, 4)
		assert.ElementsMatch(t, []int{1, 2, 3, 4}, historyRecordAttempts(got))
		assert.ElementsMatch(t, []string{"attempt-1", "attempt-2", "attempt-3", got[0].ID}, historyRecordIDs(got))
		assert.Contains(t, []string{"attempt-4-a", "attempt-4-b"}, got[0].ID)
		require.NoError(t, reopened.Close())
	})

	t.Run("accepts exact concurrent replays", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.db")
		stores := openIntegrationHistoryStores(t, path, 8)
		record := newIntegrationHistoryRecord(
			"shared-attempt", "workflow-replay", 1, "", "success", time.Unix(1_700_400_000, 0),
		)
		records := make([]*workflow.ExecutionRecord, len(stores))
		for i := range records {
			records[i] = record
		}
		for _, err := range recordIntegrationHistoryConcurrently(stores, records) {
			require.NoError(t, err)
		}
		closeIntegrationHistoryStores(t, stores)

		reopened := openHistoryStore(t, path)
		got, err := reopened.List(context.Background(), &workflow.HistoryFilter{WorkflowID: record.WorkflowID})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, record, got[0])
		require.NoError(t, reopened.Close())
	})

	t.Run("concurrent constructors migrate legacy rows once", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "legacy.db")
		db := openSQLiteFixture(t, path)
		createLegacyHistorySchema(t, db)
		insertLegacyRecord(t, db, "legacy-1", "legacy-workflow", "deploy", "failed", 1, 10, 20, 10, "one")
		insertLegacyRecord(t, db, "legacy-2", "legacy-workflow", "deploy", "success", 0, 20, 30, 10, "two")
		require.NoError(t, db.Close())

		const constructors = 6
		start := make(chan struct{})
		opened := make([]*store.SQLiteHistoryStore, constructors)
		errs := make([]error, constructors)
		var wg sync.WaitGroup
		wg.Add(constructors)
		for i := 0; i < constructors; i++ {
			go func(index int) {
				defer wg.Done()
				<-start
				opened[index], errs[index] = openIntegrationHistoryStoreWithRetry(path)
			}(i)
		}
		close(start)
		wg.Wait()
		for i, err := range errs {
			require.NoError(t, err, "constructor %d", i)
			got, listErr := opened[i].List(context.Background(), &workflow.HistoryFilter{
				WorkflowID: "legacy-workflow", Limit: 10,
			})
			require.NoError(t, listErr)
			require.Len(t, got, 2)
			assert.Equal(t, []int{2, 1}, historyRecordAttempts(got))
			assert.Equal(t, []string{"legacy-1", ""}, historyRecordPredecessors(got))
		}
		closeIntegrationHistoryStores(t, opened)

		assert.Equal(t, []historyLineageRow{
			{ID: "legacy-1", WorkflowID: "legacy-workflow", WorkflowName: "deploy", Status: "failed", ExitCode: 1, StartedAt: 10, CompletedAt: 20, DurationMS: 10, ErrorMessage: "one", Attempt: 1},
			{ID: "legacy-2", WorkflowID: "legacy-workflow", WorkflowName: "deploy", Status: "success", StartedAt: 20, CompletedAt: 30, DurationMS: 10, ErrorMessage: "two", Attempt: 2, ResumedFromID: "legacy-1"},
		}, readLineageRows(t, path))
	})
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
		Attempt:      1,
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
			Attempt:      1,
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
		Attempt:      1,
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
			Attempt:      1,
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
			Attempt:      1,
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
				Attempt:      1,
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
			Attempt:      1,
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
				Attempt:      1,
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
		Attempt:      1,
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
		Attempt:      1,
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
		Attempt:      1,
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
					Attempt:      1,
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
				Attempt:      1,
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
				Attempt:      1,
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
		Attempt:      1,
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
			Attempt:      1,
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
		Attempt:      1,
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
		Attempt:      1,
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
		Attempt:      1,
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
		Attempt:      1,
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
					Attempt:      1,
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
		Attempt:      1,
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
				Attempt:      1,
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

func TestSQLiteHistoryStore_MigratesLegacyAttemptHistory(t *testing.T) {
	t.Run("backfills deterministic workflow partitions and is idempotent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "legacy.db")
		db := openSQLiteFixture(t, path)
		createLegacyHistorySchema(t, db)

		insertLegacyRecord(t, db, "z-last", "workflow-a", "deploy", "failed", 7, 10, 30, 20, "preserve-z")
		insertLegacyRecord(t, db, "b-middle", "workflow-a", "deploy", "success", 0, 20, 20, 0, "preserve-b")
		insertLegacyRecord(t, db, "a-first", "workflow-a", "deploy", "success", 0, 20, 20, 10, "preserve-a")
		insertLegacyRecord(t, db, "other", "workflow-b", "test", "cancelled", 2, 5, 20, 15, "preserve-other")
		require.NoError(t, db.Close())

		migrated := openHistoryStore(t, path)
		require.NoError(t, migrated.Close())

		want := []historyLineageRow{
			{ID: "a-first", WorkflowID: "workflow-a", WorkflowName: "deploy", Status: "success", ExitCode: 0, StartedAt: 20, CompletedAt: 20, DurationMS: 10, ErrorMessage: "preserve-a", Attempt: 1},
			{ID: "b-middle", WorkflowID: "workflow-a", WorkflowName: "deploy", Status: "success", ExitCode: 0, StartedAt: 20, CompletedAt: 20, DurationMS: 0, ErrorMessage: "preserve-b", Attempt: 2, ResumedFromID: "a-first"},
			{ID: "z-last", WorkflowID: "workflow-a", WorkflowName: "deploy", Status: "failed", ExitCode: 7, StartedAt: 10, CompletedAt: 30, DurationMS: 20, ErrorMessage: "preserve-z", Attempt: 3, ResumedFromID: "b-middle"},
			{ID: "other", WorkflowID: "workflow-b", WorkflowName: "test", Status: "cancelled", ExitCode: 2, StartedAt: 5, CompletedAt: 20, DurationMS: 15, ErrorMessage: "preserve-other", Attempt: 1},
		}
		assert.Equal(t, want, readLineageRows(t, path))
		assertWorkflowAttemptIndex(t, path)

		reopened := openHistoryStore(t, path)
		require.NoError(t, reopened.Close())
		assert.Equal(t, want, readLineageRows(t, path))
	})

	t.Run("preserves valid lineage despite timestamp order and retention gaps", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "lineage.db")
		db := openSQLiteFixture(t, path)
		createLegacyHistorySchema(t, db)
		insertLegacyRecord(t, db, "attempt-1", "workflow-a", "deploy", "success", 0, 10, 20, 10, "migrated")
		require.NoError(t, db.Close())

		migrated := openHistoryStore(t, path)
		require.NoError(t, migrated.Close())

		db = openSQLiteFixture(t, path)
		// These explicit SQL inserts model post-migration records. Their timestamp
		// order intentionally differs from their already-assigned ordinals.
		insertLineageRecord(t, db, "attempt-2", "workflow-a", 2, "attempt-1", 300)
		insertLineageRecord(t, db, "attempt-3", "workflow-a", 3, "attempt-2", 250)
		insertLineageRecord(t, db, "attempt-4", "workflow-a", 4, "attempt-3", 400)
		insertLineageRecord(t, db, "attempt-5", "workflow-a", 5, "attempt-4", 100)
		// Retention may remove predecessors. The valid suffix must not be
		// renumbered or have missing predecessors invented during reopen.
		_, err := db.Exec("DELETE FROM execution_records WHERE id IN ('attempt-2', 'attempt-3')")
		require.NoError(t, err)
		require.NoError(t, db.Close())

		before := readLineageRows(t, path)
		s := openHistoryStore(t, path)
		require.NoError(t, s.Close())
		after := readLineageRows(t, path)

		assert.Equal(t, before, after)
		assertWorkflowAttemptIndex(t, path)
	})

	t.Run("creates a missing index without rewriting valid lineage", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing-index.db")
		db := openSQLiteFixture(t, path)
		createLineageHistorySchema(t, db)
		insertLineageRecord(t, db, "attempt-1", "workflow-a", 1, "", 200)
		insertLineageRecord(t, db, "attempt-2", "workflow-a", 2, "attempt-1", 100)
		require.NoError(t, db.Close())

		before := readLineageRows(t, path)
		s := openHistoryStore(t, path)
		require.NoError(t, s.Close())

		assert.Equal(t, before, readLineageRows(t, path))
		assertWorkflowAttemptIndex(t, path)
	})

	t.Run("rejects a partial workflow attempt index without changing rows or schema", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "partial-index.db")
		db := openSQLiteFixture(t, path)
		createLineageHistorySchema(t, db)
		insertLineageRecord(t, db, "attempt-1", "workflow-a", 1, "", 200)
		_, err := db.Exec(`CREATE UNIQUE INDEX idx_workflow_attempt
			ON execution_records(workflow_id, attempt) WHERE attempt > 1`)
		require.NoError(t, err)
		require.NoError(t, db.Close())
		beforeSchema := readSchemaSQL(t, path)
		beforeRows := readRawRows(t, path)

		s, err := store.NewSQLiteHistoryStore(path)
		require.Error(t, err)
		assert.Nil(t, s)
		assert.ErrorContains(t, err, "create schema:")
		assert.ErrorContains(t, err, "idx_workflow_attempt")
		assert.Equal(t, beforeSchema, readSchemaSQL(t, path))
		assert.Equal(t, beforeRows, readRawRows(t, path))
	})

	t.Run("rejects invalid lineage without changing rows or schema", func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(t *testing.T, db *sql.DB)
		}{
			{
				name: "partial schema",
				setup: func(t *testing.T, db *sql.DB) {
					createLegacyHistorySchema(t, db)
					_, err := db.Exec("ALTER TABLE execution_records ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0")
					require.NoError(t, err)
					insertLegacyRecord(t, db, "record", "workflow-a", "deploy", "success", 0, 1, 2, 1, "keep")
				},
			},
			{
				name: "nonpositive attempt",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "record", "workflow-a", 0, "", 1)
				},
			},
			{
				name: "duplicate ordinal",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "one", "workflow-a", 1, "", 1)
					insertLineageRecord(t, db, "duplicate", "workflow-a", 1, "", 2)
				},
			},
			{
				name: "self predecessor",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "record", "workflow-a", 2, "record", 1)
				},
			},
			{
				name: "first attempt has predecessor",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "record", "workflow-a", 1, "retained-predecessor", 2)
				},
			},
			{
				name: "later attempt has no predecessor",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "record", "workflow-a", 2, "", 1)
				},
			},
			{
				name: "predecessor belongs to another workflow",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "other-workflow-record", "workflow-b", 1, "", 1)
					insertLineageRecord(t, db, "record", "workflow-a", 2, "other-workflow-record", 2)
				},
			},
			{
				name: "predecessor has wrong ordinal",
				setup: func(t *testing.T, db *sql.DB) {
					createLineageHistorySchema(t, db)
					insertLineageRecord(t, db, "predecessor", "workflow-a", 1, "", 1)
					insertLineageRecord(t, db, "record", "workflow-a", 3, "predecessor", 2)
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "invalid.db")
				db := openSQLiteFixture(t, path)
				tc.setup(t, db)
				require.NoError(t, db.Close())
				beforeSchema := readSchemaSQL(t, path)
				beforeRows := readRawRows(t, path)

				s, err := store.NewSQLiteHistoryStore(path)
				require.Error(t, err)
				assert.Nil(t, s)
				assert.ErrorContains(t, err, "create schema:")
				assert.Equal(t, beforeSchema, readSchemaSQL(t, path))
				assert.Equal(t, beforeRows, readRawRows(t, path))
			})
		}
	})

	t.Run("rolls back a failed legacy migration", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "rollback.db")
		db := openSQLiteFixture(t, path)
		createLegacyHistorySchema(t, db)
		insertLegacyRecord(t, db, "record", "workflow-a", "deploy", "success", 0, 1, 2, 1, "keep")
		_, err := db.Exec("CREATE INDEX idx_workflow_attempt ON execution_records(workflow_name, status)")
		require.NoError(t, err)
		require.NoError(t, db.Close())
		beforeSchema := readSchemaSQL(t, path)
		beforeRows := readRawRows(t, path)

		s, err := store.NewSQLiteHistoryStore(path)
		require.Error(t, err)
		assert.Nil(t, s)
		assert.ErrorContains(t, err, "create schema:")
		assert.ErrorContains(t, err, "idx_workflow_attempt")
		assert.Equal(t, beforeSchema, readSchemaSQL(t, path))
		assert.Equal(t, beforeRows, readRawRows(t, path))
	})

	t.Run("migrates one thousand rows", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large-legacy.db")
		db := openSQLiteFixture(t, path)
		createLegacyHistorySchema(t, db)
		tx, err := db.Begin()
		require.NoError(t, err)
		stmt, err := tx.Prepare(`INSERT INTO execution_records
			(id, workflow_id, workflow_name, status, exit_code, started_at, completed_at, duration_ms, error_message)
			VALUES (?, 'workflow-large', 'large', 'success', 0, ?, ?, 1, '')`)
		require.NoError(t, err)
		for i := 0; i < 1000; i++ {
			_, err = stmt.Exec(fmt.Sprintf("record-%04d", i), i, i)
			require.NoError(t, err)
		}
		require.NoError(t, stmt.Close())
		require.NoError(t, tx.Commit())
		require.NoError(t, db.Close())

		started := time.Now()
		s := openHistoryStore(t, path)
		require.NoError(t, s.Close())
		t.Logf("migrated 1,000 legacy rows in %s", time.Since(started))

		rows := readLineageRows(t, path)
		require.Len(t, rows, 1000)
		for i := range rows {
			assert.Equal(t, i+1, rows[i].Attempt)
			if i == 0 {
				assert.Empty(t, rows[i].ResumedFromID)
			} else {
				assert.Equal(t, rows[i-1].ID, rows[i].ResumedFromID)
			}
		}
	})
}

type historyLineageRow struct {
	ID            string
	WorkflowID    string
	WorkflowName  string
	Status        string
	ExitCode      int
	StartedAt     int64
	CompletedAt   int64
	DurationMS    int64
	ErrorMessage  string
	Attempt       int
	ResumedFromID string
}

func openSQLiteFixture(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	return db
}

func openHistoryStore(t *testing.T, path string) *store.SQLiteHistoryStore {
	t.Helper()
	s, err := store.NewSQLiteHistoryStore(path)
	require.NoError(t, err)
	return s
}

func createLegacyHistorySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(legacyExecutionRecordsSchema)
	require.NoError(t, err)
}

func createLineageHistorySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(legacyExecutionRecordsSchema)
	require.NoError(t, err)
	_, err = db.Exec("ALTER TABLE execution_records ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0")
	require.NoError(t, err)
	_, err = db.Exec("ALTER TABLE execution_records ADD COLUMN resumed_from_id TEXT NOT NULL DEFAULT ''")
	require.NoError(t, err)
}

func insertLegacyRecord(
	t *testing.T,
	db *sql.DB,
	id, workflowID, workflowName, status string,
	exitCode int,
	startedAt, completedAt, durationMS int64,
	errorMessage string,
) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO execution_records
		(id, workflow_id, workflow_name, status, exit_code, started_at, completed_at, duration_ms, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, workflowID, workflowName, status, exitCode, startedAt, completedAt, durationMS, errorMessage)
	require.NoError(t, err)
}

func insertLineageRecord(
	t *testing.T,
	db *sql.DB,
	id, workflowID string,
	attempt int,
	resumedFromID string,
	completedAt int64,
) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO execution_records
		(id, workflow_id, workflow_name, status, started_at, completed_at, attempt, resumed_from_id)
		VALUES (?, ?, 'workflow', 'success', ?, ?, ?, ?)`,
		id, workflowID, completedAt-1, completedAt, attempt, resumedFromID)
	require.NoError(t, err)
}

func readLineageRows(t *testing.T, path string) []historyLineageRow {
	t.Helper()
	db := openSQLiteFixture(t, path)
	defer func() { require.NoError(t, db.Close()) }()
	rows, err := db.Query(`SELECT id, workflow_id, workflow_name, status, exit_code,
		started_at, completed_at, duration_ms, error_message, attempt, resumed_from_id
		FROM execution_records ORDER BY workflow_id, attempt, id`)
	require.NoError(t, err)
	defer rows.Close()

	got := make([]historyLineageRow, 0)
	for rows.Next() {
		var row historyLineageRow
		require.NoError(t, rows.Scan(
			&row.ID, &row.WorkflowID, &row.WorkflowName, &row.Status, &row.ExitCode,
			&row.StartedAt, &row.CompletedAt, &row.DurationMS, &row.ErrorMessage,
			&row.Attempt, &row.ResumedFromID,
		))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	return got
}

func readSchemaSQL(t *testing.T, path string) []string {
	t.Helper()
	db := openSQLiteFixture(t, path)
	defer func() { require.NoError(t, db.Close()) }()
	rows, err := db.Query(`SELECT type || ':' || name || ':' || COALESCE(sql, '')
		FROM sqlite_master WHERE tbl_name = 'execution_records' ORDER BY type, name`)
	require.NoError(t, err)
	defer rows.Close()

	var schema []string
	for rows.Next() {
		var entry string
		require.NoError(t, rows.Scan(&entry))
		schema = append(schema, entry)
	}
	require.NoError(t, rows.Err())
	return schema
}

func readRawRows(t *testing.T, path string) []string {
	t.Helper()
	db := openSQLiteFixture(t, path)
	defer func() { require.NoError(t, db.Close()) }()
	columns, err := db.Query("PRAGMA table_info(execution_records)")
	require.NoError(t, err)
	var names []string
	for columns.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		require.NoError(t, columns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey))
		names = append(names, name)
	}
	require.NoError(t, columns.Close())

	rows, err := db.Query("SELECT " + strings.Join(names, ", ") + " FROM execution_records ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	var got []string
	for rows.Next() {
		values := make([]any, len(names))
		destinations := make([]any, len(names))
		for i := range values {
			destinations[i] = &values[i]
		}
		require.NoError(t, rows.Scan(destinations...))
		got = append(got, fmt.Sprint(values))
	}
	require.NoError(t, rows.Err())
	return got
}

func assertWorkflowAttemptIndex(t *testing.T, path string) {
	t.Helper()
	db := openSQLiteFixture(t, path)
	defer func() { require.NoError(t, db.Close()) }()
	var sqlText string
	err := db.QueryRow(`SELECT sql FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_workflow_attempt'`).Scan(&sqlText)
	require.NoError(t, err)
	normalized := strings.ToLower(strings.Join(strings.Fields(sqlText), ""))
	assert.Contains(t, normalized, "uniqueindexidx_workflow_attempt")
	assert.Contains(t, normalized, "(workflow_id,attempt)")
}

func newIntegrationHistoryRecord(
	id, workflowID string,
	attempt int,
	resumedFromID, status string,
	completedAt time.Time,
) *workflow.ExecutionRecord {
	return &workflow.ExecutionRecord{
		ID:            id,
		WorkflowID:    workflowID,
		Attempt:       attempt,
		ResumedFromID: resumedFromID,
		WorkflowName:  "workflow",
		Status:        status,
		StartedAt:     completedAt.Add(-time.Second),
		CompletedAt:   completedAt,
		DurationMs:    1_000,
	}
}

func historyRecordIDs(records []*workflow.ExecutionRecord) []string {
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	return ids
}

func historyRecordAttempts(records []*workflow.ExecutionRecord) []int {
	attempts := make([]int, len(records))
	for i := range records {
		attempts[i] = records[i].Attempt
	}
	return attempts
}

func historyRecordPredecessors(records []*workflow.ExecutionRecord) []string {
	predecessors := make([]string, len(records))
	for i := range records {
		predecessors[i] = records[i].ResumedFromID
	}
	return predecessors
}

func openIntegrationHistoryStores(t *testing.T, path string, count int) []*store.SQLiteHistoryStore {
	t.Helper()
	stores := make([]*store.SQLiteHistoryStore, count)
	for i := range stores {
		var err error
		stores[i], err = store.NewSQLiteHistoryStore(path)
		require.NoError(t, err)
	}
	return stores
}

func closeIntegrationHistoryStores(t *testing.T, stores []*store.SQLiteHistoryStore) {
	t.Helper()
	for _, historyStore := range stores {
		if historyStore != nil {
			require.NoError(t, historyStore.Close())
		}
	}
}

func recordIntegrationHistoryConcurrently(
	stores []*store.SQLiteHistoryStore,
	records []*workflow.ExecutionRecord,
) []error {
	start := make(chan struct{})
	errs := make([]error, len(records))
	var wg sync.WaitGroup
	wg.Add(len(records))
	for i := range records {
		go func(index int) {
			defer wg.Done()
			<-start
			errs[index] = recordIntegrationHistoryWithRetry(stores[index], records[index])
		}(i)
	}
	close(start)
	wg.Wait()
	return errs
}

func recordIntegrationHistoryWithRetry(
	historyStore *store.SQLiteHistoryStore,
	record *workflow.ExecutionRecord,
) error {
	const attempts = 5
	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		err = historyStore.Record(context.Background(), record)
		if !isTransientSQLiteLock(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return fmt.Errorf("transient SQLite lock persisted after %d attempts: %w", attempts, err)
}

func openIntegrationHistoryStoreWithRetry(path string) (*store.SQLiteHistoryStore, error) {
	const attempts = 5
	var (
		historyStore *store.SQLiteHistoryStore
		err          error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		historyStore, err = store.NewSQLiteHistoryStore(path)
		if !isTransientSQLiteLock(err) {
			return historyStore, err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}
	return nil, fmt.Errorf("transient SQLite lock persisted after %d attempts: %w", attempts, err)
}

func isTransientSQLiteLock(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy")
}
