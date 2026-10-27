//go:build integration

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPreflightTestDir(t *testing.T) string {
	t.Helper()
	dir := setupInitTestDir(t)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("AWF_AUDIT_LOG", "")
	t.Setenv("AWF_CONFIG_PATH", "")
	t.Setenv("AWF_WORKFLOWS_PATH", filepath.Join(dir, ".awf", "workflows"))
	t.Setenv("AWF_TEST_MODE", "1")
	return dir
}

func runPreflightWorkflow(t *testing.T, dir, content string) (error, string) {
	t.Helper()
	createTestWorkflow(t, dir, "preflight.yaml", content)
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--storage=" + dir, "run", "preflight", "--skip-plugins", "--output=buffered"})
	err := cmd.Execute()
	return err, out.String()
}

func assertNoPreflightStart(t *testing.T, dir string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(dir, "data", "awf", "audit.jsonl"))
	if !os.IsNotExist(err) {
		require.NoError(t, err)
		assert.NotContains(t, string(contents), "workflow.started")
	}
	_, err = os.Stat(filepath.Join(dir, "executed"))
	require.True(t, os.IsNotExist(err), "execution canary must remain absent: %v", err)
}

func TestRunCommandRejectsUnresolvedInputsAndMalformedOperationsBeforeExecution(t *testing.T) {
	for _, tc := range []struct {
		name, state, diagnostic string
	}{
		{"undefined command input", "type: step\n    command: 'echo {{inputs.missing}}'", "undefined input"},
		{"missing GitHub title", "type: operation\n    operation: github.create_issue\n    inputs: {}", "title"},
		{"missing HTTP URL", "type: operation\n    operation: http.request\n    inputs:\n      method: GET", "url"},
		{"missing notification message", "type: operation\n    operation: notify.send\n    inputs:\n      backend: desktop", "message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupPreflightTestDir(t)
			// Even a preceding valid step must not run when a later field is invalid.
			err, output := runPreflightWorkflow(t, dir, "name: preflight\nversion: '1.0.0'\nstates:\n  initial: canary\n  canary:\n    type: step\n    command: 'touch executed'\n    on_success: invalid\n  invalid:\n    "+tc.state+"\n    on_success: done\n  done:\n    type: terminal\n")
			require.Error(t, err, output)
			assert.Contains(t, err.Error(), tc.diagnostic)
			assertNoPreflightStart(t, dir)
		})
	}
}

func TestRunCommandRejectsProvidersThatCannotRunNoninteractivelyBeforeExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX executable fixtures")
	}
	for _, tc := range []struct {
		name, script, diagnostic string
	}{
		{"unavailable", "", "unavailable"},
		{"unauthenticated", "#!/bin/sh\nif [ \"$1\" = auth ]; then printf '%s\\n' '{\"loggedIn\":false}'; exit 1; fi\ntouch executed\n", "not_ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupPreflightTestDir(t)
			bin := filepath.Join(dir, "bin")
			require.NoError(t, os.MkdirAll(bin, 0o755))
			if tc.script != "" {
				require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte(tc.script), 0o755))
			}
			t.Setenv("PATH", bin)
			err, output := runPreflightWorkflow(t, dir, "name: preflight\nversion: '1.0.0'\nstates:\n  initial: ask\n  ask:\n    type: agent\n    provider: claude\n    prompt: 'never execute'\n    on_success: done\n  done:\n    type: terminal\n")
			require.Error(t, err, output)
			assert.Contains(t, err.Error(), tc.diagnostic)
			var exitErr interface{ ExitCode() int }
			require.True(t, errors.As(err, &exitErr))
			assert.Equal(t, 1, exitErr.ExitCode())
			assertNoPreflightStart(t, dir)
		})
	}
}

// Unknown backend selection is rejected by the operation provider before Send;
// unlike malformed input schemas, it is a runtime dispatch check.
func TestRunCommandReportsNotificationBackendsDeterministicallyBeforeDispatch(t *testing.T) {
	dir := setupPreflightTestDir(t)
	const content = "name: preflight\nversion: '1.0.0'\nstates:\n  initial: notify\n  notify:\n    type: operation\n    operation: notify.send\n    inputs:\n      backend: unavailable\n      message: 'no backend must send this'\n    on_success: done\n  done:\n    type: terminal\n"
	for range 3 {
		err, output := runPreflightWorkflow(t, dir, content)
		require.Error(t, err, output)
		assert.Contains(t, err.Error(), "available: [desktop webhook]")
	}
}

func TestRunCommandRejectsConversationWithoutInputBeforeExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX executable fixture")
	}
	dir := setupPreflightTestDir(t)
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	const script = "#!/bin/sh\nif [ \"$1\" = auth ]; then printf '%s\\n' '{\"loggedIn\":true}'; exit 0; fi\n: > executed\nexit 64\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755))
	t.Setenv("PATH", bin)
	err, output := runPreflightWorkflow(t, dir, "name: preflight\nversion: '1.0.0'\nstates:\n  initial: ask\n  ask:\n    type: agent\n    provider: claude\n    mode: conversation\n    prompt: 'never execute without input'\n    on_success: done\n  done:\n    type: terminal\n")
	require.Error(t, err, output)
	assert.Contains(t, err.Error(), "interactive_input_required")
	assertNoPreflightStart(t, dir)
}
