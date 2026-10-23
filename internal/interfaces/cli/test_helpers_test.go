package cli_test

// Thread-safe test helpers for CLI test suite (C015 T002)
// Replaces os.Chdir with t.TempDir() and t.Setenv() patterns to eliminate race conditions

import (
	"os"
	"path/filepath"
	"testing"
)

// setupProjectConfig creates and selects a project-local config file for tests
// that require configuration. setupTestDir deliberately does not call it so
// missing-config scenarios can keep AWF_CONFIG_PATH unset.
func setupProjectConfig(t *testing.T, tmpDir, content string) string {
	t.Helper()

	configDir := filepath.Join(tmpDir, ".awf")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create project config directory: %v", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	// WriteFile applies the process umask. Chmod makes the helper's advertised
	// mode deterministic for tests that inspect the resulting fixture.
	if err := os.Chmod(configPath, 0o644); err != nil {
		t.Fatalf("set project config permissions: %v", err)
	}
	t.Setenv("AWF_CONFIG_PATH", configPath)

	return configPath
}

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
	t.Setenv("PWD", tmpDir)

	// Set AWF_WORKFLOWS_PATH to point to the test workflows directory
	// This ensures the CLI loads workflows from the test directory, not the project root
	t.Setenv("AWF_WORKFLOWS_PATH", workflowsDir)

	// Set AWF_PROMPTS_PATH to point to the test prompts directory
	// This ensures the CLI loads prompts from the test directory, not the project root
	t.Setenv("AWF_PROMPTS_PATH", promptsDir)

	return tmpDir
}

// setupInitTestDir creates an isolated test directory for init command tests.
// Unlike setupTestDir, this does NOT create the .awf structure, allowing init to create it from scratch.
// Returns the temporary directory path.
//
// Usage:
//
//	tmpDir := setupInitTestDir(t)
//	// Run init command
func setupInitTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Change to test directory for init command
	// Note: init command creates .awf relative to current working directory
	// This is a controlled use of os.Chdir for tests that specifically need it
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to test directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	return tmpDir
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

// TestSetupTestDir verifies the thread-safe directory setup helper.
// This test validates T002 acceptance criteria: setupTestDir uses t.TempDir().
func TestSetupTestDir(t *testing.T) {
	originalConfigPath, configPathWasSet := os.LookupEnv("AWF_CONFIG_PATH")
	if err := os.Unsetenv("AWF_CONFIG_PATH"); err != nil {
		t.Fatalf("failed to unset AWF_CONFIG_PATH: %v", err)
	}
	t.Cleanup(func() {
		if configPathWasSet {
			_ = os.Setenv("AWF_CONFIG_PATH", originalConfigPath)
			return
		}
		_ = os.Unsetenv("AWF_CONFIG_PATH")
	})

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

	if _, exists := os.LookupEnv("AWF_CONFIG_PATH"); exists {
		t.Error("setupTestDir must leave AWF_CONFIG_PATH unset")
	}

	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		info, err := os.Stat(xdgDataHome) //nolint:gosec // TestMain creates and owns this isolated test directory.
		if err != nil {
			t.Fatalf("package XDG data root is unavailable: %v", err)
		}
		if !info.IsDir() {
			t.Fatalf("package XDG data root is not a directory: %s", xdgDataHome)
		}
	}

	configContent := "log:\n  level: debug\n"
	configPath := setupProjectConfig(t, tmpDir, configContent)
	wantConfigPath := filepath.Join(tmpDir, ".awf", "config.yaml")
	if configPath != wantConfigPath {
		t.Fatalf("setupProjectConfig path = %q, want %q", configPath, wantConfigPath)
	}
	if got := os.Getenv("AWF_CONFIG_PATH"); got != wantConfigPath {
		t.Errorf("AWF_CONFIG_PATH = %q, want %q", got, wantConfigPath)
	}
	configInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("project config is unavailable: %v", err)
	}
	if got := configInfo.Mode().Perm(); got != 0o644 {
		t.Errorf("project config mode = %o, want 644", got)
	}
	gotContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read project config: %v", err)
	}
	if string(gotContent) != configContent {
		t.Errorf("project config content = %q, want %q", gotContent, configContent)
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
