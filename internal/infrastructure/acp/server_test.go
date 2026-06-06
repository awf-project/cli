package acp_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/application"
	acpinfra "github.com/awf-project/cli/internal/infrastructure/acp"
)

// newTestConn builds a Conn over an in-memory stdio pair. The returned writer end of the
// peer-output pipe lets a test drive EOF (peer disconnect), and discard absorbs protocol
// frames written toward the editor. Callers must close peerW to release the SDK receive
// goroutine.
func newTestConn(t *testing.T, logger *slog.Logger) (conn *acpinfra.Conn, peerW *io.PipeWriter) {
	t.Helper()
	agent := acpinfra.NewAgent(&application.ACPSessionService{})
	peerR, peerW := io.Pipe()
	conn = acpinfra.NewConnection(agent, io.Discard, peerR, logger)
	require.NotNil(t, conn)
	return conn, peerW
}

func TestNewConnection_NilLoggerDoesNotPanic(t *testing.T) {
	conn, peerW := newTestConn(t, nil)
	t.Cleanup(func() { _ = peerW.Close() })
	assert.NotNil(t, conn.Done(), "Done channel must be available")
}

func TestNewConnection_WithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	conn, peerW := newTestConn(t, logger)
	t.Cleanup(func() { _ = peerW.Close() })
	assert.NotNil(t, conn.Done())
}

// TestConn_DoneClosesOnPeerEOF verifies the serve-loop contract: closing the peer-output
// stream (editor disconnect / stdin EOF) terminates the connection so the <-conn.Done()
// wait in runACPServe unblocks.
func TestConn_DoneClosesOnPeerEOF(t *testing.T) {
	conn, peerW := newTestConn(t, nil)

	// Simulate peer disconnect: closing the write end yields EOF on the connection's reader.
	require.NoError(t, peerW.Close())

	select {
	case <-conn.Done():
		// connection terminated as expected
	case <-time.After(2 * time.Second):
		t.Fatal("conn.Done() did not close after peer EOF")
	}
}

// TestConn_NewEmitter verifies the connection hands back a usable emitter without the
// caller naming any SDK type. An empty kind is the emitter's documented no-op path, so it
// must return nil even though no real transport write occurs.
func TestConn_NewEmitter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	conn, peerW := newTestConn(t, logger)
	t.Cleanup(func() { _ = peerW.Close() })

	emitter := conn.NewEmitter(logger)
	require.NotNil(t, emitter)

	// Empty kind is dropped (no-op) per Emitter contract; exercises the wiring is live.
	assert.NoError(t, emitter.EmitSessionUpdate(t.Context(), "sess_x", "", nil))
}
