package application

import (
	"maps"

	"github.com/awf-project/cli/internal/domain/ports"
)

// agentAliasResolver is implemented by registries that map an alias to a provider and default options.
type agentAliasResolver interface {
	ResolveAlias(name string) (provider string, options map[string]any)
}

// agentAliasNames returns the sorted alias names of the registry, none when it has no aliases.
func agentAliasNames(registry ports.AgentRegistry) []string {
	if r, ok := registry.(interface{ AliasNames() []string }); ok {
		return r.AliasNames()
	}
	return nil
}

// resolveAgentAlias returns the provider name behind name and the alias options to merge under the step
// options. A registry without aliases, or a name that is not an alias, gives the name back with no options.
func resolveAgentAlias(registry ports.AgentRegistry, name string) (provider string, options map[string]any) {
	if r, ok := registry.(agentAliasResolver); ok {
		return r.ResolveAlias(name)
	}
	return name, nil
}

// mergeAliasOptions returns the alias options overlaid by the step options: the step wins on a conflict.
// It never modifies its arguments.
func mergeAliasOptions(alias, step map[string]any) map[string]any {
	if len(alias) == 0 {
		return step
	}
	merged := maps.Clone(alias)
	maps.Copy(merged, step)
	return merged
}
