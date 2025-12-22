package workflow

import (
	"errors"
	"time"
)

// DefaultSubWorkflowTimeout is the default timeout in seconds for sub-workflow execution.
const DefaultSubWorkflowTimeout = 300

// MaxCallStackDepth is the maximum allowed nesting depth for sub-workflow calls.
const MaxCallStackDepth = 10

// CallWorkflowConfig holds configuration for calling another workflow as a sub-workflow.
type CallWorkflowConfig struct {
	Workflow string            `yaml:"workflow"` // workflow name to invoke
	Inputs   map[string]string `yaml:"inputs"`   // parent var → sub-workflow input template
	Outputs  map[string]string `yaml:"outputs"`  // sub-workflow output → parent var name
	Timeout  int               `yaml:"timeout"`  // seconds, 0 = inherit from step
}

// Validate checks if the call workflow configuration is valid.
func (c *CallWorkflowConfig) Validate() error {
	if c.Workflow == "" {
		return errors.New("workflow name is required for call_workflow steps")
	}
	if c.Timeout < 0 {
		return errors.New("timeout must be non-negative")
	}
	return nil
}

// GetTimeout returns the effective timeout in seconds.
// Returns DefaultSubWorkflowTimeout if not explicitly set.
func (c *CallWorkflowConfig) GetTimeout() int {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultSubWorkflowTimeout
}

// SubWorkflowResult holds the result of a sub-workflow execution.
type SubWorkflowResult struct {
	WorkflowName string         // name of the executed sub-workflow
	Outputs      map[string]any // mapped output values
	Error        error          // execution error, if any
	StartedAt    time.Time
	CompletedAt  time.Time
}

// NewSubWorkflowResult creates a new SubWorkflowResult with initialized values.
func NewSubWorkflowResult(workflowName string) *SubWorkflowResult {
	return &SubWorkflowResult{
		WorkflowName: workflowName,
		Outputs:      make(map[string]any),
		StartedAt:    time.Now(),
	}
}

// Duration returns the execution time of the sub-workflow.
func (r *SubWorkflowResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// Success returns true if the sub-workflow completed without error.
func (r *SubWorkflowResult) Success() bool {
	return r.Error == nil
}
