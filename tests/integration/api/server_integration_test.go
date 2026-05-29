//go:build integration

package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/infrastructure/executor"
	infraExpr "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/store"
	"github.com/awf-project/cli/internal/interfaces/api"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/awf-project/cli/tests/integration/testhelpers"
)

// sseEvent holds a single parsed SSE event from the stream.
type sseEvent struct {
	eventType string
	data      string
}

// newTestServer constructs an httptest.Server wrapping the real api.Server wired with
// in-memory infrastructure pointing at fixtureDir for workflow YAML files.
// Returns the server, bridge, and MockLogger so tests can assert on captured log messages.
func newTestServer(t *testing.T, fixtureDir string) (*httptest.Server, *api.Bridge, *testhelpers.MockLogger) {
	t.Helper()

	statesDir := t.TempDir()
	logger := &testhelpers.MockLogger{}

	repo := repository.NewYAMLRepository(fixtureDir)
	stateStore := store.NewJSONStore(statesDir)
	shellExec := executor.NewShellExecutor()
	resolver := interpolation.NewTemplateResolver()
	evaluator := infraExpr.NewExprEvaluator()
	validator := infraExpr.NewExprValidator()

	wfSvc := application.NewWorkflowService(repo, stateStore, shellExec, logger, validator)
	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	bridge := api.NewBridge(wfSvc, execSvc, nil)
	srv := api.NewServer(bridge, "127.0.0.1:0")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts, bridge, logger
}

// readSSEEvents reads lines from an SSE response body until a terminal event
// ("workflow.completed" or "workflow.failed") is received or ctx is cancelled.
func readSSEEvents(ctx context.Context, t *testing.T, body interface{ Read([]byte) (int, error) }) []sseEvent {
	t.Helper()

	var events []sseEvent
	scanner := bufio.NewScanner(body)

	var currentType string
	var currentData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return events
		default:
		}

		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			currentType = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			currentData.WriteString(strings.TrimPrefix(line, "data: "))
		case line == "" && currentType != "":
			events = append(events, sseEvent{eventType: currentType, data: currentData.String()})
			terminal := currentType == "workflow.completed" || currentType == "workflow.failed"
			currentType = ""
			currentData.Reset()
			if terminal {
				return events
			}
		}
	}

	return events
}

// fixtureDir returns the path to tests/fixtures/api relative to the repo root.
func apiFixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testhelpers.GetRepoRoot(t), "tests", "fixtures", "api")
}

// postRunWorkflow POSTs to /api/workflows/{name}/run and returns the execution_id.
func postRunWorkflow(t *testing.T, ts *httptest.Server, name string, inputs map[string]any) string {
	t.Helper()

	if inputs == nil {
		inputs = map[string]any{}
	}
	bodyBytes, err := json.Marshal(map[string]any{"inputs": inputs})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/workflows/local/"+name+"/run", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Body.ExecutionID, "execution_id must be present in run response")

	return result.Body.ExecutionID
}

// openSSEStream opens GET /api/executions/{id}/events and returns the response.
func openSSEStream(ctx context.Context, t *testing.T, ts *httptest.Server, executionID string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		ts.URL+"/api/executions/"+executionID+"/events", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	return resp
}

// TestAPI_RunWorkflow_FullSSESequence_Integration starts a real api.Server, POSTs a
// simple two-step workflow, subscribes to SSE, and asserts the stream terminates with
// a "workflow.completed" event (US1 scenario 2).
func TestAPI_RunWorkflow_FullSSESequence_Integration(t *testing.T) {
	ts, _, mockLogger := newTestServer(t, apiFixtureDir(t))

	executionID := postRunWorkflow(t, ts, "api-simple-success", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sseResp := openSSEStream(ctx, t, ts, executionID)
	t.Cleanup(func() { sseResp.Body.Close() })

	events := readSSEEvents(ctx, t, sseResp.Body)

	require.NotEmpty(t, events, "SSE stream should have emitted at least one event")
	last := events[len(events)-1]
	assert.Equal(t, "workflow.completed", last.eventType, "last SSE event should be workflow.completed")

	var payload struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal([]byte(last.data), &payload))
	assert.Equal(t, "completed", payload.Status)
	assert.Empty(t, mockLogger.Errors(), "successful workflow execution should not produce error logs")
}

// TestAPI_CancelWorkflow_PropagatesToExecutionService_Integration POSTs a slow workflow,
// subscribes to SSE, then sends DELETE and asserts the stream reports cancellation (US3 scenario 2).
func TestAPI_CancelWorkflow_PropagatesToExecutionService_Integration(t *testing.T) {
	ts, _, mockLogger := newTestServer(t, apiFixtureDir(t))

	executionID := postRunWorkflow(t, ts, "api-slow", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Subscribe to SSE before cancelling so we can observe the terminal event.
	sseResp := openSSEStream(ctx, t, ts, executionID)
	t.Cleanup(func() { sseResp.Body.Close() })

	// Cancel the execution via DELETE.
	cancelReq, err := http.NewRequestWithContext(context.Background(), http.MethodDelete,
		ts.URL+"/api/executions/"+executionID, nil)
	require.NoError(t, err)
	cancelResp, err := http.DefaultClient.Do(cancelReq)
	require.NoError(t, err)
	cancelResp.Body.Close()
	assert.Equal(t, http.StatusNoContent, cancelResp.StatusCode)

	// SSE must emit a terminal event reflecting the cancellation.
	events := readSSEEvents(ctx, t, sseResp.Body)
	require.NotEmpty(t, events, "SSE stream should have emitted at least one event after cancel")

	last := events[len(events)-1]
	assert.Equal(t, "workflow.failed", last.eventType, "cancelled execution should emit workflow.failed terminal event")

	var payload struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal([]byte(last.data), &payload))
	assert.Equal(t, "cancelled", payload.Status, "terminal SSE event status should be 'cancelled'")
	assert.NotEmpty(t, mockLogger.Infos(), "cancelled workflow execution should produce info logs")
}

// TestAPI_FailedWorkflow_EmitsStepFailedThenWorkflowFailed_Integration POSTs a fixture
// with an intentionally failing step, subscribes to SSE, and asserts the stream contains
// "step.failed" followed by "workflow.failed" (US1 scenario 3).
func TestAPI_FailedWorkflow_EmitsStepFailedThenWorkflowFailed_Integration(t *testing.T) {
	ts, _, mockLogger := newTestServer(t, apiFixtureDir(t))

	executionID := postRunWorkflow(t, ts, "api-failing", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sseResp := openSSEStream(ctx, t, ts, executionID)
	t.Cleanup(func() { sseResp.Body.Close() })

	events := readSSEEvents(ctx, t, sseResp.Body)

	require.NotEmpty(t, events, "SSE stream should have emitted at least one event")

	eventTypes := make([]string, 0, len(events))
	for _, e := range events {
		eventTypes = append(eventTypes, e.eventType)
	}

	stepFailedIdx := -1
	workflowFailedIdx := -1
	for i, et := range eventTypes {
		if et == "step.failed" && stepFailedIdx == -1 {
			stepFailedIdx = i
		}
		if et == "workflow.failed" {
			workflowFailedIdx = i
		}
	}

	assert.GreaterOrEqual(t, stepFailedIdx, 0, "SSE stream should contain a step.failed event")
	assert.GreaterOrEqual(t, workflowFailedIdx, 0, "SSE stream should contain a workflow.failed event")
	if stepFailedIdx >= 0 && workflowFailedIdx >= 0 {
		assert.Greater(t, workflowFailedIdx, stepFailedIdx,
			"workflow.failed should appear after step.failed")
	}
	assert.NotEmpty(t, mockLogger.Infos(), "failed workflow execution should produce info logs")
}
