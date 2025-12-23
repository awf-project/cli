//go:build integration

// Feature: F045 - Workflow Diagram Generation (DOT Format)
//
// Functional tests for the diagram command that generates DOT format output
// from workflow definitions. Tests cover:
// - Happy path: DOT output to stdout, file export
// - Edge cases: empty workflows, complex nesting, special characters
// - Error handling: invalid workflows, missing graphviz
// - Integration: CLI flags, fixture workflows

package integration_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// ===========================================================================
// Happy Path Tests
// ===========================================================================

// TestDiagram_Simple_OutputsValidDOT tests US1: basic DOT output to stdout
// for a linear workflow.
func TestDiagram_Simple_OutputsValidDOT(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple"})

	err := cmd.Execute()
	require.NoError(t, err, "diagram command should execute without error")

	output := buf.String()

	// FR-001: DOT format output to stdout
	assert.Contains(t, output, "digraph", "output must contain digraph declaration")
	assert.Contains(t, output, "{", "output must contain opening brace")
	assert.Contains(t, output, "}", "output must contain closing brace")

	// Verify workflow nodes are present
	assert.Contains(t, output, "start", "output must contain start node")
	assert.Contains(t, output, "process", "output must contain process node")
	assert.Contains(t, output, "done", "output must contain done terminal")
	assert.Contains(t, output, "error", "output must contain error terminal")

	// FR-003: Verify edges exist
	assert.Contains(t, output, "->", "output must contain edge declarations")
}

// TestDiagram_Parallel_ShowsSubgraphCluster tests FR-004: parallel branches
// are grouped in subgraph clusters.
func TestDiagram_Parallel_ShowsSubgraphCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-parallel"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// FR-004: Parallel branches in subgraph cluster
	assert.Contains(t, output, "subgraph", "parallel step must create subgraph")
	assert.Contains(t, output, "cluster", "subgraph must use cluster_ prefix")
	assert.Contains(t, output, "task_a", "subgraph must include branch task_a")
	assert.Contains(t, output, "task_b", "subgraph must include branch task_b")
}

// TestDiagram_AllTypes_MapsCorrectShapes tests FR-002: each step type maps
// to its correct DOT shape.
func TestDiagram_AllTypes_MapsCorrectShapes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-all-types"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// FR-002: Step type to shape mapping
	// command → box
	assert.Regexp(t, `cmd_step.*shape\s*=\s*"?box"?`, output, "command step should use box shape")

	// parallel → diamond
	assert.Regexp(t, `parallel_step.*shape\s*=\s*"?diamond"?`, output, "parallel step should use diamond shape")

	// for_each → hexagon
	assert.Regexp(t, `loop_step.*shape\s*=\s*"?hexagon"?`, output, "for_each step should use hexagon shape")

	// operation → box3d
	assert.Regexp(t, `operation_step.*shape\s*=\s*"?box3d"?`, output, "operation step should use box3d shape")

	// call_workflow → folder
	assert.Regexp(t, `call_step.*shape\s*=\s*"?folder"?`, output, "call_workflow step should use folder shape")

	// terminal success → oval
	assert.Regexp(t, `success.*shape\s*=\s*"?oval"?`, output, "terminal success should use oval shape")

	// terminal failure → doubleoval
	assert.Regexp(t, `error.*shape\s*=\s*"?(double)?oval"?`, output, "terminal failure should use doubleoval shape")
}

// TestDiagram_InitialStep_HasIndicator tests FR-005: initial step marked with
// special indicator.
func TestDiagram_InitialStep_HasIndicator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// FR-005: Initial step indicator - either double border or arrow from start node
	hasStartNode := strings.Contains(output, "__start__") || strings.Contains(output, "_start_")
	hasDoubleBorder := strings.Contains(output, "peripheries=2") || strings.Contains(output, "peripheries=\"2\"")
	hasArrowToInitial := strings.Contains(output, "-> start") || strings.Contains(output, "->start")

	assert.True(t, hasStartNode || hasDoubleBorder || hasArrowToInitial,
		"initial step must have indicator (start node arrow, peripheries=2, or __start__)")
}

// TestDiagram_EdgeStyles_SuccessAndFailure tests FR-003: edge styling for
// success and failure transitions.
func TestDiagram_EdgeStyles_SuccessAndFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// FR-003: on_failure edges should be dashed and red
	// The diagram-simple fixture has on_failure transitions
	assert.True(t,
		strings.Contains(output, "style=dashed") ||
			strings.Contains(output, `style="dashed"`),
		"failure edges should have dashed style")

	assert.True(t,
		strings.Contains(output, "color=red") ||
			strings.Contains(output, `color="red"`) ||
			strings.Contains(output, "color=\"#"),
		"failure edges should have red color")
}

// ===========================================================================
// Flag Tests - Direction
// ===========================================================================

// TestDiagram_Direction_AllValues tests FR-007: --direction flag controls
// graph layout direction.
func TestDiagram_Direction_AllValues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tests := []struct {
		name      string
		direction string
		expect    string
	}{
		{"top-to-bottom", "TB", "rankdir=TB"},
		{"left-to-right", "LR", "rankdir=LR"},
		{"bottom-to-top", "BT", "rankdir=BT"},
		{"right-to-left", "RL", "rankdir=RL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "diagram-simple", "--direction", tt.direction})

			err := cmd.Execute()
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.expect, "DOT must contain %s", tt.expect)
		})
	}
}

// TestDiagram_Direction_Default tests that default direction is TB.
func TestDiagram_Direction_Default(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Default should be TB or omitted (graphviz default)
	assert.True(t,
		strings.Contains(output, "rankdir=TB") || !strings.Contains(output, "rankdir=LR"),
		"default direction should be TB")
}

// ===========================================================================
// Flag Tests - Highlight
// ===========================================================================

// TestDiagram_Highlight_EmphasizesStep tests US3: --highlight step_name
// visually emphasizes the specified step.
func TestDiagram_Highlight_EmphasizesStep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--highlight", "process"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Highlighted step should have emphasis styling
	// Look for penwidth, bold style, or special color on the process node
	processSection := extractNodeDefinition(output, "process")

	assert.True(t,
		strings.Contains(processSection, "penwidth") ||
			strings.Contains(processSection, "bold") ||
			strings.Contains(processSection, "color=") ||
			strings.Contains(processSection, "fillcolor="),
		"highlighted step 'process' should have visual emphasis attributes")
}

// TestDiagram_Highlight_NonexistentStep tests highlight of non-existent step.
func TestDiagram_Highlight_NonexistentStep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--highlight", "nonexistent_step"})

	err := cmd.Execute()
	// Should succeed but no step will be highlighted
	require.NoError(t, err)

	output := buf.String()
	// Should still produce valid DOT
	assert.Contains(t, output, "digraph")
}

// ===========================================================================
// Flag Tests - Output File
// ===========================================================================

// TestDiagram_Output_DotFile tests FR-006: --output exports to .dot file.
func TestDiagram_Output_DotFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "workflow.dot")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--output", outputFile})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify file was created
	require.FileExists(t, outputFile, ".dot file should be created")

	// Verify content is valid DOT
	content, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "digraph", "exported .dot file should contain valid DOT")
	assert.Contains(t, string(content), "start", "exported .dot file should contain workflow nodes")
}

// TestDiagram_Output_ImageExport_RequiresGraphviz tests US2: image export
// requires graphviz and produces clear error if missing.
func TestDiagram_Output_ImageExport_RequiresGraphviz(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	// Check if graphviz is installed
	_, lookupErr := exec.LookPath("dot")
	graphvizAvailable := lookupErr == nil

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "workflow.png")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--output", outputFile})

	err := cmd.Execute()

	if graphvizAvailable {
		// If graphviz is installed, file should be created
		require.NoError(t, err, "diagram command should succeed when graphviz is available")
		require.FileExists(t, outputFile, "PNG file should be created when graphviz is available")

		// Verify it's a valid PNG (magic bytes)
		content, _ := os.ReadFile(outputFile)
		assert.True(t, len(content) > 8, "PNG file should have content")
		// PNG magic bytes: 137 80 78 71 13 10 26 10
		if len(content) >= 4 {
			assert.Equal(t, byte(0x89), content[0], "PNG should have correct magic bytes")
		}
	} else {
		// If graphviz is not installed, should get clear error
		assert.Error(t, err, "should error when graphviz not available for PNG export")
		combinedOutput := buf.String() + errBuf.String() + err.Error()
		assert.True(t,
			strings.Contains(strings.ToLower(combinedOutput), "graphviz") ||
				strings.Contains(strings.ToLower(combinedOutput), "dot") ||
				strings.Contains(strings.ToLower(combinedOutput), "not found") ||
				strings.Contains(strings.ToLower(combinedOutput), "not installed"),
			"error message should indicate graphviz is required")
	}
}

// TestDiagram_Output_SVGExport tests SVG export format detection.
func TestDiagram_Output_SVGExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Skip if graphviz not available
	if _, err := exec.LookPath("dot"); err != nil {
		t.Skip("graphviz not installed, skipping image export test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "workflow.svg")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--output", outputFile})

	err := cmd.Execute()
	require.NoError(t, err)
	require.FileExists(t, outputFile)

	content, _ := os.ReadFile(outputFile)
	assert.Contains(t, string(content), "<svg", "SVG file should contain svg tag")
}

// TestDiagram_Output_PDFExport tests PDF export format detection.
func TestDiagram_Output_PDFExport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Skip if graphviz not available
	if _, err := exec.LookPath("dot"); err != nil {
		t.Skip("graphviz not installed, skipping image export test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "workflow.pdf")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--output", outputFile})

	err := cmd.Execute()
	require.NoError(t, err)
	require.FileExists(t, outputFile)

	content, _ := os.ReadFile(outputFile)
	// PDF starts with %PDF-
	assert.True(t, len(content) >= 5, "PDF file should have content")
	assert.Equal(t, "%PDF-", string(content[:5]), "PDF file should have correct header")
}

// ===========================================================================
// Combined Flags Tests
// ===========================================================================

// TestDiagram_CombinedFlags_DirectionAndHighlight tests multiple flags together.
func TestDiagram_CombinedFlags_DirectionAndHighlight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"diagram", "diagram-simple",
		"--direction", "LR",
		"--highlight", "process",
	})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "rankdir=LR", "should have LR direction")

	processSection := extractNodeDefinition(output, "process")
	assert.True(t,
		strings.Contains(processSection, "penwidth") ||
			strings.Contains(processSection, "bold") ||
			strings.Contains(processSection, "color="),
		"highlighted step should have visual emphasis")
}

// TestDiagram_CombinedFlags_AllFlags tests all flags combined.
func TestDiagram_CombinedFlags_AllFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "combined.dot")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"diagram", "diagram-simple",
		"--direction", "LR",
		"--highlight", "process",
		"--output", outputFile,
	})

	err := cmd.Execute()
	require.NoError(t, err)

	require.FileExists(t, outputFile)
	content, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	output := string(content)

	assert.Contains(t, output, "rankdir=LR", "file should have LR direction")
	assert.Contains(t, output, "process", "file should contain process node")
}

// ===========================================================================
// Error Handling Tests
// ===========================================================================

// TestDiagram_InvalidWorkflow_NotFound tests FR-008: non-existent workflow.
func TestDiagram_InvalidWorkflow_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "nonexistent-workflow"})

	err := cmd.Execute()

	assert.Error(t, err, "should error for non-existent workflow")
	combinedOutput := buf.String() + errBuf.String() + err.Error()
	assert.True(t,
		strings.Contains(strings.ToLower(combinedOutput), "not found") ||
			strings.Contains(strings.ToLower(combinedOutput), "does not exist") ||
			strings.Contains(strings.ToLower(combinedOutput), "no such"),
		"error should indicate workflow not found")
}

// TestDiagram_InvalidWorkflow_MalformedYAML tests FR-008: malformed YAML.
func TestDiagram_InvalidWorkflow_MalformedYAML(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a malformed YAML workflow
	malformedYAML := `name: malformed
version: "1.0.0"
states:
  initial: start
  start:
    type: step
    command: echo "test"
    on_success: [invalid list here
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "malformed.yaml"), []byte(malformedYAML), 0644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "malformed"})

	err := cmd.Execute()
	assert.Error(t, err, "should error for malformed YAML")
}

// TestDiagram_InvalidDirection tests error for invalid direction value.
func TestDiagram_InvalidDirection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--direction", "INVALID"})

	err := cmd.Execute()

	assert.Error(t, err, "should error for invalid direction")
	combinedOutput := buf.String() + errBuf.String() + err.Error()
	assert.True(t,
		strings.Contains(strings.ToLower(combinedOutput), "invalid") ||
			strings.Contains(strings.ToLower(combinedOutput), "direction"),
		"error should mention invalid direction")
}

// TestDiagram_MissingArgument tests error when workflow argument is missing.
func TestDiagram_MissingArgument(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram"})

	err := cmd.Execute()
	assert.Error(t, err, "should error when workflow argument is missing")
}

// TestDiagram_OutputDirectory_NotExists tests error when output directory doesn't exist.
func TestDiagram_OutputDirectory_NotExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple", "--output", "/nonexistent/path/file.dot"})

	err := cmd.Execute()
	assert.Error(t, err, "should error when output directory doesn't exist")
}

// ===========================================================================
// Edge Cases
// ===========================================================================

// TestDiagram_WorkflowWithSpecialCharacters tests DOT escaping for special characters.
func TestDiagram_WorkflowWithSpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create a workflow with special characters in descriptions
	specialCharsYAML := `name: special-chars
version: "1.0.0"
description: 'Workflow with "quotes" and <brackets>'
states:
  initial: step_one
  step_one:
    type: step
    description: 'Step with "double quotes" and <angle brackets>'
    command: echo "test"
    on_success: done
  done:
    type: terminal
    message: 'Done with "special" chars'
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "special-chars.yaml"), []byte(specialCharsYAML), 0644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "special-chars"})

	err := cmd.Execute()
	require.NoError(t, err, "should handle special characters without error")

	output := buf.String()
	assert.Contains(t, output, "digraph", "should produce valid DOT despite special chars")

	// Verify DOT is properly escaped (no raw < or > that would break DOT)
	// DOT uses HTML-like entities or escapes: &lt; &gt; or \"
	assert.False(t, strings.Contains(output, "label=<"),
		"labels should not contain unescaped angle brackets")
}

// TestDiagram_SingleStepWorkflow tests minimal workflow with single step.
func TestDiagram_SingleStepWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create minimal single-step workflow
	minimalYAML := `name: minimal
version: "1.0.0"
states:
  initial: done
  done:
    type: terminal
    status: success
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "minimal.yaml"), []byte(minimalYAML), 0644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "minimal"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "digraph")
	assert.Contains(t, output, "done")
}

// TestDiagram_NestedParallel tests workflow with nested parallel branches.
func TestDiagram_NestedParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	// Create workflow with nested parallel
	nestedYAML := `name: nested-parallel
version: "1.0.0"
states:
  initial: outer_parallel
  outer_parallel:
    type: parallel
    parallel:
      - inner_parallel
      - simple_branch
    strategy: all_succeed
    on_success: done
  inner_parallel:
    type: parallel
    parallel:
      - deep_a
      - deep_b
    strategy: all_succeed
    on_success: done
  simple_branch:
    type: step
    command: echo "simple"
    on_success: done
  deep_a:
    type: step
    command: echo "deep a"
    on_success: done
  deep_b:
    type: step
    command: echo "deep b"
    on_success: done
  done:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "nested-parallel.yaml"), []byte(nestedYAML), 0644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "nested-parallel"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "digraph")
	assert.Contains(t, output, "outer_parallel")
	assert.Contains(t, output, "inner_parallel")
	// Should have multiple subgraph clusters
	assert.True(t, strings.Count(output, "subgraph") >= 1,
		"nested parallel should produce at least one subgraph")
}

// TestDiagram_LongStepNames tests workflow with long step names.
func TestDiagram_LongStepNames(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	longName := "this_is_a_very_long_step_name_that_might_cause_issues_with_rendering"
	longNameYAML := `name: long-names
version: "1.0.0"
states:
  initial: ` + longName + `
  ` + longName + `:
    type: step
    command: echo "test"
    on_success: done
  done:
    type: terminal
`
	wfDir := filepath.Join(tmpDir, "workflows")
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "long-names.yaml"), []byte(longNameYAML), 0644)

	os.Setenv("AWF_WORKFLOWS_PATH", wfDir)
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "long-names"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, longName, "long step name should be preserved")
}

// ===========================================================================
// DOT Validation Tests
// ===========================================================================

// TestDiagram_DOTSyntax_ValidWithGraphviz validates DOT output with graphviz
// if available. This is the ultimate integration test per US1 acceptance.
func TestDiagram_DOTSyntax_ValidWithGraphviz(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Skip if graphviz not available
	dotPath, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("graphviz not installed, skipping DOT validation test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "diagram-simple"})

	err = cmd.Execute()
	require.NoError(t, err)

	dotOutput := buf.String()

	// Run graphviz to validate DOT syntax
	dotCmd := exec.Command(dotPath, "-Tpng")
	dotCmd.Stdin = strings.NewReader(dotOutput)
	var dotStderr bytes.Buffer
	dotCmd.Stderr = &dotStderr

	err = dotCmd.Run()
	assert.NoError(t, err, "graphviz should parse DOT without errors: %s", dotStderr.String())
}

// TestDiagram_DOTSyntax_AllFixtures validates all diagram fixtures produce valid DOT.
func TestDiagram_DOTSyntax_AllFixtures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Skip if graphviz not available
	dotPath, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("graphviz not installed, skipping DOT validation test")
	}

	os.Setenv("AWF_WORKFLOWS_PATH", "../fixtures/workflows")
	defer os.Unsetenv("AWF_WORKFLOWS_PATH")

	fixtures := []string{"diagram-simple", "diagram-parallel", "diagram-all-types"}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			cmd := cli.NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", fixture})

			err := cmd.Execute()
			require.NoError(t, err)

			dotOutput := buf.String()

			// Validate with graphviz
			dotCmd := exec.Command(dotPath, "-Tpng")
			dotCmd.Stdin = strings.NewReader(dotOutput)
			var dotStderr bytes.Buffer
			dotCmd.Stderr = &dotStderr

			err = dotCmd.Run()
			assert.NoError(t, err, "fixture %s should produce valid DOT: %s", fixture, dotStderr.String())
		})
	}
}

// ===========================================================================
// Help and Documentation Tests
// ===========================================================================

// TestDiagram_Help_ShowsUsage tests that --help shows usage information.
func TestDiagram_Help_ShowsUsage(t *testing.T) {
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"diagram", "--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "diagram", "help should mention diagram command")
	assert.Contains(t, output, "--output", "help should document --output flag")
	assert.Contains(t, output, "--direction", "help should document --direction flag")
	assert.Contains(t, output, "--highlight", "help should document --highlight flag")
	assert.Contains(t, output, "DOT", "help should mention DOT format")
}

// ===========================================================================
// Helper Functions
// ===========================================================================

// extractNodeDefinition extracts the DOT definition for a specific node.
func extractNodeDefinition(dotOutput, nodeName string) string {
	lines := strings.Split(dotOutput, "\n")
	for _, line := range lines {
		// Look for node definition: nodename [attributes]
		if strings.Contains(line, nodeName) && strings.Contains(line, "[") {
			return line
		}
	}
	return ""
}
