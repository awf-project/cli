package builtins_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
)

// TestGrep_HappyPath_ContentMode verifies content mode output.
// Acceptance: Grep walks the path, returns matching lines in output_mode "content"
// (newline-joined), with IsError: false.
func TestGrep_HappyPath_ContentMode(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")
	file2 := filepath.Join(dir, "test2.txt")

	require.NoError(t, os.WriteFile(file1, []byte("hello world\nfoo bar\nhello again\n"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("hello universe\nnothing here\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "content",
	})

	require.NoError(t, err, "CallTool should return nil error for valid pattern and path")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false for successful grep")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "hello world", "should contain matching lines from file1")
	assert.Contains(t, text, "hello again", "should contain all matching lines from file1")
	assert.Contains(t, text, "hello universe", "should contain matching lines from file2")
	assert.NotContains(t, text, "foo bar", "should not contain non-matching lines")
}

// TestGrep_FilesWithMatches_Mode verifies files_with_matches output mode.
// Acceptance: output_mode "files_with_matches" returns newline-joined file paths.
func TestGrep_FilesWithMatches_Mode(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")
	file2 := filepath.Join(dir, "test2.txt")
	file3 := filepath.Join(dir, "test3.txt")

	require.NoError(t, os.WriteFile(file1, []byte("hello world\n"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("nothing here\n"), 0o644))
	require.NoError(t, os.WriteFile(file3, []byte("hello again\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "files_with_matches",
	})

	require.NoError(t, err, "CallTool should return nil error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false for successful grep")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "test1.txt", "should include file with matches")
	assert.Contains(t, text, "test3.txt", "should include file with matches")
	assert.NotContains(t, text, "test2.txt", "should not include file without matches")
}

// TestGrep_Count_Mode verifies count output mode.
// Acceptance: output_mode "count" returns the number of matching lines.
func TestGrep_Count_Mode(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")

	require.NoError(t, os.WriteFile(file1, []byte("hello world\nhello again\nfoo bar\nhello once more\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "count",
	})

	require.NoError(t, err, "CallTool should return nil error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "3", "should report count of 3 matching lines")
}

// TestGrep_CaseInsensitive_Match verifies case_insensitive option.
// Acceptance: optional case_insensitive bool enables case-insensitive matching.
func TestGrep_CaseInsensitive_Match(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")

	require.NoError(t, os.WriteFile(file1, []byte("Hello World\nhello world\nHELLO WORLD\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern":          "hello",
		"path":             dir,
		"output_mode":      "content",
		"case_insensitive": true,
	})

	require.NoError(t, err, "CallTool should return nil error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "Hello World", "should match case variations")
	assert.Contains(t, text, "hello world", "should match case variations")
	assert.Contains(t, text, "HELLO WORLD", "should match case variations")
}

// TestGrep_InvalidRegex_ReturnsGoError verifies error on malformed pattern.
// Acceptance: Grep returns Go error on invalid regex.
func TestGrep_InvalidRegex_ReturnsGoError(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")
	require.NoError(t, os.WriteFile(file1, []byte("hello world\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern": "[invalid(regex",
		"path":    dir,
	})

	assert.Error(t, err, "CallTool should return error for invalid regex")
	assert.Nil(t, result, "result should be nil when error occurs")
}

// TestGrep_NoMatches_ReturnsEmptyText verifies empty match behavior.
// Acceptance: empty matches → Content[0].Text = "" and IsError: false.
func TestGrep_NoMatches_ReturnsEmptyText(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.txt")

	require.NoError(t, os.WriteFile(file1, []byte("hello world\nfoo bar\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern": "xyz",
		"path":    dir,
	})

	require.NoError(t, err, "CallTool should return nil error for non-matching pattern")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false when no matches found")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	assert.Equal(t, "", result.Content[0].Text, "text should be empty when no matches")
}

// TestGrep_SingleFile_ContentMode verifies grep on single file.
// Acceptance: Grep handles both file and directory for path parameter.
func TestGrep_SingleFile_ContentMode(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test.txt")

	require.NoError(t, os.WriteFile(file1, []byte("hello world\nfoo bar\nhello again\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern": "hello",
		"path":    file1,
	})

	require.NoError(t, err, "CallTool should return nil error when path is a file")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "hello world", "should match patterns in single file")
	assert.Contains(t, text, "hello again", "should match patterns in single file")
}

// TestGrep_WithGlobFilter verifies glob filtering.
// Acceptance: Grep filters by glob when set; walks matching files only.
func TestGrep_WithGlobFilter(t *testing.T) {
	dir := t.TempDir()
	file1 := filepath.Join(dir, "test1.go")
	file2 := filepath.Join(dir, "test2.txt")
	file3 := filepath.Join(dir, "test3.go")

	require.NoError(t, os.WriteFile(file1, []byte("func main() {\n"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("hello\n"), 0o644))
	require.NoError(t, os.WriteFile(file3, []byte("package main\n"), 0o644))

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"pattern":     "main",
		"path":        dir,
		"glob":        "*.go",
		"output_mode": "files_with_matches",
	})

	require.NoError(t, err, "CallTool should return nil error")
	require.NotNil(t, result, "result should not be nil")
	assert.False(t, result.IsError, "IsError should be false")
	assert.Len(t, result.Content, 1, "result should contain exactly one content block")
	text := result.Content[0].Text
	assert.Contains(t, text, "test1.go", "should include matching .go files")
	assert.Contains(t, text, "test3.go", "should include matching .go files")
	assert.NotContains(t, text, "test2.txt", "should exclude non-.go files per glob")
}

// TestGrep_MissingPattern_ReturnsError verifies schema validation.
// Acceptance: Grep schema requires "pattern" string.
func TestGrep_MissingPattern_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	provider := builtins.NewProvider()
	result, err := provider.CallTool(context.Background(), "Grep", map[string]any{
		"path": dir,
	})

	assert.Error(t, err, "CallTool should return error when required pattern is missing")
	assert.Nil(t, result, "result should be nil")
	assert.Contains(t, err.Error(), "missing required argument", "error should mention missing argument")
}
