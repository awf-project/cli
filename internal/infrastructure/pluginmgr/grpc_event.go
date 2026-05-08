package pluginmgr

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

const defaultEventHandlerTimeout = 5 * time.Second

type grpcEventAdapter struct {
	client     pluginv1.EventServiceClient
	pluginName string
}

var _ EventDeliverer = (*grpcEventAdapter)(nil)

func newGRPCEventAdapter(client pluginv1.EventServiceClient, pluginName string) *grpcEventAdapter {
	return &grpcEventAdapter{
		client:     client,
		pluginName: pluginName,
	}
}

func domainEventToProto(e *pluginmodel.DomainEvent) *pluginv1.HandleEventRequest {
	return &pluginv1.HandleEventRequest{
		Id:                 e.ID,
		Type:               e.Type,
		TimestampUnixNanos: e.Timestamp.UnixNano(),
		Source:             e.Source,
		Metadata:           e.Metadata,
		Payload:            e.Payload,
		PropagationDepth:   int32(e.PropagationDepth), //nolint:gosec // G115: propagation depth is bounded by EventBus max depth, never exceeds int32 range
	}
}

func protoToDomainEvent(r *pluginv1.HandleEventRequest) *pluginmodel.DomainEvent {
	return &pluginmodel.DomainEvent{
		ID:               r.GetId(),
		Type:             r.GetType(),
		Timestamp:        time.Unix(0, r.GetTimestampUnixNanos()),
		Source:           r.GetSource(),
		Metadata:         r.GetMetadata(),
		Payload:          r.GetPayload(),
		PropagationDepth: int(r.GetPropagationDepth()),
	}
}

func domainEventToStreamMessage(event *pluginmodel.DomainEvent, seqNum uint64) *pluginv1.EventStreamMessage {
	return &pluginv1.EventStreamMessage{
		Id:                 event.ID,
		Type:               event.Type,
		TimestampUnixNanos: event.Timestamp.UnixNano(),
		Source:             event.Source,
		Metadata:           event.Metadata,
		Payload:            event.Payload,
		PropagationDepth:   int32(event.PropagationDepth), //nolint:gosec // G115: propagation depth is bounded by EventBus max depth, never exceeds int32 range
		SequenceNumber:     seqNum,
	}
}

func (a *grpcEventAdapter) DeliverEvent(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	handlerCtx, cancel := context.WithTimeout(ctx, defaultEventHandlerTimeout)
	defer cancel()

	resp, err := a.client.HandleEvent(handlerCtx, domainEventToProto(event))
	if err != nil {
		return nil, fmt.Errorf("HandleEvent RPC [%s]: %w", a.pluginName, err)
	}

	emitted := make([]*pluginmodel.DomainEvent, 0, len(resp.GetEmittedEvents()))
	for _, e := range resp.GetEmittedEvents() {
		emitted = append(emitted, protoToDomainEvent(e))
	}
	return emitted, nil
}
