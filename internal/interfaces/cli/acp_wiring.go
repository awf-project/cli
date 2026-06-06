package cli

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
)

// acpTextWriter routes raw bytes written by the execution stack (shell step stdout, and
// any non-rendered agent output) to the editor as ACP agent_message_chunk session/update
// notifications, scoped to one session. When streamed is non-nil and an emit succeeds,
// it is set to true so HandleSessionPrompt can suppress the post-run aggregate safely.
//
// Context storage: ctx is the server shutdown signal context captured at construction.
// io.Writer.Write has no ctx parameter, so per-request context propagation is not
// possible through the io.Writer interface. v1 limitation: only SIGTERM cancellation is
// supported. The //nolint directive below is intentional and safe.
//
// Missed emits: Write is best-effort. If EmitSessionUpdate fails, the error is silently
// discarded and the byte count is still returned as len(p) so the io.Writer contract is
// upheld and the execution stack is not interrupted. Use MissedEmits() to observe the
// cumulative count of failed emissions for monitoring or debugging.
type acpTextWriter struct {
	ctx         context.Context //nolint:containedctx // io.Writer.Write has no ctx param; see struct doc for rationale
	emitter     application.SessionUpdateEmitter
	sessionID   string
	streamed    *atomic.Bool
	missedEmits atomic.Uint64
}

func newACPTextWriter(ctx context.Context, emitter application.SessionUpdateEmitter, sessionID string, streamed *atomic.Bool) *acpTextWriter {
	return &acpTextWriter{ctx: ctx, emitter: emitter, sessionID: sessionID, streamed: streamed}
}

// Write implements io.Writer. Emission failures are silently discarded (best-effort)
// so the execution stack's writer chain is never interrupted by a transient ACP
// transport error. The caller receives len(p), nil regardless of whether the
// notification reached the editor. Use MissedEmits() to detect cumulative failures.
func (w *acpTextWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	// Best-effort: a notification send failure must not abort the workflow's writer chain.
	if err := w.emitter.EmitSessionUpdate(w.ctx, w.sessionID, "agent_message_chunk", map[string]any{
		"content": map[string]any{"type": "text", "text": string(p)},
	}); err != nil {
		w.missedEmits.Add(1)
	} else if w.streamed != nil {
		w.streamed.Store(true)
	}
	return len(p), nil
}

// MissedEmits returns the cumulative number of Write calls where EmitSessionUpdate
// returned an error. Reads are atomic and safe for concurrent use.
// A non-zero value indicates ACP transport degradation; the workflow execution itself
// was not interrupted (best-effort contract).
func (w *acpTextWriter) MissedEmits() uint64 {
	return w.missedEmits.Load()
}

// streamFlaggingEmitter wraps a session-scoped SessionUpdateEmitter and flips
// streamed to true on each successful emit, so HandleSessionPrompt can suppress the
// post-run aggregate safely. It lets the per-step Renderer emit ACP SessionUpdate
// variants directly (no bespoke Sender/Message DTO) while preserving the streamed
// signal the legacy acpMessageSender used to provide.
type streamFlaggingEmitter struct {
	emitter  application.SessionUpdateEmitter
	streamed *atomic.Bool
}

func newStreamFlaggingEmitter(emitter application.SessionUpdateEmitter, streamed *atomic.Bool) *streamFlaggingEmitter {
	return &streamFlaggingEmitter{emitter: emitter, streamed: streamed}
}

func (e *streamFlaggingEmitter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	err := e.emitter.EmitSessionUpdate(ctx, sessionID, kind, fields)
	if err == nil && e.streamed != nil {
		e.streamed.Store(true)
	}
	return err
}

// compile-time assertions
var (
	_ io.Writer                        = (*acpTextWriter)(nil)
	_ application.SessionUpdateEmitter = (*streamFlaggingEmitter)(nil)
)

// sharedHistoryStore wraps a HistoryStore so the per-session ExecutionSetup.Build cleanup
// (which closes any io.Closer history store) does NOT close the server-shared store. The
// real store's lifecycle is owned by runACPServe and closed once at shutdown.
type sharedHistoryStore struct {
	ports.HistoryStore
}

func (sharedHistoryStore) Close() error { return nil }
