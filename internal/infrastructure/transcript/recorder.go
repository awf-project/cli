package transcript

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
)

type RecorderOption func(*Recorder)

type Recorder struct {
	path       string
	writer     *JSONLWriter
	writerOnce sync.Once
	writerErr  error
	fanout     *FanOut
	seq        atomic.Uint64
	masker     func(transcript.ExchangeEvent) transcript.ExchangeEvent
	closeOnce  sync.Once
	closeErr   error
	fanoutOpts []FanOutOption
}

func NewRecorder(path string, opts ...RecorderOption) (*Recorder, error) {
	r := &Recorder{path: path}
	for _, opt := range opts {
		opt(r)
	}
	r.fanout = NewFanOut(r.fanoutOpts...)
	r.fanoutOpts = nil
	return r, nil
}

func WithFanOutBufferSize(size int) RecorderOption {
	return func(r *Recorder) {
		r.fanoutOpts = append(r.fanoutOpts, WithBufferSize(size))
	}
}

func WithRecorderLogger(logger ports.Logger) RecorderOption {
	return func(r *Recorder) {
		r.fanoutOpts = append(r.fanoutOpts, WithLogger(logger))
	}
}

func WithMasker(masker func(transcript.ExchangeEvent) transcript.ExchangeEvent) RecorderOption {
	return func(r *Recorder) {
		r.masker = masker
	}
}

// initWriter lazily opens the transcript file on first successful Record.
// Intentionally skips MkdirAll so callers with nonexistent parent paths get a
// write-time error rather than a silent directory creation side-effect.
func (r *Recorder) initWriter() error {
	r.writerOnce.Do(func() {
		cleanPath := filepath.Clean(r.path)
		f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // path is caller-controlled
		if err != nil {
			r.writerErr = fmt.Errorf("opening transcript file: %w", err)
			return
		}
		r.writer = &JSONLWriter{path: cleanPath, f: f}
	})
	return r.writerErr
}

func (r *Recorder) Record(ctx context.Context, event transcript.ExchangeEvent) error { //nolint:gocritic // hugeParam: value semantics required per ports.Recorder contract
	if event.Type == "" {
		return ports.ErrInvalidEvent
	}

	if r.masker != nil {
		event = r.masker(event)
	}

	if err := r.initWriter(); err != nil {
		return err
	}

	if event.Seq == 0 {
		event.Seq = r.seq.Add(1)
	}

	if err := r.writer.Write(ctx, event); err != nil {
		return err
	}

	r.fanout.Publish(event)
	return nil
}

func (r *Recorder) Subscribe() (events <-chan transcript.ExchangeEvent, unsubscribe func()) {
	return r.fanout.Subscribe()
}

func (r *Recorder) Close() error {
	r.closeOnce.Do(func() {
		// Close both the fanout and the writer regardless of intermediate errors so a
		// fanout failure can never leak the writer's file descriptor; join the errors.
		var errs []error
		if err := r.fanout.Close(); err != nil {
			errs = append(errs, err)
		}
		if r.writer != nil {
			if err := r.writer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		r.closeErr = errors.Join(errs...)
	})
	return r.closeErr
}
