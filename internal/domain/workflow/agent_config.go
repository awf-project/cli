package workflow

import (
	"errors"
	"strings"
	"time"
)

// DefaultAgentTimeout is the default timeout in seconds for agent execution.
const DefaultAgentTimeout = 300

// AgentConfig holds configuration for invoking an AI agent.
type AgentConfig struct {
	Provider      string              `yaml:"provider"`       // agent provider: claude, codex, gemini, opencode, custom
	Prompt        string              `yaml:"prompt"`         // prompt template with {{inputs.*}} and {{states.*}} (single mode) or initial prompt (conversation mode)
	Options       map[string]any      `yaml:"options"`        // provider-specific options (model, temperature, max_tokens, etc.)
	Timeout       int                 `yaml:"timeout"`        // seconds, 0 = use DefaultAgentTimeout
	Command       string              `yaml:"command"`        // custom command template (for custom provider)
	Mode          string              `yaml:"mode"`           // execution mode: "single" (default) or "conversation"
	SystemPrompt  string              `yaml:"system_prompt"`  // system prompt preserved across conversation (conversation mode only)
	InitialPrompt string              `yaml:"initial_prompt"` // first user message in conversation mode (overrides Prompt if set)
	Conversation  *ConversationConfig `yaml:"conversation"`   // conversation-specific configuration (conversation mode only)
}

// Validate checks if the agent configuration is valid.
// The validator parameter is used to check expression syntax in conversation config.
func (c *AgentConfig) Validate(validator ExpressionCompiler) error {
	// Validate provider (required, non-empty after trimming)
	c.Provider = strings.TrimSpace(c.Provider)
	if c.Provider == "" {
		return errors.New("provider is required")
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

	// Mode-specific validation
	if c.Mode == "conversation" {
		// In conversation mode, require either InitialPrompt or Prompt
		if c.InitialPrompt == "" && c.Prompt == "" {
			return errors.New("initial_prompt or prompt is required in conversation mode")
		}
		// Validate ConversationConfig if present
		if c.Conversation != nil {
			if err := c.Conversation.Validate(validator); err != nil {
				return err
			}
		}
	} else if c.Prompt == "" {
		// In single mode, require Prompt
		return errors.New("prompt is required")
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

// GetEffectivePrompt returns the appropriate prompt based on the mode.
// In conversation mode, returns InitialPrompt if set, otherwise Prompt.
// In single mode, returns Prompt.
func (c *AgentConfig) GetEffectivePrompt() string {
	if c.IsConversationMode() && c.InitialPrompt != "" {
		return c.InitialPrompt
	}
	return c.Prompt
}

// AgentResult holds the result of an agent execution.
type AgentResult struct {
	Provider        string         // provider name used
	Output          string         // raw output from agent CLI
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
