package logger

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/vanoix/awf/internal/domain/ports"
)

// ConsoleLogger implements ports.Logger with human-readable console output.
type ConsoleLogger struct {
	writer    io.Writer
	level     Level
	useColor  bool
	masker    *SecretMasker
	ctxFields []any
}

// NewConsoleLogger creates a console logger.
func NewConsoleLogger(w io.Writer, level Level, useColor bool) *ConsoleLogger {
	return &ConsoleLogger{
		writer:   w,
		level:    level,
		useColor: useColor,
		masker:   NewSecretMasker(),
	}
}

func (l *ConsoleLogger) Debug(msg string, fields ...any) {
	if l.level > LevelDebug {
		return
	}
	l.log("DEBUG", color.FgHiBlack, msg, fields)
}

func (l *ConsoleLogger) Info(msg string, fields ...any) {
	if l.level > LevelInfo {
		return
	}
	l.log("INFO", color.FgCyan, msg, fields)
}

func (l *ConsoleLogger) Warn(msg string, fields ...any) {
	if l.level > LevelWarn {
		return
	}
	l.log("WARN", color.FgYellow, msg, fields)
}

func (l *ConsoleLogger) Error(msg string, fields ...any) {
	if l.level > LevelError {
		return
	}
	l.log("ERROR", color.FgRed, msg, fields)
}

func (l *ConsoleLogger) WithContext(ctx map[string]any) ports.Logger {
	ctxFields := make([]any, 0, len(ctx)*2)
	for k, v := range ctx {
		ctxFields = append(ctxFields, k, v)
	}
	return &ConsoleLogger{
		writer:    l.writer,
		level:     l.level,
		useColor:  l.useColor,
		masker:    l.masker,
		ctxFields: append(l.ctxFields, ctxFields...),
	}
}

func (l *ConsoleLogger) log(levelStr string, levelColor color.Attribute, msg string, fields []any) {
	timestamp := time.Now().Format("15:04:05")

	allFields := make([]any, 0, len(l.ctxFields)+len(fields))
	allFields = append(allFields, l.ctxFields...)
	allFields = append(allFields, fields...)
	masked := l.masker.MaskFields(allFields)
	fieldStr := l.formatFields(masked)

	var line string
	if l.useColor {
		c := color.New(levelColor)
		levelColored := c.Sprint(levelStr)
		line = fmt.Sprintf("%s %s %s", timestamp, levelColored, msg)
	} else {
		line = fmt.Sprintf("%s %s %s", timestamp, levelStr, msg)
	}

	if fieldStr != "" {
		line += " " + fieldStr
	}
	line += "\n"

	_, _ = fmt.Fprint(l.writer, line)
}

func (l *ConsoleLogger) formatFields(fields []any) string {
	if len(fields) == 0 {
		return ""
	}

	var parts []string
	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		value := fields[i+1]
		parts = append(parts, l.formatKV(key, value))
	}

	return strings.Join(parts, " ")
}

func (l *ConsoleLogger) formatKV(key string, value any) string {
	switch v := value.(type) {
	case string:
		if strings.Contains(v, " ") {
			return fmt.Sprintf("%s=%q", key, v)
		}
		return fmt.Sprintf("%s=%s", key, v)
	default:
		return fmt.Sprintf("%s=%v", key, v)
	}
}
