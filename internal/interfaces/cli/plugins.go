package cli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	infrastructurePlugin "github.com/awf-project/cli/internal/infrastructure/pluginmgr"
)

// PluginSystemResult contains the initialized plugin system components.
type PluginSystemResult struct {
	Service    *application.PluginService
	Manager    ports.OperationProvider
	RPCManager *infrastructurePlugin.RPCPluginManager // for validator/step-type providers (C069)
	StateStore *infrastructurePlugin.JSONPluginStateStore
	Cleanup    func()
}

// initPluginSystem initializes the plugin infrastructure for workflow execution.
// Returns a PluginSystemResult containing the PluginService and a cleanup function.
// The cleanup function must be deferred to ensure proper plugin shutdown.
//
// If plugin directories don't exist or contain no plugins, the function succeeds
// with an empty plugin service (graceful degradation).
func initPluginSystem(ctx context.Context, cfg *Config, logger ports.Logger) (*PluginSystemResult, error) {
	// Get plugin paths
	pluginPaths := getPluginSearchPaths(cfg)

	// Find all existing plugin directories
	pluginsDirs := findExistingDirs(pluginPaths)

	// Create state store for plugin enable/disable persistence
	stateStorePath := filepath.Join(cfg.StoragePath, "plugins")
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(stateStorePath)

	// Load persisted plugin states
	if err := stateStore.Load(ctx); err != nil {
		if logger != nil {
			logger.Warn("failed to load plugin states, using defaults", "error", err)
		}
	}

	// If no plugins directory exists, return a stub service (graceful degradation)
	if len(pluginsDirs) == 0 {
		// Create service with nil manager (no plugins available)
		service := application.NewPluginService(nil, stateStore, logger)
		registerBuiltins(service, Version)
		return &PluginSystemResult{
			Service:    service,
			StateStore: stateStore,
			Cleanup:    func() {},
		}, nil
	}

	// Initialize plugin infrastructure
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)
	manager := infrastructurePlugin.NewRPCPluginManager(loader)
	manager.SetPluginsDirs(pluginsDirs)

	// Create the plugin service
	service := application.NewPluginService(manager, stateStore, logger)
	registerBuiltins(service, Version)

	// Startup enabled plugins
	if err := service.StartupEnabledPlugins(ctx); err != nil {
		if logger != nil {
			logger.Warn("some plugins failed to start", "error", err)
		}
		// Don't fail workflow execution due to plugin failures
	}

	// Create cleanup function
	cleanup := func() {
		shutdownCtx := context.Background()
		if err := service.ShutdownAll(shutdownCtx); err != nil {
			if logger != nil {
				logger.Error("failed to shutdown plugins", "error", err)
			}
		}
		// Save state after shutdown
		if err := service.SaveState(shutdownCtx); err != nil {
			if logger != nil {
				logger.Error("failed to save plugin state", "error", err)
			}
		}
	}

	return &PluginSystemResult{
		Service:    service,
		Manager:    manager,
		RPCManager: manager,
		StateStore: stateStore,
		Cleanup:    cleanup,
	}, nil
}

// getPluginSearchPaths returns the plugin directories to search.
// If cfg.PluginsDir is set, it takes priority over BuildPluginPaths.
func getPluginSearchPaths(cfg *Config) []string {
	if cfg.PluginsDir != "" {
		return []string{cfg.PluginsDir}
	}

	sourcedPaths := BuildPluginPaths()
	paths := make([]string, 0, len(sourcedPaths))
	for _, sp := range sourcedPaths {
		paths = append(paths, sp.Path)
	}
	return paths
}

// findExistingDirs returns all directories that exist from the given paths.
func findExistingDirs(paths []string) []string {
	var dirs []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			dirs = append(dirs, path)
		}
	}
	return dirs
}

// findFirstExistingDir returns the first directory that exists, or empty string if none.
func findFirstExistingDir(paths []string) string {
	dirs := findExistingDirs(paths)
	if len(dirs) > 0 {
		return dirs[0]
	}
	return ""
}

// findPluginDir locates an installed plugin by name across all search paths.
// Tries the exact name first, then the short name (without "awf-plugin-" prefix)
// to handle the mismatch between manifest names and install directory names.
// Returns the full path to the plugin directory, or empty string if not found.
func findPluginDir(paths []string, name string) string {
	candidates := []string{name}
	if short := extractPluginName(name); short != name {
		candidates = append(candidates, short)
	}

	for _, dir := range findExistingDirs(paths) {
		for _, candidate := range candidates {
			pluginDir := filepath.Join(dir, candidate)
			if info, err := os.Stat(pluginDir); err == nil && info.IsDir() {
				return pluginDir
			}
		}
	}
	return ""
}

// resolvePluginStateName returns the name used in the state store for a plugin.
// Tries the exact name first, then the short name (without "awf-plugin-" prefix).
func resolvePluginStateName(getName func(string) map[string]any, name string) string {
	if data := getName(name); data != nil {
		return name
	}
	if short := extractPluginName(name); short != name {
		if data := getName(short); data != nil {
			return short
		}
	}
	return name
}

// registerBuiltins registers the built-in operation providers into the plugin service.
// Uses version as the synthesized manifest version for each built-in entry.
func registerBuiltins(svc *application.PluginService, version string) {
	svc.RegisterBuiltin("github", "GitHub operation provider", version, []string{
		"github.get_issue",
		"github.get_pr",
		"github.create_pr",
		"github.create_issue",
		"github.add_labels",
		"github.list_comments",
		"github.add_comment",
		"github.batch",
	})
	svc.RegisterBuiltin("notify", "Notification operation provider", version, []string{
		"notify.send",
	})
	svc.RegisterBuiltin("http", "HTTP operation provider", version, []string{
		"http.request",
	})
}
