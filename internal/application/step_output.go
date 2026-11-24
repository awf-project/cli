package application

import (
	"context"
	"io"

	"github.com/awf-project/cli/internal/domain/ports"
)

// stepOutputSink receives a live output chunk of a running step. It is invoked
// synchronously from the step's output writer, so implementations must not block.
type stepOutputSink func(stepName, stream, text string)

// stepOutputCtxKey scopes a per-run stepOutputSink onto a context, mirroring how
// recorderCtxKey scopes a per-run Recorder. The sink travels with the run's context
// rather than living on the shared ExecutionService, which keeps concurrent runs
// (TUI, HTTP) from seeing each other's output.
type stepOutputCtxKey struct{}

// withStepOutputSink returns a context that routes live step output to sink.
// A nil sink returns ctx unchanged so callers can wrap unconditionally.
func withStepOutputSink(ctx context.Context, sink stepOutputSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, stepOutputCtxKey{}, sink)
}

func stepOutputSinkFrom(ctx context.Context) stepOutputSink {
	sink, _ := ctx.Value(stepOutputCtxKey{}).(stepOutputSink)
	return sink
}

// stepOutputWriter forwards every write to the configured destination writer (if any)
// and reports the same bytes to the sink. The destination always sees the full write
// first, so enabling live output never changes what the CLI or log writers receive.
type stepOutputWriter struct {
	dst      io.Writer
	sink     stepOutputSink
	stepName string
	stream   string
}

func (w *stepOutputWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w.dst != nil {
		var err error
		n, err = w.dst.Write(p)
		if n > 0 {
			w.sink(w.stepName, w.stream, string(p[:n]))
		}
		return n, err //nolint:wrapcheck // transparent pass-through of the destination writer's result
	}
	if n > 0 {
		w.sink(w.stepName, w.stream, string(p))
	}
	return n, nil
}

// stepWriters returns the stdout/stderr writers to hand to a step. Without a sink on
// ctx it returns the service writers untouched (zero overhead, identical behavior);
// with one, each writer tees its output to the sink as a ports.StepOutput* stream.
func (s *ExecutionService) stepWriters(ctx context.Context, stepName string) (stdout, stderr io.Writer) {
	sink := stepOutputSinkFrom(ctx)
	if sink == nil {
		return s.stdoutWriter, s.stderrWriter
	}
	return &stepOutputWriter{dst: s.stdoutWriter, sink: sink, stepName: stepName, stream: ports.StepOutputStdout},
		&stepOutputWriter{dst: s.stderrWriter, sink: sink, stepName: stepName, stream: ports.StepOutputStderr}
}
