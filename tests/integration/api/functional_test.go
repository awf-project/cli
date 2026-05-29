//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F097

// TestAPI_ListWorkflows_Integration validates that GET /api/workflows returns all available workflows.
func TestAPI_ListWorkflows_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/workflows", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			Workflows []struct {
				Name        string `json:"name"`
				Version     string `json:"version"`
				Description string `json:"description"`
			} `json:"workflows"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Body.Workflows, "should list available workflows")

	workflowNames := make([]string, len(result.Body.Workflows))
	for i, w := range result.Body.Workflows {
		workflowNames[i] = w.Name
	}
	assert.Contains(t, workflowNames, "api-simple-success", "fixture workflow should be discoverable")
}

// TestAPI_GetWorkflow_Integration validates that GET /api/workflows/{name} returns workflow details.
func TestAPI_GetWorkflow_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/workflows/local/api-simple-success", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "api-simple-success", result.Body.Name)
	assert.Equal(t, "1.0.0", result.Body.Version)
}

// TestAPI_GetWorkflow_NotFound_Integration validates that GET /api/workflows/{invalid} returns 404.
func TestAPI_GetWorkflow_NotFound_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/workflows/local/nonexistent-workflow", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "should return 404 for unknown workflow")
}

// TestAPI_ExecutionStatusPolling_Integration validates that GET /api/executions/{id} returns live execution status.
func TestAPI_ExecutionStatusPolling_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	executionID := postRunWorkflow(t, ts, "api-simple-success", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Poll for completion instead of using SSE.
	var finalStatus string
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Fatal("execution did not complete within timeout")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
				ts.URL+"/api/executions/"+executionID, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			var result struct {
				Body struct {
					Status string `json:"status"`
				} `json:"body"`
			}
			json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
			resp.Body.Close()

			finalStatus = result.Body.Status
			if finalStatus == "completed" || finalStatus == "failed" {
				assert.Equal(t, "completed", finalStatus, "execution should reach completed status")
				return
			}
		}
	}
}

// TestAPI_ListExecutions_Integration validates that GET /api/executions tracks active executions.
func TestAPI_ListExecutions_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	// Start an execution.
	id := postRunWorkflow(t, ts, "api-simple-success", nil)

	// List active executions immediately after starting.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/executions", nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			Executions []struct {
				ExecutionID  string `json:"execution_id"`
				WorkflowName string `json:"workflow_name"`
			} `json:"executions"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	execIDs := make([]string, len(result.Body.Executions))
	for i, e := range result.Body.Executions {
		execIDs[i] = e.ExecutionID
	}

	// Started execution should be in the active list.
	assert.Contains(t, execIDs, id, "execution should be in active list after start")
}

// TestAPI_RunWorkflow_WithInputs_Integration validates that workflow inputs are accepted and propagated.
func TestAPI_RunWorkflow_WithInputs_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	inputs := map[string]any{
		"test_input": "test_value",
		"number":     42,
	}

	executionID := postRunWorkflow(t, ts, "api-simple-success", inputs)
	require.NotEmpty(t, executionID, "execution should be created with inputs")

	// Verify execution is tracked.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		ts.URL+"/api/executions/"+executionID, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		Body struct {
			ExecutionID  string `json:"execution_id"`
			WorkflowName string `json:"workflow_name"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, executionID, result.Body.ExecutionID)
	assert.Equal(t, "api-simple-success", result.Body.WorkflowName)
}

// TestAPI_RunWorkflow_InvalidInputs_Integration validates that POST with missing required inputs returns 422.
func TestAPI_RunWorkflow_InvalidInputs_Integration(t *testing.T) {
	ts, _, _ := newTestServer(t, apiFixtureDir(t))

	// POST with invalid body structure (missing inputs object).
	invalidBody := map[string]any{"invalid": "structure"}
	bodyBytes, err := json.Marshal(invalidBody)
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/workflows/local/api-simple-success/run", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Huma validates the schema and returns 422 for malformed input.
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"should reject invalid input structure")
}
