package ui

import (
	"bytes"
	"io"
	"sync"
)

// PrefixType indicates stdout or stderr prefixing.
type PrefixType int

const (
	PrefixStdout PrefixType = iota
	PrefixStderr
)

// PrefixedWriter wraps an io.Writer and prefixes each line.
type PrefixedWriter struct {
	out       io.Writer
	prefix    PrefixType
	stepName  string
	colorizer *Colorizer
	mu        sync.Mutex
	lineBuf   bytes.Buffer
}

// NewPrefixedWriter creates a new prefixed writer.
func NewPrefixedWriter(out io.Writer, prefix PrefixType, stepName string, colorizer *Colorizer) *PrefixedWriter {
	return &PrefixedWriter{
		out:       out,
		prefix:    prefix,
		stepName:  stepName,
		colorizer: colorizer,
	}
}

// Write implements io.Writer with line prefixing.
func (w *PrefixedWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	n = len(p)
	w.lineBuf.Write(p)

	for {
		line, err := w.lineBuf.ReadBytes('\n')
		if err != nil {
			// incomplete line, put back
			w.lineBuf.Write(line)
			break
		}
		w.writePrefixedLine(line)
	}

	return n, nil
}

// Flush writes any remaining buffered content.
func (w *PrefixedWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.lineBuf.Len() > 0 {
		line := w.lineBuf.Bytes()
		w.lineBuf.Reset()
		w.writePrefixedLine(append(line, '\n'))
	}
}

func (w *PrefixedWriter) writePrefixedLine(line []byte) {
	prefix := w.buildPrefix()
	w.out.Write([]byte(prefix))
	w.out.Write(line)
}

func (w *PrefixedWriter) buildPrefix() string {
	var tag string
	if w.prefix == PrefixStdout {
		tag = "OUT"
	} else {
		tag = "ERR"
	}

	var prefix string
	if w.stepName != "" {
		prefix = "[" + w.stepName + "|" + tag + "] "
	} else {
		prefix = "[" + tag + "] "
	}

	if w.colorizer != nil && w.colorizer.enabled {
		if w.prefix == PrefixStdout {
			return w.colorizer.Info(prefix)
		}
		return w.colorizer.Error(prefix)
	}

	return prefix
}
