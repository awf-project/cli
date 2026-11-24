package tui

import (
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// TreeNode represents a single node in the execution tree view.
type TreeNode struct {
	Name       string
	Status     workflow.ExecutionStatus
	Duration   time.Duration
	Depth      int
	Children   []*TreeNode
	IsParallel bool
}

// BuildTree constructs a slice of TreeNodes from an ordered step slice and a states map.
// Steps are processed in the order provided by the caller. Parallel steps produce a parent
// node (IsParallel=true) with each branch name added as a child node. Missing states
// default to StatusPending.
func BuildTree(steps []workflow.Step, states map[string]workflow.StepState) []*TreeNode {
	if len(steps) == 0 {
		return []*TreeNode{}
	}

	nodes := make([]*TreeNode, 0, len(steps))
	for i := range steps {
		step := &steps[i]
		node := buildNode(step, states, 0)
		nodes = append(nodes, node)
	}
	return nodes
}

// buildNode creates a TreeNode for a step at the given depth, resolving state from the map.
func buildNode(step *workflow.Step, states map[string]workflow.StepState, depth int) *TreeNode {
	node := &TreeNode{
		Name:       step.Name,
		Status:     resolveStatus(step.Name, states),
		Duration:   resolveDuration(step.Name, states),
		Depth:      depth,
		Children:   []*TreeNode{},
		IsParallel: step.Type == workflow.StepTypeParallel,
	}

	if step.Type == workflow.StepTypeParallel {
		for _, branch := range step.Branches {
			child := &TreeNode{
				Name:       branch,
				Status:     resolveStatus(branch, states),
				Duration:   resolveDuration(branch, states),
				Depth:      depth + 1,
				Children:   []*TreeNode{},
				IsParallel: false,
			}
			node.Children = append(node.Children, child)
		}
	}

	return node
}

// resolveStatus returns the execution status for a step name from the states map.
// Returns StatusPending when the step has no recorded state.
func resolveStatus(name string, states map[string]workflow.StepState) workflow.ExecutionStatus {
	if state, ok := states[name]; ok {
		return state.Status
	}
	return workflow.StatusPending
}

// resolveDuration computes the duration for a step from its state timestamps.
// Returns zero when no state exists or the step has not completed.
func resolveDuration(name string, states map[string]workflow.StepState) time.Duration {
	state, ok := states[name]
	if !ok {
		return 0
	}
	if state.StartedAt.IsZero() || state.CompletedAt.IsZero() {
		return 0
	}
	return state.CompletedAt.Sub(state.StartedAt)
}
