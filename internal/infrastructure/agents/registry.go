package agents

import (
	"errors"
	"fmt"
	"sync"

	"github.com/vanoix/awf/internal/domain/ports"
)

// Compile-time assertion that AgentRegistry implements ports.AgentRegistry
var _ ports.AgentRegistry = (*AgentRegistry)(nil)

// AgentRegistry manages registered agent providers.
type AgentRegistry struct {
	mu        sync.RWMutex
	providers map[string]ports.AgentProvider
}

// NewAgentRegistry creates a new AgentRegistry with default providers.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		providers: make(map[string]ports.AgentProvider),
	}
}

// Register registers a new agent provider.
func (r *AgentRegistry) Register(provider ports.AgentProvider) error {
	name := provider.Name()
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// Get retrieves a registered provider by name.
func (r *AgentRegistry) Get(name string) (ports.AgentProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %q not found", name)
	}

	return provider, nil
}

// List returns a list of all registered provider names.
func (r *AgentRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// Has checks if a provider with the given name is registered.
func (r *AgentRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.providers[name]
	return exists
}

// RegisterDefaults registers all default providers.
// It continues registering even if individual providers fail,
// collecting all errors and returning them aggregated.
func (r *AgentRegistry) RegisterDefaults() error {
	defaults := []ports.AgentProvider{
		NewClaudeProvider(),
		NewCodexProvider(),
		NewGeminiProvider(),
		NewOpenCodeProvider(),
	}

	var errs []error
	for _, provider := range defaults {
		if err := r.Register(provider); err != nil {
			// Continue registering other providers even if one fails
			errs = append(errs, fmt.Errorf("failed to register %s: %w", provider.Name(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
