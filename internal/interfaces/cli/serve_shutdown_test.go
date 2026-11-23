package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
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
	store := mocks.NewMockHistoryStore()
	now := time.Now()
	execution := &api.ActiveExecution{
		ExecutionID:  "run-1",
		WorkflowName: "fix-issue",
		StartedAt:    now.Add(-2 * time.Second),
	}

	recorded := recordInterrupted(context.Background(), store, mocks.NewMockLogger(),
		[]*api.ActiveExecution{execution}, now)

	require.Equal(t, 1, recorded)
	records, err := store.List(context.Background(), nil)
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
	store := &failingHistoryStore{MockHistoryStore: mocks.NewMockHistoryStore()}
	now := time.Now()

	recorded := recordInterrupted(context.Background(), store, mocks.NewMockLogger(),
		[]*api.ActiveExecution{
			{ExecutionID: "run-1", WorkflowName: "a", StartedAt: now},
			{ExecutionID: "run-2", WorkflowName: "b", StartedAt: now},
		}, now)

	assert.Equal(t, 1, recorded, "the second execution is recorded despite the first failure")
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
