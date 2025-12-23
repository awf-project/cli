package diagram

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckGraphviz detects if the graphviz dot command is available in PATH.
// Returns true if dot command is found and executable, false otherwise.
// Used to validate graphviz availability before image export operations.
func CheckGraphviz() bool {
	_, err := exec.LookPath("dot")
	return err == nil
}

// Export converts DOT format input to an image file using graphviz.
// The output format is determined by the file extension of outputPath:
//   - .png → PNG image
//   - .svg → SVG vector
//   - .pdf → PDF document
//   - .dot → raw DOT file (no conversion)
//
// Returns an error if graphviz is not installed or conversion fails.
func Export(dot string, outputPath string) error {
	ext := strings.ToLower(filepath.Ext(outputPath))

	// For .dot files, just write the DOT content directly
	if ext == ".dot" {
		return os.WriteFile(outputPath, []byte(dot), 0600)
	}

	// Determine output format from extension
	format := ""
	switch ext {
	case ".png":
		format = "png"
	case ".svg":
		format = "svg"
	case ".pdf":
		format = "pdf"
	default:
		return fmt.Errorf("unsupported output format: %s (supported: .png, .svg, .pdf, .dot)", ext)
	}

	// Run graphviz dot command
	cmd := exec.Command("dot", "-T"+format, "-o", outputPath)
	cmd.Stdin = strings.NewReader(dot)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("graphviz export failed: %w: %s", err, string(output))
	}

	return nil
}
