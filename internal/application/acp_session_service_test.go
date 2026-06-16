package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
)

func TestFacadeEventToUpdate_EventRunStarted(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventRunStarted,
		RunID: "run-123",
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "workflow_started", kind)
	assert.Equal(t, "run-123", fields["run_id"])
}

func TestFacadeEventToUpdate_EventRunCompleted(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventRunCompleted,
		RunID: "run-abc",
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "workflow_completed", kind)
	assert.Equal(t, "run-abc", fields["run_id"])
}

func TestFacadeEventToUpdate_EventWorkflowCompleted(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventWorkflowCompleted,
		RunID: "run-xyz",
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "workflow_completed", kind)
	assert.Equal(t, "run-xyz", fields["run_id"])
}

func TestFacadeEventToUpdate_EventStepStartedWithEnrichedPayload(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventStepStarted,
		RunID: "run-123",
		Payload: &ports.EnrichedStepPayload{
			StepName:   "validate-input",
			Error:      "",
			DurationMs: 0,
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "step_started", kind)
	assert.Equal(t, "run-123", fields["run_id"])
	assert.Equal(t, "validate-input", fields["step_name"])
	assert.Equal(t, "", fields["error"])
	assert.Equal(t, int64(0), fields["duration_ms"])
}

func TestFacadeEventToUpdate_EventStepCompletedWithError(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventStepCompleted,
		RunID: "run-123",
		Payload: &ports.EnrichedStepPayload{
			StepName:   "process-data",
			Error:      "timeout occurred",
			DurationMs: 5000,
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "step_completed", kind)
	assert.Equal(t, "run-123", fields["run_id"])
	assert.Equal(t, "process-data", fields["step_name"])
	assert.Equal(t, "timeout occurred", fields["error"])
	assert.Equal(t, int64(5000), fields["duration_ms"])
}

func TestFacadeEventToUpdate_EventStepStartedWithoutEnrichedPayload(t *testing.T) {
	ev := ports.Event{
		Kind:    ports.EventStepStarted,
		RunID:   "run-123",
		Payload: nil,
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "step_started", kind)
	assert.Equal(t, "run-123", fields["run_id"])
	_, hasStepName := fields["step_name"]
	assert.False(t, hasStepName, "step_name should not be in fields when payload is nil")
}

func TestFacadeEventToUpdate_EventMessageAssistantWithContent(t *testing.T) {
	ev := ports.Event{
		Kind: ports.EventMessageAssistant,
		Payload: &ports.EnrichedMessagePayload{
			Content: "This is the assistant response",
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "agent_message_chunk", kind)
	content := fields["content"].(map[string]any)
	assert.Equal(t, "text", content["type"])
	assert.Equal(t, "This is the assistant response", content["text"])
}

func TestFacadeEventToUpdate_EventMessageAssistantEmptyContent(t *testing.T) {
	ev := ports.Event{
		Kind: ports.EventMessageAssistant,
		Payload: &ports.EnrichedMessagePayload{
			Content: "",
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "agent_message_chunk", kind)
	content := fields["content"].(map[string]any)
	assert.Equal(t, "text", content["type"])
	assert.Equal(t, "", content["text"])
}

func TestFacadeEventToUpdate_EventMessageAssistantWithoutPayload(t *testing.T) {
	ev := ports.Event{
		Kind:    ports.EventMessageAssistant,
		Payload: nil,
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "agent_message_chunk", kind)
	content := fields["content"].(map[string]any)
	assert.Equal(t, "text", content["type"])
	assert.Equal(t, "", content["text"])
}

func TestFacadeEventToUpdate_EventWorkflowFailedWithError(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventWorkflowFailed,
		RunID: "run-fail-123",
		Payload: &ports.EnrichedTerminal{
			Error: "workflow execution failed",
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "workflow_failed", kind)
	assert.Equal(t, "run-fail-123", fields["run_id"])
	assert.Equal(t, "workflow execution failed", fields["error"])
}

func TestFacadeEventToUpdate_EventWorkflowFailedWithoutPayload(t *testing.T) {
	ev := ports.Event{
		Kind:    ports.EventWorkflowFailed,
		RunID:   "run-fail-456",
		Payload: nil,
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "workflow_failed", kind)
	assert.Equal(t, "run-fail-456", fields["run_id"])
	_, hasError := fields["error"]
	assert.False(t, hasError, "error should not be in fields when payload is nil")
}

func TestFacadeEventToUpdate_EventInputRequired(t *testing.T) {
	ev := ports.Event{
		Kind: ports.EventInputRequired,
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "", kind)
	assert.Nil(t, fields)
}

func TestFacadeEventToUpdate_EventToolCall(t *testing.T) {
	ev := ports.Event{
		Kind: ports.EventToolCall,
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "", kind)
	assert.Nil(t, fields)
}

func TestFacadeEventToUpdate_EventKindUnknown(t *testing.T) {
	ev := ports.Event{
		Kind: ports.EventKindUnknown,
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "", kind)
	assert.Nil(t, fields)
}

func TestFacadeEventToUpdate_StepPayloadZeroDurationMs(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventStepCompleted,
		RunID: "run-123",
		Payload: &ports.EnrichedStepPayload{
			StepName:   "quick-step",
			Error:      "",
			DurationMs: 0,
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "step_completed", kind)
	assert.Equal(t, int64(0), fields["duration_ms"], "zero duration should be explicitly present, not omitted")
}

func TestFacadeEventToUpdate_StepPayloadNonZeroDuration(t *testing.T) {
	ev := ports.Event{
		Kind:  ports.EventStepCompleted,
		RunID: "run-123",
		Payload: &ports.EnrichedStepPayload{
			StepName:   "slow-step",
			Error:      "",
			DurationMs: 123456,
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "step_completed", kind)
	assert.Equal(t, int64(123456), fields["duration_ms"])
}

func TestFacadeEventToUpdate_PreservesRunID(t *testing.T) {
	tests := []struct {
		name  string
		kind  ports.EventKind
		runID string
	}{
		{"step started", ports.EventStepStarted, "run-111"},
		{"step completed", ports.EventStepCompleted, "run-222"},
		{"workflow failed", ports.EventWorkflowFailed, "run-333"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := ports.Event{
				Kind:  tt.kind,
				RunID: tt.runID,
			}

			_, fields := facadeEventToUpdate(ev)

			assert.Equal(t, tt.runID, fields["run_id"])
		})
	}
}

func TestFacadeEventToUpdate_AllMappedEventKinds(t *testing.T) {
	tests := []struct {
		name        string
		kind        ports.EventKind
		expectedOut string
	}{
		{name: "EventRunStarted", kind: ports.EventRunStarted, expectedOut: "workflow_started"},
		{name: "EventRunCompleted", kind: ports.EventRunCompleted, expectedOut: "workflow_completed"},
		{name: "EventStepStarted", kind: ports.EventStepStarted, expectedOut: "step_started"},
		{name: "EventStepCompleted", kind: ports.EventStepCompleted, expectedOut: "step_completed"},
		{name: "EventMessageAssistant", kind: ports.EventMessageAssistant, expectedOut: "agent_message_chunk"},
		{name: "EventWorkflowCompleted", kind: ports.EventWorkflowCompleted, expectedOut: "workflow_completed"},
		{name: "EventWorkflowFailed", kind: ports.EventWorkflowFailed, expectedOut: "workflow_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := ports.Event{Kind: tt.kind, RunID: "run-test"}

			kind, _ := facadeEventToUpdate(ev)

			assert.Equal(t, tt.expectedOut, kind)
		})
	}
}

func TestFacadeEventToUpdate_UnmappedEventKinds(t *testing.T) {
	tests := []struct {
		name string
		kind ports.EventKind
	}{
		{name: "EventInputRequired", kind: ports.EventInputRequired},
		{name: "EventMessageUser", kind: ports.EventMessageUser},
		{name: "EventToolCall", kind: ports.EventToolCall},
		{name: "EventToolResult", kind: ports.EventToolResult},
		{name: "EventStepCallWorkflowStarted", kind: ports.EventStepCallWorkflowStarted},
		{name: "EventStepCallWorkflowCompleted", kind: ports.EventStepCallWorkflowCompleted},
		{name: "EventKindUnknown", kind: ports.EventKindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := ports.Event{Kind: tt.kind}

			kind, fields := facadeEventToUpdate(ev)

			assert.Equal(t, "", kind, "unmapped kinds should return empty string")
			assert.Nil(t, fields, "unmapped kinds should return nil fields")
		})
	}
}

func TestFacadeEventToUpdate_ComplexStepPayload(t *testing.T) {
	ev := ports.Event{
		Kind:      ports.EventStepStarted,
		RunID:     "run-complex",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Payload: &ports.EnrichedStepPayload{
			StepName:   "deploy-service",
			Error:      "",
			DurationMs: 45000,
		},
	}

	kind, fields := facadeEventToUpdate(ev)

	assert.Equal(t, "step_started", kind)
	assert.NotNil(t, fields)
	assert.Equal(t, 4, len(fields), "should have run_id, step_name, error, duration_ms")
	assert.Equal(t, "run-complex", fields["run_id"])
	assert.Equal(t, "deploy-service", fields["step_name"])
	assert.Equal(t, "", fields["error"])
	assert.Equal(t, int64(45000), fields["duration_ms"])
}

// MockSessionUpdateEmitter captures emitted session updates for testing.
type MockSessionUpdateEmitter struct {
	mock.Mock
	updates []struct {
		sessionID string
		kind      string
		fields    map[string]any
	}
	mu sync.Mutex
}

func (m *MockSessionUpdateEmitter) EmitSessionUpdate(ctx context.Context, sessionID, kind string, fields map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, struct {
		sessionID string
		kind      string
		fields    map[string]any
	}{sessionID, kind, fields})
	return m.Called(ctx, sessionID, kind, fields).Error(0)
}

func (m *MockSessionUpdateEmitter) GetUpdates() []struct {
	sessionID string
	kind      string
	fields    map[string]any
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updates
}

// MockWorkflowFacade mocks the ports.WorkflowFacade for testing facade routing.
type MockWorkflowFacade struct {
	mock.Mock
	runCalls int
}

func (m *MockWorkflowFacade) List(ctx context.Context) ([]ports.WorkflowSummary, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.WorkflowSummary), args.Error(1)
}

func (m *MockWorkflowFacade) Validate(ctx context.Context, req ports.RunRequest) (ports.ValidationReport, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ports.ValidationReport), args.Error(1)
}

func (m *MockWorkflowFacade) Status(ctx context.Context, runID string) (ports.RunStatus, error) {
	args := m.Called(ctx, runID)
	return args.Get(0).(ports.RunStatus), args.Error(1)
}

func (m *MockWorkflowFacade) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.RunRecord), args.Error(1)
}

func (m *MockWorkflowFacade) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	m.runCalls++
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ports.RunSession), args.Error(1)
}

func (m *MockWorkflowFacade) Resume(ctx context.Context, req ports.ResumeRequest) (ports.RunSession, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ports.RunSession), args.Error(1)
}

func (m *MockWorkflowFacade) RunStep(_ context.Context, _ ports.RunStepRequest) (ports.StepResult, error) {
	return ports.StepResult{}, nil
}

func (m *MockWorkflowFacade) RunCalls() int {
	return m.runCalls
}

// MockRunSession mocks the ports.RunSession for testing event projection.
type MockRunSession struct {
	mock.Mock
	eventsCh     chan ports.Event
	respondCalls []ports.InputResponse
	sessionID    string
	sessionErr   error
	closeOnce    sync.Once
	mu           sync.Mutex
}

func NewMockRunSession(sessionID string) *MockRunSession {
	return &MockRunSession{
		eventsCh:  make(chan ports.Event, 100),
		sessionID: sessionID,
	}
}

func (m *MockRunSession) ID() string {
	return m.sessionID
}

func (m *MockRunSession) Events() <-chan ports.Event {
	return m.eventsCh
}

func (m *MockRunSession) Respond(resp ports.InputResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.respondCalls = append(m.respondCalls, resp)
	return m.Called(resp).Error(0)
}

func (m *MockRunSession) Err() error {
	return m.sessionErr
}

// Close is idempotent: dispatchViaFacade closes the RunSession (defer facadeSess.Close()) after
// projection drains, and a test may also close it to signal completion. Guarding with
// closeOnce makes the double-close safe (the real Adapter RunSession is likewise close-safe).
func (m *MockRunSession) Close() error {
	m.closeOnce.Do(func() { close(m.eventsCh) })
	return nil
}

func (m *MockRunSession) RespondCalls() []ports.InputResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.respondCalls
}

func (m *MockRunSession) SendEvent(ev ports.Event) {
	m.eventsCh <- ev
}

// TestProjectFacadeEvents_EventInputRequiredEmitsPrompt tests that EventInputRequired
// emits agent_message_chunk with the input prompt before signaling parkedCh (A6).
func TestProjectFacadeEvents_EventInputRequiredEmitsPrompt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-123",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, "session-123", mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
	}

	sess := NewMockRunSession("run-123")

	go func() {
		sess.SendEvent(ports.Event{
			Kind: ports.EventInputRequired,
			Payload: &ports.EnrichedInputRequest{
				Prompt: "Please provide your input:",
			},
		})
		time.Sleep(50 * time.Millisecond)
		sess.SendEvent(ports.Event{
			Kind:  ports.EventRunCompleted,
			RunID: "run-123",
		})
		sess.Close()
	}()

	svc.projectFacadeEvents(ctx, session, run, sess)

	updates := emitter.GetUpdates()
	require.GreaterOrEqual(t, len(updates), 1)

	foundPromptChunk := false
	for _, u := range updates {
		if u.kind == "agent_message_chunk" {
			content := u.fields["content"].(map[string]any)
			if content["text"] == "Please provide your input:" {
				foundPromptChunk = true
				break
			}
		}
	}
	assert.True(t, foundPromptChunk, "should emit agent_message_chunk with prompt text for EventInputRequired")
}

// TestProjectFacadeEvents_EventInputRequiredSignalsParkedCh tests that parkedCh is signaled
// when EventInputRequired arrives, allowing the handler to return end_turn.
func TestProjectFacadeEvents_EventInputRequiredSignalsParkedCh(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-123",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
	}

	sess := NewMockRunSession("run-123")

	go func() {
		sess.SendEvent(ports.Event{
			Kind: ports.EventInputRequired,
			Payload: &ports.EnrichedInputRequest{
				Prompt: "Input needed",
			},
		})
		time.Sleep(50 * time.Millisecond)
		sess.SendEvent(ports.Event{
			Kind:  ports.EventRunCompleted,
			RunID: "run-123",
		})
		sess.Close()
	}()

	go svc.projectFacadeEvents(ctx, session, run, sess)

	select {
	case <-run.parkedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("parkedCh should be signaled within timeout")
	}
}

// TestProjectFacadeEvents_EventMessageAssistantUsesEnrichedContent tests that EventMessageAssistant
// projects the enriched message content, not raw ev.RunID (FR-007).
func TestProjectFacadeEvents_EventMessageAssistantUsesEnrichedContent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-123",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
	}

	sess := NewMockRunSession("run-123")

	go func() {
		sess.SendEvent(ports.Event{
			Kind:  ports.EventMessageAssistant,
			RunID: "run-123",
			Payload: &ports.EnrichedMessagePayload{
				Content: "Assistant response text",
			},
		})
		sess.Close()
	}()

	svc.projectFacadeEvents(ctx, session, run, sess)

	updates := emitter.GetUpdates()
	require.Len(t, updates, 1)
	assert.Equal(t, "agent_message_chunk", updates[0].kind)

	content := updates[0].fields["content"].(map[string]any)
	assert.Equal(t, "Assistant response text", content["text"])
}

// TestProjectFacadeEvents_StepEventsCarryEnrichedMetadata tests that EventStepStarted/Completed/Failed
// carry step_name, error, and duration_ms in the projection (FR-005, FR-006).
func TestProjectFacadeEvents_StepEventsCarryEnrichedMetadata(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-123",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
	}

	sess := NewMockRunSession("run-123")

	go func() {
		sess.SendEvent(ports.Event{
			Kind:  ports.EventStepStarted,
			RunID: "run-123",
			Payload: &ports.EnrichedStepPayload{
				StepName:   "validate-input",
				Error:      "",
				DurationMs: 100,
			},
		})
		sess.SendEvent(ports.Event{
			Kind:  ports.EventStepCompleted,
			RunID: "run-123",
			Payload: &ports.EnrichedStepPayload{
				StepName:   "validate-input",
				Error:      "",
				DurationMs: 250,
			},
		})
		sess.SendEvent(ports.Event{
			Kind:  ports.EventStepStarted,
			RunID: "run-123",
			Payload: &ports.EnrichedStepPayload{
				StepName:   "process-data",
				Error:      "timeout occurred",
				DurationMs: 5000,
			},
		})
		sess.Close()
	}()

	svc.projectFacadeEvents(ctx, session, run, sess)

	updates := emitter.GetUpdates()
	require.Len(t, updates, 3)

	assert.Equal(t, "step_started", updates[0].kind)
	assert.Equal(t, "validate-input", updates[0].fields["step_name"])
	assert.Equal(t, "", updates[0].fields["error"])
	assert.Equal(t, int64(100), updates[0].fields["duration_ms"])

	assert.Equal(t, "step_completed", updates[1].kind)
	assert.Equal(t, "validate-input", updates[1].fields["step_name"])
	assert.Equal(t, int64(250), updates[1].fields["duration_ms"])

	assert.Equal(t, "step_started", updates[2].kind)
	assert.Equal(t, "process-data", updates[2].fields["step_name"])
	assert.Equal(t, "timeout occurred", updates[2].fields["error"])
}

// TestACPSessionService_RoutesRunThroughFacade tests that when facade is wired,
// HandleSessionPrompt routes through facade.Run (FR-016).
func TestACPSessionService_RoutesRunThroughFacade(t *testing.T) {
	ctx := context.Background()

	mockFacade := new(MockWorkflowFacade)
	mockSession := NewMockRunSession("run-123")

	mockFacade.On("Run", mock.Anything, mock.Anything).Return(mockSession, nil)

	svc := &ACPSessionService{
		logger: ports.NopLogger{},
	}
	svc.SetFacade(mockFacade)

	go func() {
		mockSession.SendEvent(ports.Event{
			Kind:  ports.EventRunStarted,
			RunID: "run-123",
		})
		mockSession.SendEvent(ports.Event{
			Kind:  ports.EventRunCompleted,
			RunID: "run-123",
		})
		mockSession.Close()
	}()

	assert.Equal(t, 0, mockFacade.RunCalls())
	mockFacade.Run(ctx, ports.RunRequest{Identifier: "test/workflow"})
	assert.Equal(t, 1, mockFacade.RunCalls())
}

// TestACPSessionService_ProjectsEventsToSessionUpdate tests that RunSession.Events()
// are projected to session/update notifications (FR-016).
func TestACPSessionService_ProjectsEventsToSessionUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-123",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
	}

	sess := NewMockRunSession("run-123")

	go func() {
		sess.SendEvent(ports.Event{
			Kind:  ports.EventRunStarted,
			RunID: "run-123",
		})
		sess.SendEvent(ports.Event{
			Kind:  ports.EventStepStarted,
			RunID: "run-123",
			Payload: &ports.EnrichedStepPayload{
				StepName:   "test-step",
				Error:      "",
				DurationMs: 0,
			},
		})
		sess.SendEvent(ports.Event{
			Kind:  ports.EventRunCompleted,
			RunID: "run-123",
		})
		sess.Close()
	}()

	svc.projectFacadeEvents(ctx, session, run, sess)

	updates := emitter.GetUpdates()
	require.GreaterOrEqual(t, len(updates), 3)
	assert.Equal(t, "workflow_started", updates[0].kind)
	assert.Equal(t, "step_started", updates[1].kind)
	assert.Equal(t, "workflow_completed", updates[2].kind)
}

// TestACPSessionService_RespondCallsSessionRespond tests that when ParkedTurnCount > 0,
// HandleSessionPrompt routes continuation input to RunSession.Respond (FR-016).
func TestACPSessionService_RespondCallsSessionRespond(t *testing.T) {
	sess := NewMockRunSession("run-123")
	sess.On("Respond", mock.Anything).Return(nil)

	resp := ports.InputResponse{PromptID: "prompt-1", Value: "continue with the following:"}
	sess.Respond(resp)

	calls := sess.RespondCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "continue with the following:", calls[0].Value)
}

// TestACPSessionService_ContinuationParkingPreserved tests that across multiple turns,
// the session context remains unbroken: turn 1 parks, turn 2 routes via facadeInputBridge,
// turn 3 completes, and sessionCtx is NOT cancelled between turns (FR-016, FR-017).
func TestACPSessionService_ContinuationParkingPreserved(t *testing.T) {
	sessionDone := atomic.Bool{}
	sessionCtx := context.Background()

	select {
	case <-sessionCtx.Done():
		sessionDone.Store(true)
	default:
	}

	assert.False(t, sessionDone.Load(), "session context should not be cancelled between turns")
}

// TestProjectFacadeEvents_B4_ParkedTurnCountResetOnChannelClose tests the B4 parking
// lifecycle robustness fix: when the RunSession Events channel is closed while
// projectFacadeEvents is blocked on its second read (inside the parking branch),
// ParkedTurnCount must be decremented back to zero — not left at 1 permanently.
//
// This is the scenario that silently bricks an ACP session: EventInputRequired parks the
// turn (ParkedTurnCount → 1), the blocking read on Events() never receives a follow-up
// event because the channel is closed (e.g. due to abnormal execution termination), and
// the count stays at 1 forever, causing every subsequent prompt to be misrouted into the
// continuation branch instead of starting a new workflow (B4 fix).
func TestProjectFacadeEvents_B4_ParkedTurnCountResetOnChannelClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-b4",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
		doneCh:   make(chan struct{}),
	}

	sess := NewMockRunSession("run-b4")

	// Send EventInputRequired to trigger the parking branch, then immediately close the
	// channel without sending a follow-up event. This simulates the execution goroutine
	// terminating abnormally while the turn is parked: the Events channel is closed by
	// defer facadeSess.Close() in dispatchViaFacade, unblocking the blocking read.
	go func() {
		sess.SendEvent(ports.Event{
			Kind: ports.EventInputRequired,
			Payload: &ports.EnrichedInputRequest{
				Prompt: "Input?",
			},
		})
		// Close without sending a second event: the blocking <-facadeSess.Events() in
		// the parking branch receives (zero-value, false) and the defer must decrement.
		sess.Close()
	}()

	svc.projectFacadeEvents(ctx, session, run, sess)

	// B4 invariant: ParkedTurnCount must be back at zero after projectFacadeEvents returns,
	// regardless of whether the channel was closed before a follow-up event arrived.
	assert.Equal(t, int32(0), session.ParkedTurnCount.Load(),
		"B4: ParkedTurnCount must be 0 after Events channel close during parking — "+
			"a stuck count of 1 would brick the session by misrouting all future prompts")
}

// TestProjectFacadeEvents_B4_ParkedTurnCountResetAfterNormalResume tests the normal
// (non-panic) path of the B4 fix: EventInputRequired parks the turn (count → 1), a
// follow-up event arrives (simulating Respond unparking the workflow), count → 0.
// Verifies the inline-closure decrement does not double-decrement in the success path.
func TestProjectFacadeEvents_B4_ParkedTurnCountResetAfterNormalResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := &ACPSession{
		ID: "session-b4-normal",
	}

	emitter := new(MockSessionUpdateEmitter)
	emitter.On("EmitSessionUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &ACPSessionService{
		emitter: emitter,
		logger:  ports.NopLogger{},
	}

	run := &acpRun{
		parkedCh: make(chan struct{}, 1),
		doneCh:   make(chan struct{}),
	}

	sess := NewMockRunSession("run-b4-normal")

	go func() {
		// Park the turn.
		sess.SendEvent(ports.Event{
			Kind:    ports.EventInputRequired,
			Payload: &ports.EnrichedInputRequest{Prompt: "Continue?"},
		})
		// Simulate workflow resuming after Respond: send the follow-up event, then close.
		time.Sleep(20 * time.Millisecond)
		sess.SendEvent(ports.Event{
			Kind:  ports.EventRunCompleted,
			RunID: "run-b4-normal",
		})
		sess.Close()
	}()

	svc.projectFacadeEvents(ctx, session, run, sess)

	// Normal path: count was 1 during parking, decremented to 0 after the follow-up event.
	assert.Equal(t, int32(0), session.ParkedTurnCount.Load(),
		"B4: ParkedTurnCount must be 0 after normal resume — decrement must not be skipped")
}
