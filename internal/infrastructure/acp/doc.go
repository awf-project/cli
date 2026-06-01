// Package acp implements the ACP (Agent Communication Protocol) infrastructure
// adapter that bridges AWF's workflow execution to the pkg/acpserver transport
// layer. It is the infrastructure-side glue an editor/client uses to drive a
// workflow over ACP and to receive streamed agent output as session/update
// notifications.
//
// # Layering (hexagonal rule)
//
// This package lives in the infrastructure layer. It depends inward only:
//
//   - pkg/acpserver                  — JSON-RPC transport (Message types map onto it)
//   - pkg/display                    — DisplayEvent and event-kind constants
//   - internal/infrastructure/agents — DisplayEventRenderer function type
//   - internal/domain/ports          — Logger, EventPublisher, UserInputReader ports
//   - internal/infrastructure/.../pluginmodel — DomainEvent carried by EventPublisher
//   - standard library (context, fmt, sync, sync/atomic)
//
// It MUST NOT import the application layer. Every coupling to the application is
// expressed through a consumer-defined interface or a callback type declared in
// this package and satisfied/injected by the interfaces/cli wiring layer. This is
// why, for example, the input reader exposes ParkHook callbacks instead of taking an
// *application.ACPSession: the infrastructure stays application-agnostic while the
// wiring layer binds the hooks to ACPSession.ParkedTurnCount.
//
// # Components
//
// Five collaborating components live in this package:
//
//   - ACPRenderer        — converts a DisplayEvent stream (from per-provider parsers)
//     into typed ACP Message variants and forwards them to a Sender. (renderer.go)
//   - Sender / Message   — the typed message contract the renderer emits; a Sender
//     adapts those messages onto acpserver session/update notifications. (message.go)
//   - WorkflowEventProjector — translates domain workflow events into ACP
//     session/update notifications via a SessionNotifier. (event_projector.go)
//   - FanoutPublisher    — a ports.EventPublisher that fans a single domain event out
//     to multiple downstream publishers (e.g. plugin bus + projector). (fanout_publisher.go)
//   - ACPInputReader     — bridges a parked workflow goroutine across ACP turns,
//     turning a later session/prompt into the response of an earlier blocking
//     ReadInput. (input_reader.go)
//
// # ACP notifications / protocol surface
//
// Streamed output reaches the editor as JSON-RPC notifications, not responses. The
// renderer and projector both ultimately produce session/update notifications keyed
// by the active session id. MessageType values (message.go) name the update kind the
// editor renders — agent_message_chunk, agent_thought_chunk, tool_call,
// tool_call_update — and map one-to-one onto the ACP session/update payload shape.
// The transport guarantees (single-writer serialization of stdout frames, 10 MiB
// scanner ceiling, notification = no wire response) are owned by pkg/acpserver; see
// that package's doc.go. This package assumes those guarantees and never writes to
// stdout directly.
//
// # Relevant error codes
//
// Prompt-level failures surfaced through the session service use the USER.ACP.*
// taxonomy from internal/domain/errors (codes.go): ErrorCodeUserACPInvalidPrompt,
// ErrorCodeUserACPUnsupportedBlock, ErrorCodeUserACPPromptInFlight, ErrorCodeUserACPUnknownSession,
// and ErrorCodeUserACPProtocolVersionUnsupported. Transport-level failures use the
// JSON-RPC error codes from pkg/acpserver (ErrInvalidParams, ErrInternal, …).
// Components in this package do not mint new error codes; they propagate domain
// errors upward and log transport/send failures at WARN under a log+continue policy
// so a single failed emit never aborts an in-flight stream.
//
// # ACPRenderer lifecycle (per-step)
//
// ACPRenderer is instantiated once per workflow step, NOT once per session. This
// is a deliberate design choice (Decision 3 in the F102 plan): tool-call ID
// deduplication must not leak across steps. Each step has its own correlation
// namespace.
//
// Typical wiring by the caller (e.g., T026/T027):
//
//	renderer := acp.NewACPRenderer(stepID, sender, masker, logger, env)
//	filterWriter := agents.NewStreamFilterWriterWithParser(
//	    inner, parser, renderer.RenderFunc(ctx),
//	)
//	// run the agent step …
//	// renderer is discarded after the step completes
//
// The renderer must not be shared across steps. Sharing it would merge two
// independent seenTools indices and produce incorrect MsgToolCall /
// MsgToolCallUpdate classifications.
//
// # Event → Message mapping (FR-004)
//
// The following table documents every supported DisplayEvent kind and the
// corresponding ACP Message type emitted:
//
//	DisplayEvent.Kind  Condition              Message.Type
//	─────────────────  ─────────────────────  ──────────────────────
//	display.EventText      always                 MsgAgentMessageChunk
//	display.EventReasoning always                 MsgAgentThoughtChunk
//	display.EventToolUse   first sighting of ID   MsgToolCall
//	display.EventToolUse   subsequent same ID     MsgToolCallUpdate
//	anything else          —                      (silently ignored)
//
// All three event kinds are matched via the typed display.EventKind constants
// (EventText, EventReasoning, EventToolUse) exported by pkg/display. The
// renderer switches on event.Kind, not on a raw string comparison.
//
// # Secret masking (NFR-006)
//
// Every text fragment — including tool arguments — is passed through
// SecretMasker.MaskText(text, env) before being placed in Message.Content.
// The masker replaces values of env keys whose names match secret patterns
// (API_KEY, SECRET_, PASSWORD, TOKEN) with "***". Masking happens OUTSIDE the
// mutex: both masker and env are immutable after construction (set once in
// NewACPRenderer, never mutated), so concurrent callers cannot race on either.
// Only seq allocation and seenTools updates require the mutex.
//
// The SecretMasker interface is consumer-defined (declared in this package) and
// is satisfied by *logger.SecretMasker. This keeps the acp package decoupled
// from the logger infrastructure package; the concrete masker is injected at
// construction time by the wiring layer.
//
// # Tool-call ID synthesis
//
// Some providers do not consistently populate DisplayEvent.ID for tool-use
// events (e.g. Claude streaming chunks). When event.ID is empty, ACPRenderer
// synthesizes a stable ID based on the tool name:
//
//	fmt.Sprintf("%s-tool-%s", stepID, event.Name)   // when Name is non-empty
//	fmt.Sprintf("%s-tool-%d", stepID, seq)           // fallback when Name is also empty
//
// The name-based form is stable across successive streaming chunks of the same
// tool invocation, so the seenTools dedup correctly classifies the first chunk as
// MsgToolCall and every subsequent chunk as MsgToolCallUpdate (issue #4 fix).
// The seq-based fallback is a degenerate case: without a name, dedup is impossible
// and every chunk appears as a new tool call; it exists solely to prevent panics
// and produce a non-empty ToolID for the caller.
//
// # agents.DisplayEventRenderer bridge
//
// The real DisplayEventRenderer function type (defined in
// internal/infrastructure/agents/stream_filter.go) has the signature:
//
//	type DisplayEventRenderer func(events []DisplayEvent)
//
// It accepts a slice, carries no context, and returns no error. This is
// incompatible with ACPRenderer.Render(ctx, event) error, which accepts a
// context and surfaces per-event errors.
//
// The bridge is the explicit RenderFunc(ctx) method, which returns a closure
// conforming to the function type:
//
//	func (r *ACPRenderer) RenderFunc(ctx context.Context) agents.DisplayEventRenderer
//
// The closure captures ctx so that per-event Render calls remain
// cancellation-aware even though the outer function type is context-free. Send
// errors are logged at WARN level and the remaining events in the batch continue
// to be processed (log+continue policy). Aborting the batch would drop events
// that could otherwise be delivered.
//
// # agents.DisplayEvent vs display.DisplayEvent
//
// agents.DisplayEvent is a type alias for display.DisplayEvent (defined in
// internal/infrastructure/agents/display_event.go). The types are identical; no
// field mapping is required when passing events[i] to Render.
//
// # Concurrency
//
// ACPRenderer.Render is safe for concurrent use. A single sync.Mutex (mu)
// protects the seq counter and the seenTools map. The mutex is held only for
// seq allocation and seenTools update; MaskText and Sender.Send are both called
// OUTSIDE the lock. MaskText is safe outside the lock because masker and env are
// immutable after construction. Sender.Send is called outside the lock so a slow
// peer does not serialize all concurrent callers.
// Seq monotonicity is preserved (each goroutine is assigned a unique seq before the
// lock is released); emission order is not guaranteed when multiple goroutines race.
//
// # Package imports
//
// The package imports:
//   - pkg/display               — DisplayEvent, EventText, EventToolUse constants
//   - internal/infrastructure/agents — DisplayEventRenderer function type
//   - internal/domain/ports     — ports.Logger (domain port, not a local interface)
//   - standard library only (context, fmt, sync)
//
// No application layer imports are permitted (hexagonal rule: infrastructure must
// not depend on application).
//
// # WorkflowEventProjector pattern
//
// WorkflowEventProjector (event_projector.go) is the projection adapter that turns
// domain workflow lifecycle events into ACP session/update notifications. It depends
// on a consumer-defined SessionNotifier interface (NotifySessionUpdate), which the
// wiring layer satisfies with an acpserver-backed notifier bound to a session id.
// Keeping SessionNotifier local to this package avoids a direct transport dependency
// in the projection logic and keeps it unit-testable with a fake notifier. A
// notification failure is logged and swallowed rather than propagated, so one dropped
// update never tears down the workflow run.
//
// # FanoutPublisher pattern
//
// FanoutPublisher (fanout_publisher.go) implements ports.EventPublisher by delegating
// each Publish to an ordered slice of target publishers sequentially. It exists so a
// single workflow run can feed both the plugin event bus and the ACP projector from one
// ports.EventPublisher seam without the application layer knowing more than one publisher
// exists. Each target call is bounded by fanoutPublishTimeout via context.WithTimeout so
// a slow or hung target cannot block delivery to the remaining targets indefinitely.
// Publish errors from individual targets are logged as warnings and the fan-out continues
// (best-effort delivery); Close aggregates target Close errors. Sequential execution is
// sufficient for the typical 2–3 target production configuration and avoids spawning an
// unbounded number of goroutines per event (issue #3 fix).
//
// # ACPInputReader pattern and park instrumentation
//
// ACPInputReader (input_reader.go) satisfies ports.UserInputReader for a workflow
// running under the ACP server. Unlike a terminal reader, there is no live stdin to
// block on: the workflow goroutine parks on an internal buffered responseCh, and a
// later session/prompt turn delivers the user's text via Respond, unblocking it. This
// is the conversation-parking bridge (F102 US2): one logical multi-turn conversation
// is carried across several discrete ACP prompts by the same parked goroutine.
//
// The reader holds no turn counter; the size-1 buffered responseCh is the only
// synchronization primitive (one Respond per ReadInput). EndTurnNotifier fires once on
// entry to tell the serve loop the current prompt should close with end_turn while the
// goroutine keeps waiting for the next prompt.
//
// Park accounting is delegated to the caller through the OnPark/OnUnpark ParkHook
// callbacks installed via SetParkHooks. ReadInput invokes OnPark immediately before
// parking on responseCh and OnUnpark (via defer) once the wait resolves — whether a
// response arrived or ctx was cancelled. The hooks are guaranteed balanced (one
// OnUnpark per OnPark), which lets the application layer keep ACPSession.ParkedTurnCount
// accurate without this package importing application. This is the seam the application
// phase wires to atomically increment the counter before the goroutine blocks and
// decrement it after, enabling the continuation-turn branch in the session service
// (route a prompt to Respond when ParkedTurnCount > 0 instead of starting a new
// workflow). Hooks run on the workflow goroutine and must be cheap and non-blocking
// (an atomic add is the intended implementation); nil hooks are a no-op.
package acp
