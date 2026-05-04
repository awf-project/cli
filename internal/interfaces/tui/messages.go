package tui

import (
	"github.com/awf-project/cli/internal/domain/workflow"
)

// WorkflowsLoadedMsg carries the list of available workflows after discovery.
type WorkflowsLoadedMsg struct {
	Workflows []*workflow.Workflow
	Entries   []workflow.WorkflowEntry
}

// HistoryLoadedMsg carries execution history records and summary statistics.
type HistoryLoadedMsg struct {
	Records []*workflow.ExecutionRecord
	Stats   *workflow.HistoryStats
}

// ExecutionStartedMsg signals a workflow run has begun in a background goroutine.
// ExecCtx is the live execution context, observable via GetAllStepStates() during execution.
// Done receives nil on success or an error when execution completes.
type ExecutionStartedMsg struct {
	ExecutionID string
	Workflow    *workflow.Workflow
	ExecCtx     *workflow.ExecutionContext
	Done        <-chan error
}

// ExecutionFinishedMsg signals the workflow run has ended.
// Err is non-nil if execution failed.
type ExecutionFinishedMsg struct {
	Err error
}

// LaunchWorkflowMsg requests that the Model launch the given workflow.
// It is emitted by WorkflowsTab when the user presses Enter on a selected item.
// The Model handles it by switching to TabMonitoring and calling bridge.RunWorkflow.
type LaunchWorkflowMsg struct {
	Workflow *workflow.Workflow
	Inputs   map[string]any
}

// ValidationResultMsg carries the result of a workflow validation request.
// Success is true when validation passed; otherwise Error contains the message.
type ValidationResultMsg struct {
	Name    string
	Success bool
	Error   string
}

// ErrMsg wraps an error for the Update loop.
type ErrMsg struct {
	Err error
}

func (e ErrMsg) Error() string {
	return e.Err.Error()
}

// InputRequestedMsg signals that the execution goroutine is waiting for user
// input in a conversation step. The monitoring tab should display a text input
// and auto-select the running conversation step.
type InputRequestedMsg struct{}
