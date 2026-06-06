package acp_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitecture_AllowedImportsOnly(t *testing.T) {
	fset := token.NewFileSet()

	filterNonTest := func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}

	//nolint:staticcheck // SA1019: ParseDir suffices for an import-only AST scan; build-tag precision is unnecessary here
	pkgs, err := parser.ParseDir(fset, ".", filterNonTest, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse package directory: %v", err)
	}

	if len(pkgs) == 0 {
		t.Fatal("no Go files found in package directory")
	}

	allowedPrefixes := []string{
		"github.com/coder/acp-go-sdk",
		"github.com/awf-project/cli/internal/application",
		"github.com/awf-project/cli/internal/domain/ports",
		"github.com/awf-project/cli/internal/domain/workflow",
		"github.com/awf-project/cli/internal/domain/pluginmodel",
		"github.com/awf-project/cli/internal/infrastructure/agents",
		"github.com/awf-project/cli/internal/infrastructure/logger",
		"github.com/awf-project/cli/pkg/display",
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)

				if isStdlib(path) {
					continue
				}

				allowed := false
				for _, prefix := range allowedPrefixes {
					if path == prefix || strings.HasPrefix(path, prefix+"/") {
						allowed = true
						break
					}
				}

				if !allowed {
					t.Errorf("disallowed import %q in %s", path, filepath.Base(name))
				}
			}
		}
	}
}

func isStdlib(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

// TestArchitecture_SDKConfinedToExpectedFiles enforces the SDK Substitution contract
// documented in doc.go: the github.com/coder/acp-go-sdk import must appear ONLY in the
// five files that own the transport seam (agent.go, emitter.go, permission.go,
// errors.go, server.go). Any other file importing the SDK directly widens the
// substitution surface and is rejected here so an SDK swap stays localized to those files.
func TestArchitecture_SDKConfinedToExpectedFiles(t *testing.T) {
	const sdkPath = "github.com/coder/acp-go-sdk"

	allowedSDKFiles := map[string]struct{}{
		"agent.go":      {},
		"emitter.go":    {},
		"permission.go": {},
		"errors.go":     {},
		"server.go":     {},
	}

	fset := token.NewFileSet()

	filterNonTest := func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}

	//nolint:staticcheck // SA1019: ParseDir suffices for an import-only AST scan; build-tag precision is unnecessary here
	pkgs, err := parser.ParseDir(fset, ".", filterNonTest, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("failed to parse package directory: %v", err)
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			base := filepath.Base(name)
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if path != sdkPath && !strings.HasPrefix(path, sdkPath+"/") {
					continue
				}
				if _, ok := allowedSDKFiles[base]; !ok {
					t.Errorf("SDK import %q found in %s; the SDK must be confined to agent.go, emitter.go, permission.go, errors.go, server.go", path, base)
				}
			}
		}
	}
}
