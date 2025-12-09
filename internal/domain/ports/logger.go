package ports

// Logger defines the logging contract for the domain layer.
// Implementations live in infrastructure layer.
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	WithContext(ctx map[string]any) Logger
}
