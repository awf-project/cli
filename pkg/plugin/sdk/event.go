package sdk

import (
	"context"
	"errors"
	"io"
	"time"

	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (s *eventServiceServer) StreamEvents(stream pluginv1.EventService_StreamEventsServer) error { //nolint:wrapcheck // gRPC stream errors must not be wrapped; wrapping loses status codes
	subscriber, ok := s.impl.(EventSubscriber)
	if !ok {
		return status.Error(codes.Unimplemented, "plugin does not implement EventSubscriber") //nolint:wrapcheck // gRPC status errors must remain unwrapped
	}

	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return stream.SendAndClose(&pluginv1.StreamEventsResponse{}) //nolint:wrapcheck // gRPC stream errors carry status codes; wrapping loses them
		}
		if err != nil {
			return err //nolint:wrapcheck // gRPC stream errors carry status codes; wrapping loses them
		}

		event := Event{
			ID:               msg.GetId(),
			Type:             msg.GetType(),
			Timestamp:        time.Unix(0, msg.GetTimestampUnixNanos()),
			Source:           msg.GetSource(),
			Metadata:         msg.GetMetadata(),
			Payload:          msg.GetPayload(),
			PropagationDepth: int(msg.GetPropagationDepth()),
		}
		//nolint:errcheck,gosec // G104: emitted events are fire-and-forget; host drives stream lifecycle
		_, _ = subscriber.HandleEvent(stream.Context(), event)
	}
}
