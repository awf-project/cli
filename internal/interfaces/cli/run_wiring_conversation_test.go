package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/interfaces/cli"
	"github.com/stretchr/testify/assert"
)

// TestRunCommand_WiresConversationManager verifies ConversationManager is instantiated
// and injected for all conversation workflow scenarios and execution paths.
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
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Hello"
      max_turns: 3
      strategy: sliding_window
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
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Hello"
      max_turns: 2
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "multi-turn with input",
			wfName: "test-multiturn.yaml",
			args:   []string{"run", "test-multiturn", "--input", "task=analyze the code"},
			wfYAML: `name: test-multiturn
version: "1.0.0"
inputs:
  - name: task
    type: string
    default: "hello"
states:
  initial: discuss
  discuss:
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Task: {{inputs.task}}"
      max_turns: 5
      strategy: sliding_window
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
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Start conversation"
      max_turns: 2
    on_success: second_chat
  second_chat:
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Continue discussion"
      continue_from: first_chat
      max_turns: 2
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "inject_context enrichment",
			wfName: "test-inject-context.yaml",
			args:   []string{"run", "test-inject-context"},
			wfYAML: `name: test-inject-context
version: "1.0.0"
states:
  initial: chat_with_context
  chat_with_context:
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Help me with this task"
      max_turns: 3
      inject_context: "Additional context: focus on performance"
    on_success: done
  done:
    type: terminal
`,
		},
		{
			name:   "stop_condition evaluation",
			wfName: "test-stop-condition.yaml",
			args:   []string{"run", "test-stop-condition"},
			wfYAML: `name: test-stop-condition
version: "1.0.0"
states:
  initial: conditional_chat
  conditional_chat:
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Answer briefly"
      max_turns: 10
      stop_condition: "len(states.conditional_chat.conversation.turns) > 2"
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
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "First conversation"
      max_turns: 2
    on_success: done
  chat_2:
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Second conversation"
      max_turns: 2
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
    type: step
    agent:
      provider: claude
      model: sonnet
    conversation:
      initial_prompt: "Hello from single step"
      max_turns: 2
    on_success: done
  done:
    type: terminal
`,
		},
	}

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
			if err != nil {
				assert.NotContains(t, err.Error(), "conversation manager not configured",
					"ConversationManager must be wired in %s execution path", tc.name)
			}
		})
	}
}
