package diagram

// Generator Highlight Tests
// Tests for step highlighting functionality

import (
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
)

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

func TestGenerator_isHighlighted_NilConfig(t *testing.T) {
	g := NewGenerator(nil)
	// With default config, Highlight is empty, so nothing should be highlighted
	if g.isHighlighted("any_step") {
		t.Error("isHighlighted() should return false when highlight is not set")
	}
}

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

func TestGenerator_getHighlightStyle_HasBoldStyle(t *testing.T) {
	g := NewGenerator(nil)
	style := g.getHighlightStyle()

	if !strings.Contains(style.Style, "bold") {
		t.Error("getHighlightStyle() should include 'bold' in Style")
	}
}

func TestGenerator_getHighlightStyle_HasBlueColor(t *testing.T) {
	g := NewGenerator(nil)
	style := g.getHighlightStyle()

	// Per spec: color should be #2196F3 (material blue)
	if style.Color != "#2196F3" {
		t.Errorf("getHighlightStyle() Color = %q, want #2196F3", style.Color)
	}
}

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
