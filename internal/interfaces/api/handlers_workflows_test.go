package api

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// newWorkflowHandlerAPI wires a read facade (backed by the given mock lister) + WorkflowHandlers
// + humatest API and returns the API for assertions. The handlers consume the facade/reader
// ports (F108), so the mock lister is adapted via readFacade rather than a Bridge.
func newWorkflowHandlerAPI(t *testing.T, lister *mockWorkflowLister) humatest.TestAPI {
	t.Helper()
	rf := &readFacade{lister: lister, history: newMockHistoryProvider()}
	handler := NewWorkflowHandlers(rf, rf)
	_, api := humatest.New(t)
	RegisterWorkflowRoutes(api, handler)
	return api
}

func TestWorkflowHandler_List_HappyPath(t *testing.T) {
	mock := newMockWorkflowLister("deploy-prod", "test-service")
	mock.entries[0].Version = "1.0.0"
	mock.entries[0].Description = "Deploy to production"
	mock.entries[1].Version = "2.0.0"
	mock.entries[1].Description = "Run tests"

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows")
	require.Equal(t, 200, resp.Code)

	var result struct {
		Body struct {
			Workflows []WorkflowSummary `json:"workflows"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Len(t, result.Body.Workflows, 2)
	assert.Equal(t, "deploy-prod", result.Body.Workflows[0].Name)
	assert.Equal(t, "1.0.0", result.Body.Workflows[0].Version)
	assert.Equal(t, "Deploy to production", result.Body.Workflows[0].Description)
	assert.Equal(t, "local", result.Body.Workflows[0].Scope)
	assert.Equal(t, "deploy-prod", result.Body.Workflows[0].Workflow)
}

func TestWorkflowHandler_Get_NotFound_Returns404(t *testing.T) {
	// GetWorkflow must return 404 only when the workflow file genuinely does not
	// exist, i.e. a StructuredError with ErrorCodeUserInputMissingFile.
	mock := newMockWorkflowLister()
	mock.getErr = domainerrors.NewUserError(
		domainerrors.ErrorCodeUserInputMissingFile,
		"workflow not found: nonexistent",
		nil, nil,
	)

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows/local/nonexistent")
	assert.Equal(t, 404, resp.Code)
}

func TestWorkflowHandler_Get_InternalError_Returns500WithoutInternalMessage(t *testing.T) {
	// An internal error (YAML parse failure, permission error, …) must not be
	// mapped to 404 — that would hide the root cause. It must be 500, and the
	// raw internal error string must NOT be forwarded to the client.
	mock := newMockWorkflowLister()
	mock.getErr = errors.New("yaml: unmarshal errors: field unknown not found in type workflow.Workflow")

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows/local/broken-workflow")
	assert.Equal(t, 500, resp.Code, "internal get error must return 500, not 404")

	body := resp.Body.String()
	assert.NotContains(t, body, "yaml", "internal error details must not leak to client")
	assert.NotContains(t, body, "unmarshal", "internal error details must not leak to client")
}

func TestWorkflowHandler_Validate_InvalidWorkflow_Returns422(t *testing.T) {
	// M-5: a workflow that fails validation must return 422 Unprocessable Entity,
	// not 200. This distinguishes a structurally invalid workflow from a successful
	// validation that found no errors.
	mock := newMockWorkflowLister("bad-workflow")
	mock.validErr = errors.New("invalid step reference")

	api := newWorkflowHandlerAPI(t, mock)

	validateInput := struct {
		Body struct {
			Inputs map[string]any `json:"inputs"`
		} `json:"body"`
	}{}

	resp := api.Post("/api/workflows/local/bad-workflow/validate", validateInput)
	assert.Equal(t, 422, resp.Code, "validation failure must return 422, not 200")
}

func TestWorkflowHandler_List_EmptyList(t *testing.T) {
	mock := newMockWorkflowLister()

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows")
	require.Equal(t, 200, resp.Code)

	var result struct {
		Body struct {
			Workflows []WorkflowSummary `json:"workflows"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Empty(t, result.Body.Workflows)
}

func TestWorkflowHandler_Get_FoundWorkflow_ReturnsWorkflow(t *testing.T) {
	mock := newMockWorkflowLister("test-workflow")

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows/local/test-workflow")
	require.Equal(t, 200, resp.Code)

	var result struct {
		Body *workflow.Workflow `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotNil(t, result.Body)
	assert.Equal(t, "test-workflow", result.Body.Name)
}

func TestWorkflowHandler_Validate_ValidWorkflow_Returns200(t *testing.T) {
	// A workflow that passes validation returns 200 OK (no errors).
	mock := newMockWorkflowLister("valid-workflow")
	// validErr defaults to nil, which means validation passed

	api := newWorkflowHandlerAPI(t, mock)

	validateInput := struct {
		Body struct {
			Inputs map[string]any `json:"inputs"`
		} `json:"body"`
	}{}

	resp := api.Post("/api/workflows/local/valid-workflow/validate", validateInput)
	assert.Equal(t, 200, resp.Code)
}

func TestWorkflowHandler_Get_LocalScope_PassesNameOnly(t *testing.T) {
	mock := newMockWorkflowLister("deploy-prod")

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows/local/deploy-prod")
	require.Equal(t, 200, resp.Code)
	assert.Equal(t, "deploy-prod", mock.lastGetName)
}

func TestWorkflowHandler_Get_PackScope_PassesScopeSlashName(t *testing.T) {
	mock := newMockWorkflowLister("speckit/specify")

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows/speckit/specify")
	require.Equal(t, 200, resp.Code)
	assert.Equal(t, "speckit/specify", mock.lastGetName)
}

func TestWorkflowHandler_Get_UnknownWorkflow_Returns404(t *testing.T) {
	mock := newMockWorkflowLister()

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows/unknown/foo")
	assert.Equal(t, 404, resp.Code)
}

func TestWorkflowHandler_Validate_PackScope_Returns200(t *testing.T) {
	mock := newMockWorkflowLister("speckit/specify")

	api := newWorkflowHandlerAPI(t, mock)

	validateInput := struct {
		Body struct {
			Inputs map[string]any `json:"inputs"`
		} `json:"body"`
	}{}

	resp := api.Post("/api/workflows/speckit/specify/validate", validateInput)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "speckit/specify", mock.lastValidateName)
}

func TestWorkflowHandler_List_InternalError_Returns500WithoutInternalMessage(t *testing.T) {
	// M5b: ListAllWorkflows errors must be masked from the client. The response
	// must be 500 and the body must NOT expose the raw internal error string
	// (which could contain filesystem paths or SQLite details).
	mock := newMockWorkflowLister()
	mock.listErr = errors.New("sqlite3: disk I/O error on /var/data/secret.db")

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows")
	assert.Equal(t, 500, resp.Code, "List with internal error must return 500")

	// The internal error string must not leak to the client.
	body := resp.Body.String()
	assert.NotContains(t, body, "sqlite3", "internal error details must not be exposed to client")
	assert.NotContains(t, body, "secret.db", "internal path must not be exposed to client")
}

func TestWorkflowHandler_Validate_WorkflowNotFound_Returns404(t *testing.T) {
	// M5b MINOR: ValidateWorkflow on a missing workflow must return 404, not 200
	// with a synthetic validation error.
	mock := newMockWorkflowLister()
	// No workflows registered — GetWorkflow will return "workflow not found".

	api := newWorkflowHandlerAPI(t, mock)

	validateInput := struct {
		Body struct {
			Inputs map[string]any `json:"inputs"`
		} `json:"body"`
	}{}

	resp := api.Post("/api/workflows/local/does-not-exist/validate", validateInput)
	assert.Equal(t, 404, resp.Code, "Validate for missing workflow must return 404, not 200")
}

func TestWorkflowHandler_List_PopulatesScopeAndWorkflow(t *testing.T) {
	mock := newMockWorkflowLister("local-deploy", "speckit/specify")

	api := newWorkflowHandlerAPI(t, mock)

	resp := api.Get("/api/workflows")
	require.Equal(t, 200, resp.Code)

	var result struct {
		Body struct {
			Workflows []WorkflowSummary `json:"workflows"`
		} `json:"body"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	require.Len(t, result.Body.Workflows, 2)

	assert.Equal(t, "local", result.Body.Workflows[0].Scope)
	assert.Equal(t, "local-deploy", result.Body.Workflows[0].Workflow)

	assert.Equal(t, "speckit", result.Body.Workflows[1].Scope)
	assert.Equal(t, "specify", result.Body.Workflows[1].Workflow)
}
