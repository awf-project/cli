//go:build integration

package cli_test

// Feature: F109

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommand_MistralVibeAgentWorkflow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test installs a POSIX shell fixture on PATH")
	}

	tmpDir := setupInitTestDir(t)
	fakeBinDir := filepath.Join(tmpDir, "bin")
	require.NoError(t, os.MkdirAll(fakeBinDir, 0o755))
	installFakeMistralVibeBinary(t, fakeBinDir, `#!/bin/sh
set -eu
if [ "$1" != "--prompt" ] || [ "$2" != "Classify: release notes" ] || [ "$3" != "--agent" ] || [ "$4" != "default" ]; then
  printf 'unexpected args: %s\n' "$*" >&2
  exit 64
fi
printf 'approved from vibe\n'
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
	require.NoError(t, err)

	state := loadOnlyExecutionState(t, tmpDir)
	require.Equal(t, workflow.StatusCompleted, state.Status)
	assert.Equal(t, "approved from vibe", state.States["analyze"].Output)
	assert.Equal(t, "summary=approved from vibe", state.States["summarize"].Output)
}

func TestRunCommand_MistralVibeMissingBinary(t *testing.T) {
	tmpDir := setupInitTestDir(t)
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
	assert.Contains(t, err.Error(), "mistral_vibe execution failed")
	assert.Contains(t, err.Error(), "executable file not found")
}

func installFakeMistralVibeBinary(t *testing.T, dir, script string) {
	t.Helper()

	path := filepath.Join(dir, "vibe")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
}

func loadOnlyExecutionState(t *testing.T, storageDir string) *workflow.ExecutionContext {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(storageDir, "states", "*.json"))
	require.NoError(t, err)
	require.Len(t, matches, 1)

	data, err := os.ReadFile(matches[0])
	require.NoError(t, err)

	var state workflow.ExecutionContext
	require.NoError(t, json.Unmarshal(data, &state))

	return &state
}
