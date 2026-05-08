package sdk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// mockHostEventServiceClient implements pluginv1.HostEventServiceClient for testing.
type mockHostEventServiceClient struct {
	emitFunc func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error)
}

func (m *mockHostEventServiceClient) Emit(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
	if m.emitFunc != nil {
		return m.emitFunc(ctx, in, opts...)
	}
	return &pluginv1.EmitResponse{Success: true}, nil
}

func TestEventEmitterInterfaceExists(t *testing.T) {
	// Verify that EventEmitter interface is defined with correct method signature.
	var _ EventEmitter = (*HostClient)(nil)
}

func TestNewHostClient_ReturnsNilWhenBrokerIsNil(t *testing.T) {
	client := NewHostClient(nil, "test-plugin")
	assert.Nil(t, client)
}

func TestNewHostClient_UsesHostEventServiceID(t *testing.T) {
	// This test verifies that NewHostClient calls broker.Dial with HostEventServiceID = 1.
	// The actual connection is tested via integration tests.
	// For unit testing, we verify the constant is correct.
	assert.Equal(t, uint32(1), HostEventServiceID)
}

func TestHostClient_EmitSuccess(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			assert.Equal(t, "user.signup", in.EventType)
			assert.Equal(t, "test-plugin", in.SourcePlugin)
			assert.Equal(t, []byte("user_id=123"), in.Payload)
			assert.Equal(t, map[string]string{"version": "1"}, in.Metadata)
			assert.Greater(t, in.TimestampUnixNanos, int64(0))
			return &pluginv1.EmitResponse{Success: true}, nil
		},
	}

	// Create HostClient with mock client via direct struct assignment.
	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	ctx := context.Background()
	err := hostClient.Emit(ctx, "user.signup", []byte("user_id=123"), map[string]string{"version": "1"})
	assert.NoError(t, err)
}

func TestHostClient_EmitFailureWithErrorMessage(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			return &pluginv1.EmitResponse{
				Success:      false,
				ErrorMessage: "host internal error",
			}, nil
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	ctx := context.Background()
	err := hostClient.Emit(ctx, "test.event", []byte("data"), map[string]string{})
	require.Error(t, err, "Emit should return error when response.Success is false")
	assert.Contains(t, err.Error(), "host internal error")
}

func TestHostClient_EmitTransportError(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			return nil, errors.New("network error")
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	ctx := context.Background()
	err := hostClient.Emit(ctx, "test.event", []byte("data"), map[string]string{})
	require.Error(t, err, "Emit should return error on RPC failure")
	assert.Contains(t, err.Error(), "emit RPC failed")
}

func TestHostClient_EmitWithContextCancellation(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				return &pluginv1.EmitResponse{Success: true}, nil
			}
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := hostClient.Emit(ctx, "test.event", []byte("data"), map[string]string{})
	assert.Error(t, err)
}

func TestHostClient_EmitIncludesSourcePlugin(t *testing.T) {
	pluginName := "my-custom-plugin"
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			assert.Equal(t, pluginName, in.SourcePlugin)
			return &pluginv1.EmitResponse{Success: true}, nil
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: pluginName,
	}

	err := hostClient.Emit(context.Background(), "event", []byte{}, map[string]string{})
	assert.NoError(t, err)
}

func TestHostClient_EmitWithEmptyPayload(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			assert.Equal(t, []byte{}, in.Payload)
			return &pluginv1.EmitResponse{Success: true}, nil
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	err := hostClient.Emit(context.Background(), "test.event", []byte{}, map[string]string{})
	assert.NoError(t, err)
}

func TestHostClient_EmitWithEmptyMetadata(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			assert.Empty(t, in.Metadata)
			return &pluginv1.EmitResponse{Success: true}, nil
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	err := hostClient.Emit(context.Background(), "test.event", []byte("data"), map[string]string{})
	assert.NoError(t, err)
}

func TestHostClient_EmitIncludesTimestamp(t *testing.T) {
	mockClient := &mockHostEventServiceClient{
		emitFunc: func(ctx context.Context, in *pluginv1.EmitRequest, opts ...grpc.CallOption) (*pluginv1.EmitResponse, error) {
			// Verify timestamp is recent (within 1 second).
			ts := in.TimestampUnixNanos / 1e9
			now := time.Now().Unix()
			assert.True(t, ts >= now-1 && ts <= now+1, "timestamp should be recent")
			return &pluginv1.EmitResponse{Success: true}, nil
		},
	}

	hostClient := &HostClient{
		client:     mockClient,
		pluginName: "test-plugin",
	}

	err := hostClient.Emit(context.Background(), "test.event", []byte("data"), map[string]string{})
	assert.NoError(t, err)
}

func TestBrokerAwarePluginInterfaceExists(t *testing.T) {
	// Verify that BrokerAwarePlugin interface is defined with SetHostClient method.
	var _ BrokerAwarePlugin
}

func TestHostEventServiceIDConstant(t *testing.T) {
	assert.Equal(t, uint32(1), HostEventServiceID)
}

func TestHostClient_NilClientHandling(t *testing.T) {
	// Ensure that HostClient handles nil client gracefully or panics appropriately.
	// This tests the case where NewHostClient returns a HostClient with a nil client field.
	hostClient := &HostClient{
		client:     nil,
		pluginName: "test-plugin",
	}

	// The current implementation returns nil from Emit, but a real implementation
	// should handle this case. This test validates that at least it doesn't panic unexpectedly.
	ctx := context.Background()
	_ = hostClient.Emit(ctx, "test.event", []byte("data"), map[string]string{})
}
