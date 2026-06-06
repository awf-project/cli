package acp

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	sdk "github.com/coder/acp-go-sdk"

	"github.com/awf-project/cli/internal/domain/ports"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(newDiscardWriter(), nil))
}

// newDiscardWriter returns an io.Writer that drops all input; avoids buffer growth
// under the concurrency test while keeping the logger non-nil.
func newDiscardWriter() *discardWriter { return &discardWriter{} }

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// mockConnection captures RequestPermission calls and returns a scripted response.
// It is safe for concurrent use so it can back the serialization test.
type mockConnection struct {
	mu               sync.Mutex
	capturedRequests []sdk.RequestPermissionRequest
	respFunc         func(ctx context.Context, req sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error)
}

func (m *mockConnection) RequestPermission(ctx context.Context, req sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error) { //nolint:gocritic // hugeParam: signature fixed by SDK
	m.mu.Lock()
	m.capturedRequests = append(m.capturedRequests, req)
	m.mu.Unlock()
	if m.respFunc != nil {
		return m.respFunc(ctx, req)
	}
	return sdk.RequestPermissionResponse{}, nil
}

func (m *mockConnection) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.capturedRequests)
}

// newClientWithMock builds a PermissionClient backed by the injectable fake. The
// production constructor takes the concrete *sdk.AgentSideConnection; the struct
// field is the permissionRequester interface so tests can substitute a fake.
func newClientWithMock(conn permissionRequester) *PermissionClient {
	return &PermissionClient{conn: conn, logger: testLogger()}
}

func TestPermissionClient_CompileTimeAssertion(t *testing.T) {
	var _ ports.ACPClient = (*PermissionClient)(nil)
}

// TestPermissionClient_RoundTripMapping verifies every PermissionRequest field is
// mapped onto the SDK request and the selected outcome is mapped back.
func TestPermissionClient_RoundTripMapping(t *testing.T) {
	mock := &mockConnection{
		respFunc: func(_ context.Context, _ sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error) {
			return sdk.RequestPermissionResponse{
				Outcome: sdk.NewRequestPermissionOutcomeSelected(sdk.PermissionOptionId("allow_once")),
			}, nil
		},
	}
	client := newClientWithMock(mock)

	resp, err := client.RequestPermission(context.Background(), ports.PermissionRequest{
		SessionID:  "sess-42",
		ToolCallID: "call-99",
		Prompt:     "Allow filesystem access?",
		Options: []ports.PermissionOption{
			{ID: "allow_once", Label: "Allow once", Kind: "allow"},
			{ID: "deny", Label: "Deny", Kind: "deny"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "allow_once", resp.OptionID)

	require.Len(t, mock.capturedRequests, 1)
	got := mock.capturedRequests[0]
	assert.Equal(t, sdk.SessionId("sess-42"), got.SessionId)
	assert.Equal(t, sdk.ToolCallId("call-99"), got.ToolCall.ToolCallId)
	require.NotNil(t, got.ToolCall.Title)
	assert.Equal(t, "Allow filesystem access?", *got.ToolCall.Title)
	require.Len(t, got.Options, 2)
	assert.Equal(t, sdk.PermissionOptionId("allow_once"), got.Options[0].OptionId)
	assert.Equal(t, "Allow once", got.Options[0].Name)
	assert.Equal(t, sdk.PermissionOptionKind("allow"), got.Options[0].Kind)
	assert.Equal(t, sdk.PermissionOptionId("deny"), got.Options[1].OptionId)
}

// TestPermissionClient_CancelledReturnsEmptyOptionID verifies a Cancelled outcome
// maps to an empty OptionID (the documented "cancelled" sentinel).
func TestPermissionClient_CancelledReturnsEmptyOptionID(t *testing.T) {
	mock := &mockConnection{
		respFunc: func(_ context.Context, _ sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error) {
			return sdk.RequestPermissionResponse{Outcome: sdk.NewRequestPermissionOutcomeCancelled()}, nil
		},
	}
	client := newClientWithMock(mock)

	resp, err := client.RequestPermission(context.Background(), ports.PermissionRequest{
		SessionID:  "sess-1",
		ToolCallID: "call-1",
		Options:    []ports.PermissionOption{{ID: "yes", Label: "Yes", Kind: "allow"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "", resp.OptionID)
}

// TestPermissionClient_ContextCancelled verifies a pre-cancelled context returns
// context.Canceled and no SDK call is made.
func TestPermissionClient_ContextCancelled(t *testing.T) {
	mock := &mockConnection{}
	client := newClientWithMock(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.RequestPermission(ctx, ports.PermissionRequest{
		SessionID: "sess", ToolCallID: "call",
		Options: []ports.PermissionOption{{ID: "ok", Label: "OK", Kind: "allow"}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 0, mock.calls(), "no SDK call when ctx already cancelled")
}

// TestPermissionClient_ContextDeadlineExceeded verifies an expired deadline returns
// context.DeadlineExceeded and no SDK call is made.
func TestPermissionClient_ContextDeadlineExceeded(t *testing.T) {
	mock := &mockConnection{}
	client := newClientWithMock(mock)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := client.RequestPermission(ctx, ports.PermissionRequest{
		SessionID: "sess", ToolCallID: "call",
		Options: []ports.PermissionOption{{ID: "ok", Label: "OK", Kind: "allow"}},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 0, mock.calls())
}

// TestPermissionClient_SDKErrorPropagated verifies a transport error is surfaced.
func TestPermissionClient_SDKErrorPropagated(t *testing.T) {
	sentinel := errors.New("transport down")
	mock := &mockConnection{
		respFunc: func(_ context.Context, _ sdk.RequestPermissionRequest) (sdk.RequestPermissionResponse, error) {
			return sdk.RequestPermissionResponse{}, sentinel
		},
	}
	client := newClientWithMock(mock)

	_, err := client.RequestPermission(context.Background(), ports.PermissionRequest{
		SessionID: "sess", ToolCallID: "call",
		Options: []ports.PermissionOption{{ID: "ok", Label: "OK", Kind: "allow"}},
	})

	assert.ErrorIs(t, err, sentinel)
}

// TestPermissionClient_NilConnNoCall verifies the adapter degrades gracefully when
// constructed with a nil connection (no consumer wired yet — see F108).
func TestPermissionClient_NilConnNoCall(t *testing.T) {
	client := NewPermissionClient(nil, testLogger())

	resp, err := client.RequestPermission(context.Background(), ports.PermissionRequest{
		SessionID: "sess", ToolCallID: "call",
		Options: []ports.PermissionOption{{ID: "ok", Label: "OK", Kind: "allow"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "", resp.OptionID)
}

// TestPermissionClient_ConcurrentCallsNoDeadlock launches 50 concurrent
// RequestPermission calls alongside 50 background goroutines. The adapter takes no
// internal lock (SDK owns stdout serialization, SPIKE finding #8); this guards
// against a regression that adds blocking and against data races (run with -race).
func TestPermissionClient_ConcurrentCallsNoDeadlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test in short mode")
	}
	mock := &mockConnection{}
	client := newClientWithMock(mock)

	var eg errgroup.Group
	for i := range 50 {
		eg.Go(func() error {
			_, err := client.RequestPermission(context.Background(), ports.PermissionRequest{
				SessionID:  "sess",
				ToolCallID: "call",
				Prompt:     "p",
				Options:    []ports.PermissionOption{{ID: "allow", Label: "Allow", Kind: "allow"}},
			})
			_ = i
			return err
		})
	}
	for range 50 {
		eg.Go(func() error { return nil })
	}

	require.NoError(t, eg.Wait())
	assert.Equal(t, 50, mock.calls(), "all concurrent RequestPermission calls reach the SDK")
}
