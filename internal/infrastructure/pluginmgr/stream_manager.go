package pluginmgr

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
)

const defaultStreamTimeout = 5 * time.Second

// EventStreamSender is the client-side streaming interface for sending events to a plugin.
type EventStreamSender interface {
	Send(msg *pluginv1.EventStreamMessage) error
}

type streamEntry struct {
	stream EventStreamSender
	seqNum atomic.Uint64
}

// StreamManager tracks per-plugin client-side gRPC streams and provides EventDeliverer instances.
type StreamManager struct {
	mu      sync.RWMutex
	streams map[string]*streamEntry
	logger  ports.Logger
	timeout time.Duration
}

var _ EventDeliverer = (*streamDeliverer)(nil)

// NewStreamManager creates a StreamManager with the default send timeout.
func NewStreamManager(logger ports.Logger) *StreamManager {
	return &StreamManager{
		streams: make(map[string]*streamEntry),
		logger:  logger,
		timeout: defaultStreamTimeout,
	}
}

// RegisterStream stores a stream connection for pluginName.
func (m *StreamManager) RegisterStream(pluginName string, stream EventStreamSender) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams[pluginName] = &streamEntry{stream: stream}
}

// UnregisterStream removes the stream for pluginName.
func (m *StreamManager) UnregisterStream(pluginName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, pluginName)
}

// HasStream reports whether a stream is registered for pluginName.
func (m *StreamManager) HasStream(pluginName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.streams[pluginName]
	return ok
}

// GetDeliverer returns a streamDeliverer when a stream is registered, otherwise unaryFallback.
func (m *StreamManager) GetDeliverer(pluginName string, unaryFallback EventDeliverer) EventDeliverer {
	m.mu.RLock()
	entry, ok := m.streams[pluginName]
	m.mu.RUnlock()
	if !ok {
		return unaryFallback
	}
	return &streamDeliverer{
		manager:    m,
		pluginName: pluginName,
		entry:      entry,
		fallback:   unaryFallback,
	}
}

// Close removes all registered streams.
func (m *StreamManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams = make(map[string]*streamEntry)
}

type streamDeliverer struct {
	manager    *StreamManager
	pluginName string
	entry      *streamEntry
	fallback   EventDeliverer
}

func (d *streamDeliverer) DeliverEvent(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error) {
	seqNum := d.entry.seqNum.Add(1)
	msg := domainEventToStreamMessage(event, seqNum)

	err := sendWithTimeout(ctx, d.manager.timeout, func() error {
		return d.entry.stream.Send(msg)
	})
	if err != nil {
		d.manager.logger.Warn("stream send failed, falling back to unary", "plugin", d.pluginName, "error", err)
		d.manager.UnregisterStream(d.pluginName)
		return d.fallback.DeliverEvent(ctx, event)
	}
	return []*pluginmodel.DomainEvent{}, nil
}

func sendWithTimeout(ctx context.Context, timeout time.Duration, send func() error) error {
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- send()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("stream send cancelled: %w", ctx.Err())
	case <-timer.C:
		return fmt.Errorf("stream send timeout after %s", timeout)
	}
}
