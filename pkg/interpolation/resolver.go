package interpolation

import "time"

// Resolver defines the contract for variable interpolation.
type Resolver interface {
	Resolve(template string, ctx *Context) (string, error)
}

// Context provides all variable namespaces for interpolation.
type Context struct {
	Inputs   map[string]any
	States   map[string]StepStateData
	Workflow WorkflowData
	Env      map[string]string
	Context  ContextData
	Error    *ErrorData
}

// StepStateData holds step execution results for interpolation.
type StepStateData struct {
	Output   string
	Stderr   string
	ExitCode int
	Status   string
}

// WorkflowData holds workflow metadata for interpolation.
type WorkflowData struct {
	ID           string
	Name         string
	CurrentState string
	StartedAt    time.Time
}

// Duration returns workflow duration as a formatted string.
func (w WorkflowData) Duration() string {
	return time.Since(w.StartedAt).Round(time.Millisecond).String()
}

// ContextData holds runtime context information.
type ContextData struct {
	WorkingDir string
	User       string
	Hostname   string
}

// ErrorData holds error information for error hooks.
type ErrorData struct {
	Message  string
	State    string
	ExitCode int
	Type     string
}

// NewContext creates a new interpolation context with initialized maps.
func NewContext() *Context {
	return &Context{
		Inputs: make(map[string]any),
		States: make(map[string]StepStateData),
		Env:    make(map[string]string),
	}
}
