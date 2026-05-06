package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for validateCopilotOptions enum validation
// Implements F089: GitHub Copilot Agent Provider Integration
// Scope: mode and effort option validation

func TestValidateCopilotOptions_ValidMode(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "mode interactive",
			options: map[string]any{"mode": "interactive"},
			wantErr: false,
		},
		{
			name:    "mode plan",
			options: map[string]any{"mode": "plan"},
			wantErr: false,
		},
		{
			name:    "mode autopilot",
			options: map[string]any{"mode": "autopilot"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCopilotOptions(tt.options)
			if tt.wantErr {
				assert.Error(t, err, "validateCopilotOptions should return error")
			} else {
				assert.NoError(t, err, "validateCopilotOptions should not return error")
			}
		})
	}
}

func TestValidateCopilotOptions_InvalidMode(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "mode invalid_mode",
			options: map[string]any{"mode": "invalid_mode"},
			wantErr: true,
			errMsg:  "invalid mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCopilotOptions(tt.options)

			require.Error(t, err, "validateCopilotOptions should return error for %q", tt.options["mode"])
			assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
			assert.Contains(t, err.Error(), "interactive", "error should mention valid mode values")
			assert.Contains(t, err.Error(), "plan", "error should mention valid mode values")
			assert.Contains(t, err.Error(), "autopilot", "error should mention valid mode values")
			assert.Contains(t, err.Error(), "invalid_mode", "error should include the invalid value")
		})
	}
}

func TestValidateCopilotOptions_ValidEffort(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "effort low",
			options: map[string]any{"effort": "low"},
			wantErr: false,
		},
		{
			name:    "effort medium",
			options: map[string]any{"effort": "medium"},
			wantErr: false,
		},
		{
			name:    "effort high",
			options: map[string]any{"effort": "high"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCopilotOptions(tt.options)
			if tt.wantErr {
				assert.Error(t, err, "validateCopilotOptions should return error")
			} else {
				assert.NoError(t, err, "validateCopilotOptions should not return error")
			}
		})
	}
}

func TestValidateCopilotOptions_InvalidEffort(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
		errMsg  string
	}{
		{
			name:    "effort ultra",
			options: map[string]any{"effort": "ultra"},
			wantErr: true,
			errMsg:  "invalid effort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCopilotOptions(tt.options)

			require.Error(t, err, "validateCopilotOptions should return error for %q", tt.options["effort"])
			assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
			assert.Contains(t, err.Error(), "low", "error should mention valid effort values")
			assert.Contains(t, err.Error(), "medium", "error should mention valid effort values")
			assert.Contains(t, err.Error(), "high", "error should mention valid effort values")
			assert.Contains(t, err.Error(), "ultra", "error should include the invalid value")
		})
	}
}

func TestValidateCopilotOptions_EmptyOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		wantErr bool
	}{
		{
			name:    "empty options map",
			options: map[string]any{},
			wantErr: false,
		},
		{
			name:    "nil options",
			options: nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCopilotOptions(tt.options)
			if tt.wantErr {
				assert.Error(t, err, "validateCopilotOptions should return error")
			} else {
				assert.NoError(t, err, "validateCopilotOptions should not return error")
			}
		})
	}
}

func TestValidateCopilotOptions_UnknownKeysIgnored(t *testing.T) {
	options := map[string]any{
		"foo": "bar",
		"baz": 42,
	}

	err := validateCopilotOptions(options)
	assert.NoError(t, err, "validateCopilotOptions should silently ignore unknown keys")
}

func TestValidateCopilotOptions_UnknownKeysWithValidOptions(t *testing.T) {
	options := map[string]any{
		"mode": "interactive",
		"foo":  "bar",
	}

	err := validateCopilotOptions(options)
	assert.NoError(t, err, "validateCopilotOptions should silently ignore unknown keys with valid options")
}

func TestValidateCopilotOptions_BothInvalid(t *testing.T) {
	options := map[string]any{
		"mode":   "invalid_mode",
		"effort": "ultra",
	}

	err := validateCopilotOptions(options)

	require.Error(t, err, "validateCopilotOptions should return error for both invalid mode and effort")
	assert.Contains(t, err.Error(), "invalid_mode", "error should include the invalid mode value")
	assert.Contains(t, err.Error(), "ultra", "error should include the invalid effort value")
}

func TestValidateCopilotOptions_WithOtherOptions(t *testing.T) {
	options := map[string]any{
		"mode":          "interactive",
		"effort":        "high",
		"model":         "claude-opus",
		"system_prompt": "Be helpful",
		"allowed_tools": []string{"tool1", "tool2"},
	}

	err := validateCopilotOptions(options)
	assert.NoError(t, err, "validateCopilotOptions should only validate mode and effort, ignore others")
}
