package ports

import "context"

// Severity indicates the importance of a validation result.
// SeverityError is the zero value — unset severity defaults to error level.
type Severity int

const (
	SeverityError   Severity = 0
	SeverityWarning Severity = 1
	SeverityInfo    Severity = 2
)

// ValidationResult is a single finding from a workflow or step validator.
type ValidationResult struct {
	Severity Severity
	Message  string
	Step     string // empty for workflow-level findings
	Field    string // empty when not specific to a field
}

// StepExecuteRequest carries the inputs for a custom step type execution.
type StepExecuteRequest struct {
	StepName string
	StepType string
	Config   map[string]any
	Inputs   map[string]any
}

// StepExecuteResult holds the output produced by a custom step type.
type StepExecuteResult struct {
	Output   string
	Data     map[string]any
	ExitCode int
}

// WorkflowValidatorProvider runs plugin-provided validation rules against a workflow.
// workflowJSON is the JSON-encoded domain workflow struct.
type WorkflowValidatorProvider interface {
	ValidateWorkflow(ctx context.Context, workflowJSON []byte) ([]ValidationResult, error)
	ValidateStep(ctx context.Context, workflowJSON []byte, stepName string) ([]ValidationResult, error)
}

// StepTypeProvider dispatches execution of custom step types to the owning plugin.
// HasStepType must be O(1); implementations cache the type list at Init() time.
type StepTypeProvider interface {
	HasStepType(typeName string) bool
	ExecuteStep(ctx context.Context, req StepExecuteRequest) (StepExecuteResult, error)
}
