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
	"github.com/awf-project/cli/internal/infrastructure/workflowpkg"
	"github.com/awf-project/cli/internal/interfaces/api"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/awf-project/cli/tests/integration/testhelpers"
)

// newTestServerWithPacks builds an httptest.Server wired with both a local
// YAML repository and a PackDiscoverer rooted at packsDir.
func newTestServerWithPacks(t *testing.T, fixtureDir, packsDir string) *httptest.Server {
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
	wfSvc.SetPackDiscoverer(workflowpkg.NewPackDiscovererAdapter([]string{packsDir}))

	parallelExec := application.NewParallelExecutor(logger)
	execSvc := application.NewExecutionServiceWithEvaluator(
		wfSvc, shellExec, parallelExec, stateStore, logger, resolver, nil, evaluator,
	)

	bridge := api.NewBridge(wfSvc, execSvc, nil)
	srv := api.NewServer(bridge, "127.0.0.1:0")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts
}

// packFixtureDir returns the path to tests/fixtures/api/packs relative to the repo root.
func packFixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(testhelpers.GetRepoRoot(t), "tests", "fixtures", "api", "packs")
}

// assertPackWiring fails fast if GET /api/workflows does not return at least one
// entry with scope=="speckit" and one with scope=="local".
func assertPackWiring(t *testing.T, ts *httptest.Server) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/workflows", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			Workflows []struct {
				Scope string `json:"scope"`
			} `json:"workflows"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	var hasSpeckit, hasLocal bool
	for _, w := range result.Body.Workflows {
		if w.Scope == "speckit" {
			hasSpeckit = true
		}
		if w.Scope == "local" {
			hasLocal = true
		}
	}
	require.True(t, hasSpeckit, "PackDiscoverer wiring failed: no speckit-scoped workflow in GET /api/workflows")
	require.True(t, hasLocal, "local repository wiring failed: no local-scoped workflow in GET /api/workflows")
}

// TestAPI_PackWorkflow_Get sends GET /api/workflows/speckit/specify and asserts 200.
func TestAPI_PackWorkflow_Get(t *testing.T) {
	ts := newTestServerWithPacks(t, apiFixtureDir(t), packFixtureDir(t))
	assertPackWiring(t, ts)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/workflows/speckit/specify", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			Name string `json:"name"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Body.Name, "response body should contain workflow data")
}

// TestAPI_PackWorkflow_LocalGet sends GET /api/workflows/local/api-simple-success and asserts 200.
func TestAPI_PackWorkflow_LocalGet(t *testing.T) {
	ts := newTestServerWithPacks(t, apiFixtureDir(t), packFixtureDir(t))
	assertPackWiring(t, ts)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/workflows/local/api-simple-success", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestAPI_PackWorkflow_Run sends POST /api/workflows/speckit/specify/run and polls until completed.
func TestAPI_PackWorkflow_Run(t *testing.T) {
	ts := newTestServerWithPacks(t, apiFixtureDir(t), packFixtureDir(t))
	assertPackWiring(t, ts)

	bodyBytes, err := json.Marshal(map[string]any{"inputs": map[string]any{}})
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/workflows/speckit/specify/run", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var runResult struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&runResult))
	executionID := runResult.Body.ExecutionID
	require.NotEmpty(t, executionID, "execution_id must be present in run response")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("pack workflow execution did not complete within timeout")
		case <-ticker.C:
			pollReq, pollErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
				ts.URL+"/api/executions/"+executionID, nil)
			require.NoError(t, pollErr)

			pollResp, pollErr := http.DefaultClient.Do(pollReq)
			require.NoError(t, pollErr)

			var pollResult struct {
				Body struct {
					Status string `json:"status"`
				} `json:"body"`
			}
			json.NewDecoder(pollResp.Body).Decode(&pollResult) //nolint:errcheck
			pollResp.Body.Close()

			if pollResult.Body.Status == "completed" || pollResult.Body.Status == "failed" {
				assert.Equal(t, "completed", pollResult.Body.Status, "pack workflow execution should reach completed status")
				return
			}
		}
	}
}

// TestAPI_PackWorkflow_Validate sends POST /api/workflows/speckit/specify/validate and asserts 200 with no errors.
func TestAPI_PackWorkflow_Validate(t *testing.T) {
	ts := newTestServerWithPacks(t, apiFixtureDir(t), packFixtureDir(t))
	assertPackWiring(t, ts)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/workflows/speckit/specify/validate", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			Errors []string `json:"errors"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Empty(t, result.Body.Errors, "valid workflow should produce no validation errors")
}

// TestAPI_PackWorkflow_SSE runs the speckit/specify workflow and asserts the SSE stream
// terminates with workflow.completed or workflow.failed before a 5-second deadline.
func TestAPI_PackWorkflow_SSE(t *testing.T) {
	ts := newTestServerWithPacks(t, apiFixtureDir(t), packFixtureDir(t))
	assertPackWiring(t, ts)

	bodyBytes, err := json.Marshal(map[string]any{"inputs": map[string]any{}})
	require.NoError(t, err)

	runReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/workflows/speckit/specify/run", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	runReq.Header.Set("Content-Type", "application/json")

	runResp, err := http.DefaultClient.Do(runReq)
	require.NoError(t, err)
	defer runResp.Body.Close()
	require.Equal(t, http.StatusAccepted, runResp.StatusCode)

	var runResult struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(runResp.Body).Decode(&runResult))
	executionID := runResult.Body.ExecutionID
	require.NotEmpty(t, executionID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sseReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		ts.URL+"/api/executions/"+executionID+"/events", nil)
	require.NoError(t, err)
	sseReq.Header.Set("Accept", "text/event-stream")

	sseResp, err := http.DefaultClient.Do(sseReq)
	require.NoError(t, err)
	defer sseResp.Body.Close()
	require.Equal(t, http.StatusOK, sseResp.StatusCode)

	var terminalEventType string
	scanner := bufio.NewScanner(sseResp.Body)
	var currentType string
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			t.Fatal("SSE stream did not emit a terminal event within 5-second deadline")
		default:
		}
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			currentType = strings.TrimPrefix(line, "event: ")
		case line == "" && currentType != "":
			if currentType == "workflow.completed" || currentType == "workflow.failed" {
				terminalEventType = currentType
				goto done
			}
			currentType = ""
		}
	}
done:
	assert.NotEmpty(t, terminalEventType, "SSE stream should have emitted at least one terminal event")
	assert.True(
		t,
		terminalEventType == "workflow.completed" || terminalEventType == "workflow.failed",
		"terminal SSE event type should be workflow.completed or workflow.failed, got: %s", terminalEventType,
	)
}
