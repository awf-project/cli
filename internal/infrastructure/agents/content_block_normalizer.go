package agents

import (
	"bufio"
	"bytes"

	"github.com/awf-project/cli/internal/domain/transcript"
)

// maxNormalizerLineBytes bounds a single NDJSON line during normalization. Agent
// responses (especially Codex/Claude with large text blocks) routinely exceed
// bufio.Scanner's default 64KB token, which would otherwise silently drop the line.
const maxNormalizerLineBytes = 10 << 20 // 10 MiB

// lineNormalizer maps one provider NDJSON line to canonical ContentBlocks.
type lineNormalizer func(line []byte) []transcript.ContentBlock

// ContentBlockNormalizer is the infrastructure implementation of
// ports.AgentOutputNormalizer. It dispatches a provider's raw CLI output to the
// matching per-provider line normalizer (the *ToContentBlocks functions), absorbing
// provider divergence in this single layer (F106 SC-002).
type ContentBlockNormalizer struct{}

// NewContentBlockNormalizer returns a ready-to-use normalizer. It is stateless, so the
// zero value works too; the constructor exists for wiring symmetry.
func NewContentBlockNormalizer() ContentBlockNormalizer {
	return ContentBlockNormalizer{}
}

// Normalize scans the provider's raw NDJSON output line-by-line and concatenates the
// blocks produced for each line. It returns nil for an unknown provider, empty output,
// or output that yields no blocks. It never panics on malformed input — each line
// normalizer tolerates unparseable lines by returning no blocks.
func (ContentBlockNormalizer) Normalize(provider string, rawOutput []byte) []transcript.ContentBlock {
	fn := lineNormalizerFor(provider)
	if fn == nil || len(rawOutput) == 0 {
		return nil
	}

	var blocks []transcript.ContentBlock
	scanner := bufio.NewScanner(bytes.NewReader(rawOutput))
	scanner.Buffer(make([]byte, 0, 64*1024), maxNormalizerLineBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		blocks = append(blocks, fn(line)...)
	}
	return blocks
}

// lineNormalizerFor resolves the per-line normalizer for a resolved provider name.
// The names match the agent registry keys (provider.Name()). Providers without a
// transcript normalizer (e.g. opencode, reserved for a future feature) return nil.
func lineNormalizerFor(provider string) lineNormalizer {
	switch provider {
	case "claude":
		return ClaudeToContentBlocks
	case "codex":
		return CodexToContentBlocks
	case "gemini":
		return GeminiToContentBlocks
	case "github_copilot":
		return CopilotToContentBlocks
	case "openai_compatible":
		return OpenAICompatibleToContentBlocks
	default:
		return nil
	}
}
