package workflow

// InteractiveAction represents user actions during interactive execution.
type InteractiveAction string

const (
	ActionContinue InteractiveAction = "continue" // execute the step
	ActionSkip     InteractiveAction = "skip"     // skip to next step (on_success)
	ActionAbort    InteractiveAction = "abort"    // stop workflow execution
	ActionInspect  InteractiveAction = "inspect"  // show context details
	ActionEdit     InteractiveAction = "edit"     // modify an input value
	ActionRetry    InteractiveAction = "retry"    // re-run previous step
)

// InteractiveStepInfo provides step details for interactive display.
type InteractiveStepInfo struct {
	Name        string   // step name
	Index       int      // 1-based step index
	Total       int      // total steps discovered
	Step        *Step    // reference to the step definition
	Command     string   // resolved command (with interpolation)
	Transitions []string // formatted transition descriptions
}

// InteractiveResult holds the outcome of an interactive step execution.
type InteractiveResult struct {
	StepName   string          // executed step name
	Status     ExecutionStatus // completion status
	Output     string          // stdout
	Stderr     string          // stderr
	ExitCode   int             // process exit code
	DurationMs int64           // execution duration in milliseconds
	NextStep   string          // next step to execute
	WasSkipped bool            // true if step was skipped
	WasRetried bool            // true if this was a retry
	RetryCount int             // number of retries attempted
}
