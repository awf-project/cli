package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
)

// TestRunCommand_WiresConversationManager verifies ConversationManager and UserInputReader
// are instantiated and injected for all conversation workflow scenarios and execution paths.
func TestRunCommand_WiresConversationManager(t *testing.T) {
	tests := []struct {
		name   string
		wfName string
		wfYAML string
		args   []string
	}{
		{
			name:   "normal run",
			wfName: "test-conversation.yaml",
			args:   []string{"run", "test-conversation"},
			wfYAML: `name: test-conversation
version: "1.0.0"
states:
  initial: chat
  chat:
    type: agent
    provider: claude
    mode: conversation
    prompt: "Hello"
    options:
      model: sonnet
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "dry-run",
			wfName: "test-conversation-dryrun.yaml",
			args:   []string{"run", "test-conversation-dryrun", "--dry-run"},
			wfYAML: `name: test-conversation-dryrun
version: "1.0.0"
states:
  initial: chat
  chat:
    type: agent
    provider: claude
    mode: conversation
    prompt: "Hello"
    options:
      model: sonnet
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "with system prompt",
			wfName: "test-system-prompt.yaml",
			args:   []string{"run", "test-system-prompt", "--input", "task=analyze the code"},
			wfYAML: `name: test-system-prompt
version: "1.0.0"
inputs:
  - name: task
    type: string
    default: "hello"
states:
  initial: discuss
  discuss:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: "You are a helpful code reviewer"
    prompt: "Task: {{inputs.task}}"
    options:
      model: sonnet
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "continue_from cross-step resume",
			wfName: "test-continue-from.yaml",
			args:   []string{"run", "test-continue-from"},
			wfYAML: `name: test-continue-from
version: "1.0.0"
states:
  initial: first_chat
  first_chat:
    type: agent
    provider: claude
    mode: conversation
    prompt: "Start conversation"
    options:
      model: sonnet
    on_success: second_chat
  second_chat:
    type: agent
    provider: claude
    mode: conversation
    prompt: "Continue discussion"
    conversation:
      continue_from: first_chat
    options:
      model: sonnet
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "parallel conversation steps",
			wfName: "test-parallel-conversation.yaml",
			args:   []string{"run", "test-parallel-conversation"},
			wfYAML: `name: test-parallel-conversation
version: "1.0.0"
states:
  initial: parallel_chat
  parallel_chat:
    type: parallel
    strategy: all_succeed
    parallel:
      - chat_1
      - chat_2
    on_success: done
  chat_1:
    type: agent
    provider: claude
    mode: conversation
    prompt: "First conversation"
    options:
      model: sonnet
    on_success: done
  chat_2:
    type: agent
    provider: claude
    mode: conversation
    prompt: "Second conversation"
    options:
      model: sonnet
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "single-step execution path",
			wfName: "test-single-step-conversation.yaml",
			args:   []string{"run", "test-single-step-conversation", "--step", "chat_step"},
			wfYAML: `name: test-single-step-conversation
version: "1.0.0"
states:
  initial: chat_step
  chat_step:
    type: agent
    provider: claude
    mode: conversation
    prompt: "Hello from single step"
    options:
      model: sonnet
    on_success: done
  done:
    type: terminal
`,
		},
	}

	// Force PATH to an empty directory so the claude binary cannot be found.
	// These tests only verify ConversationManager/UserInputReader wiring — they
	// should never actually invoke the provider CLI. Without this guard, each
	// subtest would make a real API call to Anthropic, adding 5–10s per case
	// and leaking through to the interactive stdin loop if the call succeeds.
	t.Setenv("PATH", t.TempDir())

	// Redirect os.Stdin to /dev/null as a defensive second layer in case the
	// stdin reader is reached before the provider fails fast.
	origStdin := os.Stdin
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	os.Stdin = devNull
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = devNull.Close()
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := setupTestDir(t)
			_ = os.MkdirAll(filepath.Join(tmpDir, ".awf", "states"), 0o755)
			_ = os.MkdirAll(filepath.Join(tmpDir, "history"), 0o755)
			createTestWorkflow(t, tmpDir, tc.wfName, tc.wfYAML)

			cmd := cli.NewRootCommand()
			var out, errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs(append([]string{"--storage=" + tmpDir}, tc.args...))

			err := cmd.Execute()

			fullOutput := out.String() + errOut.String()
			assert.NotContains(t, fullOutput, "conversation manager not configured",
				"ConversationManager should be wired in %s path", tc.name)
			assert.NotContains(t, fullOutput, "conversation mode requires a UserInputReader",
				"UserInputReader should be wired in %s path", tc.name)
			if err != nil {
				assert.NotContains(t, err.Error(), "conversation manager not configured",
					"ConversationManager must be wired in %s execution path", tc.name)
				assert.NotContains(t, err.Error(), "conversation mode requires a UserInputReader",
					"UserInputReader must be wired in %s execution path", tc.name)
			}
		})
	}
}
