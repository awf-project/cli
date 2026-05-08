package pluginmgr

import (
	"context"
	"fmt"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

type manifestLookupFn func(string) (*pluginmodel.PluginInfo, bool)

type hostEventService struct {
	pluginv1.UnimplementedHostEventServiceServer
	publisher ports.EventPublisher
	lookup    manifestLookupFn
	logger    ports.Logger
}

func newHostEventService(publisher ports.EventPublisher, lookup manifestLookupFn, logger ports.Logger) *hostEventService {
	return &hostEventService{
		publisher: publisher,
		lookup:    lookup,
		logger:    logger,
	}
}

func (s *hostEventService) Emit(ctx context.Context, req *pluginv1.EmitRequest) (*pluginv1.EmitResponse, error) {
	if !s.validateEmitPermission(req.GetSourcePlugin(), req.GetEventType()) {
		return &pluginv1.EmitResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("plugin %q not authorized to emit event type %q", req.GetSourcePlugin(), req.GetEventType()),
		}, nil
	}

	event := s.emitRequestToDomainEvent(req)

	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.Warn("failed to publish event", "plugin", req.GetSourcePlugin(), "eventType", req.GetEventType(), "error", err)
		return &pluginv1.EmitResponse{Success: false}, nil
	}

	return &pluginv1.EmitResponse{
		Success: true,
		EventId: event.ID,
	}, nil
}

func (s *hostEventService) validateEmitPermission(pluginName, eventType string) bool {
	info, ok := s.lookup(pluginName)
	if !ok {
		return false
	}
	if !info.Manifest.HasCapability(pluginmodel.CapabilityEvents) {
		return false
	}
	for _, pattern := range info.Manifest.Events.Emit {
		if matchEventPattern(pattern, eventType) {
			return true
		}
	}
	return false
}

func (s *hostEventService) emitRequestToDomainEvent(req *pluginv1.EmitRequest) *pluginmodel.DomainEvent {
	event := pluginmodel.NewDomainEvent(
		req.GetEventType(),
		req.GetSourcePlugin(),
		req.GetMetadata(),
		req.GetPayload(),
	)
	if req.GetTimestampUnixNanos() != 0 {
		event.Timestamp = time.Unix(0, req.GetTimestampUnixNanos())
	}
	event.PropagationDepth = int(req.GetPropagationDepth())
	return event
}
