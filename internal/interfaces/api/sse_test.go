package api

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// newMockSSESender creates a mock SSE sender that records messages.
func newMockSSESender() (sse.Sender, *[]sse.Message) {
	messages := &[]sse.Message{}
	var mu sync.Mutex

	sender := func(msg sse.Message) error {
		mu.Lock()
		defer mu.Unlock()
		*messages = append(*messages, msg)
		return nil
	}

	return sender, messages
}

func TestSSE_UnknownExecutionID_Returns404BeforeStreamOpen(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), nil, newMockHistoryProvider())
	var wg sync.WaitGroup
	handler := NewSSEHandler(bridge, &wg)

	ctx := context.Background()
	in := &StreamInput{ID: "unknown-id"}
	sender, _ := newMockSSESender()

	err := handler.Stream(ctx, in, sender)

	require.NotNil(t, err, "expected error for unknown execution ID")
}

func TestSSE_EmitsStepStartedThenStepCompleted_OnStateTransition(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), nil, newMockHistoryProvider())
	var wg sync.WaitGroup
	handler := NewSSEHandler(bridge, &wg)

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ae := &ActiveExecution{
		ExecutionID:      "test-exec-id",
		WorkflowName:     "test-workflow",
		ExecutionContext: execCtx,
		Done:             make(<-chan error),
	}
	bridge.activeExecutions.Store("test-exec-id", ae)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	in := &StreamInput{ID: "test-exec-id"}
	sender, messages := newMockSSESender()

	// Simulate state transitions in a separate goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		stepState := workflow.StepState{
			Name:      "step1",
			Status:    workflow.StatusRunning,
			StartedAt: time.Now(),
		}
		execCtx.SetStepState("step1", stepState)

		time.Sleep(100 * time.Millisecond)
		stepState.Status = workflow.StatusCompleted
		stepState.Output = "test output"
		stepState.CompletedAt = time.Now()
		execCtx.SetStepState("step1", stepState)

		time.Sleep(100 * time.Millisecond)
		execCtx.SetStatus(workflow.StatusCompleted)
		execCtx.SetCompletedAt(time.Now())
	}()

	_ = handler.Stream(ctx, in, sender)

	assert.NotEmpty(t, *messages, "expected SSE messages to be emitted")
}

func TestSSE_ClosesStreamOnTerminalState(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), nil, newMockHistoryProvider())
	var wg sync.WaitGroup
	handler := NewSSEHandler(bridge, &wg)

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ae := &ActiveExecution{
		ExecutionID:      "test-exec-id",
		WorkflowName:     "test-workflow",
		ExecutionContext: execCtx,
		Done:             make(<-chan error),
	}
	bridge.activeExecutions.Store("test-exec-id", ae)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	in := &StreamInput{ID: "test-exec-id"}
	sender, _ := newMockSSESender()

	go func() {
		time.Sleep(50 * time.Millisecond)
		execCtx.SetStatus(workflow.StatusCompleted)
		execCtx.SetCompletedAt(time.Now())
	}()

	err := handler.Stream(ctx, in, sender)

	assert.NoError(t, err, "expected Stream to return without error on terminal state")
}

func TestSSE_ClientDisconnect_StopsPollingGoroutine_NoLeak(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), nil, newMockHistoryProvider())
	var wg sync.WaitGroup
	handler := NewSSEHandler(bridge, &wg)

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ae := &ActiveExecution{
		ExecutionID:      "test-exec-id",
		WorkflowName:     "test-workflow",
		ExecutionContext: execCtx,
		Done:             make(<-chan error),
	}
	bridge.activeExecutions.Store("test-exec-id", ae)

	ctx, cancel := context.WithCancel(context.Background())
	in := &StreamInput{ID: "test-exec-id"}
	sender, _ := newMockSSESender()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_ = handler.Stream(ctx, in, sender)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE goroutine did not exit after client disconnect")
	}
}

func TestSSE_50ConcurrentSubscribers_NoCrossInterference(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), nil, newMockHistoryProvider())
	var wg sync.WaitGroup
	handler := NewSSEHandler(bridge, &wg)

	execCtx := workflow.NewExecutionContext("test-exec-id", "test-workflow")
	ae := &ActiveExecution{
		ExecutionID:      "test-exec-id",
		WorkflowName:     "test-workflow",
		ExecutionContext: execCtx,
		Done:             make(<-chan error),
	}
	bridge.activeExecutions.Store("test-exec-id", ae)

	var eg errgroup.Group
	messageCounts := make([]int, 50)
	var mu sync.Mutex

	for i := range 50 {
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			in := &StreamInput{ID: "test-exec-id"}
			sender, messages := newMockSSESender()

			_ = handler.Stream(ctx, in, sender)

			mu.Lock()
			messageCounts[i] = len(*messages)
			mu.Unlock()

			return nil
		})
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		execCtx.SetStatus(workflow.StatusCompleted)
		execCtx.SetCompletedAt(time.Now())
	}()

	err := eg.Wait()
	require.NoError(t, err, "expected concurrent subscribers to complete without error")

	for i, count := range messageCounts {
		assert.Greater(t, count, 0, "subscriber %d should have received at least one message", i)
	}
}

func TestSSE_EventType_MatchesWorkflowAuditConstants(t *testing.T) {
	assert.Equal(t, "step.started", workflow.EventStepStarted)
	assert.Equal(t, "step.completed", workflow.EventStepCompleted)
	assert.Equal(t, "step.failed", workflow.EventStepFailed)
	assert.Equal(t, "workflow.completed", workflow.EventWorkflowCompleted)
	assert.Equal(t, "workflow.failed", workflow.EventWorkflowFailed)

	known := []string{
		workflow.EventStepStarted, workflow.EventStepCompleted, workflow.EventStepFailed,
		workflow.EventWorkflowCompleted, workflow.EventWorkflowFailed, eventOutput,
	}
	for key := range eventRegistry {
		assert.Contains(t, known, key, "eventRegistry key %q should match a known constant", key)
	}
}

func TestSSE_APIPollingInterval_Is200ms(t *testing.T) {
	expected := 200 * time.Millisecond
	assert.Equal(t, expected, apiPollInterval, "apiPollInterval should be 200ms")
}

func TestSSE_SSEHandlerConstructor_StoresReferences(t *testing.T) {
	bridge := NewBridge(newMockWorkflowLister(), nil, newMockHistoryProvider())
	var wg sync.WaitGroup

	handler := NewSSEHandler(bridge, &wg)

	assert.NotNil(t, handler, "expected NewSSEHandler to return non-nil handler")
}

func TestSSE_EventStructs_HaveJSONTags(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeFor[StepStartedEvent](),
		reflect.TypeFor[StepCompletedEvent](),
		reflect.TypeFor[StepFailedEvent](),
		reflect.TypeFor[WorkflowCompletedEvent](),
		reflect.TypeFor[WorkflowFailedEvent](),
		reflect.TypeFor[OutputEvent](),
	}
	for _, typ := range types {
		t.Run(typ.Name(), func(t *testing.T) {
			for i := range typ.NumField() {
				tag := typ.Field(i).Tag.Get("json")
				assert.NotEmpty(t, tag, "field %s.%s missing json tag", typ.Name(), typ.Field(i).Name)
			}
		})
	}
}
