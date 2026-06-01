package cli

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/acp"
	"github.com/awf-project/cli/pkg/acpserver"
)

// acpHandler is the transport-neutral request handler shape implemented by
// ACPSessionService. adaptACPHandler lifts it to an acpserver.HandlerFunc, mapping the
// application-layer error kind onto its JSON-RPC code at the interface boundary so the
// application layer never imports pkg/acpserver (M1: transport stays an interface concern).
type acpHandler func(context.Context, json.RawMessage) (any, *application.ACPHandlerError)

func adaptACPHandler(h acpHandler) acpserver.HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (any, *acpserver.Error) {
		result, herr := h(ctx, params)
		if herr == nil {
			return result, nil
		}
		// JSON-RPC 2.0: an error response carries a null result.
		// C-3: propagate the optional Data field so machine-readable codes (e.g.
		// USER.ACP.PROMPT_IN_FLIGHT) appear in the JSON-RPC error's "data" field rather
		// than in "message", which is displayed verbatim in the editor UI.
		rpcErr := &acpserver.Error{Code: acpErrorCode(herr.Kind), Message: herr.Message}
		if herr.Data != nil {
			rpcErr.Data = herr.Data
		}
		return nil, rpcErr
	}
}

// acpErrorCode maps an application ACPErrorKind onto its JSON-RPC 2.0 error code.
func acpErrorCode(kind application.ACPErrorKind) int {
	switch kind {
	case application.ACPErrInvalidParams:
		return acpserver.ErrInvalidParams
	case application.ACPErrMethodNotFound:
		return acpserver.ErrMethodNotFound
	case application.ACPErrInternal:
		return acpserver.ErrInternal
	default:
		return acpserver.ErrInternal
	}
}

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
	ctx         context.Context //nolint:containedctx // io.Writer.Write has no ctx param; signalCtx (server shutdown context) is captured at construction so a SIGTERM cancels emission instead of writing to a dead stdout. Limitation v1: the writer does not propagate per-request cancellation; this is acceptable because the ACP server is single-session-per-process in v1.
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

// acpMessageSender adapts acp.Sender (used by ACPRenderer) to the session emitter,
// mapping each Message type to its ACP sessionUpdate discriminator and fields. When
// streamed is non-nil and an emit succeeds, it is set to true so HandleSessionPrompt
// can suppress the post-run aggregate safely.
type acpMessageSender struct {
	emitter   application.SessionUpdateEmitter
	sessionID string
	streamed  *atomic.Bool
}

func newACPMessageSender(emitter application.SessionUpdateEmitter, sessionID string, streamed *atomic.Bool) *acpMessageSender {
	return &acpMessageSender{emitter: emitter, sessionID: sessionID, streamed: streamed}
}

func (s *acpMessageSender) Send(ctx context.Context, msg acp.Message) error { //nolint:gocritic // hugeParam: signature is fixed by acp.Sender interface
	var err error
	switch msg.Type {
	case acp.MsgAgentMessageChunk:
		err = s.emitter.EmitSessionUpdate(ctx, s.sessionID, "agent_message_chunk", map[string]any{
			"seq":     msg.Seq,
			"content": map[string]any{"type": "text", "text": msg.Content},
		})
	case acp.MsgAgentThoughtChunk:
		err = s.emitter.EmitSessionUpdate(ctx, s.sessionID, "agent_thought_chunk", map[string]any{
			"seq":     msg.Seq,
			"content": map[string]any{"type": "text", "text": msg.Content},
		})
	case acp.MsgToolCall, acp.MsgToolCallUpdate:
		err = s.emitter.EmitSessionUpdate(ctx, s.sessionID, string(msg.Type), map[string]any{
			"seq":        msg.Seq,
			"toolCallId": msg.ToolID,
			"title":      msg.Tool,
			"rawInput":   map[string]any{"text": msg.Content},
		})
	default:
		return nil
	}
	if err == nil && s.streamed != nil {
		s.streamed.Store(true)
	}
	return err
}

// acpSessionNotifier adapts acp.SessionNotifier (used by WorkflowEventProjector) to the
// session emitter. The projector keys updates by workflowID; routing is by the bound
// sessionID (one projector per session, built in the factory).
type acpSessionNotifier struct {
	emitter   application.SessionUpdateEmitter
	sessionID string
}

func newACPSessionNotifier(emitter application.SessionUpdateEmitter, sessionID string) *acpSessionNotifier {
	return &acpSessionNotifier{emitter: emitter, sessionID: sessionID}
}

func (n *acpSessionNotifier) NotifySessionUpdate(ctx context.Context, _ string, update acp.SessionUpdate) error {
	fields := map[string]any{}
	if update.StepName != "" {
		fields["stepName"] = update.StepName
	}
	if update.Error != "" {
		fields["error"] = update.Error
	}
	if update.Duration != "" {
		fields["duration"] = update.Duration
	}
	return n.emitter.EmitSessionUpdate(ctx, n.sessionID, update.Kind, fields)
}

// compile-time assertions
var (
	_ io.Writer           = (*acpTextWriter)(nil)
	_ acp.Sender          = (*acpMessageSender)(nil)
	_ acp.SessionNotifier = (*acpSessionNotifier)(nil)
)

// sharedHistoryStore wraps a HistoryStore so the per-session ExecutionSetup.Build cleanup
// (which closes any io.Closer history store) does NOT close the server-shared store. The
// real store's lifecycle is owned by runACPServe and closed once at shutdown.
type sharedHistoryStore struct {
	ports.HistoryStore
}

func (sharedHistoryStore) Close() error { return nil }
