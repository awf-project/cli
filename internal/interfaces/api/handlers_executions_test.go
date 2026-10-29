package api

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/builders"
	"github.com/awf-project/cli/internal/testutil/facadetest"
)

// ctxCaptureFacade records the context handed to Run/Resume so a test can assert the
// execution context is detached from the (soon-cancelled) HTTP request context.
type ctxCaptureFacade struct {
	*facadetest.Fake
	mu       sync.Mutex
	captured context.Context //nolint:containedctx // captured solely for assertion in a regression test
}

func (c *ctxCaptureFacade) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	c.mu.Lock()
	c.captured = ctx
	c.mu.Unlock()
	return c.Fake.Run(ctx, req)
}

func (c *ctxCaptureFacade) capturedCtx() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.captured
}

// TestExecutionHandler_Run_DetachesExecutionFromRequestCtx guards the regression where the
// handler passed the HTTP request context straight to facade.Run: huma cancels that context
// the instant the 202 response is written, killing the workflow before it executed (nothing
// reached history). The execution context must survive request cancellation.
func TestExecutionHandler_Run_DetachesExecutionFromRequestCtx(t *testing.T) {
	capFacade := &ctxCaptureFacade{Fake: facadetest.New().WithTerminalCompleted()}
	bridge := NewBridge()
	h := NewExecutionHandlers(bridge)
	h.SetFacade(capFacade)
	h.SetSessionRegistry(application.NewSessionRegistry())

	reqCtx, cancel := context.WithCancel(context.Background())
	_, err := h.Run(reqCtx, &RunWorkflowInput{Scope: "local", Name: "echo"})
	require.NoError(t, err)

	// The request completes: huma cancels the request context.
	cancel()

	captured := capFacade.capturedCtx()
	require.NotNil(t, captured, "facade.Run must have been called")
	assert.NoError(t, captured.Err(),
		"execution context must survive request cancellation (detached via context.WithoutCancel)")
}

// errFacade is a minimal ports.WorkflowFacade stub that returns a fixed error
// from Run and/or Resume. Used for error-path handler tests.
type errFacade struct {
	runErr    error
	resumeErr error
}

func (e *errFacade) List(_ context.Context) ([]ports.WorkflowSummary, error) {
	return nil, nil
}

func (e *errFacade) Validate(_ context.Context, _ ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{}, nil
}

func (e *errFacade) Status(_ context.Context, _ string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

func (e *errFacade) History(_ context.Context, _ ports.HistoryFilter) ([]ports.RunRecord, error) { //nolint:gocritic // hugeParam: satisfies ports.WorkflowFacade contract
	return nil, nil
}

func (e *errFacade) Run(_ context.Context, _ ports.RunRequest) (ports.RunSession, error) {
	return nil, e.runErr
}

func (e *errFacade) Resume(_ context.Context, _ ports.ResumeRequest) (ports.RunSession, error) {
	return nil, e.resumeErr
}

func (e *errFacade) RunStep(_ context.Context, _ ports.RunStepRequest) (ports.StepResult, error) {
	return ports.StepResult{}, nil
}

// newExecutionHandlerAPI wires a Bridge + ExecutionHandlers + humatest API
// WITHOUT a facade — for tests that only exercise List/Get/Cancel seeded via seedExecution.
func newExecutionHandlerAPI(t *testing.T, _ ...string) (humatest.TestAPI, *Bridge) {
	t.Helper()
	bridge := NewBridge()
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)
	return api, bridge
}

// newFacadeExecutionHandlerAPI wires a Bridge + ExecutionHandlers with a facade and
// SessionRegistry so the facade path (Run/Resume) is exercised end-to-end.
func newFacadeExecutionHandlerAPI(t *testing.T, facade ports.WorkflowFacade, _ ...string) (humatest.TestAPI, *Bridge, *application.SessionRegistry) {
	t.Helper()
	bridge := NewBridge()
	reg := application.NewSessionRegistry()
	handler := NewExecutionHandlers(bridge)
	handler.SetFacade(facade)
	handler.SetSessionRegistry(reg)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)
	return api, bridge, reg
}

// --- Run tests (facade path) ---

func TestExecutionHandler_Run_Returns202WithExecutionID(t *testing.T) {
	fake := facadetest.New().WithTerminalCompleted()
	api, _, _ := newFacadeExecutionHandlerAPI(t, fake, "deploy-prod")

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{"env": "prod"}}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	require.Equal(t, 202, resp.Code, "Run must return 202 Accepted for async execution")

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	assert.NotEmpty(t, result.Body.ExecutionID, "execution_id must be non-empty")
	assert.Equal(t, "accepted", result.Body.Status, "status must be 'accepted'")
}

func TestExecutionHandler_Run_NoFacade_Returns503(t *testing.T) {
	// When no facade is wired, Run must return 503 Service Unavailable — not 404 and not panic.
	// 503 signals that the service is temporarily unable to handle the request (facade not configured),
	// rather than 422 Unprocessable Entity which would imply a client error.
	api, _ := newExecutionHandlerAPI(t, "deploy-prod")

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{}}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	assert.Equal(t, 503, resp.Code, "Run without facade must return 503 Service Unavailable")
}

func TestExecutionHandler_Run_UnknownWorkflow_Returns404(t *testing.T) {
	// Facade returns ErrRunNotFound for an unknown identifier → handler maps to 404.
	api, _, _ := newFacadeExecutionHandlerAPI(t, &errFacade{runErr: ports.ErrRunNotFound}, "nonexistent")

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{}}

	resp := api.Post("/api/workflows/local/nonexistent/run", input)
	assert.Equal(t, 404, resp.Code, "Run with unknown workflow must return 404 Not Found")
}

func TestExecutionHandler_Run_FacadePath_BridgeTracksMetadata(t *testing.T) {
	fake := facadetest.New().WithTerminalCompleted()
	api, bridge, _ := newFacadeExecutionHandlerAPI(t, fake, "deploy-prod")

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{}}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	require.Equal(t, 202, resp.Code)

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Body.ExecutionID)

	ae, ok := bridge.GetExecution(result.Body.ExecutionID)
	assert.True(t, ok, "Bridge.activeExecutions must track facade session for List/Get/Cancel")
	if assert.NotNil(t, ae) {
		assert.Equal(t, "deploy-prod", ae.WorkflowName)
	}
}

// --- List tests ---

func TestExecutionHandler_List_HappyPath(t *testing.T) {
	bridge := NewBridge()

	// Seed two executions directly via the white-box helper.
	seedExecutionWithID(bridge, "id-exec-a", "wf-a")
	seedExecutionWithID(bridge, "id-exec-b", "wf-b")

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
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Len(t, result.Body.Executions, 2, "List must return both executions")
	workflowNames := make([]string, len(result.Body.Executions))
	for i, e := range result.Body.Executions {
		workflowNames[i] = e.WorkflowName
	}
	assert.ElementsMatch(t, []string{"wf-a", "wf-b"}, workflowNames)
}

// --- Get tests ---

func TestExecutionHandler_Get_HappyPath(t *testing.T) {
	bridge := NewBridge()
	id, _ := seedExecution(bridge, "test-workflow")

	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	resp := api.Get("/api/executions/" + id)
	require.Equal(t, 200, resp.Code, "Get with valid execution ID must return 200")

	var result struct {
		Body struct {
			ExecutionID  string `json:"execution_id"`
			WorkflowName string `json:"workflow_name"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, id, result.Body.ExecutionID, "execution_id in response must match requested ID")
	assert.Equal(t, "test-workflow", result.Body.WorkflowName)
}

func TestExecutionHandler_Get_NotFound_Returns404(t *testing.T) {
	bridge := NewBridge()
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	resp := api.Get("/api/executions/does-not-exist")

	assert.Equal(t, 404, resp.Code, "Get with unknown execution ID must return 404 Not Found")
}

// --- Cancel tests ---

func TestExecutionHandler_Cancel_PropagatesContextCancellation(t *testing.T) {
	harness := builders.NewCancellationHarness("test-workflow")
	adapter, executor, logger := harness.Adapter, harness.Executor, harness.Logger
	api, bridge, registry := newFacadeExecutionHandlerAPI(t, adapter)
	const (
		secretInput  = "input-must-not-appear"
		secretPrompt = "prompt-must-not-appear"
		secretToken  = "token-must-not-appear"
	)

	runResp := api.Post("/api/workflows/local/test-workflow/run", struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{
		"operation_id": "cancel-propagation",
		"input":        secretInput,
		"prompt":       secretPrompt,
		"token":        secretToken,
	}})
	require.Equal(t, 202, runResp.Code)

	var runResult struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(runResp.Body).Decode(&runResult))
	require.NotEmpty(t, runResult.Body.ExecutionID)
	exec, ok := bridge.GetExecution(runResult.Body.ExecutionID)
	require.True(t, ok)
	assert.NoError(t, exec.Ctx.Err(), "context must not be cancelled before cancel handler")
	select {
	case <-executor.Started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the real adapter to enter active command execution")
	}

	resp := api.Delete("/api/executions/"+runResult.Body.ExecutionID, struct {
		ID string `path:"id"`
	}{ID: runResult.Body.ExecutionID})

	require.Equal(t, 204, resp.Code, "Cancel must return 204 No Content")
	assert.ErrorIs(t, exec.Ctx.Err(), context.Canceled)
	select {
	case err := <-executor.Canceled:
		assert.ErrorIs(t, err, context.Canceled,
			"DELETE must cancel the context owned by active application work")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cancellation to reach active application work")
	}

	session, ok := registry.Get(runResult.Body.ExecutionID)
	require.True(t, ok, "the tracked execution must retain its session for terminal diagnostics")
	terminalEvents := make([]ports.Event, 0, 1)
	cleanupDeadline := time.NewTimer(time.Second)
	defer cleanupDeadline.Stop()
eventLoop:
	for {
		select {
		case event, open := <-session.Events():
			if !open {
				break eventLoop
			}
			if event.Kind == ports.EventWorkflowCompleted || event.Kind == ports.EventWorkflowFailed {
				terminalEvents = append(terminalEvents, event)
			}
		case <-cleanupDeadline.C:
			t.Fatal("timed out waiting for cancellation cleanup to close the event stream")
		}
	}
	require.Len(t, terminalEvents, 1, "cancellation cleanup must publish exactly one terminal outcome")
	assert.Equal(t, ports.EventWorkflowFailed, terminalEvents[0].Kind)
	assert.Equal(t, runResult.Body.ExecutionID, terminalEvents[0].RunID,
		"terminal diagnostics must identify the cancelled operation")
	terminal, ok := terminalEvents[0].Payload.(*ports.EnrichedTerminal)
	require.True(t, ok, "cancellation terminal event must carry EnrichedTerminal diagnostics")
	assert.Equal(t, "cancelled", terminal.Error)

	diagnostic, err := json.Marshal(terminalEvents[0])
	require.NoError(t, err)
	text := string(diagnostic)
	assert.Contains(t, text, runResult.Body.ExecutionID)
	for _, forbidden := range []string{secretInput, secretPrompt, secretToken, "operation-awaiting-input"} {
		assert.NotContains(t, text, forbidden, "terminal diagnostics must not disclose request data")
	}
	assert.NotContains(t, strings.ToLower(text), "authorization")

	capturedDiagnostics := text + "\n" + logger.Snapshot()
	assert.NotEmpty(t, logger.Snapshot(), "the real cancellation path must emit observable logs")
	for _, forbidden := range []string{
		secretInput, secretPrompt, secretToken,
		"inputs", "prompt", "environment", "request", "header", "plugin", "authorization", "credential",
	} {
		assert.NotContains(t, strings.ToLower(capturedDiagnostics), strings.ToLower(forbidden),
			"captured errors and logs must not disclose request or secret data")
	}
}

func TestExecutionHandler_Cancel_Idempotent_TwoDELETEsBothReturn204(t *testing.T) {
	bridge := NewBridge()
	id, _ := seedExecution(bridge, "test-workflow")

	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		ID string `path:"id"`
	}{ID: id}
	resp1 := api.Delete("/api/executions/"+id, input)
	require.Equal(t, 204, resp1.Code, "First DELETE must return 204")

	resp2 := api.Delete("/api/executions/"+id, input)
	require.Equal(t, 204, resp2.Code, "Second DELETE must also return 204 (idempotent)")
}

func TestExecutionHandler_Cancel_UnknownID_Returns204(t *testing.T) {
	// Cancel is idempotent — unknown IDs also return 204.
	bridge := NewBridge()
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		ID string `path:"id"`
	}{ID: "does-not-exist"}
	resp := api.Delete("/api/executions/does-not-exist", input)

	assert.Equal(t, 204, resp.Code, "Cancel with unknown ID must return 204 (idempotent)")
}

// --- Resume tests (facade path) ---

func TestExecutionHandler_Resume_NoFacade_Returns503(t *testing.T) {
	// When no facade is wired, Resume must return 503 Service Unavailable — not 422.
	// 503 signals that the service is temporarily unable to handle the request (facade not configured).
	bridge := NewBridge()
	handler := NewExecutionHandlers(bridge)
	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty"`
		FromStep       string         `json:"from_step,omitempty"`
	}{}

	resp := api.Post("/api/executions/some-id/resume", input)
	assert.Equal(t, 503, resp.Code, "Resume without facade must return 503 Service Unavailable")
}

func TestExecutionHandler_Resume_NotFound_Returns404(t *testing.T) {
	// Facade returns ErrRunNotFound → handler maps to 404.
	api, _, _ := newFacadeExecutionHandlerAPI(t, &errFacade{resumeErr: ports.ErrRunNotFound}, "test-workflow")

	input := struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty"`
		FromStep       string         `json:"from_step,omitempty"`
	}{}

	resp := api.Post("/api/executions/missing-id/resume", input)
	assert.Equal(t, 404, resp.Code, "Resume with not-found execution must return 404")
}

func TestExecutionHandler_Resume_InternalError_Returns422(t *testing.T) {
	// Facade returns a non-not-found error → handler maps to 422.
	api, _, _ := newFacadeExecutionHandlerAPI(t, &errFacade{resumeErr: ports.ErrSessionClosed}, "test-workflow")

	input := struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty"`
		FromStep       string         `json:"from_step,omitempty"`
	}{}

	resp := api.Post("/api/executions/some-id/resume", input)
	assert.Equal(t, 422, resp.Code, "Resume with completed/invalid state must return 422, not 404")
}

// --- T073: facade run path + SessionRegistry wiring ---

// TestExecutionHandlersRun_RegistersSessionForSSE verifies FR-003: when
// ExecutionHandlers has a facade and registry wired, POST /run must call
// facade.Run, register the returned RunSession in the SessionRegistry
// synchronously (before returning the HTTP response), and return an execution_id
// that resolves via SessionRegistry.Get.
func TestExecutionHandlersRun_RegistersSessionForSSE(t *testing.T) {
	fake := facadetest.New().WithTerminalCompleted()
	reg := application.NewSessionRegistry()
	bridge := NewBridge()
	handler := NewExecutionHandlers(bridge)
	handler.SetFacade(fake)
	handler.SetSessionRegistry(reg)

	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{}}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	require.Equal(t, 202, resp.Code)

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
			Status      string `json:"status"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Body.ExecutionID, "execution_id must be non-empty")
	assert.Equal(t, "accepted", result.Body.Status)

	// Core acceptance criterion: session is findable in the registry by execution ID.
	session, ok := reg.Get(result.Body.ExecutionID)
	assert.True(t, ok, "SessionRegistry.Get(execID) must return non-nil after Run (FR-003)")
	assert.NotNil(t, session)
}

// TestExecutionHandlersRun_FacadePath_BridgeTracksMetadata verifies that
// Bridge.activeExecutions still holds a metadata entry for the facade-driven
// execution so that List/Get/Cancel handlers keep working.
func TestExecutionHandlersRun_FacadePath_BridgeTracksMetadata(t *testing.T) {
	fake := facadetest.New().WithTerminalCompleted()
	reg := application.NewSessionRegistry()
	bridge := NewBridge()
	handler := NewExecutionHandlers(bridge)
	handler.SetFacade(fake)
	handler.SetSessionRegistry(reg)

	_, api := humatest.New(t)
	RegisterExecutionRoutes(api, handler)

	input := struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{}}

	resp := api.Post("/api/workflows/local/deploy-prod/run", input)
	require.Equal(t, 202, resp.Code)

	var result struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.NotEmpty(t, result.Body.ExecutionID)

	ae, ok := bridge.GetExecution(result.Body.ExecutionID)
	assert.True(t, ok, "Bridge.activeExecutions must track facade session for List/Get/Cancel")
	if assert.NotNil(t, ae) {
		assert.Equal(t, "deploy-prod", ae.WorkflowName)
	}
}

// TestExecutionHandler_Get_FacadeSession_ReflectsStatus guards limitation #1: a facade
// session carries no ExecutionContext (NopRecorder), so GET used to report an empty status
// and a zero started_at. The handler now stamps StartedAt at track time and overlays the
// live status snapshot, so GET reflects the real run state.
func TestExecutionHandler_Get_FacadeSession_ReflectsStatus(t *testing.T) {
	fake := facadetest.New().WithTerminalCompleted()
	api, _, _ := newFacadeExecutionHandlerAPI(t, fake, "echo")

	postResp := api.Post("/api/workflows/local/echo/run", struct {
		Inputs map[string]any `json:"inputs"`
	}{Inputs: map[string]any{}})
	require.Equal(t, 202, postResp.Code)

	var run struct {
		Body struct {
			ExecutionID string `json:"execution_id"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(postResp.Body).Decode(&run))
	require.NotEmpty(t, run.Body.ExecutionID)

	getResp := api.Get("/api/executions/" + run.Body.ExecutionID)
	require.Equal(t, 200, getResp.Code)

	var got struct {
		Body struct {
			Status    string `json:"status"`
			StartedAt string `json:"started_at"`
		} `json:"body"`
	}
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&got))

	assert.Equal(t, "completed", got.Body.Status,
		"facade session GET must reflect the terminal status, not an empty string")
	assert.NotEqual(t, "0001-01-01T00:00:00Z", got.Body.StartedAt,
		"facade session GET must carry a non-zero started_at stamped at track time")
}
