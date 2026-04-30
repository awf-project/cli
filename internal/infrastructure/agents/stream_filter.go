package agents

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	infralogger "github.com/awf-project/cli/internal/infrastructure/logger"
)

const maxLineSize = 10 * 1024 * 1024

var newlineBytes = []byte{'\n'}

// LineExtractor returns "" to skip a line, non-empty to emit it.
type LineExtractor func(line []byte) string

// StreamFilterWriter buffers NDJSON lines and filters them via LineExtractor.
// Lines exceeding 10 MB are dumped raw to prevent unbounded memory growth.
type StreamFilterWriter struct {
	inner   io.Writer
	extract LineExtractor
	buf     []byte
	logger  ports.Logger
}

// NewStreamFilterWriter decorates inner with line filtering. If extract is nil, data passes through unfiltered.
func NewStreamFilterWriter(inner io.Writer, extract LineExtractor, logger ...ports.Logger) *StreamFilterWriter {
	if inner == nil {
		inner = io.Discard
	}
	var l ports.Logger = infralogger.NopLogger{}
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &StreamFilterWriter{
		inner:   inner,
		extract: extract,
		buf:     make([]byte, 0, 4096),
		logger:  l,
	}
}

func (w *StreamFilterWriter) Write(p []byte) (int, error) {
	if w.extract == nil {
		n, err := w.inner.Write(p)
		if err != nil {
			return n, fmt.Errorf("write to inner: %w", err)
		}
		return n, nil
	}

	n := len(p)
	w.buf = append(w.buf, p...)

	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			if len(w.buf) > maxLineSize {
				w.logger.Warn("oversized line dumped without extraction", "size", len(w.buf), "limit", maxLineSize)
				_, err := w.inner.Write(w.buf)
				w.buf = w.buf[:0]
				if err != nil {
					return n, fmt.Errorf("write oversized buffer: %w", err)
				}
			}
			break
		}

		line := w.buf[:idx]
		if extracted := w.extract(line); extracted != "" {
			if err := w.writeExtracted(extracted); err != nil {
				return n, err
			}
		}

		w.buf = w.buf[idx+1:]
	}

	return n, nil
}

// Flush emits any buffered partial line.
func (w *StreamFilterWriter) Flush() error {
	if len(w.buf) == 0 {
		return nil
	}

	if w.extract == nil {
		w.buf = w.buf[:0]
		return nil
	}

	if extracted := w.extract(w.buf); extracted != "" {
		if err := w.writeExtracted(extracted); err != nil {
			return err
		}
	}

	w.buf = w.buf[:0]
	return nil
}

func (w *StreamFilterWriter) writeExtracted(extracted string) error {
	if _, err := io.WriteString(w.inner, extracted); err != nil {
		return fmt.Errorf("write extracted text: %w", err)
	}
	if _, err := w.inner.Write(newlineBytes); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}
	return nil
}

func extractDisplayText(raw string, extract LineExtractor) string {
	if extract == nil {
		return ""
	}

	var result strings.Builder
	for line := range strings.SplitSeq(raw, "\n") {
		if line == "" {
			continue
		}
		if extracted := extract([]byte(line)); extracted != "" {
			if result.Len() > 0 {
				result.WriteRune('\n')
			}
			result.WriteString(extracted)
		}
	}

	return result.String()
}
