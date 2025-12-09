package ui_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vanoix/awf/internal/interfaces/cli/ui"
)

func TestPrefixedWriter_StdoutPrefix(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	w.Write([]byte("hello\n"))

	assert.Equal(t, "[OUT] hello\n", buf.String())
}

func TestPrefixedWriter_StderrPrefix(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStderr, "", colorizer)
	w.Write([]byte("error\n"))

	assert.Equal(t, "[ERR] error\n", buf.String())
}

func TestPrefixedWriter_MultiLine(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	w.Write([]byte("line1\nline2\nline3\n"))

	expected := "[OUT] line1\n[OUT] line2\n[OUT] line3\n"
	assert.Equal(t, expected, buf.String())
}

func TestPrefixedWriter_PartialLine(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	w.Write([]byte("partial"))
	w.Write([]byte(" line\n"))

	assert.Equal(t, "[OUT] partial line\n", buf.String())
}

func TestPrefixedWriter_WithStepName(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "build", colorizer)
	w.Write([]byte("compiling\n"))

	assert.Equal(t, "[build|OUT] compiling\n", buf.String())
}

func TestPrefixedWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	w.Write([]byte("no newline"))
	w.Flush()

	assert.Equal(t, "[OUT] no newline\n", buf.String())
}

func TestPrefixedWriter_EmptyWrite(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	n, err := w.Write([]byte(""))

	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, buf.String())
}

func TestPrefixedWriter_ReturnsCorrectLength(t *testing.T) {
	var buf bytes.Buffer
	colorizer := ui.NewColorizer(false)

	w := ui.NewPrefixedWriter(&buf, ui.PrefixStdout, "", colorizer)
	input := []byte("hello world\n")
	n, err := w.Write(input)

	assert.NoError(t, err)
	assert.Equal(t, len(input), n)
}
