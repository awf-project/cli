package sdk

import (
	"context"
	"errors"
	"fmt"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// HostEventServiceID is the well-known broker service ID shared between host and plugin.
const HostEventServiceID = uint32(1)

// EventEmitter is implemented by types that can emit events to the host.
type EventEmitter interface {
	Emit(ctx context.Context, eventType string, payload []byte, metadata map[string]string) error
}

// BrokerAwarePlugin is an optional interface for plugins that want to emit events to the host.
// Plugins implementing this interface receive a HostClient during GRPCServer setup.
type BrokerAwarePlugin interface {
	SetHostClient(client *HostClient)
}

// HostClient wraps the gRPC connection to the host's HostEventService.
type HostClient struct {
	client     pluginv1.HostEventServiceClient
	pluginName string
}

// NewHostClient dials the host's HostEventService via the broker and returns a HostClient.
// Returns nil when broker is nil (backward compatibility).
func NewHostClient(broker *goplugin.GRPCBroker, pluginName string) *HostClient {
	if broker == nil {
		return nil
	}
	conn, err := broker.Dial(HostEventServiceID)
	if err != nil {
		return nil
	}
	return &HostClient{
		client:     pluginv1.NewHostEventServiceClient(conn),
		pluginName: pluginName,
	}
}

// Emit sends an event to the host.
func (h *HostClient) Emit(ctx context.Context, eventType string, payload []byte, metadata map[string]string) error {
	if h.client == nil {
		return errors.New("host client not initialized")
	}
	resp, err := h.client.Emit(ctx, &pluginv1.EmitRequest{
		EventType:          eventType,
		SourcePlugin:       h.pluginName,
		Payload:            payload,
		Metadata:           metadata,
		TimestampUnixNanos: time.Now().UnixNano(),
	})
	if err != nil {
		return fmt.Errorf("emit RPC failed: %w", err)
	}
	if !resp.Success {
		return errors.New(resp.ErrorMessage)
	}
	return nil
}
