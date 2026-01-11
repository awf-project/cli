package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "empty_string",
			output:   "",
			expected: 0,
		},
		{
			name:     "short_string",
			output:   "test", // 4 chars = 1 token
			expected: 1,
		},
		{
			name:     "medium_string",
			output:   "hello world test", // 16 chars = 4 tokens
			expected: 4,
		},
		{
			name:     "long_string",
			output:   "This is a longer string for testing token estimation accurately.", // 64 chars
			expected: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCloneState(t *testing.T) {
	tests := []struct {
		name  string
		state *workflow.ConversationState
	}{
		{
			name:  "nil_state",
			state: nil,
		},
		{
			name: "empty_state",
			state: &workflow.ConversationState{
				Turns: []workflow.Turn{},
			},
		},
		{
			name: "state_with_turns",
			state: &workflow.ConversationState{
				Turns: []workflow.Turn{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there"},
				},
				TotalTurns:  2,
				TotalTokens: 100,
				StoppedBy:   "user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := cloneState(tt.state)

			if tt.state == nil {
				assert.Nil(t, cloned)
				return
			}

			require.NotNil(t, cloned)
			assert.NotSame(t, tt.state, cloned, "Should be different pointer")
			assert.Equal(t, len(tt.state.Turns), len(cloned.Turns))
			assert.Equal(t, tt.state.TotalTurns, cloned.TotalTurns)
			assert.Equal(t, tt.state.TotalTokens, cloned.TotalTokens)
			assert.Equal(t, tt.state.StoppedBy, cloned.StoppedBy)

			// Verify slice is copied, not shared
			if len(tt.state.Turns) > 0 {
				assert.NotSame(t, &tt.state.Turns[0], &cloned.Turns[0])
			}
		})
	}
}

func TestGetStringOption(t *testing.T) {
	tests := []struct {
		name      string
		options   map[string]any
		key       string
		wantValue string
		wantOK    bool
	}{
		{
			name:      "nil_options",
			options:   nil,
			key:       "model",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "empty_options",
			options:   map[string]any{},
			key:       "model",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "key_not_found",
			options:   map[string]any{"other": "value"},
			key:       "model",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "key_found_correct_type",
			options:   map[string]any{"model": "haiku"},
			key:       "model",
			wantValue: "haiku",
			wantOK:    true,
		},
		{
			name:      "key_found_wrong_type_int",
			options:   map[string]any{"model": 123},
			key:       "model",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "key_found_wrong_type_nil",
			options:   map[string]any{"model": nil},
			key:       "model",
			wantValue: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := getStringOption(tt.options, tt.key)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestGetIntOption(t *testing.T) {
	tests := []struct {
		name      string
		options   map[string]any
		key       string
		wantValue int
		wantOK    bool
	}{
		{
			name:      "nil_options",
			options:   nil,
			key:       "max_tokens",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "key_found_correct_type",
			options:   map[string]any{"max_tokens": 1000},
			key:       "max_tokens",
			wantValue: 1000,
			wantOK:    true,
		},
		{
			name:      "key_found_wrong_type_string",
			options:   map[string]any{"max_tokens": "1000"},
			key:       "max_tokens",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "key_found_wrong_type_float",
			options:   map[string]any{"max_tokens": 1000.5},
			key:       "max_tokens",
			wantValue: 0,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := getIntOption(tt.options, tt.key)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestGetFloatOption(t *testing.T) {
	tests := []struct {
		name      string
		options   map[string]any
		key       string
		wantValue float64
		wantOK    bool
	}{
		{
			name:      "nil_options",
			options:   nil,
			key:       "temperature",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "key_found_correct_type",
			options:   map[string]any{"temperature": 0.7},
			key:       "temperature",
			wantValue: 0.7,
			wantOK:    true,
		},
		{
			name:      "key_found_wrong_type_int",
			options:   map[string]any{"temperature": 1},
			key:       "temperature",
			wantValue: 0,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := getFloatOption(tt.options, tt.key)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestGetBoolOption(t *testing.T) {
	tests := []struct {
		name      string
		options   map[string]any
		key       string
		wantValue bool
		wantOK    bool
	}{
		{
			name:      "nil_options",
			options:   nil,
			key:       "quiet",
			wantValue: false,
			wantOK:    false,
		},
		{
			name:      "key_found_true",
			options:   map[string]any{"quiet": true},
			key:       "quiet",
			wantValue: true,
			wantOK:    true,
		},
		{
			name:      "key_found_false",
			options:   map[string]any{"quiet": false},
			key:       "quiet",
			wantValue: false,
			wantOK:    true,
		},
		{
			name:      "key_found_wrong_type_string",
			options:   map[string]any{"quiet": "true"},
			key:       "quiet",
			wantValue: false,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, ok := getBoolOption(tt.options, tt.key)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestValidatePrompt(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		wantErr bool
	}{
		{
			name:    "valid_prompt",
			prompt:  "Test prompt",
			wantErr: false,
		},
		{
			name:    "empty_prompt",
			prompt:  "",
			wantErr: true,
		},
		{
			name:    "whitespace_only",
			prompt:  "   \t\n",
			wantErr: true,
		},
		{
			name:    "prompt_with_whitespace",
			prompt:  "  Test  ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrompt(tt.prompt)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "prompt")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateContext(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		providerName string
		wantErr      bool
	}{
		{
			name:         "valid_context",
			ctx:          context.Background(),
			providerName: "claude",
			wantErr:      false,
		},
		{
			name: "cancelled_context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			providerName: "claude",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContext(tt.ctx, tt.providerName)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.providerName)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateState(t *testing.T) {
	tests := []struct {
		name    string
		state   *workflow.ConversationState
		wantErr bool
	}{
		{
			name:    "nil_state",
			state:   nil,
			wantErr: true,
		},
		{
			name:    "valid_empty_state",
			state:   &workflow.ConversationState{},
			wantErr: false,
		},
		{
			name: "valid_state_with_turns",
			state: &workflow.ConversationState{
				Turns: []workflow.Turn{{Role: "user", Content: "Hello"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateState(tt.state)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "state")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetWorkflowID(t *testing.T) {
	tests := []struct {
		name     string
		options  map[string]any
		expected string
	}{
		{
			name:     "nil_options",
			options:  nil,
			expected: "unknown",
		},
		{
			name:     "missing_key",
			options:  map[string]any{},
			expected: "unknown",
		},
		{
			name:     "present_key",
			options:  map[string]any{"workflowID": "wf-123"},
			expected: "wf-123",
		},
		{
			name:     "wrong_type",
			options:  map[string]any{"workflowID": 123},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getWorkflowID(tt.options)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetStepName(t *testing.T) {
	tests := []struct {
		name     string
		options  map[string]any
		expected string
	}{
		{
			name:     "nil_options",
			options:  nil,
			expected: "unknown",
		},
		{
			name:     "missing_key",
			options:  map[string]any{},
			expected: "unknown",
		},
		{
			name:     "present_key",
			options:  map[string]any{"stepName": "step-1"},
			expected: "step-1",
		},
		{
			name:     "wrong_type",
			options:  map[string]any{"stepName": 123},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStepName(tt.options)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateInputTokens(t *testing.T) {
	tests := []struct {
		name         string
		turns        []workflow.Turn
		excludeLastN int
		expected     int
	}{
		{
			name:         "empty_turns",
			turns:        []workflow.Turn{},
			excludeLastN: 0,
			expected:     0,
		},
		{
			name: "single_turn_exclude_one",
			turns: []workflow.Turn{
				{Role: "user", Content: "test", Tokens: 10},
			},
			excludeLastN: 1,
			expected:     0,
		},
		{
			name: "multiple_turns_exclude_one",
			turns: []workflow.Turn{
				{Role: "user", Content: "hello", Tokens: 5},
				{Role: "assistant", Content: "hi there", Tokens: 10},
			},
			excludeLastN: 1,
			expected:     5,
		},
		{
			name: "estimate_missing_tokens",
			turns: []workflow.Turn{
				{Role: "user", Content: "test", Tokens: 0}, // Will be estimated: 4/4=1
				{Role: "assistant", Content: "response", Tokens: 0},
			},
			excludeLastN: 1,
			expected:     1,
		},
		{
			name: "exclude_more_than_available",
			turns: []workflow.Turn{
				{Role: "user", Content: "test", Tokens: 10},
			},
			excludeLastN: 5,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateInputTokens(tt.turns, tt.excludeLastN)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseJSONResponse(t *testing.T) {
	tests := []struct {
		name    string
		output  []byte
		wantErr bool
		wantNil bool
	}{
		{
			name:    "valid_json",
			output:  []byte(`{"status": "ok", "count": 5}`),
			wantErr: false,
			wantNil: false,
		},
		{
			name:    "invalid_json",
			output:  []byte(`not json`),
			wantErr: true,
			wantNil: true,
		},
		{
			name:    "empty_json_object",
			output:  []byte(`{}`),
			wantErr: false,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseJSONResponse(tt.output)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

func TestTryParseJSONResponse(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantNil bool
	}{
		{
			name:    "valid_json",
			output:  `{"key": "value"}`,
			wantNil: false,
		},
		{
			name:    "not_json_prefix",
			output:  `plain text`,
			wantNil: true,
		},
		{
			name:    "json_with_whitespace",
			output:  `  {"key": "value"}  `,
			wantNil: false,
		},
		{
			name:    "invalid_json_with_brace",
			output:  `{invalid}`,
			wantNil: true,
		},
		{
			name:    "array_not_object",
			output:  `[1, 2, 3]`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tryParseJSONResponse(tt.output)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}
