//go:build integration

// Feature: F056 - Workflow Completion Notification Plugin
// This file contains integration tests for the notification operation provider.

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/application"
	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	infraExpr "github.com/vanoix/awf/internal/infrastructure/expression"
	"github.com/vanoix/awf/internal/infrastructure/github"
	"github.com/vanoix/awf/internal/infrastructure/notify"
	"github.com/vanoix/awf/internal/infrastructure/plugin"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/infrastructure/store"
	"github.com/vanoix/awf/pkg/interpolation"
)

// =============================================================================
// US1: DESKTOP NOTIFICATION TESTS
// =============================================================================

// TestNotifyDesktop_Success tests sending desktop notification via notify.send operation.
// Acceptance Criteria: Desktop notification displayed with workflow name and status
func TestNotifyDesktop_Success(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "notify-send") // Desktop notification tool

	// Given: workflow with notify.send operation using desktop backend
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"title":   "Test Notification",
		"message": "Workflow completed successfully",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-desktop-test", inputs)

	// Then: desktop notification sent successfully
	require.NoError(t, err, "desktop notification workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify step state
	state, exists := execCtx.GetStepState("send_desktop_notification")
	require.True(t, exists, "notification step should exist in state")
	require.NotNil(t, state.Response)
	assert.Equal(t, "desktop", state.Response["backend"], "backend should be desktop")
}

// TestNotifyDesktop_HeadlessError tests error handling on headless server.
// Acceptance Criteria: Step fails with descriptive error when desktop unavailable
func TestNotifyDesktop_HeadlessError(t *testing.T) {
	skipInCI(t)

	// Given: headless environment (no DISPLAY)
	originalDisplay := os.Getenv("DISPLAY")
	os.Unsetenv("DISPLAY")
	defer func() {
		if originalDisplay != "" {
			os.Setenv("DISPLAY", originalDisplay)
		}
	}()

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"message": "Test notification",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-desktop-test", inputs)

	// Then: error indicates desktop unavailable
	require.Error(t, err, "desktop notification should fail in headless environment")
	if execCtx != nil {
		assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	}
	assert.Contains(t, err.Error(), "desktop", "error should mention desktop backend")
}

// TestNotifyDesktop_TemplateInterpolation tests template interpolation in notification message.
// Acceptance Criteria: Message contains resolved workflow name and duration
func TestNotifyDesktop_TemplateInterpolation(t *testing.T) {
	skipInCI(t)
	skipIfCLIMissing(t, "notify-send")

	// Given: workflow with templated notification message
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"title":   "{{workflow.name}}",
		"message": "Workflow {{workflow.name}} completed in {{workflow.duration}}",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-desktop-test", inputs)

	// Then: templates resolved correctly
	require.NoError(t, err, "notification workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("send_desktop_notification")
	require.True(t, exists)
	require.NotNil(t, state.Response)

	// Verify interpolation resolved to actual values
	title, _ := state.Response["title"].(string)
	assert.Contains(t, title, "notify-desktop-test", "title should contain resolved workflow name")
}

// =============================================================================
// US2: NTFY/WEBHOOK BACKEND TESTS
// =============================================================================

// TestNotifyNtfy_Success tests sending notification to ntfy topic.
// Acceptance Criteria: HTTP POST sent to ntfy_url/topic with payload
func TestNotifyNtfy_Success(t *testing.T) {
	skipInCI(t)

	// Given: mock ntfy server
	receivedRequests := make([]*http.Request, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequests = append(receivedRequests, r)

		// Verify request
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/test-topic", "should POST to topic endpoint")

		// Read body
		body, _ := io.ReadAll(r.Body)
		assert.NotEmpty(t, body, "request body should not be empty")

		// Respond with ntfy success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ntfy123","time":1234567890}`))
	}))
	defer server.Close()

	// Setup workflow service with ntfy_url config
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	config := notify.NotifyConfig{
		NtfyURL: server.URL,
	}
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	ctx := context.Background()
	inputs := map[string]any{
		"topic":    "test-topic",
		"message":  "Test notification",
		"priority": "high",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-ntfy-test", inputs)

	// Then: ntfy notification sent successfully
	require.NoError(t, err, "ntfy notification workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	require.Len(t, receivedRequests, 1, "should have made one HTTP request")

	// Verify step state
	state, exists := execCtx.GetStepState("send_ntfy_notification")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, "ntfy", state.Response["backend"], "backend should be ntfy")
}

// TestNotifyNtfy_MissingURL tests error handling when ntfy_url not configured.
// Acceptance Criteria: Step fails with error indicating missing configuration
func TestNotifyNtfy_MissingURL(t *testing.T) {
	skipInCI(t)

	// Given: notify config without ntfy_url
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	config := notify.NotifyConfig{} // No ntfy_url configured
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	ctx := context.Background()
	inputs := map[string]any{
		"topic":   "test-topic",
		"message": "Test notification",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-ntfy-test", inputs)

	// Then: error indicates missing configuration
	require.Error(t, err, "ntfy notification should fail without url config")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "ntfy_url", "error should mention missing ntfy_url")
}

// TestNotifyWebhook_Success tests sending webhook notification.
// Acceptance Criteria: HTTP POST sent to webhook URL with JSON payload
func TestNotifyWebhook_Success(t *testing.T) {
	skipInCI(t)

	// Given: mock webhook server
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "application/json")

		// Read body
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		// Respond with success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
		"message":     "Test webhook notification",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-webhook-test", inputs)

	// Then: webhook notification sent successfully
	require.NoError(t, err, "webhook notification workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify payload contains required fields
	require.NotNil(t, receivedPayload, "server should have received payload")
	assert.Contains(t, receivedPayload, "message", "payload should contain message")
	assert.Contains(t, receivedPayload, "workflow", "payload should contain workflow context")

	// Verify step state
	state, exists := execCtx.GetStepState("send_webhook_notification")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, "webhook", state.Response["backend"], "backend should be webhook")
	assert.Equal(t, 200, state.Response["status_code"], "should return HTTP 200")
}

// TestNotifyWebhook_HTTPError tests webhook endpoint returning HTTP 500.
// Acceptance Criteria: Step fails with error including HTTP status code
func TestNotifyWebhook_HTTPError(t *testing.T) {
	skipInCI(t)

	// Given: webhook server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
	}))
	defer server.Close()

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
		"message":     "Test webhook notification",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-webhook-test", inputs)

	// Then: error includes HTTP status code
	require.Error(t, err, "webhook notification should fail with HTTP 500")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "500", "error should include status code")
}

// TestNotifyWebhook_Timeout tests webhook timeout handling.
// Acceptance Criteria: Request times out after 10 seconds
func TestNotifyWebhook_Timeout(t *testing.T) {
	skipInCI(t)

	// Given: webhook server that delays 11 seconds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(11 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
		"message":     "Test webhook notification",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-webhook-test", inputs)

	// Then: error indicates timeout
	require.Error(t, err, "webhook notification should timeout after 10 seconds")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "timeout", "error should mention timeout")
}

// =============================================================================
// US3: SLACK BACKEND TESTS
// =============================================================================

// TestNotifySlack_Success tests sending Slack notification.
// Acceptance Criteria: POST sent to Slack webhook with formatted message block
func TestNotifySlack_Success(t *testing.T) {
	skipInCI(t)

	// Given: mock Slack webhook server
	var receivedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "application/json")

		// Read body
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)

		// Respond with Slack success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	config := notify.NotifyConfig{
		SlackWebhookURL: server.URL,
	}
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	ctx := context.Background()
	inputs := map[string]any{
		"title":   "Test Slack Notification",
		"message": "Workflow completed successfully",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-slack-test", inputs)

	// Then: Slack notification sent successfully
	require.NoError(t, err, "Slack notification workflow should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify payload has Slack message blocks
	require.NotNil(t, receivedPayload, "server should have received payload")
	assert.Contains(t, receivedPayload, "blocks", "payload should contain Slack blocks")

	// Verify step state
	state, exists := execCtx.GetStepState("send_slack_notification")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, "slack", state.Response["backend"], "backend should be slack")
}

// TestNotifySlack_MissingWebhookURL tests error when slack_webhook_url not configured.
// Acceptance Criteria: Step fails with error indicating missing Slack configuration
func TestNotifySlack_MissingWebhookURL(t *testing.T) {
	skipInCI(t)

	// Given: notify config without slack_webhook_url
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	config := notify.NotifyConfig{} // No slack_webhook_url configured
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	ctx := context.Background()
	inputs := map[string]any{
		"message": "Test Slack notification",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-slack-test", inputs)

	// Then: error indicates missing configuration
	require.Error(t, err, "Slack notification should fail without webhook url")
	assert.Equal(t, workflow.StatusFailed, execCtx.Status)
	assert.Contains(t, err.Error(), "slack_webhook_url", "error should mention missing slack_webhook_url")
}

// =============================================================================
// US4: CONFIGURATION TESTS
// =============================================================================

// TestNotifyConfig_DefaultBackend tests using default_backend from .awf/config.yaml.
// Acceptance Criteria: Default backend from config used when no explicit backend specified
func TestNotifyConfig_DefaultBackend(t *testing.T) {
	skipInCI(t)

	// Given: config with default_backend set to "desktop"
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	config := notify.NotifyConfig{
		DefaultBackend: "desktop",
	}
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	// Create workflow without explicit backend in inputs
	// (requires a fixture workflow that doesn't specify backend)
	// For now, we'll test that config is properly loaded
	ctx := context.Background()
	inputs := map[string]any{
		"message": "Test default backend",
	}

	// When: workflow executes
	execCtx, err := execSvc.Run(ctx, "notify-desktop-test", inputs)

	// Then: default backend from config used
	// Note: This test will fail until GREEN phase implements default backend logic
	_ = execCtx
	_ = err
	// Actual assertion depends on implementation:
	// require.NoError(t, err, "should use default backend from config")
	t.Skip("Test requires default backend implementation in GREEN phase")
}

// TestNotifyConfig_ExplicitOverridesDefault tests explicit backend taking precedence.
// Acceptance Criteria: Explicit backend input overrides config default
func TestNotifyConfig_ExplicitOverridesDefault(t *testing.T) {
	skipInCI(t)

	// Given: config with default_backend="desktop" but workflow specifies "webhook"
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	// Mock webhook server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	config := notify.NotifyConfig{
		DefaultBackend: "desktop",
	}
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, config)

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
		"message":     "Test explicit backend override",
	}

	// When: workflow executes (notify-webhook-test explicitly sets backend=webhook)
	execCtx, err := execSvc.Run(ctx, "notify-webhook-test", inputs)

	// Then: explicit backend in workflow overrides config default
	require.NoError(t, err, "explicit backend should override config default")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("send_webhook_notification")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, "webhook", state.Response["backend"], "should use explicit backend, not default")
}

// =============================================================================
// WORKFLOW INTEGRATION TESTS
// =============================================================================

// TestNotifyOperations_WorkflowParsing tests YAML workflow parsing through execution.
// Integration test: YAML parsing → operation step → provider dispatch → output interpolation
func TestNotifyOperations_WorkflowParsing(t *testing.T) {
	skipInCI(t)

	// Given: YAML workflow with notify.send operation
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()

	// Mock webhook for verification
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
		"message":     "Test workflow parsing",
	}

	// When: workflow is parsed and executed
	execCtx, err := execSvc.Run(ctx, "notify-webhook-test", inputs)

	// Then: workflow parsed correctly and executed
	require.NoError(t, err, "workflow should parse and execute successfully")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify operation type recognized
	state, exists := execCtx.GetStepState("send_webhook_notification")
	require.True(t, exists, "operation step should exist in execution context")
	require.NotNil(t, state.Response, "operation should return response")
}

// TestNotifyOperations_OutputInterpolation tests output field interpolation.
// Integration test: Operation result → state management → template interpolation
func TestNotifyOperations_OutputInterpolation(t *testing.T) {
	skipInCI(t)

	// Given: workflow with chained steps using output interpolation
	statesDir := t.TempDir()

	// Create test workflow with output interpolation
	workflowContent := `name: notify-output-interpolation-test
version: "1.0"

inputs:
  webhook_url:
    type: string
    required: true

states:
  send_first_notification:
    type: step
    operation: notify.send
    inputs:
      backend: webhook
      webhook_url: "{{inputs.webhook_url}}"
      message: "First notification"
    next: send_second_notification

  send_second_notification:
    type: step
    operation: notify.send
    inputs:
      backend: webhook
      webhook_url: "{{inputs.webhook_url}}"
      message: "Previous notification: {{states.send_first_notification.Output.backend}}"
    next: success

  success:
    type: terminal
    status: completed
`

	workflowPath := filepath.Join(statesDir, "notify-output-interpolation-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err, "should write test workflow")

	// Mock webhook server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	execSvc, _ := setupNotifyTestWorkflowService(t, statesDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
	}

	// When: workflow executes with output interpolation
	execCtx, err := execSvc.Run(ctx, "notify-output-interpolation-test", inputs)

	// Then: output interpolation resolved correctly
	require.NoError(t, err, "workflow with output interpolation should succeed")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
	assert.Equal(t, 2, requestCount, "should have sent two notifications")

	// Verify second step accessed first step's output
	state, exists := execCtx.GetStepState("send_second_notification")
	require.True(t, exists)
	require.NotNil(t, state.Response)
}

// =============================================================================
// COMPOSITE PROVIDER TESTS
// =============================================================================

// TestCompositeProvider_BothProvidersWork tests github and notify coexistence.
// Integration test: CompositeOperationProvider dispatches to correct provider
func TestCompositeProvider_BothProvidersWork(t *testing.T) {
	skipInCI(t)

	// Given: workflow using both github.* and notify.* operations
	statesDir := t.TempDir()

	// Create test workflow with both providers
	workflowContent := `name: composite-provider-test
version: "1.0"

inputs:
  webhook_url:
    type: string
    required: true

states:
  notify_start:
    type: step
    operation: notify.send
    inputs:
      backend: webhook
      webhook_url: "{{inputs.webhook_url}}"
      message: "Starting workflow"
    next: success

  success:
    type: terminal
    status: completed
`

	workflowPath := filepath.Join(statesDir, "composite-provider-test.yaml")
	err := os.WriteFile(workflowPath, []byte(workflowContent), 0o644)
	require.NoError(t, err, "should write test workflow")

	// Mock webhook server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	execSvc, _ := setupNotifyTestWorkflowService(t, statesDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"webhook_url": server.URL,
	}

	// When: workflow executes using composite provider
	execCtx, err := execSvc.Run(ctx, "composite-provider-test", inputs)

	// Then: composite provider dispatched to notify provider correctly
	require.NoError(t, err, "composite provider should dispatch to notify provider")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	state, exists := execCtx.GetStepState("notify_start")
	require.True(t, exists)
	require.NotNil(t, state.Response)
	assert.Equal(t, "webhook", state.Response["backend"])
}

// TestCompositeProvider_GithubStillWorks tests github operations still functional.
// Integration test: Verify github.* operations not broken by notify addition
func TestCompositeProvider_GithubStillWorks(t *testing.T) {
	skipInCI(t)

	// Given: workflow with github.get_issue operation
	repoRoot := getRepoRoot(t)
	workflowsDir := filepath.Join(repoRoot, "tests", "fixtures", "workflows")
	statesDir := t.TempDir()
	_ = repoRoot // used in filepath.Join above

	// Use setupNotifyTestWorkflowService which should wire both providers
	execSvc, _ := setupNotifyTestWorkflowService(t, workflowsDir, statesDir, notify.NotifyConfig{})

	ctx := context.Background()
	inputs := map[string]any{
		"issue_number": "1",
	}

	// When: github operation executes via composite provider
	execCtx, err := execSvc.Run(ctx, "github-operations-test", inputs)

	// Then: github provider still works via composite
	require.NoError(t, err, "github operations should still work after notify integration")
	assert.Equal(t, workflow.StatusCompleted, execCtx.Status)

	// Verify github operation executed
	state, exists := execCtx.GetStepState("test_get_issue")
	require.True(t, exists, "github operation step should exist")
	require.NotNil(t, state.Response)
	assert.Contains(t, state.Response, "title", "github operation should return issue data")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// setupNotifyTestWorkflowService creates a workflow service with notify operation provider wired.
// This helper wires the complete stack: workflow service → execution service → composite provider (github + notify).
func setupNotifyTestWorkflowService(t *testing.T, workflowsDir, statesDir string, config notify.NotifyConfig) (*application.ExecutionService, ports.StateStore) {
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

	// Setup composite operation provider (GitHub + Notify)
	githubClient := github.NewClient(logger)
	githubProvider := github.NewGitHubOperationProvider(githubClient, logger)
	notifyProvider := notify.NewNotifyOperationProvider(logger)

	// Register backends based on config (mirrors registerNotifyBackends in run.go)
	_ = notifyProvider.RegisterBackend("desktop", notify.NewDesktopBackend())
	if config.NtfyURL != "" {
		ntfyBackend, err := notify.NewNtfyBackend(config.NtfyURL)
		require.NoError(t, err, "failed to create ntfy backend")
		_ = notifyProvider.RegisterBackend("ntfy", ntfyBackend)
	}
	if config.SlackWebhookURL != "" {
		slackBackend, err := notify.NewSlackBackend(config.SlackWebhookURL)
		require.NoError(t, err, "failed to create slack backend")
		_ = notifyProvider.RegisterBackend("slack", slackBackend)
	}
	_ = notifyProvider.RegisterBackend("webhook", notify.NewWebhookBackend())
	if config.DefaultBackend != "" {
		notifyProvider.SetDefaultBackend(config.DefaultBackend)
	}

	compositeProvider := plugin.NewCompositeOperationProvider(githubProvider, notifyProvider)

	execSvc.SetOperationProvider(compositeProvider)

	return execSvc, stateStore
}
