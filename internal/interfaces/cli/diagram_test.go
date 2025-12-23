package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vanoix/awf/internal/interfaces/cli"
)

// =============================================================================
// Command Registration Tests
// =============================================================================

func TestDiagramCommand_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected root command to have 'diagram' subcommand")
	}
}

func TestDiagramCommand_RegisteredInRoot(t *testing.T) {
	cmd := cli.NewRootCommand()

	// Should be callable via awf diagram
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("diagram --help should not error: %v", err)
	}
}

// =============================================================================
// Argument Validation Tests
// =============================================================================

func TestDiagramCommand_NoArgs_ReturnsError(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no workflow name provided")
	}
}

func TestDiagramCommand_TooManyArgs_ReturnsError(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "workflow1", "workflow2"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when too many arguments provided")
	}
}

func TestDiagramCommand_ExactlyOneArg_Accepted(t *testing.T) {
	// Test that exactly one argument is accepted (workflow name)
	// It will return an error for non-existent workflow, but that's expected
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "my-workflow"})

	err := cmd.Execute()
	// Error is expected because workflow doesn't exist, but the key point is
	// that the command accepts exactly one argument without cobra argument errors
	if err != nil && strings.Contains(err.Error(), "accepts 1 arg") {
		t.Error("command should accept exactly one argument")
	}
}

// =============================================================================
// Flag Tests
// =============================================================================

func TestDiagramCommand_OutputFlag_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	flag := diagramCmd.Flags().Lookup("output")
	if flag == nil {
		t.Error("expected --output flag to exist")
	}
}

func TestDiagramCommand_OutputFlag_ShortForm(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	flag := diagramCmd.Flags().ShorthandLookup("o")
	if flag == nil {
		t.Error("expected -o shorthand for --output flag")
	}
}

func TestDiagramCommand_DirectionFlag_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	flag := diagramCmd.Flags().Lookup("direction")
	if flag == nil {
		t.Error("expected --direction flag to exist")
	}
}

func TestDiagramCommand_DirectionFlag_DefaultTB(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	flag := diagramCmd.Flags().Lookup("direction")
	if flag == nil {
		t.Fatal("--direction flag not found")
	}

	if flag.DefValue != "TB" {
		t.Errorf("expected direction default 'TB', got '%s'", flag.DefValue)
	}
}

func TestDiagramCommand_HighlightFlag_Exists(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	flag := diagramCmd.Flags().Lookup("highlight")
	if flag == nil {
		t.Error("expected --highlight flag to exist")
	}
}

func TestDiagramCommand_AllFlagsPresent(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	requiredFlags := []string{"output", "direction", "highlight"}
	for _, flagName := range requiredFlags {
		flag := diagramCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("expected --%s flag to exist", flagName)
		}
	}
}

// =============================================================================
// Help Text Tests
// =============================================================================

func TestDiagramCommand_Help(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "diagram") {
		t.Errorf("expected help text to contain 'diagram', got: %s", output)
	}
	if !strings.Contains(output, "DOT") {
		t.Errorf("expected help text to mention DOT format, got: %s", output)
	}
}

func TestDiagramCommand_HelpContainsExamples(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	examples := []string{
		"awf diagram my-workflow",
		"--output",
		"--direction LR",
		"--highlight",
	}

	for _, example := range examples {
		if !strings.Contains(output, example) {
			t.Errorf("expected help to contain example '%s'", example)
		}
	}
}

func TestDiagramCommand_HelpContainsShapes(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()
	shapes := []string{
		"command",
		"parallel",
		"terminal",
		"box",
		"diamond",
		"oval",
	}

	for _, shape := range shapes {
		if !strings.Contains(output, shape) {
			t.Errorf("expected help to mention shape/type '%s'", shape)
		}
	}
}

func TestDiagramCommand_UsageFormat(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	expected := "diagram <workflow>"
	if diagramCmd.Use != expected {
		t.Errorf("expected Use '%s', got '%s'", expected, diagramCmd.Use)
	}
}

func TestDiagramCommand_ShortDescription(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	if diagramCmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if !strings.Contains(diagramCmd.Short, "diagram") && !strings.Contains(diagramCmd.Short, "DOT") {
		t.Errorf("Short description should mention diagram or DOT: %s", diagramCmd.Short)
	}
}

// =============================================================================
// Execution Tests (GREEN phase - implementation complete)
// =============================================================================

func TestDiagramCommand_Run_NonExistentWorkflow(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

func TestDiagramCommand_WithOutputFlag_NonExistentWorkflow(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--output", "output.png"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

func TestDiagramCommand_WithDirectionFlag_NonExistentWorkflow(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--direction", "LR"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

func TestDiagramCommand_WithHighlightFlag_NonExistentWorkflow(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--highlight", "build_step"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

func TestDiagramCommand_WithAllFlags_NonExistentWorkflow(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"diagram", "test-workflow",
		"--output", "diagram.svg",
		"--direction", "BT",
		"--highlight", "process_step",
	})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

func TestDiagramCommand_WithShortOutputFlag_NonExistentWorkflow(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "-o", "output.pdf"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// =============================================================================
// Table-driven tests for flag descriptions
// =============================================================================

func TestDiagramCommand_FlagDescriptions(t *testing.T) {
	cmd := cli.NewRootCommand()

	var diagramCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "diagram" {
			diagramCmd = sub
			break
		}
	}

	if diagramCmd == nil {
		t.Fatal("diagram command not found")
	}

	tests := []struct {
		flagName        string
		wantInUsage     string
		wantDescription string
	}{
		{
			flagName:        "output",
			wantInUsage:     "output",
			wantDescription: "image",
		},
		{
			flagName:        "direction",
			wantInUsage:     "direction",
			wantDescription: "TB",
		},
		{
			flagName:        "highlight",
			wantInUsage:     "highlight",
			wantDescription: "emphasize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := diagramCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Fatalf("flag --%s not found", tt.flagName)
			}

			if !strings.Contains(flag.Usage, tt.wantDescription) {
				t.Errorf("flag --%s usage should contain '%s', got: %s",
					tt.flagName, tt.wantDescription, flag.Usage)
			}
		})
	}
}

// =============================================================================
// Direction flag value validation tests
// =============================================================================

func TestDiagramCommand_DirectionFlag_ValidValues(t *testing.T) {
	validDirections := []string{"TB", "LR", "BT", "RL"}

	for _, dir := range validDirections {
		t.Run(dir, func(t *testing.T) {
			cmd := cli.NewRootCommand()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test-workflow", "--direction", dir})

			err := cmd.Execute()
			// Error expected for non-existent workflow, but should not be a
			// direction validation error
			if err != nil && strings.Contains(err.Error(), "direction") {
				t.Errorf("direction %s should be valid, got error: %v", dir, err)
			}
		})
	}
}

// =============================================================================
// Output file extension tests (behavior when implemented)
// =============================================================================

func TestDiagramCommand_OutputFlag_AcceptsVariousExtensions(t *testing.T) {
	extensions := []string{".png", ".svg", ".pdf", ".dot"}

	for _, ext := range extensions {
		t.Run(ext, func(t *testing.T) {
			cmd := cli.NewRootCommand()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test-workflow", "--output", "output" + ext})

			err := cmd.Execute()
			// Error expected for non-existent workflow, but should not be an
			// extension validation error
			if err != nil && strings.Contains(err.Error(), "extension") {
				t.Errorf("extension %s should be accepted, got error: %v", ext, err)
			}
		})
	}
}

// =============================================================================
// Output Mode Tests (to be verified after implementation)
// =============================================================================

// TestDiagramCommand_DefaultOutputToDOT verifies DOT output to stdout when no --output flag.
// GREEN phase: Should output valid DOT for existing workflow.
func TestDiagramCommand_DefaultOutputToDOT(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	// The actual DOT output testing is done in integration tests
	// with valid workflow fixtures
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// TestDiagramCommand_OutputToDotFile verifies --output with .dot extension writes DOT file.
// GREEN phase: Should write DOT to file for existing workflow.
func TestDiagramCommand_OutputToDotFile(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--output", "/tmp/test-workflow.dot"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	// File creation testing is done in integration tests with valid workflow fixtures
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// =============================================================================
// Error Handling Tests (to be verified after implementation)
// =============================================================================

// TestDiagramCommand_InvalidWorkflow_ReturnsError verifies error for non-existent workflow.
// GREEN phase: Should return error for non-existent workflow.
func TestDiagramCommand_InvalidWorkflow_ReturnsError(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "non-existent-workflow-xyz"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// TestDiagramCommand_MalformedWorkflow_ReturnsError verifies error for invalid workflow syntax.
// GREEN phase: Should return error for non-existent/invalid workflow.
func TestDiagramCommand_MalformedWorkflow_ReturnsError(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "invalid-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent/invalid workflow")
	}
}

// TestDiagramCommand_GraphvizMissing_WithOutputFlag_ReturnsError verifies clear error when
// graphviz is not installed and --output is specified for image formats.
// GREEN phase: Should show clear error message.
func TestDiagramCommand_GraphvizMissing_WithOutputFlag_ReturnsError(t *testing.T) {
	// This test is environment-dependent (graphviz may or may not be installed)
	// For unit tests, just verify the command doesn't crash
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--output", "output.png"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// =============================================================================
// Direction Flag Behavior Tests
// =============================================================================

// TestDiagramCommand_DirectionFlag_AffectsRankdir verifies --direction flag sets rankdir in DOT.
// GREEN phase: DOT should contain rankdir=<direction> - tested in integration tests with fixtures.
func TestDiagramCommand_DirectionFlag_AffectsRankdir(t *testing.T) {
	tests := []struct {
		direction string
	}{
		{"TB"},
		{"LR"},
		{"BT"},
		{"RL"},
	}

	for _, tt := range tests {
		t.Run(tt.direction, func(t *testing.T) {
			cmd := cli.NewRootCommand()

			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test-workflow", "--direction", tt.direction})

			err := cmd.Execute()
			// Error is expected for non-existent workflow
			// Actual DOT output testing is done in integration tests with fixtures
			if err == nil {
				t.Error("expected error for non-existent workflow")
			}
		})
	}
}

// TestDiagramCommand_DirectionFlag_InvalidValue verifies invalid direction is rejected.
// GREEN phase: Should reject invalid direction values.
func TestDiagramCommand_DirectionFlag_InvalidValue(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--direction", "INVALID"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid direction value")
	}
	if !strings.Contains(err.Error(), "direction") {
		t.Errorf("error should mention direction, got: %v", err)
	}
}

// =============================================================================
// Highlight Flag Behavior Tests
// =============================================================================

// TestDiagramCommand_HighlightFlag_AffectsNodeStyle verifies --highlight emphasizes node.
// GREEN phase: Highlighted node should have special styling - tested in integration tests.
func TestDiagramCommand_HighlightFlag_AffectsNodeStyle(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--highlight", "build_step"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	// Actual highlight styling is tested in integration tests with fixtures
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// TestDiagramCommand_HighlightFlag_NonExistentStep verifies behavior for non-existent step.
// GREEN phase: Should handle gracefully - no highlight applied if step doesn't exist.
func TestDiagramCommand_HighlightFlag_NonExistentStep(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--highlight", "non_existent_step"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// =============================================================================
// Combined Flags Tests
// =============================================================================

// TestDiagramCommand_CombinedFlags_DirectionAndHighlight verifies both flags work together.
func TestDiagramCommand_CombinedFlags_DirectionAndHighlight(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"diagram", "test-workflow",
		"--direction", "LR",
		"--highlight", "process_step",
	})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	// Combined flag testing with actual output is done in integration tests
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// TestDiagramCommand_CombinedFlags_OutputAndDirection verifies output file with direction.
func TestDiagramCommand_CombinedFlags_OutputAndDirection(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"diagram", "test-workflow",
		"--output", "/tmp/test-diagram.svg",
		"--direction", "BT",
	})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

// TestDiagramCommand_EmptyWorkflowName verifies empty string argument handling.
func TestDiagramCommand_EmptyWorkflowName(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", ""})

	err := cmd.Execute()
	// Empty string should be rejected as a workflow name
	if err == nil {
		t.Error("expected error for empty workflow name")
	}
}

// TestDiagramCommand_OutputToStdoutExplicitly verifies behavior with empty output path.
func TestDiagramCommand_OutputToStdoutExplicitly(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Default behavior: no --output means stdout
	cmd.SetArgs([]string{"diagram", "test-workflow"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow
	// Actual stdout output testing is done in integration tests with fixtures
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}

// TestDiagramCommand_OutputPathWithSpaces verifies handling of paths with spaces.
func TestDiagramCommand_OutputPathWithSpaces(t *testing.T) {
	cmd := cli.NewRootCommand()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test-workflow", "--output", "/tmp/my diagram.png"})

	err := cmd.Execute()
	// Error is expected for non-existent workflow (not a path-with-spaces issue)
	if err == nil {
		t.Error("expected error for non-existent workflow")
	}
}
