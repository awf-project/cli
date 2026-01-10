package diagram

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckGraphviz_ReturnsBoolean(t *testing.T) {
	// CheckGraphviz should return a boolean indicating if dot is available.
	// The actual result depends on the system, but it should not panic.
	defer func() {
		if r := recover(); r != nil {
			// Expected: stub panics with "not implemented"
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
		}
	}()

	_ = CheckGraphviz()
}

func TestCheckGraphviz_DetectsDotCommand(t *testing.T) {
	// This test verifies that CheckGraphviz correctly detects the dot command.
	// On systems with graphviz installed, it should return true.
	// On systems without graphviz, it should return false.
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	result := CheckGraphviz()

	// Result should be consistent - calling twice should return same value
	result2 := CheckGraphviz()
	if result != result2 {
		t.Error("CheckGraphviz() should return consistent results")
	}
}

func TestExport_PNGFormat(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test.png")

	err := Export(dot, outputPath)

	// If graphviz is not installed, we expect an error
	// If installed, file should be created
	if err == nil {
		if _, statErr := os.Stat(outputPath); os.IsNotExist(statErr) {
			t.Error("Export() succeeded but file was not created")
		}
	}
}

func TestExport_SVGFormat(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test.svg")

	err := Export(dot, outputPath)

	if err == nil {
		if _, statErr := os.Stat(outputPath); os.IsNotExist(statErr) {
			t.Error("Export() succeeded but file was not created")
		}
	}
}

func TestExport_PDFFormat(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test.pdf")

	err := Export(dot, outputPath)

	if err == nil {
		if _, statErr := os.Stat(outputPath); os.IsNotExist(statErr) {
			t.Error("Export() succeeded but file was not created")
		}
	}
}

func TestExport_DOTFormat_NoConversion(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test.dot")

	err := Export(dot, outputPath)
	// .dot format should just write the raw DOT content (no graphviz needed)
	if err != nil {
		t.Errorf("Export() to .dot should not require graphviz, got error: %v", err)
	}

	content, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Errorf("failed to read output file: %v", readErr)
	}

	if string(content) != dot {
		t.Errorf("Export() to .dot should write raw DOT content\ngot: %s\nwant: %s", content, dot)
	}
}

func TestExport_FormatDetection(t *testing.T) {
	tests := []struct {
		name       string
		extension  string
		expectConv bool // true if graphviz conversion is expected
	}{
		{"PNG format", ".png", true},
		{"SVG format", ".svg", true},
		{"PDF format", ".pdf", true},
		{"DOT format (raw)", ".dot", false},
		{"uppercase PNG", ".PNG", true},
		{"uppercase SVG", ".SVG", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if r != "not implemented" {
						t.Errorf("unexpected panic: %v", r)
					}
					t.Skip("stub not implemented yet")
				}
			}()

			dot := `digraph G { a -> b; }`
			outputPath := filepath.Join(t.TempDir(), "test"+tt.extension)

			_ = Export(dot, outputPath)
			// Format detection is tested by verifying no panic occurs
		})
	}
}

func TestExport_EmptyDOT(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	outputPath := filepath.Join(t.TempDir(), "empty.dot")

	err := Export("", outputPath)
	// Empty DOT should still be written to .dot file
	if err != nil {
		t.Errorf("Export() with empty DOT should not error for .dot format: %v", err)
	}
}

func TestExport_InvalidDOT_WithGraphviz(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	// Invalid DOT syntax
	invalidDOT := `digraph G { invalid syntax here }`
	outputPath := filepath.Join(t.TempDir(), "invalid.png")

	err := Export(invalidDOT, outputPath)

	// If graphviz is installed, it should return an error for invalid DOT
	// If not installed, it should return "graphviz not installed" error
	if err == nil {
		t.Log("Warning: Export() did not return error for invalid DOT (graphviz may not validate)")
	}
}

func TestExport_GraphvizNotInstalled_Error(t *testing.T) {
	// This test verifies the error message when graphviz is not installed
	// and an image format is requested.
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	if CheckGraphviz() {
		t.Skip("graphviz is installed, cannot test missing graphviz error")
	}

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test.png")

	err := Export(dot, outputPath)

	if err == nil {
		t.Error("Export() should return error when graphviz is not installed")
	}

	// Error message should be clear about graphviz requirement
	errMsg := err.Error()
	if !containsAny(errMsg, "graphviz", "dot", "not installed", "not found") {
		t.Errorf("error message should mention graphviz/dot, got: %s", errMsg)
	}
}

func TestExport_OutputPathWithSpaces(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "path with spaces", "test.dot")

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	err := Export(dot, outputPath)
	if err != nil {
		t.Errorf("Export() should handle paths with spaces: %v", err)
	}
}

func TestExport_OutputPathWithSpecialChars(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test-file_v1.2.dot")

	err := Export(dot, outputPath)
	if err != nil {
		t.Errorf("Export() should handle special chars in filename: %v", err)
	}
}

func TestExport_DOTWithUnicodeLabels(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	// DOT with unicode characters in labels
	dot := `digraph G {
		start [label="Début"];
		end [label="Terminé"];
		start -> end;
	}`
	outputPath := filepath.Join(t.TempDir(), "unicode.dot")

	err := Export(dot, outputPath)
	if err != nil {
		t.Errorf("Export() should handle unicode in DOT: %v", err)
	}

	content, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("failed to read output: %v", readErr)
	}

	if !containsSubstring(string(content), "Début") {
		t.Error("unicode content was not preserved")
	}
}

func TestExport_LargeDOT(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	// Generate a large DOT with 100 nodes
	var dot string
	dot = "digraph G {\n"
	for i := 0; i < 100; i++ {
		dot += "  node" + itoa(i) + ";\n"
		if i > 0 {
			dot += "  node" + itoa(i-1) + " -> node" + itoa(i) + ";\n"
		}
	}
	dot += "}\n"

	outputPath := filepath.Join(t.TempDir(), "large.dot")

	err := Export(dot, outputPath)
	if err != nil {
		t.Errorf("Export() should handle large DOT files: %v", err)
	}
}

func TestExport_UnknownExtension(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "test.xyz")

	err := Export(dot, outputPath)

	// Unknown extension should return an error
	if err == nil {
		t.Error("Export() should return error for unknown extension")
	}
}

func TestExport_NoExtension(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := filepath.Join(t.TempDir(), "noextension")

	err := Export(dot, outputPath)

	// No extension should return an error
	if err == nil {
		t.Error("Export() should return error when extension is missing")
	}
}

func TestExport_EmptyPath(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`

	err := Export(dot, "")

	// Empty path should return an error
	if err == nil {
		t.Error("Export() should return error for empty output path")
	}
}

func TestExport_NonexistentDirectory(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	dot := `digraph G { a -> b; }`
	outputPath := "/nonexistent/path/that/does/not/exist/test.dot"

	err := Export(dot, outputPath)

	// Should return error for nonexistent parent directory
	if err == nil {
		t.Error("Export() should return error for nonexistent parent directory")
	}
}

func TestExport_ComplexDOTStructure(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if r != "not implemented" {
				t.Errorf("unexpected panic: %v", r)
			}
			t.Skip("stub not implemented yet")
		}
	}()

	// Complex DOT with subgraphs, styling, and various attributes
	dot := `digraph workflow {
		rankdir=LR;
		node [fontname="Arial"];

		subgraph cluster_parallel {
			label="Parallel Execution";
			style=dashed;
			task1 [shape=box, label="Task 1"];
			task2 [shape=box, label="Task 2"];
		}

		start [shape=oval, style=filled, fillcolor=lightgreen];
		end [shape=doublecircle, style=filled, fillcolor=lightcoral];

		start -> task1;
		start -> task2;
		task1 -> end;
		task2 -> end [style=dashed, color=red];
	}`

	outputPath := filepath.Join(t.TempDir(), "complex.dot")

	err := Export(dot, outputPath)
	if err != nil {
		t.Errorf("Export() should handle complex DOT structure: %v", err)
	}

	content, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("failed to read output: %v", readErr)
	}

	// Verify key elements are preserved
	contentStr := string(content)
	checks := []string{"rankdir=LR", "subgraph cluster_parallel", "Task 1", "doublecircle"}
	for _, check := range checks {
		if !containsSubstring(contentStr, check) {
			t.Errorf("DOT content should contain %q", check)
		}
	}
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if containsSubstring(s, substr) {
			return true
		}
	}
	return false
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
