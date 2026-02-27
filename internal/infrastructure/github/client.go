package github

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

// Client wraps GitHub API interactions via gh CLI and HTTP API with authentication fallback.
// Provides structured JSON output parsing and automatic repository detection.
type Client struct {
	// authMethod stores the detected authentication method (gh_cli, token, or none)
	authMethod AuthMethod

	// repo holds the GitHub repository in "owner/repo" format (auto-detected or explicit)
	repo string

	// repoOnce ensures repository detection runs exactly once
	repoOnce sync.Once

	// repoErr stores any error from repository detection
	repoErr error

	// logger provides structured logging for client operations
	logger ports.Logger
}

// NewClient creates a new GitHub client with the provided logger.
//
// Parameters:
//   - logger: structured logger for client operations
//
// Returns:
//   - *Client: configured GitHub client ready for use
func NewClient(logger ports.Logger) *Client {
	return &Client{
		logger: logger,
	}
}

// RunGH executes a gh CLI command and returns the raw stdout output.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - args: command arguments to pass to gh CLI (e.g., ["pr", "create", "--title", "..."])
//
// Returns:
//   - []byte: stdout output from gh CLI
//   - error: execution error
func (c *Client) RunGH(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if c.logger != nil {
		c.logger.Debug("executing gh command", "args", strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:min(len(args), 2)], " "), strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}

// DetectRepo determines the GitHub repository from git remote configuration.
// Parses git remote URL to extract owner/repo.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//
// Returns:
//   - string: repository in "owner/repo" format
//   - error: git command error or parsing error
func (c *Client) DetectRepo(ctx context.Context) (string, error) {
	c.repoOnce.Do(func() {
		out, err := exec.CommandContext(ctx, "git", "remote", "get-url", "origin").Output()
		if err != nil {
			c.repoErr = fmt.Errorf("detect repo: %w", err)
			return
		}

		repo := parseRepoFromRemote(strings.TrimSpace(string(out)))
		if repo == "" {
			c.repoErr = fmt.Errorf("detect repo: cannot parse remote URL %q", string(out))
			return
		}

		c.repo = repo
	})
	return c.repo, c.repoErr
}

// parseRepoFromRemote extracts "owner/repo" from a git remote URL.
// Supports SSH (git@github.com:owner/repo.git) and HTTPS (https://github.com/owner/repo.git).
func parseRepoFromRemote(remoteURL string) string {
	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) == 2 && strings.Contains(parts[0], "github.com") {
			return strings.TrimSuffix(parts[1], ".git")
		}
	}

	// HTTPS format: https://github.com/owner/repo.git
	if strings.Contains(remoteURL, "github.com/") {
		idx := strings.Index(remoteURL, "github.com/")
		repo := remoteURL[idx+len("github.com/"):]
		return strings.TrimSuffix(repo, ".git")
	}

	return ""
}
