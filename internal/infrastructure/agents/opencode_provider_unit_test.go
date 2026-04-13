package agents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: C025 - T008 - Unit Tests for OpenCodeProvider (WITHOUT integration build tag)
// These tests use MockCLIExecutor to avoid external CLI dependencies

func TestOpenCodeProvider_Execute_Success(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
		wantOutput string
	}{
		{
			name:       "simple prompt",
			prompt:     "Create a function",
			mockStdout: []byte("function created"),
			wantOutput: "function created",
		},
		{
			name:       "prompt with framework option",
			prompt:     "test",
			options:    map[string]any{"framework": "react"},
			mockStdout: []byte("response"),
			wantOutput: "response",
		},
		{
			name:       "prompt with verbose option",
			prompt:     "test",
			options:    map[string]any{"verbose": true},
			mockStdout: []byte("verbose response"),
			wantOutput: "verbose response",
		},
		{
			name:       "prompt with output_dir option",
			prompt:     "test",
			options:    map[string]any{"output_dir": "/tmp/output"},
			mockStdout: []byte("output written"),
			wantOutput: "output written",
		},
		{
			name:       "large output",
			prompt:     "generate",
			mockStdout: []byte("This is a very long output " + string(make([]byte, 1000))),
			wantOutput: "This is a very long output " + string(make([]byte, 1000)),
		},
		{
			name:       "empty output",
			prompt:     "silent",
			mockStdout: []byte(""),
			wantOutput: " ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "opencode", result.Provider)
			assert.Equal(t, tt.wantOutput, result.Output)
			// Token estimation is ~4 chars per token
			expectedTokens := len(tt.wantOutput) / 4
			assert.Equal(t, expectedTokens, result.Tokens)
			assert.False(t, result.StartedAt.IsZero())
			assert.False(t, result.CompletedAt.IsZero())
			assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
		})
	}
}

func TestOpenCodeProvider_Execute_WithOptions(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		options    map[string]any
		mockStdout []byte
	}{
		{
			name:       "framework option",
			prompt:     "test",
			options:    map[string]any{"framework": "react"},
			mockStdout: []byte("ok"),
		},
		{
			name:       "verbose option",
			prompt:     "test",
			options:    map[string]any{"verbose": true},
			mockStdout: []byte("ok"),
		},
		{
			name:       "output_dir option",
			prompt:     "test",
			options:    map[string]any{"output_dir": "/tmp/test"},
			mockStdout: []byte("ok"),
		},
		{
			name:       "multiple options",
			prompt:     "test",
			options:    map[string]any{"framework": "vue", "verbose": true, "output_dir": "/tmp"},
			mockStdout: []byte("ok"),
		},
		{
			name:       "no options",
			prompt:     "test",
			options:    nil,
			mockStdout: []byte("ok"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "opencode", result.Provider)

			// Verify the CLI call captured the correct arguments
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, "opencode", calls[0].Name)
			assert.Contains(t, calls[0].Args, "run")
			assert.Contains(t, calls[0].Args, tt.prompt)
		})
	}
}

func TestOpenCodeProvider_Execute_EmptyPrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr string
	}{
		{
			name:    "empty string",
			prompt:  "",
			wantErr: "prompt cannot be empty",
		},
		{
			name:    "whitespace only",
			prompt:  "   \t\n  ",
			wantErr: "prompt cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, nil, nil, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
		wantErr string
	}{
		{
			name:    "invalid output_dir type",
			prompt:  "test",
			options: map[string]any{"output_dir": 123},
			wantErr: "output_dir must be a string",
		},
		{
			name:    "invalid verbose type",
			prompt:  "test",
			options: map[string]any{"verbose": "true"},
			wantErr: "verbose must be a boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_ContextErrors(t *testing.T) {
	tests := []struct {
		name    string
		ctxFunc func() context.Context
		wantErr string
	}{
		{
			name: "canceled context",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantErr: "context canceled",
		},
		{
			name: "deadline exceeded context",
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
				defer cancel()
				return ctx
			},
			wantErr: "deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(tt.ctxFunc(), "test prompt", nil, nil, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_CLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		mockErr error
		wantErr string
	}{
		{
			name:    "command not found",
			mockErr: errors.New("executable file not found"),
			wantErr: "opencode execution failed",
		},
		{
			name:    "permission denied",
			mockErr: errors.New("permission denied"),
			wantErr: "opencode execution failed",
		},
		{
			name:    "generic error",
			mockErr: errors.New("something went wrong"),
			wantErr: "opencode execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetError(tt.mockErr)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestOpenCodeProvider_Execute_StdoutStderr(t *testing.T) {
	tests := []struct {
		name       string
		mockStdout []byte
		mockStderr []byte
		wantOutput string
	}{
		{
			name:       "stdout only",
			mockStdout: []byte("output"),
			mockStderr: nil,
			wantOutput: "output",
		},
		{
			name:       "stderr only",
			mockStdout: nil,
			mockStderr: []byte("error"),
			wantOutput: "error",
		},
		{
			name:       "both stdout and stderr",
			mockStdout: []byte("output"),
			mockStderr: []byte("warning"),
			wantOutput: "outputwarning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockStdout, tt.mockStderr)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantOutput, result.Output)
		})
	}
}

func TestOpenCodeProvider_Execute_TokenEstimation(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantTokens int
	}{
		{
			name:       "empty output",
			output:     "",
			wantTokens: 0,
		},
		{
			name:       "short output",
			output:     "test",
			wantTokens: 1,
		},
		{
			name:       "medium output",
			output:     "This is a test output",
			wantTokens: 5,
		},
		{
			name:       "long output",
			output:     string(make([]byte, 400)),
			wantTokens: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(tt.output), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTokens, result.Tokens)
		})
	}
}

func TestOpenCodeProvider_Execute_Timestamps(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test output"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	before := time.Now()
	result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.StartedAt.IsZero())
	assert.False(t, result.CompletedAt.IsZero())
	assert.True(t, result.StartedAt.After(before) || result.StartedAt.Equal(before))
	assert.True(t, result.StartedAt.Before(after) || result.StartedAt.Equal(after))
	assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
	assert.True(t, result.CompletedAt.Before(after) || result.CompletedAt.Equal(after))
}

func TestOpenCodeProvider_Execute_ProviderName(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte("test output"), nil)
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "opencode", result.Provider)
}

func TestOpenCodeProvider_Execute_JSONDetection(t *testing.T) {
	tests := []struct {
		name         string
		mockOutput   []byte
		wantResponse map[string]any
	}{
		{
			name:         "valid json object",
			mockOutput:   []byte(`{"status":"ok"}`),
			wantResponse: map[string]any{"status": "ok"},
		},
		{
			name:         "json with nested objects",
			mockOutput:   []byte(`{"result":{"count":42,"items":["a","b"]}}`),
			wantResponse: map[string]any{"result": map[string]any{"count": float64(42), "items": []any{"a", "b"}}},
		},
		{
			name:         "non-json response",
			mockOutput:   []byte("plain text"),
			wantResponse: nil,
		},
		{
			name:         "json with whitespace",
			mockOutput:   []byte("\n  {\"key\":\"value\"}  \n"),
			wantResponse: map[string]any{"key": "value"},
		},
		{
			name:         "malformed json",
			mockOutput:   []byte(`{"incomplete`),
			wantResponse: nil,
		},
		{
			name:         "json array (not detected)",
			mockOutput:   []byte(`["item1","item2"]`),
			wantResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput(tt.mockOutput, nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			result, err := provider.Execute(context.Background(), "test prompt", nil, nil, nil)

			require.NoError(t, err)
			require.NotNil(t, result)
			if tt.wantResponse != nil {
				assert.Equal(t, tt.wantResponse, result.Response)
			} else {
				assert.Nil(t, result.Response)
			}
		})
	}
}

func TestOpenCodeProvider_Name(t *testing.T) {
	provider := NewOpenCodeProvider()
	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_Validate_Success(t *testing.T) {
	provider := NewOpenCodeProvider()
	err := provider.Validate()
	// This test depends on system state (whether opencode CLI is installed)
	// We just verify it doesn't panic and returns either nil or an error
	if err != nil {
		assert.Contains(t, err.Error(), "opencode CLI not found")
	}
}

func TestOpenCodeProvider_NewOpenCodeProvider_DefaultExecutor(t *testing.T) {
	provider := NewOpenCodeProvider()
	require.NotNil(t, provider)
	assert.NotNil(t, provider.executor)
	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_NewOpenCodeProviderWithOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

	require.NotNil(t, provider)
	assert.Equal(t, mockExec, provider.executor)
	assert.Equal(t, "opencode", provider.Name())
}

func TestOpenCodeProvider_Execute_CLIArguments(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		options     map[string]any
		wantCmdName string
		wantCmdArgs []string
	}{
		{
			name:        "basic prompt with run subcommand and format flag",
			prompt:      "test prompt",
			options:     nil,
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test prompt", "--format", "json"},
		},
		{
			name:        "with framework option and format flag",
			prompt:      "test",
			options:     map[string]any{"framework": "react"},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--format", "json", "--framework", "react"},
		},
		{
			name:        "with verbose option and format flag",
			prompt:      "test",
			options:     map[string]any{"verbose": true},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--format", "json", "--verbose"},
		},
		{
			name:        "with output_dir option and format flag",
			prompt:      "test",
			options:     map[string]any{"output_dir": "/tmp/out"},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--format", "json", "--output", "/tmp/out"},
		},
		{
			name:        "with all options and format flag",
			prompt:      "test",
			options:     map[string]any{"framework": "vue", "verbose": true, "output_dir": "/tmp"},
			wantCmdName: "opencode",
			wantCmdArgs: []string{"run", "test", "--format", "json", "--framework", "vue", "--verbose", "--output", "/tmp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("ok"), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			// Verify the CLI arguments
			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)
			assert.Equal(t, tt.wantCmdName, calls[0].Name)
			assert.Equal(t, tt.wantCmdArgs, calls[0].Args)
		})
	}
}

// T006: Verify --format json flag is always passed to OpenCode Execute (FR-005)
func TestOpenCodeProvider_Execute_FormatJSONFlag(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		options map[string]any
	}{
		{
			name:    "no options",
			prompt:  "test",
			options: nil,
		},
		{
			name:    "with model option",
			prompt:  "test",
			options: map[string]any{"model": "gpt-4o"},
		},
		{
			name:    "with framework option",
			prompt:  "test",
			options: map[string]any{"framework": "react"},
		},
		{
			name:    "with verbose option",
			prompt:  "test",
			options: map[string]any{"verbose": true},
		},
		{
			name:    "with multiple options",
			prompt:  "test",
			options: map[string]any{"model": "gpt-4", "framework": "vue", "verbose": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"status":"ok"}`), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			assert.Contains(t, calls[0].Args, "--format")
			formatIdx := -1
			for i, arg := range calls[0].Args {
				if arg == "--format" {
					formatIdx = i
					break
				}
			}
			require.NotEqual(t, -1, formatIdx, "expected --format flag to be present")
			require.Less(t, formatIdx+1, len(calls[0].Args), "expected --format to have a value")
			assert.Equal(t, "json", calls[0].Args[formatIdx+1])
		})
	}
}

// T006: Verify --model flag is passed when model option is provided (FR-006)
func TestOpenCodeProvider_Execute_ModelFlag(t *testing.T) {
	tests := []struct {
		name         string
		prompt       string
		options      map[string]any
		wantHasModel bool
		wantModel    string
	}{
		{
			name:         "model provided",
			prompt:       "test",
			options:      map[string]any{"model": "gpt-4o"},
			wantHasModel: true,
			wantModel:    "gpt-4o",
		},
		{
			name:         "model provided as latest",
			prompt:       "test",
			options:      map[string]any{"model": "gpt-4o-latest"},
			wantHasModel: true,
			wantModel:    "gpt-4o-latest",
		},
		{
			name:         "no model option",
			prompt:       "test",
			options:      nil,
			wantHasModel: false,
		},
		{
			name:         "empty options",
			prompt:       "test",
			options:      map[string]any{},
			wantHasModel: false,
		},
		{
			name:         "model with other options",
			prompt:       "test",
			options:      map[string]any{"model": "gpt-3.5-turbo", "framework": "react"},
			wantHasModel: true,
			wantModel:    "gpt-3.5-turbo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte("ok"), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			_, err := provider.Execute(context.Background(), tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			if tt.wantHasModel {
				require.Contains(t, calls[0].Args, "--model", "expected --model flag")
				modelIdx := -1
				for i, arg := range calls[0].Args {
					if arg == "--model" {
						modelIdx = i
						break
					}
				}
				require.NotEqual(t, -1, modelIdx)
				require.Less(t, modelIdx+1, len(calls[0].Args))
				assert.Equal(t, tt.wantModel, calls[0].Args[modelIdx+1])
			} else {
				assert.NotContains(t, calls[0].Args, "--model", "expected no --model flag")
			}
		})
	}
}

// T006: Verify --format json and --model work correctly in ExecuteConversation (FR-006)
func TestOpenCodeProvider_ExecuteConversation_FormatAndModelFlags(t *testing.T) {
	tests := []struct {
		name          string
		prompt        string
		options       map[string]any
		wantHasFormat bool
		wantHasModel  bool
		wantModel     string
		sessionID     string
	}{
		{
			name:          "first turn with model",
			prompt:        "test",
			options:       map[string]any{"model": "gpt-4o"},
			wantHasFormat: true,
			wantHasModel:  true,
			wantModel:     "gpt-4o",
			sessionID:     "",
		},
		{
			name:          "first turn without model",
			prompt:        "test",
			options:       nil,
			wantHasFormat: true,
			wantHasModel:  false,
			sessionID:     "",
		},
		{
			name:          "resume with model",
			prompt:        "test",
			options:       map[string]any{"model": "gpt-3.5-turbo"},
			wantHasFormat: true,
			wantHasModel:  true,
			wantModel:     "gpt-3.5-turbo",
			sessionID:     "opencode-12345",
		},
		{
			name:          "resume without model",
			prompt:        "test",
			options:       map[string]any{},
			wantHasFormat: true,
			wantHasModel:  false,
			sessionID:     "opencode-67890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`Session: opencode-12345\n{"status":"ok"}`), nil)
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(mockExec))

			state := &workflow.ConversationState{
				Turns:     []workflow.Turn{},
				SessionID: tt.sessionID,
			}

			_, err := provider.ExecuteConversation(context.Background(), state, tt.prompt, tt.options, nil, nil)
			require.NoError(t, err)

			calls := mockExec.GetCalls()
			require.Len(t, calls, 1)

			if tt.wantHasFormat {
				assert.Contains(t, calls[0].Args, "--format")
				formatIdx := -1
				for i, arg := range calls[0].Args {
					if arg == "--format" {
						formatIdx = i
						break
					}
				}
				require.NotEqual(t, -1, formatIdx)
				require.Less(t, formatIdx+1, len(calls[0].Args))
				assert.Equal(t, "json", calls[0].Args[formatIdx+1])
			}

			if tt.wantHasModel {
				require.Contains(t, calls[0].Args, "--model")
				modelIdx := -1
				for i, arg := range calls[0].Args {
					if arg == "--model" {
						modelIdx = i
						break
					}
				}
				require.NotEqual(t, -1, modelIdx)
				require.Less(t, modelIdx+1, len(calls[0].Args))
				assert.Equal(t, tt.wantModel, calls[0].Args[modelIdx+1])
			} else {
				assert.NotContains(t, calls[0].Args, "--model")
			}
		})
	}
}

// T013: Verify debug log is emitted when dangerously_skip_permissions is present (FR-009)
func TestOpenCodeProvider_Execute_DangerouslySkipPermissions_DebugLog(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		hasFlag bool
	}{
		{
			name:    "dangerously_skip_permissions true",
			options: map[string]any{"dangerously_skip_permissions": true},
			hasFlag: true,
		},
		{
			name:    "dangerously_skip_permissions false",
			options: map[string]any{"dangerously_skip_permissions": false},
			hasFlag: false,
		},
		{
			name:    "no dangerously_skip_permissions",
			options: nil,
			hasFlag: false,
		},
		{
			name:    "with other options but no dangerously_skip_permissions",
			options: map[string]any{"model": "gpt-4o", "framework": "react"},
			hasFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := mocks.NewMockCLIExecutor()
			mockExec.SetOutput([]byte(`{"status":"ok"}`), nil)
			mockLogger := mocks.NewMockLogger()

			provider := NewOpenCodeProviderWithOptions(
				WithOpenCodeExecutor(mockExec),
				WithOpenCodeLogger(mockLogger),
			)

			_, err := provider.Execute(context.Background(), "test prompt", tt.options, nil, nil)
			require.NoError(t, err)

			debugMessages := mockLogger.GetMessagesByLevel("DEBUG")

			if tt.hasFlag {
				require.Greater(t, len(debugMessages), 0, "expected at least one debug message when dangerously_skip_permissions is present")
				foundMsg := false
				for _, msg := range debugMessages {
					if strings.Contains(msg.Msg, "dangerously_skip_permissions") && strings.Contains(msg.Msg, "OpenCode") {
						foundMsg = true
						break
					}
				}
				assert.True(t, foundMsg, "expected debug message mentioning dangerously_skip_permissions and OpenCode")
			} else {
				// When flag is not present, should not have any dangerously_skip_permissions debug messages
				for _, msg := range debugMessages {
					assert.NotContains(t, msg.Msg, "dangerously_skip_permissions", "should not log dangerously_skip_permissions when not provided")
				}
			}
		})
	}
}

// T013: Verify debug log content when dangerously_skip_permissions is present (FR-009)
func TestOpenCodeProvider_Execute_DangerouslySkipPermissions_LogContent(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"result":"code"}`), nil)
	mockLogger := mocks.NewMockLogger()

	provider := NewOpenCodeProviderWithOptions(
		WithOpenCodeExecutor(mockExec),
		WithOpenCodeLogger(mockLogger),
	)

	options := map[string]any{
		"dangerously_skip_permissions": true,
	}

	_, err := provider.Execute(context.Background(), "Generate code", options, nil, nil)
	require.NoError(t, err)

	messages := mockLogger.GetMessages()
	require.Greater(t, len(messages), 0, "expected at least one log message")

	// Find the debug message about dangerously_skip_permissions
	var debugMsg *mocks.LogMessage
	for i := range messages {
		if messages[i].Level == "DEBUG" && strings.Contains(messages[i].Msg, "dangerously_skip_permissions") {
			debugMsg = &messages[i]
			break
		}
	}

	require.NotNil(t, debugMsg, "expected a DEBUG level message about dangerously_skip_permissions")
	assert.Contains(t, debugMsg.Msg, "not supported", "message should indicate the option is not supported")
	assert.Contains(t, debugMsg.Msg, "ignored", "message should indicate the option will be ignored")
	assert.Contains(t, debugMsg.Msg, "OpenCode", "message should mention OpenCode")
}

// T013: Verify ExecuteConversation also emits debug log for dangerously_skip_permissions (FR-009)
func TestOpenCodeProvider_ExecuteConversation_DangerouslySkipPermissions_DebugLog(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"status":"ok","session_id":"opencode-123"}`), nil)
	mockLogger := mocks.NewMockLogger()

	provider := NewOpenCodeProviderWithOptions(
		WithOpenCodeExecutor(mockExec),
		WithOpenCodeLogger(mockLogger),
	)

	state := &workflow.ConversationState{
		Turns:     []workflow.Turn{},
		SessionID: "",
	}

	options := map[string]any{
		"dangerously_skip_permissions": true,
	}

	_, err := provider.ExecuteConversation(context.Background(), state, "Generate code", options, nil, nil)
	require.NoError(t, err)

	debugMessages := mockLogger.GetMessagesByLevel("DEBUG")
	require.Greater(t, len(debugMessages), 0, "expected at least one debug message in ExecuteConversation")

	foundMsg := false
	for _, msg := range debugMessages {
		if strings.Contains(msg.Msg, "dangerously_skip_permissions") && strings.Contains(msg.Msg, "OpenCode") {
			foundMsg = true
			break
		}
	}
	assert.True(t, foundMsg, "expected debug message about dangerously_skip_permissions in ExecuteConversation")
}

// T013: Verify no dangerously_skip_permissions flag is passed to OpenCode CLI (FR-009)
func TestOpenCodeProvider_Execute_DangerouslySkipPermissions_NoFlag(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"status":"ok"}`), nil)

	provider := NewOpenCodeProviderWithOptions(
		WithOpenCodeExecutor(mockExec),
	)

	options := map[string]any{
		"dangerously_skip_permissions": true,
	}

	_, err := provider.Execute(context.Background(), "test", options, nil, nil)
	require.NoError(t, err)

	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)

	for _, arg := range calls[0].Args {
		assert.NotEqual(t, "--dangerously-skip-permissions", arg, "should not pass dangerously_skip_permissions flag to CLI")
		assert.NotEqual(t, "--dangerously_skip_permissions", arg, "should not pass dangerously_skip_permissions flag to CLI")
		assert.NotEqual(t, "--yolo", arg, "should not pass --yolo flag (Codex specific)")
		assert.NotEqual(t, "--approval-mode", arg, "should not pass --approval-mode flag (Gemini specific)")
	}
}

// T013: Verify dangerously_skip_permissions with other options still logs debug (FR-009)
func TestOpenCodeProvider_Execute_DangerouslySkipPermissions_WithOtherOptions(t *testing.T) {
	mockExec := mocks.NewMockCLIExecutor()
	mockExec.SetOutput([]byte(`{"status":"ok"}`), nil)
	mockLogger := mocks.NewMockLogger()

	provider := NewOpenCodeProviderWithOptions(
		WithOpenCodeExecutor(mockExec),
		WithOpenCodeLogger(mockLogger),
	)

	options := map[string]any{
		"dangerously_skip_permissions": true,
		"model":                        "gpt-4o",
		"framework":                    "react",
		"verbose":                      true,
	}

	_, err := provider.Execute(context.Background(), "test", options, nil, nil)
	require.NoError(t, err)

	debugMessages := mockLogger.GetMessagesByLevel("DEBUG")
	require.Greater(t, len(debugMessages), 0, "expected debug message even with other options present")

	var found bool
	for _, msg := range debugMessages {
		if strings.Contains(msg.Msg, "dangerously_skip_permissions") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected debug message about dangerously_skip_permissions")

	// Verify other options are still passed to CLI
	calls := mockExec.GetCalls()
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0].Args, "--model")
	assert.Contains(t, calls[0].Args, "gpt-4o")
	assert.Contains(t, calls[0].Args, "--framework")
	assert.Contains(t, calls[0].Args, "react")
	assert.Contains(t, calls[0].Args, "--verbose")
}
