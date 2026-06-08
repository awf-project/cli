package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// fakeRecorder captures recorded events for testing.
type fakeRecorder struct {
	events []transcript.ExchangeEvent
	err    error // if set, Record returns this error
}

func (f *fakeRecorder) Record(_ context.Context, event transcript.ExchangeEvent) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, event)
	return nil
}

func (f *fakeRecorder) Subscribe() (<-chan transcript.ExchangeEvent, func()) {
	ch := make(chan transcript.ExchangeEvent)
	return ch, func() { close(ch) }
}

func (f *fakeRecorder) Close() error {
	return nil
}

// TestExecutionService_SetRecorder_StoresRecorder verifies SetRecorder stores the recorder.
func TestExecutionService_SetRecorder_StoresRecorder(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}

	svc.SetRecorder(recorder)

	// Verify the recorder was stored by attempting to use it
	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-verify", "test-workflow")
	step := &workflow.Step{
		Name: "verify-step",
		Type: "command",
	}

	// Call emitter which will use the stored recorder
	svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)

	// Verify event was recorded (only happens if recorder was stored)
	assert.Len(t, recorder.events, 1)
}

// TestExecutionService_SetRecorder_AllowsNil verifies nil recorder is accepted without validation.
func TestExecutionService_SetRecorder_AllowsNil(t *testing.T) {
	svc := newTestExecutionService()

	// Should not panic or error
	svc.SetRecorder(nil)

	assert.NotNil(t, svc)
}

// TestExecutionService_NilRecorderIsNoOp verifies minimal allocations when recorder is nil.
func TestExecutionService_NilRecorderIsNoOp(t *testing.T) {
	svc := newTestExecutionService()
	svc.SetRecorder(nil)

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-alloc", "test-workflow")
	step := &workflow.Step{
		Name: "alloc-step",
		Type: "command",
	}

	// Measure baseline (no allocations expected with nil recorder — just a nil check and return)
	allocsWithNil := uint64(testing.AllocsPerRun(100, func() {
		svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
		svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)
	}))

	// With nil recorder, emitters should do nothing (only check s.recorder == nil, then return)
	// Zero allocations expected on hot path per NFR-003
	assert.Equal(t, uint64(0), allocsWithNil)
}

// TestExecutionService_BuildTranscriptPath_SingleStep verifies path construction for a single step.
func TestExecutionService_BuildTranscriptPath_SingleStep(t *testing.T) {
	execCtx := workflow.NewExecutionContext("wf-123", "test-workflow")
	step := &workflow.Step{
		Name: "my-step",
		Type: "command",
	}

	// buildTranscriptPath is called internally; this test verifies the path format
	// when no loop context exists
	path := buildTranscriptPathHelper(execCtx, step)

	assert.Equal(t, "my-step", path)
}

// TestExecutionService_BuildTranscriptPath_WithLoop verifies path construction with loop context.
func TestExecutionService_BuildTranscriptPath_WithLoop(t *testing.T) {
	execCtx := workflow.NewExecutionContext("wf-123", "test-workflow")
	execCtx.CurrentLoop = &workflow.LoopContext{
		Index: 2,
		Item:  "item-2",
	}
	step := &workflow.Step{
		Name: "loop-step",
		Type: "command",
	}

	path := buildTranscriptPathHelper(execCtx, step)

	// Should include loop index
	assert.Contains(t, path, "loop-step")
	assert.Contains(t, path, "2")
}

// TestExecutionService_BuildTranscriptPath_NestedLoops verifies path construction with nested loops.
func TestExecutionService_BuildTranscriptPath_NestedLoops(t *testing.T) {
	execCtx := workflow.NewExecutionContext("wf-123", "test-workflow")

	// Create nested loop structure: parent loop with index 1, child loop with index 3
	parentLoop := &workflow.LoopContext{
		Index: 1,
		Item:  "parent-item",
	}
	childLoop := &workflow.LoopContext{
		Index:  3,
		Item:   "child-item",
		Parent: parentLoop,
	}
	execCtx.CurrentLoop = childLoop

	step := &workflow.Step{
		Name: "nested-step",
		Type: "command",
	}

	path := buildTranscriptPathHelper(execCtx, step)

	// Should walk parent chain and include both indices
	assert.Contains(t, path, "nested-step")
	assert.Contains(t, path, "1")
	assert.Contains(t, path, "3")
}

// TestExecutionService_EmitTranscriptStepStarted_ConstructsEvent verifies step.started event structure.
func TestExecutionService_EmitTranscriptStepStarted_ConstructsEvent(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}
	svc.SetRecorder(recorder)

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-456", "test-workflow")
	step := &workflow.Step{
		Name: "my-step",
		Type: "agent",
	}

	emitTranscriptStepStartedHelper(svc, ctx, execCtx, step)

	// Verify event was recorded
	require.Len(t, recorder.events, 1)
	event := recorder.events[0]

	assert.Equal(t, transcript.EventTypeStepStarted, event.Type)
	assert.Equal(t, "wf-456", event.RunID)
	assert.Equal(t, "my-step", event.Path)
	assert.NotZero(t, event.Timestamp)

	// Verify payload contains step name and kind
	payload, ok := event.Payload.(*transcript.StepPayload)
	require.True(t, ok)
	assert.Equal(t, "my-step", payload.Name)
	assert.Equal(t, "agent", payload.Kind)
}

// TestExecutionService_EmitTranscriptStepCompleted_ConstructsEvent verifies step.completed event.
func TestExecutionService_EmitTranscriptStepCompleted_ConstructsEvent(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}
	svc.SetRecorder(recorder)

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-789", "test-workflow")
	step := &workflow.Step{
		Name: "finished-step",
		Type: "command",
	}

	emitTranscriptStepCompletedHelper(svc, ctx, execCtx, step)

	// Verify event was recorded
	require.Len(t, recorder.events, 1)
	event := recorder.events[0]

	assert.Equal(t, transcript.EventTypeStepCompleted, event.Type)
	assert.Equal(t, "wf-789", event.RunID)
}

// TestExecutionService_EmitTranscriptAgentMessage_CarriesPromptAndSystemPrompt verifies message.user event.
func TestExecutionService_EmitTranscriptAgentMessage_CarriesPromptAndSystemPrompt(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}
	svc.SetRecorder(recorder)

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-abc", "test-workflow")

	userPrompt := "Generate a summary"
	systemPrompt := "You are a helpful assistant"

	emitTranscriptAgentMessageHelper(svc, ctx, execCtx, userPrompt, systemPrompt)

	// Verify message.user event
	require.Len(t, recorder.events, 1)
	event := recorder.events[0]

	assert.Equal(t, transcript.EventTypeMessageUser, event.Type)
	assert.Equal(t, "wf-abc", event.RunID)

	// Verify MessagePayload with both prompt and system_prompt as blocks
	payload, ok := event.Payload.(*transcript.MessagePayload)
	require.True(t, ok)
	assert.Equal(t, "user", payload.Role)
	require.Len(t, payload.Blocks, 2)

	// First block: user prompt
	assert.Equal(t, transcript.BlockTypeText, payload.Blocks[0].Type)
	assert.Equal(t, userPrompt, payload.Blocks[0].Text)

	// Second block: system prompt
	assert.Equal(t, transcript.BlockTypeText, payload.Blocks[1].Type)
	assert.Equal(t, systemPrompt, payload.Blocks[1].Text)
}

// TestExecutionService_EmitTranscriptStep_PropagatesParentRunID verifies P-3 / FR-007:
// events emitted within a sub-workflow run carry parent_run_id sourced from the child
// ExecutionContext, so a child transcript is navigable back to its parent. This guards the
// emitter itself (not a hand-built event), which is what was missing before.
func TestExecutionService_EmitTranscriptStep_PropagatesParentRunID(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}
	svc.SetRecorder(recorder)

	execCtx := workflow.NewExecutionContext("child-run", "child-workflow")
	execCtx.ParentRunID = "parent-run"
	step := &workflow.Step{Name: "child-step", Type: workflow.StepTypeCommand}

	emitTranscriptStepStartedHelper(svc, context.Background(), execCtx, step)

	require.Len(t, recorder.events, 1)
	ev := recorder.events[0]
	assert.Equal(t, "child-run", ev.RunID)
	assert.Equal(t, "parent-run", ev.ParentRunID, "child step event must carry parent_run_id (FR-007)")
}

// TestExecutionService_EmitTranscriptStep_NoParentRunIDForTopLevel verifies a top-level run
// (no parent) emits events with an empty parent_run_id, which omitempty drops from JSON.
func TestExecutionService_EmitTranscriptStep_NoParentRunIDForTopLevel(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}
	svc.SetRecorder(recorder)

	execCtx := workflow.NewExecutionContext("top-run", "top-workflow")
	step := &workflow.Step{Name: "top-step", Type: workflow.StepTypeCommand}

	emitTranscriptStepStartedHelper(svc, context.Background(), execCtx, step)

	require.Len(t, recorder.events, 1)
	assert.Empty(t, recorder.events[0].ParentRunID)
}

// TestExecutionService_RecorderErrorLoggedNotPropagated verifies errors are logged as WARN.
func TestExecutionService_RecorderErrorLoggedNotPropagated(t *testing.T) {
	// Create logger mock to capture WARN calls
	loggedWarnings := make([]string, 0)
	logger := &fakeLogger{
		warnFunc: func(msg string, keyVals ...any) {
			loggedWarnings = append(loggedWarnings, msg)
		},
	}

	svc := &ExecutionService{
		outputLimiter: NewOutputLimiter(workflow.DefaultOutputLimits()),
		logger:        logger,
	}

	// Configure recorder to fail
	failingRecorder := &fakeRecorder{
		err: ports.ErrInvalidEvent,
	}
	svc.SetRecorder(failingRecorder)

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-jkl", "test-workflow")
	step := &workflow.Step{
		Name: "test-step",
		Type: "command",
	}

	// Emit should not panic despite recorder error
	emitTranscriptStepStartedHelper(svc, ctx, execCtx, step)

	// Should have logged a WARN
	require.Greater(t, len(loggedWarnings), 0)
	assert.Contains(t, loggedWarnings[0], "warn") // Logger should have logged a warning
}

// TestExecutionService_StepLifecycleEmitsSequence verifies step.started and step.completed are emitted.
func TestExecutionService_StepLifecycleEmitsSequence(t *testing.T) {
	svc := newTestExecutionService()
	recorder := &fakeRecorder{}
	svc.SetRecorder(recorder)

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-mno", "test-workflow")
	step := &workflow.Step{
		Name: "lifecycle-step",
		Type: "command",
	}

	// Emit step started
	emitTranscriptStepStartedHelper(svc, ctx, execCtx, step)

	// Emit step completed
	emitTranscriptStepCompletedHelper(svc, ctx, execCtx, step)

	// Verify event sequence
	require.Len(t, recorder.events, 2)
	assert.Equal(t, transcript.EventTypeStepStarted, recorder.events[0].Type)
	assert.Equal(t, transcript.EventTypeStepCompleted, recorder.events[1].Type)
}

// TestExecutionService_NilRecorderAllowsNormalFlow verifies workflow continues when recorder is nil.
func TestExecutionService_NilRecorderAllowsNormalFlow(t *testing.T) {
	svc := newTestExecutionService()
	svc.SetRecorder(nil) // Explicitly set to nil

	ctx := context.Background()
	execCtx := workflow.NewExecutionContext("wf-pqr", "test-workflow")
	step := &workflow.Step{
		Name: "normal-step",
		Type: "command",
	}

	// Should not panic or error with nil recorder
	emitTranscriptStepStartedHelper(svc, ctx, execCtx, step)
	emitTranscriptAgentMessageHelper(svc, ctx, execCtx, "prompt", "system")
	emitTranscriptStepCompletedHelper(svc, ctx, execCtx, step)

	assert.NotNil(t, svc)
}

// fakeLogger captures log messages for testing.
type fakeLogger struct {
	warnFunc  func(msg string, keyVals ...any)
	debugFunc func(msg string, keyVals ...any)
}

func (f *fakeLogger) Debug(msg string, keyVals ...any) {
	if f.debugFunc != nil {
		f.debugFunc(msg, keyVals...)
	}
}

func (f *fakeLogger) Info(msg string, keyVals ...any) {}

func (f *fakeLogger) Warn(msg string, keyVals ...any) {
	if f.warnFunc != nil {
		f.warnFunc(msg, keyVals...)
	}
}

func (f *fakeLogger) Error(msg string, keyVals ...any) {}

func (f *fakeLogger) WithContext(ctx map[string]any) ports.Logger {
	return f
}

// Helper functions that call the actual emitter methods

func buildTranscriptPathHelper(ec *workflow.ExecutionContext, step *workflow.Step) string {
	return buildTranscriptPath(ec, step)
}

func emitTranscriptStepStartedHelper(svc *ExecutionService, ctx context.Context, ec *workflow.ExecutionContext, step *workflow.Step) {
	svc.emitTranscriptStep(ctx, ec, step, transcript.EventTypeStepStarted)
}

func emitTranscriptStepCompletedHelper(svc *ExecutionService, ctx context.Context, ec *workflow.ExecutionContext, step *workflow.Step) {
	svc.emitTranscriptStep(ctx, ec, step, transcript.EventTypeStepCompleted)
}

func emitTranscriptAgentMessageHelper(svc *ExecutionService, ctx context.Context, ec *workflow.ExecutionContext, prompt, systemPrompt string) {
	svc.emitTranscriptAgentMessage(ctx, ec, prompt, systemPrompt)
}
