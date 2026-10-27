//go:build integration

package cli_test

// Feature: F109

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

func TestRunCommand_MistralVibeRejectsUnsupportedPreflight(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test installs a POSIX shell fixture on PATH")
	}

	tmpDir := setupPreflightTestDir(t)
	fakeBinDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(fakeBinDir, 0o755))
	installFakeMistralVibeBinary(t, fakeBinDir, `#!/bin/sh
set -eu
touch executed
exit 64
`)
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	workflowContent := `name: mistral-vibe-functional
version: "1.0.0"
inputs:
  - name: task
    type: string
    required: true
states:
  initial: analyze
  analyze:
    type: agent
    provider: mistral_vibe
    prompt: "Classify: {{inputs.task}}"
    on_success: summarize
  summarize:
    type: step
    command: 'printf "summary=%s" "{{.states.analyze.Output}}"'
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mistral-vibe-functional.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storage=" + tmpDir,
		"run",
		"mistral-vibe-functional",
		"--input=task=release notes",
	})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not_ready")
	assertNoPreflightStart(t, tmpDir)
}

func TestRunCommand_MistralVibeMissingBinary(t *testing.T) {
	tmpDir := setupPreflightTestDir(t)
	t.Setenv("PATH", t.TempDir())

	workflowContent := `name: mistral-vibe-missing
version: "1.0.0"
states:
  initial: analyze
  analyze:
    type: agent
    provider: mistral_vibe
    prompt: "hello"
    on_success: done
  done:
    type: terminal
`
	createTestWorkflow(t, tmpDir, "mistral-vibe-missing.yaml", workflowContent)

	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--storage=" + tmpDir, "run", "mistral-vibe-missing"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
	assertNoPreflightStart(t, tmpDir)
}

func installFakeMistralVibeBinary(t *testing.T, dir, script string) {
	t.Helper()

	path := filepath.Join(dir, "vibe")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}
