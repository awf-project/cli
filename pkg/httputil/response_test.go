package httputil

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadBody_NoLimit verifies unlimited body reading (maxBodyBytes=0).
// This test covers the notify backend requirement to read entire response bodies.
func TestReadBody_NoLimit(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "empty body",
			body: "",
		},
		{
			name: "small body",
			body: "Hello, World!",
		},
		{
			name: "large body exceeding 1MB",
			body: strings.Repeat("A", 2*1024*1024), // 2MB
		},
		{
			name: "body with special characters",
			body: "Line 1\nLine 2\rLine 3\r\nJSON: {\"key\":\"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyReader := io.NopCloser(strings.NewReader(tt.body))

			got, truncated, err := ReadBody(bodyReader, 0)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.body, got, "body content mismatch")
				assert.False(t, truncated, "unlimited read should never truncate")
			}
		})
	}
}

// TestReadBody_WithLimit verifies body reading with size limit below threshold.
func TestReadBody_WithLimit(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		maxBodyBytes int64
		wantBody     string
		wantTrunc    bool
	}{
		{
			name:         "body within limit",
			body:         "Hello, World!",
			maxBodyBytes: 1024,
			wantBody:     "Hello, World!",
			wantTrunc:    false,
		},
		{
			name:         "body exactly at limit",
			body:         strings.Repeat("A", 100),
			maxBodyBytes: 100,
			wantBody:     strings.Repeat("A", 100),
			wantTrunc:    false,
		},
		{
			name:         "empty body with limit",
			body:         "",
			maxBodyBytes: 1024,
			wantBody:     "",
			wantTrunc:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyReader := io.NopCloser(strings.NewReader(tt.body))

			got, truncated, err := ReadBody(bodyReader, tt.maxBodyBytes)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, got, "body content mismatch")
			assert.Equal(t, tt.wantTrunc, truncated, "truncation flag mismatch")
		})
	}
}

// TestReadBody_Truncation verifies truncation detection when body exceeds limit.
// This covers the 1MB limit requirement for F058 http.request operation.
func TestReadBody_Truncation(t *testing.T) {
	tests := []struct {
		name         string
		bodySize     int
		maxBodyBytes int64
		wantTrunc    bool
	}{
		{
			name:         "body exceeds limit by 1 byte",
			bodySize:     101,
			maxBodyBytes: 100,
			wantTrunc:    true,
		},
		{
			name:         "body exceeds 1MB limit",
			bodySize:     1024*1024 + 1,
			maxBodyBytes: 1024 * 1024,
			wantTrunc:    true,
		},
		{
			name:         "body much larger than limit",
			bodySize:     10 * 1024 * 1024,
			maxBodyBytes: 1024 * 1024,
			wantTrunc:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.Repeat("A", tt.bodySize)
			bodyReader := io.NopCloser(strings.NewReader(body))

			got, truncated, err := ReadBody(bodyReader, tt.maxBodyBytes)
			require.NoError(t, err)
			assert.True(t, truncated, "truncation flag should be true")
			assert.Len(t, got, int(tt.maxBodyBytes), "body should be truncated to maxBodyBytes")
			assert.Equal(t, strings.Repeat("A", int(tt.maxBodyBytes)), got, "truncated body content mismatch")
		})
	}
}

// TestReadBody_EOFTolerance verifies graceful handling of EOF and ErrUnexpectedEOF.
// This matches the existing notify/responseToString behavior that tolerates partial reads.
func TestReadBody_EOFTolerance(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		readErr   error
		wantBody  string
		wantTrunc bool
		wantErr   bool
	}{
		{
			name:      "io.EOF during read",
			body:      "Partial data",
			readErr:   io.EOF,
			wantBody:  "Partial data",
			wantTrunc: false,
			wantErr:   false, // EOF is tolerated
		},
		{
			name:      "io.ErrUnexpectedEOF during read",
			body:      "Partial response",
			readErr:   io.ErrUnexpectedEOF,
			wantBody:  "Partial response",
			wantTrunc: false,
			wantErr:   false, // ErrUnexpectedEOF is tolerated
		},
		{
			name:      "other read error",
			body:      "Error case",
			readErr:   errors.New("read error"),
			wantBody:  "",
			wantTrunc: false,
			wantErr:   true, // Other errors are not tolerated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyReader := &errorReader{
				data: []byte(tt.body),
				err:  tt.readErr,
			}

			got, truncated, err := ReadBody(bodyReader, 0)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantBody, got, "body content mismatch")
				assert.Equal(t, tt.wantTrunc, truncated, "truncation flag mismatch")
			}
		})
	}
}

// TestReadBody_EdgeCases verifies boundary conditions and special cases.
func TestReadBody_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		maxBodyBytes int64
		wantBody     string
		wantTrunc    bool
	}{
		{
			name:         "negative limit treated as unlimited",
			body:         "Full content",
			maxBodyBytes: -1,
			wantBody:     "Full content",
			wantTrunc:    false,
		},
		{
			name:         "zero limit with non-empty body",
			body:         "Content",
			maxBodyBytes: 0,
			wantBody:     "Content",
			wantTrunc:    false,
		},
		{
			name:         "single byte body with 1 byte limit",
			body:         "A",
			maxBodyBytes: 1,
			wantBody:     "A",
			wantTrunc:    false,
		},
		{
			name:         "unicode characters within limit",
			body:         "Hello 世界 🌍",
			maxBodyBytes: 100,
			wantBody:     "Hello 世界 🌍",
			wantTrunc:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyReader := io.NopCloser(strings.NewReader(tt.body))

			got, truncated, err := ReadBody(bodyReader, tt.maxBodyBytes)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, got, "body content mismatch")
			assert.Equal(t, tt.wantTrunc, truncated, "truncation flag mismatch")
		})
	}
}

// errorReader is a test helper that simulates read errors.
type errorReader struct {
	data []byte
	err  error
	pos  int
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

func (r *errorReader) Close() error {
	return nil
}
