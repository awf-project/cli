package diagram

// Generator Node Tests
// Tests for node generation covering all step types

import (
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
)

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
