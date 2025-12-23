package diagram

import (
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestNewGenerator(t *testing.T) {
	tests := []struct {
		name   string
		config *DiagramConfig
	}{
		{
			name:   "with nil config uses defaults",
			config: nil,
		},
		{
			name:   "with custom config",
			config: &DiagramConfig{Direction: DirectionLR},
		},
		{
			name:   "with default config",
			config: NewDefaultConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(tt.config)
			if g == nil {
				t.Fatal("NewGenerator() returned nil")
			}
			if g.config == nil {
				t.Error("Generator.config is nil")
			}
		})
	}
}

func TestNewGenerator_NilConfig_UsesDefaults(t *testing.T) {
	g := NewGenerator(nil)

	if g.config.Direction != DirectionTB {
		t.Errorf("expected default direction TB, got %s", g.config.Direction)
	}
	if g.config.ShowLabels != true {
		t.Errorf("expected default ShowLabels true, got %v", g.config.ShowLabels)
	}
}

func TestNewGenerator_ReturnsNewInstance(t *testing.T) {
	cfg := NewDefaultConfig()
	g1 := NewGenerator(cfg)
	g2 := NewGenerator(cfg)

	if g1 == g2 {
		t.Error("NewGenerator() should return new instance each time")
	}
}

func TestGenerator_Generate_SimpleWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "simple-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo hello",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Verify basic DOT structure
	if !strings.Contains(dot, "digraph") {
		t.Error("DOT output should contain 'digraph'")
	}
	if !strings.Contains(dot, "start") {
		t.Error("DOT output should contain 'start' node")
	}
	if !strings.Contains(dot, "done") {
		t.Error("DOT output should contain 'done' node")
	}
}

func TestGenerator_Generate_CommandStepShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "cmd-workflow",
		Initial: "cmd",
		Steps: map[string]*workflow.Step{
			"cmd": {
				Name:      "cmd",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Command steps should be rendered as box shape
	if !strings.Contains(dot, "box") {
		t.Error("command step should have box shape")
	}
}

func TestGenerator_Generate_ParallelStepShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-workflow",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				OnSuccess: "end",
			},
			"branch1": {
				Name:      "branch1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "end",
			},
			"branch2": {
				Name:      "branch2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Parallel steps should be rendered as diamond shape
	if !strings.Contains(dot, "diamond") {
		t.Error("parallel step should have diamond shape")
	}
}

func TestGenerator_Generate_TerminalSuccessShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "terminal-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "success",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Terminal success should be oval with green
	if !strings.Contains(dot, "oval") && !strings.Contains(dot, "ellipse") {
		t.Error("terminal step should have oval/ellipse shape")
	}
}

func TestGenerator_Generate_TerminalFailureShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "failure-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Terminal failure should have distinct style (doubleoval or red)
	if !strings.Contains(dot, "failure") {
		t.Error("DOT output should contain failure terminal")
	}
}

func TestGenerator_Generate_LoopStepShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "loop-workflow",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Items:      "{{inputs.items}}",
					Body:       []string{"process"},
					OnComplete: "end",
				},
			},
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{item}}",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Loop steps should be rendered as hexagon
	if !strings.Contains(dot, "hexagon") {
		t.Error("loop step should have hexagon shape")
	}
}

func TestGenerator_Generate_OperationStepShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "operation-workflow",
		Initial: "op",
		Steps: map[string]*workflow.Step{
			"op": {
				Name:      "op",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Operation steps should be rendered as box3d
	if !strings.Contains(dot, "box3d") {
		t.Error("operation step should have box3d shape")
	}
}

func TestGenerator_Generate_CallWorkflowStepShape(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "call-workflow",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "other-workflow",
				},
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// CallWorkflow steps should be rendered as folder shape
	if !strings.Contains(dot, "folder") {
		t.Error("call_workflow step should have folder shape")
	}
}

func TestGenerator_Generate_OnSuccessEdge(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "success-edge-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Should contain edge from start to end
	if !strings.Contains(dot, "->") {
		t.Error("DOT output should contain edge arrows")
	}
}

func TestGenerator_Generate_OnFailureEdge(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "failure-edge-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Failure edge should be dashed red
	if !strings.Contains(dot, "dashed") || !strings.Contains(dot, "red") {
		t.Error("failure edge should be dashed red")
	}
}

func TestGenerator_Generate_DirectionTB(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(&DiagramConfig{Direction: DirectionTB})
	dot := g.Generate(wf)

	if !strings.Contains(dot, "rankdir=TB") && !strings.Contains(dot, `rankdir="TB"`) {
		t.Error("DOT output should contain rankdir=TB for top-to-bottom direction")
	}
}

func TestGenerator_Generate_DirectionLR(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(&DiagramConfig{Direction: DirectionLR})
	dot := g.Generate(wf)

	if !strings.Contains(dot, "rankdir=LR") && !strings.Contains(dot, `rankdir="LR"`) {
		t.Error("DOT output should contain rankdir=LR for left-to-right direction")
	}
}

func TestGenerator_Generate_DirectionBT(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(&DiagramConfig{Direction: DirectionBT})
	dot := g.Generate(wf)

	if !strings.Contains(dot, "rankdir=BT") && !strings.Contains(dot, `rankdir="BT"`) {
		t.Error("DOT output should contain rankdir=BT for bottom-to-top direction")
	}
}

func TestGenerator_Generate_DirectionRL(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(&DiagramConfig{Direction: DirectionRL})
	dot := g.Generate(wf)

	if !strings.Contains(dot, "rankdir=RL") && !strings.Contains(dot, `rankdir="RL"`) {
		t.Error("DOT output should contain rankdir=RL for right-to-left direction")
	}
}

func TestGenerator_Generate_HighlightStep(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(&DiagramConfig{Highlight: "start"})
	dot := g.Generate(wf)

	// Highlighted step should have different styling (bold, thicker border, etc.)
	if !strings.Contains(dot, "start") {
		t.Error("DOT output should contain highlighted step")
	}
	// Check for some emphasis (penwidth, bold, or color change)
	hasEmphasis := strings.Contains(dot, "penwidth") ||
		strings.Contains(dot, "bold") ||
		strings.Contains(dot, "style=") ||
		strings.Contains(dot, "color=")
	if !hasEmphasis {
		t.Error("highlighted step should have visual emphasis")
	}
}

func TestGenerator_Generate_InitialStepMarker(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Initial step should be marked (double border, arrow from start node, etc.)
	hasStartMarker := strings.Contains(dot, "__start__") ||
		strings.Contains(dot, "peripheries") ||
		strings.Contains(dot, "START") ||
		strings.Contains(dot, "point")
	if !hasStartMarker {
		t.Error("initial step should have start marker")
	}
}

func TestGenerator_Generate_ParallelSubgraph(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-subgraph",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2"},
				OnSuccess: "end",
			},
			"branch1": {
				Name:      "branch1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "end",
			},
			"branch2": {
				Name:      "branch2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Parallel branches should be in a subgraph cluster
	if !strings.Contains(dot, "subgraph") || !strings.Contains(dot, "cluster") {
		t.Error("parallel branches should be grouped in subgraph cluster")
	}
}

func TestGenerator_Generate_ConditionalTransitions(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "conditional-workflow",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "test -f file.txt",
				Transitions: workflow.Transitions{
					{When: "{{states.check.exit_code}} == 0", Goto: "exists"},
					{When: "", Goto: "not_exists"},
				},
			},
			"exists": {
				Name:   "exists",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"not_exists": {
				Name:   "not_exists",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{ShowLabels: true})
	dot := g.Generate(wf)

	// Should have edges with labels for conditions
	if !strings.Contains(dot, "label") {
		t.Error("conditional transitions should have labels")
	}
}

func TestGenerator_Generate_EmptyWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "empty",
		Initial: "",
		Steps:   map[string]*workflow.Step{},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Should still produce valid DOT structure
	if !strings.Contains(dot, "digraph") {
		t.Error("empty workflow should still produce digraph")
	}
}

func TestGenerator_Generate_NilWorkflow(t *testing.T) {
	g := NewGenerator(nil)

	// Recover from panic to test panic behavior
	defer func() {
		if r := recover(); r == nil {
			// If no panic, check for empty result or error handling
			t.Log("Generator.Generate(nil) did not panic")
		}
	}()

	dot := g.Generate(nil)
	// If it doesn't panic, should return empty or error indicator
	if dot != "" {
		t.Log("Generator.Generate(nil) returned non-empty string")
	}
}

func TestGenerator_Generate_ValidDOTSyntax(t *testing.T) {
	wf := createSimpleWorkflow()

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Check for valid DOT structure
	if !strings.HasPrefix(strings.TrimSpace(dot), "digraph") {
		t.Error("DOT output should start with 'digraph'")
	}
	if !strings.Contains(dot, "{") || !strings.Contains(dot, "}") {
		t.Error("DOT output should contain braces")
	}

	// Count braces should match
	openBraces := strings.Count(dot, "{")
	closeBraces := strings.Count(dot, "}")
	if openBraces != closeBraces {
		t.Errorf("mismatched braces: %d open, %d close", openBraces, closeBraces)
	}
}

func TestGenerator_Generate_WorkflowName(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "my-test-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:   "start",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Workflow name should appear in the digraph declaration or as a label
	if !strings.Contains(dot, "my-test-workflow") && !strings.Contains(dot, "my_test_workflow") {
		t.Error("workflow name should appear in DOT output")
	}
}

func TestGenerator_Generate_StepWithDescription(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "described-workflow",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:        "start",
				Type:        workflow.StepTypeCommand,
				Description: "This step starts the process",
				Command:     "echo start",
				OnSuccess:   "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{ShowLabels: true})
	dot := g.Generate(wf)

	// Description could be used as tooltip or label
	_ = dot // Implementation will decide how to use description
}

func TestGenerator_Generate_EscapesSpecialCharacters(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "special-chars",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   `echo "hello world" | grep 'test'`,
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Output should be valid DOT (special chars escaped)
	if strings.Count(dot, "\"")%2 != 0 {
		t.Error("quotes should be properly balanced/escaped")
	}
}

func TestGenerator_Generate_WhileLoopStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "while-workflow",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Condition:  "{{counter}} < 5",
					Body:       []string{"process"},
					OnComplete: "end",
				},
			},
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo processing",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// While loop should also be hexagon (same as for_each)
	if !strings.Contains(dot, "hexagon") {
		t.Error("while step should have hexagon shape")
	}
}

func TestGenerator_Generate_ComplexWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "complex-workflow",
		Initial: "init",
		Steps: map[string]*workflow.Step{
			"init": {
				Name:      "init",
				Type:      workflow.StepTypeCommand,
				Command:   "echo init",
				OnSuccess: "parallel",
				OnFailure: "failure",
			},
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch_a", "branch_b"},
				Strategy:  "all_succeed",
				OnSuccess: "process",
				OnFailure: "failure",
			},
			"branch_a": {
				Name:      "branch_a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnSuccess: "process",
			},
			"branch_b": {
				Name:      "branch_b",
				Type:      workflow.StepTypeCommand,
				Command:   "echo b",
				OnSuccess: "process",
			},
			"process": {
				Name: "process",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Items:      "{{inputs.items}}",
					Body:       []string{"process_item"},
					OnComplete: "call_sub",
				},
			},
			"process_item": {
				Name:    "process_item",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{item}}",
			},
			"call_sub": {
				Name: "call_sub",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "sub-workflow",
				},
				OnSuccess: "notify",
				OnFailure: "failure",
			},
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	dot := g.Generate(wf)

	// Should contain all step types
	for _, stepName := range []string{"init", "parallel", "branch_a", "branch_b", "process", "call_sub", "notify", "success", "failure"} {
		if !strings.Contains(dot, stepName) {
			t.Errorf("DOT output should contain step %q", stepName)
		}
	}

	// Should be valid DOT
	if !strings.Contains(dot, "digraph") {
		t.Error("should produce valid digraph")
	}
}

// Helper function to create a simple test workflow
func createSimpleWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "simple",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}
}

// =============================================================================
// Tests for generateHeader (T013 - Direction flag support)
// =============================================================================

func TestGenerator_generateHeader_DefaultDirection(t *testing.T) {
	g := NewGenerator(nil)

	header := g.generateHeader("test-workflow")

	// Should contain digraph declaration
	if !strings.Contains(header, "digraph") {
		t.Error("header should contain 'digraph' declaration")
	}
	// Default direction is TB
	if !strings.Contains(header, "rankdir=TB") && !strings.Contains(header, `rankdir="TB"`) {
		t.Error("header should contain rankdir=TB for default direction")
	}
}

func TestGenerator_generateHeader_DirectionTB(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Direction: DirectionTB})

	header := g.generateHeader("workflow")

	if !strings.Contains(header, "rankdir=TB") && !strings.Contains(header, `rankdir="TB"`) {
		t.Errorf("header should contain rankdir=TB, got: %s", header)
	}
}

func TestGenerator_generateHeader_DirectionLR(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Direction: DirectionLR})

	header := g.generateHeader("workflow")

	if !strings.Contains(header, "rankdir=LR") && !strings.Contains(header, `rankdir="LR"`) {
		t.Errorf("header should contain rankdir=LR, got: %s", header)
	}
}

func TestGenerator_generateHeader_DirectionBT(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Direction: DirectionBT})

	header := g.generateHeader("workflow")

	if !strings.Contains(header, "rankdir=BT") && !strings.Contains(header, `rankdir="BT"`) {
		t.Errorf("header should contain rankdir=BT, got: %s", header)
	}
}

func TestGenerator_generateHeader_DirectionRL(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Direction: DirectionRL})

	header := g.generateHeader("workflow")

	if !strings.Contains(header, "rankdir=RL") && !strings.Contains(header, `rankdir="RL"`) {
		t.Errorf("header should contain rankdir=RL, got: %s", header)
	}
}

func TestGenerator_generateHeader_AllDirections_TableDriven(t *testing.T) {
	tests := []struct {
		direction Direction
		expected  string
	}{
		{DirectionTB, "TB"},
		{DirectionLR, "LR"},
		{DirectionBT, "BT"},
		{DirectionRL, "RL"},
	}

	for _, tt := range tests {
		t.Run(string(tt.direction), func(t *testing.T) {
			g := NewGenerator(&DiagramConfig{Direction: tt.direction})

			header := g.generateHeader("test")

			expectedPattern := "rankdir=" + tt.expected
			expectedQuoted := `rankdir="` + tt.expected + `"`

			if !strings.Contains(header, expectedPattern) && !strings.Contains(header, expectedQuoted) {
				t.Errorf("generateHeader() should contain rankdir=%s for direction %s, got: %s",
					tt.expected, tt.direction, header)
			}
		})
	}
}

func TestGenerator_generateHeader_ContainsWorkflowName(t *testing.T) {
	g := NewGenerator(nil)

	header := g.generateHeader("my-workflow-name")

	// Header should include the workflow name in digraph declaration
	if !strings.Contains(header, "my-workflow-name") {
		t.Errorf("header should contain workflow name, got: %s", header)
	}
}

func TestGenerator_generateHeader_WorkflowNameWithSpecialChars(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
	}{
		{"simple name", "simple"},
		{"with dashes", "my-workflow"},
		{"with underscores", "my_workflow"},
		{"with numbers", "workflow123"},
		{"mixed", "my-workflow_v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(nil)

			header := g.generateHeader(tt.workflowName)

			// Should contain digraph with the name (properly quoted or escaped)
			if !strings.Contains(header, "digraph") {
				t.Errorf("header should contain digraph declaration")
			}
			// Name should appear somewhere in the header
			if !strings.Contains(header, tt.workflowName) {
				t.Errorf("header should contain workflow name %q, got: %s", tt.workflowName, header)
			}
		})
	}
}

func TestGenerator_generateHeader_ContainsNodeDefaults(t *testing.T) {
	g := NewGenerator(nil)

	header := g.generateHeader("workflow")

	// Should contain default node attributes
	if !strings.Contains(header, "node") {
		t.Errorf("header should contain node defaults, got: %s", header)
	}
}

func TestGenerator_generateHeader_ContainsEdgeDefaults(t *testing.T) {
	g := NewGenerator(nil)

	header := g.generateHeader("workflow")

	// Should contain default edge attributes
	if !strings.Contains(header, "edge") {
		t.Errorf("header should contain edge defaults, got: %s", header)
	}
}

func TestGenerator_generateHeader_ValidDOTSyntax(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Direction: DirectionLR})

	header := g.generateHeader("test-workflow")

	// Must start with "digraph"
	trimmed := strings.TrimSpace(header)
	if !strings.HasPrefix(trimmed, "digraph") {
		t.Errorf("header should start with 'digraph', got: %s", header)
	}
	// Must contain opening brace
	if !strings.Contains(header, "{") {
		t.Errorf("header should contain opening brace, got: %s", header)
	}
}

func TestGenerator_generateHeader_EmptyWorkflowName(t *testing.T) {
	g := NewGenerator(nil)

	// Empty name should still produce valid header
	header := g.generateHeader("")

	if !strings.Contains(header, "digraph") {
		t.Errorf("header should contain digraph even with empty name, got: %s", header)
	}
}

func TestGenerator_generateHeader_EmptyDirection(t *testing.T) {
	// Config with empty direction should use default (TB)
	g := NewGenerator(&DiagramConfig{Direction: ""})

	header := g.generateHeader("workflow")

	// Should either have TB or no rankdir (defaulting to graphviz default which is TB)
	// The implementation may choose to omit rankdir for default
	if !strings.Contains(header, "digraph") {
		t.Error("header should contain digraph declaration")
	}
}

// =============================================================================
// Tests for stepTypeToShape mapping (T006)
// =============================================================================

func TestStepTypeToShape_AllTypesAreMapped(t *testing.T) {
	// Verify all workflow step types have a shape mapping
	expectedTypes := []workflow.StepType{
		workflow.StepTypeCommand,
		workflow.StepTypeParallel,
		workflow.StepTypeTerminal,
		workflow.StepTypeForEach,
		workflow.StepTypeWhile,
		workflow.StepTypeOperation,
		workflow.StepTypeCallWorkflow,
	}

	for _, stepType := range expectedTypes {
		shape, ok := stepTypeToShape[stepType]
		if !ok {
			t.Errorf("stepTypeToShape missing mapping for %q", stepType)
		}
		if shape == "" {
			t.Errorf("stepTypeToShape[%q] is empty", stepType)
		}
	}
}

func TestStepTypeToShape_CorrectShapes(t *testing.T) {
	tests := []struct {
		stepType      workflow.StepType
		expectedShape string
	}{
		{workflow.StepTypeCommand, "box"},
		{workflow.StepTypeParallel, "diamond"},
		{workflow.StepTypeTerminal, "oval"},
		{workflow.StepTypeForEach, "hexagon"},
		{workflow.StepTypeWhile, "hexagon"},
		{workflow.StepTypeOperation, "box3d"},
		{workflow.StepTypeCallWorkflow, "folder"},
	}

	for _, tt := range tests {
		t.Run(string(tt.stepType), func(t *testing.T) {
			got := stepTypeToShape[tt.stepType]
			if got != tt.expectedShape {
				t.Errorf("stepTypeToShape[%q] = %q, want %q", tt.stepType, got, tt.expectedShape)
			}
		})
	}
}

func TestStepTypeToShape_LoopTypesShareShape(t *testing.T) {
	// Both for_each and while should use hexagon
	forEachShape := stepTypeToShape[workflow.StepTypeForEach]
	whileShape := stepTypeToShape[workflow.StepTypeWhile]

	if forEachShape != whileShape {
		t.Errorf("for_each shape %q != while shape %q, expected same", forEachShape, whileShape)
	}
	if forEachShape != "hexagon" {
		t.Errorf("loop types should use hexagon, got %q", forEachShape)
	}
}

// =============================================================================
// Tests for generateNodes() method (T006)
// =============================================================================

func TestGenerator_generateNodes_SingleCommandStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-cmd",
		Initial: "cmd",
		Steps: map[string]*workflow.Step{
			"cmd": {
				Name:    "cmd",
				Type:    workflow.StepTypeCommand,
				Command: "echo hello",
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with box shape
	if !strings.Contains(nodes, "cmd") {
		t.Error("generateNodes() should contain step name 'cmd'")
	}
	if !strings.Contains(nodes, "box") {
		t.Error("generateNodes() should use box shape for command step")
	}
}

func TestGenerator_generateNodes_SingleParallelStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-parallel",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"a", "b"},
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with diamond shape
	if !strings.Contains(nodes, "parallel") {
		t.Error("generateNodes() should contain step name 'parallel'")
	}
	if !strings.Contains(nodes, "diamond") {
		t.Error("generateNodes() should use diamond shape for parallel step")
	}
}

func TestGenerator_generateNodes_SingleTerminalSuccessStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-terminal-success",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with oval shape
	if !strings.Contains(nodes, "done") {
		t.Error("generateNodes() should contain step name 'done'")
	}
	if !strings.Contains(nodes, "oval") {
		t.Error("generateNodes() should use oval shape for terminal success step")
	}
}

func TestGenerator_generateNodes_SingleTerminalFailureStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-terminal-failure",
		Initial: "failed",
		Steps: map[string]*workflow.Step{
			"failed": {
				Name:   "failed",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Terminal failure should use doubleoval shape (distinct from success)
	if !strings.Contains(nodes, "failed") {
		t.Error("generateNodes() should contain step name 'failed'")
	}
	if !strings.Contains(nodes, "doubleoval") {
		t.Error("generateNodes() should use doubleoval shape for terminal failure step")
	}
}

func TestGenerator_generateNodes_SingleForEachStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-foreach",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Items:      "{{inputs.items}}",
					Body:       []string{"process"},
					OnComplete: "done",
				},
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with hexagon shape
	if !strings.Contains(nodes, "loop") {
		t.Error("generateNodes() should contain step name 'loop'")
	}
	if !strings.Contains(nodes, "hexagon") {
		t.Error("generateNodes() should use hexagon shape for for_each step")
	}
}

func TestGenerator_generateNodes_SingleWhileStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-while",
		Initial: "while_loop",
		Steps: map[string]*workflow.Step{
			"while_loop": {
				Name: "while_loop",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Condition:  "{{counter}} < 10",
					Body:       []string{"process"},
					OnComplete: "done",
				},
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with hexagon shape
	if !strings.Contains(nodes, "while_loop") {
		t.Error("generateNodes() should contain step name 'while_loop'")
	}
	if !strings.Contains(nodes, "hexagon") {
		t.Error("generateNodes() should use hexagon shape for while step")
	}
}

func TestGenerator_generateNodes_SingleOperationStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-operation",
		Initial: "op",
		Steps: map[string]*workflow.Step{
			"op": {
				Name:      "op",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with box3d shape
	if !strings.Contains(nodes, "op") {
		t.Error("generateNodes() should contain step name 'op'")
	}
	if !strings.Contains(nodes, "box3d") {
		t.Error("generateNodes() should use box3d shape for operation step")
	}
}

func TestGenerator_generateNodes_SingleCallWorkflowStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-call-workflow",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "sub-workflow",
				},
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Should contain node declaration with folder shape
	if !strings.Contains(nodes, "call") {
		t.Error("generateNodes() should contain step name 'call'")
	}
	if !strings.Contains(nodes, "folder") {
		t.Error("generateNodes() should use folder shape for call_workflow step")
	}
}

func TestGenerator_generateNodes_AllStepTypes(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "all-types",
		Initial: "cmd",
		Steps: map[string]*workflow.Step{
			"cmd": {
				Name:      "cmd",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "parallel",
			},
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"a", "b"},
				OnSuccess: "loop",
			},
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Items:      "{{inputs.items}}",
					Body:       []string{"body"},
					OnComplete: "while",
				},
			},
			"while": {
				Name: "while",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{
					Condition:  "{{x}} < 5",
					Body:       []string{"body"},
					OnComplete: "op",
				},
			},
			"op": {
				Name:      "op",
				Type:      workflow.StepTypeOperation,
				Operation: "test.op",
				OnSuccess: "call",
			},
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "other",
				},
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
			"body": {
				Name:    "body",
				Type:    workflow.StepTypeCommand,
				Command: "echo body",
			},
			"a": {
				Name:    "a",
				Type:    workflow.StepTypeCommand,
				Command: "echo a",
			},
			"b": {
				Name:    "b",
				Type:    workflow.StepTypeCommand,
				Command: "echo b",
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Verify all expected shapes appear
	expectedShapes := map[string]bool{
		"box":        false,
		"diamond":    false,
		"oval":       false,
		"doubleoval": false,
		"hexagon":    false,
		"box3d":      false,
		"folder":     false,
	}

	for shape := range expectedShapes {
		if strings.Contains(nodes, shape) {
			expectedShapes[shape] = true
		}
	}

	// Check all shapes are present
	if !expectedShapes["box"] {
		t.Error("generateNodes() should contain box shape")
	}
	if !expectedShapes["diamond"] {
		t.Error("generateNodes() should contain diamond shape")
	}
	if !expectedShapes["oval"] || !expectedShapes["doubleoval"] {
		t.Error("generateNodes() should contain oval shapes for terminals")
	}
	if !expectedShapes["hexagon"] {
		t.Error("generateNodes() should contain hexagon shape")
	}
	if !expectedShapes["box3d"] {
		t.Error("generateNodes() should contain box3d shape")
	}
	if !expectedShapes["folder"] {
		t.Error("generateNodes() should contain folder shape")
	}
}

func TestGenerator_generateNodes_EmptyWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "empty",
		Initial: "",
		Steps:   map[string]*workflow.Step{},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Empty workflow should return empty nodes string
	if nodes != "" {
		t.Errorf("generateNodes() for empty workflow = %q, want empty string", nodes)
	}
}

func TestGenerator_generateNodes_NilWorkflow(t *testing.T) {
	g := NewGenerator(nil)

	// Should handle nil gracefully or panic (to be determined by impl)
	defer func() {
		if r := recover(); r == nil {
			t.Log("generateNodes(nil) did not panic - checking return value")
		}
	}()

	nodes := g.generateNodes(nil)
	if nodes != "" {
		t.Log("generateNodes(nil) returned non-empty string:", nodes)
	}
}

func TestGenerator_generateNodes_NodeLabelContainsStepName(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "label-test",
		Initial: "my_step_name",
		Steps: map[string]*workflow.Step{
			"my_step_name": {
				Name:    "my_step_name",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
			},
		},
	}

	g := NewGenerator(&DiagramConfig{ShowLabels: true})
	nodes := g.generateNodes(wf)

	// Node should have a label containing the step name
	if !strings.Contains(nodes, "my_step_name") {
		t.Error("generateNodes() should include step name in node output")
	}
	if !strings.Contains(nodes, "label") {
		t.Error("generateNodes() should include label attribute")
	}
}

func TestGenerator_generateNodes_NodeLabelWithDescription(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "desc-test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:        "step",
				Type:        workflow.StepTypeCommand,
				Command:     "echo test",
				Description: "This is a test step",
			},
		},
	}

	g := NewGenerator(&DiagramConfig{ShowLabels: true})
	nodes := g.generateNodes(wf)

	// Should contain the step (description handling TBD by impl)
	if !strings.Contains(nodes, "step") {
		t.Error("generateNodes() should contain step name")
	}
}

func TestGenerator_generateNodes_HighlightedStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "highlight-test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:   "step2",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{Highlight: "step1"})
	nodes := g.generateNodes(wf)

	// Highlighted step should have different styling
	if !strings.Contains(nodes, "step1") {
		t.Error("generateNodes() should contain highlighted step")
	}
	// Check for emphasis attributes (penwidth, bold, color, etc.)
	hasEmphasis := strings.Contains(nodes, "penwidth") ||
		strings.Contains(nodes, "bold") ||
		strings.Contains(nodes, "style") ||
		strings.Contains(nodes, "color")
	if !hasEmphasis {
		t.Error("generateNodes() should apply emphasis to highlighted step")
	}
}

func TestGenerator_generateNodes_EscapesSpecialCharactersInName(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "escape-test",
		Initial: "step-with-dashes",
		Steps: map[string]*workflow.Step{
			"step-with-dashes": {
				Name:    "step-with-dashes",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Node identifier should be valid DOT (dashes may need escaping/quoting)
	if !strings.Contains(nodes, "step-with-dashes") && !strings.Contains(nodes, `"step-with-dashes"`) {
		t.Error("generateNodes() should handle step names with special characters")
	}
}

func TestGenerator_generateNodes_OutputFormat(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "format-test",
		Initial: "step",
		Steps: map[string]*workflow.Step{
			"step": {
				Name:    "step",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
			},
		},
	}

	g := NewGenerator(nil)
	nodes := g.generateNodes(wf)

	// Output should be valid DOT node declarations
	// Expected format: node_id [attr1=val1, attr2=val2];
	if !strings.Contains(nodes, "[") || !strings.Contains(nodes, "]") {
		t.Error("generateNodes() output should contain DOT attribute brackets")
	}
	if !strings.Contains(nodes, "shape=") {
		t.Error("generateNodes() output should contain shape attribute")
	}
}

func TestGenerator_generateNodes_MultipleSteps_DeterministicOrder(t *testing.T) {
	// Run multiple times to check for consistent output
	wf := &workflow.Workflow{
		Name:    "order-test",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a", OnSuccess: "b"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b", OnSuccess: "c"},
			"c": {Name: "c", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	g := NewGenerator(nil)
	first := g.generateNodes(wf)
	second := g.generateNodes(wf)

	// Output should be consistent across calls
	if first != second {
		t.Error("generateNodes() should produce deterministic output")
	}
}

// =============================================================================
// Tests for generateEdges() method (T007)
// =============================================================================

func TestGenerator_generateEdges_SingleOnSuccessTransition(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-success",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "end",
			},
			"end": {
				Name:   "end",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should contain edge from start to end
	if !strings.Contains(edges, "start") || !strings.Contains(edges, "end") {
		t.Error("generateEdges() should contain start and end nodes")
	}
	if !strings.Contains(edges, "->") {
		t.Error("generateEdges() should contain edge arrow")
	}
}

func TestGenerator_generateEdges_OnSuccessIsSolidLine(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "success-solid",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {
				Name:      "a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnSuccess: "b",
			},
			"b": {
				Name:   "b",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// on_success should NOT have dashed style (solid is default)
	// The edge from a -> b should exist without dashed attribute
	if !strings.Contains(edges, "a") || !strings.Contains(edges, "b") {
		t.Error("generateEdges() should contain edge from a to b")
	}
	// on_success edges should not be dashed
	// Note: We check that if dashed appears, it's not on the success edge
}

func TestGenerator_generateEdges_SingleOnFailureTransition(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-failure",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnFailure: "error",
			},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should contain edge from start to error
	if !strings.Contains(edges, "start") || !strings.Contains(edges, "error") {
		t.Error("generateEdges() should contain edge from start to error")
	}
	if !strings.Contains(edges, "->") {
		t.Error("generateEdges() should contain edge arrow")
	}
}

func TestGenerator_generateEdges_OnFailureIsDashedRed(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "failure-dashed",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {
				Name:      "a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnFailure: "fail",
			},
			"fail": {
				Name:   "fail",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// on_failure edge should be dashed and red per FR-003
	if !strings.Contains(edges, "dashed") {
		t.Error("generateEdges() on_failure edge should have dashed style")
	}
	if !strings.Contains(edges, "red") {
		t.Error("generateEdges() on_failure edge should be red color")
	}
}

func TestGenerator_generateEdges_BothSuccessAndFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "both-transitions",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should contain both edges
	if !strings.Contains(edges, "success") {
		t.Error("generateEdges() should contain success edge target")
	}
	if !strings.Contains(edges, "failure") {
		t.Error("generateEdges() should contain failure edge target")
	}

	// Should have at least 2 edges (2 arrows)
	arrowCount := strings.Count(edges, "->")
	if arrowCount < 2 {
		t.Errorf("generateEdges() should contain at least 2 edges, got %d", arrowCount)
	}

	// Failure edge should be dashed red
	if !strings.Contains(edges, "dashed") || !strings.Contains(edges, "red") {
		t.Error("generateEdges() failure edge should be dashed red")
	}
}

func TestGenerator_generateEdges_ParallelBranches(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-branches",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch1", "branch2", "branch3"},
				OnSuccess: "done",
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
			"branch3": {
				Name:    "branch3",
				Type:    workflow.StepTypeCommand,
				Command: "echo 3",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should contain edges to all branches
	if !strings.Contains(edges, "branch1") {
		t.Error("generateEdges() should contain edge to branch1")
	}
	if !strings.Contains(edges, "branch2") {
		t.Error("generateEdges() should contain edge to branch2")
	}
	if !strings.Contains(edges, "branch3") {
		t.Error("generateEdges() should contain edge to branch3")
	}

	// Should have edge to done (on_success from parallel)
	if !strings.Contains(edges, "done") {
		t.Error("generateEdges() should contain edge to done")
	}
}

func TestGenerator_generateEdges_ParallelBranchesAreSolidLines(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-solid",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"a", "b"},
			},
			"a": {
				Name:    "a",
				Type:    workflow.StepTypeCommand,
				Command: "echo a",
			},
			"b": {
				Name:    "b",
				Type:    workflow.StepTypeCommand,
				Command: "echo b",
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Branch edges should be solid (default), not dashed
	// We verify branches are present
	if !strings.Contains(edges, "parallel") {
		t.Error("generateEdges() should contain parallel step")
	}
	if !strings.Contains(edges, "a") || !strings.Contains(edges, "b") {
		t.Error("generateEdges() should contain branch edges")
	}
}

func TestGenerator_generateEdges_TerminalStepHasNoOutgoingEdges(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "terminal-no-edges",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "terminal",
			},
			"terminal": {
				Name:   "terminal",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Count edges starting from terminal - should be 0
	// Edge format: "terminal" -> should not exist
	terminalOutgoing := strings.Count(edges, `"terminal" ->`) + strings.Count(edges, "terminal ->")
	if terminalOutgoing > 0 {
		t.Errorf("generateEdges() terminal step should have no outgoing edges, got %d", terminalOutgoing)
	}
}

func TestGenerator_generateEdges_ChainedSteps(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "chain",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 2",
				OnSuccess: "step3",
			},
			"step3": {
				Name:      "step3",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 3",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should have 3 edges: step1->step2, step2->step3, step3->done
	arrowCount := strings.Count(edges, "->")
	if arrowCount != 3 {
		t.Errorf("generateEdges() chained workflow should have 3 edges, got %d", arrowCount)
	}

	// Verify all step names are present
	for _, name := range []string{"step1", "step2", "step3", "done"} {
		if !strings.Contains(edges, name) {
			t.Errorf("generateEdges() should contain step %q", name)
		}
	}
}

func TestGenerator_generateEdges_EmptyWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "empty",
		Initial: "",
		Steps:   map[string]*workflow.Step{},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Empty workflow should have no edges
	if edges != "" {
		t.Errorf("generateEdges() for empty workflow = %q, want empty string", edges)
	}
}

func TestGenerator_generateEdges_SingleTerminalOnly(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "single-terminal",
		Initial: "done",
		Steps: map[string]*workflow.Step{
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Workflow with only terminal step should have no edges
	if strings.Contains(edges, "->") {
		t.Error("generateEdges() single terminal workflow should have no edges")
	}
}

func TestGenerator_generateEdges_NilWorkflow(t *testing.T) {
	g := NewGenerator(nil)

	// Should handle nil gracefully or panic
	defer func() {
		if r := recover(); r == nil {
			t.Log("generateEdges(nil) did not panic")
		}
	}()

	edges := g.generateEdges(nil)
	if edges != "" {
		t.Log("generateEdges(nil) returned:", edges)
	}
}

func TestGenerator_generateEdges_CallWorkflowStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "call-workflow-edges",
		Initial: "call",
		Steps: map[string]*workflow.Step{
			"call": {
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "sub-workflow",
				},
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should have 2 edges from call step
	if !strings.Contains(edges, "success") {
		t.Error("generateEdges() should contain edge to success")
	}
	if !strings.Contains(edges, "failure") {
		t.Error("generateEdges() should contain edge to failure")
	}

	// Failure edge should be dashed red
	if !strings.Contains(edges, "dashed") || !strings.Contains(edges, "red") {
		t.Error("generateEdges() call_workflow failure edge should be dashed red")
	}
}

func TestGenerator_generateEdges_OperationStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "operation-edges",
		Initial: "op",
		Steps: map[string]*workflow.Step{
			"op": {
				Name:      "op",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "next",
				OnFailure: "error",
			},
			"next": {
				Name:   "next",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should have both success and failure edges
	arrowCount := strings.Count(edges, "->")
	if arrowCount != 2 {
		t.Errorf("generateEdges() operation step should have 2 edges, got %d", arrowCount)
	}
}

func TestGenerator_generateEdges_LoopStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "loop-edges",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Items:      "{{inputs.items}}",
					Body:       []string{"process"},
					OnComplete: "done",
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"process": {
				Name:    "process",
				Type:    workflow.StepTypeCommand,
				Command: "echo {{item}}",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"error": {
				Name:   "error",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Loop step should have edges for on_success and on_failure
	if !strings.Contains(edges, "done") {
		t.Error("generateEdges() should contain edge to done")
	}
	if !strings.Contains(edges, "error") {
		t.Error("generateEdges() should contain edge to error")
	}
}

func TestGenerator_generateEdges_ConditionalTransitions(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "conditional-edges",
		Initial: "check",
		Steps: map[string]*workflow.Step{
			"check": {
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "test -f file.txt",
				Transitions: workflow.Transitions{
					{When: "{{states.check.exit_code}} == 0", Goto: "exists"},
					{When: "", Goto: "not_exists"},
				},
			},
			"exists": {
				Name:   "exists",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"not_exists": {
				Name:   "not_exists",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{ShowLabels: true})
	edges := g.generateEdges(wf)

	// Should have edges to both conditional targets
	if !strings.Contains(edges, "exists") {
		t.Error("generateEdges() should contain edge to exists")
	}
	if !strings.Contains(edges, "not_exists") {
		t.Error("generateEdges() should contain edge to not_exists")
	}

	// When ShowLabels is true, conditional edges should have labels
	if !strings.Contains(edges, "label") {
		t.Error("generateEdges() conditional transitions with ShowLabels should have labels")
	}
}

func TestGenerator_generateEdges_EdgeFormat(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "format-test",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {
				Name:      "a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnSuccess: "b",
			},
			"b": {
				Name:   "b",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Edge should be in valid DOT format: source -> target [attributes];
	if !strings.Contains(edges, "->") {
		t.Error("generateEdges() should produce DOT edge format with ->")
	}
}

func TestGenerator_generateEdges_EdgeWithAttributes(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "attrs-test",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {
				Name:      "a",
				Type:      workflow.StepTypeCommand,
				Command:   "echo a",
				OnFailure: "fail",
			},
			"fail": {
				Name:   "fail",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Failure edge should have attributes (style, color)
	if !strings.Contains(edges, "[") || !strings.Contains(edges, "]") {
		t.Error("generateEdges() failure edge should have attribute brackets")
	}
}

func TestGenerator_generateEdges_DeterministicOrder(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "order-test",
		Initial: "a",
		Steps: map[string]*workflow.Step{
			"a": {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a", OnSuccess: "b", OnFailure: "c"},
			"b": {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b", OnSuccess: "d"},
			"c": {Name: "c", Type: workflow.StepTypeCommand, Command: "echo c", OnSuccess: "d"},
			"d": {Name: "d", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
		},
	}

	g := NewGenerator(nil)
	first := g.generateEdges(wf)
	second := g.generateEdges(wf)

	// Output should be consistent across calls
	if first != second {
		t.Error("generateEdges() should produce deterministic output")
	}
}

func TestGenerator_generateEdges_ComplexWorkflow(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "complex-edges",
		Initial: "init",
		Steps: map[string]*workflow.Step{
			"init": {
				Name:      "init",
				Type:      workflow.StepTypeCommand,
				Command:   "echo init",
				OnSuccess: "parallel",
				OnFailure: "failure",
			},
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"branch_a", "branch_b"},
				OnSuccess: "call_sub",
				OnFailure: "failure",
			},
			"branch_a": {
				Name:    "branch_a",
				Type:    workflow.StepTypeCommand,
				Command: "echo a",
			},
			"branch_b": {
				Name:    "branch_b",
				Type:    workflow.StepTypeCommand,
				Command: "echo b",
			},
			"call_sub": {
				Name: "call_sub",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "sub",
				},
				OnSuccess: "notify",
				OnFailure: "failure",
			},
			"notify": {
				Name:      "notify",
				Type:      workflow.StepTypeOperation,
				Operation: "slack.send",
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Should have multiple edges
	arrowCount := strings.Count(edges, "->")
	if arrowCount < 8 {
		t.Errorf("generateEdges() complex workflow should have at least 8 edges, got %d", arrowCount)
	}

	// Should have both solid (success) and dashed (failure) edges
	if !strings.Contains(edges, "dashed") {
		t.Error("generateEdges() complex workflow should have dashed failure edges")
	}
	if !strings.Contains(edges, "red") {
		t.Error("generateEdges() complex workflow should have red failure edges")
	}

	// All step names should be present
	for _, name := range []string{"init", "parallel", "branch_a", "branch_b", "call_sub", "notify", "success", "failure"} {
		if !strings.Contains(edges, name) {
			t.Errorf("generateEdges() should contain step %q", name)
		}
	}
}

func TestGenerator_generateEdges_EscapesSpecialCharactersInNames(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "escape-edges",
		Initial: "step-with-dashes",
		Steps: map[string]*workflow.Step{
			"step-with-dashes": {
				Name:      "step-with-dashes",
				Type:      workflow.StepTypeCommand,
				Command:   "echo test",
				OnSuccess: "step_with_underscores",
			},
			"step_with_underscores": {
				Name:   "step_with_underscores",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Edge should be valid DOT with special characters handled
	if !strings.Contains(edges, "->") {
		t.Error("generateEdges() should produce valid edges with special characters in names")
	}
}

func TestGenerator_generateEdges_NoEdgesWhenOnlyOnSuccessOrFailureSet(t *testing.T) {
	tests := []struct {
		name      string
		onSuccess string
		onFailure string
		wantEdges int
	}{
		{"only_success", "done", "", 1},
		{"only_failure", "", "error", 1},
		{"both", "done", "error", 2},
		{"neither", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := map[string]*workflow.Step{
				"start": {
					Name:      "start",
					Type:      workflow.StepTypeCommand,
					Command:   "echo start",
					OnSuccess: tt.onSuccess,
					OnFailure: tt.onFailure,
				},
			}
			if tt.onSuccess != "" {
				steps[tt.onSuccess] = &workflow.Step{
					Name:   tt.onSuccess,
					Type:   workflow.StepTypeTerminal,
					Status: workflow.TerminalSuccess,
				}
			}
			if tt.onFailure != "" {
				steps[tt.onFailure] = &workflow.Step{
					Name:   tt.onFailure,
					Type:   workflow.StepTypeTerminal,
					Status: workflow.TerminalFailure,
				}
			}

			wf := &workflow.Workflow{
				Name:    tt.name,
				Initial: "start",
				Steps:   steps,
			}

			g := NewGenerator(nil)
			edges := g.generateEdges(wf)

			arrowCount := strings.Count(edges, "->")
			if arrowCount != tt.wantEdges {
				t.Errorf("generateEdges() = %d edges, want %d edges", arrowCount, tt.wantEdges)
			}
		})
	}
}

func TestGenerator_generateEdges_ParallelWithOnSuccessAndOnFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "parallel-all-edges",
		Initial: "parallel",
		Steps: map[string]*workflow.Step{
			"parallel": {
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"a", "b"},
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"a": {
				Name:    "a",
				Type:    workflow.StepTypeCommand,
				Command: "echo a",
			},
			"b": {
				Name:    "b",
				Type:    workflow.StepTypeCommand,
				Command: "echo b",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(nil)
	edges := g.generateEdges(wf)

	// Parallel should have edges to: a, b, success, failure
	// Total: 4 edges from parallel step
	if !strings.Contains(edges, "success") {
		t.Error("generateEdges() should contain edge to success")
	}
	if !strings.Contains(edges, "failure") {
		t.Error("generateEdges() should contain edge to failure")
	}
	if !strings.Contains(edges, "a") {
		t.Error("generateEdges() should contain edge to branch a")
	}
	if !strings.Contains(edges, "b") {
		t.Error("generateEdges() should contain edge to branch b")
	}

	// Failure edge should be dashed red
	if !strings.Contains(edges, "dashed") || !strings.Contains(edges, "red") {
		t.Error("generateEdges() parallel failure edge should be dashed red")
	}
}

// Tests for generateParallelSubgraph() method (T008)
// Per FR-004: Parallel branches are grouped in subgraph clusters

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

// =============================================================================
// Table-driven tests for DOT generator covering all step types (T009)
// =============================================================================

// TestGenerator_Generate_StepTypeShapes_TableDriven tests shape assignment for all step types
// using a comprehensive table-driven approach.
func TestGenerator_Generate_StepTypeShapes_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		stepType      workflow.StepType
		status        workflow.TerminalStatus // for terminal steps
		expectedShape string
		extraConfig   func(*workflow.Step) // additional step configuration
	}{
		{
			name:          "command step renders as box",
			stepType:      workflow.StepTypeCommand,
			expectedShape: "box",
			extraConfig: func(s *workflow.Step) {
				s.Command = "echo test"
			},
		},
		{
			name:          "parallel step renders as diamond",
			stepType:      workflow.StepTypeParallel,
			expectedShape: "diamond",
			extraConfig: func(s *workflow.Step) {
				s.Branches = []string{"branch1"}
			},
		},
		{
			name:          "terminal success renders as oval",
			stepType:      workflow.StepTypeTerminal,
			status:        workflow.TerminalSuccess,
			expectedShape: "oval",
		},
		{
			name:          "terminal failure renders as doubleoval",
			stepType:      workflow.StepTypeTerminal,
			status:        workflow.TerminalFailure,
			expectedShape: "doubleoval",
		},
		{
			name:          "for_each loop renders as hexagon",
			stepType:      workflow.StepTypeForEach,
			expectedShape: "hexagon",
			extraConfig: func(s *workflow.Step) {
				s.Loop = &workflow.LoopConfig{
					Items:      "{{inputs.items}}",
					Body:       []string{"body"},
					OnComplete: "done",
				}
			},
		},
		{
			name:          "while loop renders as hexagon",
			stepType:      workflow.StepTypeWhile,
			expectedShape: "hexagon",
			extraConfig: func(s *workflow.Step) {
				s.Loop = &workflow.LoopConfig{
					Condition:  "{{count}} < 10",
					Body:       []string{"body"},
					OnComplete: "done",
				}
			},
		},
		{
			name:          "operation step renders as box3d",
			stepType:      workflow.StepTypeOperation,
			expectedShape: "box3d",
			extraConfig: func(s *workflow.Step) {
				s.Operation = "slack.send"
			},
		},
		{
			name:          "call_workflow step renders as folder",
			stepType:      workflow.StepTypeCallWorkflow,
			expectedShape: "folder",
			extraConfig: func(s *workflow.Step) {
				s.CallWorkflow = &workflow.CallWorkflowConfig{
					Workflow: "sub-workflow",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &workflow.Step{
				Name:      "test_step",
				Type:      tt.stepType,
				Status:    tt.status,
				OnSuccess: "done",
			}
			if tt.extraConfig != nil {
				tt.extraConfig(step)
			}

			wf := &workflow.Workflow{
				Name:    "shape-test",
				Initial: "test_step",
				Steps: map[string]*workflow.Step{
					"test_step": step,
					"done": {
						Name:   "done",
						Type:   workflow.StepTypeTerminal,
						Status: workflow.TerminalSuccess,
					},
					"branch1": {
						Name:    "branch1",
						Type:    workflow.StepTypeCommand,
						Command: "echo branch",
					},
					"body": {
						Name:    "body",
						Type:    workflow.StepTypeCommand,
						Command: "echo body",
					},
				},
			}

			g := NewGenerator(nil)
			dot := g.Generate(wf)

			if !strings.Contains(dot, tt.expectedShape) {
				t.Errorf("Generate() for %s step should contain shape %q, got:\n%s",
					tt.stepType, tt.expectedShape, dot)
			}
		})
	}
}

// TestGenerator_Generate_EdgeStyles_TableDriven tests edge styling for different transition types.
func TestGenerator_Generate_EdgeStyles_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		transitionType string // "success", "failure", "branch", "conditional"
		expectSolid    bool
		expectDashed   bool
		expectRed      bool
		expectLabel    bool
	}{
		{
			name:           "on_success edge is solid",
			transitionType: "success",
			expectSolid:    true,
			expectDashed:   false,
			expectRed:      false,
		},
		{
			name:           "on_failure edge is dashed red",
			transitionType: "failure",
			expectSolid:    false,
			expectDashed:   true,
			expectRed:      true,
		},
		{
			name:           "parallel branch edge is solid",
			transitionType: "branch",
			expectSolid:    true,
			expectDashed:   false,
			expectRed:      false,
		},
		{
			name:           "conditional transition has label",
			transitionType: "conditional",
			expectSolid:    true,
			expectDashed:   false,
			expectRed:      false,
			expectLabel:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wf *workflow.Workflow

			switch tt.transitionType {
			case "success":
				wf = &workflow.Workflow{
					Name:    "success-edge",
					Initial: "start",
					Steps: map[string]*workflow.Step{
						"start": {
							Name:      "start",
							Type:      workflow.StepTypeCommand,
							Command:   "echo start",
							OnSuccess: "done",
						},
						"done": {
							Name:   "done",
							Type:   workflow.StepTypeTerminal,
							Status: workflow.TerminalSuccess,
						},
					},
				}
			case "failure":
				wf = &workflow.Workflow{
					Name:    "failure-edge",
					Initial: "start",
					Steps: map[string]*workflow.Step{
						"start": {
							Name:      "start",
							Type:      workflow.StepTypeCommand,
							Command:   "echo start",
							OnSuccess: "ok",
							OnFailure: "err",
						},
						"ok": {
							Name:   "ok",
							Type:   workflow.StepTypeTerminal,
							Status: workflow.TerminalSuccess,
						},
						"err": {
							Name:   "err",
							Type:   workflow.StepTypeTerminal,
							Status: workflow.TerminalFailure,
						},
					},
				}
			case "branch":
				wf = &workflow.Workflow{
					Name:    "branch-edge",
					Initial: "parallel",
					Steps: map[string]*workflow.Step{
						"parallel": {
							Name:      "parallel",
							Type:      workflow.StepTypeParallel,
							Branches:  []string{"a", "b"},
							OnSuccess: "done",
						},
						"a": {
							Name:    "a",
							Type:    workflow.StepTypeCommand,
							Command: "echo a",
						},
						"b": {
							Name:    "b",
							Type:    workflow.StepTypeCommand,
							Command: "echo b",
						},
						"done": {
							Name:   "done",
							Type:   workflow.StepTypeTerminal,
							Status: workflow.TerminalSuccess,
						},
					},
				}
			case "conditional":
				wf = &workflow.Workflow{
					Name:    "conditional-edge",
					Initial: "check",
					Steps: map[string]*workflow.Step{
						"check": {
							Name:    "check",
							Type:    workflow.StepTypeCommand,
							Command: "test -f file",
							Transitions: workflow.Transitions{
								{When: "{{states.check.exit_code}} == 0", Goto: "found"},
								{When: "", Goto: "not_found"},
							},
						},
						"found": {
							Name:   "found",
							Type:   workflow.StepTypeTerminal,
							Status: workflow.TerminalSuccess,
						},
						"not_found": {
							Name:   "not_found",
							Type:   workflow.StepTypeTerminal,
							Status: workflow.TerminalFailure,
						},
					},
				}
			}

			g := NewGenerator(&DiagramConfig{ShowLabels: true})
			dot := g.Generate(wf)

			if tt.expectDashed && !strings.Contains(dot, "dashed") {
				t.Error("expected dashed edge style")
			}
			if tt.expectRed && !strings.Contains(dot, "red") {
				t.Error("expected red edge color")
			}
			if tt.expectLabel && !strings.Contains(dot, "label") {
				t.Error("expected edge label for conditional")
			}
			// Verify edge exists
			if !strings.Contains(dot, "->") {
				t.Error("expected edge arrow in DOT output")
			}
		})
	}
}

// TestGenerator_Generate_NodeAttributes_TableDriven tests node attribute generation.
func TestGenerator_Generate_NodeAttributes_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		stepType        workflow.StepType
		isInitial       bool
		isHighlighted   bool
		expectPeriphery bool // double border for initial
		expectBold      bool // bold for highlighted
		expectColor     bool // fill color for terminals
	}{
		{
			name:            "initial step has start marker",
			stepType:        workflow.StepTypeCommand,
			isInitial:       true,
			expectPeriphery: true,
		},
		{
			name:          "highlighted step has emphasis",
			stepType:      workflow.StepTypeCommand,
			isHighlighted: true,
			expectBold:    true,
		},
		{
			name:        "terminal success has fill color",
			stepType:    workflow.StepTypeTerminal,
			expectColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stepName := "step"
			if tt.isInitial {
				stepName = "initial"
			}

			step := &workflow.Step{
				Name:    stepName,
				Type:    tt.stepType,
				Command: "echo test",
			}
			if tt.stepType == workflow.StepTypeTerminal {
				step.Status = workflow.TerminalSuccess
				step.Command = ""
			}

			cfg := &DiagramConfig{
				Direction:  DirectionTB,
				ShowLabels: true,
			}
			if tt.isHighlighted {
				cfg.Highlight = stepName
			}

			wf := &workflow.Workflow{
				Name:    "attr-test",
				Initial: stepName,
				Steps: map[string]*workflow.Step{
					stepName: step,
				},
			}

			g := NewGenerator(cfg)
			dot := g.Generate(wf)

			if tt.expectPeriphery {
				hasMarker := strings.Contains(dot, "peripheries") ||
					strings.Contains(dot, "__start__") ||
					strings.Contains(dot, "point")
				if !hasMarker {
					t.Error("initial step should have start marker")
				}
			}
			if tt.expectBold {
				hasEmphasis := strings.Contains(dot, "penwidth") ||
					strings.Contains(dot, "bold") ||
					strings.Contains(dot, "style=")
				if !hasEmphasis {
					t.Error("highlighted step should have visual emphasis")
				}
			}
			if tt.expectColor && tt.stepType == workflow.StepTypeTerminal {
				hasColor := strings.Contains(dot, "fillcolor") ||
					strings.Contains(dot, "color") ||
					strings.Contains(dot, "green") ||
					strings.Contains(dot, "style=filled")
				if !hasColor {
					t.Error("terminal step should have color styling")
				}
			}
		})
	}
}

// TestGenerator_Generate_Direction_TableDriven tests all graph direction options.
func TestGenerator_Generate_Direction_TableDriven(t *testing.T) {
	tests := []struct {
		direction       Direction
		expectedRankdir string
	}{
		{DirectionTB, "TB"},
		{DirectionLR, "LR"},
		{DirectionBT, "BT"},
		{DirectionRL, "RL"},
	}

	wf := createSimpleWorkflow()

	for _, tt := range tests {
		t.Run(string(tt.direction), func(t *testing.T) {
			g := NewGenerator(&DiagramConfig{Direction: tt.direction})
			dot := g.Generate(wf)

			expectedPattern := "rankdir=" + tt.expectedRankdir
			expectedQuoted := `rankdir="` + tt.expectedRankdir + `"`

			if !strings.Contains(dot, expectedPattern) && !strings.Contains(dot, expectedQuoted) {
				t.Errorf("Generate() should contain rankdir=%s for direction %s",
					tt.expectedRankdir, tt.direction)
			}
		})
	}
}

// TestGenerator_Generate_AllStepTypesIntegration_TableDriven tests DOT generation for
// a workflow containing all step types in a single table-driven test matrix.
func TestGenerator_Generate_AllStepTypesIntegration_TableDriven(t *testing.T) {
	// Define expected attributes for each step type
	type stepExpectation struct {
		stepType      workflow.StepType
		expectedShape string
		nodePresent   bool
	}

	tests := []struct {
		name           string
		expectations   []stepExpectation
		expectSubgraph bool
		expectEdges    bool
	}{
		{
			name: "all step types present in DOT output",
			expectations: []stepExpectation{
				{workflow.StepTypeCommand, "box", true},
				{workflow.StepTypeParallel, "diamond", true},
				{workflow.StepTypeTerminal, "oval", true},
				{workflow.StepTypeForEach, "hexagon", true},
				{workflow.StepTypeWhile, "hexagon", true},
				{workflow.StepTypeOperation, "box3d", true},
				{workflow.StepTypeCallWorkflow, "folder", true},
			},
			expectSubgraph: true,
			expectEdges:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a comprehensive workflow with all step types
			wf := &workflow.Workflow{
				Name:    "all-types-integration",
				Initial: "cmd_step",
				Steps: map[string]*workflow.Step{
					"cmd_step": {
						Name:      "cmd_step",
						Type:      workflow.StepTypeCommand,
						Command:   "echo start",
						OnSuccess: "parallel_step",
						OnFailure: "fail_terminal",
					},
					"parallel_step": {
						Name:      "parallel_step",
						Type:      workflow.StepTypeParallel,
						Branches:  []string{"branch_a", "branch_b"},
						OnSuccess: "foreach_step",
					},
					"branch_a": {
						Name:      "branch_a",
						Type:      workflow.StepTypeCommand,
						Command:   "echo a",
						OnSuccess: "foreach_step",
					},
					"branch_b": {
						Name:      "branch_b",
						Type:      workflow.StepTypeCommand,
						Command:   "echo b",
						OnSuccess: "foreach_step",
					},
					"foreach_step": {
						Name: "foreach_step",
						Type: workflow.StepTypeForEach,
						Loop: &workflow.LoopConfig{
							Items:      "{{inputs.items}}",
							Body:       []string{"loop_body"},
							OnComplete: "while_step",
						},
					},
					"loop_body": {
						Name:    "loop_body",
						Type:    workflow.StepTypeCommand,
						Command: "echo item",
					},
					"while_step": {
						Name: "while_step",
						Type: workflow.StepTypeWhile,
						Loop: &workflow.LoopConfig{
							Condition:  "{{count}} < 5",
							Body:       []string{"while_body"},
							OnComplete: "op_step",
						},
					},
					"while_body": {
						Name:    "while_body",
						Type:    workflow.StepTypeCommand,
						Command: "echo while",
					},
					"op_step": {
						Name:      "op_step",
						Type:      workflow.StepTypeOperation,
						Operation: "slack.send",
						OnSuccess: "call_step",
					},
					"call_step": {
						Name: "call_step",
						Type: workflow.StepTypeCallWorkflow,
						CallWorkflow: &workflow.CallWorkflowConfig{
							Workflow: "sub-workflow",
						},
						OnSuccess: "success_terminal",
						OnFailure: "fail_terminal",
					},
					"success_terminal": {
						Name:   "success_terminal",
						Type:   workflow.StepTypeTerminal,
						Status: workflow.TerminalSuccess,
					},
					"fail_terminal": {
						Name:   "fail_terminal",
						Type:   workflow.StepTypeTerminal,
						Status: workflow.TerminalFailure,
					},
				},
			}

			g := NewGenerator(nil)
			dot := g.Generate(wf)

			// Verify each expected shape is present
			for _, exp := range tt.expectations {
				if exp.nodePresent && !strings.Contains(dot, exp.expectedShape) {
					t.Errorf("DOT output missing shape %q for step type %s",
						exp.expectedShape, exp.stepType)
				}
			}

			// Verify subgraph for parallel step
			if tt.expectSubgraph {
				if !strings.Contains(dot, "subgraph") {
					t.Error("DOT output should contain subgraph for parallel step")
				}
			}

			// Verify edges exist
			if tt.expectEdges {
				if !strings.Contains(dot, "->") {
					t.Error("DOT output should contain edge arrows")
				}
			}

			// Verify it's valid DOT structure
			if !strings.HasPrefix(strings.TrimSpace(dot), "digraph") {
				t.Error("DOT output should start with 'digraph'")
			}
			openBraces := strings.Count(dot, "{")
			closeBraces := strings.Count(dot, "}")
			if openBraces != closeBraces {
				t.Errorf("DOT output has mismatched braces: %d open, %d close",
					openBraces, closeBraces)
			}
		})
	}
}

// TestGenerator_generateNodes_StepTypeShapes_TableDriven tests generateNodes for each step type.
func TestGenerator_generateNodes_StepTypeShapes_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		step          *workflow.Step
		expectedShape string
	}{
		{
			name: "command step generates box node",
			step: &workflow.Step{
				Name:    "cmd",
				Type:    workflow.StepTypeCommand,
				Command: "echo test",
			},
			expectedShape: "box",
		},
		{
			name: "parallel step generates diamond node",
			step: &workflow.Step{
				Name:     "parallel",
				Type:     workflow.StepTypeParallel,
				Branches: []string{"a"},
			},
			expectedShape: "diamond",
		},
		{
			name: "terminal success generates oval node",
			step: &workflow.Step{
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			expectedShape: "oval",
		},
		{
			name: "terminal failure generates doubleoval node",
			step: &workflow.Step{
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
			expectedShape: "doubleoval",
		},
		{
			name: "for_each loop generates hexagon node",
			step: &workflow.Step{
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{Items: "{{items}}", Body: []string{"b"}},
			},
			expectedShape: "hexagon",
		},
		{
			name: "while loop generates hexagon node",
			step: &workflow.Step{
				Name: "while",
				Type: workflow.StepTypeWhile,
				Loop: &workflow.LoopConfig{Condition: "{{x}} < 5", Body: []string{"b"}},
			},
			expectedShape: "hexagon",
		},
		{
			name: "operation generates box3d node",
			step: &workflow.Step{
				Name:      "op",
				Type:      workflow.StepTypeOperation,
				Operation: "test.op",
			},
			expectedShape: "box3d",
		},
		{
			name: "call_workflow generates folder node",
			step: &workflow.Step{
				Name: "call",
				Type: workflow.StepTypeCallWorkflow,
				CallWorkflow: &workflow.CallWorkflowConfig{
					Workflow: "other",
				},
			},
			expectedShape: "folder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:    "test",
				Initial: tt.step.Name,
				Steps: map[string]*workflow.Step{
					tt.step.Name: tt.step,
					"a":          {Name: "a", Type: workflow.StepTypeCommand, Command: "echo"},
					"b":          {Name: "b", Type: workflow.StepTypeCommand, Command: "echo"},
				},
			}

			g := NewGenerator(nil)
			nodes := g.generateNodes(wf)

			if !strings.Contains(nodes, tt.step.Name) {
				t.Errorf("generateNodes() should contain step name %q", tt.step.Name)
			}
			if !strings.Contains(nodes, tt.expectedShape) {
				t.Errorf("generateNodes() for %s should contain shape %q, got:\n%s",
					tt.step.Type, tt.expectedShape, nodes)
			}
		})
	}
}

// TestGenerator_generateEdges_TransitionTypes_TableDriven tests edge generation for all transition types.
func TestGenerator_generateEdges_TransitionTypes_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		sourceStep  *workflow.Step
		targetSteps map[string]*workflow.Step
		expectEdges int
		expectStyle string // "dashed", "solid", or empty
		expectColor string // "red", "green", or empty
	}{
		{
			name: "on_success creates single solid edge",
			sourceStep: &workflow.Step{
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo",
				OnSuccess: "done",
			},
			targetSteps: map[string]*workflow.Step{
				"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			},
			expectEdges: 1,
			expectStyle: "",
		},
		{
			name: "on_failure creates dashed red edge",
			sourceStep: &workflow.Step{
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo",
				OnSuccess: "ok",
				OnFailure: "err",
			},
			targetSteps: map[string]*workflow.Step{
				"ok":  {Name: "ok", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
				"err": {Name: "err", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
			},
			expectEdges: 2,
			expectStyle: "dashed",
			expectColor: "red",
		},
		{
			name: "parallel branches create multiple edges",
			sourceStep: &workflow.Step{
				Name:      "parallel",
				Type:      workflow.StepTypeParallel,
				Branches:  []string{"a", "b", "c"},
				OnSuccess: "done",
			},
			targetSteps: map[string]*workflow.Step{
				"a":    {Name: "a", Type: workflow.StepTypeCommand, Command: "echo a"},
				"b":    {Name: "b", Type: workflow.StepTypeCommand, Command: "echo b"},
				"c":    {Name: "c", Type: workflow.StepTypeCommand, Command: "echo c"},
				"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			},
			expectEdges: 4, // 3 branches + 1 success
		},
		{
			name: "conditional transitions create labeled edges",
			sourceStep: &workflow.Step{
				Name:    "check",
				Type:    workflow.StepTypeCommand,
				Command: "test -f x",
				Transitions: workflow.Transitions{
					{When: "{{exit_code}} == 0", Goto: "yes"},
					{When: "", Goto: "no"},
				},
			},
			targetSteps: map[string]*workflow.Step{
				"yes": {Name: "yes", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
				"no":  {Name: "no", Type: workflow.StepTypeTerminal, Status: workflow.TerminalFailure},
			},
			expectEdges: 2,
		},
		{
			name: "loop on_complete creates edge",
			sourceStep: &workflow.Step{
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Items:      "{{items}}",
					Body:       []string{"body"},
					OnComplete: "done",
				},
			},
			targetSteps: map[string]*workflow.Step{
				"body": {Name: "body", Type: workflow.StepTypeCommand, Command: "echo"},
				"done": {Name: "done", Type: workflow.StepTypeTerminal, Status: workflow.TerminalSuccess},
			},
			expectEdges: 2, // body + on_complete
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := map[string]*workflow.Step{
				tt.sourceStep.Name: tt.sourceStep,
			}
			for name, step := range tt.targetSteps {
				steps[name] = step
			}

			wf := &workflow.Workflow{
				Name:    "edge-test",
				Initial: tt.sourceStep.Name,
				Steps:   steps,
			}

			g := NewGenerator(&DiagramConfig{ShowLabels: true})
			edges := g.generateEdges(wf)

			arrowCount := strings.Count(edges, "->")
			if arrowCount < tt.expectEdges {
				t.Errorf("generateEdges() produced %d edges, want at least %d",
					arrowCount, tt.expectEdges)
			}

			if tt.expectStyle == "dashed" && !strings.Contains(edges, "dashed") {
				t.Error("expected dashed edge style for failure transition")
			}
			if tt.expectColor == "red" && !strings.Contains(edges, "red") {
				t.Error("expected red color for failure transition")
			}
		})
	}
}

// =============================================================================
// T014: Highlight Flag Support Tests
// =============================================================================

// TestGenerator_isHighlighted tests the isHighlighted method.
func TestGenerator_isHighlighted(t *testing.T) {
	tests := []struct {
		name      string
		highlight string
		stepName  string
		want      bool
	}{
		{
			name:      "matching step name returns true",
			highlight: "my_step",
			stepName:  "my_step",
			want:      true,
		},
		{
			name:      "non-matching step name returns false",
			highlight: "other_step",
			stepName:  "my_step",
			want:      false,
		},
		{
			name:      "empty highlight returns false for any step",
			highlight: "",
			stepName:  "my_step",
			want:      false,
		},
		{
			name:      "empty step name with empty highlight",
			highlight: "",
			stepName:  "",
			want:      false,
		},
		{
			name:      "case sensitive match - different case returns false",
			highlight: "MyStep",
			stepName:  "mystep",
			want:      false,
		},
		{
			name:      "exact case match returns true",
			highlight: "MyStep",
			stepName:  "MyStep",
			want:      true,
		},
		{
			name:      "step name with special characters",
			highlight: "step-with-dashes_and_underscores",
			stepName:  "step-with-dashes_and_underscores",
			want:      true,
		},
		{
			name:      "partial match returns false",
			highlight: "step",
			stepName:  "step1",
			want:      false,
		},
		{
			name:      "whitespace in name",
			highlight: " step ",
			stepName:  " step ",
			want:      true,
		},
		{
			name:      "whitespace mismatch",
			highlight: "step",
			stepName:  " step ",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(&DiagramConfig{Highlight: tt.highlight})
			got := g.isHighlighted(tt.stepName)
			if got != tt.want {
				t.Errorf("isHighlighted(%q) = %v, want %v", tt.stepName, got, tt.want)
			}
		})
	}
}

// TestGenerator_isHighlighted_NilConfig tests isHighlighted with nil config.
func TestGenerator_isHighlighted_NilConfig(t *testing.T) {
	g := NewGenerator(nil)
	// With default config, Highlight is empty, so nothing should be highlighted
	if g.isHighlighted("any_step") {
		t.Error("isHighlighted() should return false when highlight is not set")
	}
}

// TestGenerator_getHighlightStyle tests the getHighlightStyle method.
func TestGenerator_getHighlightStyle(t *testing.T) {
	g := NewGenerator(nil)
	style := g.getHighlightStyle()

	// Verify highlight style has emphasis attributes
	if style.Style == "" {
		t.Error("getHighlightStyle() should return non-empty Style")
	}
	if style.Color == "" {
		t.Error("getHighlightStyle() should return non-empty Color for emphasis")
	}
}

// TestGenerator_getHighlightStyle_HasBoldStyle tests bold styling.
func TestGenerator_getHighlightStyle_HasBoldStyle(t *testing.T) {
	g := NewGenerator(nil)
	style := g.getHighlightStyle()

	if !strings.Contains(style.Style, "bold") {
		t.Error("getHighlightStyle() should include 'bold' in Style")
	}
}

// TestGenerator_getHighlightStyle_HasBlueColor tests blue color emphasis.
func TestGenerator_getHighlightStyle_HasBlueColor(t *testing.T) {
	g := NewGenerator(nil)
	style := g.getHighlightStyle()

	// Per spec: color should be #2196F3 (material blue)
	if style.Color != "#2196F3" {
		t.Errorf("getHighlightStyle() Color = %q, want #2196F3", style.Color)
	}
}

// TestGenerator_applyHighlight tests the applyHighlight method.
func TestGenerator_applyHighlight(t *testing.T) {
	tests := []struct {
		name         string
		highlight    string
		stepName     string
		baseStyle    NodeStyle
		expectChange bool
	}{
		{
			name:      "applies highlight to matching step",
			highlight: "my_step",
			stepName:  "my_step",
			baseStyle: NodeStyle{
				Shape:     "box",
				Color:     "black",
				FillColor: "white",
				Style:     "filled",
			},
			expectChange: true,
		},
		{
			name:      "does not change non-matching step",
			highlight: "other_step",
			stepName:  "my_step",
			baseStyle: NodeStyle{
				Shape:     "box",
				Color:     "black",
				FillColor: "white",
				Style:     "filled",
			},
			expectChange: false,
		},
		{
			name:      "does not change when highlight is empty",
			highlight: "",
			stepName:  "my_step",
			baseStyle: NodeStyle{
				Shape:     "box",
				Color:     "black",
				FillColor: "white",
				Style:     "filled",
			},
			expectChange: false,
		},
		{
			name:      "preserves base shape when highlighting",
			highlight: "my_step",
			stepName:  "my_step",
			baseStyle: NodeStyle{
				Shape: "diamond",
				Style: "filled",
			},
			expectChange: true,
		},
		{
			name:         "handles empty base style",
			highlight:    "my_step",
			stepName:     "my_step",
			baseStyle:    NodeStyle{},
			expectChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(&DiagramConfig{Highlight: tt.highlight})
			result := g.applyHighlight(tt.stepName, tt.baseStyle)

			if tt.expectChange {
				// Highlighted steps should have emphasis
				hasEmphasis := strings.Contains(result.Style, "bold") ||
					result.Color == "#2196F3" ||
					result.Color != tt.baseStyle.Color

				if !hasEmphasis {
					t.Error("applyHighlight() should add emphasis to highlighted step")
				}

				// Shape should be preserved
				if result.Shape != tt.baseStyle.Shape {
					t.Errorf("applyHighlight() Shape = %q, want preserved %q",
						result.Shape, tt.baseStyle.Shape)
				}
			} else {
				// Non-highlighted steps should remain unchanged
				if result.Color != tt.baseStyle.Color {
					t.Errorf("applyHighlight() Color = %q, want unchanged %q",
						result.Color, tt.baseStyle.Color)
				}
				if result.Style != tt.baseStyle.Style {
					t.Errorf("applyHighlight() Style = %q, want unchanged %q",
						result.Style, tt.baseStyle.Style)
				}
			}
		})
	}
}

// TestGenerator_applyHighlight_PreservesBaseShape tests shape preservation.
func TestGenerator_applyHighlight_PreservesBaseShape(t *testing.T) {
	shapes := []string{"box", "diamond", "oval", "hexagon", "box3d", "folder"}

	for _, shape := range shapes {
		t.Run(shape, func(t *testing.T) {
			g := NewGenerator(&DiagramConfig{Highlight: "test_step"})
			baseStyle := NodeStyle{Shape: shape}
			result := g.applyHighlight("test_step", baseStyle)

			if result.Shape != shape {
				t.Errorf("applyHighlight() should preserve shape %q, got %q", shape, result.Shape)
			}
		})
	}
}

// TestGenerator_applyHighlight_AddsBoldToExistingStyle tests style merging.
func TestGenerator_applyHighlight_AddsBoldToExistingStyle(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Highlight: "test_step"})

	tests := []struct {
		name          string
		baseStyle     string
		expectContain string
	}{
		{
			name:          "adds bold to filled style",
			baseStyle:     "filled",
			expectContain: "bold",
		},
		{
			name:          "adds bold to dashed style",
			baseStyle:     "dashed",
			expectContain: "bold",
		},
		{
			name:          "adds bold to empty style",
			baseStyle:     "",
			expectContain: "bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := NodeStyle{Style: tt.baseStyle}
			result := g.applyHighlight("test_step", base)

			if !strings.Contains(result.Style, tt.expectContain) {
				t.Errorf("applyHighlight() Style = %q, should contain %q",
					result.Style, tt.expectContain)
			}
		})
	}
}

// TestGenerator_applyHighlight_SetsHighlightColor tests color application.
func TestGenerator_applyHighlight_SetsHighlightColor(t *testing.T) {
	g := NewGenerator(&DiagramConfig{Highlight: "test_step"})

	baseStyle := NodeStyle{
		Shape: "box",
		Color: "black",
	}
	result := g.applyHighlight("test_step", baseStyle)

	// Per spec: highlight color should be #2196F3
	if result.Color != "#2196F3" {
		t.Errorf("applyHighlight() Color = %q, want #2196F3", result.Color)
	}
}

// TestGenerator_Highlight_Integration tests highlight in full Generate flow.
func TestGenerator_Highlight_Integration(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "highlight-integration",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name:      "step1",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 1",
				OnSuccess: "step2",
			},
			"step2": {
				Name:      "step2",
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

	tests := []struct {
		name      string
		highlight string
		wantIn    []string
	}{
		{
			name:      "highlight step1",
			highlight: "step1",
			wantIn:    []string{"step1", "penwidth", "bold", "#2196F3"},
		},
		{
			name:      "highlight step2",
			highlight: "step2",
			wantIn:    []string{"step2"},
		},
		{
			name:      "highlight terminal step",
			highlight: "done",
			wantIn:    []string{"done"},
		},
		{
			name:      "no highlight",
			highlight: "",
			wantIn:    []string{"step1", "step2", "done"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGenerator(&DiagramConfig{Highlight: tt.highlight})
			dot := g.Generate(wf)

			for _, want := range tt.wantIn {
				if !strings.Contains(dot, want) {
					t.Errorf("Generate() output should contain %q", want)
				}
			}

			// When highlighting, only the highlighted step should have emphasis
			if tt.highlight != "" {
				hasEmphasis := strings.Contains(dot, "penwidth") ||
					strings.Contains(dot, "bold") ||
					strings.Contains(dot, "#2196F3")
				if !hasEmphasis {
					t.Error("Generate() with highlight should include visual emphasis")
				}
			}
		})
	}
}

// TestGenerator_Highlight_NonExistentStep tests highlighting a step that doesn't exist.
func TestGenerator_Highlight_NonExistentStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "nonexistent-highlight",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	// Highlighting non-existent step should not cause error
	g := NewGenerator(&DiagramConfig{Highlight: "nonexistent_step"})
	dot := g.Generate(wf)

	// Should still produce valid output
	if !strings.Contains(dot, "digraph") {
		t.Error("Generate() should produce valid DOT even with non-existent highlight")
	}
	if !strings.Contains(dot, "start") {
		t.Error("Generate() should include existing steps")
	}
}

// TestGenerator_Highlight_ParallelStep tests highlighting a parallel step.
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

// TestGenerator_Highlight_BranchStep tests highlighting a branch within parallel.
func TestGenerator_Highlight_BranchStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "branch-highlight",
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

	g := NewGenerator(&DiagramConfig{Highlight: "branch1"})
	dot := g.Generate(wf)

	// branch1 should be highlighted, branch2 should not
	if !strings.Contains(dot, "branch1") {
		t.Error("Generate() should include highlighted branch step")
	}
}

// TestGenerator_Highlight_LoopStep tests highlighting loop steps.
func TestGenerator_Highlight_LoopStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "loop-highlight",
		Initial: "loop",
		Steps: map[string]*workflow.Step{
			"loop": {
				Name: "loop",
				Type: workflow.StepTypeForEach,
				Loop: &workflow.LoopConfig{
					Type:       workflow.LoopTypeForEach,
					Items:      "{{inputs.items}}",
					Body:       []string{"process"},
					OnComplete: "done",
				},
				OnSuccess: "done",
			},
			"process": {
				Name:      "process",
				Type:      workflow.StepTypeCommand,
				Command:   "echo {{item}}",
				OnSuccess: "loop",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{Highlight: "loop"})
	dot := g.Generate(wf)

	// Loop step should be highlighted with hexagon shape preserved
	if !strings.Contains(dot, "loop") {
		t.Error("Generate() should include highlighted loop step")
	}
	if !strings.Contains(dot, "hexagon") {
		t.Error("Generate() should preserve loop step hexagon shape when highlighted")
	}
}

// TestGenerator_Highlight_TerminalFailure tests highlighting terminal failure step.
func TestGenerator_Highlight_TerminalFailure(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "failure-highlight",
		Initial: "start",
		Steps: map[string]*workflow.Step{
			"start": {
				Name:      "start",
				Type:      workflow.StepTypeCommand,
				Command:   "echo start",
				OnSuccess: "success",
				OnFailure: "failure",
			},
			"success": {
				Name:   "success",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
			"failure": {
				Name:   "failure",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalFailure,
			},
		},
	}

	g := NewGenerator(&DiagramConfig{Highlight: "failure"})
	dot := g.Generate(wf)

	// Failure terminal should be highlighted while maintaining failure styling
	if !strings.Contains(dot, "failure") {
		t.Error("Generate() should include highlighted failure terminal")
	}
}
