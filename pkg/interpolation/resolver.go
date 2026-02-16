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
	Loop     *LoopData // loop iteration data
	AWF      map[string]string
}

// LoopData holds loop iteration context for interpolation.
type LoopData struct {
	Item   any       // current item value (for_each)
	Index  int       // 0-based iteration index
	First  bool      // true on first iteration
	Last   bool      // true on last iteration (for_each only)
	Length int       // total items count (for_each only, -1 for while)
	Parent *LoopData // reference to enclosing loop for {{.loop.Parent.*}} access (F043)
}

// Index1 returns the 1-based iteration index.
func (l *LoopData) Index1() int {
	return l.Index + 1
}

// StepStateData holds step execution results for interpolation.
type StepStateData struct {
	Output     string
	Stderr     string
	ExitCode   int
	Status     string
	Response   map[string]any // parsed JSON response from agent steps
	TokensUsed int            // total tokens used from agent steps
	JSON       any            // explicit JSON output from output_format (map[string]any or []any)
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
		AWF:    make(map[string]string),
	}
}
