package ports

import "context"

// PluginVersionLookup reports the version of an awf plugin that is installed and enabled.
type PluginVersionLookup interface {
	// EnabledPluginVersion returns the version of the named plugin, and false when it is not installed or not enabled.
	EnabledPluginVersion(name string) (version string, ok bool)
}

// AgentPluginRegistry reports the plugins installed for an agent CLI, such as the plugins of Claude Code.
type AgentPluginRegistry interface {
	// Supports reports whether the registry can check the plugins of the given agent provider.
	Supports(provider string) bool
	// Installed reports whether the plugin is installed for the provider.
	Installed(ctx context.Context, provider, plugin string) (bool, error)
}
