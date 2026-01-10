package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanoix/awf/internal/domain/ports"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level represents log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

func (l Level) toZapLevel() zapcore.Level {
	switch l {
	case LevelDebug:
		return zapcore.DebugLevel
	case LevelInfo:
		return zapcore.InfoLevel
	case LevelWarn:
		return zapcore.WarnLevel
	case LevelError:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// ParseLevel parses a string into a Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// JSONLogger implements ports.Logger with JSON output using zap.
type JSONLogger struct {
	logger *zap.Logger
	masker *SecretMasker
	file   *os.File
}

// NewJSONLogger creates a JSON logger that writes to a file.
func NewJSONLogger(path string, level Level) (*JSONLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		MessageKey:     "msg",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(file),
		level.toZapLevel(),
	)

	return &JSONLogger{
		logger: zap.New(core),
		masker: NewSecretMasker(),
		file:   file,
	}, nil
}

func (l *JSONLogger) Debug(msg string, fields ...any) {
	l.logger.Debug(msg, l.toZapFields(fields)...)
}

func (l *JSONLogger) Info(msg string, fields ...any) {
	l.logger.Info(msg, l.toZapFields(fields)...)
}

func (l *JSONLogger) Warn(msg string, fields ...any) {
	l.logger.Warn(msg, l.toZapFields(fields)...)
}

func (l *JSONLogger) Error(msg string, fields ...any) {
	l.logger.Error(msg, l.toZapFields(fields)...)
}

func (l *JSONLogger) WithContext(ctx map[string]any) ports.Logger {
	fields := make([]zap.Field, 0, len(ctx))
	for k, v := range ctx {
		fields = append(fields, zap.Any(k, v))
	}
	return &JSONLogger{
		logger: l.logger.With(fields...),
		masker: l.masker,
		file:   l.file,
	}
}

// Sync flushes buffered logs.
func (l *JSONLogger) Sync() error {
	if err := l.logger.Sync(); err != nil {
		return fmt.Errorf("sync logger: %w", err)
	}
	return nil
}

// Close closes the log file.
func (l *JSONLogger) Close() error {
	_ = l.logger.Sync()
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("close log file: %w", err)
		}
		return nil
	}
	return nil
}

func (l *JSONLogger) toZapFields(fields []any) []zap.Field {
	masked := l.masker.MaskFields(fields)
	zapFields := make([]zap.Field, 0, len(masked)/2)

	for i := 0; i+1 < len(masked); i += 2 {
		key, ok := masked[i].(string)
		if !ok {
			continue
		}
		zapFields = append(zapFields, zap.Any(key, masked[i+1]))
	}

	return zapFields
}
