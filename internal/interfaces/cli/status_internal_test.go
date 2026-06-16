package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
	"github.com/stretchr/testify/assert"
)

// TestDisplayRunStatus_Basic verifies that the ID and Status fields are always shown.
func TestDisplayRunStatus_Basic(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:       "run-abc",
		Status:      ports.RunStateCompleted,
		StartedAt:   time.Now().Add(-2 * time.Second),
		CompletedAt: time.Now(),
	}

	displayRunStatus(formatter, &s, false)

	out := buf.String()
	assert.Contains(t, out, "run-abc")
	assert.Contains(t, out, "completed")
	assert.Contains(t, out, "Duration:")
}

// TestDisplayRunStatus_CurrentStep verifies that CurrentStep is displayed when set.
func TestDisplayRunStatus_CurrentStep(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:       "run-xyz",
		Status:      ports.RunStateRunning,
		CurrentStep: "build-step",
		StartedAt:   time.Now(),
	}

	displayRunStatus(formatter, &s, false)

	out := buf.String()
	assert.Contains(t, out, "Current:")
	assert.Contains(t, out, "build-step")
}

// TestDisplayRunStatus_NoCurrentStep verifies the Current line is absent when empty.
func TestDisplayRunStatus_NoCurrentStep(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:       "run-done",
		Status:      ports.RunStateCompleted,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}

	displayRunStatus(formatter, &s, false)

	assert.NotContains(t, buf.String(), "Current:")
}

// TestDisplayRunStatus_Progress verifies the Progress line with failed count.
func TestDisplayRunStatus_Progress(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:     "run-prog",
		Status:    ports.RunStateFailed,
		StartedAt: time.Now(),
		Progress:  ports.Progress{Total: 3, Completed: 2, Failed: 1},
	}

	displayRunStatus(formatter, &s, false)

	out := buf.String()
	assert.Contains(t, out, "Progress:")
	assert.Contains(t, out, "2/3")
	assert.Contains(t, out, "1 failed")
}

// TestDisplayRunStatus_ProgressNoFailed verifies the Progress line without failed count.
func TestDisplayRunStatus_ProgressNoFailed(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:     "run-ok",
		Status:    ports.RunStateCompleted,
		StartedAt: time.Now(),
		Progress:  ports.Progress{Total: 2, Completed: 2, Failed: 0},
	}

	displayRunStatus(formatter, &s, false)

	out := buf.String()
	assert.Contains(t, out, "2/2")
	assert.NotContains(t, out, "failed")
}

// TestDisplayRunStatus_NoProgressWhenZero verifies that Progress is suppressed when Total == 0.
func TestDisplayRunStatus_NoProgressWhenZero(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:  "run-pending",
		Status: ports.RunStatePending,
	}

	displayRunStatus(formatter, &s, false)

	assert.NotContains(t, buf.String(), "Progress:")
}

// TestDisplayRunStatus_VerboseSteps verifies that Steps are shown in verbose mode.
func TestDisplayRunStatus_VerboseSteps(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	now := time.Now()
	s := ports.RunStatus{
		RunID:       "run-v",
		Status:      ports.RunStateCompleted,
		StartedAt:   now.Add(-time.Second),
		CompletedAt: now,
		Steps: []ports.StepStatus{
			{Name: "step-a", Status: ports.RunStateCompleted, StartedAt: now.Add(-time.Second), CompletedAt: now},
			{Name: "step-b", Status: ports.RunStateFailed, Error: "exit 1", StartedAt: now, CompletedAt: now},
		},
	}

	displayRunStatus(formatter, &s, true)

	out := buf.String()
	assert.Contains(t, out, "Steps:")
	assert.Contains(t, out, "step-a")
	assert.Contains(t, out, "step-b")
	assert.Contains(t, out, "exit 1")
}

// TestDisplayRunStatus_VerboseStepsHiddenWhenNonVerbose verifies Steps section is absent without --verbose.
func TestDisplayRunStatus_VerboseStepsHiddenWhenNonVerbose(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:  "run-quiet",
		Status: ports.RunStateRunning,
		Steps: []ports.StepStatus{
			{Name: "step-a", Status: ports.RunStateRunning},
		},
	}

	displayRunStatus(formatter, &s, false)

	assert.NotContains(t, buf.String(), "Steps:")
}

// TestDisplayRunStatus_VerboseInputs verifies Inputs are shown in verbose mode.
func TestDisplayRunStatus_VerboseInputs(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:  "run-inputs",
		Status: ports.RunStateCompleted,
		Inputs: map[string]any{"file": "main.go", "mode": "debug"},
	}

	displayRunStatus(formatter, &s, true)

	out := buf.String()
	assert.Contains(t, out, "Inputs:")
	assert.Contains(t, out, "file")
	assert.Contains(t, out, "main.go")
}

// TestDisplayRunStatus_VerboseInputsHiddenWhenEmpty verifies Inputs section absent when nil.
func TestDisplayRunStatus_VerboseInputsHiddenWhenEmpty(t *testing.T) {
	buf := new(bytes.Buffer)
	formatter := ui.NewFormatter(buf, ui.FormatOptions{NoColor: true})

	s := ports.RunStatus{
		RunID:  "run-no-inputs",
		Status: ports.RunStateCompleted,
	}

	displayRunStatus(formatter, &s, true)

	assert.NotContains(t, buf.String(), "Inputs:")
}

// TestRunStatusToExecutionInfo_Basic verifies ID, Status and DurationMs are populated.
func TestRunStatusToExecutionInfo_Basic(t *testing.T) {
	start := time.Now().Add(-5 * time.Second)
	end := time.Now()
	s := ports.RunStatus{
		RunID:       "id-123",
		Status:      ports.RunStateCompleted,
		StartedAt:   start,
		CompletedAt: end,
	}

	info := runStatusToExecutionInfo(&s)

	assert.Equal(t, "id-123", info.WorkflowID)
	assert.Equal(t, "completed", info.Status)
	assert.Greater(t, info.DurationMs, int64(0))
	assert.NotEmpty(t, info.StartedAt)
	assert.NotEmpty(t, info.CompletedAt)
}

// TestRunStatusToExecutionInfo_CurrentStep verifies CurrentStep is propagated.
func TestRunStatusToExecutionInfo_CurrentStep(t *testing.T) {
	s := ports.RunStatus{
		RunID:       "id-456",
		Status:      ports.RunStateRunning,
		CurrentStep: "deploy",
		StartedAt:   time.Now(),
	}

	info := runStatusToExecutionInfo(&s)

	assert.Equal(t, "deploy", info.CurrentStep)
	assert.Empty(t, info.CompletedAt)
}

// TestRunStatusToExecutionInfo_Steps verifies Steps slice is populated with times.
func TestRunStatusToExecutionInfo_Steps(t *testing.T) {
	now := time.Now()
	s := ports.RunStatus{
		RunID:  "id-789",
		Status: ports.RunStateCompleted,
		Steps: []ports.StepStatus{
			{Name: "s1", Status: ports.RunStateCompleted, StartedAt: now.Add(-time.Second), CompletedAt: now, Error: ""},
			{Name: "s2", Status: ports.RunStateFailed, Error: "timeout"},
		},
	}

	info := runStatusToExecutionInfo(&s)

	assert.Len(t, info.Steps, 2)
	assert.Equal(t, "s1", info.Steps[0].Name)
	assert.Equal(t, "completed", info.Steps[0].Status)
	assert.NotEmpty(t, info.Steps[0].StartedAt)
	assert.NotEmpty(t, info.Steps[0].CompletedAt)
	assert.Equal(t, "s2", info.Steps[1].Name)
	assert.Equal(t, "failed", info.Steps[1].Status)
	assert.Equal(t, "timeout", info.Steps[1].Error)
}

// TestRunStatusToExecutionInfo_ZeroTimes verifies zero times produce empty strings.
func TestRunStatusToExecutionInfo_ZeroTimes(t *testing.T) {
	s := ports.RunStatus{
		RunID:  "zero",
		Status: ports.RunStatePending,
	}

	info := runStatusToExecutionInfo(&s)

	assert.Empty(t, info.StartedAt)
	assert.Empty(t, info.CompletedAt)
	assert.Equal(t, int64(0), info.DurationMs)
}
