// Package transcript implements the infrastructure adapter that persists agent
// exchange events to an append-only JSONL file and broadcasts them to in-process
// subscribers over buffered channels.
//
// It is the concrete implementation of ports.Recorder declared in the domain
// layer. Its position in the hexagonal architecture: infrastructure depends
// inward on internal/domain/transcript (value types) and internal/domain/ports
// (the Recorder interface and Logger). No application or interface layer symbol
// may appear in this package's import graph.
//
// # Purpose
//
// This package closes the loop between event emission and durable storage.
// Every ExchangeEvent recorded by the domain flows through Recorder.Record,
// which enforces two invariants:
//
//  1. Disk-first ordering (FR-003): the event is written to the JSONL file
//     before it is broadcast to any subscriber. If the disk write fails the
//     broadcast is suppressed, so subscribers only see events that are durably
//     persisted.
//
//  2. Monotonic sequence numbers (D1): Seq is allocated by an atomic counter
//     owned exclusively by Recorder. The counter is never delegated to the
//     writer or the fanout so a single source guarantees ordering across
//     concurrent callers and across fan-out paths.
//
// # Public Surface
//
// The public surface consists of the following symbols:
//
//   - NewRecorder(path string, opts ...RecorderOption) (*Recorder, error)
//     Opens or creates the JSONL file at path (parent directories are created
//     with mode 0o700; the file itself is opened with mode 0o600 and O_APPEND).
//     Initializes the internal FanOut broadcaster. Returns an error if the
//     file cannot be opened. All functional options are applied before any
//     I/O so the masker and logger are available for the first Record call.
//
//   - (*Recorder).Record(ctx context.Context, event transcript.ExchangeEvent) error
//     Validates the event (Type must not be empty; returns ports.ErrInvalidEvent
//     otherwise), applies the masker if configured, allocates Seq if zero,
//     writes to the JSONL file, and on success publishes to all subscribers.
//     Honors ctx cancellation; returns ctx.Err() if the context is already done
//     before the write begins. Non-blocking: FanOut.Publish never blocks.
//
//   - (*Recorder).Subscribe() (<-chan transcript.ExchangeEvent, func())
//     Delegates to FanOut.Subscribe. Returns a buffered receive channel and an
//     idempotent cancel closure. The channel is closed when either the cancel
//     closure is called or Recorder.Close is invoked.
//
//   - (*Recorder).Close() error
//     Idempotent via sync.Once. Closes the FanOut (drains subscribers) then
//     the JSONLWriter (flushes the OS buffer and closes the file descriptor).
//     Second and subsequent calls return nil.
//
//   - RecorderOption (functional option type)
//     Passed to NewRecorder to tune behavior without breaking the constructor
//     signature. Available options are documented below under Internal Layout.
//
// # Internal Layout
//
// Three non-test files carry the implementation:
//
//   - recorder.go — Recorder struct, NewRecorder, Record, Subscribe, Close, and
//     the three functional options. Owns the atomic Seq counter (seq uint64).
//     Wires JSONLWriter and FanOut. Enforces disk-first ordering and nil-guards
//     the optional masker before applying it.
//
//   - jsonl_writer.go — JSONLWriter: mutex-protected O_APPEND file writer. Each
//     Write call marshals the event to JSON, appends a newline, and writes the
//     resulting bytes under a mutex. The mutex is held for the duration of the
//     write syscall to guarantee atomicity beyond PIPE_BUF for large payloads.
//     Parent directories are created with 0o700; the file is opened with 0o600.
//
//   - fanout.go — FanOut: bounded pub-sub broadcaster. Each subscriber receives
//     a buffered channel (default 256 events). When a subscriber's buffer is
//     full, the newest event is dropped (oldest preserved) and the drop count is
//     incremented atomically. WARN-level logging is rate-limited to one message
//     per subscriber per second to prevent log flooding during high-loss periods.
//     Subscribe and Close are idempotent via per-subscriber sync.Once semantics.
//
//   - reader.go — Reader: tolerant JSONL reader for audit and replay. Skips
//     empty lines; returns ErrLineMalformed (with line number context) on
//     malformed JSON but continues reading subsequent lines. Tolerates unknown
//     event types and block types without error (forward-compatibility policy).
//
// Functional options available via NewRecorder:
//
//   - WithFanOutBufferSize(size int) RecorderOption
//     Overrides the per-subscriber channel capacity (default 256). Must be
//     called before the first Subscribe; changing buffer size after subscription
//     has no effect on existing subscribers.
//
//   - WithRecorderLogger(logger ports.Logger) RecorderOption
//     Injects a structured logger for drop warnings and diagnostic output.
//     Defaults to ports.NopLogger. The same logger instance is forwarded to
//     the internal FanOut so all drop events share a single logger.
//
//   - WithMasker(fn func(transcript.ExchangeEvent) transcript.ExchangeEvent) RecorderOption
//     Registers a transformation function applied to every event in Record,
//     before the disk write. The masker receives the event with Seq already
//     allocated and must return a (possibly modified) ExchangeEvent. The
//     returned value is what is written to disk AND what is broadcast to
//     subscribers — masking is applied exactly once. Default is nil (no-op).
//
// # Threat Model
//
// The transcript infrastructure handles sensitive data (agent prompts, tool
// arguments, model responses). Threat scenarios addressed:
//
//   - Sensitive data at rest: The JSONL file is created with mode 0o600
//     (owner read/write only). Parent directories are created with 0o700.
//     No world-readable or group-readable bits are set. Operators running
//     AWF as a shared user must apply additional filesystem-level access
//     controls (ACLs, encrypted volumes) independently.
//
//   - Append-only integrity: The file is opened with O_APPEND which makes
//     each write atomic at the OS level for payloads up to PIPE_BUF (4 KiB
//     on Linux). For larger payloads the JSONLWriter mutex extends atomicity
//     to the full write. O_APPEND prevents seek-then-write races from
//     concurrent processes opening the same path (multi-process invariant
//     tested in jsonl_writer_atomicity_test.go).
//
//   - Secret masking — deferred opt-in policy: Secret masking is NOT applied
//     by default. Callers that require masking must inject WithMasker. This
//     deferred opt-in policy is intentional (Notes§6): shipping a masker
//     without a well-defined secret catalog would silently miss values,
//     creating a false sense of protection. Callers that configure WithMasker
//     are responsible for maintaining the catalog of sensitive field paths.
//     The masker is applied before both the disk write and the fan-out
//     broadcast so no raw secret escapes to either path once configured.
//
//   - Subscriber isolation: Each subscriber's channel is independent and
//     bounded. A slow subscriber cannot stall the Record call (FanOut.Publish
//     is non-blocking), cannot exhaust memory beyond bufferSize events per
//     subscriber, and cannot observe events that failed the disk write.
//     Drop counts are tracked atomically per subscriber and in aggregate via
//     FanOut.Stats.
//
//   - Zero-value event guard: Record returns ports.ErrInvalidEvent immediately
//     when event.Type is empty. This prevents zero-value ExchangeEvent structs
//     (common programming mistakes) from polluting the transcript or the
//     subscriber channels. The guard is checked before the masker is applied
//     so the masker never receives invalid input.
//
// # Error Taxonomy
//
// Errors fall into three classes:
//
//   - ports.ErrInvalidEvent: Returned by Record when event.Type is empty.
//     The caller must supply a well-formed event. No file I/O is attempted.
//
//   - Write errors (fmt.Errorf wrapping os.File errors): Returned by Record
//     when the JSON marshal or the file write fails. The broadcast is
//     suppressed. The file descriptor remains open; subsequent Record calls
//     may succeed if the underlying condition (full disk, revoked permission)
//     is resolved. Callers that require guaranteed delivery must handle write
//     errors and retry or escalate.
//
//   - Constructor errors (fmt.Errorf wrapping os.MkdirAll / os.OpenFile
//     errors): Returned by NewRecorder when the file or its parent directory
//     cannot be created or opened. The Recorder is nil; no resources are
//     allocated. Callers must treat a non-nil constructor error as fatal.
//
// # Dependency Contract
//
// This package is permitted to import:
//
//   - Standard library (bufio, context, encoding/json, errors, fmt, io, os,
//     path/filepath, sync, sync/atomic, time)
//   - github.com/google/uuid — used by FanOut to key subscribers.
//   - go.uber.org/zap — permitted transitively via ports.Logger; the package
//     does not import zap directly but the Logger interface is compatible.
//   - internal/domain/transcript — ExchangeEvent, EventType, ContentBlock,
//     and payload types. These are the only domain types referenced directly.
//   - internal/domain/ports — ports.Recorder (interface implemented here),
//     ports.Logger (injected via functional option), ports.ErrInvalidEvent,
//     ports.NopLogger (default logger).
//
// It MUST NOT import:
//
//   - internal/application — hexagonal rule: infrastructure must not depend
//     on the application layer.
//   - internal/interfaces — same hexagonal rule.
//   - Any other internal package not listed above.
package transcript
