package transcript_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitecture_DomainTranscript_NoForbiddenImports(t *testing.T) {
	fset := token.NewFileSet()

	filterNonTest := func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}

	//nolint:staticcheck // SA1019: ParseDir suffices for an import-only AST scan; build-tag precision is unnecessary here.
	pkgs, err := parser.ParseDir(fset, ".", filterNonTest, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse package directory: %v", err)
	}

	if len(pkgs) == 0 {
		t.Fatal("no Go files found in package directory")
	}

	allowedImports := map[string]struct{}{
		"encoding/json": {},
		"errors":        {},
		"fmt":           {},
		"time":          {},
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if _, ok := allowedImports[path]; !ok {
					t.Errorf("disallowed import %q in %s", path, filepath.Base(name))
				}
			}
		}
	}
}

func TestArchitecture_DomainTranscript_NoTestImportsLeak(t *testing.T) {
	fset := token.NewFileSet()

	filterTestOnly := func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), "_test.go")
	}

	//nolint:staticcheck // SA1019: ParseDir suffices for an import-only AST scan; build-tag precision is unnecessary here.
	pkgs, err := parser.ParseDir(fset, ".", filterTestOnly, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse test files: %v", err)
	}

	forbiddenPrefixes := []string{
		"github.com/awf-project/cli/internal/infrastructure",
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, prefix := range forbiddenPrefixes {
					if path == prefix || strings.HasPrefix(path, prefix+"/") {
						t.Errorf("test file %s imports infrastructure package %q; domain tests must not depend on infrastructure", filepath.Base(name), path)
					}
				}
			}
		}
	}
}
