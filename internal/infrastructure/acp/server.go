package acp

import (
	"io"
	"log/slog"

	sdk "github.com/coder/acp-go-sdk"
)

// Conn wraps the SDK *AgentSideConnection so the interfaces/cli layer can own the ACP
// transport lifecycle (stdin forwarding, signal-driven shutdown, the Done() wait loop)
// WITHOUT importing github.com/coder/acp-go-sdk directly. Keeping the SDK connection type
// behind this wrapper is what confines the SDK to internal/infrastructure/acp — the SDK
// Substitution contract in doc.go, enforced by architecture_test.go and .go-arch-lint.yml.
//
// It mirrors F104: mcp_serve.go delegates transport construction to
// internal/infrastructure/mcp rather than importing the MCP SDK in the interface layer
// (commit 9740292). Before this wrapper, acp_serve.go imported the SDK directly to call
// sdk.NewAgentSideConnection and to hold a *sdk.AgentSideConnection field, which widened
// the substitution surface into the interface layer and failed go-arch-lint.
type Conn struct {
	conn *sdk.AgentSideConnection
}

// NewConnection builds the agent-side ACP connection for agent over the (out, in) stdio
// pair and routes SDK diagnostics to logger (os.Stderr in production; a nil logger leaves
// the SDK default). out is the peer-input sink (protocol frames written TO the editor); in
// is the peer-output source (frames read FROM the editor). The connection owns the
// transport: it spawns the receive goroutine that reads framed JSON-RPC from in and
// dispatches to agent. The caller drives shutdown by closing in and waiting on Done().
//
// NFR-002: SetLogger directs all SDK diagnostics to logger so stdout stays reserved for
// protocol frames; the logger must therefore write to stderr (or a non-stdout sink).
func NewConnection(agent *Agent, out io.Writer, in io.Reader, logger *slog.Logger) *Conn {
	conn := sdk.NewAgentSideConnection(agent, out, in)
	if logger != nil {
		conn.SetLogger(logger)
	}
	return &Conn{conn: conn}
}

// closedDone is returned by Done on a nil/transport-less Conn: there is no live transport
// to wait on, so the wait must not block forever. Closed once at init and shared read-only.
var closedDone = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

// Done returns a channel closed when the connection terminates (peer disconnect, stdin EOF,
// or transport error). The interfaces/cli serve loop blocks on it until shutdown. A nil
// Conn (no transport wired, e.g. unit tests) reports already-done so the loop never hangs.
func (c *Conn) Done() <-chan struct{} {
	if c == nil || c.conn == nil {
		return closedDone
	}
	return c.conn.Done()
}

// NewEmitter builds a session-update emitter bound to this connection. Exposed on Conn so
// the interfaces layer obtains an application.SessionUpdateEmitter without naming the SDK
// connection type. Both the service-level emitter and each per-session emitter are built
// this way; logger routes the emitter's own diagnostics to stderr (NFR-002).
//
// A nil Conn (no transport wired) yields an emitter over a nil connection, which NewEmitter
// normalises to a no-op — preserving the graceful-degradation contract the per-session
// factory relies on when constructed without a live connection.
func (c *Conn) NewEmitter(logger *slog.Logger) *Emitter {
	if c == nil {
		return NewEmitter(nil, logger)
	}
	return NewEmitter(c.conn, logger)
}
