//go:build integration

// Feature: C045
//
// Integration tests validating package-level documentation compliance.
// Tests verify that doc.go files exist, follow Go conventions, and properly
// document core hexagonal architecture packages (workflow, ports, application).

package integration_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPackageDocumentation_WorkflowPackage validates workflow package documentation
func TestPackageDocumentation_WorkflowPackage(t *testing.T) {
	repoRoot := getRepoRoot(t)
	docPath := filepath.Join(repoRoot, "internal/domain/workflow/doc.go")

	// Verify file exists
	_, err := os.Stat(docPath)
	require.NoError(t, err, "workflow doc.go should exist")

	// Parse file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, docPath, nil, parser.ParseComments)
	require.NoError(t, err, "should parse workflow doc.go")

	// Verify package name
	assert.Equal(t, "workflow", f.Name.Name, "package name should be workflow")

	// Verify package comment exists
	require.NotNil(t, f.Doc, "package comment should exist")
	require.Greater(t, len(f.Doc.List), 0, "package comment should have at least one line")

	// Verify package comment starts with "Package workflow"
	firstLine := f.Doc.List[0].Text
	assert.True(t,
		strings.HasPrefix(firstLine, "// Package workflow"),
		"package comment should start with '// Package workflow', got: %s", firstLine)

	// Verify no build constraints
	for _, comment := range f.Comments {
		for _, line := range comment.List {
			assert.False(t,
				strings.HasPrefix(line.Text, "//go:build") || strings.HasPrefix(line.Text, "// +build"),
				"doc.go should not have build constraints: %s", line.Text)
		}
	}

	// Verify no executable code (only package declaration)
	assert.Empty(t, f.Decls, "doc.go should contain no declarations (functions, types, etc.)")
}

// TestPackageDocumentation_PortsPackage validates ports package documentation
func TestPackageDocumentation_PortsPackage(t *testing.T) {
	repoRoot := getRepoRoot(t)
	docPath := filepath.Join(repoRoot, "internal/domain/ports/doc.go")

	// Verify file exists
	_, err := os.Stat(docPath)
	require.NoError(t, err, "ports doc.go should exist")

	// Parse file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, docPath, nil, parser.ParseComments)
	require.NoError(t, err, "should parse ports doc.go")

	// Verify package name
	assert.Equal(t, "ports", f.Name.Name, "package name should be ports")

	// Verify package comment exists
	require.NotNil(t, f.Doc, "package comment should exist")
	require.Greater(t, len(f.Doc.List), 0, "package comment should have at least one line")

	// Verify package comment starts with "Package ports"
	firstLine := f.Doc.List[0].Text
	assert.True(t,
		strings.HasPrefix(firstLine, "// Package ports"),
		"package comment should start with '// Package ports', got: %s", firstLine)

	// Verify no build constraints
	for _, comment := range f.Comments {
		for _, line := range comment.List {
			assert.False(t,
				strings.HasPrefix(line.Text, "//go:build") || strings.HasPrefix(line.Text, "// +build"),
				"doc.go should not have build constraints: %s", line.Text)
		}
	}

	// Verify no executable code
	assert.Empty(t, f.Decls, "doc.go should contain no declarations")
}

// TestPackageDocumentation_ApplicationPackage validates application package documentation
func TestPackageDocumentation_ApplicationPackage(t *testing.T) {
	repoRoot := getRepoRoot(t)
	docPath := filepath.Join(repoRoot, "internal/application/doc.go")

	// Verify file exists
	_, err := os.Stat(docPath)
	require.NoError(t, err, "application doc.go should exist")

	// Parse file
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, docPath, nil, parser.ParseComments)
	require.NoError(t, err, "should parse application doc.go")

	// Verify package name
	assert.Equal(t, "application", f.Name.Name, "package name should be application")

	// Verify package comment exists
	require.NotNil(t, f.Doc, "package comment should exist")
	require.Greater(t, len(f.Doc.List), 0, "package comment should have at least one line")

	// Verify package comment starts with "Package application"
	firstLine := f.Doc.List[0].Text
	assert.True(t,
		strings.HasPrefix(firstLine, "// Package application"),
		"package comment should start with '// Package application', got: %s", firstLine)

	// Verify no build constraints
	for _, comment := range f.Comments {
		for _, line := range comment.List {
			assert.False(t,
				strings.HasPrefix(line.Text, "//go:build") || strings.HasPrefix(line.Text, "// +build"),
				"doc.go should not have build constraints: %s", line.Text)
		}
	}

	// Verify no executable code
	assert.Empty(t, f.Decls, "doc.go should contain no declarations")
}

// TestPackageDocumentation_AllPackages uses table-driven tests for all packages
func TestPackageDocumentation_AllPackages(t *testing.T) {
	repoRoot := getRepoRoot(t)

	tests := []struct {
		name             string
		docPath          string
		expectedPackage  string
		expectedPrefix   string
		minCommentLines  int
		requiresSections bool
	}{
		{
			name:             "workflow package",
			docPath:          "internal/domain/workflow/doc.go",
			expectedPackage:  "workflow",
			expectedPrefix:   "// Package workflow",
			minCommentLines:  5,
			requiresSections: true,
		},
		{
			name:             "ports package",
			docPath:          "internal/domain/ports/doc.go",
			expectedPackage:  "ports",
			expectedPrefix:   "// Package ports",
			minCommentLines:  3,
			requiresSections: true,
		},
		{
			name:             "application package",
			docPath:          "internal/application/doc.go",
			expectedPackage:  "application",
			expectedPrefix:   "// Package application",
			minCommentLines:  3,
			requiresSections: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(repoRoot, tt.docPath)

			// File existence
			info, err := os.Stat(fullPath)
			require.NoError(t, err, "doc.go should exist at %s", tt.docPath)
			assert.False(t, info.IsDir(), "doc.go should be a file, not directory")
			assert.Greater(t, info.Size(), int64(0), "doc.go should not be empty")

			// Parse file
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
			require.NoError(t, err, "should parse doc.go")

			// Package name
			assert.Equal(t, tt.expectedPackage, f.Name.Name)

			// Package comment
			require.NotNil(t, f.Doc, "package comment should exist")
			require.GreaterOrEqual(t, len(f.Doc.List), tt.minCommentLines,
				"package comment should have at least %d lines", tt.minCommentLines)

			// Comment starts correctly
			firstLine := f.Doc.List[0].Text
			assert.True(t, strings.HasPrefix(firstLine, tt.expectedPrefix),
				"first line should start with '%s', got: %s", tt.expectedPrefix, firstLine)

			// No build constraints
			for _, comment := range f.Comments {
				for _, line := range comment.List {
					assert.False(t,
						strings.Contains(line.Text, "//go:build") || strings.Contains(line.Text, "// +build"),
						"should not contain build constraints")
				}
			}

			// No declarations
			assert.Empty(t, f.Decls, "doc.go should only contain package comment")

			// Section headers (if required)
			if tt.requiresSections {
				docText := getFullDocText(f.Doc)
				assert.Contains(t, docText, "# ", "should contain section headers using '# '")
			}
		})
	}
}

// TestPackageDocumentation_NoMultiplePackageComments validates one package comment rule
func TestPackageDocumentation_NoMultiplePackageComments(t *testing.T) {
	repoRoot := getRepoRoot(t)

	packages := []struct {
		name string
		dir  string
	}{
		{"workflow", "internal/domain/workflow"},
		{"ports", "internal/domain/ports"},
		{"application", "internal/application"},
	}

	for _, pkg := range packages {
		t.Run(pkg.name, func(t *testing.T) {
			pkgDir := filepath.Join(repoRoot, pkg.dir)
			files, err := os.ReadDir(pkgDir)
			require.NoError(t, err, "should read package directory")

			packageCommentCount := 0
			filesWithPackageComment := []string{}

			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
					continue
				}

				// Skip test files
				if strings.HasSuffix(file.Name(), "_test.go") {
					continue
				}

				filePath := filepath.Join(pkgDir, file.Name())
				hasPackageComment, err := fileHasPackageComment(filePath)
				require.NoError(t, err, "should check file %s", file.Name())

				if hasPackageComment {
					packageCommentCount++
					filesWithPackageComment = append(filesWithPackageComment, file.Name())
				}
			}

			// Only doc.go should have package comment
			assert.Equal(t, 1, packageCommentCount,
				"package %s should have exactly one file with package comment (doc.go), found %d in files: %v",
				pkg.name, packageCommentCount, filesWithPackageComment)

			// That file should be doc.go
			if packageCommentCount == 1 {
				assert.Equal(t, "doc.go", filesWithPackageComment[0],
					"only doc.go should have package comment, found in: %s", filesWithPackageComment[0])
			}
		})
	}
}

// TestPackageDocumentation_MalformedFile validates parser error handling
func TestPackageDocumentation_MalformedFile(t *testing.T) {
	// Create temporary malformed file
	tmpDir := t.TempDir()
	malformedPath := filepath.Join(tmpDir, "malformed.go")

	err := os.WriteFile(malformedPath, []byte("package test\n\nfunc ( invalid syntax"), 0o644)
	require.NoError(t, err)

	// Should fail to parse
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, malformedPath, nil, parser.ParseComments)
	assert.Error(t, err, "should fail to parse malformed Go file")
}

// TestPackageDocumentation_EmptyFile validates empty file handling
func TestPackageDocumentation_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "empty.go")

	err := os.WriteFile(emptyPath, []byte(""), 0o644)
	require.NoError(t, err)

	// Should fail to parse empty file
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, emptyPath, nil, parser.ParseComments)
	assert.Error(t, err, "should fail to parse empty file")
}

// TestPackageDocumentation_NoPackageComment validates missing package comment
func TestPackageDocumentation_NoPackageComment(t *testing.T) {
	tmpDir := t.TempDir()
	noCommentPath := filepath.Join(tmpDir, "nocomment.go")

	// File with package but no comment
	err := os.WriteFile(noCommentPath, []byte("package test\n"), 0o644)
	require.NoError(t, err)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, noCommentPath, nil, parser.ParseComments)
	require.NoError(t, err, "should parse valid Go file")

	// Package comment should be nil
	assert.Nil(t, f.Doc, "file without package comment should have nil Doc field")
}

// TestPackageDocumentation_ContentQuality validates documentation completeness
func TestPackageDocumentation_ContentQuality(t *testing.T) {
	repoRoot := getRepoRoot(t)

	tests := []struct {
		name             string
		docPath          string
		requiredKeywords []string
		description      string
	}{
		{
			name:    "workflow documentation completeness",
			docPath: "internal/domain/workflow/doc.go",
			requiredKeywords: []string{
				"Workflow", "Step", "ExecutionContext",
				"state machine", "validation",
			},
			description: "should document core workflow entities and state machine",
		},
		{
			name:    "ports documentation completeness",
			docPath: "internal/domain/ports/doc.go",
			requiredKeywords: []string{
				"Repository", "StateStore", "Executor",
				"port", "interface",
			},
			description: "should document port interfaces and hexagonal architecture",
		},
		{
			name:    "application documentation completeness",
			docPath: "internal/application/doc.go",
			requiredKeywords: []string{
				"WorkflowService", "ExecutionService",
				"orchestrat", "service",
			},
			description: "should document application services and orchestration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(repoRoot, tt.docPath)

			content, err := os.ReadFile(fullPath)
			require.NoError(t, err, "should read doc.go file")

			contentStr := strings.ToLower(string(content))

			for _, keyword := range tt.requiredKeywords {
				assert.Contains(t, contentStr, strings.ToLower(keyword),
					"%s: should mention '%s'", tt.description, keyword)
			}
		})
	}
}

// TestPackageDocumentation_FormattingConventions validates Go doc conventions
func TestPackageDocumentation_FormattingConventions(t *testing.T) {
	repoRoot := getRepoRoot(t)

	docPaths := []string{
		"internal/domain/workflow/doc.go",
		"internal/domain/ports/doc.go",
		"internal/application/doc.go",
	}

	for _, docPath := range docPaths {
		t.Run(filepath.Base(filepath.Dir(docPath)), func(t *testing.T) {
			fullPath := filepath.Join(repoRoot, docPath)

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, fullPath, nil, parser.ParseComments)
			require.NoError(t, err)

			require.NotNil(t, f.Doc, "should have package comment")

			// First line should be single-line description
			firstLine := f.Doc.List[0].Text
			assert.True(t,
				strings.HasPrefix(firstLine, "//"),
				"comment should use // style, not /* */")

			// Should not have blank comment lines at start
			assert.NotEqual(t, "//", strings.TrimSpace(firstLine),
				"first comment line should not be blank")

			// Full doc text should be substantial
			docText := getFullDocText(f.Doc)
			assert.Greater(t, len(docText), 100,
				"package documentation should be substantial (>100 chars)")
		})
	}
}

// fileHasPackageComment checks if a Go file has a package-level comment
func fileHasPackageComment(filePath string) (bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return false, err
	}

	return f.Doc != nil && len(f.Doc.List) > 0, nil
}

// getFullDocText concatenates all comment lines into full documentation text
func getFullDocText(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}

	var lines []string
	for _, comment := range doc.List {
		// Remove leading // or /* */
		text := strings.TrimPrefix(comment.Text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		lines = append(lines, text)
	}

	return strings.Join(lines, "\n")
}
