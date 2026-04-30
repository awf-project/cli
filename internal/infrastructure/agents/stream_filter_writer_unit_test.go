package agents

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct {
	warnings []mockLogEntry
}

type mockLogEntry struct {
	msg  string
	args []any
}

func (m *mockLogger) Warn(msg string, args ...any) {
	m.warnings = append(m.warnings, mockLogEntry{msg: msg, args: args})
}

func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Debug(msg string, args ...any) {}
func (m *mockLogger) WithContext(ctx map[string]any) ports.Logger {
	return m
}

type failingWriter struct {
	failAfter int
	written   int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.written++
	if f.written > f.failAfter {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

// TestNewStreamFilterWriter_WithLogger verifies logger field is set when provided
func TestNewStreamFilterWriter_WithLogger(t *testing.T) {
	mockLog := &mockLogger{}
	buf := &bytes.Buffer{}

	w := NewStreamFilterWriter(buf, nil, mockLog)

	require.NotNil(t, w)
	assert.Equal(t, mockLog, w.logger)
}

// TestNewStreamFilterWriter_WithoutLogger verifies variadic param allows no logger and defaults to NopLogger
func TestNewStreamFilterWriter_WithoutLogger(t *testing.T) {
	buf := &bytes.Buffer{}

	w := NewStreamFilterWriter(buf, nil)

	require.NotNil(t, w)
	assert.NotNil(t, w.logger)
}

// TestNewStreamFilterWriter_NilInner verifies nil inner writer is handled gracefully
func TestNewStreamFilterWriter_NilInner(t *testing.T) {
	w := NewStreamFilterWriter(nil, nil)

	require.NotNil(t, w)
	assert.Equal(t, io.Discard, w.inner)
}

// TestWrite_FilteredLine verifies lines are extracted and written when extractor returns text
func TestWrite_FilteredLine(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		if bytes.Contains(line, []byte("extract")) {
			return "extracted: " + string(line)
		}
		return ""
	}

	w := NewStreamFilterWriter(buf, extractor)
	input := []byte("extract this\n")

	n, err := w.Write(input)

	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Equal(t, "extracted: extract this\n", buf.String())
}

// TestWrite_SkippedLine verifies lines are skipped when extractor returns empty string
func TestWrite_SkippedLine(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return "" // skip all lines
	}

	w := NewStreamFilterWriter(buf, extractor)
	input := []byte("skip me\n")

	n, err := w.Write(input)

	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Empty(t, buf.String())
}

// TestWrite_MultipleLines verifies multiple lines in single write are all processed
func TestWrite_MultipleLines(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return string(line) + "-processed"
	}

	w := NewStreamFilterWriter(buf, extractor)
	input := []byte("line1\nline2\nline3\n")

	n, err := w.Write(input)

	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Equal(t, "line1-processed\nline2-processed\nline3-processed\n", buf.String())
}

// TestWrite_PartialLine verifies incomplete lines are buffered
func TestWrite_PartialLine(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return string(line)
	}

	w := NewStreamFilterWriter(buf, extractor)

	n1, err1 := w.Write([]byte("partial"))
	require.NoError(t, err1)
	assert.Equal(t, 7, n1)
	assert.Empty(t, buf.String()) // not written yet

	n2, err2 := w.Write([]byte(" line\n"))
	require.NoError(t, err2)
	assert.Equal(t, 6, n2)
	assert.Equal(t, "partial line\n", buf.String())
}

// TestWrite_OversizedLineLogsWarning verifies structured warning when line exceeds 10 MB
func TestWrite_OversizedLineLogsWarning(t *testing.T) {
	mockLog := &mockLogger{}
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return ""
	}

	w := NewStreamFilterWriter(buf, extractor, mockLog)

	oversizedData := make([]byte, maxLineSize+1)

	n, err := w.Write(oversizedData)

	require.NoError(t, err)
	assert.Equal(t, len(oversizedData), n)
	require.Len(t, mockLog.warnings, 1)
	assert.Equal(t, "oversized line dumped without extraction", mockLog.warnings[0].msg)

	args := mockLog.warnings[0].args
	assert.Contains(t, args, "size")
	assert.Contains(t, args, maxLineSize+1)
	assert.Contains(t, args, "limit")
	assert.Contains(t, args, maxLineSize)
}

// TestWrite_OversizedLineDumped verifies oversized line is written unfiltered
func TestWrite_OversizedLineDumped(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return "extracted"
	}

	w := NewStreamFilterWriter(buf, extractor)

	oversizedData := make([]byte, maxLineSize+1)
	copy(oversizedData, bytes.Repeat([]byte("x"), maxLineSize+1))

	n, err := w.Write(oversizedData)

	require.NoError(t, err)
	assert.Equal(t, len(oversizedData), n)
	assert.Equal(t, string(oversizedData), buf.String())
}

// TestWrite_InnerWriteError verifies errors from inner writer are wrapped and returned
func TestWrite_InnerWriteError(t *testing.T) {
	failing := &failingWriter{failAfter: 0}
	extractor := func(line []byte) string {
		return string(line)
	}

	w := NewStreamFilterWriter(failing, extractor)
	input := []byte("test\n")

	n, err := w.Write(input)

	assert.Error(t, err)
	assert.Equal(t, 5, n)
	assert.Contains(t, err.Error(), "write extracted text")
}

// TestWrite_NoExtractor verifies nil extractor passes through unfiltered
func TestWrite_NoExtractor(t *testing.T) {
	buf := &bytes.Buffer{}

	w := NewStreamFilterWriter(buf, nil)
	input := []byte("pass through\n")

	n, err := w.Write(input)

	require.NoError(t, err)
	assert.Equal(t, len(input), n)
	assert.Equal(t, string(input), buf.String())
}

// TestWrite_NoExtractorInnerError verifies errors bubble up when no extractor
func TestWrite_NoExtractorInnerError(t *testing.T) {
	failing := &failingWriter{failAfter: 0}

	w := NewStreamFilterWriter(failing, nil)
	input := []byte("test\n")

	n, err := w.Write(input)

	assert.Error(t, err)
	assert.Equal(t, 0, n)
	assert.Contains(t, err.Error(), "write to inner")
}

// TestFlush_BufferedPartialLine verifies Flush writes buffered partial line
func TestFlush_BufferedPartialLine(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return "extracted: " + string(line)
	}

	w := NewStreamFilterWriter(buf, extractor)

	w.Write([]byte("partial"))

	err := w.Flush()

	require.NoError(t, err)
	assert.Equal(t, "extracted: partial\n", buf.String())
}

// TestFlush_EmptyBuffer verifies Flush returns nil when no buffered data
func TestFlush_EmptyBuffer(t *testing.T) {
	buf := &bytes.Buffer{}

	w := NewStreamFilterWriter(buf, nil)

	err := w.Flush()

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// TestFlush_SkipsEmpty verifies Flush skips lines that extract to empty string
func TestFlush_SkipsEmpty(t *testing.T) {
	buf := &bytes.Buffer{}
	extractor := func(line []byte) string {
		return ""
	}

	w := NewStreamFilterWriter(buf, extractor)

	w.Write([]byte("skip this"))

	err := w.Flush()

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// TestFlush_WriteError verifies errors from inner writer are wrapped
func TestFlush_WriteError(t *testing.T) {
	failing := &failingWriter{failAfter: 0}
	extractor := func(line []byte) string {
		return "text"
	}

	w := NewStreamFilterWriter(failing, extractor)

	w.Write([]byte("partial"))

	err := w.Flush()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write extracted text")
}

// TestExtractDisplayText_WithExtractor_Basic verifies basic filtered text is returned
func TestExtractDisplayText_WithExtractor_Basic(t *testing.T) {
	extractor := func(line []byte) string {
		if bytes.HasPrefix(line, []byte("keep")) {
			return string(line)
		}
		return ""
	}

	input := "keep this\nskip that\nkeep also"
	result := extractDisplayText(input, extractor)

	assert.Equal(t, "keep this\nkeep also", result)
}

// TestExtractDisplayText_NoExtractor verifies empty string when extractor is nil
func TestExtractDisplayText_NoExtractor(t *testing.T) {
	input := "line1\nline2"
	result := extractDisplayText(input, nil)

	assert.Empty(t, result)
}

// TestExtractDisplayText_AllFiltered verifies empty string when all lines filtered
func TestExtractDisplayText_AllFiltered(t *testing.T) {
	extractor := func(line []byte) string {
		return ""
	}

	input := "line1\nline2"
	result := extractDisplayText(input, extractor)

	assert.Empty(t, result)
}

// TestExtractDisplayText_EmptyLines verifies empty lines are skipped
func TestExtractDisplayText_EmptyLines(t *testing.T) {
	extractor := func(line []byte) string {
		return string(line)
	}

	input := "line1\n\n\nline2"
	result := extractDisplayText(input, extractor)

	assert.Equal(t, "line1\nline2", result)
}
