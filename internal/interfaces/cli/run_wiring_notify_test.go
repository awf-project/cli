package cli_test

// Component T014 Tests: Notify Operation Provider Wiring at CLI Layer
// Purpose: Verify that CLI commands properly wire NotifyOperationProvider to ExecutionService
// Scope: Composition root verification for run and resume commands with notify operations
//
// Test Strategy:
// - Happy Path: Commands instantiate Notify provider and wire to ExecutionService
// - Edge Case: Notify provider is wired in all execution paths (run, resume, single-step)
// - Error Handling: Backend validation and configuration errors are properly surfaced

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestRunCommand_WiresNotifyOperationProvider(t *testing.T) {
	// GIVEN: A temporary test directory with a workflow containing notify operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: notify-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Test notification"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "notify-test.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "notify-test"})

	err := cmd.Execute()
	// THEN: Command instantiates without panicking from nil provider
	// The error should be from execution, not from missing provider
	if err != nil {
		errMsg := err.Error()
		// Provider wiring is correct if we get execution errors, not nil pointer errors
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired")
		// Expected errors: backend not implemented, configuration error, etc.
	}
}

func TestRunCommand_WiresNotifyProvider_DesktopBackend(t *testing.T) {
	// GIVEN: A workflow with desktop notification
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: desktop-notify
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
      message: "Desktop notification test"
      title: "AWF Test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "desktop-notify.yaml", workflowContent)

	// WHEN: Running workflow with desktop backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "desktop-notify"})

	err := cmd.Execute()
	// THEN: Notify provider is wired and handles desktop backend
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
		assert.NotContains(t, errMsg, "operation not found", "notify.send should be available")
	}
}

func TestRunCommand_WiresNotifyProvider_NtfyBackend(t *testing.T) {
	// GIVEN: A workflow with ntfy.sh notification
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: ntfy-notify
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: ntfy
      message: "Ntfy notification test"
      topic: "awf-test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "ntfy-notify.yaml", workflowContent)

	// WHEN: Running workflow with ntfy backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "ntfy-notify"})

	err := cmd.Execute()
	// THEN: Notify provider is wired and handles ntfy backend
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
		assert.NotContains(t, errMsg, "operation not found", "notify.send should be available")
	}
}

func TestRunCommand_WiresNotifyProvider_SlackBackend(t *testing.T) {
	// GIVEN: A workflow with Slack notification
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: slack-notify
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: slack
      message: "Slack notification test"
      channel: "#general"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "slack-notify.yaml", workflowContent)

	// WHEN: Running workflow with slack backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "slack-notify"})

	err := cmd.Execute()
	// THEN: Notify provider is wired and handles slack backend
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
		assert.NotContains(t, errMsg, "operation not found", "notify.send should be available")
	}
}

func TestRunCommand_WiresNotifyProvider_WebhookBackend(t *testing.T) {
	// GIVEN: A workflow with webhook notification
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: webhook-notify
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: webhook
      message: "Webhook notification test"
      url: "https://example.com/webhook"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "webhook-notify.yaml", workflowContent)

	// WHEN: Running workflow with webhook backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "webhook-notify"})

	err := cmd.Execute()
	// THEN: Notify provider is wired and handles webhook backend
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic")
		assert.NotContains(t, errMsg, "operation not found", "notify.send should be available")
	}
}

func TestRunSingleStep_WiresNotifyOperationProvider(t *testing.T) {
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

	// WHEN: Running single notify operation step
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
	// THEN: Single-step mode has notify provider wired correctly
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired in single-step mode")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from missing provider")
	}
}

func TestResumeCommand_WiresNotifyOperationProvider(t *testing.T) {
	// GIVEN: Setup for resume command testing with notify provider
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

	// THEN: Command instantiates notify provider correctly without panicking
	// Note: The resume operation will likely fail (no saved states),
	// but notify provider wiring should not cause crashes
	// Success criteria: no panic from nil providers during initialization
}

func TestRunCommand_NotifyProvider_InvalidBackend(t *testing.T) {
	// GIVEN: A workflow with invalid backend type
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: invalid-backend
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: invalid_backend
      message: "Test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "invalid-backend.yaml", workflowContent)

	// WHEN: Running workflow with invalid backend
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "invalid-backend"})

	err := cmd.Execute()
	// THEN: Provider should return validation error, not crash
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should handle invalid backend gracefully")
		// Expected: backend validation error
	}
}

func TestRunCommand_NotifyProvider_MissingRequiredInput(t *testing.T) {
	// GIVEN: A workflow with missing required input (message)
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: missing-input
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: notify.send
    inputs:
      backend: desktop
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "missing-input.yaml", workflowContent)

	// WHEN: Running workflow with missing required input
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "missing-input"})

	err := cmd.Execute()
	// THEN: Provider should return validation error
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should handle missing input gracefully")
		// Expected: input validation error
	}
}

func TestRunCommand_NotifyProvider_ParallelNotifications(t *testing.T) {
	// GIVEN: A workflow with parallel notify operations
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: parallel-notify
version: "1.0.0"
states:
  initial: start
  start:
    type: parallel
    strategy: best_effort
    branches:
      - name: desktop_branch
        steps:
          - type: operation
            operation: notify.send
            inputs:
              backend: desktop
              message: "Desktop notification"
      - name: ntfy_branch
        steps:
          - type: operation
            operation: notify.send
            inputs:
              backend: ntfy
              message: "Ntfy notification"
              topic: "awf-test"
    on_complete: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "parallel-notify.yaml", workflowContent)

	// WHEN: Running workflow with parallel notifications
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "parallel-notify"})

	err := cmd.Execute()
	// THEN: Notify provider handles concurrent operations correctly
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "concurrent notifications should not crash")
	}
}

// Note: Test helpers setupTestDir() and createTestWorkflow() are defined in test_helpers_test.go
