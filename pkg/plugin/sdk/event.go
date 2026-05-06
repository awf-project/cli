package sdk

import (
	"context"
	"time"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

// Event is the SDK-side representation of a domain event, decoupled from internal domain types.
type Event struct {
	ID               string
	Type             string
	Timestamp        time.Time
	Source           string
	Metadata         map[string]string
	Payload          []byte
	PropagationDepth int
}

// EventSubscriber is implemented by plugins that want to receive and react to domain events.
type EventSubscriber interface {
	Patterns() []string
	HandleEvent(ctx context.Context, event Event) ([]Event, error)
}

// eventServiceServer implements pluginv1.EventServiceServer.
// Delegates to the plugin if it implements EventSubscriber; otherwise returns empty response.
type eventServiceServer struct {
	pluginv1.UnimplementedEventServiceServer
	impl Plugin
}

func (s *eventServiceServer) HandleEvent(ctx context.Context, req *pluginv1.HandleEventRequest) (*pluginv1.HandleEventResponse, error) {
	subscriber, ok := s.impl.(EventSubscriber)
	if !ok {
		return &pluginv1.HandleEventResponse{}, nil
	}

	event := Event{
		ID:               req.GetId(),
		Type:             req.GetType(),
		Timestamp:        time.Unix(0, req.GetTimestampUnixNanos()),
		Source:           req.GetSource(),
		Metadata:         req.GetMetadata(),
		Payload:          req.GetPayload(),
		PropagationDepth: int(req.GetPropagationDepth()),
	}

	emitted, err := subscriber.HandleEvent(ctx, event)
	if err != nil {
		return &pluginv1.HandleEventResponse{}, nil
	}

	protoEmitted := make([]*pluginv1.HandleEventRequest, len(emitted))
	for i, e := range emitted {
		protoEmitted[i] = &pluginv1.HandleEventRequest{
			Id:                 e.ID,
			Type:               e.Type,
			TimestampUnixNanos: e.Timestamp.UnixNano(),
			Source:             e.Source,
			Metadata:           e.Metadata,
			Payload:            e.Payload,
			PropagationDepth:   int32(e.PropagationDepth), //nolint:gosec // G115: propagation depth is bounded by domain logic
		}
	}

	return &pluginv1.HandleEventResponse{EmittedEvents: protoEmitted}, nil
}
