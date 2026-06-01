package acpserver_test

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

func TestArchitecture_NoInternalImports(t *testing.T) {
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

	require.NotEmpty(t, goFiles, "no Go files found in package")

	var allImports []string
	for _, file := range goFiles {
		f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, err, "failed to parse %s", file)

		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			allImports = append(allImports, path)
		}
	}

	for _, imp := range allImports {
		assert.False(
			t,
			strings.HasPrefix(imp, "github.com/awf-project/cli/internal/"),
			"pkg/acpserver must not import from internal/; found import: %s",
			imp,
		)
	}
}
