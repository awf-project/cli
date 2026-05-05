package agents

import (
	"errors"
	"fmt"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

// Compile-time assertion that AgentRegistry implements ports.AgentRegistry
var _ ports.AgentRegistry = (*AgentRegistry)(nil)

// AgentRegistry manages registered agent providers.
type AgentRegistry struct {
	mu        sync.RWMutex
	providers map[string]ports.AgentProvider
}

func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		providers: make(map[string]ports.AgentProvider),
	}
}

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

func (r *AgentRegistry) Get(name string) (ports.AgentProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %q not found", name)
	}

	return provider, nil
}

func (r *AgentRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

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
		NewOpenAICompatibleProvider(),
		NewOpenCodeProvider(),
		NewCopilotProvider(),
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
