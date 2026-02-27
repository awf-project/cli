package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// AgentProvider defines the contract for executing AI agent CLI commands.
// Implementations adapt specific agent CLIs (Claude, Codex, Gemini, etc.)
// to this unified interface.
type AgentProvider interface {
	// Execute invokes the agent with the given prompt and options.
	// Returns AgentResult containing output, parsed response, token usage, and any errors.
	Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error)

	// ExecuteConversation invokes the agent with conversation history for multi-turn interactions.
	// The state parameter contains the conversation history (turns) to send to the agent.
	// Returns ConversationResult containing the updated conversation state, final output, and token usage.
	ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error)

	// Name returns the provider identifier (e.g., "claude", "codex", "gemini").
	Name() string

	// Validate checks if the provider is properly configured and available.
	// Returns error if the agent CLI binary is not found or misconfigured.
	Validate() error
}

// AgentRegistry manages available agent providers and resolves them by name.
type AgentRegistry interface {
	// Register adds a provider to the registry.
	// Returns error if a provider with the same name already exists.
	Register(provider AgentProvider) error

	// Get retrieves a provider by name.
	// Returns error if provider is not found.
	Get(name string) (AgentProvider, error)

	// List returns all registered provider names.
	List() []string

	// Has checks if a provider with the given name is registered.
	Has(name string) bool
}
