package ports

type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	WithContext(ctx map[string]any) Logger
}

// NopLogger is a no-op Logger. It lives in the domain ports package (zero
// dependencies) so any layer can use it as a defensive fallback without pulling in
// internal/infrastructure/logger.
type NopLogger struct{}

func (NopLogger) Debug(string, ...any)              {}
func (NopLogger) Info(string, ...any)               {}
func (NopLogger) Warn(string, ...any)               {}
func (NopLogger) Error(string, ...any)              {}
func (NopLogger) WithContext(map[string]any) Logger { return NopLogger{} }
