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
	ScriptFile      string           `yaml:"script_file"`
	Dir             string           `yaml:"dir"`
	Timeout         string           `yaml:"timeout"`
	OnSuccess       string           `yaml:"on_success"`
	OnFailure       any              `yaml:"on_failure"` // F066: string (named terminal) or map[string]any (inline error)
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
	ExitCode        int              // for synthesized inline error terminals (FR-004); set programmatically, not from YAML

	// Loop configuration (for for_each and while types)
	Items         any      `yaml:"items"`          // string or []any for for_each
	While         string   `yaml:"while"`          // condition for while loop
	Body          []string `yaml:"body"`           // steps to execute each iteration
	MaxIterations any      `yaml:"max_iterations"` // safety limit (int or string expression)
	BreakWhen     string   `yaml:"break_when"`     // optional break condition
	OnComplete    string   `yaml:"on_complete"`    // next state after loop

	// Template reference (for using templates in steps)
	UseTemplate string         `yaml:"use_template"` // template name to use
	Parameters  map[string]any `yaml:"parameters"`   // parameters to pass to template

	// Operation configuration (for plugin operations - F021)
	OperationInputs map[string]any `yaml:"operation_inputs"` // input parameters for operations

	// Call workflow configuration (for sub-workflows - F023)
	// Flat structure: workflow, call_inputs, call_outputs directly on step
	Workflow    string            `yaml:"workflow"` // workflow name to invoke
	CallInputs  map[string]any    `yaml:"inputs"`   // parent var → sub-workflow input (or operation inputs via shim)
	CallOutputs map[string]string `yaml:"outputs"`  // sub-workflow output → parent var

	// Agent configuration (for AI agent steps - F039)
	// Flat structure: provider, prompt, options directly on step
	Provider     string         `yaml:"provider"`      // agent provider: claude, codex, gemini, opencode, custom
	Prompt       string         `yaml:"prompt"`        // prompt template with {{inputs.*}} and {{states.*}}
	PromptFile   string         `yaml:"prompt_file"`   // path to external prompt template file
	Options      map[string]any `yaml:"options"`       // provider-specific options (model, temperature, max_tokens, etc.)
	OutputFormat string         `yaml:"output_format"` // output post-processing: json, text (F065)

	// Agent conversation mode (F033) - extends agent configuration
	Mode          string                  `yaml:"mode"`           // execution mode: "single" (default) or "conversation"
	SystemPrompt  string                  `yaml:"system_prompt"`  // system prompt for conversation mode
	InitialPrompt string                  `yaml:"initial_prompt"` // initial user prompt for conversation mode
	Conversation  *yamlConversationConfig `yaml:"conversation"`   // conversation-specific configuration
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
	MaxAttempts        int      `yaml:"max_attempts"`
	InitialDelay       string   `yaml:"initial_delay"`
	MaxDelay           string   `yaml:"max_delay"`
	Backoff            string   `yaml:"backoff"`
	Multiplier         *float64 `yaml:"multiplier"`
	Jitter             float64  `yaml:"jitter"`
	RetryableExitCodes []int    `yaml:"retryable_exit_codes"`
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

// yamlTemplate is the YAML representation of a workflow template.
//
//nolint:unused // F017 stub - will be used in template_repository.go
type yamlTemplate struct {
	Name       string              `yaml:"name"`
	Parameters []yamlTemplateParam `yaml:"parameters"`
	States     yamlStates          `yaml:"states"`
}

// yamlTemplateParam is the YAML representation of a template parameter.
//
//nolint:unused // F017 stub - will be used in template_repository.go
type yamlTemplateParam struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
	Default  any    `yaml:"default"`
}

// yamlConversationConfig is the YAML representation of conversation configuration.
// F033: Agent conversations with context window management.
type yamlConversationConfig struct {
	MaxTurns         int    `yaml:"max_turns"`          // maximum number of turns (default 10, max 100)
	MaxContextTokens int    `yaml:"max_context_tokens"` // maximum tokens in context window (0 = provider default)
	Strategy         string `yaml:"strategy"`           // context window strategy: sliding_window, summarize, truncate_middle
	StopCondition    string `yaml:"stop_condition"`     // expression to evaluate for early exit
	ContinueFrom     string `yaml:"continue_from"`      // step name to continue conversation from
	InjectContext    string `yaml:"inject_context"`     // additional context to inject mid-conversation
}
