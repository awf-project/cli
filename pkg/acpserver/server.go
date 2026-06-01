package acpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// maxRequestLineBytes is the per-line ceiling for the JSON-RPC stdin scanner.
// The bufio.Scanner default (64 KiB) is far too small for legitimate ACP payloads
// (base64 images, large diffs, long prompts). We size it to 10 MiB so neither
// direction silently truncates, while still bounding adversarial input (NFR-003).
const maxRequestLineBytes = 10 * 1024 * 1024

// HandlerFunc handles a single inbound JSON-RPC method call.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, *Error)

// scanResult carries one line (or a scan error) from the stdin reader goroutine.
// A clean EOF is represented as {err: io.EOF}. oversize marks a line that exceeded
// maxRequestLineBytes and was skipped — the server stays alive and answers an error.
type scanResult struct {
	line     []byte
	err      error
	oversize bool
}

// rawResponse carries a demuxed inbound response back to a parked CallClient.
type rawResponse struct {
	result json.RawMessage
	err    error
}

// inboundFrame is the probe shape used to demux a single inbound JSON-RPC frame
// into one of: a response to a pending CallClient, an inbound request, or a
// notification. Capturing result/error lets the demux route client replies.
type inboundFrame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *Error          `json:"error"`
}

// Server is a bidirectional JSON-RPC 2.0 server over stdio. Zero value is not valid; use New.
//
// A Server is single-use: it binds to exactly one stdio session. The ready/enc
// handshake (closed once, set once) is not reset between calls, so Serve must be
// invoked at most once per Server. A second call returns errAlreadyServed rather
// than silently reusing the stale encoder or re-closing the ready channel. Create
// a fresh Server via New for each session.
type Server struct {
	mu           sync.RWMutex
	handlers     map[string]HandlerFunc
	pendingCalls sync.Map // string ID → chan rawResponse
	counter      atomic.Int64
	writeMu      sync.Mutex
	enc          *json.Encoder
	logger       *slog.Logger
	ready        chan struct{} // closed once Serve has installed the output encoder
	readyOnce    sync.Once
	served       atomic.Bool    // guards single-use: set on the first Serve call
	wg           sync.WaitGroup // tracks in-flight request handler goroutines
}

// errAlreadyServed is returned when Serve is invoked more than once on the same
// Server. The stdio session handshake is single-use; callers must create a new
// Server via New for each session.
var errAlreadyServed = errors.New("acpserver: Serve already called; Server is single-use")

// New returns a Server with an empty handler registry. A nil logger falls back to slog.Default().
func New(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		handlers: make(map[string]HandlerFunc),
		logger:   logger,
		ready:    make(chan struct{}),
	}
}

// RegisterHandler registers a handler for the given JSON-RPC method name.
func (s *Server) RegisterHandler(method string, h HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

// Serve reads newline-delimited JSON-RPC 2.0 frames from in and writes responses to out
// until ctx is cancelled or in returns EOF. Stdin is consumed in a dedicated goroutine so
// that context cancellation unblocks the loop even when no bytes arrive. Returns nil on a
// clean shutdown (EOF or ctx cancel).
//
// Caller contract: the caller must close in after Serve returns to allow the internal
// copier goroutine to exit. The copier reads from the real stdin and runs beyond Serve's
// lifetime; it exits only when in is closed (io.EOF) or when a write to the internal pipe
// fails after the pipe is closed on context cancellation. Failing to close in will not
// cause a goroutine leak inside Serve itself (the copier is intentionally not tracked in
// the WaitGroup), but it will leave the copier goroutine blocked in Read until the
// underlying reader is eventually closed by the OS at process exit.
//
// Serve is single-use: a second call on the same Server returns errAlreadyServed
// without touching the (already installed) encoder or the ready channel.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	if !s.served.CompareAndSwap(false, true) {
		return errAlreadyServed
	}

	// serveCtx is cancelled when Serve returns (EOF, ctx cancel, or fatal scan error),
	// so every in-flight request handler — and the workflow execution it drives — unwinds.
	// The deferred drain then waits for those handlers to finish, guaranteeing no goroutine
	// leak survives Serve (SC-003, verified by the goroutine-leak integration test).
	serveCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		// P1 — goroutine leak prevention: close in before wg.Wait() so the copier
		// goroutine (which is NOT tracked in wg) can unblock from its Read(in) call.
		// Order matters: cancel() fires first (unblocks closer via serveCtx.Done),
		// then we attempt to close in (unblocks copier's Read), then wg.Wait() drains
		// closer. If we called wg.Wait() first, closer might complete before the
		// copier unblocks — the copier would still exit eventually (on pipe close),
		// but closing in here ensures it exits promptly and does not outlive Serve.
		if c, ok := in.(io.Closer); ok {
			_ = c.Close()
		}
		s.wg.Wait()
	}()

	s.writeMu.Lock()
	s.enc = json.NewEncoder(out)
	s.writeMu.Unlock()
	s.readyOnce.Do(func() { close(s.ready) })

	// scanCh carries lines from the reader goroutine. Buffer of 1 avoids head-of-line
	// blocking between the goroutine and the dispatch loop.
	scanCh := make(chan scanResult, 1)

	// Option B cancellable reader (M2): wrap in in an io.Pipe so that when
	// serveCtx is cancelled the pipe writer is closed with the context error,
	// which causes ReadSlice inside scanLoop to return immediately instead of
	// blocking indefinitely on a still-open stdin (e.g. a long-lived editor
	// connection that never sends EOF). This guarantees scanLoop terminates
	// and is drained by the wg.Wait in the defer above (SC-003).
	//
	// Design constraints:
	//   - copier reads from the real stdin (in), which may block indefinitely.
	//     It runs outside wg so Serve is never held up waiting for it — the
	//     copier stays alive until in is closed by the caller (normal lifecycle).
	//   - closer watches serveCtx.Done and calls pipeWriter.CloseWithError,
	//     which unblocks scanLoop's ReadSlice.  The copier's next write to
	//     pipeWriter will then fail with io.ErrClosedPipe and it will exit.
	//   - scanLoop is also outside wg (launched below with go), but it exits
	//     as soon as pipeReader is closed, so it terminates before wg.Wait
	//     returns.
	//   - closer is tracked in wg so the defer waits for it, guaranteeing
	//     the pipe is always closed before Serve returns — via closer or copier.
	//     Multiple closes are safe: io.Pipe.Close and CloseWithError are idempotent.
	pipeReader, pipeWriter := io.Pipe()
	copierDone := make(chan struct{})

	// copier: forwards bytes from the real stdin into the pipe. Not tracked
	// in wg because it may block in Read(in) beyond Serve's lifetime — the
	// caller is responsible for closing in when the session ends. When the
	// pipe writer is closed (by closer), the next pipeWriter.Write call
	// returns io.ErrClosedPipe and io.Copy exits, closing copierDone.
	//
	// Non-EOF read errors from in are forwarded via CloseWithError so that
	// scanLoop propagates the original transport fault back through Serve
	// rather than treating the error as a clean EOF.
	go func() {
		defer close(copierDone)
		_, copyErr := io.Copy(pipeWriter, in)
		if copyErr != nil && !errors.Is(copyErr, io.ErrClosedPipe) {
			// Real read fault: surface it through the pipe so scanLoop and
			// ultimately Serve return the wrapped transport error.
			pipeWriter.CloseWithError(copyErr)
		} else {
			// EOF or pipe already closed: clean shutdown.
			_ = pipeWriter.Close()
		}
	}()

	// closer: unblocks scanLoop when the context is cancelled, before in
	// reaches EOF. Tracked in wg so the deferred wg.Wait guarantees this
	// goroutine has run CloseWithError before Serve returns.
	s.wg.Go(func() {
		select {
		case <-serveCtx.Done():
			// P4 — avoid double-close on pipeWriter: if copier has already
			// closed pipeWriter with its own error (real transport fault), do
			// not overwrite that error with context.Canceled / context.DeadlineExceeded.
			// Preserving the copier's original error lets the dispatch loop
			// (and ultimately the Serve caller) distinguish a real I/O failure
			// from a normal context-driven shutdown.
			select {
			case <-copierDone:
				// copier already closed pipeWriter with its own error; do not overwrite.
			default:
				pipeWriter.CloseWithError(serveCtx.Err())
			}
		case <-copierDone:
			// copier already closed the writer; nothing to do.
		}
	})

	go s.scanLoop(serveCtx, pipeReader, scanCh)

	for {
		select {
		case <-serveCtx.Done():
			return nil
		case sr := <-scanCh:
			if done, err := s.dispatchScanResult(serveCtx, sr); done {
				return err
			}
		}
	}
}

// scanLoop reads newline-delimited frames from in and forwards each as a scanResult on
// scanCh until serveCtx is cancelled or the stream ends. It runs in its own goroutine so
// context cancellation can unblock Serve even when no bytes arrive; every send races
// serveCtx.Done() so a shutdown never blocks the reader.
func (s *Server) scanLoop(serveCtx context.Context, in io.Reader, scanCh chan<- scanResult) {
	reader := bufio.NewReaderSize(in, 64*1024)
	for {
		line, tooLong, err := readLine(reader, maxRequestLineBytes)
		switch {
		case tooLong:
			select {
			case scanCh <- scanResult{oversize: true}:
			case <-serveCtx.Done():
				return
			}
		case len(line) > 0:
			select {
			case scanCh <- scanResult{line: line}:
			case <-serveCtx.Done():
				return
			}
		}
		if err != nil {
			select {
			case scanCh <- scanResult{err: err}:
			case <-serveCtx.Done():
				// Serve is already shutting down, so the dispatch loop will never read this
				// result. A non-EOF read fault would otherwise vanish silently; log it so a
				// transport fault during shutdown stays diagnosable (M5).
				if !errors.Is(err, io.EOF) {
					s.logger.Warn("acpserver: stdin read error dropped during shutdown", "err", err)
				}
			}
			return
		}
	}
}

// dispatchScanResult processes one scan result from the stdin reader goroutine. It returns
// done=true when the Serve loop must stop, carrying the shutdown error (nil for a clean
// io.EOF, a wrapped error for a real stdin I/O fault). done=false means keep serving.
func (s *Server) dispatchScanResult(serveCtx context.Context, sr scanResult) (done bool, err error) {
	switch {
	case sr.oversize:
		// Skip the oversize line but keep serving (NFR-003): emit a structured error
		// (id:null) rather than crashing or terminating the connection.
		s.writeOrLog(Response{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &Error{Code: ErrInvalidRequest, Message: "request line exceeds maximum size"},
		})
		return false, nil
	case sr.err != nil:
		// io.EOF is the editor closing stdin → clean shutdown (nil error). Any other read
		// error (broken pipe, I/O failure) is surfaced so the caller can distinguish an
		// orderly close from a transport fault.
		if errors.Is(sr.err, io.EOF) {
			return true, nil
		}
		return true, fmt.Errorf("acpserver: stdin read error: %w", sr.err)
	case len(sr.line) == 0:
		return false, nil
	default:
		s.handle(serveCtx, sr.line)
		return false, nil
	}
}

// readLine reads a single newline-terminated line from r, returning the line (including
// the trailing newline). If the line exceeds max bytes it is fully drained from the stream
// and reported via tooLong=true with an empty line, so the caller can answer an error and
// keep serving — unlike bufio.Scanner, which cannot resume after ErrTooLong.
func readLine(r *bufio.Reader, limit int) (line []byte, tooLong bool, err error) {
	for {
		chunk, readErr := r.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > limit {
			// Drain the remainder of the physical line so the next call starts
			// cleanly. If the drain itself hits an I/O error (not ErrBufferFull),
			// report ONLY the transport error — do NOT also set tooLong, because
			// that would cause the caller to emit a spurious ErrInvalidRequest
			// response followed immediately by the transport-error shutdown (M4).
			// A broken-pipe during drain is a fatal transport fault, not an
			// application-level oversize violation.
			drainErr := readErr
			for errors.Is(drainErr, bufio.ErrBufferFull) {
				_, drainErr = r.ReadSlice('\n')
			}
			if drainErr != nil && !errors.Is(drainErr, io.EOF) {
				return nil, false, fmt.Errorf("acpserver: drain oversize line: %w", drainErr)
			}
			return nil, true, nil
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		if readErr != nil {
			return line, false, fmt.Errorf("acpserver: read line: %w", readErr)
		}
		return line, false, nil
	}
}

// handle demuxes and processes a single inbound frame.
func (s *Server) handle(ctx context.Context, line []byte) {
	var fr inboundFrame
	if err := json.Unmarshal(line, &fr); err != nil {
		// JSON-RPC 2.0 §5.1: an unparsable frame has an unknown id, so the response MUST
		// use an explicit "id": null.
		s.writeOrLog(NewParseErrorResponse())
		return
	}

	// Inbound response to a parked CallClient? (no method, id matches a pending call)
	if fr.Method == "" && len(fr.ID) > 0 {
		key := normalizeID(fr.ID)
		if chAny, found := s.pendingCalls.Load(key); found {
			ch, ok := chAny.(chan rawResponse)
			if !ok {
				return
			}
			rr := rawResponse{result: fr.Result}
			if fr.Error != nil {
				rr.err = fmt.Errorf("acpserver: client error %d: %s", fr.Error.Code, fr.Error.Message)
			}
			select {
			case ch <- rr:
			default: // caller already unparked (e.g. ctx cancelled); drop silently
			}
		}
		return
	}
	if fr.Method == "" {
		return // neither a request nor a known response — ignore
	}

	// Inbound request (id present) or notification (id absent).
	isNotification := len(fr.ID) == 0
	s.mu.RLock()
	h, ok := s.handlers[fr.Method]
	s.mu.RUnlock()

	if !ok {
		if !isNotification {
			s.writeOrLog(Response{
				JSONRPC: "2.0",
				ID:      fr.ID,
				Error:   &Error{Code: ErrMethodNotFound, Message: "method not found: " + fr.Method},
			})
		}
		return
	}

	// Dispatch each request in its own goroutine so a long-running handler (e.g. a
	// session/prompt driving a workflow) never blocks the read loop — concurrent
	// session/cancel and session/update traffic must keep flowing. Writes stay
	// serialized through writeMu, so concurrent responses cannot interleave bytes.
	// The WaitGroup lets Serve drain all handlers on shutdown (no goroutine leak).
	id := fr.ID
	params := fr.Params
	s.wg.Go(func() {
		result, rpcErr := s.invoke(ctx, h, params)
		if isNotification {
			// JSON-RPC 2.0: notifications never receive a wire response, but a
			// handler error still warrants a server-side diagnostic log so
			// notification processing failures are not silently discarded (M3).
			if rpcErr != nil {
				s.logger.Warn(
					"acpserver: notification handler returned error",
					"method", fr.Method,
					"code", rpcErr.Code,
					"message", rpcErr.Message,
				)
			}
			return
		}
		resp := Response{JSONRPC: "2.0", ID: id}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
		s.writeOrLog(resp)
	})
}

// invoke calls a handler with panic recovery. A panic is logged at WARN and converted
// into an ErrInternal response so a single buggy handler cannot kill the loop.
func (s *Server) invoke(ctx context.Context, h HandlerFunc, params json.RawMessage) (result any, rpcErr *Error) {
	defer func() {
		if r := recover(); r != nil {
			// Capture the stack so a buggy handler is diagnosable post-mortem; the
			// loop itself stays alive and answers ErrInternal.
			s.logger.Warn(
				"acpserver: handler panic recovered",
				"panic", r,
				"stack", string(debug.Stack()),
			)
			result = nil
			rpcErr = &Error{Code: ErrInternal, Message: "internal error"}
		}
	}()
	return h(ctx, params)
}

// errNotServing is the sentinel returned by writeFrame when Serve has not been called yet
// or has already returned (enc == nil). writeOrLog checks with errors.Is rather than
// comparing error strings, avoiding a fragile string-equality test (M-3 fix).
var errNotServing = errors.New("acpserver: server not serving")

// writeFrame serializes one frame to the output, serialized through writeMu so concurrent
// inbound responses and outbound CallClient requests cannot interleave bytes.
func (s *Server) writeFrame(v any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.enc == nil {
		return errNotServing
	}
	if err := s.enc.Encode(v); err != nil {
		return fmt.Errorf("acpserver: encode frame: %w", err)
	}
	return nil
}

// writeOrLog writes a fire-and-forget frame (an inbound response), logging at WARN on
// failure. The dispatch loop cannot propagate a write error to a caller, so a broken
// pipe is logged rather than crashing the loop.
//
// P11 — disambiguate log cause: the "serving" field lets operators distinguish between
// two distinct failure modes:
//   - serving=false: encoder was nil, i.e. Serve was not yet called or already
//     returned. The error text is "acpserver: server not serving".
//   - serving=true: encoder was present but Encode() itself failed, indicating a
//     real I/O fault on the underlying transport (e.g. broken pipe, full buffer).
//
// Inspecting the error text is intentional: writeFrame holds writeMu while checking
// s.enc, so re-reading s.enc here (outside writeMu) would race with Serve's
// assignment. Using the error sentinel avoids a second lock acquisition.
func (s *Server) writeOrLog(v any) {
	if err := s.writeFrame(v); err != nil {
		// errNotServing is the sentinel from writeFrame when s.enc == nil (M-3 fix).
		serving := !errors.Is(err, errNotServing)
		s.logger.Warn(
			"acpserver: failed to write response frame",
			"err", err,
			"serving", serving,
		)
	}
}

// CallClient issues an outbound JSON-RPC 2.0 request to the client and waits for the
// matching response (or ctx cancellation). It is the single bidirectional primitive
// used by ACP for session/request_permission callbacks.
//
// Ordering invariant — ghost-ID prevention:
//
//  1. The request ID is generated first (atomic increment).
//  2. writeFrame transmits the request over the wire BEFORE the ID is registered in
//     pendingCalls. This eliminates the "ghost-ID" window: if writeFrame fails, the
//     ID was never stored, so the dispatch loop (handle) can never route a stray
//     response to an orphaned channel. A well-behaved client cannot send a response
//     before it receives the request; even a misbehaving client cannot route a reply
//     to an ID that was never inserted into pendingCalls.
//  3. Only after writeFrame succeeds do we Store the channel and defer its Delete.
//     The deferred Delete ensures the entry is removed regardless of how the wait
//     resolves (response received, ctx cancelled, or any future return path).
//
// The single remaining theoretical race — a client responding faster than Store
// completes — is benign on all Go-memory-model-compliant transports: the response
// bytes cannot arrive at the dispatch goroutine before writeFrame's Encode call
// returns on the writing goroutine (both sides of the pipe are synchronized through
// the kernel or the io.Pipe implementation). The buffered channel (cap 1) ensures
// that even if handle routes a response before the select below runs, the send in
// handle never blocks.
func (s *Server) CallClient(ctx context.Context, method string, params any) (json.RawMessage, error) {
	// Wait until Serve has installed the output encoder (or the caller's ctx is done).
	select {
	case <-s.ready:
	case <-ctx.Done():
		return nil, fmt.Errorf("acpserver: %w", ctx.Err())
	}

	idStr := strconv.FormatInt(s.counter.Add(1), 10)

	var paramsBytes json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("acpserver: marshal params: %w", err)
		}
		paramsBytes = b
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(strconv.Quote(idStr)),
		Method:  method,
		Params:  paramsBytes,
	}

	// Transmit BEFORE registering in pendingCalls (ghost-ID prevention, see above).
	if err := s.writeFrame(req); err != nil {
		return nil, fmt.Errorf("acpserver: write request: %w", err)
	}

	// Register the response channel only after the write succeeds. Any response
	// from the client is guaranteed to arrive after this point (wire ordering),
	// so no reply can be lost between writeFrame and Store.
	ch := make(chan rawResponse, 1)
	s.pendingCalls.Store(idStr, ch)
	defer s.pendingCalls.Delete(idStr)

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("acpserver: %w", ctx.Err())
	case rr := <-ch:
		if rr.err != nil {
			return nil, rr.err
		}
		return rr.result, nil
	}
}

// Notify sends a one-way JSON-RPC 2.0 notification (no id, no response expected) to the
// client. It is used for server-originated streaming updates such as session/update.
// Writes are serialized through writeMu so a notification can never interleave with a
// response or an outbound CallClient request. It waits for Serve to install the encoder
// (or for ctx to cancel) before writing.
func (s *Server) Notify(ctx context.Context, method string, params any) error {
	select {
	case <-s.ready:
	case <-ctx.Done():
		return fmt.Errorf("acpserver: %w", ctx.Err())
	}

	var paramsBytes json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("acpserver: marshal notification params: %w", err)
		}
		paramsBytes = b
	}

	if err := s.writeFrame(Notification{JSONRPC: "2.0", Method: method, Params: paramsBytes}); err != nil {
		return fmt.Errorf("acpserver: write notification: %w", err)
	}
	return nil
}

// normalizeID returns a canonical string key for a JSON-RPC id, unquoting JSON string
// ids so that `"1"` and the stored decimal key "1" compare equal.
func normalizeID(raw json.RawMessage) string {
	str := strings.TrimSpace(string(raw))
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		var unq string
		if err := json.Unmarshal(raw, &unq); err == nil {
			return unq
		}
	}
	return str
}
