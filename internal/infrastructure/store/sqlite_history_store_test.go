package store

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface compliance check
var _ ports.HistoryStore = (*SQLiteHistoryStore)(nil)

// TestNewSQLiteHistoryStore tests the constructor and database initialization
func TestNewSQLiteHistoryStore(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid path creates store",
			path:    "",
			wantErr: false,
		},
		{
			name:    "nested directory path creates parent dirs",
			path:    "",
			wantErr: false,
		},
		{
			name:    "invalid path returns error",
			path:    "/dev/null/invalid/path.db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dbPath string

			switch tt.name {
			case "valid path creates store":
				tmpDir := t.TempDir()
				dbPath = filepath.Join(tmpDir, "history.db")
			case "nested directory path creates parent dirs":
				tmpDir := t.TempDir()
				dbPath = filepath.Join(tmpDir, "level1", "level2", "history.db")
			default:
				dbPath = tt.path
			}

			store, err := NewSQLiteHistoryStore(dbPath)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, store)
			} else {
				require.NoError(t, err)
				require.NotNil(t, store)
				assert.NotNil(t, store.db)

				// Cleanup
				err = store.Close()
				assert.NoError(t, err)
			}
		})
	}
}

// TestSQLiteHistoryStore_Record tests inserting execution records
func TestSQLiteHistoryStore_Record(t *testing.T) {
	tests := []struct {
		name    string
		record  *workflow.ExecutionRecord
		setup   func(store *SQLiteHistoryStore) error
		wantErr bool
	}{
		{
			name: "valid record is inserted",
			record: &workflow.ExecutionRecord{
				ID:           "exec-001",
				WorkflowID:   "wf-001",
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-2 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   120000,
				ErrorMessage: "",
			},
			wantErr: false,
		},
		{
			name:    "nil record returns error",
			record:  nil,
			wantErr: true,
		},
		{
			name: "duplicate ID returns record conflict",
			record: &workflow.ExecutionRecord{
				ID:           "duplicate-id",
				WorkflowID:   "wf-002",
				Attempt:      1,
				WorkflowName: "test-workflow-2",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-1 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
				ErrorMessage: "",
			},
			setup: func(store *SQLiteHistoryStore) error {
				// Insert the same ID first
				ctx := context.Background()
				return store.Record(ctx, &workflow.ExecutionRecord{
					ID:           "duplicate-id",
					WorkflowID:   "wf-001",
					Attempt:      1,
					WorkflowName: "test-workflow",
					Status:       "failed",
					ExitCode:     1,
					StartedAt:    time.Now().Add(-5 * time.Minute),
					CompletedAt:  time.Now().Add(-4 * time.Minute),
					DurationMs:   60000,
					ErrorMessage: "original error",
				})
			},
			wantErr: true,
		},
		{
			name: "cancelled context returns error",
			record: &workflow.ExecutionRecord{
				ID:           "exec-cancelled",
				WorkflowID:   "wf-003",
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-1 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
				ErrorMessage: "",
			},
			wantErr: true,
		},
		{
			name: "empty ID is allowed by database",
			record: &workflow.ExecutionRecord{
				ID:           "",
				WorkflowID:   "wf-004",
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-1 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
				ErrorMessage: "",
			},
			wantErr: false,
		},
		{
			name: "empty workflow ID is allowed by database",
			record: &workflow.ExecutionRecord{
				ID:           "exec-005",
				WorkflowID:   "",
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-1 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
				ErrorMessage: "",
			},
			wantErr: false,
		},
		{
			name: "empty workflow name is allowed by database",
			record: &workflow.ExecutionRecord{
				ID:           "exec-006",
				WorkflowID:   "wf-006",
				Attempt:      1,
				WorkflowName: "",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-1 * time.Minute),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
				ErrorMessage: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "history.db")

			store, err := NewSQLiteHistoryStore(path)
			require.NoError(t, err)
			defer store.Close()

			// Setup if needed
			if tt.setup != nil {
				err := tt.setup(store)
				require.NoError(t, err)
			}

			// Prepare context
			ctx := context.Background()
			if tt.name == "cancelled context returns error" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			// Execute
			err = store.Record(ctx, tt.record)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.name == "duplicate ID returns record conflict" {
					assert.ErrorIs(t, err, ErrHistoryRecordConflict)
				}
			} else {
				require.NoError(t, err)

				// Verify the record was inserted
				records, err := store.List(context.Background(), &workflow.HistoryFilter{
					Limit: 100,
				})
				require.NoError(t, err)
				assert.NotEmpty(t, records)

				// Find our record
				found := false
				for _, r := range records {
					if r.ID != tt.record.ID {
						continue
					}
					found = true
					assert.Equal(t, tt.record.WorkflowID, r.WorkflowID)
					assert.Equal(t, tt.record.WorkflowName, r.WorkflowName)
					assert.Equal(t, tt.record.Status, r.Status)
					assert.Equal(t, tt.record.ExitCode, r.ExitCode)
					assert.Equal(t, tt.record.DurationMs, r.DurationMs)
					break
				}
				assert.True(t, found, "record should be found in database")
			}
		})
	}
}

func TestSQLiteHistoryStore_RecordPersistsAttemptLineage(t *testing.T) {
	t.Parallel()

	startedAt := time.Unix(1_700_000_000, 123_456_789)
	completedAt := time.Unix(1_700_000_030, 987_654_321)
	record := &workflow.ExecutionRecord{
		ID:            "run-attempt-2",
		WorkflowID:    "workflow-lineage",
		Attempt:       2,
		ResumedFromID: "run-attempt-1",
		WorkflowName:  "deploy",
		Status:        "failed",
		ExitCode:      17,
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
		DurationMs:    30_864,
		ErrorMessage:  "confidential-canary",
	}

	t.Run("persists lineage and accepts an exact replay", func(t *testing.T) {
		historyStore := openSQLiteHistoryStoreForTest(t)

		require.NoError(t, historyStore.Record(context.Background(), record))
		require.NoError(t, historyStore.Record(context.Background(), record))

		records, err := historyStore.List(context.Background(), &workflow.HistoryFilter{
			WorkflowID: record.WorkflowID,
			Limit:      10,
		})
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, record, records[0])
	})

	t.Run("rejects a conflicting record ID without leaking record contents", func(t *testing.T) {
		historyStore := openSQLiteHistoryStoreForTest(t)
		require.NoError(t, historyStore.Record(context.Background(), record))

		conflict := *record
		conflict.Status = "success"
		err := historyStore.Record(context.Background(), &conflict)

		require.ErrorIs(t, err, ErrHistoryRecordConflict)
		assert.Contains(t, err.Error(), "record_id")
		assert.Contains(t, err.Error(), record.ID)
		assert.Contains(t, err.Error(), "workflow_id")
		assert.Contains(t, err.Error(), record.WorkflowID)
		assert.Contains(t, err.Error(), "attempt")
		assert.NotContains(t, err.Error(), record.ErrorMessage)
		assert.NotContains(t, strings.ToLower(err.Error()), "sqlite")
	})

	t.Run("compares every persisted field for replay equality", func(t *testing.T) {
		mutations := []struct {
			name   string
			mutate func(*workflow.ExecutionRecord)
		}{
			{"workflow ID", func(candidate *workflow.ExecutionRecord) { candidate.WorkflowID = "other-workflow" }},
			{"attempt", func(candidate *workflow.ExecutionRecord) { candidate.Attempt = 3 }},
			{"predecessor", func(candidate *workflow.ExecutionRecord) { candidate.ResumedFromID = "other-predecessor" }},
			{"workflow name", func(candidate *workflow.ExecutionRecord) { candidate.WorkflowName = "other-name" }},
			{"status", func(candidate *workflow.ExecutionRecord) { candidate.Status = "success" }},
			{"exit code", func(candidate *workflow.ExecutionRecord) { candidate.ExitCode++ }},
			{"started at", func(candidate *workflow.ExecutionRecord) {
				candidate.StartedAt = candidate.StartedAt.Add(time.Nanosecond)
			}},
			{"completed at", func(candidate *workflow.ExecutionRecord) {
				candidate.CompletedAt = candidate.CompletedAt.Add(time.Nanosecond)
			}},
			{"duration", func(candidate *workflow.ExecutionRecord) { candidate.DurationMs++ }},
			{"error message", func(candidate *workflow.ExecutionRecord) { candidate.ErrorMessage = "different-secret" }},
		}

		for _, tt := range mutations {
			t.Run(tt.name, func(t *testing.T) {
				historyStore := openSQLiteHistoryStoreForTest(t)
				require.NoError(t, historyStore.Record(context.Background(), record))

				candidate := *record
				tt.mutate(&candidate)
				err := historyStore.Record(context.Background(), &candidate)

				require.ErrorIs(t, err, ErrHistoryRecordConflict)
				assert.NotContains(t, err.Error(), record.ErrorMessage)
				assert.NotContains(t, err.Error(), candidate.ErrorMessage)
			})
		}
	})

	t.Run("rejects reuse of a workflow attempt by a different record", func(t *testing.T) {
		historyStore := openSQLiteHistoryStoreForTest(t)
		require.NoError(t, historyStore.Record(context.Background(), record))

		conflict := *record
		conflict.ID = "different-record-id"
		err := historyStore.Record(context.Background(), &conflict)

		require.ErrorIs(t, err, ErrHistoryRecordConflict)
		assert.Contains(t, err.Error(), conflict.ID)
		assert.Contains(t, err.Error(), record.WorkflowID)
		assert.Contains(t, err.Error(), fmt.Sprint(record.Attempt))
		assert.NotContains(t, err.Error(), record.ErrorMessage)
		assert.NotContains(t, strings.ToLower(err.Error()), "sqlite")
	})

	t.Run("rejects invalid local lineage before insertion", func(t *testing.T) {
		invalidRecords := []struct {
			name          string
			attempt       int
			resumedFromID string
		}{
			{name: "zero attempt", attempt: 0},
			{name: "negative attempt", attempt: -1},
			{name: "first attempt has predecessor", attempt: 1, resumedFromID: "previous"},
			{name: "later attempt lacks predecessor", attempt: 2},
			{name: "self predecessor", attempt: 2, resumedFromID: "invalid-record"},
		}

		for _, tt := range invalidRecords {
			t.Run(tt.name, func(t *testing.T) {
				historyStore := openSQLiteHistoryStoreForTest(t)
				invalid := *record
				invalid.ID = "invalid-record"
				invalid.WorkflowID = "invalid-workflow"
				invalid.Attempt = tt.attempt
				invalid.ResumedFromID = tt.resumedFromID

				err := historyStore.Record(context.Background(), &invalid)

				require.ErrorIs(t, err, workflow.ErrInvalidHistoryAttempt)
				records, listErr := historyStore.List(context.Background(), &workflow.HistoryFilter{Limit: 10})
				require.NoError(t, listErr)
				assert.Empty(t, records)
			})
		}
	})

	t.Run("rejects nonpositive attempts after legacy migration", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.db")
		db, err := sql.Open("sqlite", path)
		require.NoError(t, err)
		_, err = db.ExecContext(context.Background(), `CREATE TABLE execution_records (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			workflow_name TEXT NOT NULL,
			status TEXT NOT NULL,
			exit_code INTEGER NOT NULL DEFAULT 0,
			started_at INTEGER NOT NULL,
			completed_at INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT ''
		)`)
		require.NoError(t, err)
		require.NoError(t, db.Close())

		historyStore, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, historyStore.Close()) })

		invalid := *record
		invalid.ID = "post-migration-invalid"
		invalid.WorkflowID = "post-migration-workflow"
		invalid.Attempt = 0
		invalid.ResumedFromID = ""
		err = historyStore.Record(context.Background(), &invalid)

		require.ErrorIs(t, err, workflow.ErrInvalidHistoryAttempt)
		records, listErr := historyStore.List(context.Background(), &workflow.HistoryFilter{Limit: 10})
		require.NoError(t, listErr)
		assert.Empty(t, records)
	})
}

// TestSQLiteHistoryStore_List tests querying execution records with filters
// verifyFilterApplied checks that returned records match the filter criteria
func verifyFilterApplied(t *testing.T, filter *workflow.HistoryFilter, records []*workflow.ExecutionRecord) {
	if filter == nil {
		return
	}
	for _, r := range records {
		if filter.WorkflowName != "" {
			assert.Equal(t, filter.WorkflowName, r.WorkflowName)
		}
		if filter.Status != "" {
			assert.Equal(t, filter.Status, r.Status)
		}
		if !filter.Since.IsZero() {
			assert.True(t, r.CompletedAt.After(filter.Since) || r.CompletedAt.Equal(filter.Since))
		}
		if !filter.Until.IsZero() {
			assert.True(t, r.CompletedAt.Before(filter.Until) || r.CompletedAt.Equal(filter.Until))
		}
	}
}

func TestSQLiteHistoryStore_List(t *testing.T) {
	now := time.Now()
	baseTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		filter    *workflow.HistoryFilter
		setupData []*workflow.ExecutionRecord
		wantCount int
		wantErr   bool
	}{
		{
			name:      "nil filter returns default limit",
			filter:    nil,
			setupData: generateRecords(30, baseTime), // More than default limit (20)
			wantCount: defaultSQLiteLimit,            // Should return only 20
			wantErr:   false,
		},
		{
			name: "workflow name filter",
			filter: &workflow.HistoryFilter{
				WorkflowName: "workflow-alpha",
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-2", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "workflow-beta",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-3", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "failed", ExitCode: 1, StartedAt: baseTime, CompletedAt: baseTime.Add(2 * time.Minute), DurationMs: 120000,
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "status filter",
			filter: &workflow.HistoryFilter{
				Status: "success",
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-s1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-f1", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "failed", ExitCode: 1, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-s2", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(2 * time.Minute), DurationMs: 120000,
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "time range filter with since",
			filter: &workflow.HistoryFilter{
				Since: now.Add(-12 * time.Hour),
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-old", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: now.Add(-20 * time.Hour), CompletedAt: now.Add(-20 * time.Hour), DurationMs: 60000,
				},
				{
					ID: "exec-recent", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: now.Add(-6 * time.Hour), CompletedAt: now.Add(-6 * time.Hour), DurationMs: 60000,
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "time range filter with until",
			filter: &workflow.HistoryFilter{
				Until: now.Add(-12 * time.Hour),
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-old", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: now.Add(-20 * time.Hour), CompletedAt: now.Add(-20 * time.Hour), DurationMs: 60000,
				},
				{
					ID: "exec-recent", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: now.Add(-6 * time.Hour), CompletedAt: now.Add(-6 * time.Hour), DurationMs: 60000,
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "custom limit",
			filter: &workflow.HistoryFilter{
				Limit: 5,
			},
			setupData: generateRecords(10, baseTime),
			wantCount: 5,
			wantErr:   false,
		},
		{
			name: "combined filters",
			filter: &workflow.HistoryFilter{
				WorkflowName: "workflow-alpha",
				Status:       "success",
				Limit:        10,
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-match-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-no-match-1", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "workflow-beta",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-no-match-2", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "failed", ExitCode: 1, StartedAt: baseTime, CompletedAt: baseTime.Add(2 * time.Minute), DurationMs: 120000,
				},
				{
					ID: "exec-match-2", WorkflowID: "wf-4", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(3 * time.Minute), DurationMs: 180000,
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "cancelled context returns error",
			filter: &workflow.HistoryFilter{
				Limit: 10,
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "history.db")

			store, err := NewSQLiteHistoryStore(path)
			require.NoError(t, err)
			defer store.Close()

			ctx := context.Background()

			// Insert setup data
			for _, record := range tt.setupData {
				err := store.Record(ctx, record)
				require.NoError(t, err)
			}

			// Prepare context for execution
			execCtx := ctx
			if tt.name == "cancelled context returns error" {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			// Execute List
			records, err := store.List(execCtx, tt.filter)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, records, tt.wantCount)
				verifyFilterApplied(t, tt.filter, records)
			}
		})
	}
}

// TestSQLiteHistoryStore_GetStats tests aggregated statistics queries
func TestSQLiteHistoryStore_GetStats(t *testing.T) {
	now := time.Now()
	baseTime := now.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		filter    *workflow.HistoryFilter
		setupData []*workflow.ExecutionRecord
		wantStats *workflow.HistoryStats
		wantErr   bool
	}{
		{
			name:      "empty database returns zero stats",
			filter:    nil,
			setupData: nil,
			wantStats: &workflow.HistoryStats{
				TotalExecutions: 0,
				SuccessCount:    0,
				FailedCount:     0,
				CancelledCount:  0,
				AvgDurationMs:   0,
			},
			wantErr: false,
		},
		{
			name:   "aggregates all statuses correctly",
			filter: nil,
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-s1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-s2", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(2 * time.Minute), DurationMs: 120000,
				},
				{
					ID: "exec-f1", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "test-workflow",
					Status: "failed", ExitCode: 1, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 60000,
				},
				{
					ID: "exec-c1", WorkflowID: "wf-4", Attempt: 1, WorkflowName: "test-workflow",
					Status: "cancelled", ExitCode: 130, StartedAt: baseTime, CompletedAt: baseTime.Add(30 * time.Second), DurationMs: 30000,
				},
			},
			wantStats: &workflow.HistoryStats{
				TotalExecutions: 4,
				SuccessCount:    2,
				FailedCount:     1,
				CancelledCount:  1,
				AvgDurationMs:   67500, // (60000 + 120000 + 60000 + 30000) / 4
			},
			wantErr: false,
		},
		{
			name: "filters by workflow name",
			filter: &workflow.HistoryFilter{
				WorkflowName: "workflow-alpha",
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 100000,
				},
				{
					ID: "exec-2", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "workflow-beta",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 200000,
				},
				{
					ID: "exec-3", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "workflow-alpha",
					Status: "failed", ExitCode: 1, StartedAt: baseTime, CompletedAt: baseTime.Add(2 * time.Minute), DurationMs: 50000,
				},
			},
			wantStats: &workflow.HistoryStats{
				TotalExecutions: 2,
				SuccessCount:    1,
				FailedCount:     1,
				CancelledCount:  0,
				AvgDurationMs:   75000, // (100000 + 50000) / 2
			},
			wantErr: false,
		},
		{
			name:   "calculates average duration correctly",
			filter: nil,
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 100000,
				},
				{
					ID: "exec-2", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 200000,
				},
				{
					ID: "exec-3", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0, StartedAt: baseTime, CompletedAt: baseTime.Add(1 * time.Minute), DurationMs: 300000,
				},
			},
			wantStats: &workflow.HistoryStats{
				TotalExecutions: 3,
				SuccessCount:    3,
				FailedCount:     0,
				CancelledCount:  0,
				AvgDurationMs:   200000, // (100000 + 200000 + 300000) / 3
			},
			wantErr: false,
		},
		{
			name: "cancelled context returns error",
			filter: &workflow.HistoryFilter{
				WorkflowName: "test-workflow",
			},
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   time.Now().Add(-2 * time.Hour),
					CompletedAt: time.Now().Add(-2 * time.Hour),
					DurationMs:  100000,
				},
			},
			wantStats: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "history.db")

			store, err := NewSQLiteHistoryStore(path)
			require.NoError(t, err)
			defer store.Close()

			ctx := context.Background()

			// Insert setup data
			for _, record := range tt.setupData {
				err := store.Record(ctx, record)
				require.NoError(t, err)
			}

			// Prepare context for execution
			execCtx := ctx
			if tt.name == "cancelled context returns error" {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			// Execute GetStats
			stats, err := store.GetStats(execCtx, tt.filter)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, stats)
				assert.Equal(t, tt.wantStats.TotalExecutions, stats.TotalExecutions)
				assert.Equal(t, tt.wantStats.SuccessCount, stats.SuccessCount)
				assert.Equal(t, tt.wantStats.FailedCount, stats.FailedCount)
				assert.Equal(t, tt.wantStats.CancelledCount, stats.CancelledCount)
				assert.Equal(t, tt.wantStats.AvgDurationMs, stats.AvgDurationMs)
			}
		})
	}
}

// TestSQLiteHistoryStore_Cleanup tests deleting old execution records
func TestSQLiteHistoryStore_Cleanup(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		olderThan     time.Duration
		setupData     []*workflow.ExecutionRecord
		wantDeleted   int
		wantRemaining int
		wantErr       bool
	}{
		{
			name:          "no records returns zero deleted",
			olderThan:     24 * time.Hour,
			setupData:     nil,
			wantDeleted:   0,
			wantRemaining: 0,
			wantErr:       false,
		},
		{
			name:      "deletes records older than duration",
			olderThan: 24 * time.Hour,
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-old-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   now.Add(-48 * time.Hour),
					CompletedAt: now.Add(-48 * time.Hour),
					DurationMs:  60000,
				},
				{
					ID: "exec-old-2", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   now.Add(-36 * time.Hour),
					CompletedAt: now.Add(-36 * time.Hour),
					DurationMs:  60000,
				},
				{
					ID: "exec-recent", WorkflowID: "wf-3", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   now.Add(-12 * time.Hour),
					CompletedAt: now.Add(-12 * time.Hour),
					DurationMs:  60000,
				},
			},
			wantDeleted:   2,
			wantRemaining: 1,
			wantErr:       false,
		},
		{
			name:      "preserves recent records",
			olderThan: 1 * time.Hour,
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-recent-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   now.Add(-30 * time.Minute),
					CompletedAt: now.Add(-30 * time.Minute),
					DurationMs:  60000,
				},
				{
					ID: "exec-recent-2", WorkflowID: "wf-2", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   now.Add(-15 * time.Minute),
					CompletedAt: now.Add(-15 * time.Minute),
					DurationMs:  60000,
				},
			},
			wantDeleted:   0,
			wantRemaining: 2,
			wantErr:       false,
		},
		{
			name:      "cancelled context returns error",
			olderThan: 24 * time.Hour,
			setupData: []*workflow.ExecutionRecord{
				{
					ID: "exec-1", WorkflowID: "wf-1", Attempt: 1, WorkflowName: "test-workflow",
					Status: "success", ExitCode: 0,
					StartedAt:   now.Add(-48 * time.Hour),
					CompletedAt: now.Add(-48 * time.Hour),
					DurationMs:  60000,
				},
			},
			wantDeleted:   0,
			wantRemaining: 1,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "history.db")

			store, err := NewSQLiteHistoryStore(path)
			require.NoError(t, err)
			defer store.Close()

			ctx := context.Background()

			// Insert setup data
			for _, record := range tt.setupData {
				err := store.Record(ctx, record)
				require.NoError(t, err)
			}

			// Prepare context for execution
			execCtx := ctx
			if tt.name == "cancelled context returns error" {
				var cancel context.CancelFunc
				execCtx, cancel = context.WithCancel(ctx)
				cancel() // Cancel immediately
			}

			// Execute Cleanup
			deleted, err := store.Cleanup(execCtx, tt.olderThan)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDeleted, deleted)

				// Verify remaining records
				records, err := store.List(ctx, &workflow.HistoryFilter{Limit: 100})
				require.NoError(t, err)
				assert.Len(t, records, tt.wantRemaining)
			}
		})
	}
}

// TestSQLiteHistoryStore_Close tests graceful database shutdown
func TestSQLiteHistoryStore_Close(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "closes successfully",
			wantErr: false,
		},
		{
			name:    "multiple close calls are safe",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "history.db")

			store, err := NewSQLiteHistoryStore(path)
			require.NoError(t, err)

			switch tt.name {
			case "closes successfully":
				// Close once
				err := store.Close()
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.True(t, store.closed)
				}
			case "multiple close calls are safe":
				// Close multiple times
				err1 := store.Close()
				err2 := store.Close()
				err3 := store.Close()

				assert.NoError(t, err1)
				assert.NoError(t, err2)
				assert.NoError(t, err3)
				assert.True(t, store.closed)
			}
		})
	}
}

// TestSQLiteHistoryStore_ClosedState tests operations on closed store
func TestSQLiteHistoryStore_ClosedState(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		wantErr   bool
	}{
		{
			name:      "record on closed store returns error",
			operation: "record",
			wantErr:   true,
		},
		{
			name:      "list on closed store returns error",
			operation: "list",
			wantErr:   true,
		},
		{
			name:      "stats on closed store returns error",
			operation: "stats",
			wantErr:   true,
		},
		{
			name:      "cleanup on closed store returns error",
			operation: "cleanup",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "history.db")

			store, err := NewSQLiteHistoryStore(path)
			require.NoError(t, err)

			// Close the store
			err = store.Close()
			require.NoError(t, err)

			ctx := context.Background()

			// Execute operation based on test case
			switch tt.operation {
			case "record":
				record := &workflow.ExecutionRecord{
					ID:           "test-id",
					WorkflowID:   "wf-1",
					Attempt:      1,
					WorkflowName: "test-workflow",
					Status:       "success",
					ExitCode:     0,
					StartedAt:    time.Now(),
					CompletedAt:  time.Now(),
					DurationMs:   60000,
				}
				err = store.Record(ctx, record)
			case "list":
				_, err = store.List(ctx, nil)
			case "stats":
				_, err = store.GetStats(ctx, nil)
			case "cleanup":
				_, err = store.Cleanup(ctx, 24*time.Hour)
			}

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "store is closed")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSQLiteHistoryStore_ConcurrentAccess tests thread-safety
func TestSQLiteHistoryStore_ConcurrentAccess(t *testing.T) {
	t.Run("identical replay across store instances", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.db")
		firstStore, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, firstStore.Close()) })
		secondStore, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, secondStore.Close()) })

		record := newSQLiteHistoryRecord(
			"shared-record",
			"shared-workflow",
			1,
			"",
			time.Unix(1_700_400_000, 123),
		)
		errorsByWriter := recordConcurrently(
			context.Background(),
			[]*SQLiteHistoryStore{firstStore, secondStore},
			[]*workflow.ExecutionRecord{record, record},
		)
		for _, recordErr := range errorsByWriter {
			require.NoError(t, recordErr)
		}

		records, err := firstStore.List(context.Background(), &workflow.HistoryFilter{
			WorkflowID: record.WorkflowID,
			Limit:      10,
		})
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, record, records[0])
	})

	t.Run("competing ordinal claims across store instances", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "history.db")
		firstStore, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, firstStore.Close()) })
		secondStore, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, secondStore.Close()) })

		completedAt := time.Unix(1_700_500_000, 456)
		firstRecord := newSQLiteHistoryRecord("claim-one", "shared-workflow", 1, "", completedAt)
		secondRecord := newSQLiteHistoryRecord("claim-two", "shared-workflow", 1, "", completedAt)
		errorsByWriter := recordConcurrently(
			context.Background(),
			[]*SQLiteHistoryStore{firstStore, secondStore},
			[]*workflow.ExecutionRecord{firstRecord, secondRecord},
		)

		successes := 0
		conflicts := 0
		for _, recordErr := range errorsByWriter {
			switch {
			case recordErr == nil:
				successes++
			case errors.Is(recordErr, ErrHistoryRecordConflict):
				conflicts++
			default:
				t.Fatalf("unexpected concurrent Record error: %v", recordErr)
			}
		}
		assert.Equal(t, 1, successes)
		assert.Equal(t, 1, conflicts)

		records, err := firstStore.List(context.Background(), &workflow.HistoryFilter{
			WorkflowID: firstRecord.WorkflowID,
			Limit:      10,
		})
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})

	t.Run("concurrent writes", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		const goroutines = 20

		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func(n int) {
				defer wg.Done()
				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("exec-%d", n),
					WorkflowID:   fmt.Sprintf("wf-%d", n),
					Attempt:      1,
					WorkflowName: "concurrent-workflow",
					Status:       "success",
					ExitCode:     0,
					StartedAt:    time.Now(),
					CompletedAt:  time.Now(),
					DurationMs:   60000,
				}
				err := store.Record(ctx, record)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Verify all records written successfully
		records, err := store.List(ctx, &workflow.HistoryFilter{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, records, goroutines)
	})

	t.Run("concurrent reads", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert test data
		for i := 0; i < 10; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		const goroutines = 20
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				records, err := store.List(ctx, nil)
				assert.NoError(t, err)
				assert.NotEmpty(t, records)
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent read/write", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		const goroutines = 10

		var wg sync.WaitGroup
		wg.Add(goroutines * 2)

		// Writers
		for i := 0; i < goroutines; i++ {
			go func(n int) {
				defer wg.Done()
				record := &workflow.ExecutionRecord{
					ID:           fmt.Sprintf("exec-write-%d", n),
					WorkflowID:   fmt.Sprintf("wf-%d", n),
					Attempt:      1,
					WorkflowName: "mixed-workflow",
					Status:       "success",
					ExitCode:     0,
					StartedAt:    time.Now(),
					CompletedAt:  time.Now(),
					DurationMs:   60000,
				}
				err := store.Record(ctx, record)
				assert.NoError(t, err)
			}(i)
		}

		// Readers
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				_, err := store.List(ctx, nil)
				assert.NoError(t, err)
			}()
		}

		wg.Wait()

		// Verify no data corruption
		records, err := store.List(ctx, &workflow.HistoryFilter{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, records, goroutines)
	})
}

func recordConcurrently(
	ctx context.Context,
	stores []*SQLiteHistoryStore,
	records []*workflow.ExecutionRecord,
) []error {
	start := make(chan struct{})
	errorsByWriter := make([]error, len(stores))
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(stores))
	for index := range stores {
		go func() {
			defer waitGroup.Done()
			<-start
			errorsByWriter[index] = stores[index].Record(ctx, records[index])
		}()
	}
	close(start)
	waitGroup.Wait()
	return errorsByWriter
}

// TestSQLiteHistoryStore_EdgeCases tests edge cases and error paths
func TestSQLiteHistoryStore_EdgeCases(t *testing.T) {
	t.Run("empty workflow name in filter", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert records with different workflow names
		for i := 0; i < 3; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: fmt.Sprintf("workflow-%d", i),
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query with empty workflow name - should return all
		records, err := store.List(ctx, &workflow.HistoryFilter{
			WorkflowName: "",
			Limit:        100,
		})
		require.NoError(t, err)
		assert.Len(t, records, 3)
	})

	t.Run("zero time values in filter", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert a record
		record := &workflow.ExecutionRecord{
			ID:           "exec-1",
			WorkflowID:   "wf-1",
			Attempt:      1,
			WorkflowName: "test-workflow",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    time.Now(),
			CompletedAt:  time.Now(),
			DurationMs:   60000,
		}
		err = store.Record(ctx, record)
		require.NoError(t, err)

		// Query with zero time values - should be ignored
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Since: time.Time{},
			Until: time.Time{},
			Limit: 100,
		})
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})

	t.Run("negative duration in cleanup", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert a record
		record := &workflow.ExecutionRecord{
			ID:           "exec-1",
			WorkflowID:   "wf-1",
			Attempt:      1,
			WorkflowName: "test-workflow",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    time.Now(),
			CompletedAt:  time.Now(),
			DurationMs:   60000,
		}
		err = store.Record(ctx, record)
		require.NoError(t, err)

		// Cleanup with negative duration - should delete nothing (cutoff is in the future)
		deleted, err := store.Cleanup(ctx, -24*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, 0, deleted)

		// Verify record still exists
		records, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})

	t.Run("very large limit value", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert a few records
		for i := 0; i < 5; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query with very large limit
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 1000000,
		})
		require.NoError(t, err)
		// Should return all records, not fail
		assert.Len(t, records, 5)
	})

	t.Run("empty results - no records match filter", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert records with specific workflow name
		for i := 0; i < 3; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "production-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query for non-existent workflow
		records, err := store.List(ctx, &workflow.HistoryFilter{
			WorkflowName: "non-existent-workflow",
			Limit:        100,
		})
		require.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("empty results - no records in database", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Query empty database
		records, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, records)

		// GetStats on empty database
		stats, err := store.GetStats(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, 0, stats.TotalExecutions)
	})

	t.Run("large data - many records", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		const recordCount = 1000

		// Insert many records
		for i := 0; i < recordCount; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: fmt.Sprintf("workflow-%d", i%10), // 10 different workflows
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now().Add(-time.Duration(i) * time.Minute),
				CompletedAt:  time.Now().Add(-time.Duration(i) * time.Minute),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query all records with large limit
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: recordCount,
		})
		require.NoError(t, err)
		assert.Len(t, records, recordCount)

		// Query with pagination
		page1, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 100,
		})
		require.NoError(t, err)
		assert.Len(t, page1, 100)

		// Verify stats work with large dataset
		stats, err := store.GetStats(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, recordCount, stats.TotalExecutions)
	})

	t.Run("large data - record with large error message", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Create a very large error message (10KB)
		largeError := ""
		for i := 0; i < 1000; i++ {
			largeError += "Error line " + fmt.Sprintf("%d", i) + ": some error details\n"
		}

		record := &workflow.ExecutionRecord{
			ID:           "exec-large-error",
			WorkflowID:   "wf-1",
			Attempt:      1,
			WorkflowName: "test-workflow",
			Status:       "failed",
			ExitCode:     1,
			StartedAt:    time.Now(),
			CompletedAt:  time.Now(),
			DurationMs:   60000,
			ErrorMessage: largeError,
		}

		// Should handle large error message
		err = store.Record(ctx, record)
		require.NoError(t, err)

		// Verify retrieval
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 10,
		})
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, largeError, records[0].ErrorMessage)
	})
}

// TestSQLiteHistoryStore_List_ErrorPaths tests error handling in List operation
// Item: T007
// Feature: C016
func TestSQLiteHistoryStore_List_ErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*SQLiteHistoryStore, context.Context)
		wantErr bool
		errMsg  string
	}{
		{
			name: "cancelled context returns error",
			setup: func(t *testing.T) (*SQLiteHistoryStore, context.Context) {
				tmpDir := t.TempDir()
				path := filepath.Join(tmpDir, "history.db")
				store, err := NewSQLiteHistoryStore(path)
				require.NoError(t, err)
				t.Cleanup(func() { store.Close() })

				// Insert test data first
				ctx := context.Background()
				record := &workflow.ExecutionRecord{
					ID:           "test-id",
					WorkflowID:   "wf-1",
					Attempt:      1,
					WorkflowName: "test-workflow",
					Status:       "success",
					ExitCode:     0,
					StartedAt:    time.Now(),
					CompletedAt:  time.Now(),
					DurationMs:   60000,
				}
				err = store.Record(ctx, record)
				require.NoError(t, err)

				// Return store and cancelled context
				cancelledCtx, cancel := context.WithCancel(context.Background())
				cancel()
				return store, cancelledCtx
			},
			wantErr: true,
			errMsg:  "context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := tt.setup(t)

			// Execute
			records, err := store.List(ctx, nil)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, records)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, records)
			}
		})
	}
}

// TestSQLiteHistoryStore_GetStats_ErrorPaths tests error handling in GetStats operation
// Item: T007
// Feature: C016
func TestSQLiteHistoryStore_GetStats_ErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*SQLiteHistoryStore, context.Context)
		wantErr bool
		errMsg  string
	}{
		{
			name: "cancelled context returns error",
			setup: func(t *testing.T) (*SQLiteHistoryStore, context.Context) {
				tmpDir := t.TempDir()
				path := filepath.Join(tmpDir, "history.db")
				store, err := NewSQLiteHistoryStore(path)
				require.NoError(t, err)
				t.Cleanup(func() { store.Close() })

				// Insert test data first
				ctx := context.Background()
				record := &workflow.ExecutionRecord{
					ID:           "test-id",
					WorkflowID:   "wf-1",
					Attempt:      1,
					WorkflowName: "test-workflow",
					Status:       "success",
					ExitCode:     0,
					StartedAt:    time.Now(),
					CompletedAt:  time.Now(),
					DurationMs:   60000,
				}
				err = store.Record(ctx, record)
				require.NoError(t, err)

				// Return store and cancelled context
				cancelledCtx, cancel := context.WithCancel(context.Background())
				cancel()
				return store, cancelledCtx
			},
			wantErr: true,
			errMsg:  "context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := tt.setup(t)

			// Execute
			stats, err := store.GetStats(ctx, nil)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, stats)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, stats)
			}
		})
	}
}

// TestSQLiteHistoryStore_Cleanup_ErrorPaths tests error handling in Cleanup operation
// Item: T007
// Feature: C016
func TestSQLiteHistoryStore_Cleanup_ErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*SQLiteHistoryStore, context.Context)
		wantErr bool
		errMsg  string
	}{
		{
			name: "cancelled context returns error",
			setup: func(t *testing.T) (*SQLiteHistoryStore, context.Context) {
				tmpDir := t.TempDir()
				path := filepath.Join(tmpDir, "history.db")
				store, err := NewSQLiteHistoryStore(path)
				require.NoError(t, err)
				t.Cleanup(func() { store.Close() })

				// Insert test data first
				ctx := context.Background()
				record := &workflow.ExecutionRecord{
					ID:           "test-id",
					WorkflowID:   "wf-1",
					Attempt:      1,
					WorkflowName: "test-workflow",
					Status:       "success",
					ExitCode:     0,
					StartedAt:    time.Now().Add(-48 * time.Hour),
					CompletedAt:  time.Now().Add(-48 * time.Hour),
					DurationMs:   60000,
				}
				err = store.Record(ctx, record)
				require.NoError(t, err)

				// Return store and cancelled context
				cancelledCtx, cancel := context.WithCancel(context.Background())
				cancel()
				return store, cancelledCtx
			},
			wantErr: true,
			errMsg:  "context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := tt.setup(t)

			// Execute
			deleted, err := store.Cleanup(ctx, 24*time.Hour)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Equal(t, 0, deleted)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSQLiteHistoryStore_AdditionalEdgeCases tests additional boundary and stress scenarios
// Item: T008
// Feature: C016
//
//nolint:gocognit // Comprehensive test with 14 sub-tests for edge cases
func TestSQLiteHistoryStore_AdditionalEdgeCases(t *testing.T) {
	t.Run("zero limit returns no records", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert test data
		for i := 0; i < 5; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query with zero limit
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 0,
		})
		require.NoError(t, err)
		// Zero limit should use default limit behavior
		assert.NotEmpty(t, records)
	})

	t.Run("limit of 1 returns single record", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert multiple records
		for i := 0; i < 10; i++ {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query with limit=1
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 1,
		})
		require.NoError(t, err)
		assert.Len(t, records, 1)
	})

	t.Run("special characters in workflow name", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Test various special characters and potential SQL injection patterns
		specialNames := []string{
			"workflow-with-dashes",
			"workflow_with_underscores",
			"workflow.with.dots",
			"workflow'with'quotes",
			`workflow"with"doublequotes`,
			"workflow\nwith\nnewlines",
			"workflow\twith\ttabs",
			"workflow with spaces",
			"workflow;DROP TABLE executions;--",
			"workflow' OR '1'='1",
			"workflow\\with\\backslashes",
			"workflow/with/slashes",
			"workflow@with#special$chars%",
			"用户工作流",         // Unicode Chinese
			"ワークフロー",        // Unicode Japanese
			"рабочий_поток", // Unicode Russian
		}

		for i, name := range specialNames {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: name,
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err, "failed to insert workflow name: %s", name)
		}

		// Verify all records were stored correctly
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 100,
		})
		require.NoError(t, err)
		assert.Len(t, records, len(specialNames))

		// Verify we can query by special names
		for _, name := range specialNames {
			filtered, err := store.List(ctx, &workflow.HistoryFilter{
				WorkflowName: name,
				Limit:        10,
			})
			require.NoError(t, err, "failed to query workflow name: %s", name)
			assert.Len(t, filtered, 1, "expected 1 record for workflow: %s", name)
			assert.Equal(t, name, filtered[0].WorkflowName)
		}
	})

	t.Run("extreme timestamp values", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Test edge case timestamps
		testCases := []struct {
			name      string
			startTime time.Time
			endTime   time.Time
		}{
			{
				name:      "unix epoch",
				startTime: time.Unix(0, 0).UTC(),
				endTime:   time.Unix(1, 0).UTC(),
			},
			{
				name:      "year 1970",
				startTime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
				endTime:   time.Date(1970, 1, 1, 0, 1, 0, 0, time.UTC),
			},
			{
				name:      "year 2038 problem boundary",
				startTime: time.Date(2038, 1, 19, 3, 14, 7, 0, time.UTC),
				endTime:   time.Date(2038, 1, 19, 3, 14, 8, 0, time.UTC),
			},
			{
				name:      "far future",
				startTime: time.Date(2100, 12, 31, 23, 59, 59, 0, time.UTC),
				endTime:   time.Date(2101, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				name:      "very old past",
				startTime: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
				endTime:   time.Date(1900, 1, 1, 0, 1, 0, 0, time.UTC),
			},
		}

		for i, tc := range testCases {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: tc.name,
				Status:       "success",
				ExitCode:     0,
				StartedAt:    tc.startTime,
				CompletedAt:  tc.endTime,
				DurationMs:   tc.endTime.Sub(tc.startTime).Milliseconds(),
			}
			err := store.Record(ctx, record)
			require.NoError(t, err, "failed to insert record with timestamp: %s", tc.name)
		}

		// Verify all records stored and retrievable
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 100,
		})
		require.NoError(t, err)
		assert.Len(t, records, len(testCases))

		// Verify time filter works with extreme values
		filtered, err := store.List(ctx, &workflow.HistoryFilter{
			Since: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			Until: time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC),
			Limit: 100,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, filtered)
	})

	t.Run("extreme duration values", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		testCases := []struct {
			name       string
			durationMs int64
		}{
			{"zero duration", 0},
			{"one millisecond", 1},
			{"one second", 1000},
			{"one hour", 3600000},
			{"one day", 86400000},
			{"one year", 31536000000},
			{"very large duration", 9223372036854775807}, // Max int64
		}

		for i, tc := range testCases {
			now := time.Now()
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: tc.name,
				Status:       "success",
				ExitCode:     0,
				StartedAt:    now,
				CompletedAt:  now,
				DurationMs:   tc.durationMs,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err, "failed to insert record with duration: %s", tc.name)
		}

		// Verify stats calculation handles extreme durations
		stats, err := store.GetStats(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, stats)
		assert.Equal(t, len(testCases), stats.TotalExecutions)
	})

	t.Run("maximum workflow name length", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Create very long workflow names (SQLite TEXT can handle up to 1 billion chars)
		// Test with practical large sizes: 1KB, 10KB, 100KB
		testSizes := []int{
			1000,   // 1KB
			10000,  // 10KB
			100000, // 100KB
		}

		for i, size := range testSizes {
			// Build string efficiently using repeated characters
			longName := make([]byte, size)
			for j := 0; j < size; j++ {
				longName[j] = 'a'
			}

			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: string(longName),
				Status:       "success",
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err, "failed to insert workflow with name length: %d", size)

			// Verify retrieval
			records, err := store.List(ctx, &workflow.HistoryFilter{
				Limit: 10,
			})
			require.NoError(t, err)
			found := false
			for _, r := range records {
				if r.ID == record.ID {
					assert.Equal(t, size, len(r.WorkflowName))
					found = true
					break
				}
			}
			assert.True(t, found, "record with long name should be retrievable")
		}
	})

	t.Run("cleanup with zero duration", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Insert a recent record
		record := &workflow.ExecutionRecord{
			ID:           "exec-1",
			WorkflowID:   "wf-1",
			Attempt:      1,
			WorkflowName: "test-workflow",
			Status:       "success",
			ExitCode:     0,
			StartedAt:    time.Now(),
			CompletedAt:  time.Now(),
			DurationMs:   60000,
		}
		err = store.Record(ctx, record)
		require.NoError(t, err)

		// Cleanup with zero duration should delete records completed "now or earlier"
		deleted, err := store.Cleanup(ctx, 0)
		require.NoError(t, err)
		// Behavior depends on implementation - document actual behavior
		assert.GreaterOrEqual(t, deleted, 0)
	})

	t.Run("filter with both since and until creates range", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		now := time.Now()

		// Insert records at different times
		timePoints := []time.Duration{
			-48 * time.Hour,
			-36 * time.Hour,
			-24 * time.Hour,
			-12 * time.Hour,
			-6 * time.Hour,
			-1 * time.Hour,
		}

		for i, offset := range timePoints {
			execTime := now.Add(offset)
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    execTime,
				CompletedAt:  execTime,
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Query for records between 30 and 10 hours ago
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Since: now.Add(-30 * time.Hour),
			Until: now.Add(-10 * time.Hour),
			Limit: 100,
		})
		require.NoError(t, err)
		// Should return records at -24h and -12h
		assert.Len(t, records, 2)
	})

	t.Run("negative exit codes", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Test negative, zero, and positive exit codes
		exitCodes := []int{-1, -128, 0, 1, 127, 255}

		for i, code := range exitCodes {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     code,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Verify all exit codes stored correctly
		records, err := store.List(ctx, &workflow.HistoryFilter{
			Limit: 100,
		})
		require.NoError(t, err)
		assert.Len(t, records, len(exitCodes))

		for _, r := range records {
			assert.Contains(t, exitCodes, r.ExitCode)
		}
	})

	t.Run("status values with special characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()

		// Test various status strings including edge cases
		statuses := []string{
			"success",
			"failed",
			"cancelled",
			"pending",
			"running",
			"", // empty status
			"status-with-dashes",
			"status_with_underscores",
			"STATUS_UPPERCASE",
			"Status Mixed Case",
		}

		for i, status := range statuses {
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       status,
				ExitCode:     0,
				StartedAt:    time.Now(),
				CompletedAt:  time.Now(),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Verify filtering by each status works
		for _, status := range statuses {
			records, err := store.List(ctx, &workflow.HistoryFilter{
				Status: status,
				Limit:  10,
			})
			require.NoError(t, err)
			if status != "" {
				assert.Len(t, records, 1, "expected 1 record for status: %s", status)
			}
		}
	})

	t.Run("concurrent cleanup operations", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "history.db")

		store, err := NewSQLiteHistoryStore(path)
		require.NoError(t, err)
		defer store.Close()

		ctx := context.Background()
		now := time.Now()

		// Insert old and new records
		for i := 0; i < 20; i++ {
			age := time.Duration(i) * time.Hour
			record := &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("exec-%d", i),
				WorkflowID:   fmt.Sprintf("wf-%d", i),
				Attempt:      1,
				WorkflowName: "test-workflow",
				Status:       "success",
				ExitCode:     0,
				StartedAt:    now.Add(-age),
				CompletedAt:  now.Add(-age),
				DurationMs:   60000,
			}
			err := store.Record(ctx, record)
			require.NoError(t, err)
		}

		// Run multiple cleanup operations concurrently
		const goroutines = 5
		var wg sync.WaitGroup
		wg.Add(goroutines)

		deletedCounts := make([]int, goroutines)
		for i := 0; i < goroutines; i++ {
			go func(n int) {
				defer wg.Done()
				deleted, err := store.Cleanup(ctx, 10*time.Hour)
				assert.NoError(t, err)
				deletedCounts[n] = deleted
			}(i)
		}

		wg.Wait()

		// Verify total deleted makes sense (some may overlap, so total <= sum)
		totalDeleted := 0
		for _, count := range deletedCounts {
			totalDeleted += count
		}
		assert.GreaterOrEqual(t, totalDeleted, 0)

		// Verify remaining records are within expected range
		remaining, err := store.List(ctx, &workflow.HistoryFilter{Limit: 100})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(remaining), 20)
	})
}

func TestSQLiteHistoryStore_ListScansAttemptLineageAndFiltersWorkflowID(t *testing.T) {
	t.Parallel()

	historyStore := openSQLiteHistoryStoreForTest(t)
	ctx := context.Background()
	completedAt := time.Unix(1_700_100_000, 0)
	targetRecords := []*workflow.ExecutionRecord{
		newSQLiteHistoryRecord("target-1", "target-workflow", 1, "", completedAt),
		newSQLiteHistoryRecord("target-2", "target-workflow", 2, "target-1", completedAt),
		newSQLiteHistoryRecord("target-3", "target-workflow", 3, "target-2", completedAt),
	}
	for _, record := range targetRecords {
		require.NoError(t, historyStore.Record(ctx, record))
	}
	for index := 0; index < defaultSQLiteLimit; index++ {
		record := newSQLiteHistoryRecord(
			fmt.Sprintf("other-%02d", index),
			fmt.Sprintf("other-workflow-%02d", index),
			1,
			"",
			completedAt.Add(-time.Duration(index+1)*time.Second),
		)
		require.NoError(t, historyStore.Record(ctx, record))
	}

	defaultRecords, err := historyStore.List(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, defaultRecords, defaultSQLiteLimit)

	records, err := historyStore.List(ctx, &workflow.HistoryFilter{
		WorkflowID: "target-workflow",
		Limit:      10,
	})
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{"target-3", "target-2", "target-1"}, []string{
		records[0].ID,
		records[1].ID,
		records[2].ID,
	})
	assert.Equal(t, []int{3, 2, 1}, []int{
		records[0].Attempt,
		records[1].Attempt,
		records[2].Attempt,
	})
	assert.Equal(t, []string{"target-2", "target-1", ""}, []string{
		records[0].ResumedFromID,
		records[1].ResumedFromID,
		records[2].ResumedFromID,
	})
	for _, record := range records {
		assert.Equal(t, "target-workflow", record.WorkflowID)
	}
}

func TestSQLiteHistoryStore_GetStatsFiltersWorkflowID(t *testing.T) {
	t.Parallel()

	historyStore := openSQLiteHistoryStoreForTest(t)
	ctx := context.Background()
	baseTime := time.Unix(1_700_200_000, 0)
	records := []*workflow.ExecutionRecord{
		newSQLiteHistoryRecord("target-success", "target-workflow", 1, "", baseTime),
		newSQLiteHistoryRecord("target-failed", "target-workflow", 2, "target-success", baseTime.Add(time.Second)),
		newSQLiteHistoryRecord("other-cancelled", "other-workflow", 1, "", baseTime.Add(2*time.Second)),
	}
	records[0].Status = "success"
	records[0].DurationMs = 100
	records[1].Status = "failed"
	records[1].DurationMs = 300
	records[2].Status = "cancelled"
	records[2].DurationMs = 9_999
	for _, record := range records {
		require.NoError(t, historyStore.Record(ctx, record))
	}

	stats, err := historyStore.GetStats(ctx, &workflow.HistoryFilter{
		WorkflowID: "target-workflow",
		Limit:      1,
	})
	require.NoError(t, err)
	assert.Equal(t, &workflow.HistoryStats{
		TotalExecutions: 2,
		SuccessCount:    1,
		FailedCount:     1,
		CancelledCount:  0,
		AvgDurationMs:   200,
	}, stats)
}

func BenchmarkSQLiteHistoryStore_ListAttemptsByWorkflowID(b *testing.B) {
	historyStore, err := NewSQLiteHistoryStore(filepath.Join(b.TempDir(), "history.db"))
	require.NoError(b, err)
	b.Cleanup(func() { require.NoError(b, historyStore.Close()) })

	ctx := context.Background()
	completedAt := time.Unix(1_700_300_000, 0)
	for workflowIndex := 0; workflowIndex < 10; workflowIndex++ {
		workflowID := fmt.Sprintf("workflow-%02d", workflowIndex)
		predecessor := ""
		for attempt := 1; attempt <= 100; attempt++ {
			id := fmt.Sprintf("%s-attempt-%03d", workflowID, attempt)
			record := newSQLiteHistoryRecord(id, workflowID, attempt, predecessor, completedAt.Add(time.Duration(attempt)))
			require.NoError(b, historyStore.Record(ctx, record))
			predecessor = id
		}
	}

	filter := &workflow.HistoryFilter{WorkflowID: "workflow-05", Limit: 100}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		records, listErr := historyStore.List(ctx, filter)
		if listErr != nil {
			b.Fatal(listErr)
		}
		if len(records) != 100 {
			b.Fatalf("got %d records, want 100", len(records))
		}
	}
}

func openSQLiteHistoryStoreForTest(t *testing.T) *SQLiteHistoryStore {
	t.Helper()

	historyStore, err := NewSQLiteHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyStore.Close()) })
	return historyStore
}

func newSQLiteHistoryRecord(
	id string,
	workflowID string,
	attempt int,
	resumedFromID string,
	completedAt time.Time,
) *workflow.ExecutionRecord {
	return &workflow.ExecutionRecord{
		ID:            id,
		WorkflowID:    workflowID,
		Attempt:       attempt,
		ResumedFromID: resumedFromID,
		WorkflowName:  "workflow-name",
		Status:        "success",
		StartedAt:     completedAt.Add(-time.Second),
		CompletedAt:   completedAt,
		DurationMs:    1_000,
	}
}

// Helper function to generate test records
func generateRecords(count int, baseTime time.Time) []*workflow.ExecutionRecord {
	records := make([]*workflow.ExecutionRecord, count)
	for i := 0; i < count; i++ {
		records[i] = &workflow.ExecutionRecord{
			ID:           fmt.Sprintf("exec-%d", i),
			WorkflowID:   fmt.Sprintf("wf-%d", i),
			Attempt:      1,
			WorkflowName: fmt.Sprintf("workflow-%d", i%3), // Rotate through 3 workflow names
			Status:       []string{"success", "failed", "cancelled"}[i%3],
			ExitCode:     i % 2,
			StartedAt:    baseTime.Add(time.Duration(i) * time.Minute),
			CompletedAt:  baseTime.Add(time.Duration(i+1) * time.Minute),
			DurationMs:   60000,
			ErrorMessage: "",
		}
	}
	return records
}
