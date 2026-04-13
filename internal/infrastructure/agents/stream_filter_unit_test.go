package agents

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamFilterWriter_Write_HappyPath tests successful line filtering.
func TestStreamFilterWriter_Write_HappyPath(t *testing.T) {
	var result bytes.Buffer
	extractor := func(line []byte) string {
		// Simple mock: extract text after "text:"
		if bytes.Contains(line, []byte("text:")) {
			parts := bytes.Split(line, []byte("text:"))
			if len(parts) > 1 {
				return string(bytes.TrimSpace(parts[1]))
			}
		}
		return ""
	}

	writer := NewStreamFilterWriter(&result, extractor)
	_, err := writer.Write([]byte("text: hello\n"))
	require.NoError(t, err)

	err = writer.Flush()
	require.NoError(t, err)

	assert.Equal(t, "hello\n", result.String())
}

// TestStreamFilterWriter_Write_PartialLine tests buffering incomplete lines.
func TestStreamFilterWriter_Write_PartialLine(t *testing.T) {
	var result bytes.Buffer
	extractor := func(line []byte) string {
		return string(line)
	}

	writer := NewStreamFilterWriter(&result, extractor)
	_, err := writer.Write([]byte("partial"))
	require.NoError(t, err)

	assert.Equal(t, "", result.String(), "partial line should not be written yet")

	_, err = writer.Write([]byte(" line\n"))
	require.NoError(t, err)

	assert.Contains(t, result.String(), "partial line")
}

// TestStreamFilterWriter_Flush_EmitsResidual tests Flush emits buffered data.
func TestStreamFilterWriter_Flush_EmitsResidual(t *testing.T) {
	var result bytes.Buffer
	extractor := func(line []byte) string {
		return string(line)
	}

	writer := NewStreamFilterWriter(&result, extractor)
	_, err := writer.Write([]byte("no newline"))
	require.NoError(t, err)

	assert.Equal(t, "", result.String(), "partial line not yet flushed")

	err = writer.Flush()
	require.NoError(t, err)

	assert.Equal(t, "no newline\n", result.String())
}

// TestStreamFilterWriter_OversizeBuffer tests 1 MB cap enforcement.
func TestStreamFilterWriter_OversizeBuffer(t *testing.T) {
	var result bytes.Buffer
	extractor := func(line []byte) string {
		return string(line)
	}

	writer := NewStreamFilterWriter(&result, extractor)

	bigLine := make([]byte, maxLineSize+1)
	for i := range bigLine {
		bigLine[i] = 'x'
	}

	_, err := writer.Write(bigLine)
	require.NoError(t, err)

	assert.True(t, result.Len() > 0, "oversized buffer should be flushed raw")
}

// TestStreamFilterWriter_ExtractorReturnsEmpty tests silent skipping.
func TestStreamFilterWriter_ExtractorReturnsEmpty(t *testing.T) {
	var result bytes.Buffer
	extractor := func(line []byte) string {
		return ""
	}

	writer := NewStreamFilterWriter(&result, extractor)
	_, err := writer.Write([]byte("ignored line\n"))
	require.NoError(t, err)

	err = writer.Flush()
	require.NoError(t, err)

	assert.Equal(t, "", result.String(), "empty extraction should skip line")
}

// TestStreamFilterWriter_NilExtractor tests passthrough mode.
func TestStreamFilterWriter_NilExtractor(t *testing.T) {
	var result bytes.Buffer
	writer := NewStreamFilterWriter(&result, nil)

	_, err := writer.Write([]byte("raw line\n"))
	require.NoError(t, err)

	assert.Equal(t, "raw line\n", result.String())
}

// TestStreamFilterWriter_InnerWriterError tests error propagation.
func TestStreamFilterWriter_InnerWriterError(t *testing.T) {
	failWriter := &FailingWriter{}
	extractor := func(line []byte) string {
		return string(line)
	}

	writer := NewStreamFilterWriter(failWriter, extractor)
	_, err := writer.Write([]byte("test\n"))

	assert.Error(t, err)
}

// TestExtractDisplayText_WithExtractor tests text extraction from multi-line output.
func TestExtractDisplayText_WithExtractor(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		extractor LineExtractor
		want      string
	}{
		{
			name: "single line",
			raw:  `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`,
			extractor: func(line []byte) string {
				if bytes.Contains(line, []byte("text_delta")) {
					return "extracted"
				}
				return ""
			},
			want: "extracted",
		},
		{
			name: "multi-line with mixed results",
			raw: `line1
line2
line3`,
			extractor: func(line []byte) string {
				if bytes.Contains(line, []byte("2")) {
					return "found line2"
				}
				return ""
			},
			want: "found line2",
		},
		{
			name:      "all skipped",
			raw:       "line1\nline2\nline3",
			extractor: func(line []byte) string { return "" },
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDisplayText(tt.raw, tt.extractor)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestExtractDisplayText_NilExtractor returns empty string.
func TestExtractDisplayText_NilExtractor(t *testing.T) {
	result := extractDisplayText("any text", nil)
	assert.Equal(t, "", result)
}

// BenchmarkStreamFilterWriter measures the Write() overhead per NDJSON line under 4 KB
// (NFR-001, F082 T013). Observed threshold on dev machine: ~2.5 µs/op (well under the
// NFR-001 budget of 10 ms/op for agent-step execution). This benchmark is informational;
// there is no hard gate — re-run locally with `go test -bench=BenchmarkStreamFilterWriter
// ./internal/infrastructure/agents/...` after changes to stream_filter.go to catch regressions.
func BenchmarkStreamFilterWriter(b *testing.B) {
	// Construct a realistic ~4 KB Claude stream-json line.
	text := strings.Repeat("lorem ipsum dolor sit amet ", 140) // ~3780 bytes
	line := []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"` + text + `"}}` + "\n")

	provider := NewClaudeProvider()
	extractor := LineExtractor(provider.parseClaudeStreamLine)

	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		writer := NewStreamFilterWriter(io.Discard, extractor)
		if _, err := writer.Write(line); err != nil {
			b.Fatal(err)
		}
		if err := writer.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}

// FailingWriter is a test helper that always returns an error.
type FailingWriter struct{}

func (f *FailingWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}
