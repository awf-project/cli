package diagram

// Generator Header Tests
// Tests for DOT header generation, direction handling, and shape mapping

import (
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
)

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
