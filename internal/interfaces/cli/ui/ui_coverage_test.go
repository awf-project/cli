package ui_test

import (
	"bytes"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli/ui"
)

// TestWriteDryRun_AllFormats tests WriteDryRun with all output formats
func TestWriteDryRun_AllFormats(t *testing.T) {
	plan := &workflow.DryRunPlan{
		WorkflowName: "test-workflow",
		Steps: []workflow.DryRunStep{
			{
				Name: "step1",
				Type: workflow.StepTypeCommand,
			},
		},
	}

	tests := []struct {
		name   string
		format ui.OutputFormat
	}{
		{"JSON format", ui.FormatJSON},
		{"Quiet format", ui.FormatQuiet},
		{"Text format", ui.FormatText},
		{"Table format", ui.FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := ui.NewOutputWriter(buf, buf, tt.format, false, false)

			formatter := ui.NewDryRunFormatter(buf, false)
			err := w.WriteDryRun(plan, formatter)
			if err != nil {
				t.Errorf("WriteDryRun() error = %v", err)
			}

			if buf.Len() == 0 {
				t.Errorf("WriteDryRun() produced empty output")
			}
		})
	}
}

// TestWriteResumableList_AllFormats tests WriteResumableList with all output formats
func TestWriteResumableList_AllFormats(t *testing.T) {
	infos := []ui.ResumableInfo{
		{
			WorkflowID:   "wf-123",
			WorkflowName: "test-workflow",
			CurrentStep:  "step1",
			Status:       "paused",
			UpdatedAt:    "2025-12-13T10:00:00Z",
			Progress:     "50%",
		},
		{
			WorkflowID:   "wf-456",
			WorkflowName: "another-workflow",
			CurrentStep:  "step2",
			Status:       "failed",
			UpdatedAt:    "2025-12-13T09:00:00Z",
			Progress:     "75%",
		},
	}

	tests := []struct {
		name   string
		format ui.OutputFormat
	}{
		{"JSON format", ui.FormatJSON},
		{"Quiet format", ui.FormatQuiet},
		{"Text format", ui.FormatText},
		{"Table format", ui.FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := ui.NewOutputWriter(buf, buf, tt.format, false, false)

			err := w.WriteResumableList(infos)
			if err != nil {
				t.Errorf("WriteResumableList() error = %v", err)
			}

			if buf.Len() == 0 {
				t.Errorf("WriteResumableList() produced empty output")
			}
		})
	}
}

// TestWriteResumableList_EmptyList tests WriteResumableList with no workflows
func TestWriteResumableList_EmptyList(t *testing.T) {
	infos := []ui.ResumableInfo{}

	formats := []ui.OutputFormat{ui.FormatJSON, ui.FormatQuiet, ui.FormatText, ui.FormatTable}

	for _, format := range formats {
		buf := &bytes.Buffer{}
		w := ui.NewOutputWriter(buf, buf, format, false, false)

		err := w.WriteResumableList(infos)
		if err != nil {
			t.Errorf("WriteResumableList() with empty list error = %v", err)
		}
	}
}

// TestFormatter_Println tests uncovered Println method
func TestFormatter_Println(t *testing.T) {
	buf := &bytes.Buffer{}
	f := ui.NewFormatter(buf, ui.FormatOptions{})

	f.Println("test message")

	if buf.Len() == 0 {
		t.Error("Println() produced no output")
	}

	// Test with multiple args
	buf.Reset()
	f.Println("test", "multiple", "args")

	if buf.Len() == 0 {
		t.Error("Println() with multiple args produced no output")
	}
}

// TestFormatter_Warning tests uncovered Warning method
func TestFormatter_Warning(t *testing.T) {
	tests := []struct {
		name  string
		quiet bool
	}{
		{"Warning in normal mode", false},
		{"Warning in quiet mode (suppressed)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			f := ui.NewFormatter(buf, ui.FormatOptions{Quiet: tt.quiet})

			f.Warning("test warning")

			if tt.quiet {
				if buf.Len() != 0 {
					t.Error("Warning() should be suppressed in quiet mode")
				}
			} else {
				if buf.Len() == 0 {
					t.Error("Warning() should produce output in normal mode")
				}
			}
		})
	}
}

// TestDryRunFormatter_WithHooks tests formatHooks coverage
func TestDryRunFormatter_WithHooks(t *testing.T) {
	plan := &workflow.DryRunPlan{
		WorkflowName: "workflow-with-hooks",
		Steps: []workflow.DryRunStep{
			{
				Name: "step-with-hooks",
				Type: workflow.StepTypeCommand,
				Hooks: workflow.DryRunHooks{
					Pre: []workflow.DryRunHook{
						{Type: "log", Content: "Starting step"},
						{Type: "command", Content: "echo 'pre hook'"},
					},
					Post: []workflow.DryRunHook{
						{Type: "log", Content: "Completed step"},
						{Type: "command", Content: "echo 'post hook'"},
					},
				},
			},
		},
	}

	buf := &bytes.Buffer{}
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.Format(plan)
	if err != nil {
		t.Errorf("Format() with hooks error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Format() with hooks produced no output")
	}
}

// TestDryRunFormatter_LoopStep tests formatLoopStep coverage
func TestDryRunFormatter_LoopStep(t *testing.T) {
	plan := &workflow.DryRunPlan{
		WorkflowName: "workflow-with-loop",
		Steps: []workflow.DryRunStep{
			{
				Name: "loop-step",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.DryRunLoop{
					Type:          "for_each",
					Items:         "{{inputs.items}}",
					Body:          []string{"process-item"},
					MaxIterations: 100,
				},
			},
		},
	}

	buf := &bytes.Buffer{}
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.Format(plan)
	if err != nil {
		t.Errorf("Format() with loop error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Format() with loop produced no output")
	}
}

// TestDryRunFormatter_ParallelStep tests formatParallelStep coverage
func TestDryRunFormatter_ParallelStep(t *testing.T) {
	plan := &workflow.DryRunPlan{
		WorkflowName: "workflow-with-parallel",
		Steps: []workflow.DryRunStep{
			{
				Name:          "parallel-step",
				Type:          workflow.StepTypeParallel,
				Branches:      []string{"branch1", "branch2", "branch3"},
				Strategy:      "all_succeed",
				MaxConcurrent: 2,
			},
		},
	}

	buf := &bytes.Buffer{}
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.Format(plan)
	if err != nil {
		t.Errorf("Format() with parallel error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Format() with parallel produced no output")
	}
}

// TestDryRunFormatter_ComplexTransitions tests formatTransitions with all cases
func TestDryRunFormatter_ComplexTransitions(t *testing.T) {
	plan := &workflow.DryRunPlan{
		WorkflowName: "workflow-with-transitions",
		Steps: []workflow.DryRunStep{
			{
				Name: "step-with-transitions",
				Type: workflow.StepTypeCommand,
				Transitions: []workflow.DryRunTransition{
					{Type: "success", Target: "next-step"},
					{Type: "failure", Target: "error-step"},
					{Type: "conditional", Condition: "{{result.code}} == 42", Target: "special-step"},
				},
			},
			{
				Name:   "terminal-step",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	buf := &bytes.Buffer{}
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.Format(plan)
	if err != nil {
		t.Errorf("Format() with transitions error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Format() with transitions produced no output")
	}
}

// TestDryRunFormatter_AllStepTypes tests stepTypeLabel coverage
func TestDryRunFormatter_AllStepTypes(t *testing.T) {
	plan := &workflow.DryRunPlan{
		WorkflowName: "workflow-with-all-types",
		Steps: []workflow.DryRunStep{
			{Name: "step1", Type: workflow.StepTypeCommand},
			{Name: "parallel1", Type: workflow.StepTypeParallel},
			{Name: "loop1", Type: workflow.StepTypeForEach},
			{Name: "while1", Type: workflow.StepTypeWhile},
			{Name: "terminal1", Type: workflow.StepTypeTerminal},
		},
	}

	buf := &bytes.Buffer{}
	formatter := ui.NewDryRunFormatter(buf, false)

	err := formatter.Format(plan)
	if err != nil {
		t.Errorf("Format() with all step types error = %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Format() with all step types produced no output")
	}
}
