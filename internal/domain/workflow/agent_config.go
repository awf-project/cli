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
	Provider string         `yaml:"provider"` // agent provider: claude, codex, gemini, opencode, custom
	Prompt   string         `yaml:"prompt"`   // prompt template with {{inputs.*}} and {{states.*}}
	Options  map[string]any `yaml:"options"`  // provider-specific options (model, temperature, max_tokens, etc.)
	Timeout  int            `yaml:"timeout"`  // seconds, 0 = use DefaultAgentTimeout
	Command  string         `yaml:"command"`  // custom command template (for custom provider)
}

// Validate checks if the agent configuration is valid.
func (c *AgentConfig) Validate() error {
	// Validate provider (required, non-empty after trimming)
	c.Provider = strings.TrimSpace(c.Provider)
	if c.Provider == "" {
		return errors.New("provider is required")
	}

	// Validate prompt (required, non-empty)
	if c.Prompt == "" {
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
