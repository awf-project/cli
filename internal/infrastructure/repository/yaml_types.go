package repository

// yamlWorkflow is the YAML representation of a workflow.
type yamlWorkflow struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Version     string             `yaml:"version"`
	Author      string             `yaml:"author"`
	Tags        []string           `yaml:"tags"`
	Inputs      []yamlInput        `yaml:"inputs"`
	Env         []string           `yaml:"env"`
	States      yamlStates         `yaml:"states"`
	Hooks       *yamlWorkflowHooks `yaml:"hooks"`
}

// yamlStates holds the initial state and step definitions.
type yamlStates struct {
	Initial string              `yaml:"initial"`
	Steps   map[string]yamlStep `yaml:"-"` // populated via custom unmarshaler
}

// yamlStep is the YAML representation of a step.
type yamlStep struct {
	Type            string           `yaml:"type"`
	Description     string           `yaml:"description"`
	Operation       string           `yaml:"operation"`
	Command         string           `yaml:"command"`
	Dir             string           `yaml:"dir"`
	Timeout         string           `yaml:"timeout"`
	OnSuccess       string           `yaml:"on_success"`
	OnFailure       string           `yaml:"on_failure"`
	Transitions     []yamlTransition `yaml:"transitions"`
	DependsOn       []string         `yaml:"depends_on"`
	Parallel        []string         `yaml:"parallel"`
	Strategy        string           `yaml:"strategy"`
	MaxConcurrent   int              `yaml:"max_concurrent"`
	Retry           *yamlRetry       `yaml:"retry"`
	Capture         *yamlCapture     `yaml:"capture"`
	Hooks           *yamlStepHooks   `yaml:"hooks"`
	ContinueOnError bool             `yaml:"continue_on_error"`
	Status          string           `yaml:"status"`  // for terminal steps
	Message         string           `yaml:"message"` // for terminal steps

	// Loop configuration (for for_each and while types)
	Items         any      `yaml:"items"`          // string or []any for for_each
	While         string   `yaml:"while"`          // condition for while loop
	Body          []string `yaml:"body"`           // steps to execute each iteration
	MaxIterations int      `yaml:"max_iterations"` // safety limit
	BreakWhen     string   `yaml:"break_when"`     // optional break condition
	OnComplete    string   `yaml:"on_complete"`    // next state after loop
}

// yamlTransition is the YAML representation of a conditional transition.
type yamlTransition struct {
	When string `yaml:"when"` // condition expression (empty = default)
	Goto string `yaml:"goto"` // target state name
}

// yamlInput is the YAML representation of an input.
type yamlInput struct {
	Name        string               `yaml:"name"`
	Type        string               `yaml:"type"`
	Description string               `yaml:"description"`
	Required    bool                 `yaml:"required"`
	Default     any                  `yaml:"default"`
	Validation  *yamlInputValidation `yaml:"validation"`
}

// yamlInputValidation is the YAML representation of input validation.
type yamlInputValidation struct {
	Pattern       string   `yaml:"pattern"`
	Enum          []string `yaml:"enum"`
	Min           *int     `yaml:"min"`
	Max           *int     `yaml:"max"`
	FileExists    bool     `yaml:"file_exists"`
	FileExtension []string `yaml:"file_extension"`
}

// yamlRetry is the YAML representation of retry configuration.
type yamlRetry struct {
	MaxAttempts        int     `yaml:"max_attempts"`
	InitialDelay       string  `yaml:"initial_delay"`
	MaxDelay           string  `yaml:"max_delay"`
	Backoff            string  `yaml:"backoff"`
	Multiplier         float64 `yaml:"multiplier"`
	Jitter             float64 `yaml:"jitter"`
	RetryableExitCodes []int   `yaml:"retryable_exit_codes"`
}

// yamlCapture is the YAML representation of capture configuration.
type yamlCapture struct {
	Stdout   string `yaml:"stdout"`
	Stderr   string `yaml:"stderr"`
	MaxSize  string `yaml:"max_size"`
	Encoding string `yaml:"encoding"`
}

// yamlWorkflowHooks is the YAML representation of workflow hooks.
type yamlWorkflowHooks struct {
	WorkflowStart  []yamlHookAction `yaml:"workflow_start"`
	WorkflowEnd    []yamlHookAction `yaml:"workflow_end"`
	WorkflowError  []yamlHookAction `yaml:"workflow_error"`
	WorkflowCancel []yamlHookAction `yaml:"workflow_cancel"`
}

// yamlStepHooks is the YAML representation of step hooks.
type yamlStepHooks struct {
	Pre  []yamlHookAction `yaml:"pre"`
	Post []yamlHookAction `yaml:"post"`
}

// yamlHookAction is the YAML representation of a hook action.
type yamlHookAction struct {
	Log     string `yaml:"log"`
	Command string `yaml:"command"`
}
