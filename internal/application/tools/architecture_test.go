package tools

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

// Component: T005
// Feature: F099
// Purpose: Verify application layer tools package maintains hexagonal
// architecture boundaries by ensuring no infrastructure imports exist.
// Uses AST walking (go/parser + go/ast) for structural correctness.

// TestArchitecture_NoInfrastructureImports scans all non-test Go files in
// internal/application/tools/ and fails if any import has the prefix
// github.com/awf-project/cli/internal/infrastructure/.
func TestArchitecture_NoInfrastructureImports(t *testing.T) {
	pkgPath := "."
	fset := token.NewFileSet()

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

	require.NotEmpty(t, goFiles, "no Go source files found in package")

	for _, file := range goFiles {
		f, parseErr := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, parseErr, "failed to parse %s", file)

		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			assert.False(
				t,
				strings.HasPrefix(path, "github.com/awf-project/cli/internal/infrastructure/"),
				"application/tools must not import infrastructure packages — violates hexagonal architecture; file %s imports %s",
				file,
				path,
			)
		}
	}
}
