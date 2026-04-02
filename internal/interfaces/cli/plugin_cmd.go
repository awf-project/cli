package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	infrastructurePlugin "github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	registry "github.com/awf-project/cli/pkg/registry"
	"github.com/spf13/cobra"
)

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
	cmd.AddCommand(newPluginInstallCommand(cfg))
	cmd.AddCommand(newPluginUpdateCommand(cfg))
	cmd.AddCommand(newPluginRemoveCommand(cfg))
	cmd.AddCommand(newPluginSearchCommand(cfg))

	return cmd
}

type pluginListFlags struct {
	operations bool
	details    bool
	stepTypes  bool
	validators bool
}

func newPluginListCommand(cfg *Config) *cobra.Command {
	var flags pluginListFlags

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all available plugins",
		Long:    "Display all discovered plugins with their status and capabilities.",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginList(cmd, cfg, flags)
		},
	}

	cmd.Flags().BoolVar(&flags.operations, "operations", false, "List operations provided by each plugin")
	cmd.Flags().BoolVar(&flags.details, "details", false, "List all capabilities (operations, step types, validators)")
	cmd.Flags().BoolVar(&flags.stepTypes, "step-types", false, "List step types provided by each plugin")
	cmd.Flags().BoolVar(&flags.validators, "validators", false, "List validator plugins")

	return cmd
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

func runPluginList(cmd *cobra.Command, cfg *Config, flags pluginListFlags) error {
	// Validate mutual exclusivity
	setCount := 0
	for _, set := range []bool{flags.operations, flags.details, flags.stepTypes, flags.validators} {
		if set {
			setCount++
		}
	}
	if setCount > 1 {
		return fmt.Errorf("flags --operations, --details, --step-types, and --validators are mutually exclusive")
	}

	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)

	// --validators doesn't need gRPC; others with detail flags do
	needsGRPC := flags.operations || flags.details || flags.stepTypes

	var result *PluginSystemResult
	var err error
	if needsGRPC {
		result, err = initPluginSystem(ctx, cfg, nil)
		if result != nil {
			defer result.Cleanup()
		}
	} else {
		result, err = initPluginSystemReadOnly(ctx, cfg)
	}
	if err != nil {
		if writer.IsJSONFormat() {
			return writer.WriteError(err, ExitSystem)
		}
		return fmt.Errorf("failed to initialize plugin system: %w", err)
	}

	plugins := result.Service.ListPlugins()
	disabledNames := result.Service.ListDisabledPlugins()

	infos := make([]ui.PluginInfo, 0, len(plugins)+len(disabledNames))

	for _, p := range plugins {
		if p.Manifest == nil {
			continue
		}
		enabled := result.Service.IsPluginEnabled(p.Manifest.Name)
		var sourceStr string
		dirName := filepath.Base(p.Path)
		if sd := result.StateStore.GetSourceData(p.Manifest.Name); sd != nil {
			if repo, ok := sd["repository"].(string); ok {
				sourceStr = repo
			}
		} else if sd := result.StateStore.GetSourceData(dirName); sd != nil {
			if repo, ok := sd["repository"].(string); ok {
				sourceStr = repo
			}
		}

		infos = append(infos, ui.PluginInfo{
			Name:         p.Manifest.Name,
			Type:         string(p.Type),
			Version:      p.Manifest.Version,
			Description:  p.Manifest.Description,
			Status:       string(p.Status),
			Enabled:      enabled,
			Capabilities: p.Manifest.Capabilities,
			Operations:   p.Operations,
			StepTypes:    p.StepTypes,
			Source:       sourceStr,
		})
	}

	// Add disabled plugins that weren't discovered
	existingNames := make(map[string]struct{})
	for i := range infos {
		existingNames[infos[i].Name] = struct{}{}
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

	switch {
	case flags.operations:
		return writer.WriteOperations(buildNamedEntries(infos, func(p ui.PluginInfo) []string { return p.Operations }))
	case flags.stepTypes:
		return writer.WriteStepTypes(buildNamedEntries(infos, func(p ui.PluginInfo) []string { return p.StepTypes }))
	case flags.validators:
		return writer.WriteValidators(buildValidatorEntries(infos))
	case flags.details:
		return writer.WriteCapabilities(buildCapabilityEntries(infos))
	default:
		return writer.WritePlugins(infos)
	}
}

func buildNamedEntries(infos []ui.PluginInfo, getNames func(ui.PluginInfo) []string) []ui.OperationEntry {
	var entries []ui.OperationEntry
	for i := range infos {
		for _, name := range getNames(infos[i]) {
			entries = append(entries, ui.OperationEntry{Name: name, Plugin: infos[i].Name})
		}
	}
	return entries
}

func buildValidatorEntries(infos []ui.PluginInfo) []ui.ValidatorEntry {
	var entries []ui.ValidatorEntry
	for i := range infos {
		for _, cap := range infos[i].Capabilities {
			if cap == "validators" {
				entries = append(entries, ui.ValidatorEntry{
					Name:        infos[i].Name,
					Description: infos[i].Description,
				})
				break
			}
		}
	}
	return entries
}

func buildCapabilityEntries(infos []ui.PluginInfo) []ui.CapabilityEntry {
	var entries []ui.CapabilityEntry
	for i := range infos {
		for _, opName := range infos[i].Operations {
			entries = append(entries, ui.CapabilityEntry{Type: "operation", Name: opName, Plugin: infos[i].Name})
		}
		for _, stName := range infos[i].StepTypes {
			entries = append(entries, ui.CapabilityEntry{Type: "step_type", Name: stName, Plugin: infos[i].Name})
		}
		for _, cap := range infos[i].Capabilities {
			if cap == "validators" {
				entries = append(entries, ui.CapabilityEntry{Type: "validator", Name: infos[i].Name, Plugin: infos[i].Name})
				break
			}
		}
	}
	return entries
}

func runPluginEnable(cmd *cobra.Command, cfg *Config, name string) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
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
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
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

	// Find all existing plugin directories
	pluginsDirs := findExistingDirs(pluginPaths)

	// Create state store for plugin enable/disable persistence
	stateStorePath := filepath.Join(cfg.StoragePath, "plugins")
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(stateStorePath)

	// Load persisted plugin states (non-fatal: continue with defaults)
	//nolint:errcheck,gosec // Non-fatal error: continue with default state
	stateStore.Load(ctx)

	// If no plugins directory exists, return a stub service
	if len(pluginsDirs) == 0 {
		service := application.NewPluginService(nil, stateStore, nil)
		registerBuiltins(service, Version)
		return &PluginSystemResult{
			Service:    service,
			StateStore: stateStore,
			Cleanup:    func() {},
		}, nil
	}

	// Initialize plugin infrastructure (discovery only, no startup)
	parser := infrastructurePlugin.NewManifestParser()
	loader := infrastructurePlugin.NewFileSystemLoader(parser)
	manager := infrastructurePlugin.NewRPCPluginManager(loader)
	manager.SetPluginsDirs(pluginsDirs)

	// Discover plugins without loading/initializing them (non-fatal: we can still show state store info)
	//nolint:errcheck,gosec // Non-fatal error: can still show state store info
	manager.Discover(ctx)

	// Create the plugin service
	service := application.NewPluginService(manager, stateStore, nil)
	registerBuiltins(service, Version)

	return &PluginSystemResult{
		Service:    service,
		StateStore: stateStore,
		Cleanup:    func() {},
	}, nil
}

type installOptions struct {
	version    string
	preRelease bool
	force      bool
}

func newPluginInstallCommand(cfg *Config) *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install <owner/repo>",
		Short: "Install a plugin from GitHub releases",
		Long: `Install a plugin from a GitHub repository using the owner/repo format.

The plugin binary is downloaded, checksum-verified, extracted, and installed
atomically. The plugin is enabled automatically after installation.

Examples:
  awf plugin install myorg/awf-plugin-jira
  awf plugin install myorg/awf-plugin-jira --version ">=1.0.0 <2.0.0"
  awf plugin install myorg/awf-plugin-jira --pre-release
  awf plugin install myorg/awf-plugin-jira --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginInstall(cmd, cfg, args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.version, "version", "", "version constraint (e.g. \">=1.0.0 <2.0.0\")")
	cmd.Flags().BoolVar(&opts.preRelease, "pre-release", false, "include pre-release versions")
	cmd.Flags().BoolVar(&opts.force, "force", false, "overwrite existing installation")

	return cmd
}

func runPluginInstall(cmd *cobra.Command, cfg *Config, source string, opts installOptions) error {
	if err := registry.ValidateOwnerRepo(source); err != nil {
		return err
	}
	owner, repo, _ := strings.Cut(source, "/")
	pluginName := extractPluginName(repo)

	pluginPaths := getPluginSearchPaths(cfg)
	pluginsDir := findFirstExistingDir(pluginPaths)
	if pluginsDir == "" {
		pluginsDir = pluginPaths[0]
		if err := os.MkdirAll(pluginsDir, 0o750); err != nil { //nolint:gosec // G301: 0o750 is intentional for plugin directories
			return fmt.Errorf("failed to create plugins directory: %w", err)
		}
	}

	pluginDir := filepath.Join(pluginsDir, pluginName)

	if !opts.force {
		if _, err := os.Stat(pluginDir); err == nil {
			errMsg := fmt.Sprintf("error: plugin %q is already installed; use --force to reinstall\n", pluginName)
			fmt.Fprint(cmd.ErrOrStderr(), errMsg)
			return fmt.Errorf("plugin already installed")
		}
	}

	ctx := context.Background()

	// Build a GitHub release client; redirect API calls to GITHUB_API_URL when set (testing).
	doer := registry.NewGitHubAPIDoer(os.Getenv("GITHUB_API_URL"), http.DefaultClient)
	githubClient := registry.NewGitHubReleaseClient(doer)

	ownerRepo := owner + "/" + repo
	releases, err := githubClient.ListReleases(ctx, ownerRepo)
	if err != nil {
		return fmt.Errorf("failed to fetch releases for %s: %w", ownerRepo, err)
	}
	if len(releases) == 0 {
		return fmt.Errorf("no releases found for %s", ownerRepo)
	}

	release, err := selectRelease(releases, opts.version, opts.preRelease)
	if err != nil {
		return fmt.Errorf("failed to resolve version for %s: %w", ownerRepo, err)
	}

	asset, err := registry.FindPlatformAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("failed to find platform asset: %w", err)
	}

	checksumURL := findChecksumURL(release.Assets)
	if checksumURL == "" {
		return fmt.Errorf("no checksum file found in release %s", release.TagName)
	}

	checksumData, err := registry.Download(ctx, http.DefaultClient, checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksum file: %w", err)
	}

	checksum := registry.ExtractChecksumForAsset(string(checksumData), asset.Name)
	if checksum == "" {
		return fmt.Errorf("checksum for %q not found in checksum file", asset.Name)
	}

	installer := infrastructurePlugin.NewPluginInstaller(doer)
	if installErr := installer.Install(ctx, asset.DownloadURL, checksum, pluginDir, opts.force); installErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", installErr)
		return installErr
	}

	// Persist source metadata so awf plugin update can determine the origin repo.
	stateStorePath := filepath.Join(cfg.StoragePath, "plugins")
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(stateStorePath)
	//nolint:errcheck,gosec // Non-fatal: load existing state before merging source data
	stateStore.Load(ctx)
	sourceData, err := infrastructurePlugin.SourceDataFromPluginSource(infrastructurePlugin.PluginSource{
		Repository:  ownerRepo,
		Version:     release.TagName,
		InstalledAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("failed to build source metadata: %w", err)
	}
	if err := stateStore.SetSourceData(ctx, pluginName, sourceData); err != nil {
		return fmt.Errorf("failed to persist source metadata: %w", err)
	}
	if err := stateStore.Save(ctx); err != nil {
		return fmt.Errorf("failed to save plugin state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Plugin %q installed successfully (version %s)\n", pluginName, release.TagName)
	return nil
}

// selectRelease picks the best matching release from a list.
// When versionConstraint is empty, returns the first non-prerelease release (latest stable).
// When includePrerelease is true, also considers prerelease releases.
//
// versionConstraint may be a bare version ("1.0.0"), a v-prefixed version ("v1.0.0"),
// or a semver range expression (">=1.0.0 <2.0.0").
func selectRelease(releases []registry.Release, versionConstraint string, includePrerelease bool) (registry.Release, error) {
	var constraints registry.Constraints
	if versionConstraint != "" {
		// Normalize a bare v-prefixed version like "v1.0.0" → "1.0.0" so ParseConstraints
		// can handle it. Range expressions such as ">=1.0.0" are left as-is.
		normalized := registry.NormalizeTag(versionConstraint)
		var err error
		constraints, err = registry.ParseConstraints(normalized)
		if err != nil {
			return registry.Release{}, fmt.Errorf("invalid version constraint: %w", err)
		}
	}

	for _, r := range releases {
		if r.Prerelease && !includePrerelease {
			continue
		}
		if versionConstraint == "" {
			return r, nil
		}
		versionStr := registry.NormalizeTag(r.TagName)
		v, err := registry.ParseVersion(versionStr)
		if err != nil {
			continue
		}
		if constraints.Check(v) {
			return r, nil
		}
	}
	return registry.Release{}, fmt.Errorf("no release matches constraint %q (includePrerelease=%v)", versionConstraint, includePrerelease)
}

// findChecksumURL locates the checksum file URL among release assets.
// Looks for assets named *checksums.txt or SHA256SUMS.
func findChecksumURL(assets []registry.Asset) string {
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.HasSuffix(name, "checksums.txt") || name == "sha256sums" || strings.HasSuffix(name, "sha256sums.txt") {
			return a.DownloadURL
		}
	}
	return ""
}

func extractPluginName(repo string) string {
	if strings.HasPrefix(repo, "awf-plugin-") {
		return strings.TrimPrefix(repo, "awf-plugin-")
	}
	return repo
}

type updateOptions struct {
	all bool
}

func newPluginUpdateCommand(cfg *Config) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update [plugin-name]",
		Short: "Update an installed plugin to the latest version",
		Long: `Update an installed plugin to the latest compatible version.

Use --all to update all externally installed plugins at once.

Examples:
  awf plugin update jira
  awf plugin update jira --pre-release
  awf plugin update --all`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !opts.all {
				return fmt.Errorf("requires a plugin name or --all flag\n\n%s", cmd.UsageString())
			}
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runPluginUpdate(cmd, cfg, name, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.all, "all", false, "update all externally installed plugins")

	return cmd
}

func runPluginUpdate(cmd *cobra.Command, cfg *Config, name string, opts updateOptions) error {
	ctx := context.Background()

	if opts.all {
		result, err := initPluginSystemReadOnly(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize plugin system: %w", err)
		}

		plugins := result.Service.ListPlugins()
		for _, p := range plugins {
			if p.Type == pluginmodel.PluginTypeBuiltin {
				continue
			}
			if p.Manifest != nil {
				if err := updatePlugin(cmd, cfg, p.Manifest.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return updatePlugin(cmd, cfg, name)
}

func updatePlugin(cmd *cobra.Command, cfg *Config, name string) error {
	ctx := context.Background()

	pluginPaths := getPluginSearchPaths(cfg)
	pluginDir := findPluginDir(pluginPaths, name)
	if pluginDir == "" {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	// Load plugin state to retrieve installation source metadata.
	stateStorePath := filepath.Join(cfg.StoragePath, "plugins")
	stateStore := infrastructurePlugin.NewJSONPluginStateStore(stateStorePath)
	//nolint:errcheck,gosec // Non-fatal: continue with empty state if load fails
	stateStore.Load(ctx)

	stateName := resolvePluginStateName(stateStore.GetSourceData, name)
	sourceData := stateStore.GetSourceData(stateName)
	if sourceData == nil {
		return fmt.Errorf("plugin %q was not installed from a remote source and cannot be updated automatically", name)
	}

	source, err := infrastructurePlugin.PluginSourceFromSourceData(sourceData)
	if err != nil {
		return fmt.Errorf("failed to read source metadata for plugin %q: %w", name, err)
	}

	// Fetch latest release from GitHub.
	doer := registry.NewGitHubAPIDoer(os.Getenv("GITHUB_API_URL"), http.DefaultClient)
	githubClient := registry.NewGitHubReleaseClient(doer)

	releases, err := githubClient.ListReleases(ctx, source.Repository)
	if err != nil {
		return fmt.Errorf("failed to fetch releases for %s: %w", source.Repository, err)
	}
	if len(releases) == 0 {
		return fmt.Errorf("no releases found for %s", source.Repository)
	}

	// Select latest stable release (no version constraint, no pre-release).
	release, err := selectRelease(releases, "", false)
	if err != nil {
		return fmt.Errorf("failed to resolve latest version for %s: %w", source.Repository, err)
	}

	// Compare current installed version against the latest release.
	currentNorm := registry.NormalizeTag(source.Version)
	latestNorm := registry.NormalizeTag(release.TagName)
	if currentNorm == latestNorm {
		fmt.Fprintf(cmd.OutOrStdout(), "Plugin %q is already up to date (version %s)\n", name, release.TagName)
		return nil
	}

	// Locate the platform-specific asset and checksum file.
	asset, err := registry.FindPlatformAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("failed to find platform asset: %w", err)
	}

	checksumURL := findChecksumURL(release.Assets)
	if checksumURL == "" {
		return fmt.Errorf("no checksum file found in release %s", release.TagName)
	}

	checksumData, err := registry.Download(ctx, http.DefaultClient, checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksum file: %w", err)
	}

	checksum := registry.ExtractChecksumForAsset(string(checksumData), asset.Name)
	if checksum == "" {
		return fmt.Errorf("checksum for %q not found in checksum file", asset.Name)
	}

	// Shut down gRPC connection before replacing the binary (prevents "text file busy" on Linux).
	// Best-effort: the plugin might not be running, so we only warn on failure.
	result, resultErr := initPluginSystemReadOnly(ctx, cfg)
	if resultErr == nil {
		if shutdownErr := result.Service.ShutdownPlugin(ctx, name); shutdownErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to shutdown plugin %q before update: %v\n", name, shutdownErr)
		}
	}

	// Install the new version, replacing the existing directory.
	installer := infrastructurePlugin.NewPluginInstaller(doer)
	if installErr := installer.Install(ctx, asset.DownloadURL, checksum, pluginDir, true); installErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", installErr)
		return installErr
	}

	// Persist updated source metadata with the new version and timestamp.
	previousVersion := source.Version
	source.Version = release.TagName
	source.UpdatedAt = time.Now().UTC()
	updatedSourceData, err := infrastructurePlugin.SourceDataFromPluginSource(source)
	if err != nil {
		return fmt.Errorf("failed to build updated source metadata: %w", err)
	}
	if err := stateStore.SetSourceData(ctx, stateName, updatedSourceData); err != nil {
		return fmt.Errorf("failed to persist updated source metadata: %w", err)
	}
	if err := stateStore.Save(ctx); err != nil {
		return fmt.Errorf("failed to save plugin state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated plugin %q from %s to %s\n", name, previousVersion, release.TagName)
	return nil
}

type removeOptions struct {
	keepData bool
}

func newPluginRemoveCommand(cfg *Config) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:   "remove <plugin-name>",
		Short: "Remove an installed plugin",
		Long: `Remove an installed plugin by name. The plugin binary and manifest
are deleted from the plugins directory. Plugin state is also removed.

Use --keep-data to preserve plugin configuration and state.

Examples:
  awf plugin remove jira
  awf plugin remove jira --keep-data`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginRemove(cmd, cfg, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.keepData, "keep-data", false, "preserve plugin configuration and state")

	return cmd
}

func runPluginRemove(cmd *cobra.Command, cfg *Config, name string, opts removeOptions) error {
	ctx := context.Background()

	result, err := initPluginSystemReadOnly(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize plugin system: %w", err)
	}

	// Resolve the plugin name: try exact name, then short name (without "awf-plugin-" prefix).
	// This handles the mismatch between manifest names shown in "plugin list" and internal names.
	resolvedName := name
	if _, found := result.Service.GetPlugin(name); !found {
		if short := extractPluginName(name); short != name {
			if _, found := result.Service.GetPlugin(short); found {
				resolvedName = short
			}
		}
	}

	// Check if plugin is built-in by checking in discovered plugins
	info, found := result.Service.GetPlugin(resolvedName)
	if found && info.Type == pluginmodel.PluginTypeBuiltin {
		errMsg := fmt.Sprintf("error: plugin %q is a built-in provider and cannot be removed; use 'awf plugin disable %s' to disable it\n", resolvedName, resolvedName)
		fmt.Fprint(cmd.ErrOrStderr(), errMsg)
		return fmt.Errorf("cannot remove built-in plugin")
	}

	pluginPaths := getPluginSearchPaths(cfg)
	pluginDir := findPluginDir(pluginPaths, name)
	if pluginDir == "" {
		return fmt.Errorf("plugin %q is not installed", name)
	}

	// Shut down gRPC connection before removing the binary (prevents "text file busy" on Linux).
	// Best-effort: the plugin might not be running, so we only warn on failure.
	if shutdownErr := result.Service.ShutdownPlugin(ctx, resolvedName); shutdownErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to shutdown plugin %q before removal: %v\n", resolvedName, shutdownErr)
	}

	// Always remove the plugin directory
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	// Remove state entry entirely (not just disable) so the plugin does not appear in the list.
	if !opts.keepData {
		if err := result.StateStore.RemoveState(ctx, resolvedName); err != nil {
			return fmt.Errorf("failed to remove plugin state: %w", err)
		}

		if err := result.StateStore.Save(ctx); err != nil {
			return fmt.Errorf("failed to save plugin state: %w", err)
		}
	}

	return nil
}

func newPluginSearchCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "search [query]",
		Short: "Search for available plugins on GitHub",
		Long: `Search GitHub for AWF plugins by topic or keyword.

Results are repositories tagged with the "awf-plugin" topic.
Use --output=json to get machine-readable output.

Examples:
  awf plugin search
  awf plugin search jira
  awf plugin search --output=json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			return runPluginSearch(cmd, cfg, query)
		},
	}
}

func runPluginSearch(cmd *cobra.Command, cfg *Config, query string) error {
	ctx := context.Background()

	baseURL := os.Getenv("GITHUB_API_URL")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	searchQuery := "topic:awf-plugin"
	if query != "" {
		searchQuery += "+" + url.QueryEscape(query)
	}

	apiURL := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc", baseURL, searchQuery)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody) //nolint:gosec // G107: URL from validated env var or hardcoded base
	if err != nil {
		return fmt.Errorf("failed to search plugins: %w", err)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: URL constructed from safe base + escaped query
	if err != nil {
		return fmt.Errorf("failed to search plugins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		if resp.Header.Get("X-Ratelimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits")
		}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub search API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return fmt.Errorf("failed to read search results: %w", err)
	}

	var searchResult struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &searchResult); err != nil {
		return fmt.Errorf("failed to parse search results: %w", err)
	}

	if cfg.OutputFormat == ui.FormatJSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(searchResult.Items)
	}

	if len(searchResult.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No plugins found")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %-6s  %s\n", "NAME", "STARS", "DESCRIPTION")
	for _, item := range searchResult.Items {
		desc := item.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %-6d  %s\n", item.FullName, item.Stars, desc)
	}

	return nil
}
