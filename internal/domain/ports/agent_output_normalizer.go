package ports

import "github.com/awf-project/cli/internal/domain/transcript"

// AgentOutputNormalizer converts a provider's raw CLI output (NDJSON) into the canonical
// transcript ContentBlock vocabulary for F106 message.assistant events. It is the seam
// that absorbs per-provider output divergence (Claude, Codex, Gemini, Copilot, OpenAI
// compatible) in a single place (SC-002), implemented in the infrastructure agents layer.
//
// Implementations must be total and panic-free: an unknown provider, empty output, or
// unparseable lines yield no blocks rather than an error.
type AgentOutputNormalizer interface {
	// Normalize maps the provider's raw output into ordered ContentBlocks. The provider
	// argument is the resolved provider name (matching the agent registry key).
	Normalize(provider string, rawOutput []byte) []transcript.ContentBlock
}
