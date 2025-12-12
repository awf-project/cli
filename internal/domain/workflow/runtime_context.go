package workflow

import "time"

// RuntimeContext provides all variable namespaces available during execution.
// This is a domain-pure representation used for displaying context to users.
type RuntimeContext struct {
	Inputs   map[string]any
	States   map[string]RuntimeStepState
	Workflow RuntimeWorkflowData
	Env      map[string]string
	Context  RuntimeContextData
	Error    *RuntimeErrorData
	Loop     *RuntimeLoopData
}

// RuntimeStepState holds step execution results.
type RuntimeStepState struct {
	Output   string
	Stderr   string
	ExitCode int
	Status   string
}

// RuntimeWorkflowData holds workflow metadata.
type RuntimeWorkflowData struct {
	ID           string
	Name         string
	CurrentState string
	StartedAt    time.Time
}

// Duration returns workflow duration as a formatted string.
func (w RuntimeWorkflowData) Duration() string {
	return time.Since(w.StartedAt).Round(time.Millisecond).String()
}

// RuntimeContextData holds runtime context information.
type RuntimeContextData struct {
	WorkingDir string
	User       string
	Hostname   string
}

// RuntimeErrorData holds error information for error hooks.
type RuntimeErrorData struct {
	Message  string
	State    string
	ExitCode int
	Type     string
}

// RuntimeLoopData holds loop iteration context.
type RuntimeLoopData struct {
	Item   any
	Index  int
	First  bool
	Last   bool
	Length int
	Parent *RuntimeLoopData
}

// Index1 returns the 1-based iteration index.
func (l *RuntimeLoopData) Index1() int {
	return l.Index + 1
}
