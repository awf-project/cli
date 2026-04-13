package application_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
)

// TestCloneAndInjectOutputFormat_OriginalMapNotMutated verifies that cloneAndInjectOutputFormat
// does not mutate the original options map when injecting output_format.
// This validates FR-009: the original map stays clean, preventing shared state corruption.
// Tested indirectly: when executeAgentStep calls the provider with injected options,
// the original step.Agent.Options remains unmodified.
func TestCloneAndInjectOutputFormat_OriginalMapNotMutated(t *testing.T) {
	// Arrange: capture the options passed to the provider
	var capturedOptions map[string]any
	mockProvider := testmocks.NewMockAgentProvider("test-provider")
	mockProvider.SetExecuteFunc(func(
		ctx context.Context,
		prompt string,
		options map[string]any,
		stdout, stderr io.Writer,
	) (*workflow.AgentResult, error) {
		// Capture the options received by provider
		capturedOptions = make(map[string]any)
		for k, v := range options {
			capturedOptions[k] = v
		}
		return &workflow.AgentResult{
			Output:        "raw output",
			DisplayOutput: "display output",
			Tokens:        10,
		}, nil
	})

	// Setup with a workflow that has options configured
	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "test-provider",
					Prompt:       "test prompt",
					OutputFormat: workflow.OutputFormatText,
					Options: map[string]any{
						"model":       "test-model",
						"temperature": 0.7,
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, _ := NewTestHarness(t).WithWorkflow("test", wf).Build()
	registry := testmocks.NewMockAgentRegistry()
	registry.Register(mockProvider)
	svc.SetAgentRegistry(registry)

	// Act: execute the workflow
	ctx := context.Background()
	_, err := svc.Run(ctx, "test", nil)

	// Assert: execution succeeds
	require.NoError(t, err)

	// Assert: provider received output_format in its options (injected copy)
	require.NotNil(t, capturedOptions)
	assert.Equal(t, "text", capturedOptions["output_format"])
	assert.Equal(t, "test-model", capturedOptions["model"])
	assert.Equal(t, 0.7, capturedOptions["temperature"])

	// Assert: CRITICAL - original step.Agent.Options still has NO output_format
	// This proves cloneAndInjectOutputFormat created a clone before mutation
	originalOptions := wf.Steps["step1"].Agent.Options
	assert.NotContains(t, originalOptions, "output_format",
		"original options map must not be mutated - cloneAndInjectOutputFormat should clone before injecting")

	// Assert: original options still have their original values
	assert.Equal(t, "test-model", originalOptions["model"])
	assert.Equal(t, 0.7, originalOptions["temperature"])
}

// TestExecuteAgentStep_CopiesDisplayOutputToState verifies DisplayOutput from the provider
// result is correctly copied into the step state during agent step execution.
// This validates FR-004: the application layer propagates DisplayOutput from AgentResult to StepState.
func TestExecuteAgentStep_CopiesDisplayOutputToState(t *testing.T) {
	tests := []struct {
		name          string
		displayOutput string
		rawOutput     string
	}{
		{
			name:          "copies_non_empty_display_output",
			displayOutput: "Extracted text from agent provider",
			rawOutput:     `{"type":"content_block_delta","delta":{"type":"text_delta"}}`,
		},
		{
			name:          "copies_empty_display_output",
			displayOutput: "",
			rawOutput:     `{"type":"content_block_delta"}`,
		},
		{
			name:          "copies_multiline_display_output",
			displayOutput: "Line 1\nLine 2\nLine 3",
			rawOutput:     "raw line 1\nraw line 2\nraw line 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: setup execution service with agent step
			svc, _ := NewTestHarness(t).
				WithWorkflow("test", buildTestWorkflow()).
				Build()

			// Mock provider that returns both raw and display output
			mockProvider := testmocks.NewMockAgentProvider("test-provider")
			mockProvider.SetExecuteFunc(func(
				ctx context.Context,
				prompt string,
				options map[string]any,
				stdout, stderr io.Writer,
			) (*workflow.AgentResult, error) {
				return &workflow.AgentResult{
					Output:        tt.rawOutput,
					DisplayOutput: tt.displayOutput,
					Response:      map[string]any{"text": "response"},
					Tokens:        100,
				}, nil
			})

			// Setup registry
			registry := testmocks.NewMockAgentRegistry()
			registry.Register(mockProvider)
			svc.SetAgentRegistry(registry)

			// Act: execute the workflow
			ctx := context.Background()
			execCtx, err := svc.Run(ctx, "test", nil)

			// Assert: execution succeeds
			require.NoError(t, err)
			require.NotNil(t, execCtx)

			// Assert: step state contains DisplayOutput copied from result
			stepState, exists := execCtx.States["agent-step"]
			require.True(t, exists, "agent-step state must exist")
			assert.Equal(t, tt.displayOutput, stepState.DisplayOutput,
				"DisplayOutput should be copied from provider result to step state")
			assert.Equal(t, tt.rawOutput, stepState.Output,
				"raw Output should remain unchanged")
		})
	}
}

// TestExecuteAgentStep_OutputFormatInjectedIntoOptions verifies that output_format
// from step.Agent.OutputFormat is injected into the cloned options map passed to the provider.
// This validates FR-009 (cloning) and the bridging of OutputFormat to options["output_format"].
func TestExecuteAgentStep_OutputFormatInjectedIntoOptions(t *testing.T) {
	tests := []struct {
		name          string
		outputFormat  workflow.OutputFormat
		expectedValue string
	}{
		{
			name:          "injects_text_format",
			outputFormat:  workflow.OutputFormatText,
			expectedValue: "text",
		},
		{
			name:          "injects_json_format",
			outputFormat:  workflow.OutputFormatJSON,
			expectedValue: "json",
		},
		{
			// Unspecified OutputFormatNone defaults to "text" so downstream CLI
			// providers and the F082 display-matrix pipeline always see an
			// explicit format value.
			name:          "defaults_none_to_text",
			outputFormat:  workflow.OutputFormatNone,
			expectedValue: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedOptions map[string]any
			mockProvider := testmocks.NewMockAgentProvider("test-provider")
			mockProvider.SetExecuteFunc(func(
				ctx context.Context,
				prompt string,
				options map[string]any,
				stdout, stderr io.Writer,
			) (*workflow.AgentResult, error) {
				capturedOptions = make(map[string]any)
				for k, v := range options {
					capturedOptions[k] = v
				}
				// Return valid JSON for json format test, otherwise plain text
				output := `{"type":"response","text":"raw output"}`
				if options["output_format"] != "json" {
					output = "raw output"
				}
				return &workflow.AgentResult{
					Output:        output,
					DisplayOutput: "display output",
					Tokens:        10,
				}, nil
			})

			wf := &workflow.Workflow{
				Name:    "test",
				Initial: "step1",
				Steps: map[string]*workflow.Step{
					"step1": {
						Name: "step1",
						Type: workflow.StepTypeAgent,
						Agent: &workflow.AgentConfig{
							Provider:     "test-provider",
							Prompt:       "test prompt",
							OutputFormat: tt.outputFormat,
							Options: map[string]any{
								"model": "test-model",
							},
						},
						OnSuccess: "done",
					},
					"done": {
						Name:   "done",
						Type:   workflow.StepTypeTerminal,
						Status: workflow.TerminalSuccess,
					},
				},
			}

			svc, _ := NewTestHarness(t).WithWorkflow("test", wf).Build()
			registry := testmocks.NewMockAgentRegistry()
			registry.Register(mockProvider)
			svc.SetAgentRegistry(registry)

			ctx := context.Background()
			_, err := svc.Run(ctx, "test", nil)

			require.NoError(t, err)
			require.NotNil(t, capturedOptions)
			assert.Equal(t, tt.expectedValue, capturedOptions["output_format"],
				"output_format should be injected as string into options")
		})
	}
}

// TestExecuteAgentStep_WithNilOptions verifies that cloneAndInjectOutputFormat
// handles nil options map gracefully, creating a new map with just output_format.
func TestExecuteAgentStep_WithNilOptions(t *testing.T) {
	var capturedOptions map[string]any
	mockProvider := testmocks.NewMockAgentProvider("test-provider")
	mockProvider.SetExecuteFunc(func(
		ctx context.Context,
		prompt string,
		options map[string]any,
		stdout, stderr io.Writer,
	) (*workflow.AgentResult, error) {
		capturedOptions = options
		return &workflow.AgentResult{
			Output:        "raw output",
			DisplayOutput: "display output",
			Tokens:        10,
		}, nil
	})

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "test-provider",
					Prompt:       "test prompt",
					OutputFormat: workflow.OutputFormatText,
					Options:      nil,
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, _ := NewTestHarness(t).WithWorkflow("test", wf).Build()
	registry := testmocks.NewMockAgentRegistry()
	registry.Register(mockProvider)
	svc.SetAgentRegistry(registry)

	ctx := context.Background()
	_, err := svc.Run(ctx, "test", nil)

	require.NoError(t, err)
	require.NotNil(t, capturedOptions)
	assert.Equal(t, "text", capturedOptions["output_format"])
	assert.Len(t, capturedOptions, 1, "options should contain only output_format when original is nil")
}

// TestExecuteAgentStep_PreservesMultipleOptionsWithFormat verifies that when
// cloneAndInjectOutputFormat is called, all original options are preserved in the clone
// along with the injected output_format, and the original map is untouched.
func TestExecuteAgentStep_PreservesMultipleOptionsWithFormat(t *testing.T) {
	var capturedOptions map[string]any
	mockProvider := testmocks.NewMockAgentProvider("test-provider")
	mockProvider.SetExecuteFunc(func(
		ctx context.Context,
		prompt string,
		options map[string]any,
		stdout, stderr io.Writer,
	) (*workflow.AgentResult, error) {
		capturedOptions = make(map[string]any)
		for k, v := range options {
			capturedOptions[k] = v
		}
		return &workflow.AgentResult{
			Output:        "raw output",
			DisplayOutput: "display output",
			Tokens:        10,
		}, nil
	})

	wf := &workflow.Workflow{
		Name:    "test",
		Initial: "step1",
		Steps: map[string]*workflow.Step{
			"step1": {
				Name: "step1",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "test-provider",
					Prompt:       "test prompt",
					OutputFormat: workflow.OutputFormatText,
					Options: map[string]any{
						"model":       "claude-3-sonnet",
						"temperature": 0.9,
						"max_tokens":  4096,
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}

	svc, _ := NewTestHarness(t).WithWorkflow("test", wf).Build()
	registry := testmocks.NewMockAgentRegistry()
	registry.Register(mockProvider)
	svc.SetAgentRegistry(registry)

	originalOptions := wf.Steps["step1"].Agent.Options
	originalOptionCount := len(originalOptions)

	ctx := context.Background()
	_, err := svc.Run(ctx, "test", nil)

	require.NoError(t, err)

	// Verify provider received all options plus injected output_format
	require.NotNil(t, capturedOptions)
	assert.Len(t, capturedOptions, originalOptionCount+1,
		"cloned options should have all original options plus output_format")
	assert.Equal(t, "claude-3-sonnet", capturedOptions["model"])
	assert.Equal(t, 0.9, capturedOptions["temperature"])
	assert.Equal(t, 4096, capturedOptions["max_tokens"])
	assert.Equal(t, "text", capturedOptions["output_format"])

	// Verify original options map is unchanged
	assert.Len(t, originalOptions, originalOptionCount,
		"original options map must not be mutated")
	assert.NotContains(t, originalOptions, "output_format",
		"original options should not have output_format")
}

// buildTestWorkflow creates a simple workflow with one agent step.
func buildTestWorkflow() *workflow.Workflow {
	return &workflow.Workflow{
		Name:    "test",
		Initial: "agent-step",
		Steps: map[string]*workflow.Step{
			"agent-step": {
				Name: "agent-step",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider: "test-provider",
					Prompt:   "test prompt",
				},
				OnSuccess: "done",
			},
			"done": {
				Name:   "done",
				Type:   workflow.StepTypeTerminal,
				Status: workflow.TerminalSuccess,
			},
		},
	}
}
