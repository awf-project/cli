package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F065
// Component: T002
// Tests: OutputFormat field validation in AgentConfig

func TestAgentConfig_OutputFormat_ValidValues(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat OutputFormat
		wantErr      bool
	}{
		{
			name:         "empty/none - backward compatibility",
			outputFormat: OutputFormatNone,
			wantErr:      false,
		},
		{
			name:         "json format",
			outputFormat: OutputFormatJSON,
			wantErr:      false,
		},
		{
			name:         "text format",
			outputFormat: OutputFormatText,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:     "claude",
				Prompt:       "Test prompt",
				OutputFormat: tt.outputFormat,
			}
			err := config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.outputFormat, config.OutputFormat)
			}
		})
	}
}

func TestAgentConfig_OutputFormat_InvalidValues(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat OutputFormat
		errMsg       string
	}{
		{
			name:         "xml format - not supported",
			outputFormat: "xml",
			errMsg:       "output_format must be 'json', 'text', or empty",
		},
		{
			name:         "csv format - not supported",
			outputFormat: "csv",
			errMsg:       "output_format must be 'json', 'text', or empty",
		},
		{
			name:         "yaml format - not supported",
			outputFormat: "yaml",
			errMsg:       "output_format must be 'json', 'text', or empty",
		},
		{
			name:         "random string",
			outputFormat: "random",
			errMsg:       "output_format must be 'json', 'text', or empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:     "claude",
				Prompt:       "Test prompt",
				OutputFormat: tt.outputFormat,
			}
			err := config.Validate(nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestAgentConfig_OutputFormat_CaseNormalization(t *testing.T) {
	tests := []struct {
		name           string
		inputFormat    OutputFormat
		expectedFormat OutputFormat
	}{
		{
			name:           "uppercase JSON",
			inputFormat:    "JSON",
			expectedFormat: OutputFormatJSON,
		},
		{
			name:           "mixed case Json",
			inputFormat:    "Json",
			expectedFormat: OutputFormatJSON,
		},
		{
			name:           "uppercase TEXT",
			inputFormat:    "TEXT",
			expectedFormat: OutputFormatText,
		},
		{
			name:           "mixed case Text",
			inputFormat:    "Text",
			expectedFormat: OutputFormatText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:     "claude",
				Prompt:       "Test prompt",
				OutputFormat: tt.inputFormat,
			}
			err := config.Validate(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedFormat, config.OutputFormat)
		})
	}
}

func TestAgentConfig_OutputFormat_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name           string
		inputFormat    OutputFormat
		expectedFormat OutputFormat
	}{
		{
			name:           "leading whitespace",
			inputFormat:    "  json",
			expectedFormat: OutputFormatJSON,
		},
		{
			name:           "trailing whitespace",
			inputFormat:    "json  ",
			expectedFormat: OutputFormatJSON,
		},
		{
			name:           "both leading and trailing",
			inputFormat:    "  text  ",
			expectedFormat: OutputFormatText,
		},
		{
			name:           "whitespace only - becomes empty",
			inputFormat:    "   ",
			expectedFormat: OutputFormatNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider:     "claude",
				Prompt:       "Test prompt",
				OutputFormat: tt.inputFormat,
			}
			err := config.Validate(nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedFormat, config.OutputFormat)
		})
	}
}

func TestAgentConfig_OutputFormat_WithOtherFields(t *testing.T) {
	tests := []struct {
		name   string
		config AgentConfig
	}{
		{
			name: "output_format with prompt",
			config: AgentConfig{
				Provider:     "claude",
				Prompt:       "Analyze this: {{inputs.code}}",
				OutputFormat: OutputFormatJSON,
			},
		},
		{
			name: "output_format with prompt_file",
			config: AgentConfig{
				Provider:     "claude",
				PromptFile:   "prompts/analyze.md",
				OutputFormat: OutputFormatText,
			},
		},
		{
			name: "output_format with conversation mode",
			config: AgentConfig{
				Provider:     "claude",
				Prompt:       "Initial message",
				Mode:         "conversation",
				OutputFormat: OutputFormatJSON,
			},
		},
		{
			name: "output_format with options",
			config: AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Options: map[string]any{
					"model":       "claude-sonnet-4-20250514",
					"max_tokens":  4096,
					"temperature": 0.7,
				},
				OutputFormat: OutputFormatJSON,
			},
		},
		{
			name: "output_format with timeout",
			config: AgentConfig{
				Provider:     "claude",
				Prompt:       "Test",
				Timeout:      120,
				OutputFormat: OutputFormatText,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(nil)
			require.NoError(t, err)
		})
	}
}

func TestAgentConfig_OutputFormat_DoesNotInterfereWithExistingValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing provider still fails",
			config: AgentConfig{
				Prompt:       "Test",
				OutputFormat: OutputFormatJSON,
			},
			wantErr: true,
			errMsg:  "provider",
		},
		{
			name: "missing prompt still fails",
			config: AgentConfig{
				Provider:     "claude",
				OutputFormat: OutputFormatJSON,
			},
			wantErr: true,
			errMsg:  "prompt",
		},
		{
			name: "negative timeout still fails",
			config: AgentConfig{
				Provider:     "claude",
				Prompt:       "Test",
				Timeout:      -1,
				OutputFormat: OutputFormatJSON,
			},
			wantErr: true,
			errMsg:  "timeout",
		},
		{
			name: "prompt and prompt_file mutual exclusivity still enforced",
			config: AgentConfig{
				Provider:     "claude",
				Prompt:       "Test",
				PromptFile:   "test.md",
				OutputFormat: OutputFormatJSON,
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate(nil)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAgentConfig_OutputFormat_DefaultValue(t *testing.T) {
	config := AgentConfig{
		Provider: "claude",
		Prompt:   "Test prompt",
	}

	require.Equal(t, OutputFormatNone, config.OutputFormat, "default OutputFormat should be empty/none")

	err := config.Validate(nil)
	require.NoError(t, err)
	assert.Equal(t, OutputFormatNone, config.OutputFormat, "empty output_format should remain empty after validation")
}
