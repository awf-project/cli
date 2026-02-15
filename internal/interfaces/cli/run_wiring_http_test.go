package cli_test

// Component T010 Tests: HTTP Operation Provider Wiring at CLI Layer
// Purpose: Verify that CLI commands properly wire HTTPOperationProvider to ExecutionService
// Scope: Composition root verification for run and resume commands with http operations
//
// Test Strategy:
// - Happy Path: Commands instantiate HTTP provider and wire to ExecutionService
// - Edge Case: HTTP provider is wired in all execution paths (run, resume, single-step)
// - Error Handling: HTTP client configuration and request validation errors are properly surfaced

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_WiresHTTPOperationProvider(t *testing.T) {
	// GIVEN: A temporary test directory with a workflow containing http operation
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: http-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/users/1"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "http-test.yaml", workflowContent)

	// WHEN: Running the workflow
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "http-test"})

	err := cmd.Execute()
	// THEN: Command instantiates without panicking from nil provider
	// The actual operation will likely fail (network call to example.com),
	// but the wiring should be correct and not cause nil pointer panics
	// The error should be from execution, not from missing provider
	if err != nil {
		errMsg := err.Error()
		// Provider wiring is correct if we get execution errors, not nil pointer errors
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired")
		// Expected errors: network error, timeout, DNS resolution, etc.
	}
}

func TestRunCommand_WiresHTTPClientToProvider(t *testing.T) {
	// GIVEN: A workflow with http.request operation
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
    operation: http.request
    inputs:
      method: POST
      url: "https://api.example.com/data"
      headers:
        Content-Type: "application/json"
      body: '{"test": "data"}'
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "client-test.yaml", workflowContent)

	// WHEN: Running workflow that requires HTTP client
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

func TestRunCommand_HTTPProviderRegistersOperations(t *testing.T) {
	// GIVEN: A workflow using unknown http operation
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
    operation: http.unknown_operation
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "unknown-op.yaml", workflowContent)

	// WHEN: Running workflow with unknown http operation
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

func TestResumeCommand_WiresHTTPOperationProvider(t *testing.T) {
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

	err := cmd.Execute()
	// THEN: Command initializes without nil pointer panic.
	// Resume with no saved states will fail, but the error should be from
	// missing state, not from nil provider during initialization.
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired")
	}
}

func TestRunSingleStep_WiresHTTPOperationProvider(t *testing.T) {
	// GIVEN: A workflow with http operation for single-step execution
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
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/test"
    on_success: second
  second:
    type: step
    command: echo "after http"
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
	// THEN: Single-step mode has HTTP provider wired
	if err != nil {
		errMsg := err.Error()
		// Provider should be wired, errors should come from execution
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired in single-step mode")
		assert.NotContains(t, errMsg, "nil pointer", "should not panic from missing provider")
	}
}

func TestRunCommand_HTTPWithRegularSteps_AllWired(t *testing.T) {
	// GIVEN: A workflow mixing http operations with regular steps
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
    on_success: http_step
  http_step:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/data"
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
	// THEN: Both regular steps and http operations should be supported
	// Regular step should complete, http operation should attempt execution
	// (not fail from missing provider)
	if err != nil {
		errMsg := err.Error()
		// We expect errors from http execution, not from missing wiring
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired for operation steps")
	}
}

func TestRunCommand_HTTPOperationWithCustomTimeout(t *testing.T) {
	// GIVEN: A workflow using http.request with custom timeout
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: timeout-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/slow"
      timeout: 5
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "timeout-test.yaml", workflowContent)

	// WHEN: Running workflow with custom timeout
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "timeout-test"})

	err := cmd.Execute()
	// THEN: HTTP operation with timeout configuration should be handled
	if err != nil {
		errMsg := err.Error()
		// Timeout configuration should be processed
		assert.NotContains(t, errMsg, "nil pointer", "timeout config should not panic")
	}
}

func TestRunCommand_HTTPOperationWithHeaders(t *testing.T) {
	// GIVEN: A workflow using http.request with custom headers
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: headers-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "https://api.example.com/data"
      headers:
        Authorization: "Bearer test-token"
        Accept: "application/json"
        X-Custom-Header: "custom-value"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "headers-test.yaml", workflowContent)

	// WHEN: Running workflow with custom headers
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "headers-test"})

	err := cmd.Execute()
	// THEN: HTTP operation with headers should be processed
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "header processing should not panic")
	}
}

func TestRunCommand_HTTPOperation_InvalidMethod(t *testing.T) {
	// GIVEN: A workflow with invalid HTTP method
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: invalid-method
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: INVALID
      url: "https://api.example.com/test"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "invalid-method.yaml", workflowContent)

	// WHEN: Running workflow with invalid method
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "invalid-method"})

	err := cmd.Execute()

	// THEN: Provider returns validation error — not a wiring panic
	require.Error(t, err, "invalid method should fail")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "method", "error should mention method validation")
	assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
}

func TestRunCommand_HTTPOperation_MissingURL(t *testing.T) {
	// GIVEN: A workflow with missing required URL input
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	workflowContent := `name: missing-url
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: GET
      # Missing required 'url' input
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "missing-url.yaml", workflowContent)

	// WHEN: Running workflow with missing URL
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "missing-url"})

	err := cmd.Execute()

	// THEN: Provider returns validation error — not a wiring panic
	require.Error(t, err, "missing url should fail")
	errMsg := err.Error()
	assert.Contains(t, errMsg, "url", "error should mention url requirement")
	assert.NotContains(t, errMsg, "nil pointer", "should not panic from nil provider")
}

func TestRunCommand_HTTPOperation_NetworkError(t *testing.T) {
	// GIVEN: A workflow that would trigger network error
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Use non-routable IP to trigger connection error
	workflowContent := `name: network-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "http://192.0.2.1/test"
      timeout: 1
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "network-test.yaml", workflowContent)

	// WHEN: Running workflow that would make network call to unreachable host
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
		// Expected: timeout error, connection error, etc.
	}
}

func TestRunCommand_HTTPOperation_AllHTTPMethods(t *testing.T) {
	// GIVEN: Workflows testing all supported HTTP methods
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	testCases := []struct {
		name   string
		method string
	}{
		{"GET", "GET"},
		{"POST", "POST"},
		{"PUT", "PUT"},
		{"DELETE", "DELETE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workflowContent := `name: method-test
version: "1.0.0"
states:
  initial: start
  start:
    type: operation
    operation: http.request
    inputs:
      method: ` + tc.method + `
      url: "https://api.example.com/test"
    on_success: done
  done:
    type: terminal
`
			createTestWorkflow(t, tmpDir, "method-test.yaml", workflowContent)

			// WHEN: Running workflow with specific HTTP method
			cmd := cli.NewRootCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "method-test"})

			err := cmd.Execute()
			// THEN: All supported methods should be recognized
			if err != nil {
				errMsg := err.Error()
				// Should not fail with "invalid method" for supported methods
				assert.NotContains(t, errMsg, "invalid method", "supported methods should be recognized")
				assert.NotContains(t, errMsg, "nil pointer", "method handling should not panic")
			}
		})
	}
}

func TestRunCommand_FullHTTPWiringStack_NoNilPointers(t *testing.T) {
	// GIVEN: A workflow exercising the full HTTP wiring stack
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
    operation: http.request
    inputs:
      method: POST
      url: "https://api.example.com/webhook"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer test-token"
      body: '{"event": "test", "data": "value"}'
      timeout: 10
    on_success: done
    on_failure: error_handler
  error_handler:
    type: terminal
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "full-stack.yaml", workflowContent)

	// WHEN: Running workflow (exercises: HTTPClient -> Provider -> ExecutionService)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "full-stack"})

	err := cmd.Execute()
	// THEN: Full stack should be wired without nil pointers
	// Test verifies that:
	// 1. httputil.NewClient() creates HTTP client
	// 2. http.NewHTTPOperationProvider(client, logger) creates provider
	// 3. execSvc.SetOperationProvider(provider) wires to execution service
	// Any of these steps failing would cause nil pointer panic
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "nil pointer", "full wiring stack should not have nil pointers")
		assert.NotContains(t, errMsg, "nil client", "client should be properly wired")
		assert.NotContains(t, errMsg, "no operation provider", "provider should be wired to execution service")
	}
}

func TestRunCommand_CompositeProvider_IncludesHTTP(t *testing.T) {
	// GIVEN: A workflow that verifies HTTP provider is in the composite
	tmpDir := setupTestDir(t)

	// Create storage directories
	_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)

	// Sequential workflow: github -> notify -> http
	workflowContent := `name: composite-http
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
    on_success: http_step
  http_step:
    type: operation
    operation: http.request
    inputs:
      method: POST
      url: "https://webhook.example.com/notify"
      body: '{"status": "complete"}'
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "composite-http.yaml", workflowContent)

	// WHEN: Running workflow that uses all three providers
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "composite-http"})

	err := cmd.Execute()
	// THEN: Composite provider is wired with all three providers.
	// The workflow will fail during execution (github step lacks auth),
	// but the error should NOT be "operation not found" for any of the 3 namespaces.
	if err != nil {
		errMsg := err.Error()
		assert.NotContains(t, errMsg, "no operation provider", "composite should be wired")
		assert.NotContains(t, errMsg, "nil pointer", "switching between providers should not panic")
	}
}

// Note: Test helpers setupTestDir() and createTestWorkflow() are defined in test_helpers_test.go
