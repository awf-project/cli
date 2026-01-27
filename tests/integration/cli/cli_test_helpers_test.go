//go:build integration

package cli_test

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Shared Test Helpers for CLI Integration Tests
// =============================================================================

// setupTestDir creates a temporary directory for test isolation.
// Use this when you need a clean directory for XDG_CONFIG_HOME or similar.
// Does NOT change the current working directory.
func setupTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// setupInitTestDir creates a temporary directory and changes the current
// working directory to it. The original working directory is restored when
// the test completes.
func setupInitTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("failed to restore original directory: %v", err)
		}
	})

	return tmpDir
}

// createTestWorkflow creates a workflow file in the .awf/workflows directory
// within the given base directory. Creates the directory structure if needed.
func createTestWorkflow(t *testing.T, baseDir, filename, content string) {
	t.Helper()

	workflowsDir := filepath.Join(baseDir, ".awf", "workflows")
	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatalf("failed to create workflows directory: %v", err)
	}

	workflowPath := filepath.Join(workflowsDir, filename)
	if err := os.WriteFile(workflowPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}
}

// findRepoRoot walks up the directory tree to find the repository root
// by looking for go.mod file.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
