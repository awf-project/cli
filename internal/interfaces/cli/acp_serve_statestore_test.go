package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestACPSessionStateDir_IsolatesSessions verifies that distinct ACP sessions resolve to
// distinct on-disk state directories. A shared state store would let two sessions running
// the same workflow (same WorkflowID key) clobber each other's persisted state.
func TestACPSessionStateDir_IsolatesSessions(t *testing.T) {
	dirA := acpSessionStateDir("session-aaaa")
	dirB := acpSessionStateDir("session-bbbb")

	assert.NotEqual(t, dirA, dirB, "distinct session IDs must map to distinct state dirs")
	assert.Equal(t, acpSessionStateDir("session-aaaa"), dirA, "same session ID must be stable")
}

// TestACPSessionStateDir_RootedUnderBase verifies the directory lives under the shared
// awf-acp-states base and includes the session segment.
func TestACPSessionStateDir_RootedUnderBase(t *testing.T) {
	base := filepath.Join(os.TempDir(), "awf-acp-states")
	dir := acpSessionStateDir("abc123")

	assert.True(t, strings.HasPrefix(dir, base+string(filepath.Separator)),
		"state dir %q must be rooted under base %q", dir, base)
	assert.Equal(t, filepath.Join(base, "abc123"), dir)
}

// TestACPSessionStateDir_NeutralizesPathTraversal verifies that traversal patterns in a
// session ID cannot escape the base directory, even though server-generated UUIDs are
// already safe.
func TestACPSessionStateDir_NeutralizesPathTraversal(t *testing.T) {
	base := filepath.Join(os.TempDir(), "awf-acp-states")

	tests := []struct {
		name      string
		sessionID string
	}{
		{"parent traversal", "../../../etc"},
		{"absolute path", "/etc/passwd"},
		{"nested traversal", "a/../../b"},
		{"dot", "."},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := acpSessionStateDir(tt.sessionID)

			assert.True(t, strings.HasPrefix(dir, base+string(filepath.Separator)),
				"state dir %q must stay under base %q for session %q", dir, base, tt.sessionID)
			assert.NotContains(t, dir, "..", "resolved dir must not contain traversal segments")
			// The resolved path must be exactly base/<single-segment>.
			rel, err := filepath.Rel(base, dir)
			assert.NoError(t, err)
			assert.NotContains(t, rel, string(filepath.Separator),
				"resolved dir must be a single segment under base, got %q", rel)
		})
	}
}
