package repository

import (
	"fmt"
	"strings"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"gopkg.in/yaml.v3"
)

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
	Plugins     map[string]string  `yaml:"plugins"` // alias → manifest name (e.g. pg: database)
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

	// Plugin step type configuration (C069)
	Config map[string]any `yaml:"config"` // plugin-provided step type parameters

	// Agent conversation mode (F033) - extends agent configuration
	Mode         string                  `yaml:"mode"`           // execution mode: "single" (default) or "conversation"
	SystemPrompt string                  `yaml:"system_prompt"`  // system prompt for conversation mode
	Role         string                  `yaml:"role,omitempty"` // agent role name or path (F098)
	Conversation *yamlConversationConfig `yaml:"conversation"`   // conversation-specific configuration

	// Skill references (F096) - polymorphic: string (name) or map{"path": "..."} (path-based)
	Skills []any `yaml:"skills"`

	// MCP proxy configuration (F099)
	MCPProxy *yamlMCPProxy `yaml:"mcp_proxy,omitempty"`
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
type yamlConversationConfig struct {
	ContinueFrom string `yaml:"continue_from"`
}

// yamlPluginToolExpose is the YAML representation of a plugin tool exposure entry.
type yamlPluginToolExpose struct {
	Plugin string   `yaml:"plugin"`
	Expose []string `yaml:"expose"`
}

// yamlMCPProxy is the YAML representation of MCP proxy configuration for an agent step.
type yamlMCPProxy struct {
	Enable            bool                   `yaml:"enable"`
	InterceptBuiltins *bool                  `yaml:"intercept_builtins"`
	PluginTools       []yamlPluginToolExpose `yaml:"plugin_tools"`
}

// yamlMCPProxyAlias is a type alias used during UnmarshalYAML to avoid infinite recursion.
type yamlMCPProxyAlias yamlMCPProxy

// knownMCPProxyKeys lists the valid YAML keys for mcp_proxy blocks.
var knownMCPProxyKeys = map[string]bool{
	"enable":             true,
	"intercept_builtins": true,
	"plugin_tools":       true,
}

// UnmarshalYAML implements yaml.Unmarshaler for yamlMCPProxy.
// It validates that no unknown keys are present in the mcp_proxy block,
// collecting ALL unknown keys (per project rule: report all errors, not just the first).
func (m *yamlMCPProxy) UnmarshalYAML(node *yaml.Node) error {
	// Validate unknown keys by walking the mapping node's content pairs.
	// Mapping nodes store content as [key1, value1, key2, value2, ...].
	if node.Kind == yaml.MappingNode {
		var unknownKeys []string
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			if !knownMCPProxyKeys[keyNode.Value] {
				unknownKeys = append(unknownKeys, keyNode.Value)
			}
		}
		if len(unknownKeys) > 0 {
			// Report all unknown keys in a single error message using the canonical error code
			// constant (single source of truth per T002 — never hardcode the string literal).
			return fmt.Errorf("%s: unknown field(s) in mcp_proxy: %s",
				string(domerrors.ErrorCodeUserMCPProxyUnknownKey),
				strings.Join(unknownKeys, ", "))
		}
	}

	// Delegate actual decoding to the alias type to avoid infinite recursion.
	var alias yamlMCPProxyAlias
	if err := node.Decode(&alias); err != nil {
		return err
	}
	*m = yamlMCPProxy(alias)
	return nil
}
