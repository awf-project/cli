// Package api implements the HTTP server interface for AWF.
//
// It wires the Huma v2 / chi v5 HTTP adapter and exposes workflow execution
// endpoints consumed by external callers. The package is an adapter in the
// "interfaces" layer of the hexagonal architecture: it translates HTTP requests
// into application service calls and HTTP responses back. No domain logic or
// infrastructure details reside here—only request parsing, response formatting,
// middleware wiring, and coordination between the HTTP layer, the ports.WorkflowFacade /
// ports.WorkflowReader application ports, and the Bridge live-execution metadata store.
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
// layer only through the domain ports it is wired with: ports.WorkflowFacade (run,
// resume, list, validate, status, history), ports.WorkflowReader (get, history stats),
// and the application.SessionRegistry for live session lookup. The Bridge is a small
// metadata store, not a read port. The package never imports infrastructure packages
// directly—this invariant is enforced by go-arch-lint.
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
//	            │              │              │
//	            ▼              ▼              ▼
//	  ports.WorkflowFacade  *Bridge   application.SessionRegistry
//	  ports.WorkflowReader  (live     (live RunSession lookup
//	  (list/get/validate/    exec      for SSE + status)
//	   run/resume/history)    metadata:
//	                          cancel,
//	                          List/Get)
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
// Bridge is a metadata-only store for live (in-flight) executions. It holds NO read or
// history port: workflow listing/get/validate and history queries route through
// ports.WorkflowFacade / ports.WorkflowReader directly from the handlers (F108 read-path
// consolidation). Bridge owns only the in-memory sync.Map of active execution metadata and
// exposes TrackFacadeSession / GetExecution / CancelExecution / ListExecutions / Shutdown.
//
// Bridge is NOT an anti-corruption read layer and is NOT responsible for executing
// workflows. All workflow execution (Run and Resume) is routed exclusively through
// ports.WorkflowFacade, wired via WithFacade and WithRegistryImpl on NewServer. The Bridge
// entry is created from the returned RunSession so that List/Get/Cancel keep working while
// the authoritative live session lives in the application.SessionRegistry.
//
// ## Facade execution path (F108)
//
// POST /api/workflows/{scope}/{name}/run and POST /api/executions/{id}/resume both
// require a ports.WorkflowFacade and an application.SessionRegistry to be wired.
// When either is absent, the handlers return 503 Service Unavailable immediately
// (a generic message that does not leak internal configuration detail). Genuine
// not-found stays 404 and request-validation failures stay 422.
// When both are present, execution is delegated exclusively to facade.Run or
// facade.Resume. The returned ports.RunSession is:
//
//  1. Registered in the SessionRegistry synchronously (FR-003: a subscriber
//     connecting on the SSE endpoint right after /run MUST find the session).
//  2. Tracked as lightweight metadata in Bridge.activeExecutions so that
//     List/Get/Cancel keep working (A4 / R5).
//
// The RunSession's own ID() is authoritative: it is the execution ID returned in
// the HTTP response and the key under which SessionRegistry.Get resolves later.
//
// ## Handler families
//
//   - WorkflowHandlers (handlers_workflows.go): read-only workflow operations,
//     served directly by ports.WorkflowFacade / ports.WorkflowReader.
//   - ExecutionHandlers (handlers_executions.go): lifecycle operations on async
//     workflow runs. Run and Resume require a facade and a registry; List/Get/Cancel
//     operate on Bridge live-execution metadata directly (Get overlays the live status
//     snapshot from the registry session when available).
//   - SSEHandler (sse.go): streams step and workflow transition events to
//     connected clients via Server-Sent Events. Resolves live sessions via the
//     SessionRegistry (SessionLookup), not Bridge metadata.
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
//  4. Run/Resume handlers invoke ports.WorkflowFacade and record live metadata in the
//     Bridge; read-only handlers (list/get/validate/history) call ports.WorkflowFacade /
//     ports.WorkflowReader directly. Execution List/Get/Cancel read Bridge metadata.
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
//  2. SSEHandler.getSession resolves the live RunSession from the SessionRegistry
//     (SessionLookup). Returns 404 if not found.
//  3. Events are consumed from session.Events() and serialized as SSE frames.
//  4. When the session channel closes (terminal state or session Close), the stream
//     exits cleanly.
//  5. On context cancellation (client disconnect or server shutdown), the goroutine
//     exits cleanly and decrements sseWG.
//
// # Error Handling
//
// Handlers return huma.Error* types directly (e.g., huma.Error404NotFound,
// huma.Error422UnprocessableEntity). Huma serializes these to RFC 7807
// problem+json responses. No separate error-mapping middleware is used.
//
// # Concurrency Model
//
// ## Facade-driven executions
//
// POST /api/workflows/{scope}/{name}/run delegates to ports.WorkflowFacade.Run.
// The facade owns the execution goroutine and its lifetime; the Bridge only
// records metadata. The client receives 202 Accepted with an execution ID
// immediately. Subsequent GET /api/executions/{id} or SSE stream requests
// observe the running state via the SessionRegistry.
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
//     All infrastructure access is mediated by application service ports via Bridge
//     or ports.WorkflowFacade.
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
//   - ExecutionHandlers.Run and ExecutionHandlers.Resume require both a
//     ports.WorkflowFacade and an application.SessionRegistry to be wired (via
//     WithFacade and WithRegistryImpl on NewServer). Without both, these handlers
//     return 503 Service Unavailable immediately — there is no legacy fallback path.
//
//   - SSE resolves live sessions via SessionLookup (the SessionRegistry wrapped
//     by WithSessionRegistry), NOT via Bridge.activeExecutions. This separation
//     keeps execution ownership with the facade and metadata ownership with Bridge.
//
//   - The SSE WaitGroup (sseWG) is owned by Server and passed by pointer to
//     NewSSEHandler. Handlers call wg.Add(1) and defer wg.Done() around the
//     stream goroutine lifetime.
//
//   - Shutdown timeout is configurable only via WithShutdownTimeout; it is never
//     read from CLI flags or environment variables at this layer.
package api
