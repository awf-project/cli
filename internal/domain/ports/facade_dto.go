package ports

import "time"

// RunRequest is the input for starting or validating a workflow execution.
// Zero-value RunRequest (Identifier == "") produces ErrInvalidRequest.
type RunRequest struct {
	Identifier string         // canonical "pack/workflow" identifier
	Inputs     map[string]any // workflow input values
	Options    RunOptions
}

// RunOptions carries optional execution modifiers for a RunRequest.
type RunOptions struct {
	DryRun  bool
	Verbose bool
	Timeout time.Duration
}

// RunResult carries the outcome of a completed workflow run.
type RunResult struct {
	RunID  string
	Status string
	Record *RunRecord
}

// RunStatus reflects the current state of a workflow run.
type RunStatus struct {
	RunID       string
	Status      string // pending, running, completed, failed, cancelled
	StartedAt   time.Time
	CompletedAt time.Time
}

// WorkflowSummary is a lightweight descriptor returned by WorkflowFacade.List.
type WorkflowSummary struct {
	Name        string
	Description string
	Version     string
	Tags        []string
}

// ValidationReport is returned by WorkflowFacade.Validate.
type ValidationReport struct {
	Valid  bool
	Errors []string
}

// HistoryFilter scopes a WorkflowFacade.History query.
type HistoryFilter struct {
	WorkflowName string
	Status       string
	Since        time.Time
	Until        time.Time
	Limit        int
}

// RunRecord is a single entry in the workflow execution history.
type RunRecord struct {
	RunID        string
	WorkflowName string
	Status       string
	StartedAt    time.Time
	CompletedAt  time.Time
	DurationMs   int64
	ErrorMessage string
}

// InputRequest describes a prompt that the workflow needs answered.
type InputRequest struct {
	PromptID string
	Prompt   string
	Default  string
}

// InputResponse carries the user's answer to an InputRequest.
type InputResponse struct {
	PromptID string
	Value    string
}
