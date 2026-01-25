package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file contains CLI-specific test fixtures and helper functions for CLI tests.
// Extracted from duplicate helpers across CLI test files as part of C017 CLI Test Reorganization.
//
// All fixtures:
// - Are thread-safe (use t.TempDir(), t.Setenv())
// - Follow existing testutil conventions
// - Reduce duplication across CLI test files
//
// Usage:
//
//	dir := testutil.SetupTestDir(t)                             // Create test environment
//	testutil.CreateTestWorkflow(t, dir, "test", testutil.SimpleWorkflowYAML)

// CLI Test YAML Workflow Constants
//
// These YAML workflow strings are used across multiple CLI test files
// for integration testing. Previously duplicated in:
// - internal/interfaces/cli/validate_coverage_test.go
// - internal/interfaces/cli/list_coverage_test.go
// - internal/interfaces/cli/config_cmd_test.go
// - internal/interfaces/cli/history_test.go

// SimpleWorkflowYAML is a minimal valid workflow with single command step and terminal.
// Used for basic CLI command testing.
const SimpleWorkflowYAML = `name: test
states:
  initial: start
  start:
    type: step
    command: echo "test"
    on_success: done
  done:
    type: terminal
`

// FullWorkflowYAML is a complete workflow with version, description, inputs, and validation.
// Used for testing full workflow parsing and input handling.
const FullWorkflowYAML = `name: test-full
version: "1.0.0"
description: Test workflow
inputs:
  - name: var1
    type: string
    required: true
  - name: var2
    type: integer
    default: 5
states:
  initial: step1
  step1:
    type: step
    command: echo "{{inputs.var1}}"
    on_success: done
  done:
    type: terminal
`

// BadWorkflowYAML is an invalid workflow with nonexistent state reference.
// Used for testing validation error handling.
const BadWorkflowYAML = `name: bad
states:
  initial: start
  start:
    type: step
    command: echo "test"
    on_success: nonexistent
  done:
    type: terminal
`

// CLI Test Helper Functions

// SetupTestDir creates an isolated test directory with .awf structure for CLI tests.
// Replaces the os.Chdir pattern with thread-safe t.TempDir() + directory structure creation.
//
// Returns the path to the temporary directory.
//
// The created directory structure:
//   - <tmpDir>/.awf/           # AWF project root marker
//   - <tmpDir>/.awf/workflows/ # Workflow storage directory
//   - <tmpDir>/.awf/storage/   # State storage directory
//   - <tmpDir>/.awf/prompts/   # Prompt storage directory
//
// Usage:
//
//	dir := SetupTestDir(t)
//	// dir now contains .awf structure
//	// Use dir for --storage flag or file operations
//
// Thread-safety:
// - Uses t.TempDir() for automatic cleanup
// - No os.Chdir (process-wide state)
// - Safe for parallel test execution
func SetupTestDir(t *testing.T) string {
	t.Helper()

	// Create temporary directory with automatic cleanup
	dir := t.TempDir()

	// Create .awf directory structure
	awfDir := filepath.Join(dir, ".awf")
	if err := os.MkdirAll(awfDir, 0o750); err != nil {
		t.Fatalf("Failed to create .awf directory: %v", err)
	}

	// Create subdirectories
	subdirs := []string{"workflows", "storage", "prompts"}
	for _, subdir := range subdirs {
		path := filepath.Join(awfDir, subdir)
		if err := os.MkdirAll(path, 0o750); err != nil {
			t.Fatalf("Failed to create %s directory: %v", subdir, err)
		}
	}

	return dir
}

// CreateTestWorkflow writes a YAML workflow file to the test directory.
// Helper for setting up test workflows in setupTestDir() environments.
//
// The name parameter should be a workflow filename. If it contains path separators,
// they will be replaced with hyphens to create a flat file structure.
// This ensures consistent behavior across tests and avoids nested directory complexity.
//
// Parameters:
//   - t: Testing context
//   - dir: Base directory (typically from SetupTestDir)
//   - name: Workflow name (path separators replaced with hyphens)
//   - content: YAML workflow content
//
// Usage:
//
//	dir := SetupTestDir(t)
//	CreateTestWorkflow(t, dir, "test.yaml", SimpleWorkflowYAML)
//	CreateTestWorkflow(t, dir, "bad.yaml", BadWorkflowYAML)
//	CreateTestWorkflow(t, dir, "sub/dir/wf.yaml", SimpleWorkflowYAML)  // Creates "sub-dir-wf.yaml"
func CreateTestWorkflow(t *testing.T, dir, name, content string) {
	t.Helper()

	// Flatten path separators to avoid creating nested directories
	// This ensures all workflows are created at .awf/workflows/ level
	// "workflow/a/.yaml" becomes "workflow-a-.yaml"
	safeName := strings.ReplaceAll(name, string(filepath.Separator), "-")
	workflowPath := filepath.Join(dir, ".awf", "workflows", safeName)

	// Write workflow file
	if err := os.WriteFile(workflowPath, []byte(content), 0o600); err != nil {
		t.Fatalf("Failed to create workflow file %s: %v", safeName, err)
	}
}

// SetupWorkflowsDir creates test directory with multiple workflows from a map.
// Convenience wrapper around SetupTestDir + multiple CreateTestWorkflow calls.
//
// Parameters:
//   - t: Testing context
//   - workflows: Map of workflow name -> YAML content
//
// Returns the test directory path.
//
// Usage:
//
//	dir := SetupWorkflowsDir(t, map[string]string{
//	    "test": SimpleWorkflowYAML,
//	    "full": FullWorkflowYAML,
//	})
//	// dir/.awf/workflows/test.yaml and full.yaml created
func SetupWorkflowsDir(t *testing.T, workflows map[string]string) string {
	t.Helper()

	// Create test directory
	dir := SetupTestDir(t)

	// Create each workflow file
	for name, content := range workflows {
		CreateTestWorkflow(t, dir, name, content)
	}

	return dir
}
