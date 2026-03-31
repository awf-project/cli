package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/pkg/registry"
	"github.com/spf13/cobra"
)

// workflowAPIDoer wraps an HTTP client to redirect api.github.com requests to GITHUB_API_URL when set (for testing).
type workflowAPIDoer struct {
	inner   *http.Client
	apiBase string
}

func newWorkflowAPIDoer(apiBase string, inner *http.Client) *workflowAPIDoer {
	return &workflowAPIDoer{inner: inner, apiBase: apiBase}
}

func (d *workflowAPIDoer) Do(req *http.Request) (*http.Response, error) {
	if d.apiBase != "" && req.URL != nil && req.URL.Host == "api.github.com" {
		base, err := url.Parse(d.apiBase)
		if err != nil {
			return nil, fmt.Errorf("invalid GITHUB_API_URL: %w", err)
		}
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = base.Scheme
		cloned.URL.Host = base.Host
		cloned.Host = base.Host
		req = cloned
	}
	return d.inner.Do(req) //nolint:wrapcheck,gosec // delegating to inner client; URL rewritten from validated GITHUB_API_URL
}

func newWorkflowCommand(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workflow",
		Short:   "Manage AWF workflow packs",
		Aliases: []string{"wf"},
	}

	cmd.AddCommand(newWorkflowInstallCommand(cfg))
	cmd.AddCommand(newWorkflowRemoveCommand(cfg))

	return cmd
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

	httpClient := &http.Client{Timeout: 30 * 1e9}
	checksumData, err := registry.Download(ctx, httpClient, checksumAsset.DownloadURL)
	if err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}

	// Parse checksum from checksums.txt: "<hex>  <filename>"
	checksumLines := strings.Split(string(checksumData), "\n")
	for _, line := range checksumLines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum for %s not found in checksums.txt", assetName)
}

func runWorkflowInstall(_ *cobra.Command, _ *Config, source string, flags workflowInstallFlags) error {
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

	// Create GitHub client with GITHUB_API_URL support for testing
	doer := newWorkflowAPIDoer(os.Getenv("GITHUB_API_URL"), &http.Client{Timeout: 30 * 1e9})
	githubClient := registry.NewGitHubReleaseClient(doer)

	// Resolve the version
	ctx := context.Background()
	resolvedVersion, err := githubClient.ResolveVersion(ctx, ownerRepo, versionConstraint, false)
	if err != nil {
		return fmt.Errorf("resolve version: %w", err)
	}

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

	// Create installer
	// Use a valid version for version constraint checking; use "0.5.0" for "dev" builds during testing
	cliVersion := Version
	if cliVersion == "dev" {
		cliVersion = "0.5.0"
	}
	installer := workflowpkg.NewPackInstaller(cliVersion)

	// Install the pack
	packSource := workflowpkg.PackSource{
		Repository: ownerRepo,
		Version:    resolvedVersion.String(),
	}

	if err := installer.Install(ctx, asset.DownloadURL, checksum, targetDir, flags.force, packSource); err != nil {
		return fmt.Errorf("install workflow pack: %w", err)
	}

	// Emit plugin dependency warnings (non-blocking)
	emitPluginWarnings(targetDir)

	return nil
}

// emitPluginWarnings reads the installed manifest and warns about missing plugin dependencies.
func emitPluginWarnings(packDir string) {
	manifestPath := filepath.Join(packDir, "manifest.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return
	}

	manifest, err := workflowpkg.ParseManifest(data)
	if err != nil || len(manifest.Plugins) == 0 {
		return
	}

	for pluginName, versionConstraint := range manifest.Plugins {
		fmt.Fprintf(os.Stderr, "Warning: pack requires plugin %q (%s) — install with: awf plugin install <owner>/%s\n", pluginName, versionConstraint, pluginName)
	}
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

func runWorkflowRemove(_ *cobra.Command, _ *Config, name string) error {
	localPackDir := filepath.Join(xdg.LocalWorkflowPacksDir(), name)
	if _, err := os.Stat(localPackDir); err == nil {
		return os.RemoveAll(localPackDir)
	}

	globalPackDir := filepath.Join(xdg.AWFWorkflowPacksDir(), name)
	if _, err := os.Stat(globalPackDir); err == nil {
		return os.RemoveAll(globalPackDir)
	}

	return fmt.Errorf("workflow pack %q not found in local or global directories", name)
}
