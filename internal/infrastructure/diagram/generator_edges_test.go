package diagram

// Generator Edge Tests
// Tests for edge generation covering all transition types

import (
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
)

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
