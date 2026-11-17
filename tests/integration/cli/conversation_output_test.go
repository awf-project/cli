//go:build integration

// Feature: 438
package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCommandDisplaysConversationResponseBeforeInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixtures require /bin/sh")
	}

	tests := []struct {
		provider        string
		sentinel        string
		script          string
		forbiddenOutput []string
	}{
		{
			provider: "claude",
			sentinel: "CLAUDE_ASSISTANT_VISIBLE",
			forbiddenOutput: []string{
				`{"type":"system"`,
				`{"type":"result"`,
				`"session_id":"claude-session"`,
				`"loggedIn"`,
			},
			script: `#!/bin/sh
if [ "$#" -eq 3 ] && [ "$1" = auth ] && [ "$2" = status ] && [ "$3" = --json ]; then
  printf '%s\n' '{"loggedIn":true}'
  exit 0
fi
if [ "$#" -eq 5 ] && [ "$1" = -p ] && [ "$2" = conversation-prompt ] && [ "$3" = --output-format ] && [ "$4" = stream-json ] && [ "$5" = --verbose ]; then
  printf '%s\n' '{"type":"system","subtype":"init","session_id":"claude-session"}'
  printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"CLAUDE_ASSISTANT_VISIBLE","session_id":"claude-session"}'
  exit 0
fi
printf '%s\n' 'unexpected claude fixture arguments' >&2
exit 64
`,
		},
		{
			provider: "codex",
			sentinel: "CODEX_ASSISTANT_VISIBLE",
			forbiddenOutput: []string{
				`{"type":"thread.started"`,
				`{"type":"item.completed"`,
				`"thread_id":"codex-thread"`,
				`"id":"codex-item"`,
				"Logged in using",
			},
			script: `#!/bin/sh
if [ "$#" -eq 2 ] && [ "$1" = login ] && [ "$2" = status ]; then
  printf '%s\n' 'Logged in using ChatGPT'
  exit 0
fi
if [ "$#" -eq 3 ] && [ "$1" = exec ] && [ "$2" = --json ] && [ "$3" = conversation-prompt ]; then
  printf '%s\n' '{"type":"thread.started","thread_id":"codex-thread"}'
  printf '%s\n' '{"type":"item.completed","item":{"id":"codex-item","type":"agent_message","text":"CODEX_ASSISTANT_VISIBLE"}}'
  exit 0
fi
printf '%s\n' 'unexpected codex fixture arguments' >&2
exit 64
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			env := newRootCommandTestEnvironment(t)
			fixtureDir := t.TempDir()
			fixturePath := filepath.Join(fixtureDir, tc.provider)
			require.NoError(t, os.WriteFile(fixturePath, []byte(tc.script), 0o700))
			t.Setenv("PATH", fixtureDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(fixtureDir, "claude-config"))
			t.Setenv("CODEX_HOME", filepath.Join(fixtureDir, "codex-home"))

			workflowName := "conversation-output-" + tc.provider
			env.writeWorkflow(t, workflowName, fmt.Sprintf(`name: %s
version: "1.0.0"
states:
  initial: converse
  converse:
    type: agent
    mode: conversation
    provider: %s
    prompt: conversation-prompt
    timeout: 10
    conversation: {}
    on_success: done
  done:
    type: terminal
    status: success
`, workflowName, tc.provider))

			observed := make(chan struct{})
			output := newObservingWriter(tc.sentinel, observed)
			stdinReader, stdinWriter := io.Pipe()
			t.Cleanup(func() {
				_ = stdinReader.Close()
				_ = stdinWriter.Close()
			})

			cmd := cli.NewRootCommand()
			cmd.SetIn(stdinReader)
			cmd.SetOut(output)
			cmd.SetErr(output)
			cmd.SetArgs([]string{"--storage", env.storageDir, "--no-color", "run", workflowName, "--skip-plugins"})

			execCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resultCh := make(chan error, 1)
			go func() { resultCh <- cmd.ExecuteContext(execCtx) }()

			select {
			case <-observed:
			case <-execCtx.Done():
				t.Fatalf("%s assistant sentinel observation timed out: %v", tc.provider, execCtx.Err())
			}
			select {
			case err := <-resultCh:
				t.Fatalf("%s command completed before stdin was released: %v", tc.provider, err)
			default:
			}

			_, err := io.WriteString(stdinWriter, "\n")
			require.NoError(t, err)
			require.NoError(t, stdinWriter.Close())
			select {
			case err := <-resultCh:
				require.NoError(t, err, "%s command failed", tc.provider)
			case <-execCtx.Done():
				t.Fatalf("%s command did not complete: %v", tc.provider, execCtx.Err())
			}

			capturedOutput := output.String()
			assert.Contains(t, capturedOutput, tc.sentinel)
			for _, forbidden := range tc.forbiddenOutput {
				assert.NotContains(t, capturedOutput, forbidden, "%s output exposed provider protocol or authentication material", tc.provider)
			}
			assert.NotContains(t, capturedOutput, fixtureDir, "%s output exposed its isolated fixture path", tc.provider)
			assert.NotContains(t, capturedOutput, `"prompt":"conversation-prompt"`, "%s output exposed a debug representation of the prompt", tc.provider)
			execution := env.loadExecutionByWorkflowName(t, workflowName)
			assert.Equal(t, workflow.StatusCompleted, execution.Status)
		})
	}
}

type observingWriter struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	sentinel string
	observed chan<- struct{}
	once     sync.Once
}

func newObservingWriter(sentinel string, observed chan<- struct{}) *observingWriter {
	return &observingWriter{sentinel: sentinel, observed: observed}
}

func (w *observingWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written, err := w.buffer.Write(data)
	if strings.Contains(w.buffer.String(), w.sentinel) {
		w.once.Do(func() { close(w.observed) })
	}
	return written, err
}

func (w *observingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}
