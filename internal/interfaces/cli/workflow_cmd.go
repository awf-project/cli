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
	"strings"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/awf-project/cli/pkg/registry"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newWorkflowCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workflow",
		Short:   "Manage AWF workflow packs",
		Aliases: []string{"wf"},
	}

	cmd.AddCommand(newWorkflowInstallCommand(cfg))
	cmd.AddCommand(newWorkflowRemoveCommand(cfg))
	cmd.AddCommand(newWorkflowListCommand(cfg))
	cmd.AddCommand(newWorkflowInfoCommand(cfg))
	cmd.AddCommand(newWorkflowUpdateCommand(cfg))
	cmd.AddCommand(newWorkflowSearchCommand(cfg))

	return cmd
}

func newWorkflowListCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed workflow packs",
		Long: `List all installed workflow packs with their version, source, and available workflows.

Includes a (local) pseudo-entry for workflows in .awf/workflows/.

Examples:
  awf workflow list`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowList(cmd, cfg)
		},
	}
}

func runWorkflowList(cmd *cobra.Command, cfg *Config) error {
	ctx := context.Background()
	writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor, cfg.NoHints)
	loader := workflowpkg.NewPackLoader()

	// Discover packs from all known directories; higher-priority dirs win (local > config > global)
	packMap := make(map[string]workflowpkg.PackInfo)
	dirs := workflowPackSearchDirs()
	for i := len(dirs) - 1; i >= 0; i-- {
		packs, _ := loader.DiscoverPacks(ctx, dirs[i]) //nolint:errcheck // non-existent dir is expected
		for _, p := range packs {
			packMap[p.Name] = p
		}
	}

	var packs []ui.WorkflowPackInfo

	for _, p := range packMap {
		workflowNames := make([]string, 0, len(p.Workflows))
		for wf := range p.Workflows {
			workflowNames = append(workflowNames, wf)
		}

		state, _ := loader.LoadPackState(findPackDir(p.Name)) //nolint:errcheck // state is optional
		var source string
		if state != nil {
			if ps, err := workflowpkg.PackSourceFromSourceData(state.SourceData); err == nil {
				source = ps.Repository
			}
		}

		enabled := true
		if state != nil {
			enabled = state.Enabled
		}

		packs = append(packs, ui.WorkflowPackInfo{
			Name:      p.Name,
			Version:   p.Version,
			Source:    source,
			Enabled:   enabled,
			Workflows: workflowNames,
		})
	}

	// Add pseudo-entries for loose workflows (local and global directories)
	for _, sp := range BuildWorkflowPaths() {
		var workflows []string
		if entries, readErr := os.ReadDir(sp.Path); readErr == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
					workflows = append(workflows, strings.TrimSuffix(e.Name(), ".yaml"))
				}
			}
		}
		if len(workflows) > 0 {
			packs = append(packs, ui.WorkflowPackInfo{
				Name:      "(" + sp.Source.String() + ")",
				Enabled:   true,
				Workflows: workflows,
			})
		}
	}

	return writer.WriteWorkflowPacks(packs)
}

func newWorkflowInfoCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "info <pack-name>",
		Short: "Display detailed information about an installed workflow pack",
		Long: `Show manifest fields, workflow descriptions, plugin install status, and embedded README
for an installed workflow pack.

Searches local then global workflow-packs directories.

Examples:
  awf workflow info speckit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowInfo(cmd, cfg, args[0])
		},
	}
}

// readManifestData reads and size-caps manifest.yaml from a pack directory.
func readManifestData(packDir string) ([]byte, error) {
	manifestPath := filepath.Join(packDir, "manifest.yaml")
	f, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, workflowpkg.MaxManifestSize))
}

// loadWorkflowDescription reads the description field from a workflow YAML file inside a pack.
// Returns empty string on any error (missing file, invalid YAML, oversized file).
func loadWorkflowDescription(packDir, workflowName string) string {
	f, err := os.Open(filepath.Join(packDir, "workflows", workflowName+".yaml"))
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return ""
	}
	var wf struct {
		Description string `yaml:"description"`
	}
	if yaml.Unmarshal(data, &wf) != nil {
		return ""
	}
	return wf.Description
}

// workflowPackSearchDirs returns pack directories in priority order (local first).
func workflowPackSearchDirs() []string {
	dirs := []string{
		filepath.Join(".awf", "workflow-packs"),                   // local project
		filepath.Join(xdg.ConfigHome(), ".awf", "workflow-packs"), // $XDG_CONFIG_HOME/.awf/
		filepath.Join(xdg.AWFConfigDir(), "workflow-packs"),       // $XDG_CONFIG_HOME/awf/
		xdg.AWFWorkflowPacksDir(),                                 // $XDG_DATA_HOME/awf/
	}
	// Legacy: ~/.awf/workflow-packs
	if homeDir, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(homeDir, ".awf", "workflow-packs"))
	}
	return dirs
}

// findPackDir locates an installed pack by name across all search directories.
// Tries the exact name first, then the short name (without awf-workflow- prefix).
func findPackDir(packName string) string {
	shortName := strings.TrimPrefix(packName, "awf-workflow-")
	for _, dir := range workflowPackSearchDirs() {
		for _, candidate := range []string{packName, shortName} {
			potentialPath := filepath.Join(dir, candidate)
			if info, err := os.Stat(potentialPath); err == nil && info.IsDir() {
				return potentialPath
			}
		}
	}
	return ""
}

func runWorkflowInfo(cmd *cobra.Command, _ *Config, packName string) error {
	packDir := findPackDir(packName)

	if packDir == "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: pack %q not found\n", packName)
		return fmt.Errorf("pack %q not found", packName)
	}

	// Load the manifest (size-capped to prevent OOM)
	manifestData, err := readManifestData(packDir)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := workflowpkg.ParseManifest(manifestData)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// Display pack information
	fmt.Fprintf(cmd.OutOrStdout(), "Pack: %s\n", manifest.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", manifest.Version)
	if manifest.Description != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", manifest.Description)
	}
	if manifest.Author != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Author: %s\n", manifest.Author)
	}
	if manifest.License != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "License: %s\n", manifest.License)
	}

	// Load and display source metadata from state.json
	loader := workflowpkg.NewPackLoader()
	if state, stateErr := loader.LoadPackState(packDir); stateErr == nil {
		if source, srcErr := workflowpkg.PackSourceFromSourceData(state.SourceData); srcErr == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Source: %s\n", source.Repository)
		}
	}

	if len(manifest.Workflows) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Workflows:\n")
		for _, wf := range manifest.Workflows {
			desc := loadWorkflowDescription(packDir, wf)
			if desc != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s: %s\n", wf, desc)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", wf)
			}
		}
	}

	// Load and display README if it exists (size-capped to prevent OOM from malicious packs)
	readmePath := filepath.Join(packDir, "README.md")
	readmeFile, readErr := os.Open(readmePath)
	if readErr == nil {
		defer readmeFile.Close()
		readmeData, readmeErr := io.ReadAll(io.LimitReader(readmeFile, 1<<20))
		if readmeErr == nil && len(readmeData) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nREADME:\n%s\n", string(readmeData))
		}
	}

	// Emit plugin dependency warnings (non-blocking)
	emitPluginWarnings(cmd, packDir, nil)

	return nil
}

type workflowInstallFlags struct {
	version string
	global  bool
	force   bool
}

func newWorkflowInstallCommand(cfg *Config) *cobra.Command {
	var flags workflowInstallFlags

	cmd := &cobra.Command{
		Use:   "install <owner/repo[@version]>",
		Short: "Install a workflow pack from GitHub releases",
		Long: `Install a workflow pack from a GitHub repository using the owner/repo format.

The pack archive is downloaded, checksum-verified, extracted, and installed
atomically. Pass @version to pin an exact version.

Examples:
  awf workflow install myorg/awf-workflow-speckit
  awf workflow install myorg/awf-workflow-speckit@1.2.0
  awf workflow install myorg/awf-workflow-speckit --global
  awf workflow install myorg/awf-workflow-speckit --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowInstall(cmd, cfg, args[0], flags)
		},
	}

	cmd.Flags().StringVar(&flags.version, "version", "", "version constraint (e.g. \">=1.0.0 <2.0.0\")")
	cmd.Flags().BoolVar(&flags.global, "global", false, "install to global XDG data directory")
	cmd.Flags().BoolVar(&flags.force, "force", false, "overwrite existing installation")

	return cmd
}

// parseOwnerRepoAndVersion parses owner/repo[@ version] format.
func parseOwnerRepoAndVersion(source, versionFlag string) (ownerRepo, version string) {
	ownerRepo = source
	version = versionFlag

	if at := strings.LastIndex(source, "@"); at >= 0 {
		ownerRepo = source[:at]
		version = source[at+1:]
	}

	return ownerRepo, version
}

// extractPackName extracts pack name from repository name (removes awf-workflow- prefix).
func extractPackName(ownerRepo string) string {
	packName := ownerRepo
	if idx := strings.LastIndex(ownerRepo, "/"); idx >= 0 {
		packName = ownerRepo[idx+1:]
	}
	return strings.TrimPrefix(packName, "awf-workflow-")
}

// effectiveCLIVersion returns Version, substituting a valid semver for "dev" builds
// so pack version constraint checks work correctly outside production releases.
func effectiveCLIVersion() string {
	if strings.HasPrefix(Version, "dev") {
		return "0.5.0"
	}
	return Version
}

// findPackAsset finds the .tar.gz archive in a release's assets.
// Workflow packs are platform-independent (YAML/scripts only), so there is a single archive per release.
func findPackAsset(assets []registry.Asset) (registry.Asset, error) {
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ".tar.gz") && a.Name != "checksums.txt" {
			return a, nil
		}
	}
	return registry.Asset{}, fmt.Errorf("no .tar.gz archive found in release assets")
}

// findChecksumInRelease finds the checksum for the given asset from a release's checksum file.
func findChecksumInRelease(ctx context.Context, assets []registry.Asset, assetName string) (string, error) {
	var checksumAsset *registry.Asset
	for i, a := range assets {
		if a.Name == "checksums.txt" {
			checksumAsset = &assets[i]
			break
		}
	}

	if checksumAsset == nil {
		return "", fmt.Errorf("checksums.txt not found in release assets")
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	checksumData, err := registry.Download(ctx, httpClient, checksumAsset.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}

	checksum := registry.ExtractChecksumForAsset(string(checksumData), assetName)
	if checksum == "" {
		return "", fmt.Errorf("checksum for %s not found in checksums.txt", assetName)
	}
	return checksum, nil
}

func runWorkflowInstall(cmd *cobra.Command, _ *Config, source string, flags workflowInstallFlags) error {
	// Parse owner/repo and optional @version
	ownerRepo, versionConstraint := parseOwnerRepoAndVersion(source, flags.version)

	if err := registry.ValidateOwnerRepo(ownerRepo); err != nil {
		return err
	}

	if !flags.global {
		if _, err := os.Stat(".awf"); os.IsNotExist(err) {
			return fmt.Errorf("not in an awf project (run awf init first)")
		}
	}

	githubClient := newWorkflowGitHubClient()
	ctx := context.Background()

	fmt.Fprintf(cmd.OutOrStdout(), "Fetching releases for %s...\n", ownerRepo)
	resolvedVersion, err := githubClient.ResolveVersion(ctx, ownerRepo, versionConstraint, false)
	if err != nil {
		return fmt.Errorf("resolve version: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Resolved version: %s\n", resolvedVersion.String())

	// List releases to find the matching one
	releases, err := githubClient.ListReleases(ctx, ownerRepo)
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	var matchingRelease *registry.Release
	for _, rel := range releases {
		tagVersion := registry.NormalizeTag(rel.TagName)
		v, parseErr := registry.ParseVersion(tagVersion)
		if parseErr != nil {
			continue
		}
		if v.Compare(resolvedVersion) == 0 {
			matchingRelease = &rel
			break
		}
	}

	if matchingRelease == nil {
		return fmt.Errorf("release for version %s not found", resolvedVersion.String())
	}

	// Find the pack archive asset (workflow packs are platform-independent — single .tar.gz)
	asset, err := findPackAsset(matchingRelease.Assets)
	if err != nil {
		return err
	}

	// Get checksum for the asset
	checksum, err := findChecksumInRelease(ctx, matchingRelease.Assets, asset.Name)
	if err != nil {
		return err
	}

	// Determine target pack name and directory
	packName := extractPackName(ownerRepo)
	var targetDir string
	if flags.global {
		targetDir = filepath.Join(xdg.AWFWorkflowPacksDir(), packName)
	} else {
		targetDir = filepath.Join(xdg.LocalWorkflowPacksDir(), packName)
	}

	installer := workflowpkg.NewPackInstaller(effectiveCLIVersion())

	// Install the pack
	packSource := workflowpkg.PackSource{
		Repository: ownerRepo,
		Version:    resolvedVersion.String(),
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", asset.Name)
	if err := installer.Install(ctx, asset.DownloadURL, checksum, targetDir, flags.force, packSource); err != nil {
		return fmt.Errorf("install workflow pack: %w", err)
	}

	// Emit plugin dependency warnings (non-blocking)
	emitPluginWarnings(cmd, targetDir, nil)

	fmt.Fprintf(cmd.OutOrStdout(), "Installed workflow pack %q v%s to %s\n", packName, resolvedVersion.String(), targetDir)
	return nil
}

// emitPluginWarnings reads the installed manifest and warns about missing plugin dependencies.
// When pluginSvc is provided, only warns about plugins not already installed.
// Writes warnings to the command's error stream for proper capture in tests.
func emitPluginWarnings(cmd *cobra.Command, packDir string, pluginSvc *application.PluginService) {
	data, err := readManifestData(packDir)
	if err != nil {
		return
	}

	manifest, err := workflowpkg.ParseManifest(data)
	if err != nil || len(manifest.Plugins) == 0 {
		return
	}

	for pluginName, versionConstraint := range manifest.Plugins {
		if pluginSvc != nil {
			_, exists := pluginSvc.GetPlugin(pluginName)
			if exists {
				continue
			}
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: pack requires plugin %q (%s) — install with: awf plugin install <owner>/%s\n", pluginName, versionConstraint, pluginName)
	}
}

type workflowUpdateFlags struct {
	all bool
}

func newWorkflowUpdateCommand(cfg *Config) *cobra.Command {
	var flags workflowUpdateFlags

	cmd := &cobra.Command{
		Use:   "update [pack-name]",
		Short: "Update an installed workflow pack to the latest version",
		Long: `Update an installed workflow pack by fetching the latest release from GitHub.
Uses the source repository stored in state.json to find newer versions.
Pass --all to update all installed packs.

Examples:
  awf workflow update speckit
  awf workflow update --all`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.all && len(args) > 0 {
				return fmt.Errorf("cannot specify a pack name with --all")
			}
			if !flags.all && len(args) == 0 {
				return fmt.Errorf("pack name required (or use --all)")
			}
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			return runWorkflowUpdate(cmd, cfg, name, flags)
		},
	}

	cmd.Flags().BoolVar(&flags.all, "all", false, "update all installed workflow packs")

	return cmd
}

// newWorkflowGitHubClient creates a GitHub release client with GITHUB_API_URL support for testing.
func newWorkflowGitHubClient() *registry.GitHubReleaseClient {
	doer := registry.NewGitHubAPIDoer(os.Getenv("GITHUB_API_URL"), &http.Client{Timeout: 30 * time.Second})
	return registry.NewGitHubReleaseClient(doer)
}

// discoverAllPacks discovers packs from all search directories, deduplicating by name.
// Returns a map of pack name → pack directory path (higher-priority dirs win).
func discoverAllPacks(ctx context.Context, loader *workflowpkg.PackLoader) map[string]string {
	packMap := make(map[string]string)
	dirs := workflowPackSearchDirs()
	// Scan in reverse order so higher-priority dirs overwrite lower ones
	for i := len(dirs) - 1; i >= 0; i-- {
		packs, _ := loader.DiscoverPacks(ctx, dirs[i]) //nolint:errcheck // non-existent dir is expected
		for _, p := range packs {
			packMap[p.Name] = filepath.Join(dirs[i], p.Name)
		}
	}
	return packMap
}

func runWorkflowUpdate(cmd *cobra.Command, _ *Config, packName string, flags workflowUpdateFlags) error {
	githubClient := newWorkflowGitHubClient()
	ctx := context.Background()

	if flags.all {
		loader := workflowpkg.NewPackLoader()
		packMap := discoverAllPacks(ctx, loader)

		if len(packMap) == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "No installed workflow packs to update.\n")
			return nil
		}

		updatedCount := 0
		for packName, packDir := range packMap {
			if err := updateSinglePack(ctx, cmd, githubClient, packName, packDir); err == nil {
				updatedCount++
			}
		}

		if updatedCount > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %d pack(s).\n", updatedCount)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "All packs are at their latest versions.\n")
		}
		return nil
	}

	// Update single pack
	packDir := findPackDir(packName)
	if packDir == "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: workflow pack %q not found in local or global directories\n", packName)
		return fmt.Errorf("workflow pack %q not found in local or global directories", packName)
	}

	return updateSinglePack(ctx, cmd, githubClient, packName, packDir)
}

// updateSinglePack checks for a newer version and updates if available.
// Returns nil if updated or already at latest, error otherwise.
func updateSinglePack(ctx context.Context, cmd *cobra.Command, githubClient *registry.GitHubReleaseClient, packName, packDir string) error {
	// Load current state
	loader := workflowpkg.NewPackLoader()
	state, err := loader.LoadPackState(packDir)
	if err != nil {
		return fmt.Errorf("load pack state: %w", err)
	}

	// Extract source metadata
	source, err := workflowpkg.PackSourceFromSourceData(state.SourceData)
	if err != nil {
		return fmt.Errorf("parse pack source: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Checking %s for updates (current: %s)...\n", packName, source.Version)
	// List releases to find newer version
	releases, err := githubClient.ListReleases(ctx, source.Repository)
	if err != nil {
		return fmt.Errorf("list releases: %w", err)
	}

	// Find the newest version
	currentVersion, err := registry.ParseVersion(source.Version)
	if err != nil {
		return fmt.Errorf("parse current version: %w", err)
	}

	var newerRelease *registry.Release
	var newerVersion registry.Version
	for _, rel := range releases {
		tagVersion := registry.NormalizeTag(rel.TagName)
		v, parseErr := registry.ParseVersion(tagVersion)
		if parseErr != nil {
			continue
		}
		if v.Compare(currentVersion) > 0 && (newerRelease == nil || v.Compare(newerVersion) > 0) {
			newerVersion = v
			newerRelease = &rel
		}
	}

	if newerRelease == nil {
		// Already at latest version
		fmt.Fprintf(cmd.OutOrStdout(), "%s is already at the latest version (%s)\n", packName, source.Version)
		return nil
	}

	// Find the pack asset
	asset, err := findPackAsset(newerRelease.Assets)
	if err != nil {
		return err
	}

	// Get checksum for the asset
	checksum, err := findChecksumInRelease(ctx, newerRelease.Assets, asset.Name)
	if err != nil {
		return err
	}

	installer := workflowpkg.NewPackInstaller(effectiveCLIVersion())

	// Update the pack (force=true for replacing existing)
	packSource := workflowpkg.PackSource{
		Repository:  source.Repository,
		Version:     newerVersion.String(),
		InstalledAt: source.InstalledAt,
		UpdatedAt:   time.Now(),
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", asset.Name)
	if err := installer.Install(ctx, asset.DownloadURL, checksum, packDir, true, packSource); err != nil {
		return fmt.Errorf("update workflow pack: %w", err)
	}

	// Emit plugin dependency warnings (non-blocking)
	emitPluginWarnings(cmd, packDir, nil)

	fmt.Fprintf(cmd.OutOrStdout(), "Updated %s from %s to %s\n", packName, source.Version, newerVersion.String())
	return nil
}

func newWorkflowRemoveCommand(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <pack-name>",
		Short: "Remove an installed workflow pack",
		Long: `Remove an installed workflow pack by name.

Searches local then global workflow-packs directories.

Examples:
  awf workflow remove speckit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowRemove(cmd, cfg, args[0])
		},
	}
}

func newWorkflowSearchCommand(cfg *Config) *cobra.Command {
	var outputFlag string

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for available workflow packs on GitHub",
		Long: `Search GitHub for AWF workflow packs by topic or keyword.

Results are repositories tagged with the "awf-workflow" topic.
Use --output=json to get machine-readable output.

Examples:
  awf workflow search
  awf workflow search speckit
  awf workflow search --output=json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			if outputFlag != "" {
				format, err := ui.ParseOutputFormat(outputFlag)
				if err != nil {
					return err
				}
				cfg.OutputFormat = format
			}
			return runWorkflowSearch(cmd, cfg, query)
		},
	}

	cmd.Flags().StringVar(&outputFlag, "output", "", "Output format (json)")
	return cmd
}

func runWorkflowSearch(cmd *cobra.Command, cfg *Config, query string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	baseURL := os.Getenv("GITHUB_API_URL")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	searchQuery := "topic:awf-workflow"
	if query != "" {
		searchQuery += "+" + url.QueryEscape(query)
	}

	apiURL := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc", baseURL, searchQuery)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody) //nolint:gosec // G107: URL from validated env var or hardcoded base
	if err != nil {
		return fmt.Errorf("failed to search workflows: %w", err)
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: URL constructed from safe base + escaped query
	if err != nil {
		return fmt.Errorf("failed to search workflows: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-Ratelimit-Remaining") == "0" {
		return fmt.Errorf("GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits")
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
		return json.NewEncoder(cmd.OutOrStdout()).Encode(searchResult.Items) //nolint:wrapcheck // encoding error is terminal
	}

	if len(searchResult.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No workflows found")
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

func runWorkflowRemove(cmd *cobra.Command, _ *Config, name string) error {
	packDir := findPackDir(name)
	if packDir == "" {
		return fmt.Errorf("workflow pack %q not found in local or global directories", name)
	}

	if err := os.RemoveAll(packDir); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Removed workflow pack %q from %s\n", name, packDir)
	return nil
}
