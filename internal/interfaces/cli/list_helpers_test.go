package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildPromptInfo_HappyPath verifies normal PromptInfo construction
func TestBuildPromptInfo_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(basePath, 0o755))

	// Create a test file
	testFile := filepath.Join(basePath, "system.md")
	content := []byte("You are an AI assistant")
	require.NoError(t, os.WriteFile(testFile, content, 0o644))

	// Get file info
	info, err := os.Stat(testFile)
	require.NoError(t, err)

	entry := &fakeDirEntry{
		name:  "system.md",
		isDir: false,
		info:  info,
	}

	seen := make(map[string]struct{})
	source := "local"

	result, err := buildPromptInfo(testFile, entry, basePath, source, seen)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "system.md", result.Name)
	assert.Equal(t, source, result.Source)
	assert.Equal(t, testFile, result.Path)
	assert.Equal(t, int64(len(content)), result.Size)
	assert.NotEmpty(t, result.ModTime)
}

// TestBuildPromptInfo_NestedPaths verifies relative path calculation
func TestBuildPromptInfo_NestedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "prompts")
	nestedDir := filepath.Join(basePath, "ai", "agents")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	testFile := filepath.Join(nestedDir, "claude.md")
	content := []byte("Claude system prompt")
	require.NoError(t, os.WriteFile(testFile, content, 0o644))

	info, err := os.Stat(testFile)
	require.NoError(t, err)

	entry := &fakeDirEntry{
		name:  "claude.md",
		isDir: false,
		info:  info,
	}

	seen := make(map[string]struct{})
	source := "global"

	result, err := buildPromptInfo(testFile, entry, basePath, source, seen)

	require.NoError(t, err)
	require.NotNil(t, result)
	// Should use relative path from basePath
	assert.Equal(t, filepath.Join("ai", "agents", "claude.md"), result.Name)
	assert.Equal(t, source, result.Source)
	assert.Equal(t, testFile, result.Path)
}

// TestBuildPromptInfo_Deduplication verifies seen map handling
func TestBuildPromptInfo_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(basePath, 0o755))

	testFile := filepath.Join(basePath, "duplicate.md")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o644))

	info, err := os.Stat(testFile)
	require.NoError(t, err)

	entry := &fakeDirEntry{
		name:  "duplicate.md",
		isDir: false,
		info:  info,
	}

	// Mark as already seen (earlier path wins)
	seen := map[string]struct{}{
		"duplicate.md": {},
	}
	source := "local"

	result, err := buildPromptInfo(testFile, entry, basePath, source, seen)

	// Should return nil for already-seen entries
	require.NoError(t, err)
	assert.Nil(t, result, "already-seen prompts should return nil")
}

// TestBuildPromptInfo_ErrorHandling verifies error scenarios
func TestBuildPromptInfo_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (path, basePath, source string, entry fs.DirEntry, seen map[string]struct{})
		wantErr   bool
		wantNil   bool
	}{
		{
			name: "missing file info returns error",
			setupFunc: func(t *testing.T) (string, string, string, fs.DirEntry, map[string]struct{}) {
				tmpDir := t.TempDir()
				basePath := filepath.Join(tmpDir, "prompts")
				path := filepath.Join(basePath, "missing.md")
				entry := &fakeDirEntry{name: "missing.md", isDir: false, info: nil}
				seen := make(map[string]struct{})
				return path, basePath, "local", entry, seen
			},
			wantErr: true,
			wantNil: true,
		},
		{
			name: "invalid relative path returns nil",
			setupFunc: func(t *testing.T) (string, string, string, fs.DirEntry, map[string]struct{}) {
				tmpDir := t.TempDir()
				// Use non-overlapping paths to trigger Rel() error
				path := "/completely/different/path/file.md"
				basePath := filepath.Join(tmpDir, "prompts")
				info, _ := os.Stat(tmpDir) // Use tmpDir info as placeholder
				entry := &fakeDirEntry{name: "file.md", isDir: false, info: info}
				seen := make(map[string]struct{})
				return path, basePath, "local", entry, seen
			},
			wantErr: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, basePath, source, entry, seen := tt.setupFunc(t)
			result, err := buildPromptInfo(path, entry, basePath, source, seen)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantNil {
				assert.Nil(t, result)
			}
		})
	}
}

// TestBuildPromptInfo_EdgeCases verifies boundary conditions
func TestBuildPromptInfo_EdgeCases(t *testing.T) {
	t.Run("empty source string", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(basePath, 0o755))

		testFile := filepath.Join(basePath, "test.md")
		require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o644))

		info, err := os.Stat(testFile)
		require.NoError(t, err)

		entry := &fakeDirEntry{name: "test.md", isDir: false, info: info}
		seen := make(map[string]struct{})

		result, err := buildPromptInfo(testFile, entry, basePath, "", seen)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Source, "empty source should be preserved")
	})

	t.Run("nil seen map should not panic", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(basePath, 0o755))

		testFile := filepath.Join(basePath, "test.md")
		require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o644))

		info, err := os.Stat(testFile)
		require.NoError(t, err)

		entry := &fakeDirEntry{name: "test.md", isDir: false, info: info}

		// Pass nil seen map - should handle gracefully
		result, err := buildPromptInfo(testFile, entry, basePath, "local", nil)

		// Should either handle nil gracefully or return error
		if err != nil {
			t.Logf("nil seen map handled with error: %v", err)
		} else {
			require.NotNil(t, result)
		}
	})

	t.Run("very large file size", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(basePath, 0o755))

		testFile := filepath.Join(basePath, "large.md")
		// Create a large file (1MB)
		largeContent := make([]byte, 1024*1024)
		require.NoError(t, os.WriteFile(testFile, largeContent, 0o644))

		info, err := os.Stat(testFile)
		require.NoError(t, err)

		entry := &fakeDirEntry{name: "large.md", isDir: false, info: info}
		seen := make(map[string]struct{})

		result, err := buildPromptInfo(testFile, entry, basePath, "local", seen)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(1024*1024), result.Size)
	})

	t.Run("zero-byte file", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "prompts")
		require.NoError(t, os.MkdirAll(basePath, 0o755))

		testFile := filepath.Join(basePath, "empty.md")
		require.NoError(t, os.WriteFile(testFile, []byte(""), 0o644))

		info, err := os.Stat(testFile)
		require.NoError(t, err)

		entry := &fakeDirEntry{name: "empty.md", isDir: false, info: info}
		seen := make(map[string]struct{})

		result, err := buildPromptInfo(testFile, entry, basePath, "local", seen)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(0), result.Size, "zero-byte files should report size 0")
	})
}

// TestBuildPromptInfo_ModTimeFormat verifies timestamp formatting
func TestBuildPromptInfo_ModTimeFormat(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "prompts")
	require.NoError(t, os.MkdirAll(basePath, 0o755))

	testFile := filepath.Join(basePath, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("content"), 0o644))

	info, err := os.Stat(testFile)
	require.NoError(t, err)

	entry := &fakeDirEntry{name: "test.md", isDir: false, info: info}
	seen := make(map[string]struct{})

	result, err := buildPromptInfo(testFile, entry, basePath, "local", seen)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify ModTime format matches expected pattern: "2006-01-02 15:04:05"
	assert.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, result.ModTime,
		"ModTime should match format YYYY-MM-DD HH:MM:SS")
}

// fakeDirEntry is a test double for fs.DirEntry
type fakeDirEntry struct {
	name  string
	isDir bool
	info  fs.FileInfo
}

func (f *fakeDirEntry) Name() string {
	return f.name
}

func (f *fakeDirEntry) IsDir() bool {
	return f.isDir
}

func (f *fakeDirEntry) Type() fs.FileMode {
	if f.isDir {
		return fs.ModeDir
	}
	return 0
}

func (f *fakeDirEntry) Info() (fs.FileInfo, error) {
	if f.info == nil {
		return nil, fs.ErrNotExist
	}
	return f.info, nil
}
