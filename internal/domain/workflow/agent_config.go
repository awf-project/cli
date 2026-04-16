package workflow

import (
	"errors"
	"strings"
	"time"
)

// DefaultAgentTimeout is the default timeout in seconds for agent execution.
const DefaultAgentTimeout = 300

// OutputFormat represents the format for agent output post-processing.
type OutputFormat string

const (
	OutputFormatNone OutputFormat = ""
	OutputFormatJSON OutputFormat = "json"
	OutputFormatText OutputFormat = "text"
)

var validOutputFormats = map[OutputFormat]bool{
	OutputFormatNone: true,
	OutputFormatJSON: true,
	OutputFormatText: true,
}

// AgentConfig holds configuration for invoking an AI agent.
type AgentConfig struct {
	Provider     string              `yaml:"provider"`      // agent provider: claude, cursor, codex, gemini, opencode, openai_compatible
	Prompt       string              `yaml:"prompt"`        // prompt template with {{inputs.*}} and {{states.*}} (single mode) or first user message (conversation mode)
	PromptFile   string              `yaml:"prompt_file"`   // path to external prompt template file (mutually exclusive with Prompt)
	Options      map[string]any      `yaml:"options"`       // provider-specific options (model, temperature, max_tokens, etc.)
	Timeout      int                 `yaml:"timeout"`       // seconds, 0 = use DefaultAgentTimeout
	Mode         string              `yaml:"mode"`          // execution mode: "single" (default) or "conversation"
	SystemPrompt string              `yaml:"system_prompt"` // system prompt preserved across conversation (conversation mode only)
	Conversation *ConversationConfig `yaml:"conversation"`  // conversation-specific configuration (conversation mode only)
	OutputFormat OutputFormat        `yaml:"output_format"` // output post-processing: json (strip fences + validate), text (strip fences only), or empty (no processing)
}

// Validate checks if the agent configuration is valid.
// The validator parameter is used to check expression syntax in conversation config.
func (c *AgentConfig) Validate(validator ExpressionCompiler) error {
	// Validate provider (required, non-empty after trimming)
	c.Provider = strings.TrimSpace(c.Provider)
	if c.Provider == "" {
		return errors.New("provider is required")
	}

	// Reject deprecated custom provider with migration guidance
	if c.Provider == "custom" {
		return errors.New("provider 'custom' has been removed; use 'type: step' for shell commands or 'provider: openai_compatible' for LLM API calls")
	}

	// Normalize mode (default to "single")
	c.Mode = strings.TrimSpace(strings.ToLower(c.Mode))
	if c.Mode == "" {
		c.Mode = "single"
	}

	// Validate mode value
	if c.Mode != "single" && c.Mode != "conversation" {
		return errors.New("mode must be 'single' or 'conversation'")
	}

	// Normalize and validate output_format
	c.OutputFormat = OutputFormat(strings.TrimSpace(strings.ToLower(string(c.OutputFormat))))
	if !validOutputFormats[c.OutputFormat] {
		return errors.New("output_format must be 'json', 'text', or empty")
	}

	// Validate mutual exclusivity of Prompt and PromptFile
	if c.Prompt != "" && c.PromptFile != "" {
		return errors.New("prompt and prompt_file are mutually exclusive")
	}

	// Mode-specific validation
	if c.Mode == "conversation" {
		// prompt_file not supported in conversation mode (deferred per spec)
		if c.PromptFile != "" {
			return errors.New("prompt_file is not supported in conversation mode")
		}
		if c.Prompt == "" {
			return errors.New("prompt is required in conversation mode")
		}
		// Validate ConversationConfig if present
		if c.Conversation != nil {
			if err := c.Conversation.Validate(); err != nil {
				return err
			}
		}
	} else if c.Prompt == "" && c.PromptFile == "" {
		// In single mode, require either Prompt or PromptFile
		return errors.New("prompt or prompt_file is required")
	}

	// Validate timeout (must be non-negative)
	if c.Timeout < 0 {
		return errors.New("timeout must be non-negative")
	}

	return nil
}

// GetTimeout returns the effective timeout as a time.Duration.
// Returns DefaultAgentTimeout seconds if not explicitly set.
func (c *AgentConfig) GetTimeout() time.Duration {
	if c.Timeout > 0 {
		return time.Duration(c.Timeout) * time.Second
	}
	return DefaultAgentTimeout * time.Second
}

// IsConversationMode returns true if the agent is configured for conversation mode.
func (c *AgentConfig) IsConversationMode() bool {
	return c.Mode == "conversation"
}

// GetEffectivePrompt returns the prompt for this agent step.
func (c *AgentConfig) GetEffectivePrompt() string {
	return c.Prompt
}

// AgentResult holds the result of an agent execution.
type AgentResult struct {
	Provider        string         // provider name used
	Output          string         // raw output from agent CLI
	DisplayOutput   string         // filtered human-readable output for display (empty when output_format=json or no parser)
	Response        map[string]any // parsed JSON response (if applicable)
	Tokens          int            // token usage (if reported by provider)
	TokensEstimated bool           // true if Tokens is an estimation, false if actual count
	Error           error          // execution error, if any
	StartedAt       time.Time
	CompletedAt     time.Time
	Conversation    *ConversationResult // conversation-specific result (conversation mode only)
}

// NewAgentResult creates a new AgentResult with initialized values.
func NewAgentResult(provider string) *AgentResult {
	return &AgentResult{
		Provider:  provider,
		Response:  make(map[string]any),
		StartedAt: time.Now(),
	}
}

// Duration returns the execution time of the agent invocation.
func (r *AgentResult) Duration() time.Duration {
	return r.CompletedAt.Sub(r.StartedAt)
}

// Success returns true if the agent execution completed without error.
func (r *AgentResult) Success() bool {
	return r != nil && r.Error == nil
}

// HasJSONResponse returns true if a JSON response was successfully parsed.
func (r *AgentResult) HasJSONResponse() bool {
	return len(r.Response) > 0
}
