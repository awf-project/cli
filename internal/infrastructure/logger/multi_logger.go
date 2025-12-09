package logger

import "github.com/vanoix/awf/internal/domain/ports"

// MultiLogger delegates log calls to multiple loggers.
type MultiLogger struct {
	loggers []ports.Logger
}

// NewMultiLogger creates a logger that writes to all provided loggers.
func NewMultiLogger(loggers ...ports.Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

func (m *MultiLogger) Debug(msg string, fields ...any) {
	for _, l := range m.loggers {
		l.Debug(msg, fields...)
	}
}

func (m *MultiLogger) Info(msg string, fields ...any) {
	for _, l := range m.loggers {
		l.Info(msg, fields...)
	}
}

func (m *MultiLogger) Warn(msg string, fields ...any) {
	for _, l := range m.loggers {
		l.Warn(msg, fields...)
	}
}

func (m *MultiLogger) Error(msg string, fields ...any) {
	for _, l := range m.loggers {
		l.Error(msg, fields...)
	}
}

func (m *MultiLogger) WithContext(ctx map[string]any) ports.Logger {
	wrapped := make([]ports.Logger, len(m.loggers))
	for i, l := range m.loggers {
		wrapped[i] = l.WithContext(ctx)
	}
	return &MultiLogger{loggers: wrapped}
}

// NopLogger is a no-op logger that discards all output.
type NopLogger struct{}

func (NopLogger) Debug(msg string, fields ...any) {}
func (NopLogger) Info(msg string, fields ...any)  {}
func (NopLogger) Warn(msg string, fields ...any)  {}
func (NopLogger) Error(msg string, fields ...any) {}
func (NopLogger) WithContext(ctx map[string]any) ports.Logger {
	return NopLogger{}
}
