package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
)

func TestJSONLogger_ImplementsInterface(t *testing.T) {
	var _ ports.Logger = (*JSONLogger)(nil)
}

func TestJSONLogger_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewJSONLogger(logPath, LevelDebug)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Info("test message", "key", "value")
	_ = logger.Sync()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var entry map[string]any
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)

	assert.Equal(t, "info", entry["level"])
	assert.Equal(t, "test message", entry["msg"])
	assert.Equal(t, "value", entry["key"])
	assert.Contains(t, entry, "ts")
}

func TestJSONLogger_LogLevels(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		level     Level
		logMethod string
		shouldLog bool
	}{
		{LevelDebug, "debug", true},
		{LevelDebug, "info", true},
		{LevelDebug, "warn", true},
		{LevelDebug, "error", true},
		{LevelInfo, "debug", false},
		{LevelInfo, "info", true},
		{LevelInfo, "warn", true},
		{LevelInfo, "error", true},
		{LevelWarn, "debug", false},
		{LevelWarn, "info", false},
		{LevelWarn, "warn", true},
		{LevelWarn, "error", true},
		{LevelError, "debug", false},
		{LevelError, "info", false},
		{LevelError, "warn", false},
		{LevelError, "error", true},
	}

	for _, tt := range tests {
		name := tt.level.String() + "_allows_" + tt.logMethod
		t.Run(name, func(t *testing.T) {
			logPath := filepath.Join(tmpDir, name+".log")
			logger, err := NewJSONLogger(logPath, tt.level)
			require.NoError(t, err)
			defer func() { _ = logger.Close() }()

			switch tt.logMethod {
			case "debug":
				logger.Debug("test")
			case "info":
				logger.Info("test")
			case "warn":
				logger.Warn("test")
			case "error":
				logger.Error("test")
			}
			_ = logger.Sync()

			content, _ := os.ReadFile(logPath)
			hasContent := len(strings.TrimSpace(string(content))) > 0

			assert.Equal(t, tt.shouldLog, hasContent)
		})
	}
}

func TestJSONLogger_WithContext(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "context.log")

	logger, err := NewJSONLogger(logPath, LevelInfo)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	ctx := map[string]any{
		"workflow_id":   "test-123",
		"workflow_name": "my-workflow",
	}

	ctxLogger := logger.WithContext(ctx)
	ctxLogger.Info("with context")
	_ = logger.Sync()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var entry map[string]any
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)

	assert.Equal(t, "test-123", entry["workflow_id"])
	assert.Equal(t, "my-workflow", entry["workflow_name"])
}

func TestJSONLogger_MasksSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "secrets.log")

	logger, err := NewJSONLogger(logPath, LevelInfo)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Info("secret test", "API_KEY", "sk-12345", "user", "john")
	_ = logger.Sync()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var entry map[string]any
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)

	assert.Equal(t, "***", entry["API_KEY"])
	assert.Equal(t, "john", entry["user"])
}

func TestJSONLogger_FieldTypes(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "types.log")

	logger, err := NewJSONLogger(logPath, LevelInfo)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Info("types test",
		"string", "hello",
		"int", 42,
		"float", 3.14,
		"bool", true,
	)
	_ = logger.Sync()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var entry map[string]any
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)

	assert.Equal(t, "hello", entry["string"])
	assert.Equal(t, float64(42), entry["int"])
	assert.Equal(t, 3.14, entry["float"])
	assert.Equal(t, true, entry["bool"])
}

func TestJSONLogger_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nested", "dir", "test.log")

	logger, err := NewJSONLogger(logPath, LevelInfo)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Info("test")
	_ = logger.Sync()

	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
		{Level(99), "info"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.level.String())
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"invalid", LevelInfo},
		{"", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseLevel(tt.input))
		})
	}
}
