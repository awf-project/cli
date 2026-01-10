package diagram

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// Generator generates DOT format output from workflow definitions.
type Generator struct {
	config *DiagramConfig
}

// NewGenerator creates a new DOT generator with the given configuration.
func NewGenerator(config *DiagramConfig) *Generator {
	if config == nil {
		config = NewDefaultConfig()
	}
	return &Generator{config: config}
}

// Generate produces a DOT format string from the workflow.
func (g *Generator) Generate(wf *workflow.Workflow) string {
	var sb strings.Builder

	sb.WriteString(g.generateHeader(wf.Name))
	sb.WriteString(g.generateStartNode(wf.Initial))
	sb.WriteString(g.generateNodes(wf))
	sb.WriteString(g.generateEdges(wf))
	sb.WriteString("}\n")

	return sb.String()
}

// generateStartNode creates the invisible start node pointing to the initial step.
func (g *Generator) generateStartNode(initialStep string) string {
	var sb strings.Builder
	sb.WriteString("    __start__ [shape=point, width=0.2];\n")
	sb.WriteString(fmt.Sprintf("    __start__ -> %s;\n", escapeDOTID(initialStep)))
	return sb.String()
}

// generateHeader generates the DOT digraph header with configuration.
// Per FR-007: `--direction <TB|LR|BT|RL>` flag controls graph layout direction.
// The header includes:
//   - digraph declaration with workflow name
//   - rankdir attribute based on config.Direction
//   - default node and edge attributes
//
// Example output:
//
//	digraph "workflow_name" {
//	    rankdir=LR;
//	    node [fontname="Arial", fontsize=10];
//	    edge [fontname="Arial", fontsize=9];
func (g *Generator) generateHeader(name string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("digraph %q {\n", name))
	sb.WriteString(fmt.Sprintf("    rankdir=%s;\n", g.config.Direction))
	sb.WriteString("    node [fontname=\"Arial\", fontsize=10];\n")
	sb.WriteString("    edge [fontname=\"Arial\", fontsize=9];\n")

	return sb.String()
}

// stepTypeToShape maps workflow step types to DOT shape names.
// Shapes per FR-002:
//   - command    → box
//   - parallel   → diamond
//   - terminal   → oval (doubleoval for failure)
//   - for_each   → hexagon
//   - while      → hexagon
//   - operation  → box3d
//   - call_workflow → folder
var stepTypeToShape = map[workflow.StepType]string{
	workflow.StepTypeCommand:      "box",
	workflow.StepTypeParallel:     "diamond",
	workflow.StepTypeTerminal:     "oval",
	workflow.StepTypeForEach:      "hexagon",
	workflow.StepTypeWhile:        "hexagon",
	workflow.StepTypeOperation:    "box3d",
	workflow.StepTypeCallWorkflow: "folder",
}

// generateNodes generates DOT node declarations for all workflow steps.
// Each step is rendered as a node with a shape based on its type.
// Terminal steps with failure status use doubleoval shape.
// Highlighted steps (via config.Highlight) receive visual emphasis.
func (g *Generator) generateNodes(wf *workflow.Workflow) string {
	var sb strings.Builder

	// Get sorted step names for deterministic output
	stepNames := make([]string, 0, len(wf.Steps))
	for name := range wf.Steps {
		stepNames = append(stepNames, name)
	}
	sort.Strings(stepNames)

	for _, name := range stepNames {
		step := wf.Steps[name]

		// Handle parallel steps with subgraph clusters
		if step.Type == workflow.StepTypeParallel {
			// Output the parallel step node itself (diamond shape)
			style := NodeStyle{Shape: "diamond"}
			style = g.applyHighlight(name, style)
			sb.WriteString(g.formatNode(name, style))

			// Output the subgraph cluster for branches
			sb.WriteString(g.generateParallelSubgraph(step, wf))
			continue
		}

		// Determine shape based on step type
		shape := stepTypeToShape[step.Type]
		if shape == "" {
			shape = "box" // default fallback
		}

		// Terminal steps have special styling
		if step.Type == workflow.StepTypeTerminal {
			if step.Status == workflow.TerminalFailure {
				shape = "doubleoval"
			}
		}

		// Build node style
		style := NodeStyle{Shape: shape}

		// Add color styling for terminal steps
		if step.Type == workflow.StepTypeTerminal {
			switch step.Status {
			case workflow.TerminalSuccess:
				style.FillColor = "#90EE90" // light green
				style.Style = "filled"
			case workflow.TerminalFailure:
				style.FillColor = "#FFB6C1" // light red
				style.Style = "filled"
			}
		}

		style = g.applyHighlight(name, style)

		sb.WriteString(g.formatNode(name, style))
	}

	return sb.String()
}

// formatNode formats a single node declaration with its attributes.
func (g *Generator) formatNode(name string, style NodeStyle) string {
	attrs := []string{
		fmt.Sprintf("label=%q", name),
		fmt.Sprintf("shape=%s", style.Shape),
	}

	if style.Color != "" {
		attrs = append(attrs, fmt.Sprintf("color=%q", style.Color))
	}
	if style.FillColor != "" {
		attrs = append(attrs, fmt.Sprintf("fillcolor=%q", style.FillColor))
	}
	if style.Style != "" {
		attrs = append(attrs, fmt.Sprintf("style=%q", style.Style))
		// Add penwidth for bold style to ensure visual emphasis
		if strings.Contains(style.Style, "bold") {
			attrs = append(attrs, "penwidth=3")
		}
	}

	return fmt.Sprintf("    %s [%s];\n", escapeDOTID(name), strings.Join(attrs, ", "))
}

// generateEdges generates DOT edge declarations for workflow transitions.
// Uses workflow.GetTransitions() to enumerate all transitions from each step.
// Per FR-003:
//   - on_success → solid line (default edge style)
//   - on_failure → dashed red line [style=dashed, color=red]
//   - branches (parallel) → solid line to each branch step
func (g *Generator) generateEdges(wf *workflow.Workflow) string {
	var sb strings.Builder

	// Get sorted step names for deterministic output
	stepNames := make([]string, 0, len(wf.Steps))
	for name := range wf.Steps {
		stepNames = append(stepNames, name)
	}
	sort.Strings(stepNames)

	for _, name := range stepNames {
		step := wf.Steps[name]

		// Skip terminal steps - they have no outgoing edges
		if step.Type == workflow.StepTypeTerminal {
			continue
		}

		// Handle parallel branches
		if step.Type == workflow.StepTypeParallel {
			for _, branch := range step.Branches {
				sb.WriteString(fmt.Sprintf("    %s -> %s;\n", escapeDOTID(name), escapeDOTID(branch)))
			}
		}

		// Handle loop body steps
		if step.Loop != nil && len(step.Loop.Body) > 0 {
			for _, bodyStep := range step.Loop.Body {
				sb.WriteString(fmt.Sprintf("    %s -> %s;\n", escapeDOTID(name), escapeDOTID(bodyStep)))
			}
			if step.Loop.OnComplete != "" {
				sb.WriteString(fmt.Sprintf("    %s -> %s [label=\"complete\"];\n", escapeDOTID(name), escapeDOTID(step.Loop.OnComplete)))
			}
		}

		// Handle success transition
		if step.OnSuccess != "" {
			sb.WriteString(fmt.Sprintf("    %s -> %s;\n", escapeDOTID(name), escapeDOTID(step.OnSuccess)))
		}

		// Handle failure transition with red dashed style
		if step.OnFailure != "" {
			sb.WriteString(fmt.Sprintf("    %s -> %s [style=dashed, color=red];\n", escapeDOTID(name), escapeDOTID(step.OnFailure)))
		}

		// Handle conditional transitions
		for _, tr := range step.Transitions {
			label := ""
			if tr.When != "" {
				label = fmt.Sprintf(" [label=%q]", tr.When)
			}
			sb.WriteString(fmt.Sprintf("    %s -> %s%s;\n", escapeDOTID(name), escapeDOTID(tr.Goto), label))
		}
	}

	return sb.String()
}

// generateParallelSubgraph generates a DOT subgraph cluster for a parallel step.
// Per FR-004: Parallel branches are grouped in subgraph clusters.
// The subgraph contains the branch steps and visually groups them together.
// Returns a DOT subgraph declaration in the format:
//
//	subgraph cluster_<step_name> {
//	    label="<step_name>";
//	    style=dashed;
//	    <branch_nodes>
//	}
func (g *Generator) generateParallelSubgraph(step *workflow.Step, wf *workflow.Workflow) string {
	if step == nil || wf == nil {
		return ""
	}

	var sb strings.Builder

	// Create subgraph cluster for branches
	sb.WriteString(fmt.Sprintf("    subgraph cluster_%s {\n", escapeDOTID(step.Name)))
	sb.WriteString(fmt.Sprintf("        label=%q;\n", step.Name+" branches"))
	sb.WriteString("        style=dashed;\n")

	// Add branch nodes to the cluster
	for _, branchName := range step.Branches {
		branchStep, ok := wf.Steps[branchName]
		if !ok {
			continue
		}

		shape := stepTypeToShape[branchStep.Type]
		if shape == "" {
			shape = "box"
		}
		if branchStep.Type == workflow.StepTypeTerminal && branchStep.Status == workflow.TerminalFailure {
			shape = "doubleoval"
		}
		branchStyle := NodeStyle{Shape: shape}
		branchStyle = g.applyHighlight(branchName, branchStyle)

		// Format with extra indent for subgraph
		attrs := []string{
			fmt.Sprintf("label=%q", branchName),
			fmt.Sprintf("shape=%s", branchStyle.Shape),
		}
		if branchStyle.Color != "" {
			attrs = append(attrs, fmt.Sprintf("color=%q", branchStyle.Color))
		}
		if branchStyle.FillColor != "" {
			attrs = append(attrs, fmt.Sprintf("fillcolor=%q", branchStyle.FillColor))
		}
		if branchStyle.Style != "" {
			attrs = append(attrs, fmt.Sprintf("style=%q", branchStyle.Style))
		}
		sb.WriteString(fmt.Sprintf("        %s [%s];\n", escapeDOTID(branchName), strings.Join(attrs, ", ")))
	}

	sb.WriteString("    }\n")

	return sb.String()
}

// isHighlighted checks if the given step name matches the highlight configuration.
// Per US3: `--highlight step_name` should visually emphasize the specified step.
// Returns true if the step should be highlighted in the diagram.
func (g *Generator) isHighlighted(stepName string) bool {
	return g.config.Highlight != "" && g.config.Highlight == stepName
}

// getHighlightStyle returns the NodeStyle attributes for a highlighted step.
// Highlighted steps receive visual emphasis through:
//   - Increased penwidth (bold border)
//   - Distinctive color (blue #2196F3)
//   - filled,bold style
//
// This creates clear visual distinction for the highlighted step.
func (g *Generator) getHighlightStyle() NodeStyle {
	return NodeStyle{
		Color:     "#2196F3",
		FillColor: "#BBDEFB",
		Style:     "filled,bold",
	}
}

// applyHighlight merges highlight styling onto a base NodeStyle if the step is highlighted.
// If stepName matches config.Highlight, the returned style includes:
//   - penwidth=3 for bold border
//   - color=#2196F3 (material blue) for emphasis
//   - style includes "bold" in addition to base style
//
// If stepName is not highlighted, returns the base style unchanged.
func (g *Generator) applyHighlight(stepName string, baseStyle NodeStyle) NodeStyle {
	if !g.isHighlighted(stepName) {
		return baseStyle
	}

	highlightStyle := g.getHighlightStyle()
	baseStyle.Color = highlightStyle.Color
	baseStyle.FillColor = highlightStyle.FillColor
	baseStyle.Style = highlightStyle.Style

	return baseStyle
}

// escapeDOTID escapes a string for use as a DOT identifier.
// DOT identifiers with special characters need quoting.
func escapeDOTID(s string) string {
	// If the string contains only alphanumeric and underscore, it's safe
	safe := true
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			safe = false
			break
		}
	}

	if safe && s != "" && (s[0] < '0' || s[0] > '9') {
		return s
	}

	// Quote and escape internal quotes
	escaped := strings.ReplaceAll(s, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return fmt.Sprintf("%q", escaped)
}
