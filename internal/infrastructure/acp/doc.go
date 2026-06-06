// Package acp implements the ACP (Agent Communication Protocol) infrastructure
// adapter that bridges AWF's workflow execution to the official ACP Go SDK
// transport. It is the infrastructure-side glue an editor/client uses to drive a
// workflow over ACP and to receive streamed agent output as session/update
// notifications. This package mirrors the F104 MCP adapter (commit 9740292) in
// structure: a single SDK confinement layer wrapping a clean internal port boundary.
//
// # Purpose
//
// This package wraps github.com/coder/acp-go-sdk (pinned v0.13.0) to provide a
// minimal, safe adapter between AWF's internal application services and the ACP
// protocol. It occupies the infrastructure layer of the hexagonal architecture:
// it depends inward on domain ports and the application layer's session service,
// and outward on the SDK transport. SDK types do not appear in any method signatures
// consumed by the application or domain layers; the SDK is fully confined here.
//
// The primary entry point for the CLI is the `awf acp-serve` command (T036), which
// instantiates Agent, wires it to an AgentSideConnection, and delegates to sdk.Serve.
// The server exits when the connection closes or the context is cancelled.
//
// # Public Surface
//
// The exported symbols are:
//
//   - Agent
//     Implements sdk.Agent, delegating session lifecycle to the application-layer
//     ACPSessionService. Four methods carry real delegation logic (Initialize,
//     NewSession, Prompt, Cancel); seven methods return MethodNotFound stubs for
//     optional ACP capabilities not yet implemented (Authenticate, CloseSession,
//     ListSessions, ResumeSession, SetSessionConfigOption, SetSessionMode, and any
//     future optional interface methods). All handler methods guard against panics
//     with a deferred recover; see Threat Model.
//
//   - NewAgent(svc *application.ACPSessionService) *Agent
//     Constructs an Agent backed by the provided session service. The unexported
//     sessionService interface narrows the dependency to the three handler methods,
//     enabling unit testing with a fake without importing the full application package.
//     Wired to a live AgentSideConnection in T036.
//
//   - Conn
//     Wraps the SDK *AgentSideConnection so the interfaces/cli layer owns the transport
//     lifecycle (stdin forwarding, signal-driven shutdown, the Done() wait loop) WITHOUT
//     importing the SDK. This is what keeps the SDK connection type confined to this
//     package (see SDK Substitution); it mirrors F104's mcp_serve.go delegating transport
//     construction to internal/infrastructure/mcp.
//
//   - NewConnection(agent *Agent, out io.Writer, in io.Reader, logger *slog.Logger) *Conn
//     Builds the agent-side ACP connection over the (out, in) stdio pair and routes SDK
//     diagnostics to logger (stderr in production). Exposes Done() <-chan struct{} and
//     NewEmitter(*slog.Logger) *Emitter so callers drive shutdown and obtain emitters
//     without naming any SDK type.
//
//   - Emitter
//     Implements application.SessionUpdateEmitter by forwarding session/update
//     notifications over an sdk.AgentSideConnection. Unknown update kinds are logged
//     at WARN and dropped (log+continue policy).
//
//   - NewEmitter(conn *sdk.AgentSideConnection, logger *slog.Logger) *Emitter
//     Constructs an Emitter bound to a live SDK connection.
//
//   - Renderer
//     A per-step, mutex-protected renderer (seenTools dedup) that translates DisplayEvent
//     streams directly into ACP session/update notifications via a concrete
//     application.SessionUpdateEmitter. It emits the SDK SessionUpdate variants itself —
//     there is no intermediate Message/Sender DTO.
//
//   - NewRenderer(sessionID, stepID string, emitter application.SessionUpdateEmitter, masker SecretMasker, logger *slog.Logger, env map[string]string) *Renderer
//     Constructs a Renderer bound to one ACP session and one workflow step. Must not be
//     shared across steps; the seenTools dedup index must not leak across step boundaries.
//     masker may be nil (no redaction); env supplies the secret values to mask.
//
//   - PermissionClient
//     SDK-backed client for ACP permission requests. Satisfies a consumer-defined
//     interface so the application layer can request human confirmation without
//     importing SDK types directly. Implemented in permission.go; the consumer
//     call site (PermissionGate) is delivered by F108 Axis B, not F105.
//
//   - NewPermissionClient(conn *sdk.AgentSideConnection, logger *slog.Logger) *PermissionClient
//     Constructs a PermissionClient bound to an AgentSideConnection. A nil
//     connection is normalised to "no transport"; the consumer call site that
//     drives permission requests is delivered by F108 Axis B (F105 wires transport).
//
//   - toACPError(e *application.ACPHandlerError) error
//     Unexported conversion helper. Translates ACPHandlerError to the SDK request-error
//     variant appropriate for the error kind. Listed here because it is the sole
//     error-translation seam for all handler methods in this package; no other file
//     constructs SDK error values directly.
//
//   - Error sentinels: invalidParamsErr, internalErr, methodNotFoundErr
//     Unexported helpers wrapping toACPError for the three most common request-error
//     kinds. Used by all sdk.Agent handler methods in agent.go.
//
// # Internal Layout
//
// Nine implementation files and one architecture test carry the detail:
//
//   - agent.go
//     Agent struct and all eleven sdk.Agent method implementations. Four methods
//     carry real logic (Initialize, NewSession, Prompt, Cancel); seven return
//     sdk.MethodNotFound stubs (Authenticate, CloseSession, ListSessions,
//     ResumeSession, SetSessionConfigOption, SetSessionMode). All methods wrap their
//     body in a deferred panic-recover guard (see Threat Model). The raw prompt-body guard
//     reuses application.MaxPromptBytes (1 MiB) — the single source of truth shared with the
//     application-layer parse guard, so the two layers cannot drift apart.
//
//   - errors.go
//     toACPError and the three unexported helper constructors (invalidParamsErr,
//     internalErr, methodNotFoundErr). This is the sole translation seam between
//     application.ACPHandlerError and SDK request-error variants; no other file
//     constructs SDK error values directly.
//
//   - emitter.go
//     Emitter struct and EmitSessionUpdate. Bridges application.SessionUpdateEmitter
//     onto sdk.AgentSideConnection. Logs and drops unknown update kinds silently.
//
//   - server.go
//     Conn wrapper and NewConnection. Owns sdk.NewAgentSideConnection construction and
//     SetLogger wiring, exposing only Done() and NewEmitter() to callers. This confines
//     the SDK connection type to this package so the interfaces/cli serve command never
//     imports the SDK (mirrors F104's mcp_serve.go transport delegation).
//
//   - event_projector.go
//     WorkflowEventProjector. Translates domain workflow lifecycle events into ACP
//     session/update notifications via application.SessionUpdateEmitter. It is bound to the
//     ACP session ID at construction (NOT the run's workflow_id) so each notification routes
//     to the session the editor created. Notification failures are logged and swallowed
//     (log+continue) so a single dropped notification never aborts a workflow run.
//
//   - renderer.go
//     Renderer (per-step, mutex-protected, seenTools dedup). It is the bridge between
//     DisplayEvent streams and ACP session/update notifications, emitting SDK SessionUpdate
//     variants directly through application.SessionUpdateEmitter (no Message/Sender DTO). The
//     SecretMasker interface is also declared here; it is satisfied by *logger.SecretMasker
//     and injected at construction time.
//
//   - input_reader.go
//     ACPInputReader. Satisfies ports.UserInputReader for ACP-driven workflows via a
//     size-1 buffered responseCh and balanced ParkHook callbacks (OnPark/OnUnpark).
//     The park/unpark seam lets the application layer track parked goroutines without
//     this package importing application. EndTurnNotifier fires once on ReadInput
//     entry to signal that the current prompt should close with end_turn while the
//     goroutine awaits the next prompt.
//
//   - fanout_publisher.go
//     FanoutPublisher. Implements ports.EventPublisher by delegating each Publish to
//     an ordered slice of target publishers sequentially within a bounded timeout.
//     Errors from individual targets are logged as warnings; Close aggregates errors.
//     Sequential execution is sufficient for the typical 2-3 target production
//     configuration without spawning unbounded goroutines per event.
//
//   - permission.go
//     PermissionClient. SDK-backed implementation of the consumer-defined permission
//     interface. Wired in T036 so the application layer can request human confirmation
//     without importing SDK types.
//
//   - architecture_test.go
//     AST-based import boundary enforcement (T037). Asserts that no file in this
//     package imports internal/interfaces, and that the only SDK import appears in
//     the expected files (agent.go, emitter.go, permission.go, errors.go, server.go).
//     Complements the .go-arch-lint.yml dependency rules added in T038.
//
// # Threat Model
//
// The ACP server runs as a local subprocess communicating with an editor over stdio
// (newline-delimited JSON-RPC 2.0). The stdio channel is the only protocol surface.
// Threat scenarios addressed:
//
//   - Stdout serialization invariant: The SDK transport owns stdout exclusively.
//     No file in this package writes to os.Stdout directly. All output flows through
//     the SDK's AgentSideConnection methods. Diagnostic output (logs, debug traces)
//     is directed to stderr via slog. Violating this invariant corrupts the JSON-RPC
//     framing and breaks the editor connection silently.
//
//   - panic-recover-with-no-stack-trace: Every sdk.Agent method in agent.go wraps
//     its body in a deferred recover(). Panics from the application layer or SDK
//     callbacks are caught, formatted with %v (not %+v or runtime/debug), and returned
//     as sdk.NewInternalError responses. Stack traces are never forwarded because they
//     can leak internal file paths, type names, and implementation details useful for
//     prompt-injection reconnaissance. The %v format is a deliberate choice; see
//     the F104 MCP handler.go for the established pattern (commit 9740292).
//
//   - Secret masking outside mutex: Renderer passes every text fragment through
//     SecretMasker.MaskText before emitting it. The masker replaces env key values
//     matching secret patterns (API_KEY, SECRET_, PASSWORD, TOKEN) with "***". Masking is
//     called OUTSIDE the renderer's sync.Mutex because masker and env are immutable after
//     NewRenderer returns; no concurrent caller can race on either. Only seq allocation and
//     seenTools updates require the mutex.
//
//   - 10 MiB stdio cap: The SDK's StdioTransport enforces a 10 MiB per-message
//     ceiling on stdin frames. Frames exceeding this limit are rejected at the
//     transport layer before reaching any handler in this package. The
//     application.MaxPromptBytes constant (1 MiB), reused by the guard in agent.go,
//     adds an application-level cap on raw prompt bodies to keep prompt parsing
//     bounded below the transport ceiling.
//
// # Error Taxonomy
//
// Errors fall into three classes, each mapped to a specific SDK factory:
//
//	ACPHandlerError.Kind     SDK factory                  Typical triggers
//	──────────────────────   ──────────────────────────   ────────────────────────────────
//	ACPErrInvalidParams      sdk.NewInvalidParams         Malformed prompt body, prompt
//	                                                      exceeds MaxPromptBytes, missing
//	                                                      required session fields
//	ACPErrMethodNotFound     sdk.NewMethodNotFound        Optional ACP methods not yet
//	                                                      implemented (7 stubs in Agent)
//	ACPErrInternal           sdk.NewInternalError         Application errors, recovered
//	                                                      panics, unexpected conditions
//
// Transport-level errors (connection loss, framing failures, context cancellation)
// are not wrapped; they propagate from sdk.Serve directly to the T036 caller.
// Emitter and FanoutPublisher target errors are logged at WARN and swallowed so a
// single failed notification never aborts a workflow run. This package does not mint
// new domain error codes; classification is delegated to application.ACPHandlerError.
//
// # Dependency Contract
//
// This package is permitted to import:
//
//   - Standard library (context, encoding/json, fmt, log/slog, sync, sync/atomic)
//   - github.com/coder/acp-go-sdk (pinned v0.13.0) — The official ACP Go SDK.
//     SDK types are used only in agent.go, emitter.go, permission.go, errors.go, and
//     server.go, never in signatures consumed by callers outside this package. This
//     insulates callers from SDK churn and enables SDK substitution (see below).
//   - internal/application — ACPSessionService, ACPHandlerError, SessionUpdateEmitter.
//     The unexported sessionService interface in agent.go narrows this to three
//     handler methods, enabling unit testing without the full application package.
//   - internal/domain/ports — ports.Logger, ports.EventPublisher, ports.UserInputReader.
//   - internal/infrastructure/agents — DisplayEventRenderer function type, DisplayEvent.
//   - pkg/display — EventText, EventReasoning, EventToolUse kind constants.
//
// It MUST NOT import:
//
//   - internal/interfaces — hexagonal rule: infrastructure must not depend on the
//     interface layer.
//   - pkg/acpserver — deleted in F105; this package replaces it with the SDK.
//
// These constraints are enforced by two complementary mechanisms:
//  1. architecture_test.go (T037) — AST-based import scan executed at test time.
//  2. .go-arch-lint.yml (T038) — go-arch-lint rules enforced in CI.
//
// # SDK Substitution
//
// github.com/coder/acp-go-sdk is fully confined to this package. If the SDK is
// replaced by a different ACP transport (a different module version, a fork, or a
// custom implementation), all changes are localized here: agent.go (sdk.Agent method
// signatures and recover wrappers), emitter.go (AgentSideConnection send method),
// permission.go (PermissionClient wrapping), errors.go (SDK error constructors), and
// server.go (the Conn wrapper around sdk.NewAgentSideConnection). The interfaces/cli
// serve command consumes only the Conn wrapper, never the SDK connection type directly.
// The application layer, domain layer, and all other infrastructure packages depend
// only on consumer-defined interfaces and application types, not on SDK types. No
// changes outside internal/infrastructure/acp are required for an SDK swap.
package acp
