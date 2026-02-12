package cli_test

// Component T014 Tests: GitHub Operation Provider Wiring at CLI Layer
// Purpose: Verify that CLI commands properly wire GitHubOperationProvider to ExecutionService
// Scope: Composition root verification for run and resume commands
//
// Test Strategy:
// - Happy Path: Commands instantiate GitHub provider and wire to ExecutionService
// - Edge Case: GitHub provider is wired in all execution paths (run, resume, single-step)
// - Error Handling: Missing gh CLI causes graceful fallback behavior

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// Happy Path Tests
// =============================================================================

func TestRunCommand_WiresGitHubOperationProvider(t *testing.T) {
	// GIVEN: A temporary test directory with a workflow containing github operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: github-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.get_issue
    inputs:
      repo: test/repo
      issue_number: 1
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "github-test.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "github-test"})

	err := cmd.Execute()
	// THEN: Command instantiates without panicking from nil provider
	// Note: The actual operation will fail (no gh CLI in test environment),
	// but the wiring should be correct and not cause nil pointer panics
	// The error should be from execution, not from missing provider
	if err != nil {
		errMsg := err.Error()
		// Provider wiring is correct if we get execution errors, not nil pointer errors
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired")
		// Expected errors: gh CLI not found, auth error, network error, etc.
	}
}

func TestRunCommand_WiresGitHubClientToProvider(t *testing.T) {
	// GIVEN: A workflow with github operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: client-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.create_pr
    inputs:
      title: "Test PR"
      base: "main"
      head: "feature"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "client-test.yaml", workflowContent)

	// WHEN: Running workflow that requires GitHub client
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "client-test"})

	err := cmd.Execute()
	// THEN: Client is properly wired to provider (no nil pointer panics)
	if err != nil {
		errMsg := err.Error()
		// Wiring is correct if errors come from client execution, not initialization
		assert.NotContains(t, errMsg, "nil client", "client should be wired to provider")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic during initialization")
	}
}

func TestRunCommand_GitHubProviderRegistersOperations(t *testing.T) {
	// GIVEN: A workflow using unknown github operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: unknown-op
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.unknown_operation
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "unknown-op.yaml", workflowContent)

	// WHEN: Running workflow with unknown operation
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "unknown-op"})

	err := cmd.Execute()

	// THEN: Provider should report "operation not found" (proving it's wired and queried)
	require.Error(t, err, "unknown operation should fail")
	errMsg := err.Error()
	// This proves the provider was wired, registered, and queried
	assert.Contains(t, errMsg, "not found", "provider should report operation not found")
}

func TestResumeCommand_WiresGitHubOperationProvider(t *testing.T) {
	// GIVEN: Setup for resume command testing
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// WHEN: Calling resume without workflow name (triggers list mode)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "resume"})

	_ = cmd.Execute()

	// THEN: Command instantiates services correctly without panicking
	// Note: The resume operation will likely fail (no saved states),
	// but GitHub provider wiring should not cause crashes
	// Success criteria: no panic from nil provider during initialization
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestRunSingleStep_WiresGitHubOperationProvider(t *testing.T) {
	// GIVEN: A workflow with github operation for single-step execution
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: single-step
version: "1.0.0"
states:
  initial: first
  first:
    type: operation
    operation: github.get_issue
    inputs:
      repo: test/repo
      issue_number: 1
    on_success: second
  second:
    type: step
    command: echo "after github"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "single-step.yaml", workflowContent)

	// WHEN: Running single step (different code path in run.go)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"single-step",
		"--step=first",
	})

	err := cmd.Execute()
	// THEN: Single-step mode has GitHub provider wired
	if err != nil {
		errMsg := err.Error()
		// Provider should be wired, errors should come from execution
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired in single-step mode")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from missing provider")
	}
}

func TestRunCommand_MultipleOperationTypes_AllWired(t *testing.T) {
	// GIVEN: A workflow mixing github operations with regular steps
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: mixed-ops
version: "1.0.0"
states:
  initial: regular
  regular:
    type: step
    command: echo "regular step"
    on_success: github_step
  github_step:
    type: operation
    operation: github.get_pr
    inputs:
      repo: test/repo
      pr_number: 1
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mixed-ops.yaml", workflowContent)

	// WHEN: Running workflow with mixed operation types
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "mixed-ops"})

	err := cmd.Execute()
	// THEN: Both regular steps and github operations should be supported
	// Regular step should complete, github operation should attempt execution
	// (not fail from missing provider)
	if err != nil {
		errMsg := err.Error()
		// We expect errors from gh execution, not from missing wiring
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired for operation steps")
	}
}

func TestRunCommand_BatchOperationWiring(t *testing.T) {
	// GIVEN: A workflow using github batch operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: batch-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.batch
    inputs:
      operations:
        - type: add_labels
          issue: 1
          labels: ["bug"]
        - type: add_labels
          issue: 2
          labels: ["feature"]
      strategy: best_effort
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "batch-test.yaml", workflowContent)

	// WHEN: Running workflow with batch operation
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "batch-test"})

	err := cmd.Execute()
	// THEN: Batch operation handler should be wired and invoked
	if err != nil {
		errMsg := err.Error()
		// Provider should handle batch operations
		assert.NotContains(t, errMsg, "no operation provider", "batch operations require wired provider")
		assert.NotContains(t, errMsg, "nil pointer", "batch execution should not panic")
	}
}

func TestRunCommand_ProjectStatusOperationWiring(t *testing.T) {
	// GIVEN: A workflow using github.set_project_status operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: project-status
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.set_project_status
    inputs:
      issue: 1
      project: "Project Board"
      field: "Status"
      value: "In Progress"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "project-status.yaml", workflowContent)

	// WHEN: Running workflow with project status operation
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "project-status"})

	err := cmd.Execute()
	// THEN: Project status operation should be wired
	if err != nil {
		errMsg := err.Error()
		// Operation should be registered and invoked
		assert.NotContains(t, errMsg, "no operation provider", "project status operation needs provider")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestRunCommand_MissingGitHubAuth_GracefulError(t *testing.T) {
	// GIVEN: A workflow requiring GitHub authentication
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Unset GitHub auth environment variables
	oldToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if oldToken != "" {
			_ = os.Setenv("GITHUB_TOKEN", oldToken)
		}
	}()
	_ = os.Unsetenv("GITHUB_TOKEN")

	workflowContent := `name: auth-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.create_issue
    inputs:
      repo: test/repo
      title: "Test Issue"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "auth-test.yaml", workflowContent)

	// WHEN: Running workflow without authentication
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "auth-test"})

	err := cmd.Execute()

	// THEN: Workflow should fail with error from gh CLI
	// The GitHub provider now executes real gh commands which fail without proper auth/inputs
	require.Error(t, err, "expected error from gh CLI execution")
	output := out.String()
	// Verify error indicates gh CLI execution failure (missing --title and --body when non-interactive)
	assert.Contains(t, output, "gh issue create")
}

func TestRunCommand_InvalidOperationInputs_GracefulError(t *testing.T) {
	// GIVEN: A workflow with invalid github operation inputs
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: invalid-inputs
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.get_issue
    inputs:
      # Missing required 'issue_number' input
      repo: test/repo
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "invalid-inputs.yaml", workflowContent)

	// WHEN: Running workflow with invalid inputs
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "invalid-inputs"})

	err := cmd.Execute()

	// THEN: Workflow should fail with validation error for missing required input
	// The GitHub provider now validates inputs and returns descriptive errors
	require.Error(t, err, "expected validation error for missing issue_number")
	output := out.String()
	// Verify error indicates missing required parameter
	assert.Contains(t, output, "number is required")
}

func TestRunCommand_NetworkError_GracefulFallback(t *testing.T) {
	// GIVEN: A workflow that would trigger network call
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: network-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.list_comments
    inputs:
      repo: test/repo
      issue_number: 999999
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "network-test.yaml", workflowContent)

	// WHEN: Running workflow that would make network call
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "network-test"})

	err := cmd.Execute()
	// THEN: Should fail gracefully (not panic), provider handles errors
	if err != nil {
		errMsg := err.Error()
		// Provider should handle network errors gracefully
		assert.NotContains(t, errMsg, "nil pointer", "should not panic on network errors")
		// Expected: gh CLI error, network error, auth error, etc.
	}
}

// =============================================================================
// Integration Tests: Provider + Client + Logger Wiring
// =============================================================================

func TestRunCommand_FullWiringStack_NoNilPointers(t *testing.T) {
	// GIVEN: A workflow exercising the full wiring stack
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: full-stack
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: github.add_comment
    inputs:
      repo: test/repo
      issue_number: 1
      body: "Test comment from AWF"
    on_success: done
    on_failure: error_handler
  error_handler:
    type: terminal
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "full-stack.yaml", workflowContent)

	// WHEN: Running workflow (exercises: Client -> Provider -> ExecutionService)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "full-stack"})

	err := cmd.Execute()
	// THEN: Full stack should be wired without nil pointers
	// Test verifies that:
	// 1. github.NewClient(logger) creates client
	// 2. github.NewGitHubOperationProvider(client, logger) creates provider
	// 3. execSvc.SetOperationProvider(provider) wires to execution service
	// Any of these steps failing would cause nil pointer panic
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "full wiring stack should not have nil pointers")
		assert.NotContains(t, errMsg, "nil client", "client should be properly wired")
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired to execution service")
	}
}

// Note: Test helpers setupTestDir() and createTestWorkflow() are defined in cli_test_helpers_test.go
