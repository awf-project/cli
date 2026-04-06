package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/awf-project/cli/internal/infrastructure/updater"
	"github.com/awf-project/cli/pkg/registry"
	"github.com/spf13/cobra"
)

const upgradeOwnerRepo = "awf-project/cli"

type upgradeOptions struct {
	check   bool
	force   bool
	version string
}

func newUpgradeCommand(cfg *Config) *cobra.Command {
	var opts upgradeOptions

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade AWF to the latest version",
		Long: `Check for and install AWF updates from GitHub releases.

Downloads the appropriate binary for your platform, verifies its SHA256
checksum, and replaces the current binary atomically.

Examples:
  awf upgrade                     # Upgrade to latest version
  awf upgrade --check             # Check without installing
  awf upgrade --version v0.5.0    # Install specific version
  awf upgrade --force             # Force upgrade (skip version check)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd, cfg, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.check, "check", false, "check for updates without installing")
	cmd.Flags().BoolVar(&opts.force, "force", false, "force upgrade even if already on latest")
	cmd.Flags().StringVar(&opts.version, "version", "", "install a specific version (e.g. v0.5.0)")

	return cmd
}

func runUpgrade(cmd *cobra.Command, _ *Config, opts upgradeOptions) error {
	isDevBuild := Version == "dev"

	if isDevBuild && !opts.force {
		return fmt.Errorf("cannot determine current version (dev build); use --force to upgrade anyway")
	}

	ctx := context.Background()

	doer := registry.NewGitHubAPIDoer(os.Getenv("GITHUB_API_URL"), http.DefaultClient)
	githubClient := registry.NewGitHubReleaseClient(doer)

	releases, err := githubClient.ListReleases(ctx, upgradeOwnerRepo)
	if err != nil {
		return fmt.Errorf("failed to fetch releases: %w", err)
	}
	if len(releases) == 0 {
		return fmt.Errorf("no releases found for %s", upgradeOwnerRepo)
	}

	release, err := selectTargetRelease(releases, opts.version)
	if err != nil {
		return err
	}

	targetVersion := registry.NormalizeTag(release.TagName)

	if upToDate, msg := isAlreadyUpToDate(cmd, targetVersion, release.TagName, opts, isDevBuild); upToDate {
		fmt.Fprint(cmd.OutOrStdout(), msg)
		return nil
	}

	if opts.check {
		return printUpdateAvailable(cmd, release.TagName, isDevBuild)
	}

	return downloadAndInstall(ctx, cmd, release, isDevBuild)
}

// isAlreadyUpToDate checks if the current version matches or exceeds the target.
// Returns true with a message if no upgrade is needed, false otherwise.
func isAlreadyUpToDate(cmd *cobra.Command, targetVersion, tagName string, opts upgradeOptions, isDevBuild bool) (upToDate bool, message string) {
	if opts.force || opts.version != "" || isDevBuild {
		return false, ""
	}

	currentVersion := registry.NormalizeTag(Version)
	if currentVersion == targetVersion {
		return true, fmt.Sprintf("AWF is already up to date (version %s)\n", tagName)
	}

	current, parseErr := registry.ParseVersion(currentVersion)
	if parseErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: cannot parse current version %q, skipping version comparison\n", currentVersion)
		return false, ""
	}

	target, targetErr := registry.ParseVersion(targetVersion)
	if targetErr == nil && current.Compare(target) >= 0 {
		return true, fmt.Sprintf("AWF is already up to date (version %s)\n", tagName)
	}

	return false, ""
}

func printUpdateAvailable(cmd *cobra.Command, tagName string, isDevBuild bool) error {
	if isDevBuild {
		fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s (current: dev build)\n", tagName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s (current: v%s)\n", tagName, registry.NormalizeTag(Version))
	}
	return nil
}

func downloadAndInstall(ctx context.Context, cmd *cobra.Command, release registry.Release, isDevBuild bool) error {
	asset, err := registry.FindPlatformAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return fmt.Errorf("no compatible binary found: %w", err)
	}

	checksumURL := findChecksumURL(release.Assets)
	if checksumURL == "" {
		return fmt.Errorf("no checksum file found in release %s", release.TagName)
	}

	checksumData, err := registry.Download(ctx, http.DefaultClient, checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksum file: %w", err)
	}

	expectedChecksum := registry.ExtractChecksumForAsset(string(checksumData), asset.Name)
	if expectedChecksum == "" {
		return fmt.Errorf("checksum for %q not found in checksum file", asset.Name)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", asset.Name)
	archiveData, err := registry.Download(ctx, http.DefaultClient, asset.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}

	err = registry.VerifyChecksum(archiveData, expectedChecksum)
	if err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	newBinary, err := extractBinary(archiveData)
	if err != nil {
		return err
	}

	return replaceBinaryAtExecPath(cmd, newBinary, release.TagName, isDevBuild)
}

func extractBinary(archiveData []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "awf-upgrade-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	err = registry.ExtractTarGz(archiveData, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	newBinaryPath := filepath.Join(tmpDir, "awf")
	newBinary, err := os.ReadFile(newBinaryPath)
	if err != nil {
		return nil, fmt.Errorf("awf binary not found in release archive: %w", err)
	}

	return newBinary, nil
}

func replaceBinaryAtExecPath(cmd *cobra.Command, newBinary []byte, tagName string, isDevBuild bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine binary path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		resolvedPath = execPath
	}
	if updater.IsPackageManagerPath(resolvedPath) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s appears to be managed by a package manager. Consider using your package manager to update instead.\n", resolvedPath)
	}

	dir := filepath.Dir(resolvedPath)
	if err = checkWritePermission(dir); err != nil {
		return fmt.Errorf("no write permission on %s: %w (try running with sudo)", dir, err)
	}

	if err = updater.ReplaceBinary(execPath, newBinary); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	if isDevBuild {
		fmt.Fprintf(cmd.OutOrStdout(), "Upgraded from dev to %s\n", tagName)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Upgraded from v%s to %s\n", registry.NormalizeTag(Version), tagName)
	}
	return nil
}

func selectTargetRelease(releases []registry.Release, targetVersion string) (registry.Release, error) {
	if targetVersion != "" {
		normalized := registry.NormalizeTag(targetVersion)
		for _, r := range releases {
			if registry.NormalizeTag(r.TagName) == normalized {
				return r, nil
			}
		}
		return registry.Release{}, fmt.Errorf("version %s not found", targetVersion)
	}

	for _, r := range releases {
		if !r.Prerelease {
			return r, nil
		}
	}
	return registry.Release{}, fmt.Errorf("no stable releases found")
}

func checkWritePermission(dir string) error {
	tmpFile, err := os.CreateTemp(dir, ".awf-perm-check-*")
	if err != nil {
		return err
	}
	name := tmpFile.Name()
	_ = tmpFile.Close() //nolint:errcheck // temp file for permission check only
	_ = os.Remove(name) //nolint:errcheck // best-effort cleanup
	return nil
}
