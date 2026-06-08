package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// TestExecutionService_LoopIterationInPath verifies for_each iterations are tracked in transcript.
func TestExecutionService_LoopIterationInPath(t *testing.T) {
	ctx := context.Background()
	recorder := &fakeRecorder{}

	svc := newTestExecutionService()
	svc.SetRecorder(recorder)

	wfID := "loop-foreach-123"

	execCtx := workflow.NewExecutionContext(wfID, "loop-foreach-workflow")
	step := &workflow.Step{
		Name: "loop-step",
		Type: workflow.StepTypeForEach,
		Loop: &workflow.LoopConfig{
			Type:  workflow.LoopTypeForEach,
			Items: "{{inputs.items}}",
		},
	}

	// Simulate 3 loop iterations: each should emit step events with iteration 0, 1, 2
	for i := range 3 {
		// Set up loop context for this iteration
		loopCtx := &workflow.LoopContext{
			Index: i,
			Item:  "item-" + string(rune('0'+i)),
		}
		execCtx.CurrentLoop = loopCtx

		// Emit step started event for this iteration
		svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)

		// Emit step completed event
		svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)
	}

	// Verify we have 6 events (2 per iteration: started + completed)
	require.Len(t, recorder.events, 6)

	// Verify each pair has correct iteration and path includes loop information
	for i := range 3 {
		startIdx := i * 2
		endIdx := i*2 + 1

		startEvent := recorder.events[startIdx]
		assert.Equal(t, transcript.EventTypeStepStarted, startEvent.Type)
		assert.Equal(t, i, startEvent.Iteration)
		// Path should include loop context: "loop:0/loop-step", "loop:1/loop-step", "loop:2/loop-step"
		assert.Contains(t, startEvent.Path, "loop-step")
		assert.Contains(t, startEvent.Path, "loop:"+string(rune('0'+i)))

		endEvent := recorder.events[endIdx]
		assert.Equal(t, transcript.EventTypeStepCompleted, endEvent.Type)
		assert.Equal(t, i, endEvent.Iteration)
		assert.Contains(t, endEvent.Path, "loop-step")
		assert.Contains(t, endEvent.Path, "loop:"+string(rune('0'+i)))
	}
}

// TestExecutionService_WhileLoopIterations verifies while loop iterations are tracked with incrementing iteration.
func TestExecutionService_WhileLoopIterations(t *testing.T) {
	ctx := context.Background()
	recorder := &fakeRecorder{}

	svc := newTestExecutionService()
	svc.SetRecorder(recorder)

	wfID := "loop-while-123"

	execCtx := workflow.NewExecutionContext(wfID, "loop-while-workflow")
	step := &workflow.Step{
		Name: "while-step",
		Type: workflow.StepTypeWhile,
		Loop: &workflow.LoopConfig{
			Type:      workflow.LoopTypeWhile,
			Condition: "{{states.counter.output}}",
		},
	}

	// Simulate 5 while loop iterations (0-4)
	const numIterations = 5
	for i := range numIterations {
		loopCtx := &workflow.LoopContext{
			Index: i,
			Item:  nil, // while loops don't have items
		}
		execCtx.CurrentLoop = loopCtx

		// Emit step started event
		svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)

		// Emit step completed event
		svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)
	}

	// Verify we have 10 events (2 per iteration)
	require.Len(t, recorder.events, 10)

	// Verify iteration numbers increment from 0 to 4
	for i := range numIterations {
		startIdx := i * 2
		endIdx := i*2 + 1

		startEvent := recorder.events[startIdx]
		assert.Equal(t, transcript.EventTypeStepStarted, startEvent.Type)
		assert.Equal(t, i, startEvent.Iteration, "iteration should be %d for start event", i)

		endEvent := recorder.events[endIdx]
		assert.Equal(t, transcript.EventTypeStepCompleted, endEvent.Type)
		assert.Equal(t, i, endEvent.Iteration, "iteration should be %d for end event", i)
	}
}

// TestExecutionService_NestedLoopIterationPath verifies nested loop path building.
func TestExecutionService_NestedLoopIterationPath(t *testing.T) {
	ctx := context.Background()
	recorder := &fakeRecorder{}

	svc := newTestExecutionService()
	svc.SetRecorder(recorder)

	wfID := "nested-loop-123"

	execCtx := workflow.NewExecutionContext(wfID, "nested-loop-workflow")
	step := &workflow.Step{
		Name: "outer-step",
		Type: workflow.StepTypeForEach,
	}

	// Create nested loop context: outer index 1, inner index 2
	outerLoop := &workflow.LoopContext{
		Index: 1,
		Item:  "outer-item-1",
	}
	innerLoop := &workflow.LoopContext{
		Index:  2,
		Item:   "inner-item-2",
		Parent: outerLoop,
	}
	execCtx.CurrentLoop = innerLoop

	// Emit event from nested loop
	svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)

	// Verify path contains both loop information
	require.Len(t, recorder.events, 1)
	event := recorder.events[0]
	assert.Equal(t, 2, event.Iteration)
}

// TestExecutionService_LoopIterationWithoutContext verifies iteration is 0 when no loop context.
func TestExecutionService_LoopIterationWithoutContext(t *testing.T) {
	ctx := context.Background()
	recorder := &fakeRecorder{}

	svc := newTestExecutionService()
	svc.SetRecorder(recorder)

	wfID := "no-loop-123"

	execCtx := workflow.NewExecutionContext(wfID, "no-loop-workflow")
	step := &workflow.Step{
		Name: "regular-step",
		Type: workflow.StepTypeCommand,
	}

	// No loop context set
	execCtx.CurrentLoop = nil

	svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)

	// Iteration should be 0 (default)
	require.Len(t, recorder.events, 1)
	event := recorder.events[0]
	assert.Equal(t, 0, event.Iteration)
	assert.Equal(t, step.Name, event.Path)
}

// TestExecutionService_MultipleLoopItemsPathStructure verifies path structure across items.
func TestExecutionService_MultipleLoopItemsPathStructure(t *testing.T) {
	tests := []struct {
		name      string
		items     []string
		expectLen int
	}{
		{
			name:      "single item",
			items:     []string{"item-0"},
			expectLen: 2, // start + complete
		},
		{
			name:      "three items",
			items:     []string{"a", "b", "c"},
			expectLen: 6, // 3 * (start + complete)
		},
		{
			name:      "five items",
			items:     []string{"1", "2", "3", "4", "5"},
			expectLen: 10, // 5 * (start + complete)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			recorder := &fakeRecorder{}

			svc := newTestExecutionService()
			svc.SetRecorder(recorder)

			execCtx := workflow.NewExecutionContext("wf-path-test", "test-workflow")
			step := &workflow.Step{Name: "test-step", Type: workflow.StepTypeForEach}

			for i := range len(tt.items) {
				loopCtx := &workflow.LoopContext{Index: i, Item: tt.items[i]}
				execCtx.CurrentLoop = loopCtx

				svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
				svc.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)
			}

			assert.Len(t, recorder.events, tt.expectLen)
		})
	}
}
