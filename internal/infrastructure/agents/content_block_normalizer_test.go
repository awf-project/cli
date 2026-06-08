package agents

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/transcript"
)

// TestContentBlockNormalizer_DispatchesByProvider verifies the registry maps each
// supported provider name to its per-line normalizer and produces agent_emitted blocks.
func TestContentBlockNormalizer_DispatchesByProvider(t *testing.T) {
	n := NewContentBlockNormalizer()

	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}` + "\n"
	blocks := n.Normalize("claude", []byte(raw))

	require.Len(t, blocks, 1)
	assert.Equal(t, transcript.BlockTypeText, blocks[0].Type)
	assert.Equal(t, "hello", blocks[0].Text)
	assert.Equal(t, transcript.FidelityAgentEmitted, blocks[0].Fidelity)
}

// TestContentBlockNormalizer_AccumulatesAcrossLines verifies multi-line NDJSON output
// is scanned line-by-line and the resulting blocks are concatenated in order.
func TestContentBlockNormalizer_AccumulatesAcrossLines(t *testing.T) {
	n := NewContentBlockNormalizer()

	raw := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"hmm"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"answer"}]}}`,
		"", // blank line must be skipped, not panic
	}, "\n")

	blocks := n.Normalize("claude", []byte(raw))

	require.Len(t, blocks, 2)
	assert.Equal(t, transcript.BlockTypeThinking, blocks[0].Type)
	assert.Equal(t, transcript.BlockTypeText, blocks[1].Type)
}

// TestContentBlockNormalizer_UnknownProviderYieldsNil verifies unknown providers and
// empty output produce no blocks (and never panic).
func TestContentBlockNormalizer_UnknownProviderYieldsNil(t *testing.T) {
	n := NewContentBlockNormalizer()

	assert.Nil(t, n.Normalize("unknown_provider", []byte(`{"type":"assistant"}`)))
	assert.Nil(t, n.Normalize("claude", nil))
	assert.Nil(t, n.Normalize("claude", []byte("")))
}

// TestContentBlockNormalizer_HandlesLongLines verifies a line larger than bufio.Scanner's
// default 64KB token is not silently dropped (agent responses can be large).
func TestContentBlockNormalizer_HandlesLongLines(t *testing.T) {
	n := NewContentBlockNormalizer()

	big := strings.Repeat("x", 200*1024) // 200KB > default 64KB scanner token
	raw := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + big + `"}]}}`

	blocks := n.Normalize("claude", []byte(raw))

	require.Len(t, blocks, 1)
	assert.Equal(t, big, blocks[0].Text)
}
