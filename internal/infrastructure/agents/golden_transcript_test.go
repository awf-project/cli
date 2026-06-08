package agents

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/stretchr/testify/require"
)

func TestGoldenTranscript_PerProvider(t *testing.T) {
	tests := []struct {
		provider   string
		inputPath  string
		goldenPath string
		fn         func([]byte) []transcript.ContentBlock
	}{
		{
			provider:   "claude",
			inputPath:  "testdata/transcript/claude/input.jsonl",
			goldenPath: "testdata/transcript/golden/claude.json",
			fn:         ClaudeToContentBlocks,
		},
		{
			provider:   "codex",
			inputPath:  "testdata/transcript/codex/input.jsonl",
			goldenPath: "testdata/transcript/golden/codex.json",
			fn:         CodexToContentBlocks,
		},
		{
			provider:   "gemini",
			inputPath:  "testdata/transcript/gemini/input.jsonl",
			goldenPath: "testdata/transcript/golden/gemini.json",
			fn:         GeminiToContentBlocks,
		},
		{
			provider:   "copilot",
			inputPath:  "testdata/transcript/copilot/input.jsonl",
			goldenPath: "testdata/transcript/golden/copilot.json",
			fn:         CopilotToContentBlocks,
		},
		{
			provider:   "openaicompatible",
			inputPath:  "testdata/transcript/openaicompatible/input.jsonl",
			goldenPath: "testdata/transcript/golden/openaicompatible.json",
			fn:         OpenAICompatibleToContentBlocks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			inputData, err := os.ReadFile(tt.inputPath)
			require.NoError(t, err, "reading input fixture")

			var blocks []transcript.ContentBlock
			scanner := bufio.NewScanner(bytes.NewReader(inputData))
			for scanner.Scan() {
				line := scanner.Bytes()
				if len(bytes.TrimSpace(line)) == 0 {
					continue
				}
				blocks = append(blocks, tt.fn(line)...)
			}
			require.NoError(t, scanner.Err(), "scanning input fixture")

			if blocks == nil {
				blocks = []transcript.ContentBlock{}
			}

			actual, err := json.MarshalIndent(blocks, "", "  ")
			require.NoError(t, err, "marshaling content blocks")
			actual = append(actual, '\n')

			if os.Getenv("UPDATE_GOLDEN") != "" {
				err = os.MkdirAll("testdata/transcript/golden", 0o755)
				require.NoError(t, err, "creating golden directory")
				err = os.WriteFile(tt.goldenPath, actual, 0o644)
				require.NoError(t, err, "writing golden file")
				return
			}

			expected, err := os.ReadFile(tt.goldenPath)
			require.NoErrorf(t, err, "golden file %q missing — run with UPDATE_GOLDEN=1 to generate it", tt.goldenPath)

			require.Equal(t, string(expected), string(actual),
				"golden mismatch for provider %q — if intentional, re-run with UPDATE_GOLDEN=1 to refresh the golden file",
				tt.provider)
		})
	}
}
