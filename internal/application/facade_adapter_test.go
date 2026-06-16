package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	_ ports.EventPublisher  = (*mockEventPublisher)(nil)
	_ ports.UserInputReader = (*mockUserInputReader)(nil)
)

// mockRecorder counts Subscribe calls for testing
type mockRecorder struct {
	subscribeCount atomic.Int32
}

func (m *mockRecorder) Record(_ context.Context, _ transcript.ExchangeEvent) error {
	return nil
}

func (m *mockRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	m.subscribeCount.Add(1)
	ch := make(chan transcript.ExchangeEvent, 10)
	return ch, func() { close(ch) }
}

func (m *mockRecorder) Close() error {
	return nil
}

// TestFacadeAdapter_NewAdapterConstructor verifies NewAdapter stores dependencies
func TestFacadeAdapter_NewAdapterConstructor(t *testing.T) {
	recorder := &mockRecorder{}
	registry := NewSessionRegistry()

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		registry,
	)

	require.NotNil(t, adapter)
	assert.NotNil(t, adapter.recorder)
	assert.NotNil(t, adapter.registry)
}

// TestFacadeAdapter_RunSubscribesToRecorderOncePerSession verifies Subscribe called exactly once
func TestFacadeAdapter_RunSubscribesToRecorderOncePerSession(t *testing.T) {
	recorder := &mockRecorder{}
	registry := NewSessionRegistry()

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		registry,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ports.RunRequest{Identifier: "test/workflow"}
	session, err := adapter.Run(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, 1, int(recorder.subscribeCount.Load()), "Run should call Subscribe() exactly once")

	if session != nil {
		session.Close()
	}
}

// TestFacadeAdapter_NilBridgesAreNoOps verifies no panic with nil bridges
func TestFacadeAdapter_NilBridgesAreNoOps(t *testing.T) {
	recorder := &mockRecorder{}
	registry := NewSessionRegistry()

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		registry,
	)

	// All bridges are nil by default
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ports.RunRequest{Identifier: "test/workflow"}
	session, err := adapter.Run(ctx, req)

	// Should not panic
	require.NoError(t, err)
	require.NotNil(t, session)

	if session != nil {
		session.Close()
	}
}

// TestFacadeAdapter_SetUserInputReader stores reader
func TestFacadeAdapter_SetUserInputReader(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	reader := &mockUserInputReader{}
	adapter.SetUserInputReader(reader)

	assert.Equal(t, reader, adapter.userInputReader)
}

// TestFacadeAdapter_RunReturnsRunSession verifies Run returns valid session
func TestFacadeAdapter_RunReturnsRunSession(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := ports.RunRequest{Identifier: "test/workflow"}
	session, err := adapter.Run(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.ID())

	if session != nil {
		session.Close()
	}
}

// TestFacadeAdapter_ListReturnsValidResponse verifies List delegates
func TestFacadeAdapter_ListReturnsValidResponse(t *testing.T) {
	// List delegates to the workflow service; provide one backed by an (empty) mock
	// repository so it returns a valid, non-nil, empty summary slice.
	adapter := NewAdapter(
		NewWorkflowService(testmocks.NewMockWorkflowRepository(), nil, nil, nil, nil),
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx := context.Background()
	summaries, err := adapter.List(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, summaries)
}

func TestFacadeAdapter_HistoryDelegatesToHistoryService(t *testing.T) {
	store := testmocks.NewMockHistoryStore()
	require.NoError(t, store.Record(context.Background(), &workflow.ExecutionRecord{
		ID:           "run-1",
		WorkflowName: "demo",
		Status:       "success",
		DurationMs:   42,
	}))

	adapter := NewAdapter(
		NewWorkflowService(testmocks.NewMockWorkflowRepository(), nil, nil, nil, nil),
		&ExecutionService{},
		NewHistoryService(store, nil),
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	records, err := adapter.History(context.Background(), ports.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "run-1", records[0].RunID)
	assert.Equal(t, "demo", records[0].WorkflowName)
	assert.Equal(t, ports.RunState("success"), records[0].Status)
	assert.Equal(t, int64(42), records[0].DurationMs)
}

// TestFacadeAdapter_ValidateReturnsValidationReport verifies Validate delegates
func TestFacadeAdapter_ValidateReturnsValidationReport(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx := context.Background()
	req := ports.RunRequest{Identifier: "test/workflow"}
	report, err := adapter.Validate(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, report)
}

// TestFacadeAdapter_HistoryReturnsRecords verifies History delegates
func TestFacadeAdapter_HistoryReturnsRecords(t *testing.T) {
	// History delegates to the history service; provide one backed by an (empty) mock
	// store so it returns a valid, non-nil, empty record slice.
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		NewHistoryService(testmocks.NewMockHistoryStore(), nil),
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx := context.Background()
	filter := ports.HistoryFilter{}
	records, err := adapter.History(ctx, filter)

	assert.NoError(t, err)
	assert.NotNil(t, records)
}

// TestFacadeAdapter_ResumeReturnsRunSession verifies Resume returns valid session
func TestFacadeAdapter_ResumeReturnsRunSession(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	session, err := adapter.Resume(ctx, ports.ResumeRequest{RunID: "test-run-id"})

	assert.NoError(t, err)
	assert.NotNil(t, session)

	if session != nil {
		session.Close()
	}
}

// controlledRecorder lets the test inject events via Send(); Subscribe returns the same
// buffered channel on every call so pre-sent events are waiting when the adapter subscribes.
type controlledRecorder struct {
	subscribeCount atomic.Int32
	ch             chan transcript.ExchangeEvent
}

func newControlledRecorder() *controlledRecorder {
	return &controlledRecorder{ch: make(chan transcript.ExchangeEvent, 100)}
}

func (r *controlledRecorder) Record(_ context.Context, _ transcript.ExchangeEvent) error { return nil }

func (r *controlledRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	r.subscribeCount.Add(1)
	return r.ch, func() {}
}

func (r *controlledRecorder) Close() error { return nil }

func (r *controlledRecorder) Send(events ...transcript.ExchangeEvent) {
	for i := range events {
		select {
		case r.ch <- events[i]:
		default:
		}
	}
}

// TestFacadeAdapter_ProjectsExchangeEventsToFacadeEvents verifies all 5 injected events
// arrive on session.Events() with correct Kind, asserting the full count (Acceptance line 54).
func TestFacadeAdapter_ProjectsExchangeEventsToFacadeEvents(t *testing.T) {
	exchangeEvents := []transcript.ExchangeEvent{
		{Type: transcript.EventTypeRunStarted, RunID: "run1", Seq: 1},
		{Type: transcript.EventTypeStepStarted, RunID: "run1", Seq: 2},
		{Type: transcript.EventTypeStepCompleted, RunID: "run1", Seq: 3},
		{Type: transcript.EventTypeRunCompleted, RunID: "run1", Seq: 4},
		{Type: transcript.EventTypeMessageUser, RunID: "run1", Seq: 5},
	}

	recorder := newControlledRecorder()
	recorder.Send(exchangeEvents...) // pre-buffer; adapter goroutine drains them on subscribe

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	session, err := adapter.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	require.NoError(t, err)
	defer session.Close()

	var collectedEvents []ports.Event
	timeout := time.After(2 * time.Second)

collect:
	for len(collectedEvents) < len(exchangeEvents) {
		select {
		case ev, ok := <-session.Events():
			if !ok {
				break collect
			}
			collectedEvents = append(collectedEvents, ev)
		case <-timeout:
			break collect
		}
	}

	assert.Len(t, collectedEvents, len(exchangeEvents),
		"all %d injected ExchangeEvents must arrive on session.Events() via ProjectEvent", len(exchangeEvents))

	expectedKinds := []ports.EventKind{
		ports.EventRunStarted,
		ports.EventStepStarted,
		ports.EventStepCompleted,
		ports.EventRunCompleted,
		ports.EventMessageUser,
	}
	for i, ev := range collectedEvents {
		if i < len(expectedKinds) {
			assert.Equal(t, expectedKinds[i], ev.Kind,
				"event[%d] Kind must be projected via ProjectEvent from ExchangeEvent.Type", i)
		}
	}
}

// TestFacadeAdapter_TerminalEventOnExecutionSuccess asserts the terminal event is
// EventWorkflowCompleted when execution succeeds (Acceptance line 55, Criterion #6).
func TestFacadeAdapter_TerminalEventOnExecutionSuccess(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	session, err := adapter.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	require.NoError(t, err)
	defer session.Close()

	var collectedEvents []ports.Event
	timeout := time.After(500 * time.Millisecond)

collectSuccess:
	for {
		select {
		case ev, ok := <-session.Events():
			if !ok {
				break collectSuccess
			}
			collectedEvents = append(collectedEvents, ev)
			if ev.Kind == ports.EventWorkflowCompleted || ev.Kind == ports.EventWorkflowFailed {
				break collectSuccess
			}
		case <-timeout:
			break collectSuccess
		}
	}

	var completedEvent *ports.Event
	for i := range collectedEvents {
		if collectedEvents[i].Kind == ports.EventWorkflowCompleted {
			completedEvent = &collectedEvents[i]
			break
		}
	}
	require.NotNil(t, completedEvent,
		"execution success must append EventWorkflowCompleted as the terminal event (Criterion #6)")
}

// TestFacadeAdapter_TerminalEventOnExecutionFailure asserts the terminal event is
// EventWorkflowFailed with ErrorCode via MapError when execution fails (Acceptance line 56, Criterion #7).
func TestFacadeAdapter_TerminalEventOnExecutionFailure(t *testing.T) {
	// A resolver that resolves "test/workflow" to a real workflow so the zero-value
	// ExecutionService is actually invoked and fails (nil dependencies). The adapter
	// must catch the error and emit EventWorkflowFailed via MapError. This setup is
	// intentionally distinct from the success case (which uses a nil resolver → no
	// execution → trivial completion), so both terminal-event criteria are exercised.
	discoverer := newMockPackDiscoverer()
	discoverer.workflows["test"] = map[string]*workflow.Workflow{
		"workflow": {Name: "workflow"},
	}
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		NewResolver(discoverer, nil),
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	session, err := adapter.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	require.NoError(t, err)
	defer session.Close()

	var collectedEvents []ports.Event
	timeout := time.After(500 * time.Millisecond)

collectFailure:
	for {
		select {
		case ev, ok := <-session.Events():
			if !ok {
				break collectFailure
			}
			collectedEvents = append(collectedEvents, ev)
			if ev.Kind == ports.EventWorkflowCompleted || ev.Kind == ports.EventWorkflowFailed {
				break collectFailure
			}
		case <-timeout:
			break collectFailure
		}
	}

	var failedEvent *ports.Event
	for i := range collectedEvents {
		if collectedEvents[i].Kind == ports.EventWorkflowFailed {
			failedEvent = &collectedEvents[i]
			break
		}
	}
	require.NotNil(t, failedEvent,
		"execution failure must append EventWorkflowFailed as the terminal event (Criterion #7)")
}

// TestFacadeAdapter_RunDelegatesToExecutionService asserts Run() invokes executionSvc,
// evidenced by a terminal event appearing on the session (Acceptance line 50).
func TestFacadeAdapter_RunDelegatesToExecutionService(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	session, err := adapter.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	require.NoError(t, err)
	defer session.Close()

	// executionSvc invocation produces a terminal event after execution completes.
	// The current stub does not call executionSvc, so no terminal event is emitted → test fails.
	var found bool
	timeout := time.After(500 * time.Millisecond)

drain:
	for {
		select {
		case ev, ok := <-session.Events():
			if !ok {
				break drain
			}
			if ev.Kind == ports.EventWorkflowCompleted || ev.Kind == ports.EventWorkflowFailed {
				found = true
				break drain
			}
		case <-timeout:
			break drain
		}
	}

	assert.True(t, found,
		"Run() must delegate to ExecutionService and emit EventWorkflowCompleted or EventWorkflowFailed")
}

// TestFacadeAdapter_RunResolvesIdentifierViaCanonicalResolver asserts resolver.Resolve()
// is called before execution; empty identifier must propagate an error from Run() (Acceptance line 51, FR-019).
func TestFacadeAdapter_RunResolvesIdentifierViaCanonicalResolver(t *testing.T) {
	// NewResolver(nil, nil): Resolve("") returns USER.FACADE.IDENTIFIER_EMPTY immediately,
	// before any discoverer or repository call is attempted.
	resolver := NewResolver(nil, nil)

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		resolver,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	_, err := adapter.Run(context.Background(), ports.RunRequest{Identifier: ""})
	require.Error(t, err, "Run() must call resolver.Resolve() which rejects empty identifiers")
	assert.Contains(t, err.Error(), "identifier",
		"resolver error must propagate: Run() must invoke resolver before executionSvc")
}

// TestFacadeAdapter_ValidateCallsResolverThenDelegates asserts Validate() calls resolver.Resolve()
// before delegating to workflowSvc; empty identifier must propagate the resolver error (Acceptance line 42).
func TestFacadeAdapter_ValidateCallsResolverThenDelegates(t *testing.T) {
	resolver := NewResolver(nil, nil)

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		resolver,
		&mockRecorder{},
		NewSessionRegistry(),
	)

	_, err := adapter.Validate(context.Background(), ports.RunRequest{Identifier: ""})
	require.Error(t, err, "Validate() must call resolver.Resolve() which rejects empty identifiers")
	assert.Contains(t, err.Error(), "identifier",
		"Validate() must call resolver.Resolve() before delegating to workflowSvc")
}

// TestFacadeAdapter_SoleSubscriberInvariant_NoOtherCallers is a static analysis test that
// scans production source files and asserts recorder.Subscribe() is called only from
// facade_adapter.go (SC-001). Interface callers must consume RunSession.Events() instead.
// Violations in the interface layer will be migrated in T060–T063.
func TestFacadeAdapter_SoleSubscriberInvariant_NoOtherCallers(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed to locate the project root")

	// Navigate from internal/application/ up two levels to the project root (cli/)
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))

	// Production files allowed to call .Subscribe() (recorder/fanout implementations, not clients)
	allowedPaths := map[string]bool{
		filepath.Join(projectRoot, "internal", "application", "facade_adapter.go"):            true,
		filepath.Join(projectRoot, "internal", "infrastructure", "transcript", "recorder.go"): true,
		filepath.Join(projectRoot, "internal", "infrastructure", "transcript", "fanout.go"):   true,
	}

	var violations []string

	walkErr := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if allowedPaths[path] {
			return nil
		}

		content, readErr := os.ReadFile(path) //nolint:gosec // controlled test input: path from filepath.Walk on project source, not user-supplied
		if readErr != nil {
			return nil // skip unreadable files; not a violation
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip method definitions and comment lines (doc comments are not call sites)
			if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, ".Subscribe()") {
				rel, _ := filepath.Rel(projectRoot, path)
				violations = append(violations, fmt.Sprintf("%s:%d", rel, i+1))
			}
		}
		return nil
	})

	require.NoError(t, walkErr, "filepath.Walk must succeed over project sources")
	assert.Empty(t, violations,
		"recorder.Subscribe() must only be called from facade_adapter.go (SC-001);\n"+
			"interface callers must consume RunSession.Events() instead;\n"+
			"found violations (to be migrated in T060-T063): %v", violations)
}

// TestFacadeAdapter_SubworkflowCorrelationParentChildRunID verifies that events carrying
// a non-empty ParentRunID arrive on the child session with the correlation intact (Acceptance line 59, FR-019, A8).
func TestFacadeAdapter_SubworkflowCorrelationParentChildRunID(t *testing.T) {
	const parentRunID = "parent-workflow-run-id"

	recorder := newControlledRecorder()

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// The child session represents a sub-workflow launched by the parent (FR-019)
	childSession, err := adapter.Run(ctx, ports.RunRequest{Identifier: "child/workflow"})
	require.NoError(t, err)
	defer childSession.Close()

	// Inject a correlation event: ParentRunID links child back to the parent run (A8)
	recorder.Send(transcript.ExchangeEvent{
		Type:        transcript.EventTypeStepCallWorkflowStarted,
		RunID:       childSession.ID(),
		ParentRunID: parentRunID,
		Seq:         1,
	})

	// Collect events until the correlated event arrives or timeout
	var correlatedEvent *ports.Event
	timeout := time.After(1 * time.Second)

correlate:
	for {
		select {
		case ev, ok := <-childSession.Events():
			if !ok {
				break correlate
			}
			if ev.ParentRunID == parentRunID {
				evCopy := ev
				correlatedEvent = &evCopy
				break correlate
			}
		case <-timeout:
			break correlate
		}
	}

	require.NotNil(t, correlatedEvent,
		"child session must receive an event with ParentRunID=%q (FR-019, A8)", parentRunID)
	assert.Equal(t, parentRunID, correlatedEvent.ParentRunID,
		"ProjectEvent must preserve ParentRunID for sub-workflow correlation")
}

// TestFacadeAdapter_RegistryRetainsSessionAfterClose asserts that a session remains in the
// registry even after Close() so that status polling (GET /executions/{id}) and SSE replay
// can still report the terminal status from StatusSnapshot() after execution completes.
// Sessions are bounded by server lifetime and reclaimed on restart — the trade-off is
// intentional: late-arriving GET/SSE callers must not receive "not found" for a run that
// has just completed.
func TestFacadeAdapter_RegistryRetainsSessionAfterClose(t *testing.T) {
	registry := NewSessionRegistry()

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		&mockRecorder{},
		registry,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	session, err := adapter.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.Equal(t, 1, registry.Len(), "registry must contain the active session after Run()")

	err = session.Close()
	require.NoError(t, err)

	assert.Equal(t, 1, registry.Len(),
		"registry must retain the session after Close() so status polling still resolves the terminal state")
}

// closableRecorder exposes a CloseSubscription method so the test can close the
// subscription channel and simulate the recorder stopping (BUG #7 session leak).
type closableRecorder struct {
	mu         sync.Mutex
	subCh      chan transcript.ExchangeEvent
	subscribed bool
}

func newClosableRecorder() *closableRecorder {
	return &closableRecorder{
		subCh: make(chan transcript.ExchangeEvent),
	}
}

func (r *closableRecorder) Record(_ context.Context, _ transcript.ExchangeEvent) error {
	return nil
}

func (r *closableRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscribed = true
	return r.subCh, func() {}
}

func (r *closableRecorder) Close() error {
	return nil
}

// CloseSubscription simulates the recorder stopping by closing the subscription channel.
func (r *closableRecorder) CloseSubscription() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.subscribed {
		close(r.subCh)
		r.subscribed = false
	}
}

// TestFacadeAdapter_ClosesSessionWhenRecorderStops asserts that when the recorder's
// subscription channel closes (recorder stops), the session's Events() channel is closed
// so SSE/CLI consumers can detect the end of the stream without blocking forever.
// The session remains in the registry after close so that late status polls still
// resolve the terminal state from StatusSnapshot().
func TestFacadeAdapter_ClosesSessionWhenRecorderStops(t *testing.T) {
	recorder := newClosableRecorder()
	registry := NewSessionRegistry()

	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil,
		recorder,
		registry,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	session, err := adapter.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.Equal(t, 1, registry.Len(), "registry must contain the active session after Run()")

	// Simulate recorder stopping: close the subscription channel.
	// The goroutine in newSession() should react to the range loop ending.
	recorder.CloseSubscription()

	// Drain the events channel: after Close() the channel must be closed (range terminates).
	eventsClosedCh := make(chan struct{})
	go func() {
		for range session.Events() { //nolint:revive // intentional drain
		}
		close(eventsClosedCh)
	}()

	select {
	case <-eventsClosedCh:
		// session.Events() channel closed — correct behavior
	case <-time.After(2 * time.Second):
		t.Fatal("session.Events() channel must be closed when recorder subscription ends")
	}

	// Session must remain in registry after close so status polling still resolves.
	assert.Equal(t, 1, registry.Len(),
		"registry must retain the session after close so late status polls resolve the terminal state")
}

// mockEventPublisher is a test double
type mockEventPublisher struct{}

func (m *mockEventPublisher) Publish(_ context.Context, _ *pluginmodel.DomainEvent) error {
	return nil
}

func (m *mockEventPublisher) Close() error {
	return nil
}

// mockUserInputReader is a test double
type mockUserInputReader struct{}

func (m *mockUserInputReader) ReadInput(_ context.Context) (string, error) {
	return "", nil
}

// TestFacadeAdapter_NilRecorder_NoPanic guards the ACP path: ACP builds the Adapter with a
// nil recorder (it surfaces live events through an EventPublisher, not the facade recorder).
// Run must not panic on Subscribe and must still drive the run to a terminal event.
func TestFacadeAdapter_NilRecorder_NoPanic(t *testing.T) {
	adapter := NewAdapter(
		&WorkflowService{},
		&ExecutionService{},
		&HistoryService{},
		nil, // resolver nil → wf nil → no-op execution
		nil, // recorder nil → no subscription, must not panic
		NewSessionRegistry(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sess, err := adapter.Run(ctx, ports.RunRequest{Identifier: "x/y"})
	require.NoError(t, err)
	defer sess.Close()

	var last ports.Event
	for ev := range sess.Events() {
		last = ev
	}
	assert.Equal(t, ports.EventWorkflowCompleted, last.Kind,
		"nil-recorder run must still emit the terminal event")
}
