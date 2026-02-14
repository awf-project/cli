package workflow

type DryRunPlan struct {
	WorkflowName string
	Description  string
	Inputs       map[string]DryRunInput
	Steps        []DryRunStep
}

type DryRunInput struct {
	Name     string
	Value    any
	Default  bool
	Required bool
}

type DryRunStep struct {
	Name            string
	Type            StepType
	Description     string
	Command         string
	Dir             string
	Hooks           DryRunHooks
	Transitions     []DryRunTransition
	Timeout         int
	Retry           *DryRunRetry
	Capture         *DryRunCapture
	ContinueOnError bool
	Branches        []string
	Strategy        string
	MaxConcurrent   int
	Loop            *DryRunLoop
	Status          TerminalStatus
	Agent           *DryRunAgent
}

type DryRunHooks struct {
	Pre  []DryRunHook
	Post []DryRunHook
}

type DryRunHook struct {
	Type    string
	Content string
}

type DryRunTransition struct {
	Condition string
	Target    string
	Type      string
}

type DryRunRetry struct {
	MaxAttempts    int
	InitialDelayMs int
	MaxDelayMs     int
	Backoff        string
	Multiplier     float64
}

type DryRunCapture struct {
	Stdout  string
	Stderr  string
	MaxSize string
}

type DryRunLoop struct {
	Type           string
	Items          string
	Condition      string
	Body           []string
	MaxIterations  int
	BreakCondition string
	OnComplete     string
}

type DryRunAgent struct {
	Provider       string
	ResolvedPrompt string
	CLICommand     string
	Options        map[string]any
	Timeout        int
}
