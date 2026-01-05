package workflow

// DryRunPlan represents the complete execution plan for a workflow dry-run.
// It shows what would be executed without actually running any commands.
type DryRunPlan struct {
	WorkflowName string                 // name of the workflow
	Description  string                 // workflow description
	Inputs       map[string]DryRunInput // resolved input values
	Steps        []DryRunStep           // execution plan steps in order
}

// DryRunInput represents an input value for the dry-run.
type DryRunInput struct {
	Name     string // input name
	Value    any    // resolved value
	Default  bool   // true if using default value
	Required bool   // whether the input is required
}

// DryRunStep represents a single step in the dry-run execution plan.
type DryRunStep struct {
	Name            string             // step name
	Type            StepType           // step type (command, parallel, terminal, for_each, while)
	Description     string             // step description
	Command         string             // resolved command (for command type)
	Dir             string             // working directory
	Hooks           DryRunHooks        // pre/post hooks
	Transitions     []DryRunTransition // all possible transitions
	Timeout         int                // timeout in seconds
	Retry           *DryRunRetry       // retry configuration
	Capture         *DryRunCapture     // capture configuration
	ContinueOnError bool               // whether to continue on error
	// Parallel step fields
	Branches      []string // branch names (for parallel type)
	Strategy      string   // parallel strategy
	MaxConcurrent int      // max concurrent branches
	// Loop step fields
	Loop *DryRunLoop // loop configuration (for for_each/while types)
	// Terminal step fields
	Status TerminalStatus // terminal status (success/failure)
	// Agent step fields
	Agent *DryRunAgent // agent configuration (for agent type)
}

// DryRunHooks represents hooks that would run for a step.
type DryRunHooks struct {
	Pre  []DryRunHook // pre-execution hooks
	Post []DryRunHook // post-execution hooks
}

// DryRunHook represents a single hook action.
type DryRunHook struct {
	Type    string // "log" or "command"
	Content string // log message or resolved command
}

// DryRunTransition represents a possible state transition.
type DryRunTransition struct {
	Condition string // condition expression (empty for unconditional/legacy)
	Target    string // target step name
	Type      string // "success", "failure", "conditional", "default"
}

// DryRunRetry represents retry configuration for display.
type DryRunRetry struct {
	MaxAttempts    int     // max retry attempts
	InitialDelayMs int     // initial delay in milliseconds
	MaxDelayMs     int     // max delay in milliseconds
	Backoff        string  // backoff strategy
	Multiplier     float64 // backoff multiplier
}

// DryRunCapture represents output capture configuration.
type DryRunCapture struct {
	Stdout  string // variable name for stdout
	Stderr  string // variable name for stderr
	MaxSize string // max size limit
}

// DryRunLoop represents loop configuration for display.
type DryRunLoop struct {
	Type           string   // "for_each" or "while"
	Items          string   // items expression (for for_each)
	Condition      string   // condition expression (for while)
	Body           []string // body step names
	MaxIterations  int      // max iteration limit
	BreakCondition string   // break condition expression
	OnComplete     string   // next state after loop
}

// DryRunAgent represents agent configuration for display in dry-run.
type DryRunAgent struct {
	Provider       string         // agent provider name
	ResolvedPrompt string         // prompt after interpolation
	CLICommand     string         // CLI command that would be executed
	Options        map[string]any // provider-specific options
	Timeout        int            // timeout in seconds
}
