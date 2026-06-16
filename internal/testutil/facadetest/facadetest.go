package facadetest

import (
	"context"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
)

var (
	_ ports.WorkflowFacade = (*Fake)(nil)
	_ ports.RunSession     = (*FakeSession)(nil)
)

// Fake is a scriptable test double for ports.WorkflowFacade.
// Use New() to construct, then chain Script/With* builder methods before calling Run.
type Fake struct {
	mu      sync.Mutex
	script  []ports.Event
	history []ports.RunRecord
}

// New returns a new Fake with an empty event script.
func New() *Fake {
	return &Fake{}
}

// Script appends events to the scripted sequence for the next Run call.
func (f *Fake) Script(events ...ports.Event) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script = append(f.script, events...)
	return f
}

// WithTerminalCompleted appends an EventWorkflowCompleted terminal event.
func (f *Fake) WithTerminalCompleted() *Fake {
	return f.Script(ports.Event{Kind: ports.EventWorkflowCompleted})
}

// WithTerminalFailed appends an EventWorkflowFailed terminal event.
// The Payload is *ports.EnrichedTerminal carrying the error message, matching
// the production emitTerminalEvent behavior. All consumers type-assert to
// *ports.EnrichedTerminal, so the fake must emit the same concrete type (B1).
func (f *Fake) WithTerminalFailed(err error) *Fake {
	var payload *ports.EnrichedTerminal
	if err != nil {
		payload = &ports.EnrichedTerminal{Error: err.Error()}
	}
	return f.Script(ports.Event{
		Kind:    ports.EventWorkflowFailed,
		Payload: payload,
	})
}

// WithInputRequired appends an EventInputRequired event with req as Payload.
// The FakeSession will pause after emitting this event until Respond is called.
func (f *Fake) WithInputRequired(req ports.InputRequest) *Fake {
	return f.Script(ports.Event{
		Kind:    ports.EventInputRequired,
		Payload: req,
	})
}

// WithHistory appends records to the scriptable history returned by History.
func (f *Fake) WithHistory(records ...ports.RunRecord) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.history = append(f.history, records...)
	return f
}

// List returns nil (zero-value stub).
func (f *Fake) List(_ context.Context) ([]ports.WorkflowSummary, error) {
	return nil, nil
}

// Validate returns a valid ValidationReport (zero-value stub).
func (f *Fake) Validate(_ context.Context, _ ports.RunRequest) (ports.ValidationReport, error) {
	return ports.ValidationReport{Valid: true}, nil
}

// Status returns an empty RunStatus (zero-value stub).
func (f *Fake) Status(_ context.Context, _ string) (ports.RunStatus, error) {
	return ports.RunStatus{}, nil
}

// History returns the scripted history records seeded via WithHistory.
func (f *Fake) History(_ context.Context, _ ports.HistoryFilter) ([]ports.RunRecord, error) { //nolint:gocritic // hugeParam: ports.WorkflowFacade contract requires value type; pointer would break conformance
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]ports.RunRecord, len(f.history))
	copy(result, f.history)
	return result, nil
}

// Run creates a FakeSession that emits the scripted event sequence.
// Returns ErrInvalidRequest if req.Identifier is empty.
// Always succeeds even if ctx is already cancelled — the scripted events are emitted unconditionally.
func (f *Fake) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	if req.Identifier == "" {
		return nil, ports.ErrInvalidRequest
	}
	f.mu.Lock()
	script := make([]ports.Event, len(f.script))
	copy(script, f.script)
	f.mu.Unlock()
	sess := newFakeSession(req.Identifier, script)
	go sess.pump(ctx)
	return sess, nil
}

// RunStep is intentionally absent from Fake: WorkflowFacade no longer includes
// single-step isolation (M1). Single-step execution is a CLI-only concern exposed
// through ports.SingleStepRunner, implemented by *application.Adapter directly.

// Resume returns a live FakeSession seeded from req.RunID with no scripted events.
// The session's Events() channel closes immediately (empty script).
// req.InputOverrides and req.FromStep satisfy the ports.WorkflowFacade contract
// but are not used by the fake — it always emits the scripted sequence regardless.
func (f *Fake) Resume(ctx context.Context, req ports.ResumeRequest) (ports.RunSession, error) {
	sess := newFakeSession(req.RunID, nil)
	go sess.pump(ctx)
	return sess, nil
}

// FakeSession emits scripted events on Events() in order.
// After EventInputRequired, the next event is withheld until Respond is called.
// Terminal events (EventWorkflowCompleted, EventWorkflowFailed) close the channel.
type FakeSession struct {
	id        string
	script    []ports.Event
	events    chan ports.Event
	mu        sync.Mutex
	closeOnce sync.Once
	closed    bool
	doneCh    chan struct{} // closed to signal pump to stop
	pumpDone  chan struct{} // closed when pump has fully exited
	respondCh chan struct{} // non-nil while waiting for Respond after InputRequired
}

func newFakeSession(id string, script []ports.Event) *FakeSession {
	return &FakeSession{
		id:       id,
		script:   script,
		events:   make(chan ports.Event, len(script)+1),
		doneCh:   make(chan struct{}),
		pumpDone: make(chan struct{}),
	}
}

// pump sends scripted events onto the events channel.
// Pauses after EventInputRequired until Respond is called.
// Closes the events channel when the script is exhausted, a terminal event is emitted,
// or doneCh is signaled. Ctx cancellation does NOT abort event delivery — scripted events
// are always emitted. Ctx is only respected when waiting for an InputRequired response.
func (s *FakeSession) pump(ctx context.Context) {
	defer close(s.pumpDone)
	defer close(s.events)
	for _, ev := range s.script {
		select {
		case s.events <- ev:
		case <-s.doneCh:
			return
		}
		if ev.Kind == ports.EventInputRequired {
			ch := s.activateRespond()
			select {
			case <-ch:
			case <-s.doneCh:
				return
			case <-ctx.Done():
				return
			}
		}
		if ev.Kind == ports.EventWorkflowCompleted || ev.Kind == ports.EventWorkflowFailed {
			return
		}
	}
}

func (s *FakeSession) activateRespond() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan struct{})
	s.respondCh = ch
	return ch
}

// ID returns the session identifier derived from RunRequest.Identifier.
func (s *FakeSession) ID() string { return s.id }

// Events returns the channel of scripted events.
// The channel is closed after a terminal event, context cancellation, or Close.
func (s *FakeSession) Events() <-chan ports.Event { return s.events }

// Respond unblocks the pump goroutine after an EventInputRequired event.
// Returns ErrSessionClosed if the session is already closed.
func (s *FakeSession) Respond(_ ports.InputResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ports.ErrSessionClosed
	}
	ch := s.respondCh
	if ch != nil {
		s.respondCh = nil
		close(ch)
	}
	return nil
}

// Err always returns nil for the fake.
func (s *FakeSession) Err() error { return nil }

// StatusSnapshot derives a coarse status from the scripted events, mirroring
// *application.RunSession.StatusSnapshot so interface-layer status overlays (e.g. the HTTP
// GET /executions/{id} handler) can be exercised against the fake.
func (s *FakeSession) StatusSnapshot() ports.RunStatus {
	snap := ports.RunStatus{RunID: s.id, Status: ports.RunStateRunning}
	for _, ev := range s.script {
		switch ev.Kind {
		case ports.EventStepStarted:
			if p, ok := ev.Payload.(*ports.EnrichedStepPayload); ok {
				snap.CurrentStep = p.StepName
			}
		case ports.EventStepCompleted:
			snap.CurrentStep = ""
		case ports.EventWorkflowCompleted:
			snap.Status = ports.RunStateCompleted
			snap.CurrentStep = ""
			snap.CompletedAt = ev.Timestamp
		case ports.EventWorkflowFailed:
			snap.Status = ports.RunStateFailed
			snap.CurrentStep = ""
			snap.CompletedAt = ev.Timestamp
		default:
			// Other event kinds do not change the coarse run status.
		}
	}
	return snap
}

// Close stops event emission and waits for the pump goroutine to exit.
// Idempotent: multiple calls return nil without panicking.
func (s *FakeSession) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.doneCh)
		<-s.pumpDone
	})
	return nil
}
