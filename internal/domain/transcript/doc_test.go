package transcript_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTranscriptDocFile(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("doc.go")
	require.NoError(t, err, "failed to read doc.go")
	return string(content)
}

func TestDocDrift_CommentLineCount(t *testing.T) {
	doc := loadTranscriptDocFile(t)
	lines := strings.Split(doc, "\n")

	commentLines := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			commentLines++
		}
	}

	assert.GreaterOrEqual(t, commentLines, 100,
		"doc.go should have at least 100 non-blank comment lines (got %d)", commentLines)
}

func TestDocDrift_RequiredSections(t *testing.T) {
	doc := loadTranscriptDocFile(t)

	sections := []string{
		"# Purpose",
		"# Public Surface",
		"# Threat Model",
		"# Error Taxonomy",
		"# Dependency Contract",
	}

	for _, section := range sections {
		assert.Contains(t, doc, section, "should have %s section", section)
	}
}

func TestDocDrift_ExportedSymbols(t *testing.T) {
	doc := loadTranscriptDocFile(t)
	declared := transcriptExportedDecls(t)

	for name := range declared {
		assert.Contains(t, doc, name,
			"doc.go should document exported symbol %q", name)
	}
}

func transcriptExportedDecls(t *testing.T) map[string]struct{} {
	t.Helper()
	fset := token.NewFileSet()
	//nolint:staticcheck // SA1019: ParseDir suffices for a declaration-name scan; build-tag precision is unnecessary here.
	pkgs, err := parser.ParseDir(fset, ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	require.NoError(t, err, "failed to parse package directory")

	declared := make(map[string]struct{})
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				transcriptCollectExportedDecl(decl, declared)
			}
		}
	}
	return declared
}

func transcriptCollectExportedDecl(decl ast.Decl, out map[string]struct{}) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil && d.Name.IsExported() {
			out[d.Name.Name] = struct{}{}
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			transcriptCollectExportedSpec(spec, out)
		}
	}
}

func transcriptCollectExportedSpec(spec ast.Spec, out map[string]struct{}) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		if s.Name.IsExported() {
			out[s.Name.Name] = struct{}{}
		}
	case *ast.ValueSpec:
		for _, n := range s.Names {
			if n.IsExported() {
				out[n.Name] = struct{}{}
			}
		}
	}
}
