package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// labelConstants parses labels.go and returns every string constant it declares, so the
// language guard covers a label the moment it is added, with no list to keep in sync.
func labelConstants(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "labels.go", nil, 0)
	require.NoError(t, err)

	out := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				val, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				out[name.Name] = val
			}
		}
	}
	return out
}

// TestLabels_AreEnglish guards the "TUI stays in English" rule: no label may contain a
// Latin letter outside plain ASCII (accented letters, ligatures). Symbols such as arrows,
// bullets and dashes are allowed.
func TestLabels_AreEnglish(t *testing.T) {
	labels := labelConstants(t)
	require.Greater(t, len(labels), 50, "labels.go should declare the TUI labels")

	for name, label := range labels {
		for _, r := range label {
			if r > unicode.MaxASCII && unicode.Is(unicode.Latin, r) {
				t.Errorf("%s = %q contains non-English letter %q", name, label, r)
			}
		}
	}
}

func TestLabels_AreNotEmpty(t *testing.T) {
	for name, label := range labelConstants(t) {
		assert.NotEmpty(t, label, name)
	}
}
