//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Feature: C019 - Shared Test Helpers
// Common utilities for memory management and integration tests

// skipInCI skips the test if running in a CI environment.
// Tests requiring external API access use this helper since CI lacks credentials.
func skipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping test in CI environment")
	}
}

// skipIfRoot skips the test if running as root user.
// Tests that verify permission denials should not run as root.
func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Skip("Test requires non-root user")
	}
}

// skipIfCLIMissing skips the test if a required CLI tool is not installed.
// Common tools: claude, codex, gemini, dot (graphviz), golangci-lint
func skipIfCLIMissing(t *testing.T, cliName string) {
	t.Helper()
	if _, err := exec.LookPath(cliName); err != nil {
		t.Skipf("CLI tool %q not found in PATH", cliName)
	}
}

// skipOnPlatform skips the test if running on specified platform(s).
// Platform values: "windows", "darwin", "linux"
func skipOnPlatform(t *testing.T, platforms ...string) {
	t.Helper()
	for _, platform := range platforms {
		if runtime.GOOS == platform {
			t.Skipf("Test skipped on platform: %s", platform)
		}
	}
}

// skipIfToolMissing is a general helper that skips if any system tool is unavailable.
// Alias for skipIfCLIMissing for backward compatibility.
func skipIfToolMissing(t *testing.T, toolName string) {
	t.Helper()
	skipIfCLIMissing(t, toolName)
}

// getRepoRoot returns the repository root directory.
// It walks up from the current directory until it finds a go.mod file.
func getRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("should get current directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}
