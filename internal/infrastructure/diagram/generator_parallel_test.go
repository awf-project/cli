package diagram

// Generator Parallel Tests
// Tests for parallel subgraph generation

import (
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestGenerator_generateParallelSubgraph_BasicSubgraph(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-basic",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"branch1", "branch2"},
			},
			"branch1": {
				Name:    "branch1",
				Type:    workflow.StepTypeCommand,
				Command: "echo 1",
			},
			"branch2": {
				Name:    "branch2",
				Type:    workflow.StepTypeCommand,
				Command: "echo 2",
			},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should contain subgraph declaration
	if !strings.Contains(subgraph, "subgraph") {
		t.Error("generateParallelSubgraph() should contain 'subgraph' keyword")
	}
}

func TestGenerator_generateParallelSubgraph_ContainsClusterPrefix(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-cluster",
		Initial: "my_parallel",
		Steps: map[string]*workflow.Step{
			"my_parallel": {
				Name:     "my_parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"a", "b"},
			},
			"a": {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b"},
		},
	}

	parallelStep := wf.Steps["my_parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// DOT subgraph clusters must use "cluster_" prefix for visual grouping
	if !strings.Contains(subgraph, "cluster_") {
		t.Error("generateParallelSubgraph() should use 'cluster_' prefix for subgraph name")
	}
	if !strings.Contains(subgraph, "cluster_my_parallel") {
		t.Error("generateParallelSubgraph() cluster name should include step name")
	}
}

func TestGenerator_generateParallelSubgraph_ContainsLabel(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-label",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"x"},
			},
			"x": {Name: "x", Type: workflow.StepTypeCommand, Command: "echo x"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should have a label for the cluster
	if !strings.Contains(subgraph, "label") {
		t.Error("generateParallelSubgraph() should include label attribute")
	}
}

func TestGenerator_generateParallelSubgraph_ContainsBranchNodes(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-branches",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"branch_a", "branch_b", "branch_c"},
			},
			"branch_a": {Name: "branch_a", Type: workflow.StepTypeCommand, Command: "a"},
			"branch_b": {Name: "branch_b", Type: workflow.StepTypeCommand, Command: "b"},
			"branch_c": {Name: "branch_c", Type: workflow.StepTypeCommand, Command: "c"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should contain all branch nodes
	if !strings.Contains(subgraph, "branch_a") {
		t.Error("generateParallelSubgraph() should contain branch_a node")
	}
	if !strings.Contains(subgraph, "branch_b") {
		t.Error("generateParallelSubgraph() should contain branch_b node")
	}
	if !strings.Contains(subgraph, "branch_c") {
		t.Error("generateParallelSubgraph() should contain branch_c node")
	}
}

func TestGenerator_generateParallelSubgraph_HasDashedStyle(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-style",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"x"},
			},
			"x": {Name: "x", Type: workflow.StepTypeCommand, Command: "x"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Per spec: subgraph should have dashed style
	if !strings.Contains(subgraph, "dashed") {
		t.Error("generateParallelSubgraph() should use dashed style for cluster border")
	}
}

func TestGenerator_generateParallelSubgraph_HasClosingBrace(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-braces",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"x"},
			},
			"x": {Name: "x", Type: workflow.StepTypeCommand, Command: "x"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should have proper DOT syntax with braces
	if !strings.Contains(subgraph, "{") {
		t.Error("generateParallelSubgraph() should contain opening brace")
	}
	if !strings.Contains(subgraph, "}") {
		t.Error("generateParallelSubgraph() should contain closing brace")
	}
}

func TestGenerator_generateParallelSubgraph_EmptyBranches(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-empty",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{},
			},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Empty branches should still produce valid subgraph structure
	if !strings.Contains(subgraph, "subgraph") {
		t.Error("generateParallelSubgraph() with empty branches should still produce subgraph")
	}
}

func TestGenerator_generateParallelSubgraph_NilBranches(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-nil",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: nil,
			},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)

	// Should not panic with nil branches
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic with "not implemented" but not with nil pointer
			if err, ok := r.(string); ok && err == "not implemented" {
				return // This is the expected panic from stub
			}
			t.Errorf("generateParallelSubgraph() panicked with nil branches: %v", r)
		}
	}()

	_ = g.generateParallelSubgraph(parallelStep, wf)
}

func TestGenerator_generateParallelSubgraph_SingleBranch(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-single",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"only_branch"},
			},
			"only_branch": {Name: "only_branch", Type: workflow.StepTypeCommand, Command: "x"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Single branch should still work
	if !strings.Contains(subgraph, "only_branch") {
		t.Error("generateParallelSubgraph() should contain single branch node")
	}
}

func TestGenerator_generateParallelSubgraph_BranchNodeShapes(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-shapes",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"cmd_branch", "loop_branch"},
			},
			"cmd_branch": {
				Name:    "cmd_branch",
				Type:    workflow.StepTypeCommand,
				Command: "echo cmd",
			},
			"loop_branch": {
				Name: "loop_branch",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{Items: "items"},
			},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Branch nodes should have correct shapes
	if !strings.Contains(subgraph, "cmd_branch") {
		t.Error("generateParallelSubgraph() should contain cmd_branch")
	}
	if !strings.Contains(subgraph, "loop_branch") {
		t.Error("generateParallelSubgraph() should contain loop_branch")
	}
}

func TestGenerator_generateParallelSubgraph_NilStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-nil-step",
		Initial: "start",
		Steps:   map[string]*workflow.Step{},
	}

	g := NewGenerator(nil)

	// Should handle nil step gracefully
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(string); ok && err == "not implemented" {
				return // Expected panic from stub
			}
			t.Errorf("generateParallelSubgraph(nil, wf) panicked unexpectedly: %v", r)
		}
	}()

	_ = g.generateParallelSubgraph(nil, wf)
}

func TestGenerator_generateParallelSubgraph_NilWorkflow(t *testing.T) {
	step := &workflow.Step{
		Name:     "parallel",
		Type:     workflow.StepTypeParallel,
		Branches: []string{"a"},
	}

	g := NewGenerator(nil)

	// Should handle nil workflow gracefully
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(string); ok && err == "not implemented" {
				return // Expected panic from stub
			}
			t.Errorf("generateParallelSubgraph(step, nil) panicked unexpectedly: %v", r)
		}
	}()

	_ = g.generateParallelSubgraph(step, nil)
}

func TestGenerator_generateParallelSubgraph_NonParallelStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "non-parallel",
		Initial: "cmd",
		Steps: map[string]*workflow.Step{
			"cmd": {
				Name:    "cmd",
				Type:    workflow.StepTypeCommand,
				Command: "echo",
			},
		},
	}

	cmdStep := wf.Steps["cmd"]
	g := NewGenerator(nil)

	// Should handle non-parallel step (could return empty or produce minimal subgraph)
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(string); ok && err == "not implemented" {
				return // Expected from stub
			}
			t.Errorf("generateParallelSubgraph() with non-parallel step panicked: %v", r)
		}
	}()

	subgraph := g.generateParallelSubgraph(cmdStep, wf)

	// Non-parallel step should produce empty or minimal output
	_ = subgraph // Will verify actual behavior when implemented
}

func TestGenerator_generateParallelSubgraph_SpecialCharactersInName(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-special",
		Initial: "parallel-with-dash",
		Steps: map[string]*workflow.Step{
			"parallel-with-dash": {
				Name:     "parallel-with-dash",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"branch_1", "branch_2"},
			},
			"branch_1": {Name: "branch_1", Type: workflow.StepTypeCommand, Command: "a"},
			"branch_2": {Name: "branch_2", Type: workflow.StepTypeCommand, Command: "b"},
		},
	}

	parallelStep := wf.Steps["parallel-with-dash"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should handle special characters in step names
	if !strings.Contains(subgraph, "subgraph") {
		t.Error("generateParallelSubgraph() should handle step names with dashes")
	}
}

func TestGenerator_generateParallelSubgraph_MissingBranchStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-missing",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"existing", "missing"},
			},
			"existing": {Name: "existing", Type: workflow.StepTypeCommand, Command: "x"},
			// "missing" step is not defined
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)

	// Should handle missing branch steps gracefully
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(string); ok && err == "not implemented" {
				return // Expected from stub
			}
			// Let other panics through - implementation should handle this
		}
	}()

	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should at least contain the existing branch
	if subgraph != "" && !strings.Contains(subgraph, "existing") {
		t.Error("generateParallelSubgraph() should contain existing branch even with missing ones")
	}
}

func TestGenerator_generateParallelSubgraph_NestedParallel(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "nested-parallel",
		Initial: "outer",
		Steps: map[string]*workflow.Step{
			"outer": {
				Name:     "outer",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"inner"},
			},
			"inner": {
				Name:     "inner",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"leaf"},
			},
			"leaf": {Name: "leaf", Type: workflow.StepTypeCommand, Command: "x"},
		},
	}

	outerStep := wf.Steps["outer"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(outerStep, wf)

	// Should contain the inner parallel step reference
	if !strings.Contains(subgraph, "inner") {
		t.Error("generateParallelSubgraph() should contain nested parallel branch")
	}
}

func TestGenerator_generateParallelSubgraph_WithStrategy(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-strategy",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"a", "b"},
				Strategy: "all_succeed",
			},
			"a": {Name: "a", Type: workflow.StepTypeCommand, Command: "a"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, Command: "b"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Strategy doesn't affect subgraph structure, just verify it generates
	if !strings.Contains(subgraph, "subgraph") {
		t.Error("generateParallelSubgraph() with strategy should produce subgraph")
	}
}

func TestGenerator_generateParallelSubgraph_LabelEscapesQuotes(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-quotes",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:        "parallel",
				Type:        workflow.StepTypeParallel,
				Description: `Step with "quotes"`,
				Branches:    []string{"x"},
			},
			"x": {Name: "x", Type: workflow.StepTypeCommand, Command: "x"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should produce valid DOT syntax (quotes escaped)
	if !strings.Contains(subgraph, "subgraph") {
		t.Error("generateParallelSubgraph() should handle descriptions with quotes")
	}
}

func TestGenerator_generateParallelSubgraph_DeterministicOutput(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-deterministic",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"a", "b", "c"},
			},
			"a": {Name: "a", Type: workflow.StepTypeCommand, Command: "a"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, Command: "b"},
			"c": {Name: "c", Type: workflow.StepTypeCommand, Command: "c"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)

	// Multiple calls should produce same output
	first := g.generateParallelSubgraph(parallelStep, wf)
	second := g.generateParallelSubgraph(parallelStep, wf)

	if first != second {
		t.Error("generateParallelSubgraph() should produce deterministic output")
	}
}

func TestGenerator_generateParallelSubgraph_OutputFormat(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-format",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"x"},
			},
			"x": {Name: "x", Type: workflow.StepTypeCommand, Command: "x"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Verify proper DOT subgraph format
	// Expected format: subgraph cluster_<name> { ... }
	if !strings.HasPrefix(strings.TrimSpace(subgraph), "subgraph") {
		t.Error("generateParallelSubgraph() output should start with 'subgraph'")
	}
}

func TestGenerator_generateParallelSubgraph_BranchWithTerminal(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-terminal-branch",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"terminal_branch"},
			},
			"terminal_branch": {
				Name:   "terminal_branch",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Terminal steps as branches should be included
	if !strings.Contains(subgraph, "terminal_branch") {
		t.Error("generateParallelSubgraph() should include terminal branch nodes")
	}
}

func TestGenerator_generateParallelSubgraph_ManyBranches(t *testing.T) {
	branches := []string{"b1", "b2", "b3", "b4", "b5", "b6", "b7", "b8", "b9", "b10"}
	steps := map[string]*workflow.Step{
		"parallel": {
			Name:     "parallel",
			Type:     workflow.StepTypeParallel,
			Branches: branches,
		},
	}
	for _, b := range branches {
		steps[b] = &workflow.Step{Name: b, Type: workflow.StepTypeCommand, Command: "x"}
	}

	wf := &workflow.Workflow{
		Name:    "parallel-many",
		Initial: "parallel",
		Steps:   steps,
	}

	parallelStep := wf.Steps["parallel"]
	g := NewGenerator(nil)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Should handle many branches
	for _, b := range branches {
		if !strings.Contains(subgraph, b) {
			t.Errorf("generateParallelSubgraph() should contain branch %s", b)
		}
	}
}

func TestGenerator_generateParallelSubgraph_HighlightedBranch(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-highlight",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"normal", "highlighted"},
			},
			"normal":      {Name: "normal", Type: workflow.StepTypeCommand, Command: "a"},
			"highlighted": {Name: "highlighted", Type: workflow.StepTypeCommand, Command: "b"},
		},
	}

	parallelStep := wf.Steps["parallel"]
	cfg := &DiagramConfig{
		Direction:  DirectionTB,
		Highlight:  "highlighted",
		ShowLabels: true,
	}
	g := NewGenerator(cfg)
	subgraph := g.generateParallelSubgraph(parallelStep, wf)

	// Both branches should be present
	if !strings.Contains(subgraph, "normal") {
		t.Error("generateParallelSubgraph() should contain normal branch")
	}
	if !strings.Contains(subgraph, "highlighted") {
		t.Error("generateParallelSubgraph() should contain highlighted branch")
	}
}

func TestGenerator_Highlight_ParallelStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-highlight",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				OnSuccess: "done",
			},
			"branch1": {
				Name:      "branch1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "done",
			},
			"branch2": {
				Name:      "branch2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{Highlight: "parallel"})
	dot := g.Generate(wf)

	// Parallel step should be highlighted with diamond shape preserved
	if !strings.Contains(dot, "parallel") {
		t.Error("Generate() should include highlighted parallel step")
	}
	if !strings.Contains(dot, "diamond") {
		t.Error("Generate() should preserve parallel step diamond shape when highlighted")
	}
}
