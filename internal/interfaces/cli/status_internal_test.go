package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
)

// RED Phase: Test stubs for unexported status.go helper functions
// These tests will compile but fail when run - implementation validation needed

func TestToExecutionInfo(t *testing.T) {
	tests := []struct {
		name    string
		execCtx *workflow.ExecutionContext
		checks  func(t *testing.T, info ui.ExecutionInfo)
	}{
		{
			name: "completed workflow",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("exec-123", "my-workflow")
				ctx.Status = workflow.StatusCompleted
				ctx.StartedAt = time.Now().Add(-5 * time.Second)
				ctx.CompletedAt = time.Now()
				ctx.CurrentStep = ""
				ctx.States["step1"] = workflow.StepState{
					Name:        "step1",
					Status:      workflow.StatusCompleted,
					Output:      "output data",
					ExitCode:    0,
					StartedAt:   ctx.StartedAt,
					CompletedAt: ctx.CompletedAt,
				}
				return ctx
			}(),
			checks: func(t *testing.T, info ui.ExecutionInfo) {
				assert.Equal(t, "exec-123", info.WorkflowID)
				assert.Equal(t, "my-workflow", info.WorkflowName)
				assert.Equal(t, "completed", info.Status)
				assert.NotEmpty(t, info.StartedAt)
				assert.NotEmpty(t, info.CompletedAt)
				assert.Greater(t, info.DurationMs, int64(0))
				assert.Len(t, info.Steps, 1)
				assert.Equal(t, "step1", info.Steps[0].Name)
			},
		},
		{
			name: "running workflow",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("exec-456", "running-wf")
				ctx.Status = workflow.StatusRunning
				ctx.CurrentStep = "step2"
				ctx.StartedAt = time.Now().Add(-2 * time.Second)
				// CompletedAt is zero (still running)
				ctx.States["step1"] = workflow.StepState{
					Name:        "step1",
					Status:      workflow.StatusCompleted,
					StartedAt:   ctx.StartedAt,
					CompletedAt: ctx.StartedAt.Add(time.Second),
				}
				ctx.States["step2"] = workflow.StepState{
					Name:      "step2",
					Status:    workflow.StatusRunning,
					StartedAt: ctx.StartedAt.Add(time.Second),
				}
				return ctx
			}(),
			checks: func(t *testing.T, info ui.ExecutionInfo) {
				assert.Equal(t, "exec-456", info.WorkflowID)
				assert.Equal(t, "running", info.Status)
				assert.Equal(t, "step2", info.CurrentStep)
				assert.NotEmpty(t, info.StartedAt)
				assert.Empty(t, info.CompletedAt) // Still running
				assert.Greater(t, info.DurationMs, int64(0))
				assert.Len(t, info.Steps, 2)
			},
		},
		{
			name: "failed workflow with error",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("exec-789", "failed-wf")
				ctx.Status = workflow.StatusFailed
				ctx.StartedAt = time.Now().Add(-time.Second)
				ctx.CompletedAt = time.Now()
				ctx.States["step1"] = workflow.StepState{
					Name:        "step1",
					Status:      workflow.StatusFailed,
					Error:       "command failed",
					ExitCode:    1,
					StartedAt:   ctx.StartedAt,
					CompletedAt: ctx.CompletedAt,
				}
				return ctx
			}(),
			checks: func(t *testing.T, info ui.ExecutionInfo) {
				assert.Equal(t, "failed", info.Status)
				assert.Len(t, info.Steps, 1)
				assert.Equal(t, "failed", info.Steps[0].Status)
				assert.Equal(t, "command failed", info.Steps[0].Error)
				assert.Equal(t, 1, info.Steps[0].ExitCode)
			},
		},
		{
			name: "empty workflow no steps",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("exec-empty", "empty-wf")
				ctx.Status = workflow.StatusPending
				return ctx
			}(),
			checks: func(t *testing.T, info ui.ExecutionInfo) {
				assert.Equal(t, "exec-empty", info.WorkflowID)
				assert.Equal(t, "pending", info.Status)
				assert.Empty(t, info.Steps)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := toExecutionInfo(tt.execCtx)
			tt.checks(t, info)
		})
	}
}

func TestToExecutionInfo_ZeroTimes(t *testing.T) {
	// Test edge case where times are zero values
	ctx := workflow.NewExecutionContext("zero-time", "zero-wf")
	ctx.Status = workflow.StatusPending
	ctx.StartedAt = time.Time{}   // zero time
	ctx.CompletedAt = time.Time{} // zero time

	info := toExecutionInfo(ctx)

	assert.Equal(t, "zero-time", info.WorkflowID)
	assert.Empty(t, info.StartedAt)   // Should be empty for zero time
	assert.Empty(t, info.CompletedAt) // Should be empty for zero time
}

func TestToExecutionInfo_StepWithZeroTimes(t *testing.T) {
	// Test step with zero start/completed times
	ctx := workflow.NewExecutionContext("step-zero", "step-wf")
	ctx.Status = workflow.StatusRunning
	ctx.StartedAt = time.Now()
	ctx.States["pending-step"] = workflow.StepState{
		Name:   "pending-step",
		Status: workflow.StatusPending,
		// StartedAt and CompletedAt are zero
	}

	info := toExecutionInfo(ctx)

	assert.Len(t, info.Steps, 1)
	assert.Empty(t, info.Steps[0].StartedAt)
	assert.Empty(t, info.Steps[0].CompletedAt)
}

func TestDisplayStatus_NoFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	ctx := workflow.NewExecutionContext("no-fail", "no-fail-wf")
	ctx.Status = workflow.StatusCompleted
	ctx.StartedAt = time.Now().Add(-time.Second)
	ctx.CompletedAt = time.Now()
	now := time.Now()
	ctx.States["step1"] = workflow.StepState{
		Name:        "step1",
		Status:      workflow.StatusCompleted,
		StartedAt:   now,
		CompletedAt: now,
	}
	ctx.States["step2"] = workflow.StepState{
		Name:        "step2",
		Status:      workflow.StatusCompleted,
		StartedAt:   now,
		CompletedAt: now,
	}

	displayStatus(formatter, ctx, false)

	output := buf.String()
	// Progress should show 2/2 with no failed count
	assert.Contains(t, output, "2/2")
	assert.NotContains(t, output, "failed")
}

func TestDisplayStatus_WithFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	ctx := workflow.NewExecutionContext("with-fail", "fail-wf")
	ctx.Status = workflow.StatusFailed
	ctx.StartedAt = time.Now().Add(-time.Second)
	ctx.CompletedAt = time.Now()
	now := time.Now()
	ctx.States["step1"] = workflow.StepState{
		Name:        "step1",
		Status:      workflow.StatusCompleted,
		StartedAt:   now,
		CompletedAt: now,
	}
	ctx.States["step2"] = workflow.StepState{
		Name:        "step2",
		Status:      workflow.StatusFailed,
		StartedAt:   now,
		CompletedAt: now,
	}

	displayStatus(formatter, ctx, false)

	output := buf.String()
	assert.Contains(t, output, "1/2")
	assert.Contains(t, output, "1 failed")
}

func TestDisplayStatus_NoCurrentStep(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	ctx := workflow.NewExecutionContext("no-current", "no-current-wf")
	ctx.Status = workflow.StatusCompleted
	ctx.StartedAt = time.Now()
	ctx.CompletedAt = time.Now()
	ctx.CurrentStep = "" // No current step

	displayStatus(formatter, ctx, false)

	output := buf.String()
	assert.NotContains(t, output, "Current:")
}

func TestDisplayStatus_VerboseNoSteps(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	ctx := workflow.NewExecutionContext("no-steps", "no-steps-wf")
	ctx.Status = workflow.StatusPending
	ctx.StartedAt = time.Now()
	// No states added

	displayStatus(formatter, ctx, true) // verbose but no steps

	output := buf.String()
	// Should not show "Steps:" section when there are no steps
	assert.NotContains(t, output, "Steps:")
}

func TestDisplayStatus_VerboseNoInputs(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	ctx := workflow.NewExecutionContext("no-inputs", "no-inputs-wf")
	ctx.Status = workflow.StatusCompleted
	ctx.StartedAt = time.Now()
	ctx.CompletedAt = time.Now()
	// No inputs added

	displayStatus(formatter, ctx, true) // verbose but no inputs

	output := buf.String()
	// Should not show "Inputs:" section when there are no inputs
	assert.NotContains(t, output, "Inputs:")
}

func TestDisplayStatus(t *testing.T) {
	tests := []struct {
		name    string
		execCtx *workflow.ExecutionContext
		verbose bool
		wantOut []string // substrings expected in output
	}{
		{
			name: "basic status display",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("status-123", "test-workflow")
				ctx.Status = workflow.StatusCompleted
				ctx.StartedAt = time.Now().Add(-5 * time.Second)
				ctx.CompletedAt = time.Now()
				return ctx
			}(),
			verbose: false,
			wantOut: []string{"Workflow:", "test-workflow", "ID:", "status-123", "Status:", "Duration:"},
		},
		{
			name: "running workflow with current step",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("status-456", "running-wf")
				ctx.Status = workflow.StatusRunning
				ctx.CurrentStep = "processing"
				ctx.StartedAt = time.Now()
				return ctx
			}(),
			verbose: false,
			wantOut: []string{"running-wf", "running", "Current:", "processing"},
		},
		{
			name: "verbose with steps",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("verbose-123", "verbose-wf")
				ctx.Status = workflow.StatusCompleted
				ctx.StartedAt = time.Now().Add(-time.Second)
				ctx.CompletedAt = time.Now()
				ctx.States["step1"] = workflow.StepState{
					Name:        "step1",
					Status:      workflow.StatusCompleted,
					StartedAt:   ctx.StartedAt,
					CompletedAt: ctx.CompletedAt,
				}
				ctx.States["step2"] = workflow.StepState{
					Name:        "step2",
					Status:      workflow.StatusCompleted,
					StartedAt:   ctx.StartedAt,
					CompletedAt: ctx.CompletedAt,
				}
				return ctx
			}(),
			verbose: true,
			wantOut: []string{"Steps:", "step1", "step2", "completed"},
		},
		{
			name: "verbose with inputs",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("input-123", "input-wf")
				ctx.Status = workflow.StatusCompleted
				ctx.StartedAt = time.Now()
				ctx.CompletedAt = time.Now()
				ctx.SetInput("file", "main.go")
				ctx.SetInput("mode", "debug")
				return ctx
			}(),
			verbose: true,
			wantOut: []string{"Inputs:", "file:", "main.go", "mode:", "debug"},
		},
		{
			name: "step with error",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("error-123", "error-wf")
				ctx.Status = workflow.StatusFailed
				ctx.StartedAt = time.Now()
				ctx.CompletedAt = time.Now()
				ctx.States["failing-step"] = workflow.StepState{
					Name:        "failing-step",
					Status:      workflow.StatusFailed,
					Error:       "command not found",
					StartedAt:   ctx.StartedAt,
					CompletedAt: ctx.CompletedAt,
				}
				return ctx
			}(),
			verbose: true,
			wantOut: []string{"failing-step", "failed", "Error:", "command not found"},
		},
		{
			name: "progress counter",
			execCtx: func() *workflow.ExecutionContext {
				ctx := workflow.NewExecutionContext("progress-123", "progress-wf")
				ctx.Status = workflow.StatusRunning
				ctx.StartedAt = time.Now()
				now := time.Now()
				ctx.States["done1"] = workflow.StepState{
					Name:        "done1",
					Status:      workflow.StatusCompleted,
					StartedAt:   now,
					CompletedAt: now,
				}
				ctx.States["done2"] = workflow.StepState{
					Name:        "done2",
					Status:      workflow.StatusCompleted,
					StartedAt:   now,
					CompletedAt: now,
				}
				ctx.States["pending"] = workflow.StepState{
					Name:   "pending",
					Status: workflow.StatusPending,
				}
				return ctx
			}(),
			verbose: false,
			wantOut: []string{"Progress:", "2/3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

			displayStatus(formatter, tt.execCtx, tt.verbose)

			output := buf.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, output, want, "output should contain %q", want)
			}
		})
	}
}
