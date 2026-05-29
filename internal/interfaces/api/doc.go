// Package api implements the HTTP server interface for AWF.
//
// It wires the Huma v2 / chi v5 HTTP adapter and exposes workflow execution
// endpoints consumed by external callers. The package is an adapter in the
// "interfaces" layer of the hexagonal architecture: it translates HTTP requests
// into application service calls and HTTP responses back. No domain logic or
// infrastructure details reside here—only request parsing, response formatting,
// middleware wiring, and coordination between the HTTP layer and the Bridge.
//
// # Overview
//
// The api package provides a REST API surface for AWF with the following
// capabilities:
//
//   - Workflow discovery and validation (GET /api/workflows, GET /api/workflows/{scope}/{name},
//     POST /api/workflows/{scope}/{name}/validate)
//   - Asynchronous workflow execution (POST /api/workflows/{scope}/{name}/run)
//   - Execution lifecycle management (GET/DELETE /api/executions/{id},
//     POST /api/executions/{id}/resume)
//   - Real-time event streaming via Server-Sent Events (GET /api/executions/{id}/events)
//   - Execution history and statistics (GET /api/history, GET /api/history/stats)
//   - Auto-generated OpenAPI 3.1 specification at /openapi.{json,yaml}
//   - Swagger UI at /docs
//
// The package is designed to be embedded inside the CLI (via interfaces/cli) or
// run standalone. The caller creates a Server with NewServer, calls Start in a
// goroutine, and calls Shutdown on signal receipt.
//
// # Architecture
//
// The api package sits in the interfaces layer. It depends on the application
// layer only through the Bridge adapter, which holds narrow service ports
// (WorkflowLister, WorkflowRunner, HistoryProvider, WorkflowResumer). It never
// imports infrastructure packages directly—this invariant is enforced by
// go-arch-lint.
//
//	             External HTTP clients
//	                   │
//	                   ▼
//	  ┌────────────────────────────────┐
//	  │           chi.Mux              │
//	  │  (Logger → Recoverer →         │
//	  │   RequestID)                   │
//	  └───────────────┬────────────────┘
//	                  │
//	                  ▼
//	  ┌────────────────────────────────┐
//	  │          huma.API              │
//	  │  (schema validation, OpenAPI,  │
//	  │   content negotiation, /docs)  │
//	  └───────────────┬────────────────┘
//	                  │
//	       ┌──────────┼──────────────┐
//	       ▼          ▼              ▼
//	WorkflowHandlers  ExecutionHandlers  SSEHandler  HistoryHandlers
//	       │          │              │        │
//	       └──────────┴──────────────┴────────┘
//	                         │
//	                         ▼
//	                    *Bridge
//	                         │
//	       ┌─────────────────┼─────────────────┐
//	       ▼                 ▼                 ▼
//	WorkflowLister    WorkflowRunner    HistoryProvider
//	  (app layer)      (app layer)       (app layer)
//
// # Key Types
//
// ## Server
//
// Server is the top-level assembly: it owns the chi.Mux, the huma.API instance,
// the net/http.Server, the shutdown timeout, and the SSE WaitGroup. One Server
// per process; instantiate via NewServer.
//
// ## Bridge
//
// Bridge holds the narrow service ports injected at startup. It acts as an
// anti-corruption layer: handler families call Bridge methods rather than
// application services directly, insulating handlers from service API changes.
// Bridge also owns the in-memory sync.Map of active executions and exposes
// StartExecution / GetExecution / CancelExecution / ListExecutions.
//
// ## Handler families
//
//   - WorkflowHandlers (handlers_workflows.go): read-only workflow operations.
//   - ExecutionHandlers (handlers_executions.go): lifecycle operations on async
//     workflow runs.
//   - SSEHandler (sse.go): streams step and workflow transition events to
//     connected clients via Server-Sent Events.
//   - HistoryHandlers (handlers_history.go): queries past execution records and
//     aggregate statistics.
//
// # Request Lifecycle
//
// 1. chi.Mux receives the request and applies middleware in order:
//
//   - middleware.Logger: structured access log.
//
//   - middleware.Recoverer: converts panics to 500 responses.
//
//   - middleware.RequestID: injects X-Request-Id header.
//
//     2. huma.API parses the request: validates path/query parameters, deserializes
//     the request body against the registered JSON schema, and populates the
//     typed input struct.
//
// 3. The registered handler function is called with the validated input.
//
// 4. The handler calls a Bridge method, which calls an application service port.
//
// 5. The handler returns a typed output struct or a huma.Error*.
//
// 6. huma.API serializes the response body and writes HTTP headers and status.
//
// # SSE Event Flow
//
// GET /api/executions/{id}/events establishes a long-lived SSE connection:
//
//  1. SSEHandler.Stream increments s.sseWG (owned by Server) so Shutdown knows
//     how many streams are active.
//  2. A 200ms ticker polls ExecutionContext.GetAllStepStates().
//  3. On each tick, step status transitions are compared with the previous snapshot.
//     Changed steps emit typed SSE events: StepStartedEvent, StepCompletedEvent,
//     StepFailedEvent.
//  4. When the workflow reaches a terminal state (completed, failed, cancelled),
//     WorkflowCompletedEvent or WorkflowFailedEvent is emitted and the goroutine exits.
//  5. On context cancellation (client disconnect or server shutdown), the goroutine
//     exits cleanly and decrements sseWG.
//
// Event types are registered in eventRegistry, which maps audit-event constants to
// Go structs. huma/sse derives the SSE event name from the struct type name at
// send time.
//
// # Error Handling
//
// Handlers return huma.Error* types directly (e.g., huma.Error404NotFound,
// huma.Error422UnprocessableEntity). Huma serializes these to RFC 7807
// problem+json responses. No separate error-mapping middleware is used.
//
// # Concurrency Model
//
// ## Async executions
//
// RunWorkflow (POST /api/workflows/{scope}/{name}/run) starts workflow execution in a
// background goroutine managed by Bridge.StartExecution. The client receives
// 202 Accepted with an execution ID immediately. Subsequent calls to
// GET /api/executions/{id} or the SSE stream observe the running state.
//
// ## SSE goroutine lifecycle
//
// Each SSE connection runs in its own goroutine managed by huma/sse. Server.sseWG
// tracks active SSE goroutines. Server.Shutdown waits on sseWG after calling
// httpSrv.Shutdown so that in-flight streams can complete before the process exits.
// The maximum wait is bounded by shutdownTimeout (default 30s, configurable via
// WithShutdownTimeout).
//
// ## Graceful shutdown sequence
//
//  1. Caller calls Server.Shutdown(ctx).
//  2. context.WithTimeout(ctx, shutdownTimeout) is created.
//  3. httpSrv.Shutdown(shutdownCtx) stops accepting new connections and waits for
//     active HTTP handlers to complete (chi sends context cancellation to handlers).
//  4. SSE goroutines detect ctx.Done() and exit, decrementing sseWG.
//  5. sseWG.Wait() blocks until all SSE goroutines have exited.
//  6. Shutdown returns.
//
// # Architectural Invariants
//
// These invariants are enforced by go-arch-lint (NFR-001, NFR-006):
//
//   - interfaces/api MUST NOT import internal/infrastructure/*.
//     All infrastructure access is mediated by application service ports via Bridge.
//
//   - All route registration MUST happen inside NewServer, never at package init
//     or in handler constructors. This keeps the full routing table visible in a
//     single function for discoverability.
//
//   - One huma.API instance per Server. No global huma.API variables.
//
//   - The Bridge and handler family constructors (NewWorkflowHandlers,
//     NewExecutionHandlers, NewSSEHandler, NewHistoryHandlers) receive their
//     dependencies at construction time; they never fetch from global state.
//
//   - The SSE WaitGroup (sseWG) is owned by Server and passed by pointer to
//     NewSSEHandler. Handlers call wg.Add(1) and defer wg.Done() around the
//     stream goroutine lifetime.
//
//   - Shutdown timeout is configurable only via WithShutdownTimeout; it is never
//     read from CLI flags or environment variables at this layer.
package api
