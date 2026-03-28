package pluginmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ValidateOwnerRepo validates that the input is a valid GitHub repository reference in owner/repo format.
//
// Valid format: owner/repo where both owner and repo are non-empty strings.
// Rejects:
//   - No slash separator
//   - Multiple slashes
//   - Empty segments (before/after slash)
//   - https:// prefix
//   - Strings longer than 200 characters
//
// Parameters:
//   - ownerRepo: the owner/repo string to validate
//
// Returns:
//   - error: validation error, or nil if valid
func ValidateOwnerRepo(ownerRepo string) error {
	// Reject https prefix
	if strings.HasPrefix(ownerRepo, "https://") || strings.HasPrefix(ownerRepo, "http://") {
		return fmt.Errorf("invalid owner/repo format: cannot contain URL protocol")
	}

	// Check length
	if len(ownerRepo) > 200 {
		return fmt.Errorf("invalid owner/repo format: length exceeds 200 characters")
	}

	// Check for exactly one slash
	slashCount := strings.Count(ownerRepo, "/")
	if slashCount == 0 {
		return fmt.Errorf("invalid owner/repo format: missing slash separator")
	}
	if slashCount > 1 {
		return fmt.Errorf("invalid owner/repo format: multiple slashes not allowed")
	}

	// Split and validate non-empty segments
	parts := strings.Split(ownerRepo, "/")
	owner, repo := parts[0], parts[1]

	if owner == "" {
		return fmt.Errorf("invalid owner/repo format: empty owner segment")
	}
	if repo == "" {
		return fmt.Errorf("invalid owner/repo format: empty repo segment")
	}

	return nil
}

// httpDoer abstracts HTTP request execution for testing.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Release represents a GitHub release.
type Release struct {
	TagName    string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	URL        string  `json:"url"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a release asset (binary, archive, etc).
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int    `json:"size"`
}

// GitHubReleaseClient fetches and resolves releases from GitHub.
type GitHubReleaseClient struct {
	doer httpDoer
}

// NewGitHubReleaseClient creates a new GitHub release client.
// If doer is nil, uses the default http.DefaultClient.
func NewGitHubReleaseClient(doer httpDoer) *GitHubReleaseClient {
	if doer == nil {
		return &GitHubReleaseClient{doer: http.DefaultClient}
	}
	return &GitHubReleaseClient{doer: doer}
}

// ListReleases fetches all releases for an owner/repo from GitHub API.
func (c *GitHubReleaseClient) ListReleases(ctx context.Context, ownerRepo string) ([]Release, error) {
	if err := ValidateOwnerRepo(ownerRepo); err != nil {
		return nil, fmt.Errorf("invalid owner/repo: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases", ownerRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	c.buildGitHubHeaders(ctx, req)

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for rate limit (429 status or X-RateLimit-Remaining: 0)
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limit exceeded from GitHub API")
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return nil, fmt.Errorf("rate limit exceeded from GitHub API")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.Unmarshal(bodyBytes, &releases); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return releases, nil
}

// ResolveVersion resolves the best matching version from available releases.
// constraintStr: empty string or version constraint like ">=1.0.0 <2.0.0"
// includePrerelease: if true, includes pre-release versions
func (c *GitHubReleaseClient) ResolveVersion(ctx context.Context, ownerRepo, constraintStr string, includePrerelease bool) (Version, error) {
	if err := ValidateOwnerRepo(ownerRepo); err != nil {
		return Version{}, fmt.Errorf("invalid owner/repo: %w", err)
	}

	releases, err := c.ListReleases(ctx, ownerRepo)
	if err != nil {
		return Version{}, err
	}

	// Parse constraint if provided
	var constraints Constraints
	if constraintStr != "" {
		var err error
		constraints, err = ParseConstraints(constraintStr)
		if err != nil {
			return Version{}, fmt.Errorf("invalid constraint: %w", err)
		}
	}

	// Filter and sort versions
	var candidates []Version
	for _, release := range releases {
		// Filter by prerelease preference
		if release.Prerelease && !includePrerelease {
			continue
		}

		// Parse version from tag name (normalize v prefix)
		versionStr := NormalizeTag(release.TagName)
		version, err := ParseVersion(versionStr)
		if err != nil {
			// Skip invalid tags
			continue
		}

		// Check constraint
		if !constraints.Check(version) {
			continue
		}

		candidates = append(candidates, version)
	}

	if len(candidates) == 0 {
		return Version{}, fmt.Errorf("no matching versions found for constraint %q", constraintStr)
	}

	// Sort in descending order and return the highest
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Compare(candidates[j]) > 0
	})

	return candidates[0], nil
}

// FindPlatformAsset finds the .tar.gz asset matching goos/goarch from a release's asset list.
// Pattern: awf-plugin-name_version_os_arch.tar.gz
// Returns an error listing available platforms when no match is found.
func FindPlatformAsset(assets []Asset, goos, goarch string) (Asset, error) {
	pattern := fmt.Sprintf("_%s_%s.tar.gz", goos, goarch)
	var availablePlatforms []string

	for _, asset := range assets {
		// Only consider .tar.gz files
		if !strings.HasSuffix(asset.Name, ".tar.gz") {
			continue
		}

		// Check if this asset matches the requested platform
		if strings.Contains(asset.Name, pattern) {
			return asset, nil
		}

		// Extract os_arch from filename for error message
		base := strings.TrimSuffix(asset.Name, ".tar.gz")
		parts := strings.Split(base, "_")
		if len(parts) >= 3 {
			// The last two underscore-separated parts should be os and arch
			osName := parts[len(parts)-2]
			archName := parts[len(parts)-1]
			availablePlatforms = append(availablePlatforms, osName+"_"+archName)
		}
	}

	// No match found - return error that contains requested OS, arch, and available platforms
	if len(availablePlatforms) > 0 {
		return Asset{}, fmt.Errorf("no matching asset found for %s %s. Available: %s", goos, goarch, strings.Join(availablePlatforms, ", "))
	}
	return Asset{}, fmt.Errorf("no matching asset found for %s %s", goos, goarch)
}

// buildGitHubHeaders sets HTTP headers for GitHub API requests on req.
// Implements 3-tier authentication: GITHUB_TOKEN env, gh auth token CLI, unauthenticated.
func (c *GitHubReleaseClient) buildGitHubHeaders(ctx context.Context, req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")

	token := os.Getenv("GITHUB_TOKEN")

	if token == "" {
		// Try gh auth token as fallback
		out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
		if err == nil {
			token = strings.TrimSpace(string(out))
		}
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
