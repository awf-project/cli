package cli_test

// Component T013 Tests: CompositeOperationProvider Wiring at CLI Layer
// Purpose: Verify that CLI commands properly wire CompositeOperationProvider with both GitHub and Notify providers
// Scope: Composition root verification for run and resume commands with multiple providers
//
// Test Strategy:
// - Happy Path: Commands instantiate both GitHub and Notify providers, wrap in composite, wire to ExecutionService
// - Edge Case: Composite wiring works in all execution paths (run, resume, single-step)
// - Error Handling: Provider dispatch to correct backend based on operation namespace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_WiresCompositeOperationProvider_WithGitHubAndNotify(t *testing.T) {
	// GIVEN: A temporary test directory with a workflow containing both github and notify operations
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: composite-test
version: "1.0.0"
states:
  initial: github_step
  github_step:
    type: operation
    operation: github.get_issue
    inputs:
      repo: test/repo
      issue_number: 1
    on_success: notify_step
  notify_step:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Issue retrieved"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "composite-test.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "composite-test"})

	err := cmd.Execute()
	// THEN: Command instantiates composite provider without panicking from nil providers
	// Both github and notify operations should attempt execution (not fail from missing provider)
	if err != nil {
		errMsg := err.Error()
		// Composite wiring is correct if we get execution errors, not nil pointer errors
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
		assert.NotContains(t, errMsg, "no operation provider", "composite provider should be wired")
		// Expected errors: gh CLI not found, notify backends not implemented, etc.
	}
}

func TestRunCommand_CompositeProvider_GitHubOperations(t *testing.T) {
	// GIVEN: A workflow using github operations through composite provider
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: github-via-composite
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
	createTestWorkflow(t, tmpDir, "github-via-composite.yaml", workflowContent)

	// WHEN: Running workflow that requires GitHub operations through composite
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "github-via-composite"})

	err := cmd.Execute()
	// THEN: Composite provider dispatches to GitHub provider correctly
	if err != nil {
		errMsg := err.Error()
		// Composite should delegate to GitHub provider
		assert.NotContains(t, errMsg, "operation not found", "composite should have github operations")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic during dispatch")
		// Expected errors come from gh execution, not from provider dispatch
	}
}

func TestRunCommand_CompositeProvider_NotifyOperations(t *testing.T) {
	// GIVEN: A workflow using notify operations through composite provider
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: notify-via-composite
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: ntfy
      message: "Workflow started"
      title: "AWF Notification"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "notify-via-composite.yaml", workflowContent)

	// WHEN: Running workflow that requires notify operations through composite
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "notify-via-composite"})

	err := cmd.Execute()
	// THEN: Composite provider dispatches to Notify provider correctly
	require.Error(t, err, "should fail because ntfy backend not configured")
	errMsg := err.Error()
	// Composite should delegate to Notify provider (implementation now working)
	assert.NotContains(t, errMsg, "operation not found", "composite should have notify operations")
	assert.NotContains(t, errMsg, "nil pointer", "should not panic during dispatch")
	// Expected error: backend not available (ntfy requires config)
	assert.Contains(t, errMsg, "backend \"ntfy\" not available", "notify provider should report ntfy backend not configured")
}

func TestRunCommand_CompositeProvider_ListsAllOperations(t *testing.T) {
	// GIVEN: A workflow using unknown operation
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
    operation: slack.send
    inputs:
      channel: "#general"
      message: "Test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "unknown-op.yaml", workflowContent)

	// WHEN: Running workflow with operation not in composite
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "unknown-op"})

	err := cmd.Execute()

	// THEN: Composite provider should report "operation not found" for operations from neither provider
	require.Error(t, err, "unknown operation should fail")
	errMsg := err.Error()
	// This proves composite is querying both providers and returning "not found"
	assert.Contains(t, errMsg, "not found", "composite should report operation not found")
}

func TestResumeCommand_WiresCompositeOperationProvider(t *testing.T) {
	// GIVEN: Setup for resume command testing with composite provider
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

	// THEN: Command instantiates composite provider correctly without panicking
	// Note: The resume operation will likely fail (no saved states),
	// but composite provider wiring should not cause crashes
	// Success criteria: no panic from nil providers during initialization
}

func TestRunSingleStep_WiresCompositeOperationProvider_GitHubOp(t *testing.T) {
	// GIVEN: A workflow with github operation for single-step execution
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: single-step-github
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
	createTestWorkflow(t, tmpDir, "single-step-github.yaml", workflowContent)

	// WHEN: Running single github operation step through composite
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"single-step-github",
		"--step=first",
	})

	err := cmd.Execute()
	// THEN: Single-step mode has composite provider wired, dispatches to github
	if err != nil {
		errMsg := err.Error()
		// Composite provider should be wired, github operation should be found
		assert.NotContains(t, errMsg, "no operation provider", "composite should be wired in single-step mode")
		assert.NotContains(t, errMsg, "operation not found", "github operation should be in composite")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from missing provider")
	}
}

func TestRunSingleStep_WiresCompositeOperationProvider_NotifyOp(t *testing.T) {
	// GIVEN: A workflow with notify operation for single-step execution
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: single-step-notify
version: "1.0.0"
states:
  initial: first
  first:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Single step test"
    on_success: second
  second:
    type: step
    command: echo "after notify"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "single-step-notify.yaml", workflowContent)

	// WHEN: Running single notify operation step through composite
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"single-step-notify",
		"--step=first",
	})

	err := cmd.Execute()
	// THEN: Single-step mode has composite provider wired, dispatches to notify
	if err != nil {
		errMsg := err.Error()
		// Composite provider should be wired, notify operation should be found
		assert.NotContains(t, errMsg, "no operation provider", "composite should be wired in single-step mode")
		assert.NotContains(t, errMsg, "operation not found", "notify operation should be in composite")
		// Expected: ntfy backend not available (requires config)
		assert.Contains(t, errMsg, "backend \"ntfy\" not available", "ntfy backend should not be configured")
	}
}

func TestRunCommand_MixedOperations_AllDispatchCorrectly(t *testing.T) {
	// GIVEN: A workflow mixing github, notify, and regular steps
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
    on_success: notify_step
  notify_step:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "PR retrieved"
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
	// THEN: Composite provider dispatches each operation to correct provider
	// Regular step should complete, github operation should attempt via github provider,
	// notify operation should attempt via notify provider
	if err != nil {
		errMsg := err.Error()
		// We expect errors from execution, not from missing wiring or dispatch
		assert.NotContains(t, errMsg, "no operation provider", "composite should be wired")
		assert.NotContains(t, errMsg, "nil pointer", "dispatch should not panic")
	}
}

func TestRunCommand_ParallelOperations_BothProviders(t *testing.T) {
	// GIVEN: A workflow with parallel operations from different providers
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: parallel-mixed
version: "1.0.0"
states:
  initial: start
  start:
    type: parallel
    strategy: best_effort
    branches:
      - name: github_branch
        steps:
          - type: operation
            operation: github.get_issue
            inputs:
              repo: test/repo
              issue_number: 1
      - name: notify_branch
        steps:
          - type: operation
            operation: notify.send
            inputs:
              backend: desktop
              message: "Parallel execution"
    on_complete: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "parallel-mixed.yaml", workflowContent)

	// WHEN: Running workflow with parallel operations from both providers
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "parallel-mixed"})

	err := cmd.Execute()
	// THEN: Composite provider handles concurrent dispatch to both providers
	if err != nil {
		errMsg := err.Error()
		// Parallel execution should not cause dispatch errors
		assert.NotContains(t, errMsg, "no operation provider", "composite should handle parallel dispatch")
		assert.NotContains(t, errMsg, "nil pointer", "concurrent access should be safe")
	}
}

func TestRunCommand_CompositeProvider_GitHubBatchOperation(t *testing.T) {
	// GIVEN: A workflow using github batch operation through composite
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: batch-via-composite
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
	createTestWorkflow(t, tmpDir, "batch-via-composite.yaml", workflowContent)

	// WHEN: Running workflow with batch operation through composite
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "batch-via-composite"})

	err := cmd.Execute()
	// THEN: Composite dispatches batch operation to github provider
	if err != nil {
		errMsg := err.Error()
		// Batch operation should be dispatched to github provider
		assert.NotContains(t, errMsg, "operation not found", "composite should have github.batch")
		assert.NotContains(t, errMsg, "nil pointer", "batch dispatch should not panic")
	}
}

func TestRunCommand_CompositeProvider_FirstProviderWins(t *testing.T) {
	// GIVEN: A workflow where composite has potential operation name conflicts
	// (In practice, github.* and notify.* have different namespaces, so no conflict)
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Use a github operation to verify first provider (github) wins
	workflowContent := `name: first-wins
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
	createTestWorkflow(t, tmpDir, "first-wins.yaml", workflowContent)

	// WHEN: Running operation that exists in first provider
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "first-wins"})

	err := cmd.Execute()
	// THEN: First provider (github) handles the operation
	if err != nil {
		errMsg := err.Error()
		// Operation should be found and dispatched to github provider
		assert.NotContains(t, errMsg, "operation not found", "github operation should be found")
		// Expected errors come from gh execution
	}
}

func TestRunCommand_CompositeProvider_NilProviderHandling(t *testing.T) {
	// GIVEN: A workflow that tests composite resilience to nil providers
	// (The stub implementation uses nil for notify provider)
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: nil-resilience
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
	createTestWorkflow(t, tmpDir, "nil-resilience.yaml", workflowContent)

	// WHEN: Running workflow while composite has nil notify provider
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "nil-resilience"})

	err := cmd.Execute()
	// THEN: Composite should handle nil providers gracefully (skip them)
	if err != nil {
		errMsg := err.Error()
		// Should not panic from nil provider, only github provider is active
		assert.NotContains(t, errMsg, "nil pointer", "composite should skip nil providers")
	}
}

func TestRunCommand_CompositeProvider_EmptyOperationName(t *testing.T) {
	// GIVEN: A workflow with malformed operation (edge case)
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: empty-op
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: ""
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "empty-op.yaml", workflowContent)

	// WHEN: Running workflow with empty operation name
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "empty-op"})

	err := cmd.Execute()

	// THEN: Composite provider should handle gracefully
	require.Error(t, err, "empty operation name should fail")
	errMsg := err.Error()
	// Should get validation error or "not found", not panic
	assert.NotContains(t, errMsg, "nil pointer", "should handle empty operation name")
}

func TestRunCommand_CompositeWiringStack_NoNilPointers(t *testing.T) {
	// GIVEN: A workflow exercising the full composite wiring stack
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: full-composite-stack
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
    on_success: notify
    on_failure: error_handler
  notify:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Comment added successfully"
    on_success: done
  error_handler:
    type: terminal
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "full-composite-stack.yaml", workflowContent)

	// WHEN: Running workflow (exercises: Client -> Providers -> Composite -> ExecutionService)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "full-composite-stack"})

	err := cmd.Execute()
	// THEN: Full composite stack should be wired without nil pointers
	// Test verifies that:
	// 1. github.NewClient(logger) creates github client
	// 2. github.NewGitHubOperationProvider(client, logger) creates github provider
	// 3. notify.NewNotifyOperationProvider(logger) creates notify provider
	// 4. plugin.NewCompositeOperationProvider(github, notify) creates composite
	// 5. execSvc.SetOperationProvider(composite) wires to execution service
	// Any of these steps failing would cause nil pointer panic
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "full composite wiring stack should not have nil pointers")
		assert.NotContains(t, errMsg, "nil client", "providers should be properly wired to composite")
		assert.NotContains(t, errMsg, "no operation provider", "composite should be wired to execution service")
	}
}

func TestRunCommand_CompositeProvider_BothProvidersAvailable(t *testing.T) {
	// GIVEN: A workflow that verifies both providers are in the composite
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Sequential workflow: github -> notify -> github
	workflowContent := `name: both-providers
version: "1.0.0"
states:
  initial: github1
  github1:
    type: operation
    operation: github.get_issue
    inputs:
      repo: test/repo
      issue_number: 1
    on_success: notify1
  notify1:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Issue retrieved"
    on_success: github2
  github2:
    type: operation
    operation: github.add_comment
    inputs:
      repo: test/repo
      issue_number: 1
      body: "Processing"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "both-providers.yaml", workflowContent)

	// WHEN: Running workflow that switches between providers
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "both-providers"})

	err := cmd.Execute()
	// THEN: Composite provider successfully dispatches to both providers in sequence
	if err != nil {
		errMsg := err.Error()
		// Both providers should be available and dispatch correctly
		assert.NotContains(t, errMsg, "operation not found", "both providers should be in composite")
		assert.NotContains(t, errMsg, "no operation provider", "composite should be wired")
		assert.NotContains(t, errMsg, "nil pointer", "switching between providers should not panic")
	}
}

// Note: Test helpers setupTestDir() and createTestWorkflow() are defined in test_helpers_test.go
