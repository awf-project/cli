package acp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	sdk "github.com/coder/acp-go-sdk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSessionUpdater is an in-package fake for the unexported sessionUpdater seam.
// It records every forwarded notification so tests can exercise the real
// marshal → unmarshal → conn.SessionUpdate path without a live SDK transport.
type fakeSessionUpdater struct {
	calls []sdk.SessionNotification
	err   error
}

func (f *fakeSessionUpdater) SessionUpdate(_ context.Context, n sdk.SessionNotification) error { //nolint:gocritic // hugeParam: signature fixed by the sessionUpdater/SDK interface
	f.calls = append(f.calls, n)
	return f.err
}

func newTestEmitter(conn sessionUpdater) (*Emitter, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &Emitter{conn: conn, logger: slog.New(slog.NewTextHandler(buf, nil))}, buf
}

// TestEmitter_ForwardsValidKindToConn covers the conn-non-nil happy path that the
// external (nil-conn) tests cannot reach: a recognized kind is marshaled, unmarshaled
// into the matching SDK variant, and forwarded to the connection with the right session.
func TestEmitter_ForwardsValidKindToConn(t *testing.T) {
	fake := &fakeSessionUpdater{}
	emitter, buf := newTestEmitter(fake)

	err := emitter.EmitSessionUpdate(context.Background(), "sess-1", "agent_message_chunk", map[string]any{
		"content": map[string]any{"type": "text", "text": "hello"},
	})

	require.NoError(t, err)
	require.Len(t, fake.calls, 1, "valid kind must be forwarded to the connection exactly once")
	assert.Equal(t, sdk.SessionId("sess-1"), fake.calls[0].SessionId)
	assert.NotNil(t, fake.calls[0].Update.AgentMessageChunk, "agent_message_chunk must decode to the AgentMessageChunk variant")
	assert.NotContains(t, buf.String(), "skipping unrecognized update kind")
}

// TestEmitter_WrapsConnError covers the error branch: a transport failure from the
// connection is wrapped (not swallowed) and returned to the caller.
func TestEmitter_WrapsConnError(t *testing.T) {
	fake := &fakeSessionUpdater{err: errors.New("transport down")}
	emitter, _ := newTestEmitter(fake)

	err := emitter.EmitSessionUpdate(context.Background(), "sess-1", "agent_message_chunk", map[string]any{
		"content": map[string]any{"type": "text", "text": "hello"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "session update")
	assert.ErrorContains(t, err, "transport down")
}

// TestEmitter_EmptyKindNotForwarded confirms the empty-kind guard short-circuits before
// any connection call even when a live connection is present.
func TestEmitter_EmptyKindNotForwarded(t *testing.T) {
	fake := &fakeSessionUpdater{}
	emitter, buf := newTestEmitter(fake)

	err := emitter.EmitSessionUpdate(context.Background(), "sess-1", "", map[string]any{})

	require.NoError(t, err)
	assert.Empty(t, fake.calls, "empty kind must never reach the connection")
	assert.Contains(t, buf.String(), "empty update kind dropped")
}
