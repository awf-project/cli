package mcpserver_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArchitecture_NoInternalImports verifies that pkg/mcpserver has zero
// imports from internal/ packages. This ensures the package remains reusable
// and standalone.
func TestArchitecture_NoInternalImports(t *testing.T) {
	pkgPath := "."
	fset := token.NewFileSet()

	// Find all .go files in the current directory (excluding test files)
	entries, err := os.ReadDir(pkgPath)
	require.NoError(t, err)

	var goFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			goFiles = append(goFiles, filepath.Join(pkgPath, name))
		}
	}

	require.NotEmpty(t, goFiles, "no Go files found in package")

	// Parse each file and collect imports
	var allImports []string
	for _, file := range goFiles {
		f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, err, "failed to parse %s", file)

		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			allImports = append(allImports, path)
		}
	}

	// Assert no imports start with "github.com/awf-project/cli/internal/"
	for _, imp := range allImports {
		assert.False(
			t,
			strings.HasPrefix(imp, "github.com/awf-project/cli/internal/"),
			"pkg/mcpserver must not import from internal/; found import: %s",
			imp,
		)
	}
}
