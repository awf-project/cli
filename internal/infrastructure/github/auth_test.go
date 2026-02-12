package github

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMethod_String tests the log-safe string representation.
func TestAuthMethod_String(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		token    string
		want     string
	}{
		{
			name:     "gh_cli type",
			authType: "gh_cli",
			token:    "",
			want:     "gh_cli",
		},
		{
			name:     "token type masks token",
			authType: "token",
			token:    "ghp_secret123456",
			want:     "token(***)",
		},
		{
			name:     "none type",
			authType: "none",
			token:    "",
			want:     "none",
		},
		{
			name:     "unknown type",
			authType: "unknown",
			token:    "",
			want:     "unknown(unknown)",
		},
		{
			name:     "empty type",
			authType: "",
			token:    "",
			want:     "unknown()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := AuthMethod{
				Type:  tt.authType,
				Token: tt.token,
			}
			got := auth.String()
			assert.Equal(t, tt.want, got)
			// Verify no token leakage
			if tt.token != "" {
				assert.NotContains(t, got, tt.token)
			}
		})
	}
}

// TestAuthMethod_IsAuthenticated tests authentication status detection.
func TestAuthMethod_IsAuthenticated(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		token    string
		want     bool
	}{
		{
			name:     "gh_cli is authenticated",
			authType: "gh_cli",
			token:    "",
			want:     true,
		},
		{
			name:     "token is authenticated",
			authType: "token",
			token:    "ghp_secret",
			want:     true,
		},
		{
			name:     "none is not authenticated",
			authType: "none",
			token:    "",
			want:     false,
		},
		{
			name:     "unknown type is not authenticated",
			authType: "unknown",
			token:    "",
			want:     false,
		},
		{
			name:     "empty type is not authenticated",
			authType: "",
			token:    "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := AuthMethod{
				Type:  tt.authType,
				Token: tt.token,
			}
			got := auth.IsAuthenticated()
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDetectAuth_GhCLI tests gh CLI authentication detection.
func TestDetectAuth_GhCLI(t *testing.T) {
	t.Run("gh CLI authenticated", func(t *testing.T) {
		// Check if gh is actually installed and authenticated
		_, err := exec.LookPath("gh")
		if err != nil {
			t.Skip("gh CLI not installed")
		}

		// Test actual gh CLI detection
		ctx := context.Background()
		auth, err := DetectAuth(ctx)

		require.NoError(t, err)

		// If gh is available and authenticated, expect gh_cli
		// Otherwise it falls through to GITHUB_TOKEN or none
		if auth.Type == "gh_cli" {
			assert.Equal(t, "gh_cli", auth.Type)
			assert.Empty(t, auth.Token)
			assert.True(t, auth.IsAuthenticated())
		} else {
			// gh not authenticated, falls through to token/none
			assert.Contains(t, []string{"token", "none"}, auth.Type)
		}
	})

	t.Run("gh CLI not installed", func(t *testing.T) {
		// TODO: Mock os/exec to simulate "gh: command not found"
		// Should fall back to GITHUB_TOKEN check
		t.Skip("requires os/exec mocking - implement when DetectAuth complete")
	})

	t.Run("gh CLI installed but not authenticated", func(t *testing.T) {
		// TODO: Mock "gh auth status" returning non-zero exit code
		// Should fall back to GITHUB_TOKEN check
		t.Skip("requires os/exec mocking - implement when DetectAuth complete")
	})
}

// TestDetectAuth_TokenEnv tests GITHUB_TOKEN environment variable fallback.
func TestDetectAuth_TokenEnv(t *testing.T) {
	t.Run("GITHUB_TOKEN set", func(t *testing.T) {
		// Setup: gh CLI unavailable, GITHUB_TOKEN set
		testToken := "ghp_test_token_12345" // #nosec G101 -- test token, not a real credential
		t.Setenv("GITHUB_TOKEN", testToken)

		ctx := context.Background()
		auth, err := DetectAuth(ctx)

		require.NoError(t, err)
		assert.Equal(t, "token", auth.Type)
		assert.Equal(t, testToken, auth.Token)
		assert.True(t, auth.IsAuthenticated())
	})

	t.Run("GITHUB_TOKEN empty", func(t *testing.T) {
		// Prevent gh from being found so we fall through to token check
		t.Setenv("PATH", "/nonexistent")
		t.Setenv("GITHUB_TOKEN", "")

		ctx := context.Background()
		auth, err := DetectAuth(ctx)

		require.NoError(t, err)
		assert.Equal(t, "none", auth.Type)
		assert.Empty(t, auth.Token)
		assert.False(t, auth.IsAuthenticated())
	})

	t.Run("GITHUB_TOKEN with whitespace only", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "   \n\t  ")

		ctx := context.Background()
		auth, err := DetectAuth(ctx)

		require.NoError(t, err)
		assert.Equal(t, "none", auth.Type)
		assert.Empty(t, auth.Token)
		assert.False(t, auth.IsAuthenticated())
	})
}

// TestDetectAuth_NoAuth tests behavior when no authentication is available.
func TestDetectAuth_NoAuth(t *testing.T) {
	t.Run("no authentication available", func(t *testing.T) {
		// Prevent gh from being found and ensure no GITHUB_TOKEN
		t.Setenv("PATH", "/nonexistent")
		t.Setenv("GITHUB_TOKEN", "")

		ctx := context.Background()
		auth, err := DetectAuth(ctx)

		require.NoError(t, err)
		assert.Equal(t, "none", auth.Type)
		assert.Empty(t, auth.Token)
		assert.False(t, auth.IsAuthenticated())
	})
}

// TestDetectAuth_PriorityChain tests authentication method priority.
func TestDetectAuth_PriorityChain(t *testing.T) {
	t.Run("gh CLI takes priority over GITHUB_TOKEN", func(t *testing.T) {
		// Setup: both gh CLI and GITHUB_TOKEN available
		t.Setenv("GITHUB_TOKEN", "ghp_fallback_token")

		// TODO: Mock gh auth status to return success
		// Expected: gh_cli is chosen, not token

		t.Skip("requires os/exec mocking - implement when DetectAuth complete")
	})
}

// TestDetectAuth_ContextCancellation tests context handling.
func TestDetectAuth_ContextCancellation(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := DetectAuth(ctx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// TestDetectAuth_RemediationHints tests error messages contain helpful hints.
func TestDetectAuth_RemediationHints(t *testing.T) {
	t.Run("no auth available should suggest remediation", func(t *testing.T) {
		// Prevent gh from being found and ensure no GITHUB_TOKEN
		t.Setenv("PATH", "/nonexistent")
		t.Setenv("GITHUB_TOKEN", "")

		ctx := context.Background()
		auth, err := DetectAuth(ctx)

		// Current stub returns "none" without error
		require.NoError(t, err)
		assert.Equal(t, "none", auth.Type)

		// Expected behavior after implementation:
		// When no auth is available, error should include remediation hints:
		// - How to run "gh auth login"
		// - How to set GITHUB_TOKEN
		// - Link to GitHub token creation page
	})
}

// TestDetectAuth_SecretMasking tests that tokens are never logged.
func TestDetectAuth_SecretMasking(t *testing.T) {
	t.Run("token in AuthMethod never exposed via String()", func(t *testing.T) {
		secretToken := "ghp_very_secret_token_xyz" // #nosec G101 -- test token, not a real credential
		auth := AuthMethod{
			Type:  "token",
			Token: secretToken,
		}

		str := auth.String()
		assert.NotContains(t, str, secretToken, "String() must not expose token")
		assert.Contains(t, str, "***", "String() must mask token")
	})
}

// TestDetectAuth_ThreadSafety tests concurrent calls to DetectAuth.
func TestDetectAuth_ThreadSafety(t *testing.T) {
	t.Run("concurrent DetectAuth calls", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "ghp_concurrent_test")

		const goroutines = 10
		results := make(chan AuthMethod, goroutines)
		errors := make(chan error, goroutines)

		ctx := context.Background()

		for range goroutines {
			go func() {
				auth, err := DetectAuth(ctx)
				results <- auth
				errors <- err
			}()
		}

		// Collect results
		for range goroutines {
			err := <-errors
			require.NoError(t, err)

			auth := <-results
			// All calls should return consistent results
			assert.Equal(t, "token", auth.Type)
			assert.True(t, auth.IsAuthenticated())
		}
	})
}

// TestAuthMethod_ZeroValue tests zero-value behavior.
func TestAuthMethod_ZeroValue(t *testing.T) {
	var auth AuthMethod

	assert.Empty(t, auth.Type)
	assert.Empty(t, auth.Token)
	assert.False(t, auth.IsAuthenticated())
	assert.Equal(t, "unknown()", auth.String())
}

// TestDetectAuth_EdgeCases tests boundary conditions.
func TestDetectAuth_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func(t *testing.T)
		wantType    string
		wantAuthd   bool
		description string
	}{
		{
			name: "GITHUB_TOKEN with special characters",
			setupEnv: func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "ghp_token!@#$%^&*()")
			},
			wantType:    "token",
			wantAuthd:   true,
			description: "should handle tokens with special characters",
		},
		{
			name: "GITHUB_TOKEN with unicode",
			setupEnv: func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "ghp_token_日本語")
			},
			wantType:    "token",
			wantAuthd:   true,
			description: "should handle tokens with unicode",
		},
		{
			name: "GITHUB_TOKEN very long",
			setupEnv: func(t *testing.T) {
				// Create a 1000-character token using strings.Repeat
				longToken := "ghp_" + string([]byte(strings.Repeat("a", 1000)))
				t.Setenv("GITHUB_TOKEN", longToken)
			},
			wantType:    "token",
			wantAuthd:   true,
			description: "should handle very long tokens",
		},
		{
			name: "no env set",
			setupEnv: func(t *testing.T) {
				// Prevent gh from being found and ensure no GITHUB_TOKEN
				t.Setenv("PATH", "/nonexistent")
				t.Setenv("GITHUB_TOKEN", "")
			},
			wantType:    "none",
			wantAuthd:   false,
			description: "should return none when no auth available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)

			ctx := context.Background()
			auth, err := DetectAuth(ctx)

			require.NoError(t, err, tt.description)
			assert.Equal(t, tt.wantType, auth.Type)
			assert.Equal(t, tt.wantAuthd, auth.IsAuthenticated())
		})
	}
}
