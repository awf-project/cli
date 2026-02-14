//go:build integration

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T010
// Feature: C017

// TestDocGo_FileExists verifies that doc_internal_test.go file exists in the correct location
func TestDocGo_FileExists(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	info, err := os.Stat(docPath)

	require.NoError(t, err, "doc_internal_test.go should exist")
	assert.False(t, info.IsDir(), "doc_internal_test.go should be a file, not a directory")
}

// TestDocGo_HasBuildTag verifies that doc_internal_test.go contains the integration build tag
func TestDocGo_HasBuildTag(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "should read doc_internal_test.go")

	lines := strings.Split(string(content), "\n")

	require.Greater(t, len(lines), 0, "doc_internal_test.go should not be empty")
	assert.Equal(t, "//go:build integration", lines[0],
		"first line should be integration build tag")
}

// TestDocGo_HasPackageDeclaration verifies that doc_internal_test.go declares the correct package
func TestDocGo_HasPackageDeclaration(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "should read doc_internal_test.go")

	assert.Contains(t, string(content), "package cli_test",
		"doc_internal_test.go should declare package cli_test")
}

// TestDocGo_HasDocumentation verifies that doc_internal_test.go contains package documentation
func TestDocGo_HasDocumentation(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "should read doc_internal_test.go")

	assert.Contains(t, string(content), "// Package cli_test",
		"doc_internal_test.go should have package documentation comment")
	assert.Contains(t, string(content), "integration tests",
		"documentation should mention integration tests")
}

// TestDocGo_BuildTagFormat verifies the build tag format matches Go conventions
func TestDocGo_BuildTagFormat(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "should read doc_internal_test.go")

	lines := strings.Split(string(content), "\n")

	require.Greater(t, len(lines), 2, "doc_internal_test.go should have at least build tag, blank line, and package")
	assert.Equal(t, "//go:build integration", lines[0], "build tag should be first line")

	assert.Empty(t, lines[1], "second line should be blank after build tag")
}

// TestDocGo_PackageNameConsistency verifies package name matches directory convention
func TestDocGo_PackageNameConsistency(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "should read doc_internal_test.go")

	// Package should be cli_test (directory name + _test suffix for white-box testing)
	assert.Contains(t, string(content), "package cli_test",
		"package name should be cli_test for white-box integration testing")
}

// TestDocGo_NoExecutableCode verifies that doc_internal_test.go contains no executable code
func TestDocGo_NoExecutableCode(t *testing.T) {
	repoRoot, err := findRepoRoot()
	require.NoError(t, err, "should find repository root")

	docPath := filepath.Join(repoRoot, "tests", "integration", "cli", "doc_internal_test.go")

	content, err := os.ReadFile(docPath)
	require.NoError(t, err, "should read doc_internal_test.go")

	assert.NotContains(t, string(content), "func ", "doc_internal_test.go should not contain functions")
	assert.NotContains(t, string(content), "var ", "doc_internal_test.go should not contain variables")
	assert.NotContains(t, string(content), "const ", "doc_internal_test.go should not contain constants")
	assert.NotContains(t, string(content), "type ", "doc_internal_test.go should not contain type definitions")
	assert.NotContains(t, string(content), "import ", "doc_internal_test.go should not contain imports")
}

// TestDocGo_EmptyPackageCompiles verifies the package can be compiled
func TestDocGo_EmptyPackageCompiles(t *testing.T) {
	// This test passes if the file compiles, which is verified by the test suite running
	// The presence of this test file in the same package proves compilation works

	assert.True(t, true, "if this test runs, the package compiled successfully")
}
