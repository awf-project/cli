package api

import (
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// --- Workflow list ---

// WorkflowSummary is the listing DTO for GET /api/workflows entries.
// Scope and Workflow are the two components of the canonical URL grammar
// `/api/workflows/{scope}/{name}`; Name is the display identifier
// (`scope/workflow` for pack entries, plain name for local/global).
type WorkflowSummary struct {
	Name        string `json:"name" doc:"Workflow name." example:"deploy-prod"`
	Scope       string `json:"scope" doc:"Workflow scope (\"local\" or vendor pack name)." example:"local"`
	Workflow    string `json:"workflow" doc:"Workflow name without scope prefix." example:"deploy-prod"`
	Version     string `json:"version" doc:"Workflow version." example:"1.0.0"`
	Description string `json:"description" doc:"Short description." example:"Deploy to production"`
}

type listWorkflowsBody struct {
	Workflows []WorkflowSummary `json:"workflows" doc:"All available workflows."`
}

type ListWorkflowsOutput struct {
	Body struct {
		Body listWorkflowsBody `json:"body"`
	}
}

// --- Workflow get ---

type GetWorkflowInput struct {
	Scope string `path:"scope" doc:"Workflow scope (\"local\" or vendor pack name)." example:"local" required:"true"`
	Name  string `path:"name" doc:"Workflow name." example:"deploy-prod" required:"true"`
}

type GetWorkflowOutput struct {
	Body struct {
		Body *workflow.Workflow `json:"body"`
	}
}

// --- Workflow validate ---

type ValidateWorkflowInput struct {
	Scope string `path:"scope" doc:"Workflow scope (\"local\" or vendor pack name)." example:"local" required:"true"`
	Name  string `path:"name" doc:"Workflow name." example:"deploy-prod" required:"true"`
}

type validateWorkflowBody struct {
	Errors []string `json:"errors" doc:"Validation errors; empty when the workflow is valid."`
}

type ValidateWorkflowOutput struct {
	Body struct {
		Body validateWorkflowBody `json:"body"`
	}
}

// --- Workflow run ---

type RunWorkflowInput struct {
	Scope string `path:"scope" doc:"Workflow scope (\"local\" or vendor pack name)." example:"local" required:"true"`
	Name  string `path:"name" doc:"Workflow name." example:"deploy-prod" required:"true"`
	Body  struct {
		Inputs map[string]any `json:"inputs" doc:"Key/value inputs passed to the workflow."`
	}
}

type runWorkflowBody struct {
	ExecutionID string `json:"execution_id" doc:"Unique identifier for the async execution." example:"550e8400-e29b-41d4-a716-446655440000"`
	Status      string `json:"status" doc:"Always 'accepted' for async runs." example:"accepted"`
}

// runWorkflowOutputBody wraps runWorkflowBody with an inline OpenAPI schema.
// Implementing huma.SchemaProvider prevents Huma from creating a $ref component,
// allowing the test to traverse schema.Properties["body"].Properties["execution_id"]
// without following refs (FR-013 schema inspection invariant).
type runWorkflowOutputBody struct {
	Body runWorkflowBody `json:"body"`
}

func (runWorkflowOutputBody) Schema(_ huma.Registry) *huma.Schema {
	return &huma.Schema{
		Type: "object",
		Properties: map[string]*huma.Schema{
			"body": {
				Type: "object",
				Properties: map[string]*huma.Schema{
					"execution_id": {Type: "string", Description: "Unique identifier for the async execution."},
					"status":       {Type: "string", Description: "Always 'accepted' for async runs."},
				},
			},
		},
	}
}

type RunWorkflowOutput struct {
	Body runWorkflowOutputBody
}

// --- Execution list ---

type listExecutionsBody struct {
	Executions []executionBody `json:"executions" doc:"All active executions."`
}

type ListExecutionsOutput struct {
	Body struct {
		Body listExecutionsBody `json:"body"`
	}
}

// --- Execution get ---

type GetExecutionInput struct {
	ID string `path:"id" doc:"Execution ID." example:"550e8400-e29b-41d4-a716-446655440000" required:"true"`
}

// --- Execution cancel ---

type CancelExecutionInput struct {
	ID string `path:"id" doc:"Execution ID to cancel." example:"550e8400-e29b-41d4-a716-446655440000" required:"true"`
}

// --- Execution resume ---

type ResumeExecutionInput struct {
	ID   string `path:"id" doc:"Execution ID to resume." example:"550e8400-e29b-41d4-a716-446655440000" required:"true"`
	Body struct {
		InputOverrides map[string]any `json:"input_overrides,omitempty" doc:"Input values to override from the original run."`
		FromStep       string         `json:"from_step,omitempty" doc:"Step to resume from: 'current', 'previous', or a named step." example:"build"`
	}
}

// --- Execution status ---

type executionBody struct {
	ExecutionID  string    `json:"execution_id" doc:"Unique execution identifier." example:"550e8400-e29b-41d4-a716-446655440000"`
	WorkflowName string    `json:"workflow_name" doc:"Name of the executed workflow." example:"deploy-prod"`
	Status       string    `json:"status" doc:"Current status: running, success, failed, cancelled." example:"running"`
	CurrentStep  string    `json:"current_step" doc:"Name of the step currently executing." example:"build"`
	StartedAt    time.Time `json:"started_at" doc:"Execution start time (RFC 3339)." example:"2024-01-15T10:30:00Z"`
	UpdatedAt    time.Time `json:"updated_at" doc:"Last status update time (RFC 3339)." example:"2024-01-15T10:30:05Z"`
}

type ExecutionOutput struct {
	Body struct {
		Body executionBody `json:"body"`
	}
}

// --- History ---

type HistoryEntry struct {
	ID           string    `json:"id" doc:"Execution record identifier." example:"rec-abc123"`
	WorkflowName string    `json:"workflow_name" doc:"Name of the workflow." example:"deploy-prod"`
	Status       string    `json:"status" doc:"Outcome: success, failed, cancelled." example:"success"`
	StartedAt    time.Time `json:"started_at" doc:"Execution start time." example:"2024-01-15T10:00:00Z"`
	CompletedAt  time.Time `json:"completed_at" doc:"Execution completion time." example:"2024-01-15T10:05:00Z"`
	DurationMs   int64     `json:"duration_ms" doc:"Duration in milliseconds." example:"300000"`
}

type HistoryListInput struct {
	Workflow string    `query:"workflow" doc:"Filter by workflow name." example:"deploy-prod"`
	Status   string    `query:"status" doc:"Filter by status: success, failed, cancelled." example:"success"`
	Since    time.Time `query:"since" doc:"Return records after this time (RFC 3339)." example:"2024-01-01T00:00:00Z"`
	Until    time.Time `query:"until" doc:"Return records before this time (RFC 3339)." example:"2024-02-01T00:00:00Z"`
	Limit    int       `query:"limit" doc:"Maximum number of records to return." example:"50"`
}

type historyListBody struct {
	Entries []HistoryEntry `json:"entries" doc:"List of execution history records."`
}

type HistoryListOutput struct {
	Body struct {
		Body historyListBody `json:"body"`
	}
}

type HistoryStatsOutput struct {
	Body struct {
		Body *workflow.HistoryStats `json:"body"`
	}
}
