package application

import (
	"context"
	"errors"
	"sync"
	"time"

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
	// inputs stores the workflow input values supplied at run-start so that
	// StatusSnapshot can expose them without re-deriving them from the event stream
	// (inputs are not emitted as events). Set once AFTER construction via setInputs
	// (called by Adapter.Run immediately after newSession returns);
	// nil when not supplied (Resume path or tests that do not pass inputs).
	inputs map[string]any
	// onClose is called exactly once inside Close(), under sync.Once, after the
	// events channel is closed. The adapter uses this to remove the session from
	// the registry synchronously (BUG #7). nil is safe — Close() guards with a nil check.
	onClose func()
}

func newRunSession(ctx context.Context, id string, replayBufferSize int) *RunSession {
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
//
// The closed flag is checked under a read lock before the select to give a
// deterministic ErrSessionClosed when the session is already closed. Without the
// lock-guarded pre-check, the three-branch select races between ctx.Done() and the
// empty respondCh send — both are immediately ready after Close(), so the scheduler
// could non-deterministically choose either branch.
func (s *RunSession) Respond(r ports.InputResponse) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ports.ErrSessionClosed
	}
	s.mu.RUnlock()

	select {
	case <-s.ctx.Done():
		return ports.ErrSessionClosed
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

// CancelWithEvent emits a terminal cancellation event before closing the session.
// It is used by interfaces that cancel a live facade run directly through the
// session instead of waiting for the execution goroutine to return.
func (s *RunSession) CancelWithEvent(reason string) error {
	if reason == "" {
		reason = "cancelled"
	}
	s.setErr(errors.New(reason))
	s.appendEvent(ports.Event{
		Kind:      ports.EventWorkflowFailed,
		RunID:     s.id,
		Timestamp: time.Now(),
		Payload:   &ports.EnrichedTerminal{Error: reason},
	})
	return s.Close()
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

// ReplayFromSeq is the exported form of replayFromSeq consumed at the interface boundary —
// the SSE handler type-asserts this method on ports.RunSession to replay buffered events on
// a Last-Event-ID reconnect. Without it the assertion silently fails and replay is a no-op.
func (s *RunSession) ReplayFromSeq(seq uint64) []ports.Event {
	return s.replayFromSeq(seq)
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

// StatusSnapshot derives a rich run status from the buffered event history WITHOUT
// consuming the live Events() channel: it scans the replay buffer under a read lock, so it
// never competes with the SSE/CLI/TUI consumer for events. Interfaces that need a
// point-in-time status (HTTP GET /executions/{id}, awf status) use this while the live
// stream stays owned by a single consumer.
//
// Status starts at "running" (a tracked session is, by definition, underway) and is refined
// as the buffer is scanned in order:
//   - EventRunStarted stamps StartedAt.
//   - EventStepStarted records the step in stepIndex (observation order) and sets CurrentStep.
//   - EventStepCompleted stamps the step's CompletedAt, clears CurrentStep, and increments
//     Progress.Completed.
//   - EventWorkflowFailed / EventWorkflowCompleted set the terminal Status and CompletedAt.
//
// A closed session keeps its buffer, so a completed run still reports "completed" after
// Close(). When the recorder is a NopRecorder (HTTP/TUI facade) only the terminal event is
// buffered, so StartedAt/CurrentStep stay zero and the caller supplies StartedAt from its
// own tracking.
//
// Inputs is populated from the session's stored inputs field (set once AFTER construction
// by setInputs, called by Adapter.Run immediately after newSession returns). It is nil
// when inputs were not supplied at run-start (Resume path, tests).
func (s *RunSession) StatusSnapshot() ports.RunStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap := ports.RunStatus{
		RunID:  s.id,
		Status: ports.RunStateRunning,
		Inputs: s.inputs,
	}

	acc := newStepAccumulator()

	for i := 0; i < s.size; i++ {
		ev := s.replayBuffer[(s.head+i)%s.replayBufferSize]
		acc.apply(&snap, &ev)
	}

	stepOrder := acc.order
	stepMap := acc.byName

	// Build the ordered Steps slice and compute Progress from the final step states.
	if len(stepOrder) > 0 {
		snap.Steps = make([]ports.StepStatus, len(stepOrder))
		for _, name := range stepOrder {
			e := stepMap[name]
			snap.Steps[e.idx] = e.status
			snap.Progress.Total++
			switch e.status.Status {
			case ports.RunStateCompleted:
				snap.Progress.Completed++
			case ports.RunStateFailed:
				snap.Progress.Failed++
			case ports.RunStatePending, ports.RunStateRunning, ports.RunStateCancelled:
				// In-progress or non-terminal step states are counted only in Total,
				// not in Completed/Failed buckets.
			}
		}
	}

	return snap
}

// stepEntry tracks a single step's position in observation order plus its accumulated
// status as events are replayed.
type stepEntry struct {
	idx    int
	status ports.StepStatus
}

// stepAccumulator folds the replay buffer into ordered per-step status while also
// updating run-level snapshot fields. It is split out of StatusSnapshot to keep the
// per-event dispatch readable and below the cognitive-complexity threshold; it holds
// no concurrency state and is used under the caller's RLock.
type stepAccumulator struct {
	order  []string
	byName map[string]*stepEntry
}

func newStepAccumulator() *stepAccumulator {
	return &stepAccumulator{byName: make(map[string]*stepEntry)}
}

// apply folds a single event into the snapshot and the per-step accumulator.
func (a *stepAccumulator) apply(snap *ports.RunStatus, ev *ports.Event) {
	switch ev.Kind {
	case ports.EventRunStarted:
		snap.StartedAt = ev.Timestamp
	case ports.EventStepStarted:
		a.applyStepStarted(snap, ev)
	case ports.EventStepCompleted:
		a.applyStepCompleted(snap, ev)
	case ports.EventWorkflowCompleted:
		snap.Status = ports.RunStateCompleted
		snap.CurrentStep = ""
		snap.CompletedAt = ev.Timestamp
	case ports.EventWorkflowFailed:
		snap.Status = ports.RunStateFailed
		snap.CurrentStep = ""
		snap.CompletedAt = ev.Timestamp
	default:
		// Other event kinds (messages, tool calls, input.required, call-workflow,
		// run.started/completed, unknown) do not change the run or step status.
	}
}

func (a *stepAccumulator) applyStepStarted(snap *ports.RunStatus, ev *ports.Event) {
	p, ok := ev.Payload.(*ports.EnrichedStepPayload)
	if !ok || p.StepName == "" {
		return
	}
	snap.CurrentStep = p.StepName
	if e, seen := a.byName[p.StepName]; seen {
		// Step restarted (resume): update StartedAt and reset status.
		e.status.Status = ports.RunStateRunning
		e.status.StartedAt = ev.Timestamp
		e.status.CompletedAt = nilTime()
		e.status.Error = ""
		return
	}
	// First time we see this step: register it in observation order.
	a.byName[p.StepName] = &stepEntry{
		idx: len(a.order),
		status: ports.StepStatus{
			Name:      p.StepName,
			Status:    ports.RunStateRunning,
			StartedAt: ev.Timestamp,
		},
	}
	a.order = append(a.order, p.StepName)
}

func (a *stepAccumulator) applyStepCompleted(snap *ports.RunStatus, ev *ports.Event) {
	// Step finished; no step is in progress until the next one starts.
	snap.CurrentStep = ""
	p, ok := ev.Payload.(*ports.EnrichedStepPayload)
	if !ok || p.StepName == "" {
		return
	}
	e, seen := a.byName[p.StepName]
	if !seen {
		return
	}
	e.status.Status = ports.RunStateCompleted
	e.status.CompletedAt = ev.Timestamp
	if p.Error != "" {
		e.status.Status = ports.RunStateFailed
		e.status.Error = p.Error
	}
}

// nilTime returns the zero time.Time, used to reset a previously-stamped CompletedAt
// when a step is restarted via resume. This tiny helper documents the intent.
func nilTime() time.Time { return time.Time{} }

// setInputs stores the workflow inputs supplied at run-start. Called once by the adapter
// after session construction so StatusSnapshot can expose them to callers without
// re-deriving inputs from the event stream (inputs are not emitted as events).
// A nil map is safe — it signals "no inputs supplied" (e.g. Resume path).
func (s *RunSession) setInputs(inputs map[string]any) {
	s.mu.Lock()
	s.inputs = inputs
	s.mu.Unlock()
}

// setErr stores the terminal cause; called by the adapter before Close.
func (s *RunSession) setErr(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}
