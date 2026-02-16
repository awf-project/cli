package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Feature: F065
// Component: T004
// Tests: OutputFormat field in DryRunAgent struct

func TestDryRunAgent_OutputFormat_FieldExists(t *testing.T) {
	agent := DryRunAgent{
		Provider:       "claude",
		ResolvedPrompt: "Analyze this code",
		CLICommand:     "claude agent -p 'prompt'",
		Options: map[string]any{
			"model": "claude-sonnet-4-20250514",
		},
		Timeout:      300,
		OutputFormat: OutputFormatJSON,
	}

	assert.Equal(t, OutputFormatJSON, agent.OutputFormat)
}

func TestDryRunAgent_OutputFormat_AllValidFormats(t *testing.T) {
	tests := []struct {
		name   string
		format OutputFormat
	}{
		{
			name:   "none/empty format",
			format: OutputFormatNone,
		},
		{
			name:   "json format",
			format: OutputFormatJSON,
		},
		{
			name:   "text format",
			format: OutputFormatText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := DryRunAgent{
				Provider:       "claude",
				ResolvedPrompt: "Test prompt",
				OutputFormat:   tt.format,
			}

			assert.Equal(t, tt.format, agent.OutputFormat)
		})
	}
}

func TestDryRunAgent_OutputFormat_JSONSerialization(t *testing.T) {
	tests := []struct {
		name         string
		agent        DryRunAgent
		expectedJSON string
	}{
		{
			name: "with json format",
			agent: DryRunAgent{
				Provider:       "claude",
				ResolvedPrompt: "Extract JSON from code",
				CLICommand:     "claude agent -p 'prompt'",
				Timeout:        120,
				OutputFormat:   OutputFormatJSON,
			},
			expectedJSON: `{"Provider":"claude","ResolvedPrompt":"Extract JSON from code","CLICommand":"claude agent -p 'prompt'","Options":null,"Timeout":120,"OutputFormat":"json"}`,
		},
		{
			name: "with text format",
			agent: DryRunAgent{
				Provider:       "gemini",
				ResolvedPrompt: "Generate text",
				CLICommand:     "gemini agent -p 'prompt'",
				Timeout:        60,
				OutputFormat:   OutputFormatText,
			},
			expectedJSON: `{"Provider":"gemini","ResolvedPrompt":"Generate text","CLICommand":"gemini agent -p 'prompt'","Options":null,"Timeout":60,"OutputFormat":"text"}`,
		},
		{
			name: "with empty format - backward compatibility",
			agent: DryRunAgent{
				Provider:       "claude",
				ResolvedPrompt: "Default behavior",
				CLICommand:     "claude agent -p 'prompt'",
				Timeout:        300,
				OutputFormat:   OutputFormatNone,
			},
			expectedJSON: `{"Provider":"claude","ResolvedPrompt":"Default behavior","CLICommand":"claude agent -p 'prompt'","Options":null,"Timeout":300,"OutputFormat":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.agent)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expectedJSON, string(jsonBytes))
		})
	}
}

func TestDryRunAgent_OutputFormat_JSONDeserialization(t *testing.T) {
	tests := []struct {
		name           string
		jsonInput      string
		expectedFormat OutputFormat
	}{
		{
			name:           "deserialize json format",
			jsonInput:      `{"Provider":"claude","ResolvedPrompt":"test","CLICommand":"cmd","Timeout":300,"OutputFormat":"json"}`,
			expectedFormat: OutputFormatJSON,
		},
		{
			name:           "deserialize text format",
			jsonInput:      `{"Provider":"claude","ResolvedPrompt":"test","CLICommand":"cmd","Timeout":300,"OutputFormat":"text"}`,
			expectedFormat: OutputFormatText,
		},
		{
			name:           "deserialize empty format",
			jsonInput:      `{"Provider":"claude","ResolvedPrompt":"test","CLICommand":"cmd","Timeout":300,"OutputFormat":""}`,
			expectedFormat: OutputFormatNone,
		},
		{
			name:           "missing OutputFormat field defaults to empty",
			jsonInput:      `{"Provider":"claude","ResolvedPrompt":"test","CLICommand":"cmd","Timeout":300}`,
			expectedFormat: OutputFormatNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var agent DryRunAgent
			err := json.Unmarshal([]byte(tt.jsonInput), &agent)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedFormat, agent.OutputFormat)
		})
	}
}

func TestDryRunAgent_OutputFormat_InDryRunStep(t *testing.T) {
	step := DryRunStep{
		Name:        "analyze_code",
		Type:        StepTypeAgent,
		Description: "Analyze code with agent",
		Agent: &DryRunAgent{
			Provider:       "claude",
			ResolvedPrompt: "Analyze: {{inputs.code}}",
			CLICommand:     "claude agent -p 'Analyze: test.go'",
			Options: map[string]any{
				"model":      "claude-sonnet-4-20250514",
				"max_tokens": 4096,
			},
			Timeout:      180,
			OutputFormat: OutputFormatJSON,
		},
	}

	require.NotNil(t, step.Agent)
	assert.Equal(t, OutputFormatJSON, step.Agent.OutputFormat)
}

func TestDryRunAgent_OutputFormat_InCompletePlan(t *testing.T) {
	plan := DryRunPlan{
		WorkflowName: "analyze-workflow",
		Description:  "Workflow with agent output formats",
		Inputs: map[string]DryRunInput{
			"code_path": {
				Name:     "code_path",
				Value:    "src/main.go",
				Required: true,
			},
		},
		Steps: []DryRunStep{
			{
				Name:        "extract_json",
				Type:        StepTypeAgent,
				Description: "Extract JSON structure",
				Agent: &DryRunAgent{
					Provider:       "claude",
					ResolvedPrompt: "Extract JSON from src/main.go",
					CLICommand:     "claude agent -p 'prompt'",
					Timeout:        120,
					OutputFormat:   OutputFormatJSON,
				},
			},
			{
				Name:        "generate_summary",
				Type:        StepTypeAgent,
				Description: "Generate plain text summary",
				Agent: &DryRunAgent{
					Provider:       "gemini",
					ResolvedPrompt: "Summarize code",
					CLICommand:     "gemini agent -p 'prompt'",
					Timeout:        60,
					OutputFormat:   OutputFormatText,
				},
			},
			{
				Name:        "legacy_step",
				Type:        StepTypeAgent,
				Description: "Legacy step without format",
				Agent: &DryRunAgent{
					Provider:       "claude",
					ResolvedPrompt: "Old style prompt",
					CLICommand:     "claude agent -p 'prompt'",
					Timeout:        300,
					OutputFormat:   OutputFormatNone,
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(plan)
	require.NoError(t, err)

	var decoded DryRunPlan
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	require.Len(t, decoded.Steps, 3)
	assert.Equal(t, OutputFormatJSON, decoded.Steps[0].Agent.OutputFormat)
	assert.Equal(t, OutputFormatText, decoded.Steps[1].Agent.OutputFormat)
	assert.Equal(t, OutputFormatNone, decoded.Steps[2].Agent.OutputFormat)
}

func TestDryRunAgent_OutputFormat_WithOptions(t *testing.T) {
	agent := DryRunAgent{
		Provider:       "claude",
		ResolvedPrompt: "Complex prompt with {{inputs.data}}",
		CLICommand:     "claude agent -p 'prompt' --model claude-sonnet-4-20250514",
		Options: map[string]any{
			"model":       "claude-sonnet-4-20250514",
			"temperature": 0.7,
			"max_tokens":  4096,
			"top_p":       0.9,
		},
		Timeout:      240,
		OutputFormat: OutputFormatJSON,
	}

	jsonBytes, err := json.Marshal(agent)
	require.NoError(t, err)

	var decoded DryRunAgent
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)

	assert.Equal(t, OutputFormatJSON, decoded.OutputFormat)
	assert.Equal(t, "claude", decoded.Provider)
	assert.Equal(t, 240, decoded.Timeout)
	assert.NotNil(t, decoded.Options)
	assert.Equal(t, "claude-sonnet-4-20250514", decoded.Options["model"])
}

func TestDryRunAgent_OutputFormat_ZeroValue(t *testing.T) {
	var agent DryRunAgent

	assert.Equal(t, OutputFormatNone, agent.OutputFormat, "zero value should be empty/none")
}
