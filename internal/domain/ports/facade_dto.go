package ports

import (
	"time"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
)

// RunState is the canonical lifecycle state of a workflow run or step.
// Using a named string type rather than a bare string prevents accidental
// assignment of arbitrary strings and makes switch exhaustiveness visible
// to the linter and the reader.
type RunState string

const (
	// RunStatePending means the run has been accepted but execution has not started.
	RunStatePending RunState = "pending"
	// RunStateRunning means the run is actively executing.
	RunStateRunning RunState = "running"
	// RunStateCompleted means the run finished successfully.
	RunStateCompleted RunState = "completed"
	// RunStateFailed means the run stopped due to an error.
	RunStateFailed RunState = "failed"
	// RunStateCancelled means the run was cancelled before completion.
	RunStateCancelled RunState = "cancelled"
)

// ValidateOptions carries validation-only knobs honored by WorkflowFacade.Validate
// and BatchValidator. Zero value = full validation (run all plugin validators, no
// extra per-validation deadline).
type ValidateOptions struct {
	SkipPlugins      bool          // true ⇒ plugin validators are not invoked
	ValidatorTimeout time.Duration // >0 ⇒ bounds the plugin-validator phase via context deadline; 0 ⇒ no extra deadline
}

// RunRequest is the input for starting or validating a workflow execution.
// Zero-value RunRequest (Identifier == "") produces ErrInvalidRequest.
// ValidateOpts is honored only by Validate; ignored by Run/Resume.
type RunRequest struct {
	Identifier   string         // canonical "pack/workflow" identifier
	Inputs       map[string]any // workflow input values
	Options      RunOptions
	ValidateOpts ValidateOptions // honored only by Validate; ignored by Run/Resume
}

// RunOptions carries optional execution modifiers for a RunRequest.
type RunOptions struct {
	DryRun  bool
	Verbose bool
	Timeout time.Duration
}

// StepStatus carries the observed lifecycle state of a single workflow step,
// derived from the event stream projected into the RunSession replay buffer.
// It is a snapshot: zero-value times mean the event was not yet observed.
type StepStatus struct {
	// Name is the step identifier as declared in the workflow YAML.
	Name string
	// Status is the last known state of the step: running, completed, failed, etc.
	Status RunState
	// Error holds the step-level error message when Status == RunStateFailed; empty otherwise.
	Error string
	// StartedAt is stamped when EventStepStarted is observed for this step; zero if not yet started.
	StartedAt time.Time
	// CompletedAt is stamped when EventStepCompleted (or a failure event) is observed; zero if not finished.
	CompletedAt time.Time
}

// Progress summarizes step-level execution counts derived from the event stream.
// It is a read-only snapshot: counts only reflect events that reached the replay buffer.
type Progress struct {
	// Total is the number of distinct steps for which at least one lifecycle event was observed.
	Total int
	// Completed is the number of steps for which EventStepCompleted was observed.
	Completed int
	// Failed is the number of steps for which a failure was observed.
	Failed int
}

// RunStatus reflects the current state of a workflow run.
// Fields added since the initial DTO are backward-compatible: existing callers that only
// read RunID/Status/StartedAt/CompletedAt are unaffected; the new fields are populated
// when derivable from the session's event stream (live runs) and left at their zero
// value when not available (history-fallback path where only persisted metadata exists).
type RunStatus struct {
	// RunID is the unique identifier of the run.
	RunID string
	// Status is the lifecycle state: pending, running, completed, failed, cancelled.
	Status RunState
	// CurrentStep is the name of the step currently executing; empty when no step is
	// in progress (i.e. between steps, or once the run reached a terminal state).
	CurrentStep string
	// StartedAt is the wall-clock time at which execution started (EventRunStarted timestamp).
	// Zero when the event was not observed (NopRecorder path or history fallback).
	StartedAt time.Time
	// CompletedAt is the wall-clock time at which the run reached a terminal state.
	// Zero for still-running sessions.
	CompletedAt time.Time
	// Steps lists all steps for which at least one lifecycle event was observed, in
	// observation order (first EventStepStarted wins for ordering). Empty when the run
	// has not yet produced any step events (pending / history fallback without step data).
	Steps []StepStatus
	// Progress summarizes step counts derived from Steps. Zero-value when Steps is empty.
	Progress Progress
	// Inputs carries the workflow input values that were passed at run start.
	// Populated only when the run was started through the facade (live session path).
	// Nil for history-fallback records where inputs are not stored.
	Inputs map[string]any
}

// WorkflowSummary is a lightweight descriptor returned by WorkflowFacade.List.
//
// Scope and Workflow are the two components of the canonical identifier grammar
// "scope/workflow" (Scope is "local"/"global"/"env" or a vendor pack name; Workflow
// is the local part). They are populated from the underlying WorkflowEntry so that
// interface layers (notably the HTTP API's /api/workflows/{scope}/{name} grammar) can
// reconstruct addressable identifiers from a listing without re-parsing Name.
type WorkflowSummary struct {
	Name        string
	Scope       string
	Workflow    string
	Description string
	Version     string
	Tags        []string
}

// ValidationError is a single structured error entry in a ValidationReport.
// Code provides a machine-readable ErrorCode for programmatic branching.
// Field names the workflow element that failed validation (may be empty for
// top-level errors). Message is the human-readable explanation.
type ValidationError struct {
	Code    domerrors.ErrorCode
	Field   string
	Message string
}

// ValidationReport is returned by WorkflowFacade.Validate.
type ValidationReport struct {
	Valid  bool
	Errors []ValidationError
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
	WorkflowID   string
	WorkflowName string
	Status       RunState
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

// RunStepRequest is the input for executing a single workflow step in isolation
// (the `awf run --step` path). It bypasses the state machine and runs one step.
type RunStepRequest struct {
	Identifier string            // canonical "pack/workflow" identifier
	StepName   string            // the step to execute
	Inputs     map[string]any    // workflow input values
	Mocks      map[string]string // mocked upstream state outputs
}

// StepResult carries the outcome of a single isolated step execution.
type StepResult struct {
	StepName    string
	Output      string
	Stderr      string
	ExitCode    int
	Status      RunState // pending, running, completed, failed, cancelled
	Error       string
	StartedAt   time.Time
	CompletedAt time.Time
}
