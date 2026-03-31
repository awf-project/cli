package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/awf-project/cli/pkg/httpx"
)

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
	doer httpx.HTTPDoer
}

// ValidateOwnerRepo validates that the input is a valid GitHub repository reference in owner/repo format.
func ValidateOwnerRepo(ownerRepo string) error {
	if strings.HasPrefix(ownerRepo, "https://") || strings.HasPrefix(ownerRepo, "http://") {
		return fmt.Errorf("invalid owner/repo format: cannot contain URL protocol")
	}

	if len(ownerRepo) > 200 {
		return fmt.Errorf("invalid owner/repo format: length exceeds 200 characters")
	}

	owner, repo, found := strings.Cut(ownerRepo, "/")
	if !found {
		return fmt.Errorf("invalid owner/repo format: missing slash separator")
	}
	if strings.Contains(repo, "/") {
		return fmt.Errorf("invalid owner/repo format: multiple slashes not allowed")
	}

	if owner == "" {
		return fmt.Errorf("invalid owner/repo format: empty owner segment")
	}
	if repo == "" {
		return fmt.Errorf("invalid owner/repo format: empty repo segment")
	}

	return nil
}

// NewGitHubReleaseClient creates a new GitHub release client.
// If doer is nil, uses http.DefaultClient.
func NewGitHubReleaseClient(doer httpx.HTTPDoer) *GitHubReleaseClient {
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

	if resp.StatusCode == 429 || resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return nil, fmt.Errorf("rate limit exceeded from GitHub API")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return releases, nil
}

// ResolveVersion resolves the best matching version from available releases.
func (c *GitHubReleaseClient) ResolveVersion(ctx context.Context, ownerRepo, constraintStr string, includePrerelease bool) (Version, error) {
	releases, err := c.ListReleases(ctx, ownerRepo)
	if err != nil {
		return Version{}, err
	}

	var constraints Constraints
	if constraintStr != "" {
		constraints, err = ParseConstraints(constraintStr)
		if err != nil {
			return Version{}, fmt.Errorf("invalid constraint: %w", err)
		}
	}

	var candidates []Version
	for _, release := range releases {
		if release.Prerelease && !includePrerelease {
			continue
		}

		versionStr := NormalizeTag(release.TagName)
		version, err := ParseVersion(versionStr)
		if err != nil {
			continue
		}

		if !constraints.Check(version) {
			continue
		}

		candidates = append(candidates, version)
	}

	if len(candidates) == 0 {
		return Version{}, fmt.Errorf("no matching versions found for constraint %q", constraintStr)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Compare(candidates[j]) > 0
	})

	return candidates[0], nil
}

// FindPlatformAsset finds the .tar.gz asset matching goos/goarch from a release's asset list.
func FindPlatformAsset(assets []Asset, goos, goarch string) (Asset, error) {
	pattern := fmt.Sprintf("_%s_%s.tar.gz", goos, goarch)
	var availablePlatforms []string

	for _, asset := range assets {
		if !strings.HasSuffix(asset.Name, ".tar.gz") {
			continue
		}

		if strings.Contains(asset.Name, pattern) {
			return asset, nil
		}

		base := strings.TrimSuffix(asset.Name, ".tar.gz")
		parts := strings.Split(base, "_")
		if len(parts) >= 3 {
			osName := parts[len(parts)-2]
			archName := parts[len(parts)-1]
			availablePlatforms = append(availablePlatforms, osName+"_"+archName)
		}
	}

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
		out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
		if err == nil {
			token = strings.TrimSpace(string(out))
		}
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
