package acpserver_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/acpserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingReader is an io.Reader that blocks until the done channel is closed.
// Used to test context cancellation unblocks Serve without requiring stdin to close.
type blockingReader struct {
	done chan struct{}
	once sync.Once
	buf  []byte
}

func newBlockingReader(initial string) *blockingReader {
	return &blockingReader{done: make(chan struct{}), buf: []byte(initial)}
}

func (r *blockingReader) Close() {
	r.once.Do(func() { close(r.done) })
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}
	<-r.done
	return 0, io.EOF
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

// runClient simulates the editor side of the ACP connection. It reads each frame the
// server writes to r, asserts the frame is well-formed (catches interleaved/corrupt
// writes), and for every outbound CallClient request (method + id present) replies to w
// with a result response echoing the id. It returns a channel closed when r reaches EOF.
func runClient(t *testing.T, r io.Reader, w io.Writer, result any) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		resBytes, _ := json.Marshal(result)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			var fr struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &fr); err != nil {
				t.Errorf("client received corrupt/interleaved frame: %q", scanner.Bytes())
				continue
			}
			if fr.Method != "" && len(fr.ID) > 0 {
				reply := `{"jsonrpc":"2.0","id":` + string(fr.ID) + `,"result":` + string(resBytes) + "}\n"
				if _, err := io.WriteString(w, reply); err != nil {
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			t.Errorf("runClient: scanner error: %v", err)
		}
	}()
	return done
}

func TestNew_ReturnsServer(t *testing.T) {
	srv := acpserver.New(discardLogger())
	require.NotNil(t, srv, "New should return a non-nil server")
}

func TestRegisterHandler_StoresHandler(t *testing.T) {
	srv := acpserver.New(discardLogger())
	called := false

	handler := func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		called = true
		return "pong", nil
	}

	srv.RegisterHandler("ping", handler)

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	assert.True(t, called, "handler should have been called")
}

func TestServe_HandlesValidRequest(t *testing.T) {
	srv := acpserver.New(discardLogger())
	srv.RegisterHandler("test_method", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		return map[string]string{"status": "ok"}, nil
	})

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test_method"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	var resp acpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")
	assert.Equal(t, json.RawMessage("1"), resp.ID, "response ID should match request ID")
	assert.Nil(t, resp.Error, "response should not have error")
}

func TestServe_NotificationsProduceNoResponse(t *testing.T) {
	srv := acpserver.New(discardLogger())
	handlerCalled := false

	srv.RegisterHandler("my_notification", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		handlerCalled = true
		return nil, nil
	})

	stdin := strings.NewReader(`{"jsonrpc":"2.0","method":"my_notification"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	assert.Empty(t, stdout.String(), "notifications must not produce any response")
	assert.True(t, handlerCalled, "notification handler should still be called")
}

func TestServe_ParseError_ReturnsErrParse(t *testing.T) {
	srv := acpserver.New(discardLogger())

	stdin := strings.NewReader(`{invalid json}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	responseStr := stdout.String()
	require.NotEmpty(t, responseStr, "parse error should produce a response")

	var resp acpserver.Response
	err := json.Unmarshal([]byte(responseStr), &resp)
	require.NoError(t, err, "response should be valid JSON")

	assert.NotNil(t, resp.Error, "response should contain error")
	assert.Equal(t, acpserver.ErrParse, resp.Error.Code, "error code should be ErrParse (-32700)")
	assert.Contains(t, responseStr, `"id":null`, "parse error response must have id:null")
}

func TestServe_MethodNotFound_ReturnsError(t *testing.T) {
	srv := acpserver.New(discardLogger())

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"unknown_method"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	var resp acpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")

	require.NotNil(t, resp.Error, "response should contain error")
	assert.Equal(t, acpserver.ErrMethodNotFound, resp.Error.Code, "error code should be ErrMethodNotFound")
	assert.Contains(t, resp.Error.Message, "unknown_method", "error message should mention the method name")
}

func TestServe_HandlerPanic_RecoveredAndLogged(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	srv := acpserver.New(logger)
	srv.RegisterHandler("panic_method", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		panic("handler panic")
	})

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"panic_method"}` + "\n")
	stdout := &bytes.Buffer{}

	// Finite stdin: Serve returns on natural EOF after the panic is recovered and
	// its error frame is written. Avoid a hard deadline that can flake under -race.
	_ = srv.Serve(context.Background(), stdin, stdout)

	var firstResp acpserver.Response
	err := json.NewDecoder(stdout).Decode(&firstResp)
	require.NoError(t, err, "first response should be valid JSON")

	assert.NotNil(t, firstResp.Error, "panic should produce error response")
	assert.Equal(t, acpserver.ErrInternal, firstResp.Error.Code, "error code should be ErrInternal")

	// MINOR-2: the recovered panic must be logged with a stack trace for post-mortem.
	logged := logBuf.String()
	assert.Contains(t, logged, "handler panic recovered", "panic recovery should be logged")
	assert.Contains(t, logged, "stack=", "panic log must include a stack trace")
}

// TestServe_NotificationHandlerError_IsLogged asserts that when a notification
// handler returns a non-nil *Error, the error is logged at WARN level (M3) and
// no response frame is written (JSON-RPC 2.0 §5 forbids responses to
// notifications).
func TestServe_NotificationHandlerError_IsLogged(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	srv := acpserver.New(logger)

	srv.RegisterHandler("notify/fail", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		return nil, &acpserver.Error{Code: acpserver.ErrInternal, Message: "handler failed"}
	})

	// A notification frame has no "id" field.
	stdin := strings.NewReader(`{"jsonrpc":"2.0","method":"notify/fail"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	// M3: No wire response must be written for a notification.
	assert.Empty(t, stdout.String(), "notification must not produce a wire response even on error")

	// M3: The error must be logged at WARN level with method, code, message.
	logged := logBuf.String()
	assert.Contains(t, logged, "notification handler returned error",
		"notification handler error must be logged")
	assert.Contains(t, logged, "notify/fail",
		"log entry must include the method name")
}

func TestServe_OversizeLineProducesError(t *testing.T) {
	srv := acpserver.New(discardLogger())

	largePayload := strings.Repeat("x", 11*1024*1024)
	input := `{"jsonrpc":"2.0","id":1,"method":"test","params":"` + largePayload + `"}` + "\n"
	stdin := strings.NewReader(input)
	stdout := &bytes.Buffer{}

	// stdin is a finite strings.Reader: Serve returns naturally on EOF once the
	// oversize line has been drained and its error frame written. A wall-clock
	// deadline here is unnecessary and flakes under -race when draining the 11 MiB
	// line races the timeout, so we rely on the natural EOF instead.
	_ = srv.Serve(context.Background(), stdin, stdout)

	var resp acpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")

	assert.NotNil(t, resp.Error, "oversize line should produce error response")
	assert.Equal(t, acpserver.ErrInvalidRequest, resp.Error.Code, "error code should be ErrInvalidRequest")
}

func TestServe_ContextCancelUnblocks(t *testing.T) {
	srv := acpserver.New(discardLogger())

	reader := newBlockingReader("")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithCancel(context.Background())

	serveComplete := make(chan error)
	go func() {
		serveComplete <- srv.Serve(ctx, reader, stdout)
	}()

	// Deterministically wait until Serve has installed its output encoder instead of
	// sleeping a fixed interval (m1): Notify blocks on the server's ready signal and only
	// returns once Serve is running, so the subsequent cancel exercises a live Serve loop.
	require.NoError(t, srv.Notify(ctx, "test/ready", nil))
	cancel()
	reader.Close()

	select {
	case err := <-serveComplete:
		assert.NoError(t, err, "Serve should return when context is cancelled")
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Serve did not unblock within 50ms of context cancellation")
	}
}

func TestCallClient_RoundTripsRequest(t *testing.T) {
	srv := acpserver.New(discardLogger())

	in, inWriter := io.Pipe()
	outReader, out := io.Pipe()

	serveComplete := make(chan error, 1)
	go func() {
		serveComplete <- srv.Serve(context.Background(), in, out)
	}()
	clientDone := runClient(t, outReader, inWriter, map[string]bool{"granted": true})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := srv.CallClient(ctx, "session/request_permission", map[string]string{"resource": "test"})
	require.NoError(t, err, "CallClient should not error on valid response")
	require.NotNil(t, result, "CallClient should return result")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(result, &parsed), "result should be valid JSON")
	assert.Equal(t, true, parsed["granted"], "result should contain expected data")

	inWriter.Close()
	<-serveComplete
	out.Close()
	<-clientDone
}

func TestCallClient_ContextCancelation(t *testing.T) {
	srv := acpserver.New(discardLogger())

	stdin := strings.NewReader("")
	out := &bytes.Buffer{}

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = srv.Serve(context.Background(), stdin, out)
	}()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	result, err := srv.CallClient(ctx, "test_method", nil)

	assert.Nil(t, result, "CallClient should return nil result when context cancelled")
	require.Error(t, err, "CallClient should return error when context cancelled")
	assert.ErrorIs(t, err, context.Canceled, "error should be context.Canceled")
	<-serveDone
}

func TestServer_OutboundWritesDoNotInterleave(t *testing.T) {
	srv := acpserver.New(discardLogger())

	in, inWriter := io.Pipe()
	outReader, out := io.Pipe()

	serveComplete := make(chan error, 1)
	go func() {
		serveComplete <- srv.Serve(context.Background(), in, out)
	}()
	// The client validates every frame it reads — a corrupt frame means the writeMu
	// failed to serialize concurrent writes — and replies so each CallClient unparks.
	clientDone := runClient(t, outReader, inWriter, map[string]bool{"ok": true})

	var wg sync.WaitGroup
	const numGoroutines = 100
	var successCount atomic.Int32

	for range numGoroutines {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if _, err := srv.CallClient(ctx, "increment", map[string]int{"value": 1}); err == nil {
				successCount.Add(1)
			}
		})
	}
	wg.Wait()

	inWriter.Close()
	<-serveComplete
	out.Close()
	<-clientDone

	assert.Positive(t, successCount.Load(), "all CallClient calls should succeed under concurrency")
}

func TestHandlerFuncSignature(t *testing.T) {
	srv := acpserver.New(discardLogger())

	handler := func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		return map[string]string{"ok": "true"}, nil
	}

	srv.RegisterHandler("test", handler)

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	var resp acpserver.Response
	err := json.NewDecoder(stdout).Decode(&resp)
	require.NoError(t, err, "response should be valid JSON")
	assert.Nil(t, resp.Error, "handler should be called and return no error")
}

func TestServe_MultipleRequests(t *testing.T) {
	srv := acpserver.New(discardLogger())
	var counter atomic.Int64

	// The server dispatches each request in its own goroutine (so a long-running
	// session/prompt never blocks concurrent session/cancel traffic). Responses may
	// therefore arrive in any order, and handler state must be concurrency-safe — this
	// test asserts the SET of returned IDs rather than their delivery order.
	srv.RegisterHandler("increment", func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		return map[string]int64{"count": counter.Add(1)}, nil
	})

	stdin := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"increment"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"increment"}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"increment"}` + "\n",
	)
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = srv.Serve(ctx, stdin, stdout)

	decoder := json.NewDecoder(stdout)
	gotIDs := map[string]bool{}
	for i := range 3 {
		resp := &acpserver.Response{}
		require.NoError(t, decoder.Decode(resp), "response %d should be valid", i)
		assert.Nil(t, resp.Error, "handler should not error")
		gotIDs[string(resp.ID)] = true
	}

	assert.Equal(t, map[string]bool{"1": true, "2": true, "3": true}, gotIDs,
		"all three request IDs must be answered exactly once, in any order")
	assert.Equal(t, int64(3), counter.Load(), "handler must run exactly three times")
}

// errReader returns the configured payload once, then a non-EOF I/O error. It models a
// transport fault (broken pipe, device error) so we can assert Serve distinguishes it
// from a clean EOF shutdown.
type errReader struct {
	payload []byte
	err     error
	done    bool
}

func (r *errReader) Read(p []byte) (int, error) {
	if !r.done && len(r.payload) > 0 {
		n := copy(p, r.payload)
		r.payload = r.payload[n:]
		if len(r.payload) == 0 {
			r.done = true
		}
		return n, nil
	}
	return 0, r.err
}

// TestServe_EOFReturnsNil asserts a clean stdin close (io.EOF) is an orderly shutdown.
func TestServe_EOFReturnsNil(t *testing.T) {
	srv := acpserver.New(discardLogger())

	stdin := strings.NewReader(`{"jsonrpc":"2.0","method":"session/update"}` + "\n")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := srv.Serve(ctx, stdin, stdout)
	assert.NoError(t, err, "clean EOF must be a nil-error shutdown")
}

// TestServe_NonEOFReadErrorIsSurfaced asserts a real I/O fault on stdin is returned as an
// error (not swallowed as a clean shutdown), wrapping the underlying read error.
func TestServe_NonEOFReadErrorIsSurfaced(t *testing.T) {
	srv := acpserver.New(discardLogger())

	ioErr := errors.New("simulated broken pipe")
	stdin := &errReader{payload: []byte(`{"jsonrpc":"2.0","method":"session/update"}` + "\n"), err: ioErr}
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := srv.Serve(ctx, stdin, stdout)
	require.Error(t, err, "a non-EOF stdin read error must be surfaced")
	assert.ErrorIs(t, err, ioErr, "the underlying read error must be wrapped via %%w")
	assert.Contains(t, err.Error(), "stdin read error")
}

// oversizeThenErrReader simulates an oversized line (exceeds maxRequestLineBytes)
// followed by an I/O fault during drain. It is used to verify M4: a drain-time
// I/O error must NOT produce a spurious ErrInvalidRequest response before
// surfacing the transport error — only the transport error should be returned.
type oversizeThenErrReader struct {
	sent    int
	ioErr   error
	errAt   int // byte index at which to inject the I/O error
	payload []byte
}

func newOversizeThenErrReader(oversizeBytes, errAt int, ioErr error) *oversizeThenErrReader {
	// Build a payload that exceeds the limit without a newline, so readLine
	// keeps reading and accumulates > limit bytes, triggering drain mode.
	payload := make([]byte, oversizeBytes)
	for i := range payload {
		payload[i] = 'x'
	}
	return &oversizeThenErrReader{payload: payload, errAt: errAt, ioErr: ioErr}
}

func (r *oversizeThenErrReader) Read(p []byte) (int, error) {
	if r.sent >= r.errAt {
		return 0, r.ioErr
	}
	remaining := r.errAt - r.sent
	toSend := min(len(p), remaining, len(r.payload)-r.sent)
	if toSend <= 0 {
		return 0, r.ioErr
	}
	n := copy(p[:toSend], r.payload[r.sent:r.sent+toSend])
	r.sent += n
	return n, nil
}

// TestServe_OversizeDrainError_NoSpuriousResponse asserts that when an oversize
// line's drain fails with a non-EOF I/O error, Serve surfaces ONLY the I/O
// error and does NOT first emit a spurious ErrInvalidRequest response (M4).
// Before the fix, readLine returned tooLong=true AND err!=nil, causing the
// dispatch loop to both send an ErrInvalidRequest frame and then terminate —
// two events instead of one.
func TestServe_OversizeDrainError_NoSpuriousResponse(t *testing.T) {
	srv := acpserver.New(discardLogger())

	ioErr := errors.New("simulated drain I/O fault")

	// Send enough bytes to exceed the 10 MiB limit without a newline, then
	// inject an I/O error at a point past the limit (during the drain phase).
	// errAt is set to 11 MiB so the first chunk exceeds the 10 MiB limit and
	// triggers drain mode; the error arrives while draining.
	const limit = 10 * 1024 * 1024
	stdin := newOversizeThenErrReader(12*1024*1024, limit+512*1024, ioErr)
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := srv.Serve(ctx, stdin, stdout)

	// M4: Serve must return a transport error (not nil / not ErrInvalidRequest
	// wrapping), because the underlying cause is a drain I/O fault.
	require.Error(t, err, "drain I/O fault must be surfaced as a Serve error (M4)")
	assert.ErrorIs(t, err, ioErr, "drain I/O fault must wrap the original error")

	// M4: no ErrInvalidRequest response must have been written — stdout must
	// be empty because the drain failed before the oversize signal could be
	// processed cleanly.
	assert.Empty(t, stdout.String(), "no ErrInvalidRequest response must be emitted when drain fails (M4)")
}

// TestServe_SingleUse asserts a Server binds to exactly one stdio session: a second Serve
// returns an error instead of silently reusing the stale encoder / re-closing ready.
func TestServe_SingleUse(t *testing.T) {
	srv := acpserver.New(discardLogger())

	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	require.NoError(t, srv.Serve(ctx, stdin, stdout), "first Serve should complete cleanly")

	err := srv.Serve(ctx, strings.NewReader(""), &bytes.Buffer{})
	require.Error(t, err, "a second Serve call must be rejected")
	assert.Contains(t, err.Error(), "single-use")
}

// TestCallClient_WriteFrameFailure_NoPendingCallsLeak asserts that when writeFrame
// fails (e.g. the output pipe is broken), CallClient returns an error AND does NOT
// insert the response channel into pendingCalls. Without the ghost-ID fix the
// channel was stored before writeFrame, so a subsequent stray response arriving
// with the same ID would be routed to an orphaned, never-read channel. After the
// fix, writeFrame is called first; on failure we return early before Store, leaving
// pendingCalls unmodified.
//
// The test verifies this indirectly: we close the output pipe read-end to make all
// Encode calls fail, call CallClient (which must return an error), then restore a
// working output writer and issue a second call that succeeds. If the first call had
// leaked an entry, the second call's Store would shadow it but the leaked channel
// would remain in the map forever — the test validates the second call succeeds
// cleanly, which would not happen if the dispatch loop was confused by a phantom
// pending entry from the first call.
func TestCallClient_WriteFrameFailure_NoPendingCallsLeak(t *testing.T) {
	// Use a net.Pipe pair so we can close the client side to break the output
	// writer, then reconnect with a fresh pipe for the second call.
	//
	// Architecture: Serve writes to outConn; we read from outClient.
	// Closing outClient makes outConn.Write return io.ErrClosedPipe.
	inConn, inClient := net.Pipe()
	outConn, outClient := net.Pipe()

	srv := acpserver.New(discardLogger())

	serveComplete := make(chan error, 1)
	go func() {
		serveComplete <- srv.Serve(t.Context(), inConn, outConn)
	}()

	// net.Pipe is synchronous: Notify writes to outConn only when a goroutine is
	// concurrently reading from outClient. Start a draining goroutine before the
	// readiness probe so Notify (and any other server-originated frames written
	// before we close outClient) does not block indefinitely.
	drainerDone := make(chan struct{})
	go func() {
		defer close(drainerDone)
		io.Copy(io.Discard, outClient) //nolint:errcheck // drainer: discard until close
	}()

	// Wait for Serve to be ready.
	ctxReady, cancelReady := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancelReady()
	require.NoError(t, srv.Notify(ctxReady, "probe/ready", nil), "server must be ready")

	// Break the output pipe by closing the read end. The next Encode call inside
	// writeFrame will fail with io.ErrClosedPipe (or "write: broken pipe").
	outClient.Close()
	<-drainerDone // wait for the drainer goroutine to exit so goleak is clean

	// CallClient must return an error — the write to the broken pipe fails and
	// the ghost-ID fix means the channel is never stored in pendingCalls.
	callCtx, callCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer callCancel()

	result, err := srv.CallClient(callCtx, "session/request_permission", nil)
	require.Error(t, err, "CallClient must return an error when writeFrame fails")
	require.Nil(t, result, "CallClient must return nil result on write failure")
	require.Contains(t, err.Error(), "write request", "error must identify the write step")

	// Drain the inClient side to unblock any pending reads, then close both ends
	// to let Serve exit cleanly.
	inClient.Close()
	inConn.Close()

	select {
	case <-serveComplete:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Serve did not return after pipe close")
	}
}

func TestCallClient_ConcurrentWrites(t *testing.T) {
	srv := acpserver.New(discardLogger())

	in, inWriter := io.Pipe()
	outReader, out := io.Pipe()

	serveComplete := make(chan error, 1)
	go func() {
		serveComplete <- srv.Serve(context.Background(), in, out)
	}()
	clientDone := runClient(t, outReader, inWriter, map[string]int{"value": 1})

	var wg sync.WaitGroup
	const numCalls = 20

	for i := range numCalls {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, _ = srv.CallClient(ctx, "test", map[string]int{"id": i})
		})
	}
	wg.Wait()

	inWriter.Close()
	<-serveComplete
	out.Close()
	<-clientDone
}
