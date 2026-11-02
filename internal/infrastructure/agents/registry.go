package agents

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

// Compile-time assertion that AgentRegistry implements ports.AgentRegistry
var _ ports.AgentRegistry = (*AgentRegistry)(nil)

// AgentRegistry manages registered agent providers.
type AgentRegistry struct {
	mu        sync.RWMutex
	providers map[string]ports.AgentProvider
	aliases   map[string]Alias
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
		if len(r.aliases) > 0 {
			return nil, fmt.Errorf("provider %q not found (known aliases: %s)", name, strings.Join(r.sortedAliasNames(), ", "))
		}
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
//
// Pass nil for cmdExec to disable MCP proxy support for providers that require a
// CommandExecutor (Gemini); attempting to use mcp_proxy with such a provider
// will fail at step startup with a clear error rather than a nil-pointer panic.
// OpenCode MCP proxy uses workspace file config and does not require a CommandExecutor.
func (r *AgentRegistry) RegisterDefaults(cmdExec ports.CommandExecutor, openAIOpts ...OpenAICompatibleProviderOption) error {
	defaults := []ports.AgentProvider{
		NewClaudeProvider(),
		NewCodexProvider(),
		NewGeminiProviderWithOptions(WithGeminiCommandExecutor(cmdExec)),
		NewOpenAICompatibleProvider(openAIOpts...),
		NewOpenCodeProvider(),
		NewCopilotProvider(),
		NewMistralVibeProvider(),
	}

	var errs []error
	for _, provider := range defaults {
		if err := r.Register(provider); err != nil {
			errs = append(errs, fmt.Errorf("failed to register %s: %w", provider.Name(), err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Alias names a provider and default options, so that a workflow can say `provider: einstein`
// and each user decides which tool and model that stands for.
type Alias struct {
	Provider string
	Options  map[string]any
}

// SetAliases replaces the alias table. An alias is resolved before the provider names, so it shadows
// a provider with the same name. Aliases do not chain: Provider must name a registered provider.
func (r *AgentRegistry) SetAliases(aliases map[string]Alias) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases = aliases
}

// ResolveAlias returns the provider name and the default options behind name. A name that is not an
// alias is returned unchanged with no options.
func (r *AgentRegistry) ResolveAlias(name string) (provider string, options map[string]any) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if a, ok := r.aliases[name]; ok {
		return a.Provider, a.Options
	}
	return name, nil
}

// AliasNames returns the alias names in alphabetical order.
func (r *AgentRegistry) AliasNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sortedAliasNames()
}

func (r *AgentRegistry) sortedAliasNames() []string {
	names := make([]string, 0, len(r.aliases))
	for n := range r.aliases {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
