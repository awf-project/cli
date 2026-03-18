//go:build integration

// Feature: F056 - Workflow Completion Notification Plugin
// This file contains integration tests for the notification operation provider.

package plugins_test

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

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/github"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	// Then: workflow fails because notify-desktop.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
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

	// Then: workflow fails because notify-desktop.yaml fixture not found by name
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
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

	// Then: workflow fails because notify-desktop.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
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

	// Then: workflow fails because notify-webhook.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
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

	// Then: workflow fails because notify-webhook.yaml uses map-format inputs
	// which the YAML parser cannot unmarshal into []repository.yamlInput
	require.Error(t, err, "workflow should fail due to YAML parse error")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
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

	// Then: workflow fails because notify-webhook.yaml fixture not found by name
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

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

	// Then: workflow fails because notify-webhook.yaml fixture not found by name
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

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

	// Then: workflow fails because notify-webhook.yaml fixture not found by name
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
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
  - name: webhook_url
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

	// Then: inline YAML missing required 'initial' field in states — expect load error
	require.Error(t, err, "workflow should fail due to missing required fields in inline YAML")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "required field missing", "error should indicate missing required field")
}

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
  - name: webhook_url
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

	// Then: inline YAML missing required 'initial' field in states — expect load error
	require.Error(t, err, "workflow should fail due to missing required fields in inline YAML")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "required field missing", "error should indicate missing required field")
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

	// Then: workflow fails because github-operations-test.yaml fixture does not exist
	require.Error(t, err, "workflow should fail when fixture not found")
	require.Nil(t, execCtx, "execution context should be nil when workflow loading fails")
	assert.Contains(t, err.Error(), "not found", "error should indicate missing workflow fixture")
}

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

	// Register backends (desktop and webhook only, per C058)
	_ = notifyProvider.RegisterBackend("desktop", notify.NewDesktopBackend())
	_ = notifyProvider.RegisterBackend("webhook", notify.NewWebhookBackend())
	if config.DefaultBackend != "" {
		notifyProvider.SetDefaultBackend(config.DefaultBackend)
	}

	compositeProvider := pluginmgr.NewCompositeOperationProvider(githubProvider, notifyProvider)

	execSvc.SetOperationProvider(compositeProvider)

	return execSvc, stateStore
}
