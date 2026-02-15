package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiLogger_ImplementsInterface(t *testing.T) {
	var _ ports.Logger = (*MultiLogger)(nil)
}

func TestMultiLogger_DelegatesToAll(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	var consoleBuf bytes.Buffer

	jsonLogger, err := NewJSONLogger(logPath, LevelInfo)
	require.NoError(t, err)
	defer func() { _ = jsonLogger.Close() }()

	consoleLogger := NewConsoleLogger(&consoleBuf, LevelInfo, false)

	multi := NewMultiLogger(jsonLogger, consoleLogger)
	multi.Info("test message", "key", "value")
	_ = jsonLogger.Sync()

	// Check JSON file
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var entry map[string]any
	err = json.Unmarshal(content, &entry)
	require.NoError(t, err)
	assert.Equal(t, "test message", entry["msg"])

	// Check console output
	assert.Contains(t, consoleBuf.String(), "test message")
	assert.Contains(t, consoleBuf.String(), "key=value")
}

func TestMultiLogger_AllLevels(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger1 := NewConsoleLogger(&buf1, LevelDebug, false)
	logger2 := NewConsoleLogger(&buf2, LevelDebug, false)

	multi := NewMultiLogger(logger1, logger2)

	multi.Debug("debug msg")
	multi.Info("info msg")
	multi.Warn("warn msg")
	multi.Error("error msg")

	for _, buf := range []*bytes.Buffer{&buf1, &buf2} {
		output := buf.String()
		assert.Contains(t, output, "debug msg")
		assert.Contains(t, output, "info msg")
		assert.Contains(t, output, "warn msg")
		assert.Contains(t, output, "error msg")
	}
}

func TestMultiLogger_WithContext(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger1 := NewConsoleLogger(&buf1, LevelInfo, false)
	logger2 := NewConsoleLogger(&buf2, LevelInfo, false)

	multi := NewMultiLogger(logger1, logger2)
	ctx := map[string]any{"workflow_id": "test-123"}

	ctxMulti := multi.WithContext(ctx)
	ctxMulti.Info("with context")

	for _, buf := range []*bytes.Buffer{&buf1, &buf2} {
		assert.Contains(t, buf.String(), "workflow_id=test-123")
	}
}

func TestMultiLogger_Empty(t *testing.T) {
	multi := NewMultiLogger()

	// Should not panic with no loggers
	multi.Debug("test")
	multi.Info("test")
	multi.Warn("test")
	multi.Error("test")
	multi.WithContext(map[string]any{})
}

func TestMultiLogger_SingleLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, LevelInfo, false)

	multi := NewMultiLogger(logger)
	multi.Info("test")

	assert.Contains(t, buf.String(), "test")
}

func TestNopLogger_ImplementsInterface(t *testing.T) {
	var _ ports.Logger = NopLogger{}
}

func TestNopLogger_NoOp(t *testing.T) {
	logger := NopLogger{}

	// Should not panic
	logger.Debug("test")
	logger.Info("test")
	logger.Warn("test")
	logger.Error("test")

	ctx := logger.WithContext(map[string]any{"key": "value"})
	assert.NotNil(t, ctx)
}
