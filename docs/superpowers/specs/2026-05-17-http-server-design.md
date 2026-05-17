# HTTP Server Interface Layer — Design Spec

**Date**: 2026-05-17
**Status**: Approved
**Scope**: New interface layer for AWF CLI — HTTP API with auto-generated OpenAPI spec

## Objective

Expose AWF workflow monitoring and execution capabilities through an HTTP API, alongside the existing CLI and TUI interfaces. The OpenAPI spec is auto-generated from Go types — no separate spec file to maintain.

## Decisions

| Decision | Choice | Alternative considered | Trade-off |
|----------|--------|----------------------|-----------|
| Framework | Huma v2 + chi v5 | chi + swaggo, ogen, net/http only | Huma imposes input/output struct conventions but guarantees spec-code sync |
| Real-time | SSE via `huma/sse` | WebSocket, polling-only | SSE is unidirectional (sufficient for monitoring), simpler than WebSocket |
| Auth | None in v1 (localhost-only) | API key, JWT | Deferred to reduce scope; `--host` flag allows explicit override |
| Execution model | Async via `RunAsync()` | Sync blocking | Matches TUI pattern; client gets `execution_id` immediately, follows via SSE |

## Architecture

```
internal/interfaces/api/
├── server.go               # Server struct, chi router, huma API, Start/Shutdown
├── bridge.go               # Bridge adapter (WorkflowService, ExecutionService, HistoryService)
├── handlers_workflow.go    # Workflow CRUD + validation + run
├── handlers_execution.go   # Execution monitoring + cancel + resume
├── handlers_history.go     # History listing + stats
├── types.go                # Huma input/output structs (drive OpenAPI generation)
└── doc.go                  # Package documentation
```

New CLI command: `awf serve` in `internal/interfaces/cli/serve.go`.

### Layer Dependencies

- `api/` imports: `application/`, `domain/workflow/`, `domain/ports/` (inward only)
- `api/` does NOT import: `infrastructure/`, `cli/`, `tui/`
- Bridge pattern identical to `tui/bridge.go` — adapts application services to handler needs

## API Endpoints

### Workflows

| Method | Path | OperationID | Description |
|--------|------|-------------|-------------|
| GET | `/api/workflows` | `list-workflows` | List all workflows with metadata |
| GET | `/api/workflows/{name}` | `get-workflow` | Full workflow definition (steps, inputs, hooks) |
| POST | `/api/workflows/{name}/validate` | `validate-workflow` | Static validation, returns errors list |
| POST | `/api/workflows/{name}/run` | `run-workflow` | Start async execution, returns `execution_id` |

### Executions

| Method | Path | OperationID | Description |
|--------|------|-------------|-------------|
| GET | `/api/executions` | `list-executions` | Active and recent executions |
| GET | `/api/executions/{id}` | `get-execution` | Execution detail (status, steps, outputs) |
| GET | `/api/executions/{id}/events` | `stream-execution-events` | SSE stream of execution events |
| DELETE | `/api/executions/{id}` | `cancel-execution` | Cancel running execution |
| POST | `/api/executions/{id}/resume` | `resume-execution` | Resume failed execution |

### History

| Method | Path | OperationID | Description |
|--------|------|-------------|-------------|
| GET | `/api/history` | `list-history` | Execution history with filters |
| GET | `/api/history/stats` | `get-history-stats` | Aggregated statistics |

### Auto-generated Routes (by Huma)

- `GET /docs` — Swagger UI
- `GET /openapi.json` — OpenAPI 3.1 spec (JSON)
- `GET /openapi.yaml` — OpenAPI 3.1 spec (YAML)

## SSE Event Types

```go
map[string]any{
    "step_started":        StepStartedEvent{},
    "step_completed":      StepCompletedEvent{},
    "step_failed":         StepFailedEvent{},
    "workflow_completed":  WorkflowCompletedEvent{},
    "workflow_failed":     WorkflowFailedEvent{},
    "output":              OutputEvent{},
}
```

Events are typed Go structs — Huma's `sse.Register` matches data type to event name automatically.

## Huma Type Examples

```go
type ListWorkflowsOutput struct {
    Body []WorkflowSummary
}

type WorkflowSummary struct {
    Name        string `json:"name" doc:"Workflow identifier"`
    Version     string `json:"version" doc:"Semantic version"`
    Description string `json:"description" doc:"Human-readable description"`
}

type RunWorkflowInput struct {
    Name string `path:"name" doc:"Workflow name"`
    Body struct {
        Inputs map[string]any `json:"inputs" doc:"Workflow input values"`
    }
}

type RunWorkflowOutput struct {
    Body struct {
        ExecutionID string `json:"execution_id" doc:"Unique execution identifier"`
        Status      string `json:"status" doc:"Initial execution status"`
    }
}
```

Struct tags (`doc:`, `example:`, `required:`, `json:`) feed the OpenAPI spec directly.

## Concurrency Model

```
Client                    Server                      ExecutionService
  |                         |                               |
  |-- POST /run ----------->|-- RunAsync() --------------->|
  |<-- 202 {exec_id} ------|   store in activeExecutions   |
  |                         |                               |
  |-- GET /events (SSE) --->|-- poll ExecutionContext <-----|
  |<-- step_started --------|   every 200ms                 |
  |<-- step_completed ------|                               |
  |<-- workflow_completed --|   cleanup activeExecutions    |
  |    (stream closes)      |                               |
```

- `activeExecutions`: `sync.Map` in Bridge, keyed by execution ID
- Polling interval: 200ms (matches TUI)
- SSE stream closes after terminal event or client disconnect
- Context cancellation propagates to `ExecutionService` on DELETE

## CLI Command

```
awf serve [flags]

Flags:
  --port    int     Server port (default: 2511)
  --host    string  Bind address (default: 127.0.0.1)
```

Graceful shutdown: `signal.NotifyContext(SIGINT, SIGTERM)` + `srv.Shutdown()` with 30s timeout for active SSE streams.

## Wiring

Same pattern as `cli/run.go`:

1. Create infrastructure (repository, stores, executors, logger)
2. Create application services (WorkflowService, ExecutionService, HistoryService)
3. Wire optional providers (agents, plugins, OTel)
4. Create Bridge with services
5. Create Server with Bridge
6. `server.Start(ctx)`

## Out of Scope (v1)

- Authentication/authorization (localhost-only binding)
- HTTPS termination (use reverse proxy)
- WebSocket (SSE sufficient for unidirectional monitoring)
- Plugin management via API
- Configuration management via API
- Rate limiting

## Dependencies

New:
- `github.com/danielgtaylor/huma/v2`
- `github.com/go-chi/chi/v5`

No changes to existing packages.

## Testing Strategy

- Unit tests: each handler with mocked Bridge methods
- Integration tests: full server startup, HTTP requests, SSE stream consumption
- Benchmark: SSE throughput with concurrent subscribers
- Race detection: `make test-race` for `sync.Map` and concurrent executions
