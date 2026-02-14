//go:build integration

// Feature: C056
//
// Integration tests validating infrastructure and interface package documentation.
// Tests verify that doc.go files exist, follow Go conventions, and properly
// document infrastructure adapters and CLI interface packages.

package integration_test

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

// TestInfrastructureDocumentation_AllPackages uses table-driven tests for all infrastructure packages
func TestInfrastructureDocumentation_AllPackages(t *testing.T) {
	repoRoot := getRepoRoot(t)

	tests := []struct {
		name             string
		docPath          string
		expectedPackage  string
		expectedPrefix   string
		minCommentLines  int
		requiresSections bool
		minDocChars      int
	}{
		{
			name:             "executor package",
			docPath:          "internal/infrastructure/executor/doc.go",
			expectedPackage:  "executor",
			expectedPrefix:   "// Package executor",
			minCommentLines:  3,
			requiresSections: false, // concise style
			minDocChars:      100,
		},
		{
			name:             "expression package",
			docPath:          "internal/infrastructure/expression/doc.go",
			expectedPackage:  "expression",
			expectedPrefix:   "// Package expression",
			minCommentLines:  3,
			requiresSections: false, // concise style
			minDocChars:      100,
		},
		{
			name:             "store package",
			docPath:          "internal/infrastructure/store/doc.go",
			expectedPackage:  "store",
			expectedPrefix:   "// Package store",
			minCommentLines:  3,
			requiresSections: false, // concise style
			minDocChars:      100,
		},
		{
			name:             "logger package",
			docPath:          "internal/infrastructure/logger/doc.go",
			expectedPackage:  "logger",
			expectedPrefix:   "// Package logger",
			minCommentLines:  4,
			requiresSections: true, // medium style
			minDocChars:      150,
		},
		{
			name:             "cli/ui package",
			docPath:          "internal/interfaces/cli/ui/doc.go",
			expectedPackage:  "ui",
			expectedPrefix:   "// Package ui",
			minCommentLines:  4,
			requiresSections: true, // medium style
			minDocChars:      150,
		},
		{
			name:             "agents package",
			docPath:          "internal/infrastructure/agents/doc.go",
			expectedPackage:  "agents",
			expectedPrefix:   "// Package agents",
			minCommentLines:  5,
			requiresSections: true, // comprehensive style
			minDocChars:      200,
		},
		{
			name:             "repository package",
			docPath:          "internal/infrastructure/repository/doc.go",
			expectedPackage:  "repository",
			expectedPrefix:   "// Package repository",
			minCommentLines:  5,
			requiresSections: true, // comprehensive style
			minDocChars:      200,
		},
		{
			name:             "plugin package",
			docPath:          "internal/infrastructure/plugin/doc.go",
			expectedPackage:  "plugin",
			expectedPrefix:   "// Package plugin",
			minCommentLines:  5,
			requiresSections: true, // comprehensive style
			minDocChars:      200,
		},
		{
			name:             "cli package",
			docPath:          "internal/interfaces/cli/doc.go",
			expectedPackage:  "cli",
			expectedPrefix:   "// Package cli",
			minCommentLines:  5,
			requiresSections: true, // comprehensive style
			minDocChars:      200,
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

			// Documentation length
			docText := getFullDocText(f.Doc)
			assert.GreaterOrEqual(t, len(docText), tt.minDocChars,
				"documentation should be substantial (>%d chars)", tt.minDocChars)

			// Section headers (if required)
			if tt.requiresSections {
				assert.Contains(t, docText, "# ", "should contain section headers using '# '")
			}
		})
	}
}

// TestInfrastructureDocumentation_NoMultiplePackageComments validates one package comment rule
func TestInfrastructureDocumentation_NoMultiplePackageComments(t *testing.T) {
	repoRoot := getRepoRoot(t)

	packages := []struct {
		name string
		dir  string
	}{
		{"executor", "internal/infrastructure/executor"},
		{"expression", "internal/infrastructure/expression"},
		{"store", "internal/infrastructure/store"},
		{"logger", "internal/infrastructure/logger"},
		{"agents", "internal/infrastructure/agents"},
		{"repository", "internal/infrastructure/repository"},
		{"plugin", "internal/infrastructure/plugin"},
		{"cli", "internal/interfaces/cli"},
		{"ui", "internal/interfaces/cli/ui"},
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

// TestInfrastructureDocumentation_MalformedFile validates parser error handling
func TestInfrastructureDocumentation_MalformedFile(t *testing.T) {
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

// TestInfrastructureDocumentation_EmptyFile validates empty file handling
func TestInfrastructureDocumentation_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyPath := filepath.Join(tmpDir, "empty.go")

	err := os.WriteFile(emptyPath, []byte(""), 0o644)
	require.NoError(t, err)

	// Should fail to parse empty file
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, emptyPath, nil, parser.ParseComments)
	assert.Error(t, err, "should fail to parse empty file")
}

// TestInfrastructureDocumentation_NoPackageComment validates missing package comment
func TestInfrastructureDocumentation_NoPackageComment(t *testing.T) {
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

// TestInfrastructureDocumentation_ContentQuality validates documentation completeness
func TestInfrastructureDocumentation_ContentQuality(t *testing.T) {
	repoRoot := getRepoRoot(t)

	tests := []struct {
		name             string
		docPath          string
		requiredKeywords []string
		description      string
	}{
		{
			name:    "executor documentation completeness",
			docPath: "internal/infrastructure/executor/doc.go",
			requiredKeywords: []string{
				"executor", "shell", "command",
				"adapter", "port",
			},
			description: "should document ShellExecutor and CommandExecutor port",
		},
		{
			name:    "expression documentation completeness",
			docPath: "internal/infrastructure/expression/doc.go",
			requiredKeywords: []string{
				"expression", "evaluator", "validator",
				"adapter",
			},
			description: "should document expression evaluation adapters",
		},
		{
			name:    "store documentation completeness",
			docPath: "internal/infrastructure/store/doc.go",
			requiredKeywords: []string{
				"store", "state", "persistence",
				"json", "sqlite",
			},
			description: "should document state and history storage",
		},
		{
			name:    "logger documentation completeness",
			docPath: "internal/infrastructure/logger/doc.go",
			requiredKeywords: []string{
				"logger", "console", "json",
				"masking", "secret",
			},
			description: "should document logger implementations and secret masking",
		},
		{
			name:    "agents documentation completeness",
			docPath: "internal/infrastructure/agents/doc.go",
			requiredKeywords: []string{
				"agent", "provider", "claude",
				"registry", "executor",
			},
			description: "should document AI agent providers and registry",
		},
		{
			name:    "repository documentation completeness",
			docPath: "internal/infrastructure/repository/doc.go",
			requiredKeywords: []string{
				"repository", "yaml", "workflow",
				"template", "composite",
			},
			description: "should document workflow repository implementations",
		},
		{
			name:    "plugin documentation completeness",
			docPath: "internal/infrastructure/plugin/doc.go",
			requiredKeywords: []string{
				"plugin", "rpc", "manifest",
				"loader", "version",
			},
			description: "should document plugin system components",
		},
		{
			name:    "cli documentation completeness",
			docPath: "internal/interfaces/cli/doc.go",
			requiredKeywords: []string{
				"cli", "cobra", "command",
				"signal", "exit",
			},
			description: "should document CLI interface and command structure",
		},
		{
			name:    "ui documentation completeness",
			docPath: "internal/interfaces/cli/ui/doc.go",
			requiredKeywords: []string{
				"ui", "output", "color",
				"prompt", "format",
			},
			description: "should document UI components and formatting",
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

// TestInfrastructureDocumentation_FormattingConventions validates Go doc conventions
func TestInfrastructureDocumentation_FormattingConventions(t *testing.T) {
	repoRoot := getRepoRoot(t)

	docPaths := []string{
		"internal/infrastructure/executor/doc.go",
		"internal/infrastructure/expression/doc.go",
		"internal/infrastructure/store/doc.go",
		"internal/infrastructure/logger/doc.go",
		"internal/infrastructure/agents/doc.go",
		"internal/infrastructure/repository/doc.go",
		"internal/infrastructure/plugin/doc.go",
		"internal/interfaces/cli/doc.go",
		"internal/interfaces/cli/ui/doc.go",
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

// Helper functions shared with c045_package_documentation_test.go:
// - getRepoRoot(t *testing.T) string - defined in test_helpers_test.go
// - fileHasPackageComment(filePath string) (bool, error) - defined in c045_package_documentation_test.go
// - getFullDocText(doc *ast.CommentGroup) string - defined in c045_package_documentation_test.go
