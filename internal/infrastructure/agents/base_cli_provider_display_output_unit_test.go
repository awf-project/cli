package agents

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseCLIProvider_Execute_WithParseStreamLineHook(t *testing.T) {
	tests := []struct {
		name             string
		parseStreamLine  LineExtractor
		rawOutput        string
		expectDisplayOut string
		expectResultErr  bool
	}{
		{
			name: "with parseStreamLine hook - extracts text",
			parseStreamLine: func(line []byte) string {
				// Simple parser: extract anything after "TEXT:"
				return "extracted text"
			},
			rawOutput:        "raw output line",
			expectDisplayOut: "extracted text",
			expectResultErr:  false,
		},
		{
			name:             "with nil parseStreamLine - empty DisplayOutput",
			parseStreamLine:  nil,
			rawOutput:        "raw output",
			expectDisplayOut: "",
			expectResultErr:  false,
		},
		{
			name: "parseStreamLine returning empty string - empty DisplayOutput",
			parseStreamLine: func(line []byte) string {
				return ""
			},
			rawOutput:        "raw output",
			expectDisplayOut: "",
			expectResultErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := mocks.NewMockCLIExecutor()
			mockExecutor.SetOutput([]byte(tt.rawOutput), []byte(""))

			hooks := cliProviderHooks{
				buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
					return []string{"--model", "test"}, nil
				},
				parseStreamLine: tt.parseStreamLine,
			}

			provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

			result, rawOut, err := provider.execute(context.Background(), "test prompt", map[string]any{}, nil, nil)

			if tt.expectResultErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.rawOutput, rawOut)
			assert.Equal(t, tt.expectDisplayOut, result.DisplayOutput)
		})
	}
}

func TestBaseCLIProvider_Execute_RawOutputPreserved(t *testing.T) {
	parseFunc := func(line []byte) string {
		return "filtered"
	}

	rawOutput := `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`
	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)
	result, _, err := provider.execute(context.Background(), "test prompt", map[string]any{}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output)
}

func TestBaseCLIProvider_ExecuteConversation_WithParseStreamLineHook(t *testing.T) {
	tests := []struct {
		name             string
		parseStreamLine  LineExtractor
		rawOutput        string
		expectDisplayOut string
		expectResultErr  bool
	}{
		{
			name: "conversation with parseStreamLine hook",
			parseStreamLine: func(line []byte) string {
				return "conversation response"
			},
			rawOutput:        "raw conversation output",
			expectDisplayOut: "conversation response",
			expectResultErr:  false,
		},
		{
			name:             "conversation with nil parseStreamLine",
			parseStreamLine:  nil,
			rawOutput:        "raw conversation output",
			expectDisplayOut: "",
			expectResultErr:  false,
		},
		{
			name: "conversation with parseStreamLine returning empty",
			parseStreamLine: func(line []byte) string {
				return ""
			},
			rawOutput:        "raw output",
			expectDisplayOut: "",
			expectResultErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := mocks.NewMockCLIExecutor()
			mockExecutor.SetOutput([]byte(tt.rawOutput), []byte(""))

			hooks := cliProviderHooks{
				buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
					return []string{"--model", "test"}, nil
				},
				extractSessionID: func(output string) (string, error) {
					return "", nil
				},
				parseStreamLine: tt.parseStreamLine,
			}

			provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

			state := workflow.NewConversationState("")
			result, rawOut, err := provider.executeConversation(context.Background(), state, "test prompt", map[string]any{}, nil, nil)

			if tt.expectResultErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.rawOutput, rawOut)
			assert.Equal(t, tt.expectDisplayOut, result.DisplayOutput)
		})
	}
}

func TestBaseCLIProvider_ExecuteConversation_RawOutputPreserved(t *testing.T) {
	parseFunc := func(line []byte) string {
		return "filtered conversation"
	}

	rawOutput := `{"session_id":"sess123"}`
	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "sess123", nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)
	state := workflow.NewConversationState("")
	result, _, err := provider.executeConversation(context.Background(), state, "test prompt", map[string]any{}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output)
}

func TestBaseCLIProvider_Execute_WithWriter_AndParseStreamLine(t *testing.T) {
	parseFunc := func(line []byte) string {
		return "filtered"
	}

	rawOutput := "raw line"
	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	result, _, err := provider.execute(context.Background(), "test prompt", map[string]any{}, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, parseFunc([]byte(rawOutput)), result.DisplayOutput)
}

func TestBaseCLIProvider_Execute_MultilineOutput_WithParseStreamLine(t *testing.T) {
	parseFunc := func(line []byte) string {
		// Extract only lines containing "KEEP"
		if bytes.Contains(line, []byte("KEEP")) {
			return string(line)
		}
		return ""
	}

	rawOutput := `KEEP this line
SKIP this line
KEEP this too`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)
	result, _, err := provider.execute(context.Background(), "test prompt", map[string]any{}, nil, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output)
}

func TestBaseCLIProvider_Execute_TimestampsSet(t *testing.T) {
	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte("test output"), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: func(line []byte) string {
			return "parsed"
		},
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	beforeExec := time.Now()
	result, _, err := provider.execute(context.Background(), "test prompt", map[string]any{}, nil, nil)
	afterExec := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.StartedAt.After(beforeExec.Add(-time.Second)))
	assert.True(t, result.CompletedAt.Before(afterExec.Add(time.Second)))
	assert.True(t, result.CompletedAt.After(result.StartedAt))
}

func TestBaseCLIProvider_ExecuteConversation_TimestampsSet(t *testing.T) {
	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte("test conversation output"), []byte(""))

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "sess123", nil
		},
		parseStreamLine: func(line []byte) string {
			return "parsed"
		},
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)
	state := workflow.NewConversationState("")

	beforeExec := time.Now()
	result, _, err := provider.executeConversation(context.Background(), state, "test prompt", map[string]any{}, nil, nil)
	afterExec := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.StartedAt.After(beforeExec.Add(-time.Second)))
	assert.True(t, result.CompletedAt.Before(afterExec.Add(time.Second)))
	assert.True(t, result.CompletedAt.After(result.StartedAt))
}

func TestCLIProviderHooks_ParseStreamLineField(t *testing.T) {
	tests := []struct {
		name         string
		parseFunc    LineExtractor
		expectNil    bool
		callableFunc bool
	}{
		{
			name:         "parseStreamLine can be nil",
			parseFunc:    nil,
			expectNil:    true,
			callableFunc: false,
		},
		{
			name: "parseStreamLine can be set to a function",
			parseFunc: func(line []byte) string {
				return "test"
			},
			expectNil:    false,
			callableFunc: true,
		},
		{
			name:         "parseStreamLine with empty function",
			parseFunc:    func(line []byte) string { return "" },
			expectNil:    false,
			callableFunc: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hooks := cliProviderHooks{
				parseStreamLine: tt.parseFunc,
			}

			if tt.expectNil {
				assert.Nil(t, hooks.parseStreamLine)
			} else {
				assert.NotNil(t, hooks.parseStreamLine)
			}

			if tt.callableFunc && tt.parseFunc != nil {
				result := tt.parseFunc([]byte("test input"))
				assert.NotEmpty(t, result)
			}
		})
	}
}

// T008: Scenario 1 - execute() + parseStreamLine + output_format=text
// Should wrap stdout with filter, populate DisplayOutput, preserve raw Output
func TestBaseCLIProvider_Execute_Scenario1_TextFormatWithFilter(t *testing.T) {
	parseFunc := func(line []byte) string {
		// Extract only lines containing "RESPONSE"
		if bytes.Contains(line, []byte("RESPONSE")) {
			return "extracted response"
		}
		return ""
	}

	rawOutput := `{"type":"content_block_delta"}
{"text":"RESPONSE: hello"}
{"type":"content_block_end"}`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	options := map[string]any{"output_format": "text"}
	result, _, err := provider.execute(context.Background(), "test prompt", options, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output, "Output should contain raw NDJSON")
	assert.NotEmpty(t, result.DisplayOutput, "DisplayOutput should be populated with filtered text")
	assert.True(t, strings.Contains(result.DisplayOutput, "extracted response"), "DisplayOutput should contain extracted text")
}

// T008: Scenario 2 - execute() + parseStreamLine + output_format=json
// Should NOT wrap stdout, keep DisplayOutput empty, preserve raw Output
func TestBaseCLIProvider_Execute_Scenario2_JSONFormatNoFilter(t *testing.T) {
	parseFunc := func(line []byte) string {
		return "should not be called"
	}

	rawOutput := `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	options := map[string]any{"output_format": "json"}
	result, _, err := provider.execute(context.Background(), "test prompt", options, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output, "Output should contain raw NDJSON")
	assert.Empty(t, result.DisplayOutput, "DisplayOutput should be empty when output_format is json")
}

// T008: Scenario 3 - execute() + nil parseStreamLine
// Should NOT wrap stdout, keep DisplayOutput empty, preserve raw Output
func TestBaseCLIProvider_Execute_Scenario3_NilParserNoFilter(t *testing.T) {
	rawOutput := `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildExecuteArgs: func(prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		parseStreamLine: nil,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	options := map[string]any{}
	result, _, err := provider.execute(context.Background(), "test prompt", options, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output, "Output should contain raw NDJSON")
	assert.Empty(t, result.DisplayOutput, "DisplayOutput should be empty when parseStreamLine is nil")
}

// T008: Scenario 4 - executeConversation() + parseStreamLine + output_format=text
// Should wrap stdout with filter, populate DisplayOutput, preserve raw Output
func TestBaseCLIProvider_ExecuteConversation_Scenario1_TextFormatWithFilter(t *testing.T) {
	parseFunc := func(line []byte) string {
		if bytes.Contains(line, []byte("init")) {
			return "session initialized"
		}
		return ""
	}

	rawOutput := `{"type":"init","session_id":"sess123"}
{"type":"message","text":"response"}`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "sess123", nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	state := workflow.NewConversationState("")
	options := map[string]any{"output_format": "text"}
	result, _, err := provider.executeConversation(context.Background(), state, "test prompt", options, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output, "Output should contain raw NDJSON")
	assert.NotEmpty(t, result.DisplayOutput, "DisplayOutput should be populated with filtered text")
}

// T008: Scenario 5 - executeConversation() + parseStreamLine + output_format=json
// Should NOT wrap stdout, keep DisplayOutput empty, preserve raw Output
func TestBaseCLIProvider_ExecuteConversation_Scenario2_JSONFormatNoFilter(t *testing.T) {
	parseFunc := func(line []byte) string {
		return "should not be called"
	}

	rawOutput := `{"type":"init","session_id":"sess123"}`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "sess123", nil
		},
		parseStreamLine: parseFunc,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	state := workflow.NewConversationState("")
	options := map[string]any{"output_format": "json"}
	result, _, err := provider.executeConversation(context.Background(), state, "test prompt", options, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output, "Output should contain raw NDJSON")
	assert.Empty(t, result.DisplayOutput, "DisplayOutput should be empty when output_format is json")
}

// T008: Scenario 6 - executeConversation() + nil parseStreamLine
// Should NOT wrap stdout, keep DisplayOutput empty, preserve raw Output
func TestBaseCLIProvider_ExecuteConversation_Scenario3_NilParserNoFilter(t *testing.T) {
	rawOutput := `{"type":"init","session_id":"sess123"}`

	mockExecutor := mocks.NewMockCLIExecutor()
	mockExecutor.SetOutput([]byte(rawOutput), []byte(""))

	hooks := cliProviderHooks{
		buildConversationArgs: func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
			return []string{"--model", "test"}, nil
		},
		extractSessionID: func(output string) (string, error) {
			return "sess123", nil
		},
		parseStreamLine: nil,
	}

	provider := newBaseCLIProvider("test", "test-bin", mockExecutor, logger.NopLogger{}, hooks)

	var stdoutBuf bytes.Buffer
	state := workflow.NewConversationState("")
	options := map[string]any{}
	result, _, err := provider.executeConversation(context.Background(), state, "test prompt", options, &stdoutBuf, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, rawOutput, result.Output, "Output should contain raw NDJSON")
	assert.Empty(t, result.DisplayOutput, "DisplayOutput should be empty when parseStreamLine is nil")
}

// Test wantsRawDisplay helper function behavior
func TestWantsRawDisplay_Helper(t *testing.T) {
	tests := []struct {
		name     string
		options  map[string]any
		expected bool
	}{
		{
			name:     "nil options returns false",
			options:  nil,
			expected: false,
		},
		{
			name:     "empty options returns false",
			options:  map[string]any{},
			expected: false,
		},
		{
			name:     "output_format=json returns true",
			options:  map[string]any{"output_format": "json"},
			expected: true,
		},
		{
			name:     "output_format=text returns false",
			options:  map[string]any{"output_format": "text"},
			expected: false,
		},
		{
			name:     "output_format=none returns false",
			options:  map[string]any{"output_format": "none"},
			expected: false,
		},
		{
			name:     "output_format not a string returns false",
			options:  map[string]any{"output_format": 123},
			expected: false,
		},
		{
			name:     "missing output_format returns false",
			options:  map[string]any{"other_key": "value"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wantsRawDisplay(tt.options)
			assert.Equal(t, tt.expected, result)
		})
	}
}
