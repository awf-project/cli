package facadetest

import (
	"context"
	"fmt"
	"sync"

	"github.com/awf-project/cli/internal/application"
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
// The event Payload is set to the ErrorCode mapped from err via application.MapError.
func (f *Fake) WithTerminalFailed(err error) *Fake {
	return f.Script(ports.Event{
		Kind:    ports.EventWorkflowFailed,
		Payload: application.MapError(err),
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
// Returns the context error if ctx is already cancelled.
func (f *Fake) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	if req.Identifier == "" {
		return nil, ports.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("facadetest: run context: %w", err)
	}
	f.mu.Lock()
	script := make([]ports.Event, len(f.script))
	copy(script, f.script)
	f.mu.Unlock()
	sess := newFakeSession(req.Identifier, script)
	go sess.pump(ctx)
	return sess, nil
}

// Resume returns nil (zero-value stub).
func (f *Fake) Resume(_ context.Context, _ string) (ports.RunSession, error) {
	return nil, nil
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
// the context is cancelled, or doneCh is signaled.
func (s *FakeSession) pump(ctx context.Context) {
	defer close(s.pumpDone)
	defer close(s.events)
	for _, ev := range s.script {
		select {
		case s.events <- ev:
		case <-s.doneCh:
			return
		case <-ctx.Done():
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
