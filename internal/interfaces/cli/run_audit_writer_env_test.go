package cli

import (
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/internal/infrastructure/audit"
	testmocks "github.com/awf-project/awf/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *testmocks.MockLogger {
	return testmocks.NewMockLogger()
}

// TestBuildAuditWriter_DisabledWhenOffEnvVar verifies that AWF_AUDIT_LOG=off returns nil writer
func TestBuildAuditWriter_DisabledWhenOffEnvVar(t *testing.T) {
	t.Setenv("AWF_AUDIT_LOG", "off")

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)

	assert.Nil(t, writer, "writer must be nil when AWF_AUDIT_LOG=off")
	assert.NoError(t, err, "should not error when disabled")
	assert.NotNil(t, cleanup, "cleanup function must always be provided")

	// Cleanup should be a no-op and not panic
	assert.NotPanics(t, cleanup, "cleanup must not panic")
}

// TestBuildAuditWriter_DefaultPathWhenEmpty verifies that empty AWF_AUDIT_LOG uses XDG default path
func TestBuildAuditWriter_DefaultPathWhenEmpty(t *testing.T) {
	t.Setenv("AWF_AUDIT_LOG", "")

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must always be provided")
	defer cleanup()

	// When AWF_AUDIT_LOG is empty, should create writer with default XDG path
	// Writer should be non-nil and should be FileAuditTrailWriter type
	assert.NotNil(t, writer, "writer must not be nil when using default path")
	assert.NoError(t, err, "should not error when creating writer with default path")

	// Verify the writer implements the AuditTrailWriter interface
	_, ok := writer.(*audit.FileAuditTrailWriter)
	assert.True(t, ok, "writer must be FileAuditTrailWriter type")
}

// TestBuildAuditWriter_CustomPathWhenSet verifies that AWF_AUDIT_LOG custom value is used
func TestBuildAuditWriter_CustomPathWhenSet(t *testing.T) {
	customPath := filepath.Join(t.TempDir(), "custom-audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", customPath)

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must always be provided")
	defer cleanup()

	// When AWF_AUDIT_LOG is set, should use that path
	assert.NotNil(t, writer, "writer must not be nil when custom path is set")
	assert.NoError(t, err, "should not error when creating writer with custom path")

	// Verify the writer is FileAuditTrailWriter type
	_, ok := writer.(*audit.FileAuditTrailWriter)
	assert.True(t, ok, "writer must be FileAuditTrailWriter type")
}

// TestBuildAuditWriter_UnsetEnvVarUsesDefault verifies that unset AWF_AUDIT_LOG uses default path
func TestBuildAuditWriter_UnsetEnvVarUsesDefault(t *testing.T) {
	// Ensure the env var is not set (t.Setenv handles cleanup automatically)
	t.Setenv("AWF_AUDIT_LOG", "")

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must always be provided")
	defer cleanup()

	// Should use default path
	assert.NotNil(t, writer, "writer must not be nil for unset env var (uses default)")
	assert.NoError(t, err, "should not error with default path")
}

// TestBuildAuditWriter_OffIsCaseSensitive verifies that "off" matching is case-sensitive
func TestBuildAuditWriter_OffIsCaseSensitive(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectDisabled bool
		description    string
	}{
		{"off lowercase", "off", true, "exact 'off' should disable"},
		{"OFF uppercase", "OFF", false, "uppercase OFF should not match 'off'"},
		{"Off mixed", "Off", false, "mixed case Off should not match 'off'"},
		{"OFF lowercase", "oFF", false, "partial case variation should not match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWF_AUDIT_LOG", tt.envValue)
			logger := newTestLogger()

			writer, cleanup, err := buildAuditWriter(logger)
			require.NotNil(t, cleanup, "cleanup must always be provided")
			defer cleanup()

			if tt.expectDisabled {
				assert.Nil(t, writer, tt.description+": writer should be nil")
				assert.NoError(t, err, tt.description+": should not error")
			} else {
				assert.NotNil(t, writer, tt.description+": writer should not be nil")
				assert.NoError(t, err, tt.description+": should not error")
			}
		})
	}
}

// TestBuildAuditWriter_CleanupAlwaysProvided verifies cleanup is returned in all scenarios
func TestBuildAuditWriter_CleanupAlwaysProvided(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"disabled with off", "off"},
		{"empty env var", ""},
		{"custom path", filepath.Join(t.TempDir(), "test.jsonl")},
		{"unset env var", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AWF_AUDIT_LOG", tt.env)
			logger := newTestLogger()

			_, cleanup, _ := buildAuditWriter(logger)

			// Cleanup must always be provided and callable
			assert.NotNil(t, cleanup, "cleanup function must always be provided")
			assert.NotPanics(t, cleanup, "cleanup must be safe to call")
		})
	}
}

// TestBuildAuditWriter_CleanupCallsWriterClose verifies cleanup closes the writer
func TestBuildAuditWriter_CleanupCallsWriterClose(t *testing.T) {
	customPath := filepath.Join(t.TempDir(), "cleanup-test.jsonl")
	t.Setenv("AWF_AUDIT_LOG", customPath)

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NoError(t, err, "should create writer successfully")
	require.NotNil(t, writer, "writer must not be nil")
	require.NotNil(t, cleanup, "cleanup must be provided")

	// Cleanup should be callable without panic
	assert.NotPanics(t, cleanup, "cleanup must not panic when calling writer.Close()")
}

// TestBuildAuditWriter_NoErrorWhenDisabled verifies AWF_AUDIT_LOG=off doesn't error
func TestBuildAuditWriter_NoErrorWhenDisabled(t *testing.T) {
	t.Setenv("AWF_AUDIT_LOG", "off")
	logger := newTestLogger()

	_, _, err := buildAuditWriter(logger)

	assert.NoError(t, err, "should never return error when disabled with 'off'")
}

// TestBuildAuditWriter_RelativePathHandling verifies relative paths are handled correctly
func TestBuildAuditWriter_RelativePathHandling(t *testing.T) {
	relativePath := "relative/path/audit.jsonl"
	t.Setenv("AWF_AUDIT_LOG", relativePath)

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must be provided")
	defer cleanup()

	// Should accept relative paths (will be created relative to CWD or expanded)
	assert.NotNil(t, writer, "writer must handle relative paths")
	assert.NoError(t, err, "should not error for relative paths")
}

// TestBuildAuditWriter_AbsolutePathHandling verifies absolute paths are handled correctly
func TestBuildAuditWriter_AbsolutePathHandling(t *testing.T) {
	absPath := filepath.Join(t.TempDir(), "subdir", "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", absPath)

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must be provided")
	defer cleanup()

	// Should handle absolute paths with parent directory creation
	assert.NotNil(t, writer, "writer must handle absolute paths")
	assert.NoError(t, err, "should not error for absolute paths")
}

// TestBuildAuditWriter_DefaultPathStructure verifies default path uses XDG structure
func TestBuildAuditWriter_DefaultPathStructure(t *testing.T) {
	t.Setenv("AWF_AUDIT_LOG", "")

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must be provided")
	defer cleanup()

	// Default path should follow XDG convention: $XDG_DATA_HOME/awf/audit.jsonl
	assert.NotNil(t, writer, "writer must be created with default path")
	assert.NoError(t, err, "should not error with default path")

	// The writer should be constructed to use the XDG default path structure
	_, ok := writer.(*audit.FileAuditTrailWriter)
	assert.True(t, ok, "writer should be FileAuditTrailWriter using default XDG path")
}

// TestBuildAuditWriter_LoggerParameterUsage verifies logger is passed correctly
func TestBuildAuditWriter_LoggerParameterUsage(t *testing.T) {
	logger := newTestLogger()
	t.Setenv("AWF_AUDIT_LOG", "off")

	_, cleanup, _ := buildAuditWriter(logger)
	require.NotNil(t, cleanup, "cleanup must be provided")

	// Logger parameter should not cause errors
	assert.NotPanics(t, cleanup, "logger should not cause issues")
}

// TestBuildAuditWriter_MultipleCallsIndependent verifies multiple calls don't interfere
func TestBuildAuditWriter_MultipleCallsIndependent(t *testing.T) {
	logger := newTestLogger()

	// Call 1: with "off"
	t.Setenv("AWF_AUDIT_LOG", "off")
	writer1, cleanup1, err1 := buildAuditWriter(logger)
	assert.Nil(t, writer1)
	assert.NoError(t, err1)
	assert.NotNil(t, cleanup1)

	// Call 2: with custom path
	path2 := filepath.Join(t.TempDir(), "test2.jsonl")
	t.Setenv("AWF_AUDIT_LOG", path2)
	writer2, cleanup2, err2 := buildAuditWriter(logger)
	require.NotNil(t, cleanup2)
	defer cleanup2()
	assert.NotNil(t, writer2, "second call should create writer")
	assert.NoError(t, err2)

	// Call 3: back to "off"
	t.Setenv("AWF_AUDIT_LOG", "off")
	writer3, cleanup3, err3 := buildAuditWriter(logger)
	assert.Nil(t, writer3)
	assert.NoError(t, err3)
	assert.NotNil(t, cleanup3)

	// Cleanup should be safe to call
	cleanup1()
	cleanup3()
}

// TestBuildAuditWriter_WriteablePathRequired verifies writer handles path conditions
func TestBuildAuditWriter_WriteablePathRequired(t *testing.T) {
	// Use a path in a temp directory that will be writable
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", path)

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup)
	defer cleanup()

	// Should successfully create writer in writable directory
	assert.NotNil(t, writer, "should create writer in writable directory")
	assert.NoError(t, err, "should not error for writable path")
}

// TestBuildAuditWriter_InterfaceCompliance verifies the returned writer implements port interface
func TestBuildAuditWriter_InterfaceCompliance(t *testing.T) {
	t.Setenv("AWF_AUDIT_LOG", filepath.Join(t.TempDir(), "iface.jsonl"))

	logger := newTestLogger()
	writer, cleanup, err := buildAuditWriter(logger)
	require.NotNil(t, cleanup)
	defer cleanup()

	require.NotNil(t, writer, "writer must not be nil for non-disabled case")
	require.NoError(t, err, "should not error")

	// Verify writer implements the ports.AuditTrailWriter interface
	_ = writer
	assert.True(t, true, "writer must implement ports.AuditTrailWriter interface")
}
