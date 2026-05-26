package builtins_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/infrastructure/tools/builtins"
)

func TestRead_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{"path": path})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, "text", result.Content[0].Type)
	assert.Equal(t, "hello world", result.Content[0].Text)
}

func TestRead_MissingFile_IsError(t *testing.T) {
	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{"path": "/nonexistent/no/such/file.txt"})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.NotEmpty(t, result.Content[0].Text)
}

func TestRead_Offset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	content := "line1\nline2\nline3\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{
		"path":   path,
		"offset": 1,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.False(t, strings.Contains(result.Content[0].Text, "line1"))
	assert.True(t, strings.Contains(result.Content[0].Text, "line2"))
}

func TestRead_Limit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	content := "line1\nline2\nline3\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{
		"path":  path,
		"limit": 1,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.True(t, strings.Contains(result.Content[0].Text, "line1"))
	assert.False(t, strings.Contains(result.Content[0].Text, "line2"))
}

func TestRead_MissingFile_ErrorInContent(t *testing.T) {
	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{
		"path": "/nonexistent/path/to/file.txt",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Len(t, result.Content, 1)
	assert.Equal(t, "text", result.Content[0].Type)
	assert.NotEmpty(t, result.Content[0].Text, "error message should be in Content")
}

// TestRead_RootDir_BlocksTraversal verifies that with WithRootDir set, a Read on
// a path outside rootDir returns IsError instead of silently exposing the file.
// This is the regression guard for the F099 path-traversal review finding.
func TestRead_RootDir_BlocksTraversal(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("PRIVATE"), 0o600))

	p := builtins.NewProvider(builtins.WithRootDir(root))
	result, err := p.CallTool(context.Background(), "Read", map[string]any{"path": outside})

	require.NoError(t, err, "CallTool returns IsError, not a Go error, for traversal attempts")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "Read outside rootDir must be flagged IsError")
	assert.NotContains(t, result.Content[0].Text, "PRIVATE",
		"the file's contents must never leak in the error message")
}

// TestRead_RootDir_AllowsPathWithinRoot verifies the happy path under WithRootDir.
func TestRead_RootDir_AllowsPathWithinRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ok.txt")
	require.NoError(t, os.WriteFile(path, []byte("inside"), 0o644))

	p := builtins.NewProvider(builtins.WithRootDir(root))
	result, err := p.CallTool(context.Background(), "Read", map[string]any{"path": path})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Equal(t, "inside", result.Content[0].Text)
}

// TestRead_SizeCap_TruncatesOversizedFile verifies that Read enforces MaxReadBytes
// to prevent OOM via /dev/zero or large files (F099 review finding).
func TestRead_SizeCap_TruncatesOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	// Write MaxReadBytes + 1 KiB to force truncation.
	oversize := make([]byte, builtins.MaxReadBytes+1024)
	for i := range oversize {
		oversize[i] = 'a'
	}
	require.NoError(t, os.WriteFile(path, oversize, 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{"path": path})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "truncated",
		"truncation notice must surface to the agent so it can stop reading")
	// The bulk of the content should still be present — caller decides what to do
	// with the truncation flag.
	assert.GreaterOrEqual(t, len(result.Content[0].Text), builtins.MaxReadBytes)
}

func TestRead_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "combined.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	p := builtins.NewProvider()
	result, err := p.CallTool(context.Background(), "Read", map[string]any{
		"path":   path,
		"offset": 1,
		"limit":  2,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	assert.False(t, strings.Contains(result.Content[0].Text, "line1"))
	assert.True(t, strings.Contains(result.Content[0].Text, "line2"))
	assert.True(t, strings.Contains(result.Content[0].Text, "line3"))
	assert.False(t, strings.Contains(result.Content[0].Text, "line4"))
}
