package workflow_test

import (
	"testing"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestExecutionStatusString(t *testing.T) {
	statuses := []struct {
		status workflow.ExecutionStatus
		want   string
	}{
		{workflow.StatusPending, "pending"},
		{workflow.StatusRunning, "running"},
		{workflow.StatusCompleted, "completed"},
		{workflow.StatusFailed, "failed"},
		{workflow.StatusCancelled, "cancelled"},
	}

	for _, tt := range statuses {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ExecutionStatus.String() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNewExecutionContext(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-workflow-123", "analyze-code")

	if ctx.WorkflowID != "test-workflow-123" {
		t.Errorf("expected WorkflowID 'test-workflow-123', got '%s'", ctx.WorkflowID)
	}
	if ctx.WorkflowName != "analyze-code" {
		t.Errorf("expected WorkflowName 'analyze-code', got '%s'", ctx.WorkflowName)
	}
	if ctx.Status != workflow.StatusPending {
		t.Errorf("expected Status 'pending', got '%s'", ctx.Status)
	}
	if ctx.Inputs == nil {
		t.Error("expected Inputs to be initialized")
	}
	if ctx.States == nil {
		t.Error("expected States to be initialized")
	}
	if ctx.Env == nil {
		t.Error("expected Env to be initialized")
	}
}

func TestExecutionContextSetInput(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")

	ctx.SetInput("file_path", "/tmp/test.py")
	ctx.SetInput("count", 42)

	val, ok := ctx.GetInput("file_path")
	if !ok {
		t.Error("expected input 'file_path' to exist")
	}
	if val != "/tmp/test.py" {
		t.Errorf("expected '/tmp/test.py', got '%v'", val)
	}

	valInt, ok := ctx.GetInput("count")
	if !ok {
		t.Error("expected input 'count' to exist")
	}
	if valInt != 42 {
		t.Errorf("expected 42, got '%v'", valInt)
	}

	_, ok = ctx.GetInput("nonexistent")
	if ok {
		t.Error("expected nonexistent input to not exist")
	}
}

func TestExecutionContextStepState(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")

	state := workflow.StepState{
		Name:      "validate",
		Status:    workflow.StatusCompleted,
		Output:    "valid",
		ExitCode:  0,
		Attempt:   1,
		StartedAt: time.Now().Add(-time.Second),
	}
	state.CompletedAt = time.Now()

	ctx.SetStepState("validate", state)

	retrieved, ok := ctx.GetStepState("validate")
	if !ok {
		t.Error("expected step state 'validate' to exist")
	}
	if retrieved.Output != "valid" {
		t.Errorf("expected output 'valid', got '%s'", retrieved.Output)
	}
	if retrieved.Status != workflow.StatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", retrieved.Status)
	}
	if retrieved.Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", retrieved.Attempt)
	}

	_, ok = ctx.GetStepState("nonexistent")
	if ok {
		t.Error("expected nonexistent step state to not exist")
	}
}

func TestExecutionContextUpdateTimestamp(t *testing.T) {
	ctx := workflow.NewExecutionContext("id", "name")
	initialUpdate := ctx.UpdatedAt

	time.Sleep(time.Millisecond)
	ctx.SetInput("key", "value")

	if !ctx.UpdatedAt.After(initialUpdate) {
		t.Error("expected UpdatedAt to be updated after SetInput")
	}
}

func TestStepStateFields(t *testing.T) {
	state := workflow.StepState{
		Name:        "test",
		Status:      workflow.StatusFailed,
		Output:      "stdout content",
		Stderr:      "error output",
		ExitCode:    1,
		Attempt:     3,
		Error:       "command failed",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	if state.Name != "test" {
		t.Errorf("expected Name 'test', got '%s'", state.Name)
	}
	if state.Stderr != "error output" {
		t.Errorf("expected Stderr 'error output', got '%s'", state.Stderr)
	}
	if state.Error != "command failed" {
		t.Errorf("expected Error 'command failed', got '%s'", state.Error)
	}
}
