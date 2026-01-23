package cli_test

// Thread-safe test helpers for CLI test suite (C015 T002)
// Replaces os.Chdir with t.TempDir() and t.Setenv() patterns to eliminate race conditions

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestDir creates an isolated test directory with .awf/workflows structure.
// Returns the temporary directory path.
// Thread-safe: uses t.TempDir() instead of os.Chdir to avoid process-wide state changes.
//
// Usage:
//
//	tmpDir := setupTestDir(t)
//	// Write test files to tmpDir
//	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "workflow"})
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create standard .awf directory structure
	awfDir := filepath.Join(tmpDir, ".awf")
	workflowsDir := filepath.Join(awfDir, "workflows")
	promptsDir := filepath.Join(awfDir, "prompts")

	if err := os.MkdirAll(workflowsDir, 0o755); err != nil {
		t.Fatalf("failed to create workflows directory: %v", err)
	}
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("failed to create prompts directory: %v", err)
	}

	// Set PWD environment variable to the test directory
	// This makes relative path resolution work without os.Chdir
	setTestEnv(t, "PWD", tmpDir)

	// Set AWF_WORKFLOWS_PATH to point to the test workflows directory
	// This ensures the CLI loads workflows from the test directory, not the project root
	setTestEnv(t, "AWF_WORKFLOWS_PATH", workflowsDir)

	// Set AWF_PROMPTS_PATH to point to the test prompts directory
	// This ensures the CLI loads prompts from the test directory, not the project root
	setTestEnv(t, "AWF_PROMPTS_PATH", promptsDir)

	return tmpDir
}

// setTestEnv sets an environment variable for the duration of the test.
// Thread-safe: uses t.Setenv() which is scoped to the test.
//
// Usage:
//
//	setTestEnv(t, "AWF_LOG_LEVEL", "debug")
func setTestEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// createTestWorkflow writes a workflow YAML file to the test directory.
// Returns the full path to the created workflow file.
//
// Usage:
//
//	tmpDir := setupTestDir(t)
//	workflowPath := createTestWorkflow(t, tmpDir, "test.yaml", workflowContent)
func createTestWorkflow(t *testing.T, baseDir, filename, content string) string {
	t.Helper()

	workflowsDir := filepath.Join(baseDir, ".awf", "workflows")
	workflowPath := filepath.Join(workflowsDir, filename)

	if err := os.WriteFile(workflowPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	return workflowPath
}

// =============================================================================
// Helper Validation Tests
// =============================================================================

// TestSetupTestDir verifies the thread-safe directory setup helper.
// This test validates T002 acceptance criteria: setupTestDir uses t.TempDir().
func TestSetupTestDir(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Verify directory was created
	if tmpDir == "" {
		t.Fatal("setupTestDir returned empty path")
	}

	// Verify .awf/workflows structure exists
	workflowsDir := filepath.Join(tmpDir, ".awf", "workflows")
	if _, err := os.Stat(workflowsDir); os.IsNotExist(err) {
		t.Errorf("expected .awf/workflows directory to exist at %s", workflowsDir)
	}

	// Verify PWD environment variable was set
	pwd := os.Getenv("PWD")
	if pwd != tmpDir {
		t.Errorf("expected PWD=%s, got PWD=%s", tmpDir, pwd)
	}
}

// TestSetTestEnv verifies the thread-safe environment variable helper.
// This test validates T002 acceptance criteria: setTestEnv wraps t.Setenv().
func TestSetTestEnv(t *testing.T) {
	const testKey = "TEST_CLI_HELPER_VAR"
	const testValue = "test-value-123"

	setTestEnv(t, testKey, testValue)

	got := os.Getenv(testKey)
	if got != testValue {
		t.Errorf("expected %s=%s, got %s=%s", testKey, testValue, testKey, got)
	}
}

// TestCreateTestWorkflow verifies the workflow file creation helper.
// This test validates T002 acceptance criteria: createTestWorkflow writes files correctly.
func TestCreateTestWorkflow(t *testing.T) {
	tmpDir := setupTestDir(t)

	workflowContent := `name: test-workflow
version: "1.0.0"
states:
  initial: start
  start:
    type: terminal
`

	workflowPath := createTestWorkflow(t, tmpDir, "test.yaml", workflowContent)

	// Verify file exists
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		t.Fatalf("workflow file not created at %s", workflowPath)
	}

	// Verify content matches
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("failed to read workflow file: %v", err)
	}

	if string(content) != workflowContent {
		t.Errorf("workflow content mismatch:\nwant: %s\ngot:  %s", workflowContent, string(content))
	}
}
