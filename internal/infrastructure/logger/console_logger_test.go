package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestConsoleLogger_ImplementsInterface(t *testing.T) {
	var _ ports.Logger = (*ConsoleLogger)(nil)
}

func TestConsoleLogger_WritesToBuffer(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, LevelDebug, false)

	logger.Info("test message", "key", "value")

	output := buf.String()
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key=value")
}

func TestConsoleLogger_LogLevels(t *testing.T) {
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
			var buf bytes.Buffer
			logger := NewConsoleLogger(&buf, tt.level, false)

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

			hasOutput := buf.Len() > 0
			assert.Equal(t, tt.shouldLog, hasOutput)
		})
	}
}

func TestConsoleLogger_LevelPrefixes(t *testing.T) {
	tests := []struct {
		method string
		prefix string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewConsoleLogger(&buf, LevelDebug, false)

			switch tt.method {
			case "debug":
				logger.Debug("test")
			case "info":
				logger.Info("test")
			case "warn":
				logger.Warn("test")
			case "error":
				logger.Error("test")
			}

			assert.Contains(t, buf.String(), tt.prefix)
		})
	}
}

func TestConsoleLogger_WithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, LevelInfo, false)

	ctx := map[string]any{
		"workflow_id": "test-123",
	}

	ctxLogger := logger.WithContext(ctx)
	ctxLogger.Info("with context")

	output := buf.String()
	assert.Contains(t, output, "workflow_id=test-123")
}

func TestConsoleLogger_MasksSecrets(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, LevelInfo, false)

	logger.Info("secret test", "API_KEY", "sk-12345", "user", "john")

	output := buf.String()
	assert.Contains(t, output, "API_KEY=***")
	assert.Contains(t, output, "user=john")
	assert.NotContains(t, output, "sk-12345")
}

func TestConsoleLogger_FieldFormatting(t *testing.T) {
	tests := []struct {
		name     string
		fields   []any
		expected string
	}{
		{
			name:     "string value",
			fields:   []any{"key", "value"},
			expected: "key=value",
		},
		{
			name:     "int value",
			fields:   []any{"count", 42},
			expected: "count=42",
		},
		{
			name:     "bool value",
			fields:   []any{"enabled", true},
			expected: "enabled=true",
		},
		{
			name:     "multiple fields",
			fields:   []any{"a", "1", "b", "2"},
			expected: "a=1 b=2",
		},
		{
			name:     "string with spaces",
			fields:   []any{"msg", "hello world"},
			expected: `msg="hello world"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewConsoleLogger(&buf, LevelInfo, false)

			logger.Info("test", tt.fields...)

			assert.Contains(t, buf.String(), tt.expected)
		})
	}
}

func TestConsoleLogger_Timestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, LevelInfo, false)

	logger.Info("test")

	output := buf.String()
	// Should contain time in format like "15:04:05" or similar
	assert.True(t, strings.Contains(output, ":"), "should contain timestamp")
}

func TestConsoleLogger_NoColor(t *testing.T) {
	var buf bytes.Buffer
	logger := NewConsoleLogger(&buf, LevelInfo, false)

	logger.Error("test")

	output := buf.String()
	// ANSI escape codes start with \x1b[
	assert.NotContains(t, output, "\x1b[")
}
