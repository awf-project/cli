package builtins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteHandler_CreatesParentDirs verifies that the Write handler (and the
// atomicWrite function it delegates to) automatically creates all missing parent
// directories before writing the file. Without MkdirAll, writing to a nested path
// such as /tmp/a/b/c.txt fails if /tmp/a/b/ does not exist.
func TestWriteHandler_CreatesParentDirs(t *testing.T) {
	rootDir := t.TempDir()
	p := NewProvider(WithRootDir(rootDir))

	nestedPath := filepath.Join(rootDir, "a", "b", "c", "file.txt")
	const content = "hello nested world"

	result, err := p.writeHandler(context.Background(), map[string]any{
		"path":    nestedPath,
		"content": content,
	})

	require.NoError(t, err, "writeHandler must not return a Go error for nested paths")
	require.NotNil(t, result)
	assert.False(t, result.IsError, "IsError must be false when write succeeds")

	// Verify the file was created with the expected content.
	got, readErr := os.ReadFile(nestedPath)
	require.NoError(t, readErr, "file must exist after writeHandler succeeds")
	assert.Equal(t, content, string(got), "file content must match what was written")
}

// TestAtomicWrite_CreatesParentDirs directly exercises the atomicWrite helper to
// ensure that directory creation is not a side-effect of the handler layer.
func TestAtomicWrite_CreatesParentDirs(t *testing.T) {
	rootDir := t.TempDir()
	target := filepath.Join(rootDir, "x", "y", "z", "data.txt")

	err := atomicWrite(target, []byte("atomic content"))
	require.NoError(t, err, "atomicWrite must not fail when parent directories are absent")

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "atomic content", string(got))
}

// TestWriteHandler_MaxWriteBytes_IsError verifies that Write rejects content larger than MaxWriteBytes.
func TestWriteHandler_MaxWriteBytes_IsError(t *testing.T) {
	rootDir := t.TempDir()
	p := NewProvider(WithRootDir(rootDir))

	oversized := make([]byte, MaxWriteBytes+1)
	for i := range oversized {
		oversized[i] = 'a'
	}

	result, err := p.writeHandler(context.Background(), map[string]any{
		"path":    filepath.Join(rootDir, "big.txt"),
		"content": string(oversized),
	})

	require.NoError(t, err, "writeHandler must not return a Go error for oversized content")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "IsError must be true when content exceeds MaxWriteBytes")
	assert.Contains(t, result.Content[0].Text, "exceeds", "error message must mention the limit")
}

// TestAtomicWrite_ExistingDir verifies that atomicWrite behaves correctly when
// the parent directory already exists (the MkdirAll call must be idempotent).
func TestAtomicWrite_ExistingDir(t *testing.T) {
	rootDir := t.TempDir()
	target := filepath.Join(rootDir, "existing.txt")

	require.NoError(t, atomicWrite(target, []byte("first")))
	require.NoError(t, atomicWrite(target, []byte("second")))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "second", string(got), "second write must overwrite the first")
}
