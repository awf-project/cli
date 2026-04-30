//go:build integration

package features_test

// Feature: F084

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/agents"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamFilter_RealisticNDJSONStream_Integration(t *testing.T) {
	extract := func(line []byte) string {
		s := string(line)
		if strings.Contains(s, `"type":"content"`) {
			idx := strings.Index(s, `"text":"`)
			if idx < 0 {
				return ""
			}
			start := idx + len(`"text":"`)
			end := strings.Index(s[start:], `"`)
			if end < 0 {
				return ""
			}
			return s[start : start+end]
		}
		return ""
	}

	stream := strings.Join([]string{
		`{"type":"system","id":"abc123"}`,
		`{"type":"content","text":"Hello"}`,
		`{"type":"content","text":"World"}`,
		`{"type":"system","id":"done"}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	w := agents.NewStreamFilterWriter(&out, extract)

	data := []byte(stream)
	chunkSize := 37
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		n, err := w.Write(data[i:end])
		require.NoError(t, err)
		assert.Equal(t, end-i, n)
	}

	require.NoError(t, w.Flush())
	assert.Equal(t, "Hello\nWorld\n", out.String())
}

func TestStreamFilter_LargeLineWithinCap_Integration(t *testing.T) {
	const nineMB = 9 * 1024 * 1024

	payload := strings.Repeat("x", nineMB-50)
	line := fmt.Sprintf(`{"type":"content","text":"%s"}`, payload)

	extract := func(l []byte) string { return "EXTRACTED" }

	log := mocks.NewMockLogger()
	var out bytes.Buffer
	w := agents.NewStreamFilterWriter(&out, extract, log)

	_, err := w.Write(append([]byte(line), '\n'))
	require.NoError(t, err)

	assert.Equal(t, "EXTRACTED\n", out.String())
	assert.Empty(t, log.GetMessagesByLevel("WARN"))
}

func TestStreamFilter_OversizedLineThenContinuation_Integration(t *testing.T) {
	const tenMB = 10 * 1024 * 1024

	extract := func(line []byte) string {
		if len(line) < 1000 {
			return "normal:" + string(line)
		}
		return "large"
	}

	log := mocks.NewMockLogger()
	var out bytes.Buffer
	w := agents.NewStreamFilterWriter(&out, extract, log)

	oversized := bytes.Repeat([]byte("O"), tenMB+1024)
	n, err := w.Write(oversized)
	require.NoError(t, err)
	assert.Equal(t, len(oversized), n)

	normalLines := []string{"line-one", "line-two", "line-three"}
	for _, line := range normalLines {
		_, err := w.Write([]byte(line + "\n"))
		require.NoError(t, err)
	}

	output := out.String()
	for _, line := range normalLines {
		assert.Contains(t, output, "normal:"+line)
	}

	warnings := log.GetMessagesByLevel("WARN")
	assert.Len(t, warnings, 1)
}

func TestStreamFilter_OversizedLineWarning_Integration(t *testing.T) {
	const tenMB = 10 * 1024 * 1024

	extract := func(line []byte) string { return "filtered" }

	log := mocks.NewMockLogger()
	var out bytes.Buffer
	w := agents.NewStreamFilterWriter(&out, extract, log)

	oversized := bytes.Repeat([]byte("W"), tenMB+512)
	_, err := w.Write(oversized)
	require.NoError(t, err)

	warnings := log.GetMessagesByLevel("WARN")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Msg, "oversized")

	hasSize := false
	hasLimit := false
	for i := 0; i < len(warnings[0].Fields)-1; i += 2 {
		key, ok := warnings[0].Fields[i].(string)
		if !ok {
			continue
		}
		if key == "size" {
			hasSize = true
		}
		if key == "limit" {
			hasLimit = true
		}
	}
	assert.True(t, hasSize, "warning should include 'size' field")
	assert.True(t, hasLimit, "warning should include 'limit' field")
}
