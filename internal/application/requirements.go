package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/registry"
)

// SetPluginVersionLookup configures the lookup that checks the `requires.plugins` of a workflow.
// Without it, the plugin requirements are not checked.
func (s *WorkflowService) SetPluginVersionLookup(lookup ports.PluginVersionLookup) {
	s.pluginVersions = lookup
}

// SetAgentPluginRegistry configures the registry that checks the `requires.agent_plugins` of a workflow.
// Without it, the agent plugin requirements are not checked.
func (s *WorkflowService) SetAgentPluginRegistry(agentPluginRegistry ports.AgentPluginRegistry) {
	s.agentPlugins = agentPluginRegistry
}

// validateRequirements checks the `requires:` block of a workflow before its steps, so that a missing plugin is named
// with the command that installs it rather than reported as an unknown operation. With SkipPlugins, the awf plugins
// are not checked.
func (s *WorkflowService) validateRequirements(ctx context.Context, wf *workflow.Workflow, opts ports.ValidateOptions) error {
	var errs []error
	if s.pluginVersions != nil && !opts.SkipPlugins {
		for _, name := range sortedKeys(wf.Requires.Plugins) {
			if err := s.checkPluginRequirement(name, wf.Requires.Plugins[name]); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if s.agentPlugins != nil {
		for _, provider := range sortedKeys(wf.Requires.AgentPlugins) {
			for _, plugin := range wf.Requires.AgentPlugins[provider] {
				if err := s.checkAgentPluginRequirement(ctx, provider, plugin); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	return errors.Join(errs...)
}

func (s *WorkflowService) checkPluginRequirement(name, constraint string) error {
	path := "requires.plugins." + name
	version, ok := s.pluginVersions.EnabledPluginVersion(name)
	if !ok {
		return requirementError(path, fmt.Sprintf("the workflow requires the awf plugin %q, which is not installed or not enabled: install it with `awf plugin install <owner>/<repository>`, then run `awf plugin enable %s`", name, name))
	}
	if constraint == "" {
		return nil
	}
	constraints, err := registry.ParseConstraints(constraint)
	if err != nil {
		return requirementError(path, fmt.Sprintf("the version constraint %q of the awf plugin %q is not valid: %v", constraint, name, err))
	}
	installed, err := registry.ParseVersion(version)
	if err != nil {
		// A plugin without a semantic version (a built-in of a development build reports "dev") cannot be compared:
		// its presence is enough.
		return nil
	}
	if !constraints.Check(installed) {
		return requirementError(path, fmt.Sprintf("the workflow requires the awf plugin %q %s, and version %s is installed: run `awf plugin update %s`", name, constraint, version, name))
	}
	return nil
}

func (s *WorkflowService) checkAgentPluginRequirement(ctx context.Context, provider, plugin string) error {
	path := "requires.agent_plugins." + provider
	if !s.agentPlugins.Supports(provider) {
		return requirementError(path, fmt.Sprintf("awf cannot check the plugins of the agent provider %q; the supported provider is claude", provider))
	}
	installed, err := s.agentPlugins.Installed(ctx, provider, plugin)
	if err != nil {
		return requirementError(path, fmt.Sprintf("the plugins of the agent provider %q cannot be read: %v", provider, err))
	}
	if !installed {
		return requirementError(path, fmt.Sprintf("the workflow requires the %s plugin %q, which is not installed: install it with `claude plugin install %s`", provider, plugin, plugin))
	}
	return nil
}

func requirementError(path, message string) error {
	return workflow.ValidationError{Level: workflow.ValidationLevelError, Code: workflow.ErrRequirementNotMet, Message: message, Path: path}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
