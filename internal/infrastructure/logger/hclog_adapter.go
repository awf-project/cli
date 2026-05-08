package logger

import (
	"io"
	"log"
	"strings"

	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ hclog.Logger = (*HCLogAdapter)(nil)

// HCLogAdapter bridges hclog.Logger to zap.Logger for go-plugin host-side logging.
type HCLogAdapter struct {
	logger      *zap.Logger
	name        string
	impliedArgs []interface{}
	masker      *SecretMasker
}

// NewHCLogAdapter creates an hclog.Logger that forwards to the given zap.Logger.
func NewHCLogAdapter(zapLogger *zap.Logger, name string) *HCLogAdapter {
	return &HCLogAdapter{
		logger: zapLogger,
		name:   name,
		masker: NewSecretMasker(),
	}
}

func (a *HCLogAdapter) log(level zapcore.Level, msg string, args ...interface{}) {
	ce := a.logger.Check(level, msg)
	if ce == nil {
		return
	}
	combined := append(a.impliedArgs, args...) //nolint:gocritic // intentional: building combined slice for field extraction
	masked := a.masker.MaskFields(combined)
	fields := hcArgsToZapFields(masked)
	ce.Write(fields...)
}

func hcArgsToZapFields(args []interface{}) []zap.Field {
	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, args[i+1]))
	}
	return fields
}

func hcLevelToZap(level hclog.Level) zapcore.Level {
	switch level {
	case hclog.Trace, hclog.Debug:
		return zapcore.DebugLevel
	case hclog.Info:
		return zapcore.InfoLevel
	case hclog.Warn:
		return zapcore.WarnLevel
	case hclog.Error:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func (a *HCLogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	a.log(hcLevelToZap(level), msg, args...)
}

func (a *HCLogAdapter) Trace(msg string, args ...interface{}) {
	a.log(zapcore.DebugLevel, msg, args...)
}

func (a *HCLogAdapter) Debug(msg string, args ...interface{}) {
	a.log(zapcore.DebugLevel, msg, args...)
}

func (a *HCLogAdapter) Info(msg string, args ...interface{}) {
	a.log(zapcore.InfoLevel, msg, args...)
}

func (a *HCLogAdapter) Warn(msg string, args ...interface{}) {
	a.log(zapcore.WarnLevel, msg, args...)
}

func (a *HCLogAdapter) Error(msg string, args ...interface{}) {
	a.log(zapcore.ErrorLevel, msg, args...)
}

func (a *HCLogAdapter) IsTrace() bool {
	return a.logger.Core().Enabled(zapcore.DebugLevel)
}

func (a *HCLogAdapter) IsDebug() bool {
	return a.logger.Core().Enabled(zapcore.DebugLevel)
}

func (a *HCLogAdapter) IsInfo() bool {
	return a.logger.Core().Enabled(zapcore.InfoLevel)
}

func (a *HCLogAdapter) IsWarn() bool {
	return a.logger.Core().Enabled(zapcore.WarnLevel)
}

func (a *HCLogAdapter) IsError() bool {
	return a.logger.Core().Enabled(zapcore.ErrorLevel)
}

func (a *HCLogAdapter) ImpliedArgs() []interface{} { return a.impliedArgs }

func (a *HCLogAdapter) With(args ...interface{}) hclog.Logger {
	combined := make([]interface{}, 0, len(a.impliedArgs)+len(args))
	combined = append(combined, a.impliedArgs...)
	combined = append(combined, args...)
	return &HCLogAdapter{
		logger:      a.logger,
		name:        a.name,
		impliedArgs: combined,
		masker:      a.masker,
	}
}

func (a *HCLogAdapter) Name() string { return a.name }

func (a *HCLogAdapter) Named(name string) hclog.Logger {
	newName := name
	if a.name != "" {
		newName = a.name + "." + name
	}
	return &HCLogAdapter{
		logger:      a.logger,
		name:        newName,
		impliedArgs: a.impliedArgs,
		masker:      a.masker,
	}
}

func (a *HCLogAdapter) ResetNamed(name string) hclog.Logger {
	return &HCLogAdapter{
		logger:      a.logger,
		name:        name,
		impliedArgs: a.impliedArgs,
		masker:      a.masker,
	}
}

func (a *HCLogAdapter) SetLevel(_ hclog.Level) {}

func (a *HCLogAdapter) GetLevel() hclog.Level {
	core := a.logger.Core()
	switch {
	case core.Enabled(zapcore.DebugLevel):
		return hclog.Debug
	case core.Enabled(zapcore.InfoLevel):
		return hclog.Info
	case core.Enabled(zapcore.WarnLevel):
		return hclog.Warn
	case core.Enabled(zapcore.ErrorLevel):
		return hclog.Error
	default:
		return hclog.Off
	}
}

func (a *HCLogAdapter) StandardLogger(_ *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(a.StandardWriter(nil), "", 0)
}

func (a *HCLogAdapter) StandardWriter(_ *hclog.StandardLoggerOptions) io.Writer {
	return NewLogWriter(a.logger, zapcore.InfoLevel)
}

// logWriter implements io.Writer by logging each line at the specified zap level.
// Used for SyncStdout/SyncStderr capture of plugin subprocess output.
type logWriter struct {
	logger *zap.Logger
	level  zapcore.Level
}

// NewLogWriter creates an io.Writer that logs each line at the given zap level.
func NewLogWriter(zapLogger *zap.Logger, level zapcore.Level) io.Writer {
	return &logWriter{logger: zapLogger, level: level}
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	lines := strings.Split(string(p), "\n")
	// Trim trailing empty element produced by a trailing newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for _, line := range lines {
		if ce := w.logger.Check(w.level, line); ce != nil {
			ce.Write()
		}
	}

	return len(p), nil
}
