package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	done := make(chan error, 1)
	execCtx := workflow.NewExecutionContext("exec-abc-123", "test")
	msg := ExecutionStartedMsg{ExecutionID: "exec-abc-123", Workflow: &workflow.Workflow{Name: "test"}, ExecCtx: execCtx, Done: done}

	assert.Equal(t, "exec-abc-123", msg.ExecutionID)
	assert.NotNil(t, msg.ExecCtx)
}

func TestExecutionStartedMsg_EmptyID(t *testing.T) {
	done := make(chan error, 1)
	msg := ExecutionStartedMsg{ExecutionID: "", Workflow: &workflow.Workflow{Name: "test"}, Done: done}

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

// --- Message lifecycle integration ---

func TestMessages_AllTypesCanCoexist(t *testing.T) {
	done := make(chan error, 1)
	messages := []any{
		WorkflowsLoadedMsg{Workflows: []*workflow.Workflow{{Name: "wf1"}}},
		HistoryLoadedMsg{Records: []*workflow.ExecutionRecord{{ID: "exec-1"}}},
		ExecutionStartedMsg{ExecutionID: "exec-1", Workflow: &workflow.Workflow{Name: "wf1"}, Done: done},
		ExecutionFinishedMsg{Err: nil},
		LogLineMsg{Entry: LogEntry{Event: "workflow.started", WorkflowName: "test"}},
		ErrMsg{Err: errors.New("oops")},
	}

	assert.Len(t, messages, 6)
	for _, msg := range messages {
		assert.NotNil(t, msg)
	}
}
