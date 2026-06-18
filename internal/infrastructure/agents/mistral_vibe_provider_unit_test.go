package agents

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
)

func TestMistralVibeProvider_NewMistralVibeProviderReturnsNonNilProviderWhoseNameReturnsExactlyMistralVibe(t *testing.T) {
	provider := NewMistralVibeProvider()

	require.NotNil(t, provider)
	assert.Equal(t, "mistral_vibe", provider.Name())
}

func TestMistralVibeProvider_OptionTypeIsDefinedInOptionsGo(t *testing.T) {
	applied := false
	var option MistralVibeProviderOption = func(*MistralVibeProvider) {
		applied = true
	}

	provider := NewMistralVibeProviderWithOptions(option)

	require.NotNil(t, provider)
	assert.True(t, applied)
}

func TestMistralVibeProvider_NewWithOptionsIsPublicConstructorSurfaceReturnsNonNilAppliesOptionsInOrderAndDefaultDelegates(t *testing.T) {
	order := make([]string, 0, 2)

	provider := NewMistralVibeProviderWithOptions(
		func(*MistralVibeProvider) {
			order = append(order, "first")
		},
		func(*MistralVibeProvider) {
			order = append(order, "second")
		},
	)
	defaultProvider := NewMistralVibeProvider()
	explicitNoOptionsProvider := NewMistralVibeProviderWithOptions()

	require.NotNil(t, provider)
	require.NotNil(t, defaultProvider)
	require.NotNil(t, explicitNoOptionsProvider)
	assert.Equal(t, []string{"first", "second"}, order)
	assert.Equal(t, explicitNoOptionsProvider.Name(), defaultProvider.Name())
}

func TestMistralVibeProvider_ImplementsPortsAgentProvider(t *testing.T) {
	var _ ports.AgentProvider = (*MistralVibeProvider)(nil)
}

func TestMistralVibeProvider_ValidateValidatesBinaryNameVibeWhenUnavailableReturnsExactError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	provider := NewMistralVibeProvider()

	err := provider.Validate()

	require.Error(t, err)
	assert.Equal(t, "mistral_vibe provider validation failed: vibe binary not found in PATH", err.Error())
}

func TestMistralVibeProvider_ExecuteRejectsEmptyOrWhitespaceOnlyPromptsBeforeAnyExecutorCall(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
	}{
		{name: "empty", prompt: ""},
		{name: "whitespace", prompt: "  \t\n  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
				p.executor = executor
			})

			result, err := provider.Execute(context.Background(), tt.prompt, nil, nil, nil)

			require.Error(t, err)
			assert.Equal(t, "prompt cannot be empty", err.Error())
			assert.Nil(t, result)
			assert.Empty(t, executor.GetCalls())
		})
	}
}

func TestMistralVibeProvider_ExecuteHelloInvokesBinaryVibeWithArgvPromptHello(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("hello response"), nil)
	provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
		p.executor = executor
	})

	result, err := provider.Execute(context.Background(), "hello", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	calls := executor.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "vibe", calls[0].Name)
	assert.Equal(t, []string{"--prompt", "hello", "--agent", "default"}, calls[0].Args)
}

func TestMistralVibeProvider_ExecuteConversationUsesSameVibePromptPathAndMissingStableSessionIDRemainsEmpty(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("conversation response"), nil)
	provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
		p.executor = executor
	})
	state := workflow.NewConversationState("")

	result, err := provider.ExecuteConversation(context.Background(), state, "hello", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.State)
	assert.Equal(t, "", result.State.SessionID)
	calls := executor.GetCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "vibe", calls[0].Name)
	assert.Equal(t, []string{"--prompt", "hello", "--agent", "default"}, calls[0].Args)
}

func TestMistralVibeProvider_ExecuteConversationSecondTurnSendsTranscriptPrompt(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("second response"), nil)
	provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
		p.executor = executor
	})
	state := workflow.NewConversationState("You ask concise confirmation questions.")
	require.NoError(t, state.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, "Should I continue?")))
	require.NoError(t, state.AddTurn(workflow.NewTurn(workflow.TurnRoleAssistant, "Reply yes to continue.")))

	result, err := provider.ExecuteConversation(context.Background(), state, "yes", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	calls := executor.GetCalls()
	require.Len(t, calls, 1)
	require.Len(t, calls[0].Args, 4)
	assert.Equal(t, "--prompt", calls[0].Args[0])
	prompt := calls[0].Args[1]
	assert.Contains(t, prompt, "System: You ask concise confirmation questions.")
	assert.Contains(t, prompt, "User: Should I continue?")
	assert.Contains(t, prompt, "Assistant: Reply yes to continue.")
	assert.Contains(t, prompt, "User: yes")
	assert.NotEqual(t, "yes", prompt)
	assert.Equal(t, []string{"--agent", "default"}, calls[0].Args[2:])
}

func TestMistralVibeProvider_Execute_WithOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		want    []string
	}{
		{
			name:    "MistralVibeProvider.Execute passes the safe explicit default agent profile default when agent_profile is absent",
			options: nil,
			want:    []string{"--prompt", "hello", "--agent", "default"},
		},
		{
			name:    "agent_profile plan maps to --agent plan",
			options: map[string]any{"agent_profile": "plan"},
			want:    []string{"--prompt", "hello", "--agent", "plan"},
		},
		{
			name:    "output_format accepts text json and streaming and maps to repeated argv pair --output value",
			options: map[string]any{"output_format": "streaming"},
			want:    []string{"--prompt", "hello", "--agent", "default", "--output", "streaming"},
		},
		{
			name:    "max_turns accepts positive integers only and maps to --max-turns n",
			options: map[string]any{"max_turns": 3},
			want:    []string{"--prompt", "hello", "--agent", "default", "--max-turns", "3"},
		},
		{
			name:    "max_tokens accepts positive integers only and maps to --max-tokens n",
			options: map[string]any{"max_tokens": 4096},
			want:    []string{"--prompt", "hello", "--agent", "default", "--max-tokens", "4096"},
		},
		{
			name:    "max_price accepts non-negative numbers only and maps to --max-price value",
			options: map[string]any{"max_price": 1.25},
			want:    []string{"--prompt", "hello", "--agent", "default", "--max-price", "1.25"},
		},
		{
			name:    "enabled_tools accepts a string list and emits repeated --enabled-tools value argv pairs in input order",
			options: map[string]any{"enabled_tools": []string{"read", "write"}},
			want: []string{
				"--prompt", "hello", "--agent", "default",
				"--enabled-tools", "read",
				"--enabled-tools", "write",
			},
		},
		{
			name:    "add_dirs accepts a string list and emits repeated --add-dir clean-path argv pairs in input order",
			options: map[string]any{"add_dirs": []string{"./src", "a/./b"}},
			want: []string{
				"--prompt", "hello", "--agent", "default",
				"--add-dir", "src",
				"--add-dir", "a/b",
			},
		},
		{
			name:    "trust true maps to --trust",
			options: map[string]any{"trust": true},
			want:    []string{"--prompt", "hello", "--agent", "default", "--trust"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			executor.SetOutput([]byte("hello response"), nil)
			provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
				p.executor = executor
			})

			result, err := provider.Execute(context.Background(), "hello", tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			calls := executor.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "vibe", calls[0].Name)
			assert.Equal(t, tt.want, calls[0].Args)
		})
	}
}

func TestMistralVibeProvider_ValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr string
	}{
		{
			name:    "Invalid output_format yaml returns exact error",
			options: map[string]any{"output_format": "yaml"},
			wantErr: "mistral_vibe option output_format invalid: expected one of text, json, streaming, got yaml",
		},
		{
			name:    "max_turns 0 returns exact positive integer error",
			options: map[string]any{"max_turns": 0},
			wantErr: "mistral_vibe option max_turns invalid: expected positive integer, got 0",
		},
		{
			name:    "max_tokens -1 returns exact positive integer error",
			options: map[string]any{"max_tokens": -1},
			wantErr: "mistral_vibe option max_tokens invalid: expected positive integer, got -1",
		},
		{
			name:    "max_price -0.01 returns exact non-negative number error",
			options: map[string]any{"max_price": -0.01},
			wantErr: "mistral_vibe option max_price invalid: expected non-negative number, got -0.01",
		},
		{
			name:    "max_price NaN returns canonical float type error",
			options: map[string]any{"max_price": math.NaN()},
			wantErr: "mistral_vibe option max_price invalid: expected non-negative number, got float",
		},
		{
			name:    "max_price positive infinity returns canonical float type error",
			options: map[string]any{"max_price": math.Inf(1)},
			wantErr: "mistral_vibe option max_price invalid: expected non-negative number, got float",
		},
		{
			name:    "Invalid numeric option types return canonical type vocabulary",
			options: map[string]any{"max_turns": "3"},
			wantErr: "mistral_vibe option max_turns invalid: expected positive integer, got string",
		},
		{
			name:    "enabled_tools rejects non-string list entries with exact indexed error",
			options: map[string]any{"enabled_tools": []any{"read", 42}},
			wantErr: "mistral_vibe option enabled_tools invalid: expected string at index 1, got int",
		},
		{
			name:    "workdir empty returns exact path cannot be empty error",
			options: map[string]any{"workdir": ""},
			wantErr: "mistral_vibe option workdir invalid: path cannot be empty",
		},
		{
			name:    "workdir traversal returns exact traversal error",
			options: map[string]any{"workdir": "../outside"},
			wantErr: "mistral_vibe option workdir invalid: path must not contain traversal segments",
		},
		{
			name:    "add_dirs rejects empty entries with exact indexed error",
			options: map[string]any{"add_dirs": []string{"src", ""}},
			wantErr: "mistral_vibe option add_dirs invalid at index 1: path cannot be empty",
		},
		{
			name:    "add_dirs rejects traversal-like entries with exact indexed error",
			options: map[string]any{"add_dirs": []string{"src", "../outside"}},
			wantErr: "mistral_vibe option add_dirs invalid at index 1: path must not contain traversal segments",
		},
		{
			name:    "trust values of any non-bool type return exact canonical type error",
			options: map[string]any{"trust": "true"},
			wantErr: "mistral_vibe option trust invalid: expected bool, got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
				p.executor = executor
			})

			result, err := provider.Execute(context.Background(), "hello", tt.options, nil, nil)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Equal(t, tt.wantErr, err.Error())
			assert.Empty(t, executor.GetCalls())
		})
	}
}

func TestMistralVibeProvider_UnsafeApproval(t *testing.T) {
	t.Run("dangerously_skip_permissions true maps to --agent auto-approve", func(t *testing.T) {
		executor := mocks.NewMockCLIExecutor()
		executor.SetOutput([]byte("hello response"), nil)
		provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
			p.executor = executor
		})

		result, err := provider.Execute(context.Background(), "hello", map[string]any{
			"dangerously_skip_permissions": true,
		}, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		calls := executor.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"--prompt", "hello", "--agent", "auto-approve"}, calls[0].Args)
	})

	t.Run("dangerously_skip_permissions true with agent_profile plan returns exact conflict error and does not call executor", func(t *testing.T) {
		executor := mocks.NewMockCLIExecutor()
		provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
			p.executor = executor
		})

		result, err := provider.Execute(context.Background(), "hello", map[string]any{
			"dangerously_skip_permissions": true,
			"agent_profile":                "plan",
		}, nil, nil)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "mistral_vibe option agent_profile conflicts with dangerously_skip_permissions: expected auto-approve, got plan", err.Error())
		assert.Empty(t, executor.GetCalls())
	})

	t.Run("dangerously_skip_permissions true with agent_profile auto-approve is accepted and emits one --agent auto-approve", func(t *testing.T) {
		executor := mocks.NewMockCLIExecutor()
		executor.SetOutput([]byte("hello response"), nil)
		provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
			p.executor = executor
		})

		result, err := provider.Execute(context.Background(), "hello", map[string]any{
			"dangerously_skip_permissions": true,
			"agent_profile":                "auto-approve",
		}, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		calls := executor.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"--prompt", "hello", "--agent", "auto-approve"}, calls[0].Args)
	})
}

func TestMistralVibeProvider_ArgumentBoundaries(t *testing.T) {
	t.Run("enabled_tools values containing shell metacharacters remain single argv values", func(t *testing.T) {
		tool := `grep *.go "quoted"; rm -rf $HOME | $(echo no)`
		executor := mocks.NewMockCLIExecutor()
		executor.SetOutput([]byte("hello response"), nil)
		provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
			p.executor = executor
		})

		result, err := provider.Execute(context.Background(), "hello", map[string]any{
			"enabled_tools": []string{tool},
		}, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		calls := executor.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{
			"--prompt", "hello", "--agent", "default",
			"--enabled-tools", tool,
		}, calls[0].Args)
	})

	t.Run("Valid workdir ./workspace is cleaned to workspace and emits argv pair --workdir workspace", func(t *testing.T) {
		executor := mocks.NewMockCLIExecutor()
		executor.SetOutput([]byte("hello response"), nil)
		provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
			p.executor = executor
		})

		result, err := provider.Execute(context.Background(), "hello", map[string]any{
			"workdir": "./workspace",
		}, nil, nil)

		require.NoError(t, err)
		require.NotNil(t, result)
		calls := executor.GetCalls()
		require.Len(t, calls, 1)
		assert.Equal(t, []string{"--prompt", "hello", "--agent", "default", "--workdir", "workspace"}, calls[0].Args)
	})
}

func TestMistralVibeProvider_SuccessfulTextStdoutFromMockedVibeExecutionIsReturnedAsCleanAgentResultOutput(t *testing.T) {
	executor := mocks.NewMockCLIExecutor()
	executor.SetOutput([]byte("  clean text output\n"), nil)
	provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
		p.executor = executor
	})

	result, err := provider.Execute(context.Background(), "hello", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "clean text output", result.Output)
}

func TestMistralVibeProvider_ExecutorErrorsArePrefixedAndPreserveOriginalError(t *testing.T) {
	originalErr := errors.New("exit status 42")
	executor := mocks.NewMockCLIExecutor()
	executor.SetError(originalErr)
	provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
		p.executor = executor
	})

	result, err := provider.Execute(context.Background(), "hello", nil, nil, nil)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "mistral_vibe execution failed: exit status 42", err.Error())
	assert.ErrorIs(t, err, originalErr)
}

func TestMistralVibeProvider_ExtractTextContent(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "extractMistralVibeTextContent(output string) returns plain text unchanged for text output that is not a JSON or NDJSON provider envelope",
			output: "plain assistant text with {literal braces} and [literal brackets]",
			want:   "plain assistant text with {literal braces} and [literal brackets]",
		},
		{
			name:   "plain JSON assistant output is not treated as a provider envelope",
			output: `{"status":"ok","count":2}`,
			want:   `{"status":"ok","count":2}`,
		},
		{
			name:   "bracket-prefixed plain text is not treated as a provider envelope",
			output: "[draft] summarize the release",
			want:   "[draft] summarize the release",
		},
		{
			name:   "non-assistant JSON provider envelope is suppressed",
			output: `{"type":"message","role":"tool","content":"tool envelope only"}`,
			want:   "",
		},
		{
			name:   "malformed JSON provider envelope is suppressed",
			output: `{"type":"message","role":"assistant","content":`,
			want:   "",
		},
		{
			name:   "Empty output returns empty text and reports no assistant text found",
			output: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMistralVibeTextContent(tt.output)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMistralVibeProvider_ExtractAssistantText(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantText  string
		wantFound bool
	}{
		{
			name:      "extractMistralVibeAssistantText(output string) returns the assistant message text from a JSON Vibe envelope and reports that assistant text was found",
			output:    `{"type":"message","role":"assistant","content":"final answer"}`,
			wantText:  "final answer",
			wantFound: true,
		},
		{
			name:      "extractMistralVibeAssistantText(output string) returns nested assistant content from a JSON Vibe envelope and reports that assistant text was found",
			output:    `{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"final "},{"type":"text","text":"answer"}]}}`,
			wantText:  "final answer",
			wantFound: true,
		},
		{
			name:      "extractMistralVibeAssistantText(output string) returns the last assistant message from NDJSON or streaming output when multiple assistant messages are present",
			output:    "{\"type\":\"message\",\"role\":\"assistant\",\"content\":\"first\"}\n" + `{"type":"message","role":"tool","content":"ignored"}` + "\n" + `{"type":"message","role":"assistant","content":[{"type":"text","text":"last"}]}`,
			wantText:  "last",
			wantFound: true,
		},
		{
			name:      "extractMistralVibeAssistantText(output string) excludes raw provider envelope JSON from the returned assistant text",
			output:    `{"type":"message","role":"tool","content":"tool envelope only"}`,
			wantText:  "",
			wantFound: false,
		},
		{
			name:      "Malformed JSON or malformed NDJSON lines do not panic and do not cause raw provider envelopes to be returned as normal assistant text",
			output:    `{"type":"message","role":"assistant","content":`,
			wantText:  "",
			wantFound: false,
		},
		{
			name:      "Empty output returns empty text and reports no assistant text found",
			output:    "",
			wantText:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotFound := extractMistralVibeAssistantText(tt.output)

			assert.Equal(t, tt.wantText, gotText)
			assert.Equal(t, tt.wantFound, gotFound)
		})
	}
}

func TestMistralVibeProvider_Execute_ResponsePopulation(t *testing.T) {
	tests := []struct {
		name         string
		mockStdout   string
		wantOutput   string
		wantResponse map[string]any
	}{
		{
			name:       "MistralVibeProvider.Execute sets workflow.AgentResult.Output to the extracted assistant text for JSON and NDJSON outputs",
			mockStdout: `{"type":"message","role":"assistant","content":"clean JSON output"}`,
			wantOutput: "clean JSON output",
		},
		{
			name:       "MistralVibeProvider.Execute sets workflow.AgentResult.Output to the extracted assistant text for NDJSON outputs",
			mockStdout: `{"type":"message","role":"assistant","content":"first"}` + "\n" + `{"type":"message","role":"assistant","content":"clean NDJSON output"}`,
			wantOutput: "clean NDJSON output",
		},
		{
			name:         "MistralVibeProvider.Execute populates workflow.AgentResult.Response only when the extracted assistant text itself is valid JSON",
			mockStdout:   `{"type":"message","role":"assistant","content":"{\"status\":\"ok\",\"count\":2}"}`,
			wantOutput:   `{"status":"ok","count":2}`,
			wantResponse: map[string]any{"status": "ok", "count": float64(2)},
		},
		{
			name:       "MistralVibeProvider.Execute preserves direct JSON assistant output that is not a provider envelope",
			mockStdout: `{"status":"ok","count":2}`,
			wantOutput: `{"status":"ok","count":2}`,
		},
		{
			name:       "MistralVibeProvider.Execute preserves bracket-prefixed text output that is not a provider envelope",
			mockStdout: "[draft] summarize the release",
			wantOutput: "[draft] summarize the release",
		},
		{
			name:       "MistralVibeProvider.Execute leaves workflow.AgentResult.Response nil or empty when extracted assistant text is not valid JSON",
			mockStdout: `{"type":"message","role":"assistant","content":"not json"}`,
			wantOutput: "not json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			executor.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
				p.executor = executor
			})

			result, err := provider.Execute(context.Background(), "hello", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantOutput, result.Output)
			if tt.wantResponse == nil {
				assert.Empty(t, result.Response)
				return
			}
			assert.Equal(t, tt.wantResponse, result.Response)
		})
	}
}

func TestMistralVibeProvider_Execute_MalformedOutput(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout string
	}{
		{
			name:       "Malformed JSON or malformed NDJSON lines do not panic and do not cause raw provider envelopes to be returned as normal assistant text",
			mockStdout: `{"type":"message","role":"assistant","content":`,
		},
		{
			name:       "Malformed NDJSON envelope lines do not cause raw provider envelopes to be returned as normal assistant text",
			mockStdout: "{\"type\":\"message\",\"role\":\"assistant\",\"content\":\"ok\"}\n" + `{"type":"message","role":"assistant","content":`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			executor.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
				p.executor = executor
			})

			result, err := provider.Execute(context.Background(), "hello", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Empty(t, result.Output)
			assert.Empty(t, result.Response)
		})
	}
}

func TestMistralVibeProvider_ExecuteConversation_UsesCleanAssistantText(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout string
		wantOutput string
	}{
		{
			name:       "valid assistant envelope returns clean assistant text",
			mockStdout: `{"type":"message","role":"assistant","content":"conversation text"}`,
			wantOutput: "conversation text",
		},
		{
			name:       "malformed envelope-like JSON does not leak raw provider output",
			mockStdout: `{"type":"message","role":"assistant","content":`,
			wantOutput: "",
		},
		{
			name:       "valid non-assistant envelope does not become assistant text",
			mockStdout: `{"type":"message","role":"tool","content":"secret"}`,
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := mocks.NewMockCLIExecutor()
			executor.SetOutput([]byte(tt.mockStdout), nil)
			provider := NewMistralVibeProviderWithOptions(func(p *MistralVibeProvider) {
				p.executor = executor
			})
			state := workflow.NewConversationState("")

			result, err := provider.ExecuteConversation(context.Background(), state, "hello", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.State)
			assert.Equal(t, tt.wantOutput, result.Output)
			assert.Empty(t, result.State.SessionID)
			require.Len(t, result.State.Turns, 2)
			assert.Equal(t, tt.wantOutput, result.State.Turns[1].Content)
		})
	}
}

func TestMistralVibeMCPInjector_WritesTemporaryVibeHomeAndWhitelistsTools(t *testing.T) {
	sourceHome := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceHome, "config.toml"), []byte("default_agent = \"default\"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(sourceHome, ".env"), []byte("MISTRAL_API_KEY=test\n"), 0o600))
	t.Setenv("VIBE_HOME", sourceHome)

	provider := NewMistralVibeProvider()
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: true,
		PluginTools: []workflow.PluginToolExpose{
			{Plugin: "awf-plugin-time", Expose: []string{"time"}},
		},
	}

	args, opts, cleanup, err := provider.mistralVibeMCPInjector(
		context.Background(),
		[]string{"--prompt", "x"},
		cfg,
		"/tmp/awf mcp config.json",
		map[string]any{"output_format": "text"},
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.Contains(t, args, "--enabled-tools")
	assert.Contains(t, args, "awf-proxy_Bash")
	assert.Contains(t, args, "awf-proxy_awf-plugin-time_time")

	env, ok := opts[cliProviderEnvOptionKey].(map[string]string)
	require.True(t, ok)
	vibeHome := env["VIBE_HOME"]
	require.NotEmpty(t, vibeHome)
	assert.NotEqual(t, sourceHome, vibeHome)

	configData, err := os.ReadFile(filepath.Join(vibeHome, "config.toml"))
	require.NoError(t, err)
	config := string(configData)
	assert.Contains(t, config, "default_agent = \"default\"")
	assert.Contains(t, config, "[[mcp_servers]]")
	assert.Contains(t, config, "name = \"awf-proxy\"")
	assert.Contains(t, config, "transport = \"stdio\"")
	assert.Contains(t, config, "mcp-serve")
	assert.Contains(t, config, "--config=/tmp/awf mcp config.json")
	assert.Contains(t, config, "sampling_enabled = false")

	envData, err := os.ReadFile(filepath.Join(vibeHome, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "MISTRAL_API_KEY=test\n", string(envData))

	require.NoError(t, cleanup())
	_, statErr := os.Stat(vibeHome)
	assert.True(t, os.IsNotExist(statErr))
	assert.NoError(t, cleanup(), "cleanup should be idempotent")
}

func TestMistralVibeMCPInjector_WhitelistsPluginToolsWhenBuiltinsAreNotIntercepted(t *testing.T) {
	sourceHome := t.TempDir()
	t.Setenv("VIBE_HOME", sourceHome)
	provider := NewMistralVibeProvider()
	cfg := &workflow.MCPProxyConfig{
		Enable:            true,
		InterceptBuiltins: false,
		PluginTools: []workflow.PluginToolExpose{
			{Plugin: "awf-plugin-time", Expose: []string{"time"}},
		},
	}

	args, _, cleanup, err := provider.mistralVibeMCPInjector(
		context.Background(),
		[]string{"--prompt", "x"},
		cfg,
		"/tmp/mcp.json",
		map[string]any{"output_format": "text"},
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.NotContains(t, args, "awf-proxy_Bash")
	assert.Contains(t, args, "--enabled-tools")
	assert.Contains(t, args, "awf-proxy_awf-plugin-time_time")
	require.NoError(t, cleanup())
}

func TestMistralVibeMCPInjector_DoesNotMutateInputOptions(t *testing.T) {
	sourceHome := t.TempDir()
	t.Setenv("VIBE_HOME", sourceHome)
	provider := NewMistralVibeProvider()
	options := map[string]any{"output_format": "text"}

	_, opts, cleanup, err := provider.mistralVibeMCPInjector(
		context.Background(),
		[]string{"--prompt", "x"},
		&workflow.MCPProxyConfig{Enable: true},
		"/tmp/mcp.json",
		options,
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.NotSame(t, &options, &opts)
	assert.NotContains(t, options, cliProviderEnvOptionKey)
	assert.Contains(t, opts, cliProviderEnvOptionKey)
	require.NoError(t, cleanup())
}

func TestMistralVibeMCPConfigBlock_QuotesCommandArguments(t *testing.T) {
	block := mistralVibeMCPConfigBlock([]string{"/tmp/awf bin", "mcp-serve", "--config=/tmp/cfg with spaces.json"})

	assert.Contains(t, block, `command = "/tmp/awf bin"`)
	assert.Contains(t, block, `args = ["mcp-serve", "--config=/tmp/cfg with spaces.json"]`)
	assert.False(t, strings.Contains(block, "'"), "TOML block should use quoted strings generated by strconv.Quote")
}

func TestSanitizeMistralVibeConfig_RemovesExistingMCPServers(t *testing.T) {
	input := `
default_agent = "default"
mcp_servers = [
  { name = "existing", transport = "stdio", command = "old" },
]

[[mcp_servers]]
name = "other"
transport = "stdio"
command = "old"

[models.local]
provider = "mistral"
`

	got := sanitizeMistralVibeConfig(input)

	assert.Contains(t, got, `default_agent = "default"`)
	assert.Contains(t, got, `[models.local]`)
	assert.NotContains(t, got, `mcp_servers =`)
	assert.NotContains(t, got, `[[mcp_servers]]`)
	assert.NotContains(t, got, `command = "old"`)
}
