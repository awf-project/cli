package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/vanoix/awf/internal/domain/plugin"
	"github.com/vanoix/awf/internal/domain/ports"
)

// ErrPluginDisabled indicates the plugin is disabled and cannot be loaded.
var ErrPluginDisabled = errors.New("plugin is disabled")

// ErrPluginNameEmpty indicates an empty plugin name was provided.
var ErrPluginNameEmpty = errors.New("plugin name cannot be empty")

// PluginService orchestrates plugin lifecycle operations.
// It coordinates between the PluginManager (loading/init/shutdown)
// and PluginStateStore (enable/disable persistence).
type PluginService struct {
	manager    ports.PluginManager
	stateStore ports.PluginStateStore
	logger     ports.Logger
}

// NewPluginService creates a new PluginService with injected dependencies.
func NewPluginService(
	manager ports.PluginManager,
	stateStore ports.PluginStateStore,
	logger ports.Logger,
) *PluginService {
	return &PluginService{
		manager:    manager,
		stateStore: stateStore,
		logger:     logger,
	}
}

// DiscoverPlugins scans the plugins directory and returns discovered plugins.
// It filters out disabled plugins based on the state store.
func (s *PluginService) DiscoverPlugins(ctx context.Context) ([]*plugin.PluginInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	if s.manager == nil {
		return nil, nil
	}

	discovered, err := s.manager.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}

	// Filter out disabled plugins
	if s.stateStore == nil {
		return discovered, nil
	}

	enabled := make([]*plugin.PluginInfo, 0, len(discovered))
	for _, p := range discovered {
		if p.Manifest != nil && s.stateStore.IsEnabled(p.Manifest.Name) {
			enabled = append(enabled, p)
		}
	}

	return enabled, nil
}

// LoadPlugin loads a single plugin by name.
// Returns an error if the plugin is disabled.
func (s *PluginService) LoadPlugin(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if name == "" {
		return ErrPluginNameEmpty
	}

	// Check if plugin is disabled
	if s.stateStore != nil && !s.stateStore.IsEnabled(name) {
		return ErrPluginDisabled
	}

	if s.manager == nil {
		return nil
	}

	if err := s.manager.Load(ctx, name); err != nil {
		return fmt.Errorf("load plugin %s: %w", name, err)
	}
	return nil
}

// InitPlugin initializes a loaded plugin with stored configuration.
// The configuration is retrieved from the state store.
func (s *PluginService) InitPlugin(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if name == "" {
		return ErrPluginNameEmpty
	}

	if s.manager == nil {
		return nil
	}

	// Get stored config from state store
	var config map[string]any
	if s.stateStore != nil {
		config = s.stateStore.GetConfig(name)
	}

	if err := s.manager.Init(ctx, name, config); err != nil {
		return fmt.Errorf("init plugin %s: %w", name, err)
	}
	return nil
}

// LoadAndInitPlugin loads and initializes a plugin in one operation.
// Convenience method for typical plugin startup.
func (s *PluginService) LoadAndInitPlugin(ctx context.Context, name string) error {
	if err := s.LoadPlugin(ctx, name); err != nil {
		return err
	}
	return s.InitPlugin(ctx, name)
}

// ShutdownPlugin gracefully stops a running plugin.
func (s *PluginService) ShutdownPlugin(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if s.manager == nil {
		return nil
	}

	if err := s.manager.Shutdown(ctx, name); err != nil {
		return fmt.Errorf("shutdown plugin %s: %w", name, err)
	}
	return nil
}

// ShutdownAll gracefully stops all running plugins.
func (s *PluginService) ShutdownAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if s.manager == nil {
		return nil
	}

	if err := s.manager.ShutdownAll(ctx); err != nil {
		return fmt.Errorf("shutdown all plugins: %w", err)
	}
	return nil
}

// EnablePlugin enables a plugin and persists the state.
// Does not automatically load or initialize the plugin.
func (s *PluginService) EnablePlugin(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if s.stateStore == nil {
		return nil
	}

	if err := s.stateStore.SetEnabled(ctx, name, true); err != nil {
		return fmt.Errorf("enable plugin %s: %w", name, err)
	}
	return nil
}

// DisablePlugin disables a plugin, shuts it down if running, and persists the state.
func (s *PluginService) DisablePlugin(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Shutdown the plugin if it's running
	if s.manager != nil {
		if info, found := s.manager.Get(name); found {
			if info.Status == plugin.StatusRunning || info.Status == plugin.StatusInitialized {
				if err := s.manager.Shutdown(ctx, name); err != nil {
					return fmt.Errorf("shutdown plugin %s: %w", name, err)
				}
			}
		}
	}

	// Persist the disabled state
	if s.stateStore == nil {
		return nil
	}

	if err := s.stateStore.SetEnabled(ctx, name, false); err != nil {
		return fmt.Errorf("disable plugin %s: %w", name, err)
	}
	return nil
}

// SetPluginConfig stores configuration for a plugin.
func (s *PluginService) SetPluginConfig(ctx context.Context, name string, config map[string]any) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if s.stateStore == nil {
		return nil
	}

	if err := s.stateStore.SetConfig(ctx, name, config); err != nil {
		return fmt.Errorf("set config for plugin %s: %w", name, err)
	}
	return nil
}

// GetPluginConfig retrieves stored configuration for a plugin.
func (s *PluginService) GetPluginConfig(name string) map[string]any {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.GetConfig(name)
}

// GetPlugin returns plugin info by name.
// Returns (nil, false) if plugin not found.
func (s *PluginService) GetPlugin(name string) (*plugin.PluginInfo, bool) {
	if s.manager == nil {
		return nil, false
	}
	return s.manager.Get(name)
}

// ListPlugins returns all known plugins.
func (s *PluginService) ListPlugins() []*plugin.PluginInfo {
	if s.manager == nil {
		return nil
	}
	return s.manager.List()
}

// ListEnabledPlugins returns only enabled plugins.
func (s *PluginService) ListEnabledPlugins() []*plugin.PluginInfo {
	if s.manager == nil {
		return nil
	}

	all := s.manager.List()
	if s.stateStore == nil {
		return all
	}

	enabled := make([]*plugin.PluginInfo, 0, len(all))
	for _, p := range all {
		if p.Manifest != nil && s.stateStore.IsEnabled(p.Manifest.Name) {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// ListDisabledPlugins returns names of disabled plugins.
func (s *PluginService) ListDisabledPlugins() []string {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.ListDisabled()
}

// IsPluginEnabled returns whether a plugin is enabled.
func (s *PluginService) IsPluginEnabled(name string) bool {
	if s.stateStore == nil {
		return true // Default: enabled
	}
	return s.stateStore.IsEnabled(name)
}

// SaveState persists all plugin states to storage.
func (s *PluginService) SaveState(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if s.stateStore == nil {
		return nil
	}

	if err := s.stateStore.Save(ctx); err != nil {
		return fmt.Errorf("save plugin state: %w", err)
	}
	return nil
}

// LoadState loads plugin states from storage.
func (s *PluginService) LoadState(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	if s.stateStore == nil {
		return nil
	}

	if err := s.stateStore.Load(ctx); err != nil {
		return fmt.Errorf("load plugin state: %w", err)
	}
	return nil
}

// StartupEnabledPlugins discovers, loads, and initializes all enabled plugins.
// Typically called at application startup.
func (s *PluginService) StartupEnabledPlugins(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Discover plugins
	discovered, err := s.DiscoverPlugins(ctx)
	if err != nil {
		return err // Already wrapped in DiscoverPlugins
	}

	// Load and init each enabled plugin, collecting errors
	var errs []error
	for _, p := range discovered {
		if p.Manifest == nil {
			continue
		}

		name := p.Manifest.Name
		if err := s.LoadAndInitPlugin(ctx, name); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
