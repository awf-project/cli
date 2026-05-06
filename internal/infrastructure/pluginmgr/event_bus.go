package pluginmgr

import (
	"context"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
)

const (
	defaultEventBufferSize = 256
	maxPropagationDepth    = 3
)

// EventDeliverer decouples EventBus from the gRPC adapter, enabling unit testing with mock deliverers.
type EventDeliverer interface {
	DeliverEvent(ctx context.Context, event *pluginmodel.DomainEvent) ([]*pluginmodel.DomainEvent, error)
}

type eventEnvelope struct {
	ctx   context.Context
	event *pluginmodel.DomainEvent
}

type eventSubscription struct {
	pluginName string
	patterns   []string
	deliverer  EventDeliverer
	eventCh    chan *eventEnvelope
	done       chan struct{}
	stopped    chan struct{}
}

type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*eventSubscription
	logger        ports.Logger
	bufferSize    int
}

var _ ports.EventPublisher = (*EventBus)(nil)

// NewEventBus creates an EventBus with default buffer size.
func NewEventBus(logger ports.Logger) *EventBus {
	return NewEventBusWithBufferSize(logger, defaultEventBufferSize)
}

// NewEventBusWithBufferSize creates an EventBus with a custom buffer size (for testing).
func NewEventBusWithBufferSize(logger ports.Logger, size int) *EventBus {
	return &EventBus{
		subscriptions: make(map[string]*eventSubscription),
		logger:        logger,
		bufferSize:    size,
	}
}

// matchEventPattern matches an event type against a dot-segment glob pattern.
// '*' matches exactly one dot-segment; '.' is a segment separator.
func matchEventPattern(pattern, eventType string) bool {
	if pattern == "" || eventType == "" {
		return false
	}
	patternSegments := strings.Split(pattern, ".")
	eventSegments := strings.Split(eventType, ".")
	if len(patternSegments) != len(eventSegments) {
		return false
	}
	for i, seg := range patternSegments {
		if seg != "*" && seg != eventSegments[i] {
			return false
		}
	}
	return true
}

// matchesAnyPattern returns true if eventType matches any of the subscription patterns.
// A bare "*" acts as catch-all and matches any event type regardless of segment count.
func (b *EventBus) matchesAnyPattern(patterns []string, eventType string) bool {
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if matchEventPattern(p, eventType) {
			return true
		}
	}
	return false
}

func (b *EventBus) Subscribe(pluginName string, patterns []string, deliverer EventDeliverer) {
	sub := &eventSubscription{
		pluginName: pluginName,
		patterns:   patterns,
		deliverer:  deliverer,
		eventCh:    make(chan *eventEnvelope, b.bufferSize),
		done:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}

	b.mu.Lock()
	b.subscriptions[pluginName] = sub
	b.mu.Unlock()

	go b.runDelivery(sub)
}

func (b *EventBus) runDelivery(sub *eventSubscription) {
	defer close(sub.stopped)
	for {
		select {
		case <-sub.done:
			return
		case env, ok := <-sub.eventCh:
			if !ok {
				return
			}
			emitted, err := sub.deliverer.DeliverEvent(env.ctx, env.event)
			if err != nil {
				continue
			}
			for _, e := range emitted {
				depth := env.event.PropagationDepth + 1
				if e.PropagationDepth > depth {
					depth = e.PropagationDepth
				}
				if depth >= maxPropagationDepth {
					b.logger.Warn("propagation depth exceeded", "plugin", sub.pluginName, "depth", depth)
					continue
				}
				e.PropagationDepth = depth
				if pubErr := b.Publish(env.ctx, e); pubErr != nil {
					b.logger.Warn("failed to republish emitted event", "plugin", sub.pluginName)
				}
			}
		}
	}
}

func (b *EventBus) Unsubscribe(pluginName string) {
	b.mu.Lock()
	sub, ok := b.subscriptions[pluginName]
	if ok {
		delete(b.subscriptions, pluginName)
	}
	b.mu.Unlock()

	if ok {
		close(sub.done)
		<-sub.stopped
	}
}

func (b *EventBus) Publish(ctx context.Context, event *pluginmodel.DomainEvent) error {
	b.mu.RLock()
	subs := make([]*eventSubscription, 0, len(b.subscriptions))
	for _, sub := range b.subscriptions {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()

	env := &eventEnvelope{ctx: ctx, event: event}
	for _, sub := range subs {
		if !b.matchesAnyPattern(sub.patterns, event.Type) {
			continue
		}
		select {
		case sub.eventCh <- env:
		default:
			b.logger.Warn("event buffer full", "plugin", sub.pluginName, "eventType", event.Type)
		}
	}
	return nil
}

func (b *EventBus) Close() error {
	b.mu.RLock()
	plugins := make([]string, 0, len(b.subscriptions))
	for name := range b.subscriptions {
		plugins = append(plugins, name)
	}
	b.mu.RUnlock()

	for _, name := range plugins {
		b.Unsubscribe(name)
	}
	return nil
}
