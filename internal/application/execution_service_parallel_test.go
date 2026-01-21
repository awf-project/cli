package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// =============================================================================
// Parallel Execution Tests
// Feature: C008 - Test File Restructuring
// Component: extract_parallel_tests (T007)
// =============================================================================
//
// This file contains parallel execution integration tests for ExecutionService.
// Tests verify parallel/branch execution with resume functionality.
//
// Extracted from: execution_service_test.go (lines 1359-1399)
// Test count: 1 parallel execution test
// =============================================================================

// Mock types are defined in:
// - service_test.go (common mocks)
// - execution_service_specialized_mocks_test.go (specialized mocks)

func TestExecutionService_Resume_ParallelStep(t *testing.T) {
	// Resume from a parallel step
	wf := &workflow.Workflow{
		Name:    "parallel-resume",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				Strategy:  "all_succeed",
				OnSuccess: "done",
			},
			"branch1": {Name: "branch1", Type: workflow.StepTypeCommand, Command: "echo b1", OnSuccess: "done"},
			"branch2": {Name: "branch2", Type: workflow.StepTypeCommand, Command: "echo b2", OnSuccess: "done"},
			"done":    {Name: "done", Type: workflow.StepTypeTerminal},
		},
	}

	execSvc, mocks := NewTestHarness(t).
		WithWorkflow("parallel-resume", wf).
		Build()

	// Setup existing execution state for resume
	err := mocks.StateStore.Save(context.Background(), &workflow.ExecutionContext{
		WorkflowID:   "parallel-id",
		WorkflowName: "parallel-resume",
		Status:       workflow.StatusRunning,
		CurrentStep:  "parallel",
		Inputs:       make(map[string]any),
		States:       make(map[string]workflow.StepState),
		Env:          make(map[string]string),
	})
	require.NoError(t, err, "setup should succeed")

	ctx, err := execSvc.Resume(context.Background(), "parallel-id", nil)

	require.NoError(t, err, "resume with parallel step should succeed")
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)
	assert.Equal(t, "done", ctx.CurrentStep)
}
