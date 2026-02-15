package application_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/application"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: C019 - Output streaming to prevent OOM

// TestNewOutputStreamer tests creating an output streamer with various configurations.
func TestNewOutputStreamer(t *testing.T) {
	tests := []struct {
		name   string
		config workflow.OutputLimits
	}{
		{
			name: "default config with streaming disabled",
			config: workflow.OutputLimits{
				MaxSize:           1048576,
				StreamLargeOutput: false,
				TempDir:           "",
			},
		},
		{
			name: "streaming enabled with default temp dir",
			config: workflow.OutputLimits{
				MaxSize:           1048576,
				StreamLargeOutput: true,
				TempDir:           "",
			},
		},
		{
			name: "streaming enabled with custom temp dir",
			config: workflow.OutputLimits{
				MaxSize:           1048576,
				StreamLargeOutput: true,
				TempDir:           "/tmp/awf-test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamer := application.NewOutputStreamer(tt.config)
			require.NotNil(t, streamer)
		})
	}
}

// TestOutputStreamer_StreamOutput_SmallContent tests that small content is not streamed.
func TestOutputStreamer_StreamOutput_SmallContent(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1024, // 1KB limit
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	tests := []struct {
		name    string
		content string
	}{
		{"empty string", ""},
		{"small string", "hello world"},
		{"100 bytes", strings.Repeat("a", 100)},
		{"exactly at limit", strings.Repeat("x", 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := streamer.StreamOutput(tt.content)
			require.NoError(t, err)
			assert.Empty(t, path, "small content should not be streamed")
		})
	}
}

// TestOutputStreamer_StreamOutput_LargeContent tests streaming large content to files.
func TestOutputStreamer_StreamOutput_LargeContent(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1024, // 1KB limit
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	tests := []struct {
		name       string
		contentGen func() string
	}{
		{
			name:       "1025 bytes (just over limit)",
			contentGen: func() string { return strings.Repeat("a", 1025) },
		},
		{
			name:       "10KB",
			contentGen: func() string { return strings.Repeat("b", 10240) },
		},
		{
			name:       "1MB",
			contentGen: func() string { return strings.Repeat("c", 1048576) },
		},
		{
			name: "large multiline content",
			contentGen: func() string {
				return strings.Repeat("line of output data\n", 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := tt.contentGen()
			path, err := streamer.StreamOutput(content)
			require.NoError(t, err)
			require.NotEmpty(t, path, "large content should be streamed to file")

			// Verify file exists
			_, err = os.Stat(path)
			require.NoError(t, err, "streamed file should exist")

			// Verify file content matches
			fileContent, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, content, string(fileContent), "file content should match original")

			// Verify file is in correct directory
			assert.Contains(t, path, config.TempDir, "file should be in configured temp dir")

			// Cleanup
			os.Remove(path)
		})
	}
}

// TestOutputStreamer_StreamOutput_FilePattern tests temp file naming pattern.
func TestOutputStreamer_StreamOutput_FilePattern(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	content := strings.Repeat("x", 200)
	path, err := streamer.StreamOutput(content)
	require.NoError(t, err)

	// Verify file naming pattern (should include awf prefix and timestamp)
	basename := filepath.Base(path)
	assert.Contains(t, basename, "awf-output", "file should have awf prefix")

	// Cleanup
	os.Remove(path)
}

// TestOutputStreamer_StreamOutput_UniqueFiles tests that multiple streams create unique files.
func TestOutputStreamer_StreamOutput_UniqueFiles(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	content1 := strings.Repeat("a", 200)
	content2 := strings.Repeat("b", 200)

	path1, err := streamer.StreamOutput(content1)
	require.NoError(t, err)

	path2, err := streamer.StreamOutput(content2)
	require.NoError(t, err)

	// Verify paths are different
	assert.NotEqual(t, path1, path2, "each stream should create a unique file")

	// Verify both files exist with correct content
	data1, err := os.ReadFile(path1)
	require.NoError(t, err)
	assert.Equal(t, content1, string(data1))

	data2, err := os.ReadFile(path2)
	require.NoError(t, err)
	assert.Equal(t, content2, string(data2))

	// Cleanup
	os.Remove(path1)
	os.Remove(path2)
}

// TestOutputStreamer_StreamOutput_EmptyString tests handling empty strings.
func TestOutputStreamer_StreamOutput_EmptyString(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	path, err := streamer.StreamOutput("")
	require.NoError(t, err)
	assert.Empty(t, path, "empty string should not be streamed")
}

// TestOutputStreamer_StreamOutput_InvalidTempDir tests error handling for invalid temp directories.
func TestOutputStreamer_StreamOutput_InvalidTempDir(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           "/nonexistent/invalid/path/that/cannot/be/created",
	}
	streamer := application.NewOutputStreamer(config)

	content := strings.Repeat("x", 200)
	path, err := streamer.StreamOutput(content)

	assert.Error(t, err, "should error when temp directory is invalid")
	assert.Empty(t, path)
}

// TestOutputStreamer_StreamOutput_Unicode tests streaming content with multibyte Unicode characters.
func TestOutputStreamer_StreamOutput_Unicode(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	// Unicode content with emoji and multibyte characters exceeding limit
	content := strings.Repeat("Hello 世界 🌍 ", 50)
	path, err := streamer.StreamOutput(content)
	require.NoError(t, err)
	require.NotEmpty(t, path, "large unicode content should be streamed")

	// Verify content is preserved correctly
	fileContent, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(fileContent), "unicode content should be preserved")

	// Cleanup
	os.Remove(path)
}

// TestOutputStreamer_StreamBoth_HappyPath tests streaming both output and stderr.
func TestOutputStreamer_StreamBoth_HappyPath(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1024,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	tests := []struct {
		name         string
		output       string
		stderr       string
		expectOutPtr bool
		expectErrPtr bool
	}{
		{
			name:         "both small (no streaming)",
			output:       "small output",
			stderr:       "small error",
			expectOutPtr: false,
			expectErrPtr: false,
		},
		{
			name:         "large output, small stderr",
			output:       strings.Repeat("o", 2000),
			stderr:       "small error",
			expectOutPtr: true,
			expectErrPtr: false,
		},
		{
			name:         "small output, large stderr",
			output:       "small output",
			stderr:       strings.Repeat("e", 2000),
			expectOutPtr: false,
			expectErrPtr: true,
		},
		{
			name:         "both large (stream both)",
			output:       strings.Repeat("o", 2000),
			stderr:       strings.Repeat("e", 2000),
			expectOutPtr: true,
			expectErrPtr: true,
		},
		{
			name:         "empty both",
			output:       "",
			stderr:       "",
			expectOutPtr: false,
			expectErrPtr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath, stderrPath, err := streamer.StreamBoth(tt.output, tt.stderr)
			require.NoError(t, err)

			// Check output path
			if tt.expectOutPtr {
				assert.NotEmpty(t, outputPath, "large output should be streamed")
				fileContent, err := os.ReadFile(outputPath)
				require.NoError(t, err)
				assert.Equal(t, tt.output, string(fileContent))
				os.Remove(outputPath)
			} else {
				assert.Empty(t, outputPath, "small output should not be streamed")
			}

			// Check stderr path
			if tt.expectErrPtr {
				assert.NotEmpty(t, stderrPath, "large stderr should be streamed")
				fileContent, err := os.ReadFile(stderrPath)
				require.NoError(t, err)
				assert.Equal(t, tt.stderr, string(fileContent))
				os.Remove(stderrPath)
			} else {
				assert.Empty(t, stderrPath, "small stderr should not be streamed")
			}
		})
	}
}

// TestOutputStreamer_StreamBoth_DifferentContent tests that output and stderr stream independently.
func TestOutputStreamer_StreamBoth_DifferentContent(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	output := strings.Repeat("OUTPUT", 50)
	stderr := strings.Repeat("ERROR", 50)

	outputPath, stderrPath, err := streamer.StreamBoth(output, stderr)
	require.NoError(t, err)
	require.NotEmpty(t, outputPath)
	require.NotEmpty(t, stderrPath)
	assert.NotEqual(t, outputPath, stderrPath, "output and stderr should stream to different files")

	// Verify content
	outContent, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, output, string(outContent))

	errContent, err := os.ReadFile(stderrPath)
	require.NoError(t, err)
	assert.Equal(t, stderr, string(errContent))

	// Cleanup
	os.Remove(outputPath)
	os.Remove(stderrPath)
}

// TestOutputStreamer_CleanupTempFiles tests cleanup of temporary files.
func TestOutputStreamer_CleanupTempFiles(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	content := strings.Repeat("x", 200)
	path, err := streamer.StreamOutput(content)
	require.NoError(t, err)
	require.NotEmpty(t, path)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Cleanup using os.Remove directly
	err = os.Remove(path)
	require.NoError(t, err)

	// Verify file is deleted
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err), "file should be deleted after cleanup")
}

// TestOutputStreamer_StreamOutput_SystemTempDir tests using system temp directory when not specified.
func TestOutputStreamer_StreamOutput_SystemTempDir(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           "", // empty means use system temp dir
	}
	streamer := application.NewOutputStreamer(config)

	content := strings.Repeat("x", 200)
	path, err := streamer.StreamOutput(content)
	require.NoError(t, err)
	require.NotEmpty(t, path)

	// Verify file is in system temp directory
	systemTempDir := os.TempDir()
	assert.Contains(t, path, systemTempDir, "file should be in system temp dir when TempDir is empty")

	// Cleanup
	os.Remove(path)
}

// TestOutputStreamer_StreamOutput_ConcurrentWrites tests concurrent streaming operations.
func TestOutputStreamer_StreamOutput_ConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}

	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	const numGoroutines = 10
	done := make(chan string, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			content := strings.Repeat(string(rune('a'+id%26)), 200)
			path, err := streamer.StreamOutput(content)
			if err != nil {
				errors <- err
				return
			}
			done <- path
		}(i)
	}

	// Collect results
	paths := make([]string, 0, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		select {
		case path := <-done:
			paths = append(paths, path)
		case err := <-errors:
			t.Fatalf("concurrent write failed: %v", err)
		}
	}

	// Verify all paths are unique
	pathSet := make(map[string]bool)
	for _, path := range paths {
		assert.False(t, pathSet[path], "duplicate path detected: %s", path)
		pathSet[path] = true

		// Verify file exists
		_, err := os.Stat(path)
		assert.NoError(t, err)

		// Cleanup
		os.Remove(path)
	}
}

// TestOutputStreamer_StreamOutput_BoundaryValues tests edge cases around the size limit.
func TestOutputStreamer_StreamOutput_BoundaryValues(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           1000,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	tests := []struct {
		name         string
		size         int
		expectStream bool
	}{
		{"0 bytes", 0, false},
		{"1 byte", 1, false},
		{"999 bytes", 999, false},
		{"exactly 1000 bytes", 1000, false},
		{"1001 bytes", 1001, true},
		{"2000 bytes", 2000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := strings.Repeat("x", tt.size)
			path, err := streamer.StreamOutput(content)
			require.NoError(t, err)

			if tt.expectStream {
				assert.NotEmpty(t, path, "content over limit should be streamed")
				os.Remove(path)
			} else {
				assert.Empty(t, path, "content at or under limit should not be streamed")
			}
		})
	}
}

// TestOutputStreamer_StreamOutput_FilePermissions tests that created files have correct permissions.
func TestOutputStreamer_StreamOutput_FilePermissions(t *testing.T) {
	config := workflow.OutputLimits{
		MaxSize:           100,
		StreamLargeOutput: true,
		TempDir:           t.TempDir(),
	}
	streamer := application.NewOutputStreamer(config)

	content := strings.Repeat("x", 200)
	path, err := streamer.StreamOutput(content)
	require.NoError(t, err)
	require.NotEmpty(t, path)

	// Check file permissions (should be readable by owner)
	info, err := os.Stat(path)
	require.NoError(t, err)
	mode := info.Mode()
	assert.True(t, mode.IsRegular(), "should be a regular file")
	assert.True(t, mode.Perm()&0o400 != 0, "file should be readable by owner")

	// Cleanup
	os.Remove(path)
}
