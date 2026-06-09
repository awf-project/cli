package application

import (
	"context"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

var _ ports.RunSession = (*RunSession)(nil)

// RunSession is the concrete implementation of ports.RunSession.
// The adapter (T058) owns the subscription goroutine and drives appendEvent.
// No goroutine is started here — see constraint in T057.
//
// Concurrency contract for appendEvent / Close:
//   - s.closed is set to true inside Close() while holding s.mu (write lock).
//   - appendEvent() checks s.closed under s.mu (write lock) before any send.
//     If closed, it returns without touching s.events.
//   - The channel send in appendEvent() is performed while still holding s.mu,
//     so it is serialized against the close(s.events) in Close().
//   - Receivers (range s.Events()) never take s.mu, so holding s.mu for the
//     non-blocking send cannot cause a deadlock.
type RunSession struct {
	id               string
	events           chan ports.Event
	respondCh        chan ports.InputResponse
	replayBuffer     []ports.Event
	replayBufferSize int
	head             int
	size             int
	err              error
	closed           bool
	closeOnce        sync.Once
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	// onClose is called exactly once inside Close(), under sync.Once, after the
	// events channel is closed. The adapter uses this to remove the session from
	// the registry synchronously (BUG #7). nil is safe — Close() guards with a nil check.
	onClose func()
}

func newRunSession(id string, ctx context.Context, replayBufferSize int) *RunSession { //nolint:revive // context.Context as non-first param is intentional here
	if replayBufferSize <= 0 {
		replayBufferSize = 256
	}
	childCtx, cancel := context.WithCancel(ctx) //nolint:gocritic,gosec // cancel is stored in RunSession.cancel and called by Close(); G118 false positive
	return &RunSession{
		id:               id,
		events:           make(chan ports.Event, replayBufferSize),
		respondCh:        make(chan ports.InputResponse, 1),
		replayBuffer:     make([]ports.Event, replayBufferSize),
		replayBufferSize: replayBufferSize,
		ctx:              childCtx,
		cancel:           cancel,
	}
}

func (s *RunSession) ID() string {
	return s.id
}

func (s *RunSession) Events() <-chan ports.Event {
	return s.events
}

// Respond delivers a response to the input bridge consumer (T059).
// Non-blocking: if no consumer is parked, returns ErrDuplicateResponse (capacity=1 buffer full).
// Returns ErrSessionClosed without panicking if called after Close.
func (s *RunSession) Respond(r ports.InputResponse) error {
	select {
	case <-s.ctx.Done():
		return ports.ErrSessionClosed
	default:
	}
	select {
	case s.respondCh <- r:
		return nil
	default:
		return ports.ErrDuplicateResponse
	}
}

func (s *RunSession) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// Close is idempotent via sync.Once. Cancels ctx and closes the events channel.
// Buffered events are NOT drained: a closed buffered channel still yields its
// buffered values to range readers, so all in-flight events remain visible.
//
// If an onClose hook was registered (by the adapter for registry eviction, BUG #7),
// it is called synchronously inside the Once block after the channel is closed.
func (s *RunSession) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.mu.Lock()
		s.closed = true
		close(s.events)
		s.mu.Unlock()
		if s.onClose != nil {
			s.onClose()
		}
	})
	return nil
}

// appendEvent writes ev to the events channel and appends to the replay buffer.
// When the buffer is at capacity, the oldest entry is overwritten (drop-oldest policy).
// Called by the adapter goroutine (T058); safe under mu.
//
// The channel send is performed while holding s.mu to prevent the TOCTOU race:
// Close() sets s.closed=true and calls close(s.events) under the same lock, so
// appendEvent will either see s.closed==true (and skip the send) or complete
// the non-blocking send before Close() closes the channel - never both.
func (s *RunSession) appendEvent(ev ports.Event) { //nolint:gocritic // hugeParam: Event is part of the ports contract; pointer indirection would couple adapters to *Event
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	if s.size < s.replayBufferSize {
		s.replayBuffer[(s.head+s.size)%s.replayBufferSize] = ev
		s.size++
	} else {
		// overwrite oldest: advance head
		s.replayBuffer[s.head] = ev
		s.head = (s.head + 1) % s.replayBufferSize
	}

	select {
	case s.events <- ev:
	default:
		// channel full: event dropped — back-pressure documented in spec edge case
	}
}

// replayFromSeq returns buffered events with Seq >= seq in monotonic order.
// Events evicted from the bounded buffer are silently skipped (overflow documented per spec).
func (s *RunSession) replayFromSeq(seq uint64) []ports.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ports.Event, 0, s.size)
	for i := 0; i < s.size; i++ {
		ev := s.replayBuffer[(s.head+i)%s.replayBufferSize]
		if ev.Seq >= seq {
			result = append(result, ev)
		}
	}
	return result
}

// setErr stores the terminal cause; called by the adapter before Close.
func (s *RunSession) setErr(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}
