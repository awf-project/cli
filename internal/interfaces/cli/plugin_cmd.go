package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/application"
	infrastructurePlugin "github.com/vanoix/awf/internal/infrastructure/plugin"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

// ErrPluginCLINotImplemented indicates a stub method that needs implementation.
var ErrPluginCLINotImplemented = errors.New("plugin CLI: not implemented")

func newPluginCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage AWF plugins",
		Long: `Manage AWF plugins: list, enable, and disable plugins.

Plugins extend AWF functionality by providing custom operations,
commands, and validators.

Examples:
  awf plugin list
  awf plugin enable slack-notifier
  awf plugin disable slack-notifier`,
		Aliases: []string{"plugins"},
	}

	cmd.AddCommand(newPluginListCommand(cfg))
	cmd.AddCommand(newPluginEnableCommand(cfg))
	cmd.AddCommand(newPluginDisableCommand(cfg))

	return cmd
}

func newPluginListCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all available plugins",
		Long:    "Display all discovered plugins with their status and capabilities.",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginList(cmd, cfg)
		},
	}
}

func newPluginEnableCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <plugin-name>",
		Short: "Enable a plugin",
		Long: `Enable a plugin by name. The plugin will be loaded and initialized
on next workflow execution or application startup.

Examples:
  awf plugin enable slack-notifier`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginEnable(cmd, cfg, args[0])
		},
	}
}

func newPluginDisableCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <plugin-name>",
		Short: "Disable a plugin",
		Long: `Disable a plugin by name. The plugin will be shut down if running
and will not be loaded on next startup.

Examples:
  awf plugin disable slack-notifier`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginDisable(cmd, cfg, args[0])
		},
	}
}

func runPluginList(cmd *cobra.Command, cfg *Config) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)

	// Initialize plugin system (read-only mode)
	result, err := initPluginSystemReadOnly(ctx, cfg)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitSystem)
		}
		return fmt.Errorf("failed to initialize plugin system: %w", err)
	}

	// Get all plugins (discovered + disabled)
	plugins := result.Service.ListPlugins()
	disabledNames := result.Service.ListDisabledPlugins()

	// Build plugin info list
	infos := make([]ui.PluginInfo, 0, len(plugins)+len(disabledNames))

	// Add discovered plugins
	for _, p := range plugins {
		if p.Manifest == nil {
			continue
		}
		enabled := result.Service.IsPluginEnabled(p.Manifest.Name)
		infos = append(infos, ui.PluginInfo{
			Name:         p.Manifest.Name,
			Version:      p.Manifest.Version,
			Description:  p.Manifest.Description,
			Status:       string(p.Status),
			Enabled:      enabled,
			Capabilities: p.Manifest.Capabilities,
		})
	}

	// Add disabled plugins that weren't discovered (might be removed from disk)
	existingNames := make(map[string]struct{})
	for _, info := range infos {
		existingNames[info.Name] = struct{}{}
	}
	for _, name := range disabledNames {
		if _, exists := existingNames[name]; !exists {
			infos = append(infos, ui.PluginInfo{
				Name:    name,
				Status:  "not_found",
				Enabled: false,
			})
		}
	}

	return writer.WritePlugins(infos)
}

func runPluginEnable(cmd *cobra.Command, cfg *Config, name string) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Initialize plugin system
	result, err := initPluginSystemReadOnly(ctx, cfg)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitSystem)
		}
		return fmt.Errorf("failed to initialize plugin system: %w", err)
	}

	// Enable the plugin
	if err := result.Service.EnablePlugin(ctx, name); err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return fmt.Errorf("failed to enable plugin %q: %w", name, err)
	}

	// Save state
	if err := result.Service.SaveState(ctx); err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitSystem)
		}
		return fmt.Errorf("failed to save plugin state: %w", err)
	}

	if writer.IsJSONFormat() {
		return writer.WriteJSON(map[string]any{
			"plugin":  name,
			"enabled": true,
		})
	}

	formatter.Success(fmt.Sprintf("Plugin %q enabled", name))
	return nil
}

func runPluginDisable(cmd *cobra.Command, cfg *Config, name string) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)
	formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
		Verbose: cfg.Verbose,
		Quiet:   cfg.Quiet,
		NoColor: cfg.NoColor,
	})

	// Initialize plugin system
	result, err := initPluginSystemReadOnly(ctx, cfg)
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitSystem)
		}
		return fmt.Errorf("failed to initialize plugin system: %w", err)
	}

	// Disable the plugin
	if err := result.Service.DisablePlugin(ctx, name); err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitUser)
		}
		return fmt.Errorf("failed to disable plugin %q: %w", name, err)
	}

	// Save state
	if err := result.Service.SaveState(ctx); err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitSystem)
		}
		return fmt.Errorf("failed to save plugin state: %w", err)
	}

	if writer.IsJSONFormat() {
		return writer.WriteJSON(map[string]any{
			"plugin":  name,
			"enabled": false,
		})
	}

	formatter.Success(fmt.Sprintf("Plugin %q disabled", name))
	return nil
}

// initPluginSystemReadOnly initializes the plugin system without starting plugins.
// Used by CLI commands that only need to query plugin state.
func initPluginSystemReadOnly(ctx context.Context, cfg *Config) (*PluginSystemResult, error) {
	// Get plugin paths
	pluginPaths := getPluginSearchPaths(cfg)

	// Find the first existing plugin directory
	pluginsDir := findFirstExistingDir(pluginPaths)

	// Create state store for plugin enable/disable persistence
	stateStorePath := filepath.Join(cfg.StoragePath, "plugins")
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(stateStorePath)

	// Load persisted plugin states (non-fatal: continue with defaults)
	//nolint:errcheck,gosec // Non-fatal error: continue with default state
	stateStore.Load(ctx)

	// If no plugins directory exists, return a stub service
	if pluginsDir == "" {
		service := application.NewPluginService(nil, stateStore, nil)
		return &PluginSystemResult{
			Service: service,
			Cleanup: func() {},
		}, nil
	}

	// Initialize plugin infrastructure (discovery only, no startup)
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)
	manager := infrastructurePlugin.NewRPCPluginManager(loader)
	manager.SetPluginsDir(pluginsDir)

	// Discover plugins without loading/initializing them (non-fatal: we can still show state store info)
	//nolint:errcheck,gosec // Non-fatal error: can still show state store info
	manager.Discover(ctx)

	// Create the plugin service
	service := application.NewPluginService(manager, stateStore, nil)

	return &PluginSystemResult{
		Service: service,
		Cleanup: func() {},
	}, nil
}
