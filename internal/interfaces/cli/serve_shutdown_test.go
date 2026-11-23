package cli

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/api"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

func TestWaitForExecutionsReturnsImmediatelyWhenIdle(t *testing.T) {
	start := time.Now()

	pending := waitForExecutions(&application.Adapter{}, 5*time.Second)

	assert.Empty(t, pending)
	assert.Less(t, time.Since(start), time.Second)
}

func TestInterruptedExecutionsKeepOnlyTrackedMetadata(t *testing.T) {
	bridge := api.NewBridge()
	bridge.TrackFacadeSession(stubSession{id: "run-1"}, "fix-issue")

	executions := interruptedExecutions(bridge, []string{"run-1", "unknown"})

	require.Len(t, executions, 1)
	assert.Equal(t, "run-1", executions[0].ExecutionID)
	assert.Equal(t, "fix-issue", executions[0].WorkflowName)
}

func TestRecordInterruptedWritesFailedHistoryRecord(t *testing.T) {
	historyStore := mocks.NewMockHistoryStore()
	now := time.Now()
	execution := &api.ActiveExecution{
		ExecutionID:  "run-1",
		WorkflowName: "fix-issue",
		StartedAt:    now.Add(-2 * time.Second),
	}

	recorded := recordInterrupted(context.Background(), historyStore, mocks.NewMockStateStore(),
		mocks.NewMockLogger(), []*api.ActiveExecution{execution}, now)

	require.Equal(t, 1, recorded)
	records, err := historyStore.List(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, records, 1)
	record := records[0]
	assert.Equal(t, "run-1", record.WorkflowID)
	assert.Equal(t, "fix-issue", record.WorkflowName)
	assert.Equal(t, string(workflow.StatusFailed), record.Status)
	assert.Equal(t, interruptedExitCode, record.ExitCode)
	assert.Equal(t, 1, record.Attempt)
	assert.Equal(t, now.Add(-2*time.Second), record.StartedAt)
	assert.Equal(t, now, record.CompletedAt)
	assert.Equal(t, int64(2000), record.DurationMs)
	assert.Contains(t, record.ErrorMessage, "shut down")
}

func TestRecordInterruptedKeepsRecordingAfterOneFailure(t *testing.T) {
	historyStore := &failingHistoryStore{MockHistoryStore: mocks.NewMockHistoryStore()}
	now := time.Now()

	recorded := recordInterrupted(context.Background(), historyStore, mocks.NewMockStateStore(),
		mocks.NewMockLogger(), []*api.ActiveExecution{
			{ExecutionID: "run-1", WorkflowName: "a", StartedAt: now},
			{ExecutionID: "run-2", WorkflowName: "b", StartedAt: now},
		}, now)

	assert.Equal(t, 1, recorded, "the second execution is recorded despite the first failure")
}

// TestRecordInterruptedReusesPersistedAttemptIdentity covers a resumed run: the
// predecessor attempt is already in history and the live attempt owns a later
// identity. Recording attempt 1 again would collide with the predecessor and
// leave the resumed attempt missing.
func TestRecordInterruptedReusesPersistedAttemptIdentity(t *testing.T) {
	ctx := context.Background()
	historyStore := newSQLiteHistoryStore(t)
	stateStore := store.NewJSONStore(t.TempDir())
	now := time.Now().Truncate(time.Millisecond)
	firstStartedAt := now.Add(-time.Minute)

	require.NoError(t, historyStore.Record(ctx, cancelledRecord("record-1", 1, "",
		firstStartedAt, firstStartedAt.Add(time.Second))))
	resumedStartedAt := now.Add(-2 * time.Second)
	require.NoError(t, stateStore.Save(ctx, &workflow.ExecutionContext{
		WorkflowID:              "run-1",
		WorkflowName:            "fix-issue",
		HistoryRecordID:         "record-2",
		HistoryAttempt:          2,
		PreviousHistoryRecordID: "record-1",
		StartedAt:               resumedStartedAt,
	}))

	recorded := recordInterrupted(ctx, historyStore, stateStore, mocks.NewMockLogger(),
		[]*api.ActiveExecution{{ExecutionID: "run-1", WorkflowName: "fix-issue", StartedAt: resumedStartedAt}}, now)

	require.Equal(t, 1, recorded)
	records, err := historyStore.List(ctx, &workflow.HistoryFilter{WorkflowID: "run-1"})
	require.NoError(t, err)
	require.Len(t, records, 2)
	byAttempt := make(map[int]*workflow.ExecutionRecord, len(records))
	for _, record := range records {
		byAttempt[record.Attempt] = record
	}
	require.Contains(t, byAttempt, 1)
	require.Contains(t, byAttempt, 2)
	assert.Equal(t, "record-2", byAttempt[2].ID)
	assert.Equal(t, "record-1", byAttempt[2].ResumedFromID)
	assert.Equal(t, string(workflow.StatusFailed), byAttempt[2].Status)
	assert.Equal(t, resumedStartedAt, byAttempt[2].StartedAt)
}

// TestRecordInterruptedLeavesFinalizedAttemptAlone covers the order where the
// execution records its own outcome before the fallback write.
func TestRecordInterruptedLeavesFinalizedAttemptAlone(t *testing.T) {
	tests := []struct {
		name           string
		recordedMarker bool
	}{
		{name: "finalization marked in state", recordedMarker: true},
		{name: "record written before the state marker", recordedMarker: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			historyStore := newSQLiteHistoryStore(t)
			stateStore := store.NewJSONStore(t.TempDir())
			now := time.Now().Truncate(time.Millisecond)
			startedAt := now.Add(-2 * time.Second)

			require.NoError(t, historyStore.Record(ctx, cancelledRecord("record-1", 1, "",
				startedAt, now)))
			require.NoError(t, stateStore.Save(ctx, &workflow.ExecutionContext{
				WorkflowID:      "run-1",
				WorkflowName:    "fix-issue",
				HistoryRecordID: "record-1",
				HistoryAttempt:  1,
				HistoryRecorded: tt.recordedMarker,
				StartedAt:       startedAt,
			}))

			recorded := recordInterrupted(ctx, historyStore, stateStore, mocks.NewMockLogger(),
				[]*api.ActiveExecution{{ExecutionID: "run-1", WorkflowName: "fix-issue", StartedAt: startedAt}}, now)

			assert.Equal(t, 0, recorded, "the execution's own record is kept")
			records, err := historyStore.List(ctx, &workflow.HistoryFilter{WorkflowID: "run-1"})
			require.NoError(t, err)
			require.Len(t, records, 1)
			assert.Equal(t, string(workflow.StatusCancelled), records[0].Status)
		})
	}
}

// TestRecordInterruptedFirstKeepsLaterFinalizationOut covers the order where the
// fallback writes before the execution reaches its own terminal write. Both
// writes claim the same attempt, so the later one is rejected instead of adding
// a second row for that attempt.
func TestRecordInterruptedFirstKeepsLaterFinalizationOut(t *testing.T) {
	ctx := context.Background()
	historyStore := newSQLiteHistoryStore(t)
	stateStore := store.NewJSONStore(t.TempDir())
	now := time.Now().Truncate(time.Millisecond)
	startedAt := now.Add(-2 * time.Second)

	require.NoError(t, stateStore.Save(ctx, &workflow.ExecutionContext{
		WorkflowID:      "run-1",
		WorkflowName:    "fix-issue",
		HistoryRecordID: "record-1",
		HistoryAttempt:  1,
		StartedAt:       startedAt,
	}))

	recorded := recordInterrupted(ctx, historyStore, stateStore, mocks.NewMockLogger(),
		[]*api.ActiveExecution{{ExecutionID: "run-1", WorkflowName: "fix-issue", StartedAt: startedAt}}, now)
	require.Equal(t, 1, recorded)

	err := historyStore.Record(ctx, cancelledRecord("record-1", 1, "", startedAt, now.Add(-time.Second)))
	require.ErrorIs(t, err, store.ErrHistoryRecordConflict)

	records, err := historyStore.List(ctx, &workflow.HistoryFilter{WorkflowID: "run-1"})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "record-1", records[0].ID)
	assert.Equal(t, string(workflow.StatusFailed), records[0].Status)
}

func newSQLiteHistoryStore(t *testing.T) *store.SQLiteHistoryStore {
	t.Helper()
	historyStore, err := store.NewSQLiteHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, historyStore.Close()) })
	return historyStore
}

func cancelledRecord(id string, attempt int, resumedFrom string, startedAt, completedAt time.Time) *workflow.ExecutionRecord {
	return &workflow.ExecutionRecord{
		ID:            id,
		WorkflowID:    "run-1",
		WorkflowName:  "fix-issue",
		Attempt:       attempt,
		ResumedFromID: resumedFrom,
		Status:        string(workflow.StatusCancelled),
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
		DurationMs:    completedAt.Sub(startedAt).Milliseconds(),
	}
}

// stubSession satisfies ports.RunSession for metadata-only assertions.
type stubSession struct{ id string }

func (s stubSession) ID() string                        { return s.id }
func (s stubSession) Events() <-chan ports.Event        { return nil }
func (s stubSession) Respond(ports.InputResponse) error { return nil }
func (s stubSession) Err() error                        { return nil }
func (s stubSession) Close() error                      { return nil }

// failingHistoryStore fails the first Record call and delegates the rest.
type failingHistoryStore struct {
	*mocks.MockHistoryStore
	calls int
}

func (s *failingHistoryStore) Record(ctx context.Context, record *workflow.ExecutionRecord) error {
	s.calls++
	if s.calls == 1 {
		return assert.AnError
	}
	return s.MockHistoryStore.Record(ctx, record)
}
