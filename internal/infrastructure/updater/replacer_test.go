package updater_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/updater"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceBinary_HappyPath(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "awf")

	// Create existing binary
	require.NoError(t, os.WriteFile(execPath, []byte("old-binary"), 0o755))

	newBinary := []byte("new-binary-content")
	err := updater.ReplaceBinary(execPath, newBinary)

	require.NoError(t, err)

	// Verify content replaced
	data, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, newBinary, data)

	// Verify executable permission preserved
	info, err := os.Stat(execPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0o111 != 0, "binary should be executable")
}

func TestReplaceBinary_SymlinkResolution(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "awf-real")
	symlinkPath := filepath.Join(dir, "awf-link")

	require.NoError(t, os.WriteFile(realPath, []byte("old-binary"), 0o755))
	require.NoError(t, os.Symlink(realPath, symlinkPath))

	newBinary := []byte("new-binary-via-symlink")
	err := updater.ReplaceBinary(symlinkPath, newBinary)

	require.NoError(t, err)

	// Verify the real file was replaced, not the symlink
	data, err := os.ReadFile(realPath)
	require.NoError(t, err)
	assert.Equal(t, newBinary, data)

	// Verify symlink still points to real file
	target, err := os.Readlink(symlinkPath)
	require.NoError(t, err)
	assert.Equal(t, realPath, target)
}

func TestReplaceBinary_NonExistentPath(t *testing.T) {
	err := updater.ReplaceBinary("/nonexistent/path/awf", []byte("data"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve")
}

func TestReplaceBinary_EmptyData(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "awf")
	require.NoError(t, os.WriteFile(execPath, []byte("old"), 0o755))

	err := updater.ReplaceBinary(execPath, []byte{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestReplaceBinary_CrossFilesystemFallback(t *testing.T) {
	srcDir, err := os.MkdirTemp("/tmp", "awf-cross-fs-src-*")
	require.NoError(t, err)
	defer os.RemoveAll(srcDir)

	execPath := filepath.Join(srcDir, "awf")
	require.NoError(t, os.WriteFile(execPath, []byte("old-binary"), 0o755))

	newBinary := []byte("cross-filesystem-binary")
	err = updater.ReplaceBinary(execPath, newBinary)

	require.NoError(t, err)

	data, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, newBinary, data)
}

func TestIsPackageManagerPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"homebrew", "/opt/homebrew/bin/awf", true},
		{"linuxbrew", "/home/user/.linuxbrew/bin/awf", true},
		{"snap", "/snap/bin/awf", true},
		{"nix_store", "/nix/store/abc123/bin/awf", true},
		{"nix_profile", "/nix/profile/bin/awf", true},
		{"usr_bin", "/usr/bin/awf", true},
		{"usr_local_bin", "/usr/local/bin/awf", false},
		{"home_bin", "/home/user/bin/awf", false},
		{"tmp", "/tmp/awf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updater.IsPackageManagerPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
