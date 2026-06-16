package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// --- WorkflowsLoadedMsg ---

func TestWorkflowsLoadedMsg_CarriesWorkflows(t *testing.T) {
	workflows := []*workflow.Workflow{
		{Name: "workflow-1"},
		{Name: "workflow-2"},
	}

	msg := WorkflowsLoadedMsg{Workflows: workflows}

	assert.Equal(t, workflows, msg.Workflows)
	assert.Len(t, msg.Workflows, 2)
}

func TestWorkflowsLoadedMsg_EmptyList(t *testing.T) {
	msg := WorkflowsLoadedMsg{Workflows: []*workflow.Workflow{}}

	assert.Empty(t, msg.Workflows)
	assert.NotNil(t, msg.Workflows)
}

func TestWorkflowsLoadedMsg_NilList(t *testing.T) {
	msg := WorkflowsLoadedMsg{Workflows: nil}

	assert.Nil(t, msg.Workflows)
}

// --- HistoryLoadedMsg ---

func TestHistoryLoadedMsg_CarriesRecordsAndStats(t *testing.T) {
	records := []*workflow.ExecutionRecord{
		{ID: "exec-1", WorkflowName: "wf1", Status: "success"},
		{ID: "exec-2", WorkflowName: "wf2", Status: "failed"},
	}
	stats := &workflow.HistoryStats{
		TotalExecutions: 2,
		SuccessCount:    1,
		FailedCount:     1,
	}

	msg := HistoryLoadedMsg{Records: records, Stats: stats}

	assert.Equal(t, records, msg.Records)
	assert.Equal(t, stats, msg.Stats)
	assert.Len(t, msg.Records, 2)
}

func TestHistoryLoadedMsg_EmptyRecords(t *testing.T) {
	msg := HistoryLoadedMsg{Records: []*workflow.ExecutionRecord{}, Stats: nil}

	assert.Empty(t, msg.Records)
	assert.Nil(t, msg.Stats)
}

func TestHistoryLoadedMsg_NilRecordsAndStats(t *testing.T) {
	msg := HistoryLoadedMsg{}

	assert.Nil(t, msg.Records)
	assert.Nil(t, msg.Stats)
}

// --- ExecutionStartedMsg ---

func TestExecutionStartedMsg_CarriesExecutionID(t *testing.T) {
	msg := ExecutionStartedMsg{ExecutionID: "exec-abc-123", Workflow: &workflow.Workflow{Name: "test"}}

	assert.Equal(t, "exec-abc-123", msg.ExecutionID)
	assert.NotNil(t, msg.Workflow)
}

func TestExecutionStartedMsg_EmptyID(t *testing.T) {
	msg := ExecutionStartedMsg{ExecutionID: ""}

	assert.Empty(t, msg.ExecutionID)
}

// --- ExecutionFinishedMsg ---

func TestExecutionFinishedMsg_WithError(t *testing.T) {
	msg := ExecutionFinishedMsg{Err: errors.New("execution failed")}

	assert.EqualError(t, msg.Err, "execution failed")
}

func TestExecutionFinishedMsg_Success(t *testing.T) {
	msg := ExecutionFinishedMsg{Err: nil}

	assert.Nil(t, msg.Err)
}

// --- ErrMsg ---

func TestErrMsg_ImplementsErrorInterface(t *testing.T) {
	err := errors.New("test error")
	msg := ErrMsg{Err: err}

	var _ error = msg
}

func TestErrMsg_Error_ReturnsErrorMessage(t *testing.T) {
	err := errors.New("test error message")
	msg := ErrMsg{Err: err}

	assert.Equal(t, "test error message", msg.Error())
}

func TestErrMsg_Error_WithNilError_Panics(t *testing.T) {
	msg := ErrMsg{Err: nil}

	require.Panics(t, func() {
		_ = msg.Error()
	})
}

func TestErrMsg_Error_WithWrappedError_ReturnsOuterMessage(t *testing.T) {
	wrapped := errors.New("wrapped error")
	msg := ErrMsg{Err: wrapped}

	assert.Equal(t, "wrapped error", msg.Error())
}

func TestErrMsg_WithCustomError_ImplementsError(t *testing.T) {
	customErr := errors.New("custom workflow error")
	msg := ErrMsg{Err: customErr}

	assert.Error(t, msg)
	assert.Equal(t, "custom workflow error", msg.Error())
}

// --- facadeEventMsg ---

func TestFacadeEventMsg_CarriesEvent(t *testing.T) {
	event := ports.Event{
		Seq:   1,
		Kind:  ports.EventRunStarted,
		RunID: "run-123",
	}

	msg := facadeEventMsg{Event: event}

	assert.Equal(t, uint64(1), msg.Event.Seq)
	assert.Equal(t, ports.EventRunStarted, msg.Event.Kind)
	assert.Equal(t, "run-123", msg.Event.RunID)
}

func TestFacadeEventMsg_WithEventStepStarted(t *testing.T) {
	payload := &ports.EnrichedStepPayload{
		StepName: "deploy",
	}
	event := ports.Event{
		Seq:     2,
		Kind:    ports.EventStepStarted,
		Payload: payload,
	}

	msg := facadeEventMsg{Event: event}

	assert.Equal(t, uint64(2), msg.Event.Seq)
	assert.Equal(t, ports.EventStepStarted, msg.Event.Kind)
	assert.Equal(t, payload, msg.Event.Payload)
}

func TestFacadeEventMsg_WithEventWorkflowCompleted(t *testing.T) {
	event := ports.Event{
		Seq:   3,
		Kind:  ports.EventWorkflowCompleted,
		RunID: "run-456",
	}

	msg := facadeEventMsg{Event: event}

	assert.Equal(t, uint64(3), msg.Event.Seq)
	assert.Equal(t, ports.EventWorkflowCompleted, msg.Event.Kind)
}

func TestFacadeEventMsg_WithEventWorkflowFailed(t *testing.T) {
	event := ports.Event{
		Seq:     4,
		Kind:    ports.EventWorkflowFailed,
		RunID:   "run-789",
		Payload: &ports.EnrichedTerminal{Error: "step execution failed"},
	}

	msg := facadeEventMsg{Event: event}

	assert.Equal(t, uint64(4), msg.Event.Seq)
	assert.Equal(t, ports.EventWorkflowFailed, msg.Event.Kind)
	terminal, ok := msg.Event.Payload.(*ports.EnrichedTerminal)
	require.True(t, ok, "payload should be EnrichedTerminal")
	assert.Equal(t, "step execution failed", terminal.Error)
}

func TestFacadeEventMsg_WithNilPayload(t *testing.T) {
	event := ports.Event{
		Seq:     5,
		Kind:    ports.EventRunStarted,
		Payload: nil,
	}

	msg := facadeEventMsg{Event: event}

	assert.Nil(t, msg.Event.Payload)
}

// --- Message lifecycle integration ---

func TestMessages_AllTypesCanCoexist(t *testing.T) {
	messages := []any{
		WorkflowsLoadedMsg{Workflows: []*workflow.Workflow{{Name: "wf1"}}},
		HistoryLoadedMsg{Records: []*workflow.ExecutionRecord{{ID: "exec-1"}}},
		ExecutionStartedMsg{ExecutionID: "exec-1", Workflow: &workflow.Workflow{Name: "wf1"}},
		ExecutionFinishedMsg{Err: nil},
		LogLineMsg{Entry: LogEntry{Event: "workflow.started", WorkflowName: "test"}},
		ErrMsg{Err: errors.New("oops")},
		facadeEventMsg{Event: ports.Event{Seq: 1, Kind: ports.EventRunStarted}},
	}

	assert.Len(t, messages, 7)
	for _, msg := range messages {
		assert.NotNil(t, msg)
	}
}
