package application

import (
	"context"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

// userInputReaderCtxKey scopes a per-run UserInputReader onto a context, mirroring
// recorderCtxKey. The facade Adapter binds a session-aware reader per Run so that
// interactive conversation turns route through the RunSession parking mechanism
// (EventInputRequired -> RunSession.Respond) instead of a shared, process-global
// stdin reader. This keeps conversation input wiring consistent across every
// interface (CLI, HTTP, TUI, ACP) that consumes the facade.
type userInputReaderCtxKey struct{}

// withUserInputReader returns a context that routes conversation user input to r.
// A nil reader is returned unchanged so callers can wrap unconditionally.
func withUserInputReader(ctx context.Context, r ports.UserInputReader) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, userInputReaderCtxKey{}, r)
}

// userInputReaderFrom resolves a context-scoped UserInputReader, if one was bound by
// the facade for this run. Returns nil when none is present (legacy/static wiring).
func userInputReaderFrom(ctx context.Context) ports.UserInputReader {
	if r, ok := ctx.Value(userInputReaderCtxKey{}).(ports.UserInputReader); ok && r != nil {
		return r
	}
	return nil
}

// sessionInputReader is the session-bound ports.UserInputReader used by the facade
// conversation path. ReadInput emits an EventInputRequired event into the RunSession
// (so interface drivers can prompt the user and surface the parking state) and then
// blocks on the session's respond channel until RunSession.Respond delivers a value
// or the session/run context is cancelled.
//
// This mirrors the ACP facadeInputBridge: input requests surface as events and the
// continuation arrives via RunSession.Respond, rather than a blocking read on a
// process-global stdin handle. EOF/cancellation on the driver side maps to an empty
// response, which the ConversationManager treats as a graceful user exit.
type sessionInputReader struct {
	session *RunSession
}

// newSessionInputReader binds a UserInputReader to a RunSession.
func newSessionInputReader(session *RunSession) *sessionInputReader {
	return &sessionInputReader{session: session}
}

var _ ports.UserInputReader = (*sessionInputReader)(nil)

// ReadInput parks the workflow on user input: it appends an EventInputRequired event
// carrying the prompt, then waits for a response delivered via RunSession.Respond.
// A cancelled session or run context unblocks the read and is reported to the caller
// as the context error so the conversation loop can stop cleanly.
func (r *sessionInputReader) ReadInput(ctx context.Context) (string, error) {
	r.session.appendEvent(ports.Event{
		Kind:      ports.EventInputRequired,
		RunID:     r.session.id,
		Timestamp: time.Now(),
		Payload:   &ports.EnrichedInputRequest{Prompt: "> "},
	})

	resp, err := awaitSessionResponse(ctx, r.session, r.session.respondCh)
	if err != nil {
		return "", err
	}
	return resp.Value, nil
}

// awaitSessionResponse blocks until a value arrives on respCh, the caller ctx is
// cancelled, or the session context is cancelled. It is the shared waiting primitive
// used by both sessionInputReader and InputBridge so the tri-select logic lives in
// exactly one place.
//
// Drain behavior on cancellation is NOT applied here: whether to drain respCh after
// a cancelled wait is caller-specific. sessionInputReader reads session.respondCh
// directly — a pending value there should remain available for the next ReadInput
// call (the channel is session-scoped, not bridge-scoped). InputBridge owns its own
// private responseCh and must drain stale values between sequential ReadInput calls;
// that drain is handled by InputBridge.ReadInput around the awaitSessionResponse call.
func awaitSessionResponse(ctx context.Context, sess *RunSession, respCh chan ports.InputResponse) (ports.InputResponse, error) {
	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return ports.InputResponse{}, ctx.Err()
	case <-sess.ctx.Done():
		return ports.InputResponse{}, sess.ctx.Err()
	}
}
