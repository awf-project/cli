package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

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

// StatusIcon returns the display icon for the given execution status.
func StatusIcon(status workflow.ExecutionStatus) string {
	switch status {
	case workflow.StatusPending:
		return "⏳"
	case workflow.StatusRunning:
		return "⟳"
	case workflow.StatusCompleted:
		return "✓"
	case workflow.StatusFailed:
		return "✗"
	case workflow.StatusCancelled:
		return "⏭"
	default:
		return "⏳"
	}
}

// StatusBadgeIcon returns a colored icon string for the given execution status using theme colors.
func StatusBadgeIcon(status workflow.ExecutionStatus) string {
	switch status {
	case workflow.StatusPending:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("⏳")
	case workflow.StatusRunning:
		return lipgloss.NewStyle().Foreground(ColorRunning).Render("⟳")
	case workflow.StatusCompleted:
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
	case workflow.StatusFailed:
		return lipgloss.NewStyle().Foreground(ColorError).Render("✗")
	case workflow.StatusCancelled:
		return lipgloss.NewStyle().Foreground(ColorWarning).Render("⏭")
	default:
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("⏳")
	}
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

// RenderTree produces a formatted string representing the execution tree.
// Each node is rendered with a status icon, its name, and optionally its duration.
// Parallel children are prefixed with "║" while sequential nodes use "│".
// Depth indentation is 2 spaces per level.
func RenderTree(nodes []*TreeNode) string {
	var sb strings.Builder
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		renderNode(&sb, node, "", isLast, false)
	}
	return sb.String()
}

// renderNode writes a single node and its children into the builder.
// When parentIsParallel is true, parallel box-drawing connectors (╠══/╚══) and
// the double vertical bar (║) are used instead of the sequential equivalents.
func renderNode(sb *strings.Builder, node *TreeNode, prefix string, last, parentIsParallel bool) {
	var connector string
	switch {
	case prefix == "":
		connector = ""
	case parentIsParallel && last:
		connector = "╚══ "
	case parentIsParallel:
		connector = "╠══ "
	case last:
		connector = "└── "
	default:
		connector = "├── "
	}

	icon := StatusBadgeIcon(node.Status)

	namePrefix := ""
	if node.IsParallel {
		namePrefix = "⑊ "
	}

	durationStr := ""
	if node.Duration > 0 {
		durationStr = " " + lipgloss.NewStyle().Foreground(ColorMuted).Render("("+formatDuration(node.Duration)+")")
	}

	line := prefix + connector + icon + " " + namePrefix + node.Name + durationStr
	sb.WriteString(line)
	sb.WriteString("\n")

	for i, child := range node.Children {
		isLast := i == len(node.Children)-1

		// Build child prefix: extend parent prefix based on position and parallel context.
		var childPrefix string
		switch {
		case prefix == "" && connector == "":
			childPrefix = "  "
		case parentIsParallel && last:
			childPrefix = prefix + "    "
		case parentIsParallel:
			childPrefix = prefix + "║   "
		case last:
			childPrefix = prefix + "    "
		default:
			childPrefix = prefix + "│   "
		}

		renderNode(sb, child, childPrefix, isLast, node.IsParallel)
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
