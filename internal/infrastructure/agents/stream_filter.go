package agents

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

const maxLineSize = 1024 * 1024

var newlineBytes = []byte{'\n'}

// LineExtractor parses a single NDJSON line and returns extracted text.
// Returning "" indicates the line should be skipped.
type LineExtractor func(line []byte) string

// StreamFilterWriter is an io.Writer decorator that buffers NDJSON lines,
// extracts text via LineExtractor, and writes filtered content to an inner writer.
// It enforces a 1 MB cap per line to prevent unbounded memory growth.
type StreamFilterWriter struct {
	inner   io.Writer
	extract LineExtractor
	buf     []byte
}

// NewStreamFilterWriter creates a new StreamFilterWriter that decorates the given writer.
// If extract is nil, lines are passed through unfiltered.
func NewStreamFilterWriter(inner io.Writer, extract LineExtractor) *StreamFilterWriter {
	if inner == nil {
		inner = io.Discard
	}
	return &StreamFilterWriter{
		inner:   inner,
		extract: extract,
		buf:     make([]byte, 0, 4096),
	}
}

// Write implements io.Writer. It buffers incoming data until a newline is encountered,
// then parses and filters the complete line.
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
			if _, err := io.WriteString(w.inner, extracted); err != nil {
				return n, fmt.Errorf("write extracted text: %w", err)
			}
			if _, err := w.inner.Write(newlineBytes); err != nil {
				return n, fmt.Errorf("write newline: %w", err)
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

	if extracted := w.extract(w.buf); extracted != "" {
		if _, err := io.WriteString(w.inner, extracted); err != nil {
			return fmt.Errorf("flush write extracted: %w", err)
		}
		if _, err := w.inner.Write(newlineBytes); err != nil {
			return fmt.Errorf("flush write newline: %w", err)
		}
	}

	w.buf = w.buf[:0]
	return nil
}

// extractDisplayText applies the provided LineExtractor to each line of raw output
// and returns the concatenated filtered text. Returns empty string if extract is nil.
func extractDisplayText(raw string, extract LineExtractor) string {
	if extract == nil {
		return ""
	}

	var result strings.Builder
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
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
