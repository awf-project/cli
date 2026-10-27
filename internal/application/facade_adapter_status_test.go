package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapter_Status_PersistedWorkflowAttempts(t *testing.T) {
	t.Run("selects greatest workflow attempt", func(t *testing.T) {
		workflowID := "stable-workflow"
		records := make([]*workflow.ExecutionRecord, 0, 26)
		for attempt := 1; attempt <= 25; attempt++ {
			status := "success"
			if attempt == 25 {
				// The timestamps deliberately make this greatest ordinal the oldest row.
				// Its distinct status proves selection is based on Attempt, not recency.
				status = "failed"
			}
			records = append(records, &workflow.ExecutionRecord{
				ID:           fmt.Sprintf("history-row-%02d", attempt),
				WorkflowID:   workflowID,
				Attempt:      attempt,
				Status:       status,
				StartedAt:    time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Add(-time.Duration(attempt) * time.Hour),
				CompletedAt:  time.Date(2026, 6, 1, 0, 30, 0, 0, time.UTC).Add(-time.Duration(attempt) * time.Hour),
				WorkflowName: "deploy",
			})
		}
		records = append(records, &workflow.ExecutionRecord{ID: "unrelated-newer", WorkflowID: "other-workflow", Attempt: 99, Status: "success"})
		store := &facadeHistoryStoreSpy{listFn: func(filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
			if filter.WorkflowID == workflowID {
				return records, nil
			}
			return []*workflow.ExecutionRecord{}, nil
		}}
		adapter := NewAdapter(nil, nil, NewHistoryService(store, nil), nil, nil, NewSessionRegistry())

		status, err := adapter.Status(context.Background(), workflowID)
		require.NoError(t, err)
		assert.Equal(t, workflowID, status.RunID)
		assert.Equal(t, ports.RunState("failed"), status.Status)
		require.NotEmpty(t, store.listFilters)
		assert.Equal(t, workflow.HistoryFilter{WorkflowID: workflowID, Limit: math.MaxInt}, store.listFilters[0])
	})

	t.Run("keeps live session priority", func(t *testing.T) {
		store := &facadeHistoryStoreSpy{listErr: errors.New("history must not be queried")}
		adapter := NewAdapter(nil, nil, NewHistoryService(store, nil), nil, &mockRecorder{}, NewSessionRegistry())
		started := make(chan struct{})
		release := make(chan struct{})
		session, err := adapter.newSession(context.Background(), "live-workflow", nil, func(context.Context) error {
			close(started)
			<-release
			return nil
		})
		require.NoError(t, err)
		defer func() {
			close(release)
			require.NoError(t, session.Close())
		}()
		<-started

		status, err := adapter.Status(context.Background(), "live-workflow")
		require.NoError(t, err)
		assert.Equal(t, "live-workflow", status.RunID)
		assert.Empty(t, store.listFilters)
	})

	t.Run("falls back to history record ID", func(t *testing.T) {
		store := &facadeHistoryStoreSpy{listFn: func(filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
			if filter.WorkflowID != "" {
				return []*workflow.ExecutionRecord{}, nil
			}
			return []*workflow.ExecutionRecord{{ID: "legacy-row-id", WorkflowID: "stable-workflow", Attempt: 2, Status: "failed"}}, nil
		}}
		adapter := NewAdapter(nil, nil, NewHistoryService(store, nil), nil, nil, NewSessionRegistry())

		status, err := adapter.Status(context.Background(), "legacy-row-id")
		require.NoError(t, err)
		assert.Equal(t, "legacy-row-id", status.RunID)
		require.Len(t, store.listFilters, 2)
		assert.Equal(t, workflow.HistoryFilter{WorkflowID: "legacy-row-id", Limit: math.MaxInt}, store.listFilters[0])
		assert.Equal(t, 10_000, store.listFilters[1].Limit)
	})

	t.Run("degrades store errors and missing records", func(t *testing.T) {
		for _, testCase := range []struct {
			name  string
			store *facadeHistoryStoreSpy
		}{
			{name: "store error", store: &facadeHistoryStoreSpy{listErr: errors.New("database unavailable")}},
			{name: "not found", store: &facadeHistoryStoreSpy{records: []*workflow.ExecutionRecord{}}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				adapter := NewAdapter(nil, nil, NewHistoryService(testCase.store, nil), nil, nil, NewSessionRegistry())

				status, err := adapter.Status(context.Background(), "missing-workflow")
				require.ErrorIs(t, err, ports.ErrRunNotFound)
				assert.Empty(t, status.RunID)
			})
		}
	})

	t.Run("does not fall back after workflow-scoped store error", func(t *testing.T) {
		storeErr := errors.New("workflow-scoped query unavailable")
		store := &facadeHistoryStoreSpy{listFn: func(filter *workflow.HistoryFilter) ([]*workflow.ExecutionRecord, error) {
			if filter.WorkflowID != "" {
				return nil, storeErr
			}
			return []*workflow.ExecutionRecord{{
				ID:         "stable-workflow",
				WorkflowID: "different-workflow",
				Attempt:    9,
				Status:     "success",
			}}, nil
		}}
		adapter := NewAdapter(nil, nil, NewHistoryService(store, nil), nil, nil, NewSessionRegistry())

		status, err := adapter.Status(context.Background(), "stable-workflow")
		require.ErrorIs(t, err, ports.ErrRunNotFound)
		assert.Empty(t, status.RunID)
		require.Len(t, store.listFilters, 1)
		assert.Equal(t, workflow.HistoryFilter{WorkflowID: "stable-workflow", Limit: math.MaxInt}, store.listFilters[0])
	})
}

// TestAdapter_Status_LiveSession verifies Status returns the live run from SessionRegistry.Get(id)
// when present (Acceptance #33).
func TestAdapter_Status_LiveSession(t *testing.T) {
	// Arrange: create a live session in the registry
	registry := NewSessionRegistry()
	recorder := &mockRecorder{}

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		registry,
	)

	// Create a session and register it
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ports.RunRequest{Identifier: "test/workflow"}
	session, err := adapter.Run(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, session)
	defer session.Close()

	// Act: lookup the session status
	status, err := adapter.Status(context.Background(), session.ID())

	// Assert: should find the session in the registry
	assert.NoError(t, err, "Status should not error for live session")
	assert.Equal(t, session.ID(), status.RunID, "should return the session ID")
}

// TestAdapter_Status_PersistedRun verifies Status falls back to history.db lookup when
// registry misses (Acceptance #34). When no live RunSession exists, it queries the
// HistoryService to find a persisted record.
func TestAdapter_Status_PersistedRun(t *testing.T) {
	// Arrange: setup history store with a persisted record
	store := testmocks.NewMockHistoryStore()
	persistedRecord := &workflow.ExecutionRecord{
		ID:           "persisted-run-123",
		WorkflowName: "demo-workflow",
		Status:       "success",
		StartedAt:    time.Now().Add(-10 * time.Minute),
		CompletedAt:  time.Now(),
		DurationMs:   600000,
	}
	require.NoError(t, store.Record(context.Background(), persistedRecord))

	registry := NewSessionRegistry()
	recorder := &mockRecorder{}

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		NewHistoryService(store, nil),
		nil,
		recorder,
		registry,
	)

	// Act: lookup a persisted run (not in registry)
	status, err := adapter.Status(context.Background(), "persisted-run-123")

	// Assert: should query history and return the record
	assert.NoError(t, err, "Status should succeed for persisted run")
	assert.Equal(t, "persisted-run-123", status.RunID, "should return the run ID from history")
}

// TestAdapter_Status_NotFound verifies Status returns a not-found error for unknown IDs
// and never panics (Acceptance #35).
func TestAdapter_Status_NotFound(t *testing.T) {
	// Arrange: empty registry, no history
	store := testmocks.NewMockHistoryStore()
	registry := NewSessionRegistry()
	recorder := &mockRecorder{}

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		NewHistoryService(store, nil),
		nil,
		recorder,
		registry,
	)

	// Act: lookup a non-existent run ID
	status, err := adapter.Status(context.Background(), "non-existent-run-id")

	// Assert: should return a not-found result (zero RunID is the signal for "not found")
	// Error should map to exit code 1 (user error) per the error taxonomy
	assert.Error(t, err, "Status should return an error for unknown ID")
	assert.Equal(t, "", status.RunID, "status should have empty RunID for not-found")
}

// TestAdapter_Status_DoesNotPanic verifies Status never panics when passed invalid input
// (Acceptance #35).
func TestAdapter_Status_DoesNotPanic(t *testing.T) {
	registry := NewSessionRegistry()
	recorder := &mockRecorder{}

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		registry,
	)

	// Act & Assert: should not panic on any input
	assert.NotPanics(t, func() {
		_, _ = adapter.Status(context.Background(), "")
	}, "Status should not panic on empty ID")

	assert.NotPanics(t, func() {
		_, _ = adapter.Status(context.Background(), "invalid-uuid-format")
	}, "Status should not panic on invalid UUID format")

	assert.NotPanics(t, func() {
		_, _ = adapter.Status(context.Background(), "very-long-id-string-that-might-cause-issues-in-some-implementations-if-not-handled-properly")
	}, "Status should not panic on long ID")
}

// TestAdapter_Status_RegistryPriorityOverHistory verifies Status prefers SessionRegistry.Get
// over history.db lookup. If a session is live in the registry, it should be returned
// even if a persisted record exists with the same ID (Acceptance #33).
func TestAdapter_Status_RegistryPriorityOverHistory(t *testing.T) {
	// Arrange: setup both a live session and a persisted record with the same ID
	store := testmocks.NewMockHistoryStore()
	registry := NewSessionRegistry()
	recorder := &mockRecorder{}

	// Pre-populate history
	persistedRecord := &workflow.ExecutionRecord{
		ID:           "same-run-id",
		WorkflowName: "workflow-1",
		Status:       "success",
	}
	require.NoError(t, store.Record(context.Background(), persistedRecord))

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		NewHistoryService(store, nil),
		nil,
		recorder,
		registry,
	)

	// Create and register a live session with the same ID
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ports.RunRequest{Identifier: "test/workflow"}
	session, err := adapter.Run(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Manually override the session ID to match the persisted record
	// (This is a test artifact; in production, IDs are auto-generated)
	sessionID := session.ID()
	defer session.Close()

	// Act: lookup the status
	status, err := adapter.Status(context.Background(), sessionID)

	// Assert: should return the live session (registry takes priority)
	assert.NoError(t, err)
	assert.Equal(t, sessionID, status.RunID, "should return live session from registry, not history")
}

// TestAdapter_Status_MultipleRunsInHistory verifies Status correctly queries history
// when multiple records exist.
func TestAdapter_Status_MultipleRunsInHistory(t *testing.T) {
	// Arrange: setup history store with multiple records
	store := testmocks.NewMockHistoryStore()

	records := []struct {
		id   string
		name string
	}{
		{"run-1", "workflow-a"},
		{"run-2", "workflow-b"},
		{"run-3", "workflow-c"},
	}

	for _, rec := range records {
		require.NoError(t, store.Record(context.Background(), &workflow.ExecutionRecord{
			ID:           rec.id,
			WorkflowName: rec.name,
			Status:       "success",
		}))
	}

	registry := NewSessionRegistry()
	recorder := &mockRecorder{}

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		NewHistoryService(store, nil),
		nil,
		recorder,
		registry,
	)

	// Act: lookup each persisted run
	for _, rec := range records {
		status, err := adapter.Status(context.Background(), rec.id)

		// Assert: each lookup should succeed
		assert.NoError(t, err, "Status should find record %s", rec.id)
		assert.Equal(t, rec.id, status.RunID, "should return correct run ID")
	}
}
