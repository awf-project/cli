package audit_skips_test

import "testing"

// Test fixture for audit-skips.sh testing
// Contains CLI tool dependency skip patterns

func TestClaudeNotInstalled(t *testing.T) {
	t.Skip("claude CLI not installed")
}

func TestGeminiNotInstalled(t *testing.T) {
	t.Skip("gemini not available")
}

func TestCodexNotInstalled(t *testing.T) {
	t.Skip("codex not installed")
}
