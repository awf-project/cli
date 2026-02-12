package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// AuthMethod represents the authentication method used for GitHub API access.
// Supports three-tier authentication chain: gh CLI > GITHUB_TOKEN > none.
type AuthMethod struct {
	// Type identifies the authentication method: "gh_cli", "token", or "none"
	Type string

	// Token stores the authentication token when Type is "token"
	// Empty for "gh_cli" and "none" types
	Token string
}

// String returns a log-safe string representation of the auth method.
// Never exposes tokens or secrets.
func (a AuthMethod) String() string {
	switch a.Type {
	case "gh_cli":
		return "gh_cli"
	case "token":
		return "token(***)"
	case "none":
		return "none"
	default:
		return fmt.Sprintf("unknown(%s)", a.Type)
	}
}

// IsAuthenticated returns true if the auth method can authenticate GitHub API requests.
// Returns false for Type "none".
func (a AuthMethod) IsAuthenticated() bool {
	return a.Type == "gh_cli" || a.Type == "token"
}

// DetectAuth determines the active GitHub authentication method.
// Priority chain: gh CLI > GITHUB_TOKEN environment variable > none.
//
// Returns:
//   - AuthMethod{Type: "gh_cli"} if gh CLI is authenticated
//   - AuthMethod{Type: "token", Token: value} if GITHUB_TOKEN is set
//   - AuthMethod{Type: "none"} if no auth available
//   - error with remediation hints if detection fails
func DetectAuth(ctx context.Context) (AuthMethod, error) {
	// Check for context cancellation before starting
	if ctx.Err() != nil {
		return AuthMethod{Type: "none"}, fmt.Errorf("auth detection canceled: %w", ctx.Err())
	}

	// Priority 1: Check gh CLI authentication
	// Run "gh auth status" - exit code 0 means authenticated
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	if err := cmd.Run(); err == nil {
		// gh CLI is authenticated
		return AuthMethod{Type: "gh_cli"}, nil
	}
	// Fall through on error (gh not installed or not authenticated)

	// Priority 2: Check GITHUB_TOKEN environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		// Trim whitespace - treat whitespace-only as empty
		token = strings.TrimSpace(token)
		if token != "" {
			return AuthMethod{
				Type:  "token",
				Token: token,
			}, nil
		}
	}

	// Priority 3: No authentication available
	return AuthMethod{Type: "none"}, nil
}
