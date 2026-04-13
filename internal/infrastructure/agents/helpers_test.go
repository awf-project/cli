package agents

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			name: "state_with_session_id",
			state: &workflow.ConversationState{
				Turns:     []workflow.Turn{},
				SessionID: "session-abc123",
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
			assert.Equal(t, tt.state.SessionID, cloned.SessionID, "SessionID must be propagated")

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
