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

// DisplayEventRenderer receives parsed DisplayEvents for rendering to output.
type DisplayEventRenderer func(events []DisplayEvent)

// StreamFilterWriter buffers NDJSON lines and filters them.
// Lines exceeding 10 MB are dumped raw to prevent unbounded memory growth.
type StreamFilterWriter struct {
	inner    io.Writer
	extract  func(line []byte) string
	parser   DisplayEventParser
	renderer DisplayEventRenderer
	buf      []byte
	logger   ports.Logger
}

func newStreamFilterBase(inner io.Writer, logger []ports.Logger) *StreamFilterWriter {
	if inner == nil {
		inner = io.Discard
	}
	var l ports.Logger = infralogger.NopLogger{}
	if len(logger) > 0 && logger[0] != nil {
		l = logger[0]
	}
	return &StreamFilterWriter{inner: inner, buf: make([]byte, 0, 4096), logger: l}
}

// NewStreamFilterWriterWithParser decorates inner with event-aware filtering.
// Parsed events are forwarded to renderer; if renderer is nil events are discarded.
func NewStreamFilterWriterWithParser(inner io.Writer, parser DisplayEventParser, renderer DisplayEventRenderer, logger ...ports.Logger) *StreamFilterWriter {
	w := newStreamFilterBase(inner, logger)
	w.parser = parser
	w.renderer = renderer
	return w
}

// NewStreamFilterWriter decorates inner with line filtering. If extract is nil, data passes through unfiltered.
func NewStreamFilterWriter(inner io.Writer, extract func(line []byte) string, logger ...ports.Logger) *StreamFilterWriter {
	w := newStreamFilterBase(inner, logger)
	w.extract = extract
	return w
}

func (w *StreamFilterWriter) Write(p []byte) (int, error) {
	if w.extract == nil && w.parser == nil {
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

		if err := w.processLine(w.buf[:idx]); err != nil {
			return n, err
		}
		w.buf = w.buf[idx+1:]
	}

	return n, nil
}

func (w *StreamFilterWriter) processLine(line []byte) error {
	if w.parser != nil {
		return w.parseAndRenderLine(line)
	}
	if extracted := w.extract(line); extracted != "" {
		return w.writeExtracted(extracted)
	}
	return nil
}

func (w *StreamFilterWriter) parseAndRenderLine(line []byte) error {
	if len(line) >= maxLineSize {
		w.logger.Warn("oversized line dumped without extraction", "size", len(line), "limit", maxLineSize)
		if _, err := w.inner.Write(line); err != nil {
			return fmt.Errorf("write oversized buffer: %w", err)
		}
		if _, err := w.inner.Write(newlineBytes); err != nil {
			return fmt.Errorf("write newline: %w", err)
		}
		return nil
	}
	events := w.parser(line)
	if len(events) == 0 {
		return nil
	}
	if w.renderer != nil {
		w.renderer(events)
	}
	return w.writeTextEvents(events)
}

func (w *StreamFilterWriter) writeTextEvents(events []DisplayEvent) error {
	for _, event := range events {
		if event.Kind == EventText && event.Text != "" {
			if event.Delta {
				if _, err := io.WriteString(w.inner, event.Text); err != nil {
					return fmt.Errorf("write delta text: %w", err)
				}
			} else {
				if err := w.writeExtracted(event.Text); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Flush emits any buffered partial line.
func (w *StreamFilterWriter) Flush() error {
	if len(w.buf) == 0 {
		return nil
	}

	if w.parser != nil {
		events := w.parser(w.buf)
		if len(events) > 0 {
			if w.renderer != nil {
				w.renderer(events)
			}
			if err := w.writeTextEvents(events); err != nil {
				return err
			}
		}
		w.buf = w.buf[:0]
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

func extractDisplayTextFromEvents(raw string, parser DisplayEventParser) string {
	if parser == nil {
		return ""
	}

	var result strings.Builder
	for line := range strings.SplitSeq(raw, "\n") {
		if line == "" {
			continue
		}
		events := parser([]byte(line))
		for _, evt := range events {
			if evt.Kind != EventText {
				continue
			}
			if result.Len() > 0 {
				result.WriteRune('\n')
			}
			result.WriteString(evt.Text)
		}
	}

	return result.String()
}
