package testutil

import (
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// This file contains domain-specific assertion helpers that reduce boilerplate
// in test validation and provide clear error messages.

// AssertWorkflowValid validates that a workflow is valid according to domain rules.
// It checks for required fields, valid step references, and structural integrity.
//
// Usage:
//
//	wf := NewWorkflowBuilder().WithName("test").Build()
//	testutil.AssertWorkflowValid(t, wf)
func AssertWorkflowValid(t testing.TB, wf *workflow.Workflow) {
	t.Helper()

	// Check workflow has Name
	if wf.Name == "" {
		t.Fatalf("workflow name is required")
	}

	// Check workflow has Initial step
	if wf.Initial == "" {
		t.Fatalf("initial step is required")
	}

	// Check Initial references valid step in Steps map
	if _, exists := wf.Steps[wf.Initial]; !exists {
		t.Fatalf("initial step '%s' not found", wf.Initial)
	}

	// Check all step transitions reference valid steps
	for stepName, step := range wf.Steps {
		// Check OnSuccess target exists (if specified)
		if step.OnSuccess != "" {
			if _, exists := wf.Steps[step.OnSuccess]; !exists {
				t.Fatalf("step '%s' references invalid next step '%s'", stepName, step.OnSuccess)
			}
		}

		// Check OnFailure target exists (if specified)
		if step.OnFailure != "" {
			if _, exists := wf.Steps[step.OnFailure]; !exists {
				t.Fatalf("step '%s' references invalid next step '%s'", stepName, step.OnFailure)
			}
		}

		// Check parallel branches exist
		if step.Type == workflow.StepTypeParallel {
			for _, branchName := range step.Branches {
				if _, exists := wf.Steps[branchName]; !exists {
					t.Fatalf("step '%s' references invalid branch step '%s'", stepName, branchName)
				}
			}
		}
	}
}

// AssertStepOutput validates that a step has executed successfully with expected output.
// It checks the step state exists, has completed status, and optionally matches expected output.
//
// Usage:
//
//	testutil.AssertStepOutput(t, execCtx, "step1", "expected output")
//	testutil.AssertStepOutput(t, execCtx, "step2", "") // just verify success, don't check output
func AssertStepOutput(t testing.TB, ctx *workflow.ExecutionContext, stepName, expectedOutput string) {
	t.Helper()

	// Check ctx.States[stepName] exists
	state, exists := ctx.States[stepName]
	if !exists {
		t.Fatalf("step '%s' not found", stepName)
	}

	// Check step status is StatusCompleted
	if state.Status != workflow.StatusCompleted {
		t.Fatalf("step '%s' status is '%s', expected 'completed'", stepName, state.Status)
	}

	// If expectedOutput is non-empty, check Output field matches
	if expectedOutput != "" && state.Output != expectedOutput {
		t.Fatalf("output mismatch: expected '%s', got '%s'", expectedOutput, state.Output)
	}
}

// AssertExecutionCompleted validates that a workflow execution has completed successfully.
// It checks the execution context status, completion timestamp, and final step.
//
// Usage:
//
//	testutil.AssertExecutionCompleted(t, execCtx)
//	testutil.AssertExecutionCompleted(t, execCtx, "terminal_step")
func AssertExecutionCompleted(t testing.TB, ctx *workflow.ExecutionContext, expectedFinalStep ...string) {
	t.Helper()

	// Check ctx.Status == StatusCompleted
	if ctx.Status != workflow.StatusCompleted {
		t.Fatalf("execution status is '%s', expected 'completed'", ctx.Status)
	}

	// Check ctx.CompletedAt is not zero
	if ctx.CompletedAt.IsZero() {
		t.Fatalf("execution completed but CompletedAt is zero")
	}

	// If expectedFinalStep provided, check ctx.CurrentStep matches
	if len(expectedFinalStep) > 0 {
		expected := expectedFinalStep[0]
		if ctx.CurrentStep != expected {
			t.Fatalf("current step is '%s', expected '%s'", ctx.CurrentStep, expected)
		}
	}
}
