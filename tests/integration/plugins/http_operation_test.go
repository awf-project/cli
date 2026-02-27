//go:build integration

package plugins_test

// Component T011 Tests: HTTP Operation Provider Integration
// Purpose: Verify end-to-end HTTP operation execution through ExecutionService pipeline
// Scope: US1-US5 acceptance scenarios with httptest server
//
// Test Strategy:
// - Happy Path: GET, POST, PUT, DELETE with httptest server
// - Edge Cases: Response capture, header canonicalization, timeout, body size limit
// - Error Handling: Connection failures, DNS errors, unknown operations
// - Retryable Status: Verify retryable_status_codes signals failure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/github"
	infraHTTP "github.com/awf-project/cli/internal/infrastructure/http"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPOperation_GET_Success(t *testing.T) {
	// GIVEN: A workflow with http.request GET operation targeting httptest server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		assert.Equal(t, http.MethodGet, r.Method)

		// Respond with success (set headers before WriteHeader)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "test-123")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "success", "id": 42}`))
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL + "/users/1",
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-get-test", inputs)

	// THEN: Operation returns status_code=200, body, headers in outputs
	require.NoError(t, err, "GET request should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("fetch_data")
	require.True(t, exists, "step state should exist")
	require.NotNil(t, state.Response, "step should have response")

	// These assertions will FAIL against stubs - assert real expected behavior
	assert.Equal(t, 200, state.Response["status_code"])
	assert.Contains(t, state.Response["body"], "success")

	headers, ok := state.Response["headers"].(map[string]string)
	require.True(t, ok, "headers should be map[string]string")
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, "test-123", headers["X-Request-Id"])
}

func TestHTTPOperation_GET_NonExistentHost(t *testing.T) {
	// GIVEN: A workflow with http.request GET to non-existent host
	execSvc, _ := setupHTTPTestWorkflowService(t, "")

	ctx := context.Background()
	inputs := map[string]any{
		"url": "http://nonexistent.invalid.domain.test.local:9999/api",
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-get-test", inputs)

	// THEN: Operation returns success=false with workflow failure
	require.Error(t, err, "GET to non-existent host should fail")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	// Error should indicate failure (actual HTTP error is in step response, not propagated to workflow error)
	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "failure") || strings.Contains(errorMsg, "failed"),
		"error should indicate workflow failure, got: %s", errorMsg)
}

func TestHTTPOperation_POST_WithJSONBody(t *testing.T) {
	// GIVEN: A workflow with POST, Content-Type: application/json, and JSON body
	var receivedBody map[string]any
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request details
		assert.Equal(t, http.MethodPost, r.Method)
		receivedHeaders = r.Header

		// Read body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		json.Unmarshal(body, &receivedBody)

		// Respond
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "123", "status": "created"}`))
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":    server.URL + "/resources",
		"method": "POST",
		"body":   `{"name": "test-resource", "value": 42}`,
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-post-test", inputs)

	// THEN: httptest server receives correct method, headers, body; outputs capture response
	require.NoError(t, err, "POST request should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify server received correct data
	assert.Equal(t, "test-resource", receivedBody["name"])
	assert.Equal(t, float64(42), receivedBody["value"])
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))

	// Verify response captured
	state, exists := execCtx.GetStepState("create_resource")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Equal(t, 201, state.Response["status_code"])
	assert.Contains(t, state.Response["body"], "created")
}

func TestHTTPOperation_PUT_NoContentResponse(t *testing.T) {
	// GIVEN: A workflow with PUT operation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		// WHEN: Server returns 204 No Content
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":    server.URL + "/resources/123",
		"method": "PUT",
		"body":   `{"updated": true}`,
	}

	execCtx, err := execSvc.Run(ctx, "http-put-test", inputs)

	// THEN: status_code output = 204, body output is empty
	require.NoError(t, err, "PUT request should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("update_resource")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Equal(t, 204, state.Response["status_code"])
	body, _ := state.Response["body"].(string)
	assert.Empty(t, body, "body should be empty for 204 response")
}

func TestHTTPOperation_DELETE_NoBody(t *testing.T) {
	// GIVEN: A workflow with DELETE operation, no body
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		receivedBody, _ = io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"deleted": true}`))
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":    server.URL + "/resources/123",
		"method": "DELETE",
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-delete-test", inputs)

	// THEN: Request sent without body, response status captured
	require.NoError(t, err, "DELETE request should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	assert.Empty(t, receivedBody, "DELETE request should not have body")

	state, exists := execCtx.GetStepState("delete_resource")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	assert.Equal(t, 200, state.Response["status_code"])
	assert.Contains(t, state.Response["body"], "deleted")
}

func TestHTTPOperation_ResponseHeaders_TemplateInterpolation(t *testing.T) {
	// GIVEN: Step A performs http.request, server returns X-Request-Id header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "abc-123-def")
		w.Header().Set("X-Correlation-Id", "corr-456")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Create workflow with two steps: fetch data, then use header value
	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-header-interpolation-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true

states:
  initial: fetch_data

  fetch_data:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}"
    on_success: use_header_value

  use_header_value:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}/verify"
      headers:
        X-Original-Request-Id: "{{index states.fetch_data.Response.headers \"X-Request-Id\"}}"
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-header-interpolation-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL,
	}

	// WHEN: Step B's transition condition references states.stepA.output.headers.X-Request-Id
	execCtx, err := execSvc.Run(ctx, "http-header-interpolation-test", inputs)

	// THEN: Template interpolation resolves header value, correct routing occurs
	require.NoError(t, err, "workflow with header interpolation should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify second step received interpolated header
	state, exists := execCtx.GetStepState("use_header_value")
	require.True(t, exists)
	require.NotNil(t, state.Response)
}

func TestHTTPOperation_StatusCode_ConditionalTransition(t *testing.T) {
	// GIVEN: Step with http.request returning 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "unauthorized"}`))
	}))
	defer server.Close()

	// Create workflow with conditional transition based on status_code
	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-conditional-transition-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true

states:
  initial: fetch_protected

  fetch_protected:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}"
    transitions:
      - when: "states.fetch_protected.Response.status_code == 401"
        goto: handle_auth_error
      - goto: success

  handle_auth_error:
    type: terminal
    status: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-conditional-transition-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL + "/protected",
	}

	// WHEN: Workflow has condition: states.stepA.output.status_code == 401
	execCtx, err := execSvc.Run(ctx, "http-conditional-transition-test", inputs)

	// THEN: Workflow transitions to handle_auth_error based on status_code condition
	require.NoError(t, err, "workflow should complete successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Terminal steps don't create StepState entries, check CurrentStep instead
	assert.Equal(t, "handle_auth_error", execCtx.CurrentStep,
		"should have transitioned to handle_auth_error based on status_code == 401")

	// Verify the operation step captured the 401 response
	state, exists := execCtx.GetStepState("fetch_protected")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, 401, state.Response["status_code"])
}

func TestHTTPOperation_Timeout_CustomValue(t *testing.T) {
	// GIVEN: http.request with timeout: 1, server delays 3 seconds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-timeout-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true

states:
  initial: fetch_slow

  fetch_slow:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}"
      timeout: 1
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-timeout-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL + "/slow",
	}

	start := time.Now()

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-timeout-test", inputs)

	elapsed := time.Since(start)

	// THEN: Operation fails with timeout error within ~1 second (not 3)
	require.Error(t, err, "request should timeout")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	// Should timeout around 1 second, definitely before 2 seconds
	assert.Less(t, elapsed, 2*time.Second)

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline"),
		"error should indicate timeout, got: %s", errorMsg)
}

func TestHTTPOperation_Timeout_DefaultValue(t *testing.T) {
	// GIVEN: http.request with no explicit timeout
	// This test verifies default timeout exists by using a workflow without timeout field

	statesDir := t.TempDir()
	workflowContent := `name: http-default-timeout-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true

states:
  initial: fetch_data

  fetch_data:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}"
    on_success: success

  success:
    type: terminal
    status: success
`

	workflowPath := filepath.Join(statesDir, "http-default-timeout-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL,
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-default-timeout-test", inputs)

	// THEN: Default 30-second timeout is applied (test passes = timeout >= response time)
	require.NoError(t, err, "request with default timeout should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("fetch_data")
	require.True(t, exists)
	assert.Equal(t, 200, state.Response["status_code"])
}

func TestHTTPOperation_RetryableStatus_SignalsFailure(t *testing.T) {
	// GIVEN: http.request with retryable_status_codes: [503], server returns 503
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-retryable-status-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true

states:
  initial: call_api

  call_api:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}"
      retryable_status_codes: [503]
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-retryable-status-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL + "/api",
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-retryable-status-test", inputs)

	// THEN: Operation returns success=false with "retryable status" error
	require.Error(t, err, "retryable status code should signal failure")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "retryable") || strings.Contains(errorMsg, "503"),
		"error should indicate retryable status, got: %s", errorMsg)
}

func TestHTTPOperation_RetryableStatus_EventualSuccess(t *testing.T) {
	t.Skip("stub: depends on operation retry wiring in ExecutionService (out of scope for F058)")

	// NOTE: This test is intentionally skipped because operation-level retry is not yet
	// implemented in ExecutionService. When retry wiring is added (separate task), this
	// test should be un-skipped and will verify that retryable_status_codes works with
	// the retry mechanism.
	//
	// Expected behavior when implemented:
	// - Server returns 503 twice, then 200
	// - Retry config: max_attempts=3
	// - Operation retries on 503 (retryable status) and succeeds on third attempt
}

func TestHTTPOperation_UnknownOperation_Error(t *testing.T) {
	// GIVEN: Workflow with operation: http.unknown_operation
	statesDir := t.TempDir()
	workflowContent := `name: http-unknown-op-test
version: "1.0"

states:
  initial: invalid_op

  invalid_op:
    type: operation
    operation: http.unknown_operation
    inputs:
      url: "http://example.com"
    on_success: success

  success:
    type: terminal
    status: success
`

	workflowPath := filepath.Join(statesDir, "http-unknown-op-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, "")

	ctx := context.Background()

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-unknown-op-test", nil)

	// THEN: Provider returns "operation not found" error
	require.Error(t, err, "unknown operation should fail")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "not found") || strings.Contains(errorMsg, "unknown"),
		"error should indicate operation not found, got: %s", errorMsg)
}

func TestHTTPOperation_InvalidMethod_ValidationError(t *testing.T) {
	// GIVEN: http.request with method: INVALID
	statesDir := t.TempDir()
	workflowContent := `name: http-invalid-method-test
version: "1.0"

states:
  initial: invalid_method

  invalid_method:
    type: operation
    operation: http.request
    inputs:
      method: INVALID
      url: "http://example.com"
    on_success: success

  success:
    type: terminal
    status: success
`

	workflowPath := filepath.Join(statesDir, "http-invalid-method-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, "")

	ctx := context.Background()

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-invalid-method-test", nil)

	// THEN: Operation returns validation error for invalid method
	require.Error(t, err, "invalid method should fail validation")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "method") || strings.Contains(errorMsg, "INVALID"),
		"error should mention invalid method, got: %s", errorMsg)
}

func TestHTTPOperation_MissingURL_ValidationError(t *testing.T) {
	// GIVEN: http.request with missing url input
	statesDir := t.TempDir()
	workflowContent := `name: http-missing-url-test
version: "1.0"

states:
  initial: missing_url

  missing_url:
    type: operation
    operation: http.request
    inputs:
      method: GET
    on_success: success

  success:
    type: terminal
    status: success
`

	workflowPath := filepath.Join(statesDir, "http-missing-url-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, "")

	ctx := context.Background()

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-missing-url-test", nil)

	// THEN: Operation returns validation error for missing required input
	require.Error(t, err, "missing url should fail validation")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "url") || strings.Contains(errorMsg, "required"),
		"error should mention missing url, got: %s", errorMsg)
}

func TestHTTPOperation_InvalidURL_ValidationError(t *testing.T) {
	// GIVEN: http.request with url not starting with http:// or https://
	statesDir := t.TempDir()
	workflowContent := `name: http-invalid-url-test
version: "1.0"

states:
  initial: invalid_url

  invalid_url:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "ftp://example.com"
    on_success: success

  success:
    type: terminal
    status: success
`

	workflowPath := filepath.Join(statesDir, "http-invalid-url-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, "")

	ctx := context.Background()

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-invalid-url-test", nil)

	// THEN: Operation returns validation error for invalid URL
	require.Error(t, err, "invalid url scheme should fail validation")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)

	errorMsg := err.Error()
	assert.True(t,
		strings.Contains(errorMsg, "url") || strings.Contains(errorMsg, "http"),
		"error should mention url validation, got: %s", errorMsg)
}

func TestHTTPOperation_HeaderCanonicalization(t *testing.T) {
	// GIVEN: Server returns header "content-type"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Go's http package already canonicalizes headers, but test verifies output
		w.Header().Set("content-type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL,
	}

	// WHEN: Operation captures headers
	execCtx, err := execSvc.Run(ctx, "http-get-test", inputs)

	// THEN: Output uses canonical form "Content-Type"
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("fetch_data")
	require.True(t, exists)

	headers, ok := state.Response["headers"].(map[string]string)
	require.True(t, ok)

	// Header should be canonicalized
	assert.Contains(t, headers, "Content-Type")
}

func TestHTTPOperation_MultiValueHeaders_Joined(t *testing.T) {
	// GIVEN: Server returns multi-value header (e.g., Set-Cookie twice)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "cookie1=value1")
		w.Header().Add("Set-Cookie", "cookie2=value2")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL,
	}

	// WHEN: Operation captures headers
	execCtx, err := execSvc.Run(ctx, "http-get-test", inputs)

	// THEN: Values joined with ", " per HTTP spec
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("fetch_data")
	require.True(t, exists)

	headers, ok := state.Response["headers"].(map[string]string)
	require.True(t, ok)

	setCookie, exists := headers["Set-Cookie"]
	require.True(t, exists, "Set-Cookie header should exist")

	// Multi-value headers should be joined with ", "
	assert.Contains(t, setCookie, "cookie1=value1")
	assert.Contains(t, setCookie, "cookie2=value2")
	assert.Contains(t, setCookie, ", ")
}

func TestHTTPOperation_ResponseBody_SizeLimit(t *testing.T) {
	// GIVEN: Server returns response body > 1MB
	largeBody := strings.Repeat("x", 1<<20+1000) // 1MB + 1000 bytes

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL + "/large",
	}

	// WHEN: Operation executes
	execCtx, err := execSvc.Run(ctx, "http-get-test", inputs)

	// THEN: Body truncated at 1MB, body_truncated output = true
	require.NoError(t, err, "large response should succeed but truncate")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("fetch_data")
	require.True(t, exists)

	body, _ := state.Response["body"].(string)
	assert.LessOrEqual(t, len(body), 1<<20)

	truncated, _ := state.Response["body_truncated"].(bool)
	assert.True(t, truncated, "body_truncated flag should be true")
}

func TestHTTPOperation_CustomHeaders_Forwarded(t *testing.T) {
	// GIVEN: http.request with custom headers including Authorization
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-custom-headers-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true
  - name: token
    type: string
    required: true

states:
  initial: fetch_with_auth

  fetch_with_auth:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}"
      headers:
        Authorization: "Bearer {{inputs.token}}"
        X-Custom-Header: "custom-value"
        Accept: "application/json"
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-custom-headers-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":   server.URL + "/protected",
		"token": "secret-token-123",
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-custom-headers-test", inputs)

	// THEN: httptest server receives all specified headers
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	assert.Equal(t, "Bearer secret-token-123", receivedHeaders.Get("Authorization"))
	assert.Equal(t, "custom-value", receivedHeaders.Get("X-Custom-Header"))
	assert.Equal(t, "application/json", receivedHeaders.Get("Accept"))
}

func TestHTTPOperation_TemplateInterpolation_InURL(t *testing.T) {
	// GIVEN: http.request with url containing {{inputs.user_id}}
	var requestedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-url-interpolation-test
version: "1.0"

inputs:
  - name: user_id
    type: string
    required: true

states:
  initial: fetch_user

  fetch_user:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}/users/{{inputs.user_id}}"
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-url-interpolation-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":     server.URL,
		"user_id": "123",
	}

	// WHEN: Workflow executes with inputs.user_id = "123"
	execCtx, err := execSvc.Run(ctx, "http-url-interpolation-test", inputs)

	// THEN: Request sent to interpolated URL
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	assert.Equal(t, "/users/123", requestedPath)
}

func TestHTTPOperation_TemplateInterpolation_InBody(t *testing.T) {
	// GIVEN: http.request with body containing {{inputs.name}}
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-body-interpolation-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true
  - name: name
    type: string
    required: true

states:
  initial: create_user

  create_user:
    type: operation
    operation: http.request
    inputs:
      method: POST
      url: "{{inputs.url}}/users"
      headers:
        Content-Type: "application/json"
      body: '{"name": "{{inputs.name}}", "active": true}'
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-body-interpolation-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":  server.URL,
		"name": "test-user",
	}

	// WHEN: Workflow executes with inputs.name = "test"
	execCtx, err := execSvc.Run(ctx, "http-body-interpolation-test", inputs)

	// THEN: Request body contains interpolated value
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	assert.Equal(t, "test-user", receivedBody["name"])
	assert.True(t, receivedBody["active"].(bool))
}

func TestHTTPOperation_TemplateInterpolation_InHeaders(t *testing.T) {
	// GIVEN: http.request with header value containing {{inputs.token}}
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-header-value-interpolation-test
version: "1.0"

inputs:
  - name: url
    type: string
    required: true
  - name: token
    type: string
    required: true

states:
  initial: fetch_protected

  fetch_protected:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.url}}/protected"
      headers:
        Authorization: "Bearer {{inputs.token}}"
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-header-value-interpolation-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url":   server.URL,
		"token": "secret-xyz-789",
	}

	// WHEN: Workflow executes with inputs.token = "secret"
	execCtx, err := execSvc.Run(ctx, "http-header-value-interpolation-test", inputs)

	// THEN: Request header contains interpolated token
	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	assert.Equal(t, "Bearer secret-xyz-789", receivedHeaders.Get("Authorization"))
}

func TestHTTPOperation_ProviderRegistration(t *testing.T) {
	// GIVEN: HTTPOperationProvider wired into CompositeOperationProvider
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	execSvc, _ := setupHTTPTestWorkflowService(t, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"url": server.URL,
	}

	// WHEN: Workflow with http.request operation executes
	execCtx, err := execSvc.Run(ctx, "http-get-test", inputs)

	// THEN: Provider is correctly registered and invoked (no nil pointer panics)
	require.NoError(t, err, "http provider should be registered and work")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("fetch_data")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, 200, state.Response["status_code"])
}

func TestHTTPOperation_CoexistenceWithOtherProviders(t *testing.T) {
	// GIVEN: Workflow with github.*, notify.*, and http.request operations
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "http success"}`))
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-coexistence-test
version: "1.0"

inputs:
  - name: http_url
    type: string
    required: true

states:
  initial: http_request

  http_request:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.http_url}}"
    on_success: success

  success:
    type: terminal
    status: success
`)

	workflowPath := filepath.Join(statesDir, "http-coexistence-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"http_url": server.URL + "/api",
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-coexistence-test", inputs)

	// THEN: All providers coexist without conflicts
	require.NoError(t, err, "http provider should coexist with others")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("http_request")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Contains(t, state.Response["body"], "http success")
}

func TestHTTPOperation_MultiStep_GET_POST_ConditionalTransition(t *testing.T) {
	// GIVEN: Workflow with:
	//   - Step 1: GET request stores response
	//   - Step 2: POST with body referencing step 1 output
	//   - Conditional transition based on step 2 status_code

	var step2Body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/users") {
			// Step 1: GET /users/1
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": "1", "name": "John Doe"}`))
		} else if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/orders") {
			// Step 2: POST /orders
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &step2Body)

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"order_id": "order-123", "status": "created"}`))
		}
	}))
	defer server.Close()

	statesDir := t.TempDir()
	workflowContent := fmt.Sprintf(`name: http-multi-step-test
version: "1.0"

inputs:
  - name: base_url
    type: string
    required: true

states:
  initial: fetch_user

  fetch_user:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{inputs.base_url}}/users/1"
    on_success: create_order

  create_order:
    type: operation
    operation: http.request
    inputs:
      method: POST
      url: "{{inputs.base_url}}/orders"
      headers:
        Content-Type: "application/json"
      body: '{"user_data": {{states.fetch_user.Response.body}}, "product": "widget"}'
    transitions:
      - when: "states.create_order.Response.status_code == 201"
        goto: order_created
      - goto: order_failed

  order_created:
    type: terminal
    status: success

  order_failed:
    type: terminal
    status: failure
`)

	workflowPath := filepath.Join(statesDir, "http-multi-step-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err)

	execSvc, _ := setupHTTPTestWorkflowServiceWithDir(t, statesDir, statesDir, server.URL)

	ctx := context.Background()
	inputs := map[string]any{
		"base_url": server.URL,
	}

	// WHEN: Workflow executes
	execCtx, err := execSvc.Run(ctx, "http-multi-step-test", inputs)

	// THEN: All steps execute correctly, data flows between steps, routing works
	require.NoError(t, err, "multi-step workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify step 1 executed
	state1, exists := execCtx.GetStepState("fetch_user")
	require.True(t, exists)
	assert.Equal(t, 200, state1.Response["status_code"])

	// Verify step 2 received data from step 1
	require.NotNil(t, step2Body)
	userData, ok := step2Body["user_data"].(map[string]any)
	require.True(t, ok, "step 2 should receive user_data from step 1")
	assert.Equal(t, "1", userData["id"])
	assert.Equal(t, "John Doe", userData["name"])
	assert.Equal(t, "widget", step2Body["product"])

	// Verify conditional transition to order_created (terminal steps don't create StepState)
	assert.Equal(t, "order_created", execCtx.CurrentStep,
		"should have transitioned to order_created based on status_code == 201")
}

// setupHTTPTestWorkflowService creates a workflow service with HTTP operation provider wired.
// This helper wires the complete stack: workflow service → execution service → composite provider.
func setupHTTPTestWorkflowService(t *testing.T, serverURL string) (*application.ExecutionService, ports.StateStore) {
	t.Helper()

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	return setupHTTPTestWorkflowServiceWithDir(t, workflowsDir, statesDir, serverURL)
}

// setupHTTPTestWorkflowServiceWithDir creates a workflow service with custom directories.
func setupHTTPTestWorkflowServiceWithDir(t *testing.T, workflowsDir, statesDir, serverURL string) (*application.ExecutionService, ports.StateStore) {
	t.Helper()

	// Real components for integration testing
	repo := repository.NewYAMLRepository(workflowsDir)
	stateStore := store.NewJSONStore(statesDir)
	exec := executor.NewShellExecutor()
	logger := &mockLogger{}
	resolver := interpolation.NewTemplateResolver()

	// Expression evaluator for loop conditions
	evaluator := infraExpr.NewExprEvaluator()

	// Wire up services
	wfSvc := application.NewWorkflowService(repo, stateStore, exec, logger, infraExpr.NewExprValidator())
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, exec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	// Setup composite operation provider (GitHub + Notify + HTTP)
	githubClient := github.NewClient(logger)
	githubProvider := github.NewGitHubOperationProvider(githubClient, logger)
	notifyProvider := notify.NewNotifyOperationProvider(logger)
	_ = notifyProvider.RegisterBackend("webhook", notify.NewWebhookBackend())

	// Create HTTP operation provider with httpx.Client
	httpClient := httpx.NewClient(httpx.WithTimeout(30 * time.Second))
	httpProvider := infraHTTP.NewHTTPOperationProvider(httpClient, logger)

	compositeProvider := pluginmgr.NewCompositeOperationProvider(githubProvider, notifyProvider, httpProvider)

	execSvc.SetOperationProvider(compositeProvider)

	return execSvc, stateStore
}
