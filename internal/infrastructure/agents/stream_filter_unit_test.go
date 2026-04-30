package agents

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamFilterWriter_10MBBoundary(t *testing.T) {
	const (
		oneMB          = 1024 * 1024
		tenMB          = 10 * oneMB
		justUnderTenMB = tenMB - 1
		justOverTenMB  = tenMB + 1
	)

	tests := []struct {
		name        string
		writes      [][]byte
		extract     LineExtractor
		wantWritten string
		needsFlush  bool
	}{
		{
			name:        "line at exactly 10 MB boundary without newline - buffered not dumped",
			writes:      [][]byte{bytes.Repeat([]byte("a"), tenMB)},
			extract:     func(line []byte) string { return string(line) },
			wantWritten: "",
		},
		{
			name:        "line exceeding 10 MB boundary without newline - dumped without extraction",
			writes:      [][]byte{bytes.Repeat([]byte("a"), justOverTenMB)},
			extract:     func(line []byte) string { return "EXTRACTED" },
			wantWritten: string(bytes.Repeat([]byte("a"), justOverTenMB)),
		},
		{
			name:        "line just under 10 MB without newline - buffered",
			writes:      [][]byte{bytes.Repeat([]byte("x"), justUnderTenMB)},
			extract:     func(line []byte) string { return string(line) },
			wantWritten: "",
		},
		{
			name:        "line at 10 MB with newline - extracted and written",
			writes:      [][]byte{bytes.Repeat([]byte("b"), tenMB-1), []byte("\n")},
			extract:     func(line []byte) string { return "FILTERED" },
			wantWritten: "FILTERED\n",
		},
		{
			name:        "line exceeding 10 MB with late newline - dumped raw then extracts remainder",
			writes:      [][]byte{bytes.Repeat([]byte("c"), justOverTenMB), []byte("\nREST")},
			extract:     func(line []byte) string { return "EXTRACTED" },
			wantWritten: string(bytes.Repeat([]byte("c"), justOverTenMB)) + "EXTRACTED\n",
		},
		{
			name: "accumulated writes crossing 10 MB boundary with newline",
			writes: [][]byte{
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				bytes.Repeat([]byte("d"), oneMB),
				[]byte("\n"),
			},
			extract:     func(line []byte) string { return "ACCUMULATED" },
			wantWritten: "ACCUMULATED\n",
		},
		{
			name:        "write that triggers dump followed by normal extraction",
			writes:      [][]byte{bytes.Repeat([]byte("e"), justOverTenMB), []byte("small"), []byte("\n")},
			extract:     func(line []byte) string { return "SMALL" },
			wantWritten: string(bytes.Repeat([]byte("e"), justOverTenMB)) + "SMALL\n",
		},
		{
			name:        "empty extract result filtered out",
			writes:      [][]byte{[]byte("ignored"), []byte("\n")},
			extract:     func(line []byte) string { return "" },
			wantWritten: "",
		},
		{
			name:        "nil extract parameter - passthrough",
			writes:      [][]byte{[]byte("passthrough data\n"), []byte("more data\n")},
			extract:     nil,
			wantWritten: "passthrough data\nmore data\n",
		},
		{
			name:        "flush after partial line at 10 MB",
			writes:      [][]byte{bytes.Repeat([]byte("f"), tenMB)},
			extract:     func(line []byte) string { return "FLUSHED" },
			wantWritten: "FLUSHED\n",
			needsFlush:  true,
		},
		{
			name: "multiple lines crossing 10 MB threshold",
			writes: [][]byte{
				bytes.Repeat([]byte("g"), 5*oneMB), []byte("\n"),
				bytes.Repeat([]byte("g"), 4*oneMB), []byte("\n"),
				bytes.Repeat([]byte("g"), 2*oneMB), []byte("\n"),
			},
			extract:     func(line []byte) string { return "LINE" },
			wantWritten: "LINE\nLINE\nLINE\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			w := NewStreamFilterWriter(&out, tt.extract)

			for _, writeData := range tt.writes {
				_, err := w.Write(writeData)
				require.NoError(t, err)
			}

			if tt.needsFlush {
				require.NoError(t, w.Flush())
			}

			assert.Equal(t, tt.wantWritten, out.String(), tt.name)
		})
	}
}

func TestStreamFilterWriter_WriteReturnValue(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		extract       LineExtractor
		wantBytesRead int
	}{
		{
			name:          "write returns all bytes read",
			input:         []byte("test input"),
			extract:       func(line []byte) string { return string(line) },
			wantBytesRead: 10,
		},
		{
			name:          "large write returns correct count",
			input:         bytes.Repeat([]byte("x"), 1000),
			extract:       func(line []byte) string { return string(line) },
			wantBytesRead: 1000,
		},
		{
			name:          "oversized write returns bytes written",
			input:         bytes.Repeat([]byte("x"), 10*1024*1024+1),
			extract:       func(line []byte) string { return "filtered" },
			wantBytesRead: 10*1024*1024 + 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			w := NewStreamFilterWriter(&out, tt.extract)

			n, err := w.Write(tt.input)

			require.NoError(t, err)
			assert.Equal(t, tt.wantBytesRead, n)
		})
	}
}

func BenchmarkStreamFilterWriter_10MBBoundary(b *testing.B) {
	const tenMB = 10 * 1024 * 1024

	extract := func(line []byte) string {
		return string(line)
	}

	oneMBData := bytes.Repeat([]byte("a"), tenMB/10)
	newlineData := []byte("\n")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		w := NewStreamFilterWriter(&out, extract)

		for j := 0; j < 10; j++ {
			_, _ = w.Write(oneMBData)
		}
		_, _ = w.Write(newlineData)

		w.Flush()
	}
}

func BenchmarkStreamFilterWriter_LargeLines(b *testing.B) {
	const (
		tenMB         = 10 * 1024 * 1024
		justOverTenMB = tenMB + 1024
	)

	largeLineData := bytes.Repeat([]byte("x"), justOverTenMB)
	smallLineData := []byte("small\n")
	totalBytes := int64(justOverTenMB + len(smallLineData))

	// Validate 10MB boundary enforcement: extract must never receive a line exceeding 10MB.
	extract := func(line []byte) string {
		if len(line) > tenMB {
			b.Errorf("extract called with line exceeding 10MB cap: got %d bytes", len(line))
		}
		return "filtered"
	}

	b.SetBytes(totalBytes) // enables MB/s throughput reporting
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		w := NewStreamFilterWriter(&out, extract)

		_, _ = w.Write(largeLineData)
		_, _ = w.Write(smallLineData)

		w.Flush()
	}
}
