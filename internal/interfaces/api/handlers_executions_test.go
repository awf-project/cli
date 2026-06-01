package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// mockWorkflowResumer implements WorkflowResumer for testing.
type mockWorkflowResumer struct {
	resumeErr error
	execCtx   *workflow.ExecutionContext
}

func newMockWorkflowResumer() *mockWorkflowResumer {
	return &mockWorkflowResumer{
		execCtx: workflow.NewExecutionContext("resumed-exec", "test-workflow"),
	}
}

func (m *mockWorkflowResumer) Resume(
	_ context.Context,
	_ string,
	_ map[string]any,
	_ string,
) (*workflow.ExecutionContext, error) {
	if m.resumeErr != nil {
		return nil, m.resumeErr
	}
	return m.execCtx, nil
}

// newBlockingExecutionHandlerAPI wires a full execution-handler test stack with a
// blocking runner. The runner's Done channel stays open until test cleanup, which
// prevents Bridge's cleanup goroutine in StartExecution from removing tracked
// executions before assertions run. Returns the api (for HTTP calls), the bridge
// (for GetExecution/ListExecutions assertions), and the lister (so callers can
// mutate getErr / entries fields before issuing requests).
func newBlockingExecutionHandlerAPI(t *testing.T, workflowNames ...string) (humatest.TestAPI, *Bridge, *mockWorkflowLister) {
	t.Helper()
	block := make(chan error)
	t.Cleanup(func() { close(block) })
	lister := newMockWorkflowLister(workflowNames...)
	runner := newMockWorkflowRunnerWithDone(block)
	bridge := NewBridge(lister, runner, newMockHistoryProvider())
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)
	return api, bridge, lister
}

// --- Tests ---

func TestExecutionHandler_Run_Returns202WithExecutionID_WithinDeadline(t *testing.T) {
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "deploy-prod")

	// Verify the response is built and returned BEFORE the async work completes.
	// FR-006 deadline: 100ms from request receipt.
	startTime := time.Now()
	timeout := time.AfterFunc(100*time.Millisecond, func() {
		t.Fatal("FR-006 deadline exceeded: Run handler did not return within 100ms")
	})
	defer timeout.Stop()

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{
		Inputs: map[string]any{"env": "prod"},
	}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	elapsed := time.Since(startTime)

	timeout.Stop()

	require.Equal(t, 202, resp.Code, "Run must return 202 Accepted for async execution")

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Body.ExecutionID, "execution_id must be non-empty")
	assert.Equal(t, "accepted", result.Body.Status, "status must be 'accepted'")
	assert.Less(t, elapsed, 100*time.Millisecond, "handler must return within FR-006 deadline")

	stored, ok := bridge.GetExecution(result.Body.ExecutionID)
	assert.True(t, ok, "execution must be stored in Bridge")
	require.NotNil(t, stored)
	assert.Equal(t, "deploy-prod", stored.WorkflowName)
}

func TestExecutionHandler_Run_UnknownWorkflow_Returns404(t *testing.T) {
	lister := newMockWorkflowLister()
	lister.getErr = errors.New("workflow not found")
	runner := newMockWorkflowRunner()
	bridge := NewBridge(lister, runner, newMockHistoryProvider())
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{
		Inputs: map[string]any{},
	}

	resp := api.Post("/api/workflows/local/nonexistent/run", input)

	assert.Equal(t, 404, resp.Code, "Run with unknown workflow must return 404 Not Found")
}

func TestExecutionHandler_List_HappyPath(t *testing.T) {
	// Blocking channels prevent cleanup goroutine from removing entries.
	blockA := make(chan error)
	blockB := make(chan error)
	t.Cleanup(func() { close(blockA); close(blockB) })

	runner := &mockWorkflowRunner{}
	bridge := NewBridge(newMockWorkflowLister("wf-a", "wf-b"), runner, newMockHistoryProvider())

	// Start two executions.
	wfA := &workflow.Workflow{Name: "wf-a", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	runner.done = blockA
	_, _, err := bridge.StartExecution(context.Background(), wfA, nil)
	require.NoError(t, err)

	wfB := &workflow.Workflow{Name: "wf-b", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	runner.done = blockB
	_, _, err = bridge.StartExecution(context.Background(), wfB, nil)
	require.NoError(t, err)

	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	resp := api.Get("/api/executions")
	require.Equal(t, 200, resp.Code)

	var result struct {
		Body struct {
			Executions []struct {
				ExecutionID  string `json:"execution_id"`
				WorkflowName string `json:"workflow_name"`
			} `json:"executions"`
		} `json:"body"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Len(t, result.Body.Executions, 2, "List must return both executions")
	workflowNames := make([]string, len(result.Body.Executions))
	for i, e := range result.Body.Executions {
		workflowNames[i] = e.WorkflowName
	}
	assert.ElementsMatch(t, []string{"wf-a", "wf-b"}, workflowNames)
}

func TestExecutionHandler_Get_HappyPath(t *testing.T) {
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "test-workflow")

	// Start an execution to get a valid ID.
	wf := &workflow.Workflow{Name: "test-workflow", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	id, _, err := bridge.StartExecution(context.Background(), wf, nil)
	require.NoError(t, err)

	resp := api.Get("/api/executions/" + id)
	require.Equal(t, 200, resp.Code, "Get with valid execution ID must return 200")

	var result struct {
		Body struct {
			ExecutionID  string `json:"execution_id"`
			WorkflowName string `json:"workflow_name"`
		} `json:"body"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, id, result.Body.ExecutionID, "execution_id in response must match requested ID")
	assert.Equal(t, "test-workflow", result.Body.WorkflowName)
}

func TestExecutionHandler_Get_NotFound_Returns404(t *testing.T) {
	lister := newMockWorkflowLister()
	runner := newMockWorkflowRunner()
	bridge := NewBridge(lister, runner, newMockHistoryProvider())
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	resp := api.Get("/api/executions/does-not-exist")

	assert.Equal(t, 404, resp.Code, "Get with unknown execution ID must return 404 Not Found")
}

func TestExecutionHandler_Cancel_PropagatesContextCancellation(t *testing.T) {
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "test-workflow")

	// Start an execution.
	wf := &workflow.Workflow{Name: "test-workflow", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	id, exec, err := bridge.StartExecution(context.Background(), wf, nil)
	require.NoError(t, err)

	// Verify context is not yet cancelled.
	assert.NoError(t, exec.Ctx.Err(), "context must not be cancelled before cancel handler")

	// Call the cancel handler.
	input := struct {
		ID string `path:"id"`
	}{ID: id}
	resp := api.Delete("/api/executions/"+id, input)

	// Assert 204 No Content (idempotent).
	require.Equal(t, 204, resp.Code, "Cancel must return 204 No Content")

	// Verify context is now cancelled.
	assert.Error(t, exec.Ctx.Err(), "context must be cancelled after cancel handler")
	assert.ErrorIs(t, exec.Ctx.Err(), context.Canceled)
}

func TestExecutionHandler_Cancel_Idempotent_TwoDELETEsBothReturn204(t *testing.T) {
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "test-workflow")

	// Start an execution.
	wf := &workflow.Workflow{Name: "test-workflow", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	id, _, err := bridge.StartExecution(context.Background(), wf, nil)
	require.NoError(t, err)

	// First DELETE.
	input := struct {
		ID string `path:"id"`
	}{ID: id}
	resp1 := api.Delete("/api/executions/"+id, input)
	require.Equal(t, 204, resp1.Code, "First DELETE must return 204")

	// Second DELETE (idempotent).
	resp2 := api.Delete("/api/executions/"+id, input)
	require.Equal(t, 204, resp2.Code, "Second DELETE must also return 204 (idempotent)")
}

func TestExecutionHandler_Cancel_UnknownID_Returns204(t *testing.T) {
	// Edge case spec line 108: Cancel is idempotent — unknown IDs also return 204.
	lister := newMockWorkflowLister()
	runner := newMockWorkflowRunner()
	bridge := NewBridge(lister, runner, newMockHistoryProvider())
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	// Try to cancel a non-existent execution.
	input := struct {
		ID string `path:"id"`
	}{ID: "does-not-exist"}
	resp := api.Delete("/api/executions/does-not-exist", input)

	// Assert 204 No Content (idempotent DELETE semantics).
	assert.Equal(t, 204, resp.Code, "Cancel with unknown ID must return 204 (idempotent)")
}

func TestExecutionHandler_Run_PackScope_Returns202_CanonicalNamePassed(t *testing.T) {
	api, _, lister := newBlockingExecutionHandlerAPI(t, "speckit/specify")

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{
		Inputs: map[string]any{},
	}

	resp := api.Post("/api/workflows/speckit/specify/run", input)
	require.Equal(t, 202, resp.Code)

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Body.ExecutionID)
	assert.Equal(t, "accepted", result.Body.Status)
	assert.Equal(t, "speckit/specify", lister.lastGetName, "mock must receive canonical name for pack scope")
}

func TestExecutionHandler_Run_LocalScope_Returns202_NameOnlyPassed(t *testing.T) {
	api, _, lister := newBlockingExecutionHandlerAPI(t, "deploy-prod")

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{
		Inputs: map[string]any{},
	}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	require.Equal(t, 202, resp.Code)

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Body.ExecutionID)
	assert.Equal(t, "accepted", result.Body.Status)
	assert.Equal(t, "deploy-prod", lister.lastGetName, "mock must receive name only for local scope")
}

func TestExecutionHandler_Run_UnknownScope_Returns404(t *testing.T) {
	// This test asserts the 404 response when the workflow lookup fails, so no
	// execution is ever started or tracked. We intentionally bypass
	// newBlockingExecutionHandlerAPI (which wires a blocking runner to keep
	// executions observable) and use the plain non-blocking mock runner.
	lister := newMockWorkflowLister()
	runner := newMockWorkflowRunner()
	bridge := NewBridge(lister, runner, newMockHistoryProvider())
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{
		Inputs: map[string]any{},
	}

	resp := api.Post("/api/workflows/unknown/foo/run", input)

	assert.Equal(t, 404, resp.Code, "Run with unknown scope must return 404 Not Found")
}

func TestExecutionHandler_Resume_NotFound_Returns404(t *testing.T) {
	// M5b: Resume must return 404 when the execution record does not exist.
	// The handler must use errors.Is(err, application.ErrExecutionNotFound) —
	// NOT a string-match — so that future message rewording stays correct.
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "test-workflow")

	resumer := newMockWorkflowResumer()
	// Wrap the sentinel the same way ExecutionService.Resume does.
	resumer.resumeErr = fmt.Errorf("workflow execution not found: missing-id: %w", application.ErrExecutionNotFound)
	bridge.SetResumer(resumer)

	input := struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty"`
		FromStep       string         `json:"from_step,omitempty"`
	}{}

	resp := api.Post("/api/executions/missing-id/resume", input)
	assert.Equal(t, 404, resp.Code, "Resume with not-found execution must return 404")
}

func TestExecutionHandler_Resume_InternalError_Returns422NotExposingDetails(t *testing.T) {
	// M5b: Resume errors that are not "not found" (e.g. already completed,
	// workflow load failure) must return 422 Unprocessable Entity, not 404.
	// The raw internal error string must not be forwarded verbatim.
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "test-workflow")

	resumer := newMockWorkflowResumer()
	resumer.resumeErr = errors.New("workflow already completed, cannot resume")
	bridge.SetResumer(resumer)

	input := struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty"`
		FromStep       string         `json:"from_step,omitempty"`
	}{}

	resp := api.Post("/api/executions/some-id/resume", input)
	assert.Equal(t, 422, resp.Code, "Resume with completed/invalid state must return 422, not 404")
}

func TestExecutionHandler_Resume_FailedExecution_RestartsFromFailedStep(t *testing.T) {
	// Setup: execution stored in Bridge, resumer mocked.
	api, bridge, _ := newBlockingExecutionHandlerAPI(t, "test-workflow")

	// Wire the resumer.
	resumer := newMockWorkflowResumer()
	bridge.SetResumer(resumer)

	// Start an execution (represents the failed run).
	wf := &workflow.Workflow{Name: "test-workflow", Steps: map[string]*workflow.Step{"s1": {Name: "s1"}}}
	failedID, _, err := bridge.StartExecution(context.Background(), wf, nil)
	require.NoError(t, err)

	// Call the resume handler.
	input := struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty"`
		FromStep       string         `json:"from_step,omitempty"`
	}{
		InputOverrides: map[string]any{"retry": true},
		FromStep:       "build",
	}

	resp := api.Post("/api/executions/"+failedID+"/resume", input)
	require.Equal(t, 200, resp.Code, "Resume must return 200 OK with new RunWorkflowOutput")

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Body.ExecutionID, "Resume must return a new execution ID")
	assert.Equal(t, "accepted", result.Body.Status)
}
