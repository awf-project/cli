package application

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestACP_NoDirectRecorderSubscribe asserts that within the internal/application package
// only Adapter.newSession (in facade_adapter.go) calls Recorder.Subscribe(), enforcing
// the SC-004 constraint. Uses go/ast so comments and string literals are excluded from
// the scan.
//
// Scope is the application package only. The infra-owned transcript.MirrorToFile subscriber
// in internal/infrastructure/transcript is out of scope and remains the sanctioned infra
// subscriber. The existing repo-wide allowlist in facade_adapter_test.go is unchanged.
func TestACP_NoDirectRecorderSubscribe(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must succeed to locate the application package")

	pkgDir := filepath.Dir(currentFile)
	allowedFile := filepath.Join(pkgDir, "facade_adapter.go")

	entries, dirErr := os.ReadDir(pkgDir)
	require.NoError(t, dirErr, "os.ReadDir must succeed on the application package directory")

	fset := token.NewFileSet()
	var violations []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filename := filepath.Join(pkgDir, name)
		if filename == allowedFile {
			continue
		}
		f, parseErr := parser.ParseFile(fset, filename, nil, 0)
		if parseErr != nil {
			continue // skip unreadable files; not a violation
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "Subscribe" && len(call.Args) == 0 {
				pos := fset.Position(call.Pos())
				violations = append(violations, fmt.Sprintf("%s:%d", name, pos.Line))
			}
			return true
		})
	}

	assert.Empty(t, violations,
		"only Adapter.newSession in facade_adapter.go may call Recorder.Subscribe() "+
			"within the internal/application package (SC-004);\n"+
			"found violations: %v", violations)
}
