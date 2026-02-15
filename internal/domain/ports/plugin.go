package ports

import (
	"context"

	"github.com/awf-project/awf/internal/domain/plugin"
)

// Plugin defines the contract that all plugins must implement.
type Plugin interface {
	// Name returns the unique plugin identifier.
	Name() string
	// Version returns the plugin version.
	Version() string
	// Init initializes the plugin with configuration.
	Init(ctx context.Context, config map[string]any) error
	// Shutdown gracefully stops the plugin.
	Shutdown(ctx context.Context) error
}

// PluginManager handles plugin lifecycle operations.
type PluginManager interface {
	// Discover finds plugins in the plugins directory.
	Discover(ctx context.Context) ([]*plugin.PluginInfo, error)
	// Load loads a plugin by name.
	Load(ctx context.Context, name string) error
	// Init initializes a loaded plugin.
	Init(ctx context.Context, name string, config map[string]any) error
	// Shutdown stops a running plugin.
	Shutdown(ctx context.Context, name string) error
	// ShutdownAll stops all running plugins.
	ShutdownAll(ctx context.Context) error
	// Get returns plugin info by name.
	Get(name string) (*plugin.PluginInfo, bool)
	// List returns all known plugins.
	List() []*plugin.PluginInfo
}

// OperationProvider supplies plugin-provided operations.
type OperationProvider interface {
	// GetOperation returns an operation by name.
	GetOperation(name string) (*plugin.OperationSchema, bool)
	// ListOperations returns all available operations.
	ListOperations() []*plugin.OperationSchema
	// Execute runs a plugin operation.
	Execute(ctx context.Context, name string, inputs map[string]any) (*plugin.OperationResult, error)
}

// PluginRegistry manages registration of plugin-provided extensions.
type PluginRegistry interface {
	// RegisterOperation adds a plugin operation.
	RegisterOperation(op *plugin.OperationSchema) error
	// UnregisterOperation removes a plugin operation.
	UnregisterOperation(name string) error
	// Operations returns all registered operations.
	Operations() []*plugin.OperationSchema
}

// PluginLoader discovers and loads plugins from the filesystem.
type PluginLoader interface {
	// DiscoverPlugins scans a directory for plugins and returns their info.
	// Each subdirectory with a plugin.yaml is considered a plugin.
	DiscoverPlugins(ctx context.Context, pluginsDir string) ([]*plugin.PluginInfo, error)
	// LoadPlugin loads a single plugin from a directory path.
	LoadPlugin(ctx context.Context, pluginDir string) (*plugin.PluginInfo, error)
	// ValidatePlugin checks if a discovered plugin is valid and compatible.
	ValidatePlugin(info *plugin.PluginInfo) error
}

// PluginStore handles plugin state persistence.
type PluginStore interface {
	// Save persists all plugin states to storage.
	Save(ctx context.Context) error
	// Load reads plugin states from storage.
	Load(ctx context.Context) error
	// GetState returns the full state for a plugin, or nil if not found.
	GetState(name string) *plugin.PluginState
	// ListDisabled returns names of all explicitly disabled plugins.
	ListDisabled() []string
}

// PluginConfig manages plugin configuration and enabled state.
type PluginConfig interface {
	// SetEnabled enables or disables a plugin by name.
	SetEnabled(ctx context.Context, name string, enabled bool) error
	// IsEnabled returns whether a plugin is enabled.
	IsEnabled(name string) bool
	// GetConfig returns the stored configuration for a plugin.
	GetConfig(name string) map[string]any
	// SetConfig stores configuration for a plugin.
	SetConfig(ctx context.Context, name string, config map[string]any) error
}

// PluginStateStore combines persistence and configuration interfaces.
// Maintains backward compatibility while enabling consumers to use narrower interfaces.
type PluginStateStore interface {
	PluginStore
	PluginConfig
}
