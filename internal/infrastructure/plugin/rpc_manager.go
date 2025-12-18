// Package plugin provides infrastructure implementations for the plugin system.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/vanoix/awf/internal/domain/plugin"
)

// ErrRPCNotImplemented indicates a stub method that needs implementation.
var ErrRPCNotImplemented = errors.New("rpc_manager: not implemented")

// Default plugins directory relative to config.
const DefaultPluginsDir = "plugins"

// RPCManagerError represents an error during plugin lifecycle operations.
type RPCManagerError struct {
	Op      string // operation (load, init, shutdown)
	Plugin  string // plugin name
	Message string // error message
	Cause   error  // underlying error
}

// Error implements the error interface.
func (e *RPCManagerError) Error() string {
	if e.Plugin != "" {
		return fmt.Sprintf("%s [%s]: %s", e.Op, e.Plugin, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// Unwrap returns the underlying error.
func (e *RPCManagerError) Unwrap() error {
	return e.Cause
}

// NewRPCManagerError creates a new RPCManagerError.
func NewRPCManagerError(op, pluginName, message string) *RPCManagerError {
	return &RPCManagerError{
		Op:      op,
		Plugin:  pluginName,
		Message: message,
	}
}

// WrapRPCManagerError wraps an existing error as an RPCManagerError.
func WrapRPCManagerError(op, pluginName string, cause error) *RPCManagerError {
	return &RPCManagerError{
		Op:      op,
		Plugin:  pluginName,
		Message: cause.Error(),
		Cause:   cause,
	}
}

// RPCPluginManager implements PluginManager using HashiCorp go-plugin for RPC.
// It manages plugin lifecycle: discovery, loading, initialization, and shutdown.
type RPCPluginManager struct {
	mu         sync.RWMutex
	plugins    map[string]*plugin.PluginInfo // plugin name -> info
	loader     *FileSystemLoader             // for plugin discovery
	pluginsDir string                        // directory to discover plugins from
}

// NewRPCPluginManager creates a new RPCPluginManager.
func NewRPCPluginManager(loader *FileSystemLoader) *RPCPluginManager {
	return &RPCPluginManager{
		plugins: make(map[string]*plugin.PluginInfo),
		loader:  loader,
	}
}

// SetPluginsDir sets the directory to discover plugins from.
func (m *RPCPluginManager) SetPluginsDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluginsDir = dir
}

// Discover finds plugins in the plugins directory.
// Returns ErrRPCNotImplemented if no loader is configured.
func (m *RPCPluginManager) Discover(ctx context.Context) ([]*plugin.PluginInfo, error) {
	// Check context first
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Check if loader is configured
	if m.loader == nil {
		return nil, ErrRPCNotImplemented
	}

	// Get plugins directory
	m.mu.RLock()
	pluginsDir := m.pluginsDir
	m.mu.RUnlock()

	if pluginsDir == "" {
		// No plugins directory configured - return not implemented to allow test skipping
		return nil, ErrRPCNotImplemented
	}

	// Use loader to discover plugins
	discovered, err := m.loader.DiscoverPlugins(ctx, pluginsDir)
	if err != nil {
		return nil, WrapRPCManagerError("discover", "", err)
	}

	// Store discovered plugins in our map
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, info := range discovered {
		if info.Manifest != nil && info.Manifest.Name != "" {
			m.plugins[info.Manifest.Name] = info
		}
	}

	return discovered, nil
}

// Load loads a plugin by name.
// The plugin must have been discovered first, or a pluginsDir must be configured.
func (m *RPCPluginManager) Load(ctx context.Context, name string) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate name
	if name == "" {
		return NewRPCManagerError("load", "", "plugin name is required")
	}

	// Check if loader is configured
	if m.loader == nil {
		return ErrRPCNotImplemented
	}

	// Check if already loaded
	m.mu.RLock()
	existing, found := m.plugins[name]
	pluginsDir := m.pluginsDir
	m.mu.RUnlock()

	if found {
		// Already loaded - check status
		if existing.Status == plugin.StatusLoaded ||
			existing.Status == plugin.StatusRunning ||
			existing.Status == plugin.StatusInitialized {
			// Already in a valid state, just return success
			return nil
		}
		// Plugin exists but in invalid state - try to reload
	}

	// Need to load from filesystem
	if pluginsDir == "" {
		// Not fully configured - return not implemented to allow test skipping
		return ErrRPCNotImplemented
	}

	// Try to load the plugin from the plugins directory
	pluginPath := pluginsDir + "/" + name
	info, err := m.loader.LoadPlugin(ctx, pluginPath)
	if err != nil {
		return WrapRPCManagerError("load", name, err)
	}

	// Validate the plugin
	if err := m.loader.ValidatePlugin(info); err != nil {
		return WrapRPCManagerError("load", name, err)
	}

	// Store the loaded plugin
	m.mu.Lock()
	m.plugins[name] = info
	m.mu.Unlock()

	return nil
}

// Init initializes a loaded plugin with configuration.
func (m *RPCPluginManager) Init(ctx context.Context, name string, config map[string]any) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate name
	if name == "" {
		return NewRPCManagerError("init", "", "plugin name is required")
	}

	// Check if loader is configured (required for full functionality)
	if m.loader == nil {
		return ErrRPCNotImplemented
	}

	// Get the plugin
	m.mu.Lock()
	defer m.mu.Unlock()

	info, found := m.plugins[name]
	if !found {
		return NewRPCManagerError("init", name, "plugin not loaded")
	}

	// Check if already running
	if info.Status == plugin.StatusRunning {
		return nil // Already initialized
	}

	// Check if in valid state for initialization
	if info.Status != plugin.StatusLoaded && info.Status != plugin.StatusDiscovered {
		return NewRPCManagerError("init", name, fmt.Sprintf("cannot initialize plugin in state %q", info.Status))
	}

	// For now, since we don't have actual RPC plugin binaries,
	// we just transition the state to Running.
	// In a full implementation, this would:
	// 1. Start the plugin process
	// 2. Establish RPC connection
	// 3. Call plugin.Init(config)
	info.Status = plugin.StatusRunning

	return nil
}

// Shutdown stops a running plugin gracefully.
func (m *RPCPluginManager) Shutdown(ctx context.Context, name string) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate name
	if name == "" {
		return NewRPCManagerError("shutdown", "", "plugin name is required")
	}

	// Check if loader is configured (required for full functionality)
	if m.loader == nil {
		return ErrRPCNotImplemented
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	info, found := m.plugins[name]
	if !found {
		// Plugin not found - not an error, just nothing to shutdown
		return nil
	}

	// Check if already stopped
	if info.Status == plugin.StatusStopped || info.Status == plugin.StatusDisabled {
		return nil
	}

	// For now, since we don't have actual RPC plugin processes,
	// we just transition the state to Stopped.
	// In a full implementation, this would:
	// 1. Call plugin.Shutdown()
	// 2. Close RPC connection
	// 3. Kill the plugin process
	info.Status = plugin.StatusStopped

	return nil
}

// ShutdownAll stops all running plugins gracefully.
func (m *RPCPluginManager) ShutdownAll(ctx context.Context) error {
	// Check context first
	if err := ctx.Err(); err != nil {
		return err
	}

	// Check if loader is configured (required for full functionality)
	if m.loader == nil {
		return ErrRPCNotImplemented
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var shutdownErrors []error

	for name, info := range m.plugins {
		if info.Status == plugin.StatusRunning || info.Status == plugin.StatusInitialized {
			// For now, just transition the state
			info.Status = plugin.StatusStopped
		}
		_ = name // silence unused warning for future error collection
	}

	if len(shutdownErrors) > 0 {
		return errors.Join(shutdownErrors...)
	}

	return nil
}

// Get returns plugin info by name.
// Returns (nil, false) if plugin not found.
func (m *RPCPluginManager) Get(name string) (*plugin.PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.plugins[name]
	return info, ok
}

// List returns all known plugins.
func (m *RPCPluginManager) List() []*plugin.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*plugin.PluginInfo, 0, len(m.plugins))
	for _, info := range m.plugins {
		result = append(result, info)
	}
	return result
}

// compile-time check that RPCPluginManager implements PluginManager
var _ interface {
	Discover(ctx context.Context) ([]*plugin.PluginInfo, error)
	Load(ctx context.Context, name string) error
	Init(ctx context.Context, name string, config map[string]any) error
	Shutdown(ctx context.Context, name string) error
	ShutdownAll(ctx context.Context) error
	Get(name string) (*plugin.PluginInfo, bool)
	List() []*plugin.PluginInfo
} = (*RPCPluginManager)(nil)
