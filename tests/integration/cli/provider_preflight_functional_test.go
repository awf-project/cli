//go:build integration

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_RejectsUnauthenticatedClaudeBeforePrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test installs a POSIX shell fixture on PATH")
	}

	inheritedAuditPath := filepath.Join(t.TempDir(), "user-audit.jsonl")
	const inheritedAudit = "{\"user\":\"untouched\"}\n"
	require.NoError(t, os.WriteFile(inheritedAuditPath, []byte(inheritedAudit), 0o600))
	t.Setenv("AWF_AUDIT_LOG", inheritedAuditPath)
	tmpDir := setupPreflightTestDir(t)
	fakeBinDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(fakeBinDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fakeBinDir, "claude"), []byte(`#!/bin/sh
set -eu
if [ "$#" -ne 3 ] || [ "$1" != "auth" ] || [ "$2" != "status" ] || [ "$3" != "--json" ]; then
  printf 'model prompt was invoked: %s\n' "$*" >&2
  exit 64
fi
printf '%s\n' '{"loggedIn":false}'
`), 0o755))
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	createTestWorkflow(t, tmpDir, "claude-preflight.yaml", `name: claude-preflight
version: "1.0.0"
states:
  initial: ask
  ask:
    type: agent
    provider: claude
    prompt: "this prompt must never be sent"
    on_success: done
  done:
    type: terminal
`)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "claude-preflight", "--skip-plugins"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not_ready")
	assert.NotContains(t, out.String(), "model prompt was invoked")
	assertNoPreflightStart(t, tmpDir)
	contents, readErr := os.ReadFile(inheritedAuditPath)
	require.NoError(t, readErr)
	assert.Equal(t, inheritedAudit, string(contents))
}
