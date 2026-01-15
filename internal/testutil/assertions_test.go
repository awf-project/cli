package testutil

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// TestAssertWorkflowValid_HappyPath tests validation of valid workflows
// mockT is a mock implementation of testing.TB that captures Fatalf calls
// instead of actually failing the test. This allows us to test assertion helpers.
type mockT struct {
	testing.TB
	failed       bool
	fatalMsg     string
	helperCalled bool
}

func (m *mockT) Helper() {
	m.helperCalled = true
}

func (m *mockT) Fatalf(format string, args ...any) {
	m.failed = true
	m.fatalMsg = fmt.Sprintf(format, args...)
	// Panic to stop execution like real testing.T does
	// The panic will be recovered by the test runner
	panic("test failed")
}

func (m *mockT) Failed() bool {
	return m.failed
}

func TestAssertWorkflowValid_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		workflow *workflow.Workflow
	}{
		{
			name: "minimal valid workflow",
			workflow: &workflow.Workflow{
				Name:    "test-workflow",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Type:      workflow.StepTypeCommand,
						OnSuccess: "done",
					},
					"done": {
						Type: workflow.StepTypeTerminal,
					},
				},
			},
		},
		{
			name: "workflow with multiple steps",
			workflow: &workflow.Workflow{
				Name:    "multi-step",
				Initial: "step1",
				Steps: map[string]*workflow.Step{
					"step1": {
						Type:      workflow.StepTypeCommand,
						OnSuccess: "step2",
					},
					"step2": {
						Type:      workflow.StepTypeCommand,
						OnSuccess: "done",
					},
					"done": {
						Type: workflow.StepTypeTerminal,
					},
				},
			},
		},
		{
			name: "workflow with parallel step",
			workflow: &workflow.Workflow{
				Name:    "parallel-workflow",
				Initial: "parallel",
				Steps: map[string]*workflow.Step{
					"parallel": {
						Type:      workflow.StepTypeParallel,
						Branches:  []string{"task1", "task2"},
						OnSuccess: "done",
					},
					"task1": {
						Type: workflow.StepTypeCommand,
					},
					"task2": {
						Type: workflow.StepTypeCommand,
					},
					"done": {
						Type: workflow.StepTypeTerminal,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail with "not implemented" during RED phase
			// In GREEN phase, it should not panic
			AssertWorkflowValid(t, tt.workflow)
		})
	}
}

// TestAssertWorkflowValid_InvalidWorkflows tests detection of invalid workflows
func TestAssertWorkflowValid_InvalidWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		workflow *workflow.Workflow
		wantErr  string
	}{
		{
			name: "missing name",
			workflow: &workflow.Workflow{
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: "workflow name is required",
		},
		{
			name: "missing initial step",
			workflow: &workflow.Workflow{
				Name: "test",
				Steps: map[string]*workflow.Step{
					"start": {Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: "initial step is required",
		},
		{
			name: "initial step not in steps map",
			workflow: &workflow.Workflow{
				Name:    "test",
				Initial: "nonexistent",
				Steps: map[string]*workflow.Step{
					"start": {Type: workflow.StepTypeTerminal},
				},
			},
			wantErr: "initial step 'nonexistent' not found",
		},
		{
			name: "step references invalid next",
			workflow: &workflow.Workflow{
				Name:    "test",
				Initial: "start",
				Steps: map[string]*workflow.Step{
					"start": {
						Type:      workflow.StepTypeCommand,
						OnSuccess: "nonexistent",
					},
				},
			},
			wantErr: "step 'start' references invalid next step 'nonexistent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use mockT to capture Fatalf call instead of actually failing
			mock := &mockT{TB: t}

			// Recover from panic that mockT.Fatalf throws
			defer func() {
				_ = recover() // Expected panic from mockT.Fatalf
			}()

			AssertWorkflowValid(mock, tt.workflow)

			// Verify the assertion failed with expected message
			if !mock.failed {
				t.Errorf("expected assertion to fail, but it didn't")
			}
			if !strings.Contains(mock.fatalMsg, tt.wantErr) {
				t.Errorf("expected error message to contain %q, got %q", tt.wantErr, mock.fatalMsg)
			}
		})
	}
}

// TestAssertStepOutput_HappyPath tests validation of successful step execution
func TestAssertStepOutput_HappyPath(t *testing.T) {
	tests := []struct {
		name           string
		ctx            *workflow.ExecutionContext
		stepName       string
		expectedOutput string
	}{
		{
			name: "step completed with expected output",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{
					"step1": {
						Name:   "step1",
						Status: workflow.StatusCompleted,
						Output: "success output",
					},
				},
			},
			stepName:       "step1",
			expectedOutput: "success output",
		},
		{
			name: "step completed - output not checked",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{
					"step1": {
						Name:   "step1",
						Status: workflow.StatusCompleted,
						Output: "any output",
					},
				},
			},
			stepName:       "step1",
			expectedOutput: "", // empty string means don't check output
		},
		{
			name: "step completed with empty output",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{
					"step1": {
						Name:   "step1",
						Status: workflow.StatusCompleted,
						Output: "",
					},
				},
			},
			stepName:       "step1",
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail with "not implemented" during RED phase
			// In GREEN phase, it should not panic
			AssertStepOutput(t, tt.ctx, tt.stepName, tt.expectedOutput)
		})
	}
}

// TestAssertStepOutput_Failures tests detection of step execution failures
func TestAssertStepOutput_Failures(t *testing.T) {
	tests := []struct {
		name           string
		ctx            *workflow.ExecutionContext
		stepName       string
		expectedOutput string
		wantErr        string
	}{
		{
			name: "step not found",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{},
			},
			stepName:       "missing",
			expectedOutput: "",
			wantErr:        "step 'missing' not found",
		},
		{
			name: "step still running",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{
					"step1": {
						Name:   "step1",
						Status: workflow.StatusRunning,
						Output: "",
					},
				},
			},
			stepName:       "step1",
			expectedOutput: "",
			wantErr:        "step 'step1' status is 'running', expected 'completed'",
		},
		{
			name: "step failed",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{
					"step1": {
						Name:   "step1",
						Status: workflow.StatusFailed,
						Error:  "command failed",
					},
				},
			},
			stepName:       "step1",
			expectedOutput: "",
			wantErr:        "step 'step1' status is 'failed'",
		},
		{
			name: "output mismatch",
			ctx: &workflow.ExecutionContext{
				States: map[string]workflow.StepState{
					"step1": {
						Name:   "step1",
						Status: workflow.StatusCompleted,
						Output: "actual output",
					},
				},
			},
			stepName:       "step1",
			expectedOutput: "expected output",
			wantErr:        "output mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use mockT to capture Fatalf call instead of actually failing
			mock := &mockT{TB: t}

			// Recover from panic that mockT.Fatalf throws
			defer func() {
				_ = recover() // Expected panic from mockT.Fatalf
			}()

			AssertStepOutput(mock, tt.ctx, tt.stepName, tt.expectedOutput)

			// Verify the assertion failed with expected message
			if !mock.failed {
				t.Errorf("expected assertion to fail, but it didn't")
			}
			if !strings.Contains(mock.fatalMsg, tt.wantErr) {
				t.Errorf("expected error message to contain %q, got %q", tt.wantErr, mock.fatalMsg)
			}
		})
	}
}

// TestAssertExecutionCompleted_HappyPath tests validation of completed executions
func TestAssertExecutionCompleted_HappyPath(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name              string
		ctx               *workflow.ExecutionContext
		expectedFinalStep []string
	}{
		{
			name: "execution completed - no final step check",
			ctx: &workflow.ExecutionContext{
				Status:      workflow.StatusCompleted,
				CompletedAt: now,
				CurrentStep: "done",
			},
			expectedFinalStep: nil,
		},
		{
			name: "execution completed - with final step check",
			ctx: &workflow.ExecutionContext{
				Status:      workflow.StatusCompleted,
				CompletedAt: now,
				CurrentStep: "done",
			},
			expectedFinalStep: []string{"done"},
		},
		{
			name: "execution completed - alternative final step",
			ctx: &workflow.ExecutionContext{
				Status:      workflow.StatusCompleted,
				CompletedAt: now,
				CurrentStep: "success",
			},
			expectedFinalStep: []string{"success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail with "not implemented" during RED phase
			// In GREEN phase, it should not panic
			AssertExecutionCompleted(t, tt.ctx, tt.expectedFinalStep...)
		})
	}
}

// TestAssertExecutionCompleted_Failures tests detection of incomplete executions
func TestAssertExecutionCompleted_Failures(t *testing.T) {
	tests := []struct {
		name          string
		ctx           *workflow.ExecutionContext
		finalStepName string
		wantErr       string
	}{
		{
			name: "execution still running",
			ctx: &workflow.ExecutionContext{
				Status: workflow.StatusRunning,
			},
			finalStepName: "",
			wantErr:       "execution status is 'running', expected 'completed'",
		},
		{
			name: "execution failed",
			ctx: &workflow.ExecutionContext{
				Status: workflow.StatusFailed,
			},
			finalStepName: "",
			wantErr:       "execution status is 'failed', expected 'completed'",
		},
		{
			name: "execution cancelled",
			ctx: &workflow.ExecutionContext{
				Status: workflow.StatusCancelled,
			},
			finalStepName: "",
			wantErr:       "execution status is 'cancelled', expected 'completed'",
		},
		{
			name: "completed but no completion timestamp",
			ctx: &workflow.ExecutionContext{
				Status: workflow.StatusCompleted,
				// CompletedAt is zero value
			},
			finalStepName: "",
			wantErr:       "execution completed but CompletedAt is zero",
		},
		{
			name: "final step mismatch",
			ctx: &workflow.ExecutionContext{
				Status:      workflow.StatusCompleted,
				CompletedAt: time.Now(),
				CurrentStep: "actual_step",
			},
			finalStepName: "expected_step",
			wantErr:       "current step is 'actual_step', expected 'expected_step'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use mockT to capture Fatalf call instead of actually failing
			mock := &mockT{TB: t}

			// Recover from panic that mockT.Fatalf throws
			defer func() {
				_ = recover() // Expected panic from mockT.Fatalf
			}()

			AssertExecutionCompleted(mock, tt.ctx, tt.finalStepName)

			// Verify the assertion failed with expected message
			if !mock.failed {
				t.Errorf("expected assertion to fail, but it didn't")
			}
			if !strings.Contains(mock.fatalMsg, tt.wantErr) {
				t.Errorf("expected error message to contain %q, got %q", tt.wantErr, mock.fatalMsg)
			}
		})
	}
}
