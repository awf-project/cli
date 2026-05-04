package pluginmgr

import (
	"context"
	"os"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
)

// SystemResult contains the initialized plugin system components.
type SystemResult struct {
	Service    *application.PluginService
	Manager    ports.OperationProvider
	RPCManager *RPCPluginManager
	StateStore *JSONPluginStateStore
	Cleanup    func()
}

// InitSystem initializes the plugin infrastructure from the given directories.
// pluginDirs may be nil or empty — returns a stub service with graceful degradation.
// stateStorePath: directory for persisting plugin enable/disable state.
// logger may be nil; log calls are skipped when it is nil.
func InitSystem(ctx context.Context, pluginDirs []string, stateStorePath string, logger ports.Logger) (*SystemResult, error) {
	stateStore := NewJSONPluginStateStore(stateStorePath)
	if err := stateStore.Load(ctx); err != nil {
		if logger != nil {
			logger.Warn("failed to load plugin states, using defaults", "error", err)
		}
	}

	existingDirs := findExistingPluginDirs(pluginDirs)

	if len(existingDirs) == 0 {
		service := application.NewPluginService(nil, stateStore, logger)
		return &SystemResult{
			Service:    service,
			StateStore: stateStore,
			Cleanup:    func() {},
		}, nil
	}

	parser := NewManifestParser()
	loader := NewFileSystemLoader(parser)
	manager := NewRPCPluginManager(loader)
	manager.SetPluginsDirs(existingDirs)

	service := application.NewPluginService(manager, stateStore, logger)

	if err := service.StartupEnabledPlugins(ctx); err != nil {
		if logger != nil {
			logger.Warn("some plugins failed to start", "error", err)
		}
	}

	cleanup := func() {
		shutdownCtx := context.Background()
		if err := service.ShutdownAll(shutdownCtx); err != nil {
			if logger != nil {
				logger.Error("failed to shutdown plugins", "error", err)
			}
		}
		if err := service.SaveState(shutdownCtx); err != nil {
			if logger != nil {
				logger.Error("failed to save plugin state", "error", err)
			}
		}
	}

	return &SystemResult{
		Service:    service,
		Manager:    manager,
		RPCManager: manager,
		StateStore: stateStore,
		Cleanup:    cleanup,
	}, nil
}

// findExistingPluginDirs returns all paths that exist and are directories.
func findExistingPluginDirs(paths []string) []string {
	var dirs []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			dirs = append(dirs, path)
		}
	}
	return dirs
}
