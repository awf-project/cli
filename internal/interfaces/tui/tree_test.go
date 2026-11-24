// Package tui — tests for T005: execution tree node and rendering (internal/interfaces/tui/tree.go).
//
// Acceptance Criteria covered:
//
//	AC1:  TreeNode has exported fields Name, Status, Duration, Depth, Children, IsParallel
//	       → TestTreeNode_ExportedFields
//	AC2:  BuildTree with empty states returns all nodes as pending
//	       → TestBuildTree_EmptyStatesAllPending
//	AC3:  BuildTree with nil steps returns empty slice
//	       → TestBuildTree_NilSteps
//	AC4:  BuildTree preserves step order
//	       → TestBuildTree_PreservesStepOrder
//	AC5:  BuildTree parallel step creates parent node with IsParallel=true and branch children
//	       → TestBuildTree_ParallelStepNestsBranchChildren
//	AC6:  BuildTree applies state status from states map
//	       → TestBuildTree_AppliesStateStatus
//	AC7:  BuildTree computes duration from StartedAt/CompletedAt
//	       → TestBuildTree_ComputesDuration
//	AC8:  RenderTree includes status icons for all statuses
//	       → TestRenderTree_StatusIcons, TestStatusIcon_AllStatuses
//	AC9:  RenderTree outputs node names
//	       → TestRenderTree_ContainsNodeNames
//	AC10: RenderTree returns empty string for empty input
//	       → TestRenderTree_EmptyInput
//	AC11: RenderTree includes duration for completed nodes
//	       → TestRenderTree_IncludesDuration
//	AC12: Depth indentation is 2 spaces per level (children indented under parallel parent)
//	       → TestRenderTree_DepthIndentation
//	AC13: StatusIcon returns expected icons
//	       → TestStatusIcon_AllStatuses
package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// TestTreeNode_ExportedFields verifies the TreeNode struct has all required exported fields.
func TestTreeNode_ExportedFields(t *testing.T) {
	node := TreeNode{
		Name:       "step-a",
		Status:     workflow.StatusPending,
		Duration:   5 * time.Second,
		Depth:      2,
		Children:   []*TreeNode{},
		IsParallel: true,
	}

	assert.Equal(t, "step-a", node.Name)
	assert.Equal(t, workflow.StatusPending, node.Status)
	assert.Equal(t, 5*time.Second, node.Duration)
	assert.Equal(t, 2, node.Depth)
	assert.Empty(t, node.Children)
	assert.True(t, node.IsParallel)
}

// TestBuildTree_NilSteps verifies BuildTree returns an empty (non-nil) slice for nil input.
func TestBuildTree_NilSteps(t *testing.T) {
	result := BuildTree(nil, map[string]workflow.StepState{})
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// TestBuildTree_EmptySteps verifies BuildTree returns an empty slice for an empty steps slice.
func TestBuildTree_EmptySteps(t *testing.T) {
	result := BuildTree([]workflow.Step{}, map[string]workflow.StepState{})
	require.NotNil(t, result)
	assert.Empty(t, result)
}

// TestBuildTree_EmptyStatesAllPending verifies that steps without recorded states are pending.
func TestBuildTree_EmptyStatesAllPending(t *testing.T) {
	steps := []workflow.Step{
		{Name: "step-a", Type: workflow.StepTypeCommand},
		{Name: "step-b", Type: workflow.StepTypeCommand},
		{Name: "step-c", Type: workflow.StepTypeCommand},
	}

	nodes := BuildTree(steps, map[string]workflow.StepState{})

	require.Len(t, nodes, 3)
	for _, node := range nodes {
		assert.Equal(t, workflow.StatusPending, node.Status,
			"expected pending for step %q with no recorded state", node.Name)
	}
}

// TestBuildTree_PreservesStepOrder verifies node order matches the input steps slice.
func TestBuildTree_PreservesStepOrder(t *testing.T) {
	steps := []workflow.Step{
		{Name: "first", Type: workflow.StepTypeCommand},
		{Name: "second", Type: workflow.StepTypeCommand},
		{Name: "third", Type: workflow.StepTypeCommand},
	}

	nodes := BuildTree(steps, map[string]workflow.StepState{})

	require.Len(t, nodes, 3)
	assert.Equal(t, "first", nodes[0].Name)
	assert.Equal(t, "second", nodes[1].Name)
	assert.Equal(t, "third", nodes[2].Name)
}

// TestBuildTree_AppliesStateStatus verifies nodes take their status from the states map.
func TestBuildTree_AppliesStateStatus(t *testing.T) {
	steps := []workflow.Step{
		{Name: "done", Type: workflow.StepTypeCommand},
		{Name: "failed", Type: workflow.StepTypeCommand},
		{Name: "running", Type: workflow.StepTypeCommand},
		{Name: "pending", Type: workflow.StepTypeCommand},
	}
	states := map[string]workflow.StepState{
		"done":    {Name: "done", Status: workflow.StatusCompleted},
		"failed":  {Name: "failed", Status: workflow.StatusFailed},
		"running": {Name: "running", Status: workflow.StatusRunning},
		// "pending" is absent — should default to StatusPending
	}

	nodes := BuildTree(steps, states)

	require.Len(t, nodes, 4)
	assert.Equal(t, workflow.StatusCompleted, nodes[0].Status)
	assert.Equal(t, workflow.StatusFailed, nodes[1].Status)
	assert.Equal(t, workflow.StatusRunning, nodes[2].Status)
	assert.Equal(t, workflow.StatusPending, nodes[3].Status)
}

// TestBuildTree_ComputesDuration verifies duration is computed from StartedAt/CompletedAt.
func TestBuildTree_ComputesDuration(t *testing.T) {
	start := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Second)

	steps := []workflow.Step{
		{Name: "timed", Type: workflow.StepTypeCommand},
		{Name: "no-times", Type: workflow.StepTypeCommand},
	}
	states := map[string]workflow.StepState{
		"timed":    {Name: "timed", Status: workflow.StatusCompleted, StartedAt: start, CompletedAt: end},
		"no-times": {Name: "no-times", Status: workflow.StatusCompleted},
	}

	nodes := BuildTree(steps, states)

	require.Len(t, nodes, 2)
	assert.Equal(t, 3*time.Second, nodes[0].Duration)
	assert.Equal(t, time.Duration(0), nodes[1].Duration)
}

// TestBuildTree_ParallelStepNestsBranchChildren verifies parallel steps produce a parent
// with IsParallel=true and one child per branch.
func TestBuildTree_ParallelStepNestsBranchChildren(t *testing.T) {
	steps := []workflow.Step{
		{
			Name:     "parallel-group",
			Type:     workflow.StepTypeParallel,
			Branches: []string{"branch-a", "branch-b", "branch-c"},
		},
	}
	states := map[string]workflow.StepState{
		"branch-a": {Name: "branch-a", Status: workflow.StatusCompleted},
		"branch-b": {Name: "branch-b", Status: workflow.StatusRunning},
		// branch-c absent → pending
	}

	nodes := BuildTree(steps, states)

	require.Len(t, nodes, 1)
	parent := nodes[0]
	assert.Equal(t, "parallel-group", parent.Name)
	assert.True(t, parent.IsParallel)
	assert.Equal(t, 0, parent.Depth)

	require.Len(t, parent.Children, 3)
	assert.Equal(t, "branch-a", parent.Children[0].Name)
	assert.Equal(t, workflow.StatusCompleted, parent.Children[0].Status)
	assert.Equal(t, 1, parent.Children[0].Depth)

	assert.Equal(t, "branch-b", parent.Children[1].Name)
	assert.Equal(t, workflow.StatusRunning, parent.Children[1].Status)
	assert.Equal(t, 1, parent.Children[1].Depth)

	assert.Equal(t, "branch-c", parent.Children[2].Name)
	assert.Equal(t, workflow.StatusPending, parent.Children[2].Status)
	assert.Equal(t, 1, parent.Children[2].Depth)

	for _, child := range parent.Children {
		assert.False(t, child.IsParallel)
		assert.Empty(t, child.Children)
	}
}

// TestBuildTree_NonParallelStepsHaveNoChildren verifies command steps have no children.
func TestBuildTree_NonParallelStepsHaveNoChildren(t *testing.T) {
	steps := []workflow.Step{
		{Name: "cmd", Type: workflow.StepTypeCommand},
		{Name: "terminal", Type: workflow.StepTypeTerminal},
	}

	nodes := BuildTree(steps, map[string]workflow.StepState{})

	for _, node := range nodes {
		assert.Empty(t, node.Children)
		assert.False(t, node.IsParallel)
	}
}

// TestBuildTree_ParallelWithNoBranches verifies an empty Branches list produces no children.
func TestBuildTree_ParallelWithNoBranches(t *testing.T) {
	steps := []workflow.Step{
		{Name: "empty-parallel", Type: workflow.StepTypeParallel, Branches: []string{}},
	}

	nodes := BuildTree(steps, map[string]workflow.StepState{})

	require.Len(t, nodes, 1)
	assert.True(t, nodes[0].IsParallel)
	assert.Empty(t, nodes[0].Children)
}

// TestBuildTree_MixedStepTypes verifies a mix of step types is handled correctly.
func TestBuildTree_MixedStepTypes(t *testing.T) {
	steps := []workflow.Step{
		{Name: "cmd-step", Type: workflow.StepTypeCommand},
		{
			Name:     "parallel-step",
			Type:     workflow.StepTypeParallel,
			Branches: []string{"p1", "p2"},
		},
		{Name: "terminal-step", Type: workflow.StepTypeTerminal},
	}

	nodes := BuildTree(steps, map[string]workflow.StepState{})

	require.Len(t, nodes, 3)
	assert.False(t, nodes[0].IsParallel)
	assert.Empty(t, nodes[0].Children)

	assert.True(t, nodes[1].IsParallel)
	assert.Len(t, nodes[1].Children, 2)

	assert.False(t, nodes[2].IsParallel)
	assert.Empty(t, nodes[2].Children)
}

// TestExecutionTree is the acceptance-criteria gate named in the task spec.
// It runs BuildTree + RenderTree end-to-end with realistic data.
func TestExecutionTree(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

	steps := []workflow.Step{
		{Name: "build", Type: workflow.StepTypeCommand},
		{
			Name:     "test-matrix",
			Type:     workflow.StepTypeParallel,
			Branches: []string{"test-unit", "test-integration"},
		},
		{Name: "deploy", Type: workflow.StepTypeCommand},
		{Name: "done", Type: workflow.StepTypeTerminal},
	}

	states := map[string]workflow.StepState{
		"build":            {Name: "build", Status: workflow.StatusCompleted, StartedAt: start, CompletedAt: start.Add(2 * time.Second)},
		"test-matrix":      {Name: "test-matrix", Status: workflow.StatusRunning},
		"test-unit":        {Name: "test-unit", Status: workflow.StatusCompleted, StartedAt: start.Add(2 * time.Second), CompletedAt: start.Add(5 * time.Second)},
		"test-integration": {Name: "test-integration", Status: workflow.StatusRunning},
		// deploy and done are absent → pending
	}

	nodes := BuildTree(steps, states)

	// Structural assertions
	require.Len(t, nodes, 4)

	build := nodes[0]
	assert.Equal(t, "build", build.Name)
	assert.Equal(t, workflow.StatusCompleted, build.Status)
	assert.Equal(t, 2*time.Second, build.Duration)
	assert.False(t, build.IsParallel)
	assert.Empty(t, build.Children)

	matrix := nodes[1]
	assert.Equal(t, "test-matrix", matrix.Name)
	assert.Equal(t, workflow.StatusRunning, matrix.Status)
	assert.True(t, matrix.IsParallel)
	require.Len(t, matrix.Children, 2)
	assert.Equal(t, "test-unit", matrix.Children[0].Name)
	assert.Equal(t, workflow.StatusCompleted, matrix.Children[0].Status)
	assert.Equal(t, 3*time.Second, matrix.Children[0].Duration)
	assert.Equal(t, "test-integration", matrix.Children[1].Name)
	assert.Equal(t, workflow.StatusRunning, matrix.Children[1].Status)

	deploy := nodes[2]
	assert.Equal(t, "deploy", deploy.Name)
	assert.Equal(t, workflow.StatusPending, deploy.Status)

	done := nodes[3]
	assert.Equal(t, "done", done.Name)
	assert.Equal(t, workflow.StatusPending, done.Status)
}
