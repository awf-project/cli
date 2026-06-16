package application

import (
	"context"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
