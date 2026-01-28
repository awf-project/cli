//go:build integration

// Component: T004
// Feature: C028
package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/interfaces/cli"
	"github.com/vanoix/awf/internal/testutil"
)

// setupDiagramTest creates a test directory with workflows and sets AWF_WORKFLOWS_PATH
func setupDiagramTest(t *testing.T, workflows map[string]string) string {
	t.Helper()
	dir := testutil.SetupWorkflowsDir(t, workflows)
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(dir, ".awf/workflows"))
	return dir
}

// TestRunDiagram_InvalidDirection tests error handling for invalid direction argument
func TestRunDiagram_InvalidDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantErr   string
	}{
		{
			name:      "completely invalid",
			direction: "INVALID",
			wantErr:   "invalid direction \"INVALID\": must be one of TB, LR, BT, RL",
		},
		{
			name:      "lowercase invalid",
			direction: "xyz",
			wantErr:   "invalid direction",
		},
		{
			name:      "numeric invalid",
			direction: "123",
			wantErr:   "invalid direction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create test directory with valid workflow
			setupDiagramTest(t, map[string]string{
				"test.yaml": testutil.SimpleWorkflowYAML,
			})

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test", "--direction", tt.direction})

			// Act: execute diagram command with invalid direction
			err := cmd.Execute()

			// Assert: command fails with appropriate error message
			require.Error(t, err, "diagram should fail for invalid direction")
			assert.Contains(t, err.Error(), tt.wantErr, "error should mention invalid direction")
		})
	}
}

// TestRunDiagram_WorkflowNotFound tests error handling when workflow doesn't exist
func TestRunDiagram_WorkflowNotFound(t *testing.T) {
	// Arrange: create empty test directory
	setupDiagramTest(t, map[string]string{})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "nonexistent"})

	// Act: execute diagram command for missing workflow
	err := cmd.Execute()

	// Assert: command fails with workflow not found error
	require.Error(t, err, "diagram should fail for missing workflow")
	assert.Contains(t, err.Error(), "workflow not found", "error should mention workflow not found")
}

// TestRunDiagram_StdoutOutput tests DOT output to stdout (no --output flag)
func TestRunDiagram_StdoutOutput(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantInOut []string
	}{
		{
			name:      "default direction TB",
			direction: "TB",
			wantInOut: []string{"digraph", "rankdir=TB"},
		},
		{
			name:      "direction LR",
			direction: "LR",
			wantInOut: []string{"digraph", "rankdir=LR"},
		},
		{
			name:      "direction BT",
			direction: "BT",
			wantInOut: []string{"digraph", "rankdir=BT"},
		},
		{
			name:      "direction RL",
			direction: "RL",
			wantInOut: []string{"digraph", "rankdir=RL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create test directory with valid workflow
			setupDiagramTest(t, map[string]string{
				"test.yaml": testutil.SimpleWorkflowYAML,
			})

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test", "--direction", tt.direction})

			// Act: execute diagram command without output file
			err := cmd.Execute()

			// Assert: command succeeds and outputs DOT format to stdout
			require.NoError(t, err, "diagram should succeed")
			output := buf.String()
			for _, want := range tt.wantInOut {
				assert.Contains(t, output, want, "output should contain expected DOT content")
			}
		})
	}
}

// TestRunDiagram_DotFileOutput tests DOT output to .dot file
func TestRunDiagram_DotFileOutput(t *testing.T) {
	// Arrange: create test directory with valid workflow
	dir := setupDiagramTest(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	outputPath := filepath.Join(dir, "output.dot")
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", "test", "--output", outputPath})

	// Act: execute diagram command with .dot output
	err := cmd.Execute()

	// Assert: command succeeds and creates .dot file
	require.NoError(t, err, "diagram should succeed")

	// Verify file exists
	_, err = os.Stat(outputPath)
	require.NoError(t, err, ".dot file should be created")

	// Verify file contains DOT content
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err, "should be able to read .dot file")
	dotContent := string(content)
	assert.Contains(t, dotContent, "digraph", "file should contain DOT graph declaration")
	assert.Contains(t, dotContent, "rankdir", "file should contain direction specification")
}

// TestRunDiagram_ImageExport tests image export behavior (depends on graphviz availability)
func TestRunDiagram_ImageExport(t *testing.T) {
	tests := []struct {
		name      string
		extension string
	}{
		{name: "PNG export", extension: ".png"},
		{name: "SVG export", extension: ".svg"},
		{name: "PDF export", extension: ".pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create test directory with valid workflow
			dir := setupDiagramTest(t, map[string]string{
				"test.yaml": testutil.SimpleWorkflowYAML,
			})

			outputPath := filepath.Join(dir, "output"+tt.extension)
			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test", "--output", outputPath})

			// Act: execute diagram command with image output
			err := cmd.Execute()

			// Assert: behavior depends on graphviz availability
			// If graphviz not installed: should fail with graphviz error
			// If graphviz installed: should succeed and create file
			if err != nil {
				// Error path: graphviz not available
				assert.Contains(t, err.Error(), "graphviz", "error should mention graphviz requirement")
				assert.Contains(t, err.Error(), "not installed", "error should indicate installation needed")
			} else {
				// Success path: graphviz available, file created
				_, err = os.Stat(outputPath)
				assert.NoError(t, err, "image file should be created when graphviz is available")
			}
		})
	}
}

// TestRunDiagram_HighlightOption tests diagram generation with --highlight flag
func TestRunDiagram_HighlightOption(t *testing.T) {
	tests := []struct {
		name         string
		highlightID  string
		wantContains string
	}{
		{
			name:         "highlight start state",
			highlightID:  "start",
			wantContains: "start",
		},
		{
			name:         "highlight done state",
			highlightID:  "done",
			wantContains: "done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create test directory with valid workflow
			setupDiagramTest(t, map[string]string{
				"test.yaml": testutil.SimpleWorkflowYAML,
			})

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test", "--highlight", tt.highlightID})

			// Act: execute diagram command with highlight option
			err := cmd.Execute()

			// Assert: command succeeds
			require.NoError(t, err, "diagram should succeed with highlight option")
			output := buf.String()
			assert.Contains(t, output, tt.wantContains, "output should contain highlighted state")
			// Note: Actual highlight styling (color, bold) is implementation detail
			// We verify the command accepts the flag and includes the state
		})
	}
}

// TestRunDiagram_OutputDirectory tests file output to different directory structures
func TestRunDiagram_OutputDirectory(t *testing.T) {
	tests := []struct {
		name       string
		outputPath string
		createDir  bool
	}{
		{
			name:       "current directory",
			outputPath: "output.dot",
			createDir:  false,
		},
		{
			name:       "subdirectory exists",
			outputPath: "diagrams/output.dot",
			createDir:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create test directory with valid workflow
			dir := setupDiagramTest(t, map[string]string{
				"test.yaml": testutil.SimpleWorkflowYAML,
			})

			outputPath := filepath.Join(dir, tt.outputPath)
			if tt.createDir {
				err := os.MkdirAll(filepath.Dir(outputPath), 0o755)
				require.NoError(t, err, "should create output directory")
			}

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test", "--output", outputPath})

			// Act: execute diagram command
			err := cmd.Execute()

			// Assert: command succeeds and creates file
			require.NoError(t, err, "diagram should succeed")
			_, err = os.Stat(outputPath)
			require.NoError(t, err, "output file should be created")
		})
	}
}

// TestRunDiagram_CombinedOptions tests multiple CLI options together
func TestRunDiagram_CombinedOptions(t *testing.T) {
	// Arrange: create test directory with valid workflow
	dir := setupDiagramTest(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	outputPath := filepath.Join(dir, "combined.dot")
	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"diagram",
		"test",
		"--direction", "LR",
		"--highlight", "start",
		"--output", outputPath,
	})

	// Act: execute diagram command with multiple options
	err := cmd.Execute()

	// Assert: command succeeds with all options
	require.NoError(t, err, "diagram should succeed with combined options")

	// Verify file created
	_, err = os.Stat(outputPath)
	require.NoError(t, err, "output file should be created")

	// Verify content reflects options
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err, "should read output file")
	dotContent := string(content)
	assert.Contains(t, dotContent, "rankdir=LR", "output should use LR direction")
	assert.Contains(t, dotContent, "start", "output should include highlighted state")
}

// TestRunDiagram_EmptyWorkflowName tests error handling for empty workflow name
func TestRunDiagram_EmptyWorkflowName(t *testing.T) {
	// Arrange: create test directory
	setupDiagramTest(t, map[string]string{
		"test.yaml": testutil.SimpleWorkflowYAML,
	})

	cmd := cli.NewRootCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"diagram", ""})

	// Act: execute diagram command with empty name
	err := cmd.Execute()

	// Assert: command fails (Cobra validates required positional args)
	require.Error(t, err, "diagram should fail for empty workflow name")
}

// TestRunDiagram_ValidDirections tests all valid direction values
func TestRunDiagram_ValidDirections(t *testing.T) {
	validDirections := []string{"TB", "LR", "BT", "RL"}

	for _, direction := range validDirections {
		t.Run(direction, func(t *testing.T) {
			// Arrange: create test directory with valid workflow
			setupDiagramTest(t, map[string]string{
				"test.yaml": testutil.SimpleWorkflowYAML,
			})

			cmd := cli.NewRootCommand()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"diagram", "test", "--direction", direction})

			// Act: execute diagram command
			err := cmd.Execute()

			// Assert: command succeeds for all valid directions
			require.NoError(t, err, "diagram should succeed for valid direction %s", direction)
			output := buf.String()
			assert.Contains(t, output, "rankdir="+direction, "output should use specified direction")
		})
	}
}
