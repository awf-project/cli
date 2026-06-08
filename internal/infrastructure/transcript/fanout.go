package transcript

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

type FanOutOption func(*FanOut)

type FanOutStats struct {
	Subscribers int
	Drops       uint64
}

type subscriber struct {
	ch         chan transcript.ExchangeEvent
	once       sync.Once
	dropCount  atomic.Uint64
	lastWarnAt atomic.Int64 // unix nano
}

type FanOut struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]*subscriber
	bufferSize  int
	logger      ports.Logger
	closed      bool
	totalDrops  atomic.Uint64
}

func NewFanOut(opts ...FanOutOption) *FanOut {
	fo := &FanOut{
		subscribers: make(map[uuid.UUID]*subscriber),
		bufferSize:  256,
		logger:      ports.NopLogger{},
	}

	for _, opt := range opts {
		opt(fo)
	}

	return fo
}

func WithBufferSize(size int) FanOutOption {
	return func(fo *FanOut) {
		fo.bufferSize = size
	}
}

func WithLogger(logger ports.Logger) FanOutOption {
	return func(fo *FanOut) {
		fo.logger = logger
	}
}

func (fo *FanOut) Subscribe() (events <-chan transcript.ExchangeEvent, unsubscribe func()) {
	fo.mu.Lock()
	defer fo.mu.Unlock()

	if fo.closed {
		return nil, func() {}
	}

	id := uuid.New()
	sub := &subscriber{
		ch: make(chan transcript.ExchangeEvent, fo.bufferSize),
	}
	fo.subscribers[id] = sub

	cancel := func() {
		fo.mu.Lock()
		delete(fo.subscribers, id)
		fo.mu.Unlock()
		sub.once.Do(func() { close(sub.ch) })
	}

	return sub.ch, cancel
}

func (fo *FanOut) Publish(event transcript.ExchangeEvent) { //nolint:gocritic // hugeParam: value semantics required; callers pass struct literals matching the domain port contract
	if event.Type == "" {
		fo.logger.Warn("publish called with zero-value event")
		return
	}

	fo.mu.RLock()
	defer fo.mu.RUnlock()

	for _, sub := range fo.subscribers {
		select {
		case sub.ch <- event:
		default:
			sub.dropCount.Add(1)
			fo.totalDrops.Add(1)
			last := sub.lastWarnAt.Load()
			if now := time.Now().UnixNano(); now-last > int64(time.Second) {
				sub.lastWarnAt.Store(now)
				fo.logger.Warn("subscriber buffer full, dropping event")
			}
		}
	}
}

func (fo *FanOut) Stats() FanOutStats {
	fo.mu.RLock()
	subscribers := len(fo.subscribers)
	fo.mu.RUnlock()

	return FanOutStats{
		Subscribers: subscribers,
		Drops:       fo.totalDrops.Load(),
	}
}

func (fo *FanOut) Close() error {
	fo.mu.Lock()
	if fo.closed {
		fo.mu.Unlock()
		return nil
	}
	fo.closed = true
	subs := make([]*subscriber, 0, len(fo.subscribers))
	for _, sub := range fo.subscribers {
		subs = append(subs, sub)
	}
	fo.subscribers = make(map[uuid.UUID]*subscriber)
	fo.mu.Unlock()

	for _, sub := range subs {
		sub.once.Do(func() { close(sub.ch) })
	}
	return nil
}
