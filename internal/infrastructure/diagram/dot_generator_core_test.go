package diagram

// Generator Core Tests
// Integration tests for the DOT generator

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
				assertHasStartMarker(t, dot)
			}
			if tt.expectBold {
				assertHasEmphasis(t, dot)
			}
			if tt.expectColor && tt.stepType == workflow.StepTypeTerminal {
				assertHasColorStyling(t, dot)
			}
		})
	}
}

// assertHasStartMarker checks if the DOT output contains start marker indicators
func assertHasStartMarker(t *testing.T, dot string) {
	t.Helper()
	hasMarker := strings.Contains(dot, "peripheries") ||
		strings.Contains(dot, "__start__") ||
		strings.Contains(dot, "point")
	if !hasMarker {
		t.Error("initial step should have start marker")
	}
}

// assertHasEmphasis checks if the DOT output contains emphasis styling
func assertHasEmphasis(t *testing.T, dot string) {
	t.Helper()
	hasEmphasis := strings.Contains(dot, "penwidth") ||
		strings.Contains(dot, "bold") ||
		strings.Contains(dot, "style=")
	if !hasEmphasis {
		t.Error("highlighted step should have visual emphasis")
	}
}

// assertHasColorStyling checks if the DOT output contains color fill attributes
func assertHasColorStyling(t *testing.T, dot string) {
	t.Helper()
	hasColor := strings.Contains(dot, "fillcolor") ||
		strings.Contains(dot, "color") ||
		strings.Contains(dot, "green") ||
		strings.Contains(dot, "style=filled")
	if !hasColor {
		t.Error("terminal step should have color styling")
	}
}

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
