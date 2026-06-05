package mcp_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchitecture_AllowedImportsOnly(t *testing.T) {
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

	allowedPrefixes := []string{
		"github.com/modelcontextprotocol/go-sdk/",
		"github.com/awf-project/cli/internal/domain/ports",
	}

	for _, file := range goFiles {
		f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		require.NoError(t, err, "failed to parse %s", file)

		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)

			if !strings.Contains(path, ".") {
				// stdlib: no dot in the first path segment
				continue
			}

			allowed := false
			for _, prefix := range allowedPrefixes {
				if path == prefix || strings.HasPrefix(path, prefix) {
					allowed = true
					break
				}
			}

			if !allowed {
				t.Errorf("disallowed import %q in %s", path, file)
			}
		}
	}
}
