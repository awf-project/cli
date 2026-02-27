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
	Service *application.PluginService
	Cleanup func()
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

	// Find the first existing plugin directory
	pluginsDir := findFirstExistingDir(pluginPaths)

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
	if pluginsDir == "" {
		// Create service with nil manager (no plugins available)
		service := application.NewPluginService(nil, stateStore, logger)
		return &PluginSystemResult{
			Service: service,
			Cleanup: func() {},
		}, nil
	}

	// Initialize plugin infrastructure
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)
	manager := infrastructurePlugin.NewRPCPluginManager(loader)
	manager.SetPluginsDir(pluginsDir)

	// Create the plugin service
	service := application.NewPluginService(manager, stateStore, logger)

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
		Service: service,
		Cleanup: cleanup,
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

// findFirstExistingDir returns the first directory that exists, or empty string if none.
func findFirstExistingDir(paths []string) string {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}
