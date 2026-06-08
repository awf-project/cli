// Package transcript defines the canonical in-memory model for agent exchange
// transcripts produced by AWF workflow executions. Every interaction between AWF
// and an AI agent — user prompts, assistant replies, tool invocations, and workflow
// lifecycle events — is represented as an ExchangeEvent carrying a typed Payload.
// The model is serialization-first: all types implement or participate in custom
// JSON marshaling to produce a stable, forward-compatible wire format as required
// by FR-014, D2, and NFR-005.
//
// # Purpose
//
// This package occupies the domain layer of the hexagonal architecture. It defines
// the shared vocabulary used by the application layer to record and replay agent
// interactions, and by the infrastructure layer to persist transcripts as JSON lines.
// The package has no external dependencies beyond the Go standard library; it must
// remain importable without pulling in any third-party module so that consumers
// (transport adapters, test helpers, analytics tooling) can use it independently.
//
// An agent exchange transcript is an ordered sequence of ExchangeEvent values
// emitted during a workflow run. Each event carries a monotonically increasing Seq
// counter, a RunID identifying the workflow instance, optional ParentRunID and
// ChildRunID for nested call_workflow steps, an EventType discriminant, a Path
// marking the current step, an Iteration counter for retry loops, a UTC Timestamp,
// and an opaque Payload whose concrete type is determined by EventType.
//
// The closed vocabulary of EventType values documented below is exhaustive. Decoders
// that encounter an unrecognized EventType must return ErrUnknownEventType and
// discard the event rather than silently skipping it, so callers retain the option
// to surface unknown events as warnings. The same forward-compat contract (D2)
// applies to BlockType: decoders return ErrUnknownBlockType on unrecognized values.
//
// # Public Surface
//
// event.go — ExchangeEvent and the EventType closed vocabulary:
//
//   - EventType — string alias discriminating the ten recognized event kinds.
//     The complete closed vocabulary (exhaustive; no other values are valid):
//     EventTypeRunStarted                = "run.started"
//     EventTypeRunCompleted              = "run.completed"
//     EventTypeStepStarted               = "step.started"
//     EventTypeStepCompleted             = "step.completed"
//     EventTypeStepCallWorkflowStarted   = "step.call_workflow.started"
//     EventTypeStepCallWorkflowCompleted = "step.call_workflow.completed"
//     EventTypeMessageUser               = "message.user"
//     EventTypeMessageAssistant          = "message.assistant"
//     EventTypeToolCall                  = "tool.call"
//     EventTypeToolResult                = "tool.result"
//
//   - ErrUnknownEventType — sentinel returned by decoders for unrecognized EventType
//     strings. Callers must use errors.Is(err, ErrUnknownEventType) to distinguish
//     forward-compat unknown-type events from structural parse failures.
//
//   - ExchangeEvent — the top-level record type for one event in the transcript.
//     Fields: Seq (uint64, monotone counter), RunID (string, UUID of the workflow
//     run), ParentRunID (string, optional UUID of the enclosing parent run for nested
//     workflows), ChildRunID (string, optional UUID of a spawned child run),
//     Type (EventType), Path (string, dot-separated step path within the workflow
//     definition), Iteration (int, zero-based retry counter), Timestamp (time.Time,
//     UTC), Payload (any, typed by Type — see payload.go for the concrete dispatch).
//     ExchangeEvent.MarshalJSON encodes the Payload using the concrete types defined
//     in payload.go, selecting the target struct via a Type switch.
//     ExchangeEvent.UnmarshalJSON decodes the raw payload bytes into the appropriate
//     concrete Payload type based on the Type field value, returning ErrUnknownEventType
//     if the Type is not in the closed vocabulary.
//
// content.go — ContentBlock, BlockType, and Fidelity:
//
//   - BlockType — string alias discriminating the six recognized content block kinds.
//     The complete closed vocabulary (exhaustive):
//     BlockTypeText        = "text"
//     BlockTypeThinking    = "thinking"
//     BlockTypeToolUse     = "tool_use"
//     BlockTypeToolResult  = "tool_result"
//     BlockTypeCommand     = "command"
//     BlockTypeStream      = "stream"
//
//   - Fidelity — string alias indicating the provenance of a ContentBlock. Exactly
//     two values are defined:
//     FidelityRouter       = "router"        — block synthesized by the AWF router
//     FidelityAgentEmitted = "agent_emitted" — block emitted directly by the AI agent
//
//   - ErrUnknownBlockType — sentinel returned by validators and decoders for
//     unrecognized BlockType strings. Usage mirrors ErrUnknownEventType exactly.
//
//   - ContentBlock — a single typed content element within a message or tool payload.
//     Fields: Type (BlockType), Fidelity (Fidelity), Text (string, for text and
//     stream blocks), Thinking (string, for thinking blocks), ToolName (string),
//     ToolID (string, opaque correlation token), ToolInput (any), ToolContent (any),
//     Command (string, for command blocks), Chunk (string, for stream blocks).
//     ContentBlock.MarshalJSON routes to the appropriate field subset based on Type;
//     fields irrelevant to the block's Type are omitted via omitempty JSON tags.
//
//   - ValidBlockType(bt BlockType) bool — returns true if bt is one of the six
//     recognized BlockType values. Used by decoders before construction to gate
//     ErrUnknownBlockType. Does not allocate; pure closed-set membership test.
//
//   - ValidFidelity(f Fidelity) bool — returns true if f is FidelityRouter or
//     FidelityAgentEmitted. Guards ContentBlock construction in strict-mode decoders.
//     Does not allocate.
//
// payload.go — concrete Payload types keyed by EventType:
//
//   - MessagePayload — payload for EventTypeMessageUser and EventTypeMessageAssistant.
//     Fields: Role (string, "user" or "assistant"), Blocks ([]ContentBlock, ordered
//     content sequence; nil and empty slice are semantically equivalent).
//
//   - StepPayload — payload for EventTypeStepStarted, EventTypeStepCompleted,
//     EventTypeStepCallWorkflowStarted, and EventTypeStepCallWorkflowCompleted.
//     Fields: Name (string, step identifier matching the workflow YAML key),
//     Kind (string, "step" or "parallel"), Error (string, omitempty),
//     Result (any, omitempty, serialized step output on completion).
//
//   - ToolPayload — payload for EventTypeToolCall and EventTypeToolResult.
//     Fields: Name (string, tool name), CallID (string, opaque correlation ID
//     shared between the call and result events), Input (any, decoded tool arguments),
//     Output (any, decoded tool result; nil for EventTypeToolCall events),
//     Fidelity (Fidelity, source attribution for the tool interaction).
//
// # Threat Model
//
// The transcript domain operates entirely within process memory during a workflow
// run. The primary threat surface is malformed or adversarially crafted JSON
// consumed during transcript replay or import from external files.
//
//   - Unknown EventType / BlockType values (D2, NFR-005): Decoders MUST return
//     ErrUnknownEventType or ErrUnknownBlockType rather than silently skipping or
//     defaulting unrecognized values. This contract ensures forward-compat events
//     surface as explicit errors rather than silently corrupting aggregation or replay.
//
//   - Arbitrary Payload bytes: ExchangeEvent.UnmarshalJSON must not pass raw
//     attacker-controlled bytes to any execution primitive. The Payload any field
//     is decoded only into the concrete types defined in payload.go; no eval, no
//     dynamic dispatch beyond the closed EventType switch.
//
//   - Nil ContentBlock.Blocks slice: MessagePayload.Blocks may be nil for empty
//     messages. Consumers must treat nil and empty slice identically (len == 0).
//     Range over a nil slice is safe in Go; no nil-guard required at call sites.
//
//   - Oversized payloads: This package sets no size limit on ContentBlock.Text or
//     ToolPayload.Input. Callers operating over untrusted sources (file import, HTTP
//     upload) must impose their own size caps before passing bytes to UnmarshalJSON.
//
// # Error Taxonomy
//
// This package exposes two sentinel errors:
//
//   - ErrUnknownEventType: Returned by ExchangeEvent.UnmarshalJSON when the "type"
//     JSON field contains a string not in the EventType closed vocabulary. Callers
//     implementing permissive forward-compat decoding should check
//     errors.Is(err, ErrUnknownEventType) and treat it as a warning rather than fatal,
//     allowing the transcript stream to continue past unknown future event kinds.
//
//   - ErrUnknownBlockType: Returned by ValidBlockType-gated code paths when a
//     ContentBlock.Type value is not in the BlockType closed vocabulary. Identical
//     forward-compat handling applies. Both errors wrap no inner error; callers that
//     need the specific unknown string value should capture it before calling this
//     package.
//
// Both sentinels are plain values (no wrapped context). Use errors.Is for matching.
//
// # Dependency Contract
//
// This package is permitted to import only the following standard library packages:
//
//   - encoding/json — for MarshalJSON / UnmarshalJSON implementations.
//   - errors — for ErrUnknownEventType and ErrUnknownBlockType sentinel definitions.
//   - fmt — for error message formatting in UnmarshalJSON dispatch.
//   - time — for ExchangeEvent.Timestamp (time.Time, always UTC).
//
// It MUST NOT import:
//
//   - internal/application — hexagonal rule: domain must not depend on application.
//   - internal/infrastructure — domain must not depend on infrastructure adapters.
//   - internal/interfaces — domain must not depend on the interface/CLI layer.
//   - Any third-party module — the domain layer must remain dependency-free to
//     prevent transitive version conflicts and to enable use in lightweight analysis
//     tooling outside the full AWF module graph.
//
// The test TestArchitecture_DomainTranscript_NoForbiddenImports (architecture_test.go)
// enforces this contract via AST import scanning at every test run. It asserts that
// across all non-test Go files in this package the only imports are the four stdlib
// packages listed above; any other import triggers a test failure.
package transcript
