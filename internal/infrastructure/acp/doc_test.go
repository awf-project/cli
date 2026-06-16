package acp_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadDocFile(t *testing.T) string {
	t.Helper()
	docPath := filepath.Join(".", "doc.go")
	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "failed to read doc.go")
	return string(content)
}

func TestDocGo_PackageDeclaration(t *testing.T) {
	doc := loadDocFile(t)
	assert.Contains(t, doc, "// Package acp", "should have package documentation comment")
	assert.Contains(t, doc, "package acp", "should have package acp declaration")
}

func TestDocGo_HasAllSevenSections(t *testing.T) {
	doc := loadDocFile(t)

	sections := []string{
		"# Purpose",
		"# Public Surface",
		"# Internal Layout",
		"# Threat Model",
		"# Error Taxonomy",
		"# Dependency Contract",
		"# SDK Substitution",
	}

	for _, section := range sections {
		assert.Contains(t, doc, section, "should have %s section", section)
	}
}

func TestDocGo_PublicSurfaceExports(t *testing.T) {
	doc := loadDocFile(t)

	exports := []string{
		"Agent",
		"Emitter",
		"Renderer",
		"PermissionClient",
		"NewAgent",
		"NewEmitter",
		"NewRenderer",
		"NewPermissionClient",
		"toACPError",
	}

	for _, export := range exports {
		assert.Contains(t, doc, export, "should mention exported %s", export)
	}
}

// TestDocGo_DocumentedExportsExist guards against documentation drift: every exported
// type or constructor named in the "# Public Surface" section must actually exist in
// the compiled package. This catches stale doc entries (e.g. a type described in doc.go
// but never implemented or since removed) that a plain string-contains check would miss.
func TestDocGo_DocumentedExportsExist(t *testing.T) {
	// Exported symbols the Public Surface promises callers can rely on.
	documentedExports := []string{
		"Agent",
		"Emitter",
		"Renderer",
		"PermissionClient",
		"NewAgent",
		"NewEmitter",
		"NewRenderer",
		"NewPermissionClient",
	}

	declared := exportedDecls(t)
	for _, name := range documentedExports {
		assert.Contains(t, declared, name,
			"doc.go documents exported %q in Public Surface, but no such symbol is declared in the package", name)
	}
}

// exportedDecls parses every non-test Go file in the package and returns the set of
// exported top-level identifiers (types, funcs, consts, vars).
func exportedDecls(t *testing.T) map[string]struct{} {
	t.Helper()
	fset := token.NewFileSet()
	//nolint:staticcheck // SA1019: ParseDir suffices for a declaration-name scan; build-tag precision is unnecessary here
	pkgs, err := parser.ParseDir(fset, ".", func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	require.NoError(t, err, "failed to parse package directory")

	declared := make(map[string]struct{})
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				collectExportedDecl(decl, declared)
			}
		}
	}
	return declared
}

// collectExportedDecl records the exported top-level name(s) introduced by decl.
func collectExportedDecl(decl ast.Decl, out map[string]struct{}) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		// Methods (Recv != nil) are not part of the package's top-level exported surface.
		if d.Recv == nil && d.Name.IsExported() {
			out[d.Name.Name] = struct{}{}
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			collectExportedSpec(spec, out)
		}
	}
}

// collectExportedSpec records exported type/const/var names declared by spec.
func collectExportedSpec(spec ast.Spec, out map[string]struct{}) {
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

func TestDocGo_InternalLayoutFiles(t *testing.T) {
	doc := loadDocFile(t)

	files := []string{
		"agent.go",
		"errors.go",
		"emitter.go",
		"event_projector.go",
		"renderer.go",
		"fanout_publisher.go",
		"permission.go",
		"architecture_test.go",
	}

	for _, file := range files {
		assert.Contains(t, doc, file, "should list %s in Internal Layout", file)
	}
}

func TestDocGo_ThreatModelElements(t *testing.T) {
	doc := loadDocFile(t)

	threats := []string{
		"Stdout serialization invariant",
		"panic-recover",
		"Secret masking",
		"10 MiB",
	}

	for _, threat := range threats {
		assert.Contains(t, doc, threat, "Threat Model should mention %s", threat)
	}
}

func TestDocGo_ErrorTaxonomyMapping(t *testing.T) {
	doc := loadDocFile(t)

	// Should have error kind mappings
	assert.Contains(t, doc, "ACPHandlerError", "should document ACPHandlerError mapping")
	assert.Contains(t, doc, "ACPErrInvalidParams", "should map ACPErrInvalidParams")
	assert.Contains(t, doc, "ACPErrMethodNotFound", "should map ACPErrMethodNotFound")
	assert.Contains(t, doc, "ACPErrInternal", "should map ACPErrInternal")
}

func TestDocGo_DependencyContract(t *testing.T) {
	doc := loadDocFile(t)

	// Should mention allowed dependencies
	assert.Contains(t, doc, "github.com/coder/acp-go-sdk", "should document SDK import")
	assert.Contains(t, doc, "v0.13", "should reference SDK version constraint")
	assert.Contains(t, doc, "internal/application", "should allow application imports")
	assert.Contains(t, doc, "internal/domain/ports", "should allow domain/ports imports")

	// Should forbid certain imports
	assert.Contains(t, doc, "MUST NOT import", "should document forbidden imports")
	assert.Contains(t, doc, "internal/interfaces", "should forbid interface layer imports")
}

func TestDocGo_SDKSubstitution(t *testing.T) {
	doc := loadDocFile(t)

	assert.Contains(t, doc, "SDK Substitution", "should have SDK Substitution section")
	assert.Contains(t, doc, "github.com/coder/acp-go-sdk", "should mention SDK in substitution section")
	assert.Contains(t, doc, "fully confined", "should document SDK confinement strategy")
}

func TestDocGo_ReferenceF104(t *testing.T) {
	doc := loadDocFile(t)

	assert.Contains(t, doc, "F104", "should reference F104 MCP as blueprint")
	assert.Contains(t, doc, "9740292", "should reference commit 9740292")
}

func TestDocGo_CommentLineCount(t *testing.T) {
	doc := loadDocFile(t)

	lines := strings.Split(doc, "\n")
	commentLines := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") && i > 0 {
			if !strings.Contains(trimmed, "package acp") {
				commentLines++
			}
		}
	}

	assert.GreaterOrEqual(t, commentLines, 145,
		"should have at least 145 non-blank comment lines (got %d)",
		commentLines)
}

func TestDocGo_OnlyPackageDeclaration(t *testing.T) {
	doc := loadDocFile(t)

	// Split into comment block and code
	parts := strings.SplitN(doc, "package acp", 2)
	require.Len(t, parts, 2, "should have package acp declaration")

	codeSection := strings.TrimSpace(parts[1])

	// After package acp, there should be only a newline (no other code)
	assert.Equal(t, "", codeSection,
		"file should contain only package comment and 'package acp' declaration, no other code")
}

func TestDocGo_BuildSucceeds(t *testing.T) {
	cmd := exec.Command("go", "build", "./") //nolint:noctx // test-controlled subprocess; no cancellation needed
	cmd.Dir = "."

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("go build failed: %v\nstderr: %s", err, stderr.String())
	}
}

func TestDocGo_VetClean(t *testing.T) {
	cmd := exec.Command("go", "vet", "./...") //nolint:noctx // test-controlled subprocess; no cancellation needed
	cmd.Dir = "."

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("go vet failed: %v\nstderr: %s", err, stderr.String())
	}
}

func TestDocGo_SectionOrdering(t *testing.T) {
	doc := loadDocFile(t)

	sections := []string{
		"# Purpose",
		"# Public Surface",
		"# Internal Layout",
		"# Threat Model",
		"# Error Taxonomy",
		"# Dependency Contract",
		"# SDK Substitution",
	}

	positions := make([]int, 0, len(sections))
	for _, section := range sections {
		pos := strings.Index(doc, section)
		require.NotEqual(t, -1, pos, "section %s not found", section)
		positions = append(positions, pos)
	}

	// Verify ordering: each position should be greater than the previous
	for i := 1; i < len(positions); i++ {
		assert.Greater(t, positions[i], positions[i-1],
			"section %s should come before %s",
			sections[i-1], sections[i])
	}
}

func TestDocGo_AgentMethodStubCount(t *testing.T) {
	doc := loadDocFile(t)

	// Public Surface mentions 4 real + 7 stub methods
	assert.Contains(t, doc, "Four methods", "should mention 4 real methods")
	assert.Contains(t, doc, "seven", "should mention 7 stub methods")
	assert.Regexp(t, regexp.MustCompile("(?i)(Authenticate|CloseSession|ListSessions|ResumeSession|SetSessionConfigOption|SetSessionMode)"),
		doc, "should list optional stub methods")
}

func TestDocGo_T037AndT038References(t *testing.T) {
	doc := loadDocFile(t)

	assert.Contains(t, doc, "T037", "should reference T037 architecture test")
	assert.Contains(t, doc, "T038", "should reference T038 go-arch-lint")
	assert.Contains(t, doc, ".go-arch-lint.yml", "should mention go-arch-lint config")
}

func TestDocGo_MaxPromptBytesDocumented(t *testing.T) {
	doc := loadDocFile(t)

	assert.Contains(t, doc, "MaxPromptBytes", "should document the MaxPromptBytes constant")
	assert.Contains(t, doc, "1 MiB", "should specify MaxPromptBytes as 1 MiB")
}
