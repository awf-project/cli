// Package acpserver implements a bidirectional JSON-RPC 2.0 engine over stdio
// for the Agent Communication Protocol (ACP). It provides a minimal, general-purpose
// server that handles inbound requests from an editor/client and can issue outbound
// requests back to that same client — a capability required by the ACP v1 permission
// callback flow (session/request_permission).
//
// # Stability and Layering
//
// This package lives under pkg/ and MUST have zero imports from
// github.com/awf-project/cli/internal/. This invariant is enforced by the
// architecture_test.go AST scan included in the package. External consumers can
// embed a Server without pulling in any internal AWF dependency.
//
// The server is a general-purpose JSON-RPC engine. ACP-specific semantics (method
// names, payload shapes, session lifecycle) live in the handlers registered by the
// caller, not in this package. This separation means the engine can be reused for
// future protocols without modification.
//
// Because the package is public, any breaking change to the exported surface is a
// SemVer break for the whole module. The exported surface is intentionally small:
// New, Server.RegisterHandler, Server.Serve, Server.CallClient, HandlerFunc, Error,
// plus the wire types Request, Response, Notification and the error-code constants
// defined in protocol.go.
//
// # Concurrency Model
//
// Serve dispatches each inbound request in its own goroutine (via sync.WaitGroup.Go).
// Handlers therefore run CONCURRENTLY with respect to each other and with the read
// loop. A long-running handler — such as one driving a workflow execution — does NOT
// block subsequent inbound frames (session/cancel, session/update) from being
// dispatched. Handlers MUST be safe for concurrent invocation: any state shared
// between handlers must be guarded by a mutex or equivalent synchronization primitive.
//
// The sync.WaitGroup (wg) tracks all in-flight handler goroutines. Serve's deferred
// cleanup calls cancel() followed by wg.Wait(), guaranteeing that every handler
// goroutine has returned before Serve itself returns — no goroutine leak survives
// Serve (SC-003).
//
// Notifications (frames without an ID) also dispatch a handler goroutine when a
// handler is registered for the method. No wire response is written for notifications
// per JSON-RPC 2.0 §5, but handler errors are logged at WARN level (M3).
//
// The stdin reader runs in a separate goroutine (scanLoop) and communicates with
// the main dispatch loop via a buffered channel. A context-cancellable io.Pipe
// interposes between the real stdin and scanLoop: when Serve's context is cancelled
// the pipe's write end is closed with the context error, causing scanLoop's
// blocking ReadSlice to unblock immediately rather than waiting for the next byte
// (M2 goroutine-leak fix). Stdin is forwarded into the pipe by a separate copier
// goroutine that runs for the lifetime of the underlying stdin reader and is not
// tracked in wg — it exits when the caller closes stdin (normal session end).
//
// The handler registry is guarded by a sync.RWMutex so RegisterHandler is safe to
// call from any goroutine. The canonical pattern is to register all handlers before
// calling Serve.
//
// # Bidirectional CallClient Rule
//
// Unlike a plain unidirectional request/response server, acpserver supports outbound
// calls via CallClient. All stdout writes — both inbound response writes and outbound
// CallClient request writes — serialize through a single writeMu-protected json.Encoder.
// Without this serialization, concurrent goroutines can interleave partial JSON frames
// and corrupt the stream (P0 data-integrity risk under concurrent load).
//
// Inbound frame demuxing works as follows: every received frame is probe-unmarshaled
// into a minimal {ID, Method} struct. If Method is empty and the ID matches a parked
// CallClient caller in the pendingCalls sync.Map, the frame is routed to that caller's
// response channel. Otherwise the frame is dispatched as a normal inbound request or
// silently discarded as a notification.
//
// Pending CallClient callers are tracked in a sync.Map keyed by a decimal string ID.
// IDs are generated from an atomically-incremented int64 counter, guaranteeing
// uniqueness within a single server instance without locks.
//
// # Panic Recovery Contract
//
// Handler panics are recovered in the dispatch path with defer/recover. The panic
// value is logged at WARN level via the injected slog.Logger (never written to stdout,
// which carries the JSON-RPC framing), together with the captured goroutine stack
// (debug.Stack) so a buggy handler is diagnosable post-mortem. The offending request
// receives an ErrInternal response with a generic, redacted message. The Serve loop
// continues; subsequent requests are handled normally.
//
// Stack traces are logged server-side only and are never forwarded to the client, to
// prevent information leakage — traces can reveal file paths, internal type names, and
// other detail useful for prompt-injection reconnaissance.
//
// # Response Wire Contract (JSON-RPC 2.0 §5)
//
// A Response always serializes the "result" member: success carries the handler's value,
// and an error response carries "result":null (present, never omitted) because the spec
// requires "result" to be null when "error" is present. Likewise the "id" member is
// always emitted, including the explicit "id":null literal for responses whose request
// id is unknown (parse error, oversize line). Only "result" and "id" of the Response,
// plus the optional Error.Data, follow this presence rule; Request and Notification keep
// omitempty on their optional members.
//
// # Lifecycle: Single-Use
//
// A Server instance binds to exactly one stdio session. The ready-channel handshake and
// output encoder are installed once and never reset, so Serve must be called at most
// once per Server: a second call returns an error rather than reusing the stale encoder
// or re-closing the already-closed ready channel. Callers needing a fresh session must
// construct a new Server via New. A clean stdin close (io.EOF) ends Serve with a nil
// error; any other stdin read error is surfaced as a wrapped error so a transport fault
// is distinguishable from an orderly shutdown.
//
// # Scanner Ceiling
//
// The stdin scanner buffer is grown to maxRequestLineBytes (10 MiB) at Serve startup.
// The bufio.Scanner default of 64 KiB is too small for legitimate ACP payloads such
// as base64-encoded files, large diffs, or multi-turn conversation context. The 10 MiB
// cap matches the agent providers' response body limit so neither direction silently
// truncates valid payloads.
//
// Lines that exceed the 10 MiB ceiling produce an ErrInvalidRequest response with
// id:null; the loop then continues processing subsequent frames (NFR-003 compliance).
// A ceiling violation must not crash the server or leave it in a broken state.
//
// # Notification Handling
//
// Inbound JSON-RPC notifications (frames without an ID field) MUST NOT produce a wire
// response per the JSON-RPC 2.0 specification §5. The server silently discards them.
// Handlers may be registered for notification method names (useful for side-effect
// processing), but any value returned by the handler is not written to the wire.
//
// # Error Codes
//
// The package exposes the standard JSON-RPC 2.0 error codes defined in protocol.go:
//
//   - ErrParse (-32700): the request could not be parsed as JSON.
//   - ErrInvalidRequest (-32600): the JSON was valid but not a valid JSON-RPC request.
//   - ErrMethodNotFound (-32601): no handler is registered for the requested method.
//   - ErrInvalidParams (-32602): the method exists but the params are malformed.
//   - ErrInternal (-32603): an internal server error (including recovered panics).
//
// NewParseErrorResponse constructs a well-formed parse-error response with "id":null
// as required by the JSON-RPC 2.0 spec (id is unknown when parsing fails).
//
// # Usage
//
// Register handlers and start the server:
//
//	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
//	srv := acpserver.New(logger)
//
//	srv.RegisterHandler("session/new", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
//	    var p struct {
//	        AgentID string `json:"agent_id"`
//	    }
//	    if err := json.Unmarshal(params, &p); err != nil {
//	        return nil, &acpserver.Error{Code: acpserver.ErrInvalidParams, Message: err.Error()}
//	    }
//	    return map[string]string{"session_id": "abc123"}, nil
//	})
//
//	// A handler can call back into the client to request a permission grant:
//	srv.RegisterHandler("session/prompt", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
//	    raw, err := srv.CallClient(ctx, "session/request_permission", map[string]string{
//	        "prompt": "Allow file write to /tmp/out.txt?",
//	    })
//	    if err != nil {
//	        return nil, &acpserver.Error{Code: acpserver.ErrInternal, Message: err.Error()}
//	    }
//	    return raw, nil
//	})
//
//	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
//	    log.Fatal(err)
//	}
package acpserver
