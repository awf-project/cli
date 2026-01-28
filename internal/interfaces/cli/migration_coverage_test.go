package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T002
// Feature: C028

// resetMigrationState resets the package-level migration state between tests
func resetMigrationState() {
	migrationNoticeMu.Lock()
	defer migrationNoticeMu.Unlock()
	migrationNoticeShown = false
}

// TestCheckMigration_NoLegacyDir tests that no notice is shown when legacy directory doesn't exist
func TestCheckMigration_NoLegacyDir(t *testing.T) {
	resetMigrationState()

	// Set up a temporary home directory without .awf
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	buf := &bytes.Buffer{}
	CheckMigration(buf)

	// Should produce no output when no legacy directory exists
	assert.Empty(t, buf.String(), "expected no output when legacy directory doesn't exist")
}

// TestCheckMigration_WithLegacyDir tests that notice is shown when legacy directory exists
func TestCheckMigration_WithLegacyDir(t *testing.T) {
	resetMigrationState()

	// Set up a temporary home directory with .awf
	tmpHome := t.TempDir()
	legacyDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755), "failed to create legacy directory")
	t.Setenv("HOME", tmpHome)

	buf := &bytes.Buffer{}
	CheckMigration(buf)

	// Should produce output when legacy directory exists
	output := buf.String()
	assert.NotEmpty(t, output, "expected output when legacy directory exists")
	assert.Contains(t, output, "NOTICE: Legacy ~/.awf directory detected", "expected notice message")
	assert.Contains(t, output, "AWF now uses XDG Base Directory Specification", "expected XDG explanation")
	assert.Contains(t, output, "To migrate, move your files:", "expected migration instructions")
}

// TestCheckMigration_SuppressionOnSecondCall tests that notice is only shown once (singleton pattern)
func TestCheckMigration_SuppressionOnSecondCall(t *testing.T) {
	resetMigrationState()

	// Set up a temporary home directory with .awf
	tmpHome := t.TempDir()
	legacyDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755), "failed to create legacy directory")
	t.Setenv("HOME", tmpHome)

	// First call should show the notice
	buf1 := &bytes.Buffer{}
	CheckMigration(buf1)
	assert.NotEmpty(t, buf1.String(), "expected output on first call")

	// Second call should NOT show the notice (suppressed)
	buf2 := &bytes.Buffer{}
	CheckMigration(buf2)
	assert.Empty(t, buf2.String(), "expected no output on second call (suppressed)")
}

// TestCheckMigration_ConcurrentCalls tests thread-safety of the singleton pattern
func TestCheckMigration_ConcurrentCalls(t *testing.T) {
	resetMigrationState()

	// Set up a temporary home directory with .awf
	tmpHome := t.TempDir()
	legacyDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755), "failed to create legacy directory")
	t.Setenv("HOME", tmpHome)

	// Spawn multiple goroutines calling CheckMigration concurrently
	const numGoroutines = 10
	outputs := make([]*bytes.Buffer, numGoroutines)
	done := make(chan int, numGoroutines)

	for i := range numGoroutines {
		outputs[i] = &bytes.Buffer{}
		go func(idx int) {
			CheckMigration(outputs[idx])
			done <- idx
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// Exactly one output should contain the notice
	nonEmptyCount := 0
	for i := range numGoroutines {
		if outputs[i].Len() > 0 {
			nonEmptyCount++
			assert.Contains(t, outputs[i].String(), "NOTICE: Legacy ~/.awf directory detected")
		}
	}

	assert.Equal(t, 1, nonEmptyCount, "expected exactly one goroutine to show the notice")
}

// TestCheckMigration_OutputFormat tests the format of the migration notice
func TestCheckMigration_OutputFormat(t *testing.T) {
	resetMigrationState()

	// Set up a temporary home directory with .awf
	tmpHome := t.TempDir()
	legacyDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755), "failed to create legacy directory")
	t.Setenv("HOME", tmpHome)

	buf := &bytes.Buffer{}
	CheckMigration(buf)

	output := buf.String()

	// Verify key sections are present
	assert.Contains(t, output, "NOTICE: Legacy ~/.awf directory detected")
	assert.Contains(t, output, "AWF now uses XDG Base Directory Specification:")
	assert.Contains(t, output, "Config:")
	assert.Contains(t, output, "Data:")
	assert.Contains(t, output, "Workflows:")
	assert.Contains(t, output, "To migrate, move your files:")
	assert.Contains(t, output, "mv ~/.awf/workflows/*")
	assert.Contains(t, output, "mv ~/.awf/storage/states/*")

	// Verify output starts with blank line
	assert.True(t, output != "" && output[0] == '\n', "expected output to start with blank line")
}

// TestCheckMigration_NilWriter tests edge case of nil writer (will panic)
func TestCheckMigration_NilWriter(t *testing.T) {
	resetMigrationState()

	// Set up a temporary home directory with .awf
	tmpHome := t.TempDir()
	legacyDir := filepath.Join(tmpHome, ".awf")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755), "failed to create legacy directory")
	t.Setenv("HOME", tmpHome)

	// Nil writer causes panic when fmt.Fprintln is called
	assert.Panics(t, func() {
		CheckMigration(nil)
	}, "CheckMigration should panic with nil writer")
}

// TestCheckMigration_EmptyHomeDir tests edge case when HOME is not set
func TestCheckMigration_EmptyHomeDir(t *testing.T) {
	resetMigrationState()

	// Unset HOME environment variable
	t.Setenv("HOME", "")

	buf := &bytes.Buffer{}

	// Should not panic when HOME is empty
	assert.NotPanics(t, func() {
		CheckMigration(buf)
	}, "CheckMigration should not panic when HOME is empty")

	// Should produce no output (LegacyDirExists will return false)
	assert.Empty(t, buf.String(), "expected no output when HOME is empty")
}
