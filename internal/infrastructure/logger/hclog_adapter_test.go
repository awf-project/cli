package logger

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewHCLogAdapter_Constructor(t *testing.T) {
	core, _ := observer.New(zapcore.DebugLevel)
	adapter := NewHCLogAdapter(zap.New(core), "my-plugin")

	assert.Equal(t, "my-plugin", adapter.Name())
	assert.Empty(t, adapter.ImpliedArgs())
}

func TestHCLogAdapter_LevelMapping(t *testing.T) {
	tests := []struct {
		name      string
		logFn     func(*HCLogAdapter)
		wantLevel zapcore.Level
	}{
		{"Trace maps to Debug", func(a *HCLogAdapter) { a.Trace("msg") }, zapcore.DebugLevel},
		{"Debug maps to Debug", func(a *HCLogAdapter) { a.Debug("msg") }, zapcore.DebugLevel},
		{"Info maps to Info", func(a *HCLogAdapter) { a.Info("msg") }, zapcore.InfoLevel},
		{"Warn maps to Warn", func(a *HCLogAdapter) { a.Warn("msg") }, zapcore.WarnLevel},
		{"Error maps to Error", func(a *HCLogAdapter) { a.Error("msg") }, zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			tt.logFn(NewHCLogAdapter(zap.New(core), "test"))

			require.Equal(t, 1, logs.Len())
			assert.Equal(t, tt.wantLevel, logs.All()[0].Level)
		})
	}
}

func TestHCLogAdapter_IsLevelEnabled(t *testing.T) {
	tests := []struct {
		name     string
		zapLevel zapcore.Level
		isTrace  bool
		isDebug  bool
		isInfo   bool
		isWarn   bool
		isError  bool
	}{
		{"Debug level enables all", zapcore.DebugLevel, true, true, true, true, true},
		{"Info level disables Trace and Debug", zapcore.InfoLevel, false, false, true, true, true},
		{"Warn level disables up to Info", zapcore.WarnLevel, false, false, false, true, true},
		{"Error level disables up to Warn", zapcore.ErrorLevel, false, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, _ := observer.New(tt.zapLevel)
			a := NewHCLogAdapter(zap.New(core), "test")

			assert.Equal(t, tt.isTrace, a.IsTrace(), "IsTrace")
			assert.Equal(t, tt.isDebug, a.IsDebug(), "IsDebug")
			assert.Equal(t, tt.isInfo, a.IsInfo(), "IsInfo")
			assert.Equal(t, tt.isWarn, a.IsWarn(), "IsWarn")
			assert.Equal(t, tt.isError, a.IsError(), "IsError")
		})
	}
}

func TestHCLogAdapter_With_ImpliedArgs(t *testing.T) {
	core, _ := observer.New(zapcore.DebugLevel)
	adapter := NewHCLogAdapter(zap.New(core), "test")

	adapted := adapter.With("key1", "value1", "key2", "value2")

	hcAdapter, ok := adapted.(*HCLogAdapter)
	require.True(t, ok)
	assert.Equal(t, []interface{}{"key1", "value1", "key2", "value2"}, hcAdapter.ImpliedArgs())
}

func TestHCLogAdapter_Named_PrependName(t *testing.T) {
	core, _ := observer.New(zapcore.DebugLevel)
	adapter := NewHCLogAdapter(zap.New(core), "parent")

	named := adapter.Named("child")

	hcAdapter, ok := named.(*HCLogAdapter)
	require.True(t, ok)
	assert.Equal(t, "parent.child", hcAdapter.Name())
}

func TestHCLogAdapter_ResetNamed(t *testing.T) {
	core, _ := observer.New(zapcore.DebugLevel)
	adapter := NewHCLogAdapter(zap.New(core), "parent")

	reset := adapter.ResetNamed("sibling")

	hcAdapter, ok := reset.(*HCLogAdapter)
	require.True(t, ok)
	assert.Equal(t, "sibling", hcAdapter.Name())
}

func TestHCLogAdapter_GetLevel(t *testing.T) {
	tests := []struct {
		name      string
		zapLevel  zapcore.Level
		wantLevel hclog.Level
	}{
		{"Debug maps to Debug", zapcore.DebugLevel, hclog.Debug},
		{"Info maps to Info", zapcore.InfoLevel, hclog.Info},
		{"Warn maps to Warn", zapcore.WarnLevel, hclog.Warn},
		{"Error maps to Error", zapcore.ErrorLevel, hclog.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, _ := observer.New(tt.zapLevel)
			adapter := NewHCLogAdapter(zap.New(core), "test")

			assert.Equal(t, tt.wantLevel, adapter.GetLevel())
		})
	}
}

func TestHCLogAdapter_SetLevel_NoOp(t *testing.T) {
	core, _ := observer.New(zapcore.InfoLevel)
	adapter := NewHCLogAdapter(zap.New(core), "test")

	adapter.SetLevel(hclog.Debug)
	assert.Equal(t, hclog.Info, adapter.GetLevel())
}

func TestHCLogAdapter_StandardLoggerAndWriter(t *testing.T) {
	core, _ := observer.New(zapcore.InfoLevel)
	adapter := NewHCLogAdapter(zap.New(core), "test")

	assert.NotNil(t, adapter.StandardLogger(nil))
	assert.NotNil(t, adapter.StandardWriter(nil))
}

func TestHCLogAdapter_LogWithFields(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	adapter := NewHCLogAdapter(zap.New(core), "test")

	adapter.Info("test message", "key1", "value1", "key2", "value2")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Equal(t, "test message", entry.Message)
	assert.Equal(t, zapcore.InfoLevel, entry.Level)
}

func TestHCLogAdapter_SecretMasking(t *testing.T) {
	var buf strings.Builder
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.DebugLevel)
	adapter := NewHCLogAdapter(zap.New(core), "test")

	adapter.Info("secret test", "API_KEY", "sk-12345", "user", "john")

	output := buf.String()
	assert.Contains(t, output, `"API_KEY":"***"`)
	assert.NotContains(t, output, "sk-12345")
	assert.Contains(t, output, `"user":"john"`)
}

func TestHCLogAdapter_SecretMasking_AllPatterns(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		shouldMask bool
	}{
		{"SECRET_ prefix", "SECRET_TOKEN", "token123", true},
		{"API_KEY prefix", "API_KEY", "sk-12345", true},
		{"PASSWORD prefix", "PASSWORD", "hunter2", true},
		{"non-secret key", "username", "john", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
			core := zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.DebugLevel)
			adapter := NewHCLogAdapter(zap.New(core), "test")

			adapter.Info("test", tt.key, tt.value)

			output := buf.String()
			if tt.shouldMask {
				assert.Contains(t, output, `"`+tt.key+`":"***"`)
				assert.NotContains(t, output, tt.value)
			} else {
				assert.Contains(t, output, tt.value)
			}
		})
	}
}

func TestLogWriter_Write_LogsLinesStrippingNewlines(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	w := NewLogWriter(zap.New(core), zapcore.WarnLevel)

	n, err := w.Write([]byte("line one\nline two\n"))

	assert.NoError(t, err)
	assert.Equal(t, 18, n)
	require.Equal(t, 2, logs.Len())
	assert.Equal(t, "line one", logs.All()[0].Message)
	assert.Equal(t, "line two", logs.All()[1].Message)
	assert.Equal(t, zapcore.WarnLevel, logs.All()[0].Level)
}

func TestLogWriter_Write_SingleLineNoNewline(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	w := NewLogWriter(zap.New(core), zapcore.WarnLevel)

	n, err := w.Write([]byte("single line"))

	assert.NoError(t, err)
	assert.Equal(t, 11, n)
	require.Equal(t, 1, logs.Len())
	assert.Equal(t, "single line", logs.All()[0].Message)
}

func TestLogWriter_Write_EmptyInput(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	w := NewLogWriter(zap.New(core), zapcore.WarnLevel)

	n, err := w.Write([]byte{})

	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, logs.Len())
}

func TestLogWriter_Write_OnlyNewlines(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	w := NewLogWriter(zap.New(core), zapcore.ErrorLevel)

	n, err := w.Write([]byte("\n\n\n"))

	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	require.Equal(t, 3, logs.Len())
	assert.Equal(t, "", logs.All()[0].Message)
	assert.Equal(t, "", logs.All()[1].Message)
	assert.Equal(t, "", logs.All()[2].Message)
}

func TestLogWriter_Write_DifferentLevels(t *testing.T) {
	tests := []struct {
		name  string
		level zapcore.Level
	}{
		{"Debug level", zapcore.DebugLevel},
		{"Info level", zapcore.InfoLevel},
		{"Warn level", zapcore.WarnLevel},
		{"Error level", zapcore.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(tt.level)
			w := NewLogWriter(zap.New(core), tt.level)

			_, _ = w.Write([]byte("test\n"))

			require.Equal(t, 1, logs.Len())
			assert.Equal(t, tt.level, logs.All()[0].Level)
		})
	}
}
