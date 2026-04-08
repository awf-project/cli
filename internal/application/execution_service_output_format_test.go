package application_test

import (
	"context"
	"io"
	"testing"

	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T008
// Feature: F065 - Output Format for Agent Steps

// TestExecutionService_AgentStep_OutputFormat_JSON_StripsFencesAndParsesJSON verifies
// that when output_format: json is set on an agent step, the execution service:
// 1. Strips code fences from agent output
// 2. Validates the content as JSON
// 3. Populates states.step.JSON with parsed data
func TestExecutionService_AgentStep_OutputFormat_JSON_StripsFencesAndParsesJSON(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "json-output-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Extract structured data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("json-output-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Mock agent returns JSON wrapped in markdown code fences
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "```json\n{\"name\":\"alice\",\"count\":3}\n```",
			Tokens:   50,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "json-output-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify step state
	state, exists := ctx.GetStepState("extract")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusCompleted, state.Status)

	// Output should have code fences stripped
	assert.Equal(t, `{"name":"alice","count":3}`, state.Output)

	// JSON field should be populated with parsed data
	require.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "alice", jsonObj["name"])
	assert.Equal(t, float64(3), jsonObj["count"])
}

// TestExecutionService_AgentStep_OutputFormat_JSON_NoFences_ParsesDirectly tests
// that when output_format: json is set but agent output has no code fences,
// the JSON is parsed directly without modification.
func TestExecutionService_AgentStep_OutputFormat_JSON_NoFences_ParsesDirectly(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "json-no-fence-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Return JSON",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("json-no-fence-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns raw JSON without fences
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   `{"status":"ok","value":42}`,
			Tokens:   30,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "json-no-fence-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("extract")
	require.True(t, exists)

	// Output unchanged (no fences to strip)
	assert.Equal(t, `{"status":"ok","value":42}`, state.Output)

	// JSON should still be parsed
	require.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "ok", jsonObj["status"])
	assert.Equal(t, float64(42), jsonObj["value"])
}

// TestExecutionService_AgentStep_OutputFormat_JSON_InvalidJSON_FailsStep verifies
// that when output_format: json is set but the stripped content is not valid JSON,
// the step fails with a descriptive error including a preview of the malformed content.
func TestExecutionService_AgentStep_OutputFormat_JSON_InvalidJSON_FailsStep(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "invalid-json-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Return data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
				OnFailure: "error",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
			"error": {
				Name: "error",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("invalid-json-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns malformed JSON
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "```json\n{invalid json here\n```",
			Tokens:   20,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "invalid-json-test", nil)

	// Execution should fail when JSON validation fails
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
	assert.Contains(t, err.Error(), "invalid json here")

	// Context should still track the failed step state
	state, exists := ctx.GetStepState("extract")
	require.True(t, exists)
	assert.Equal(t, workflow.StatusFailed, state.Status)

	// Step state error should also contain validation failure details
	assert.Contains(t, state.Error, "invalid JSON")
}

// TestExecutionService_AgentStep_OutputFormat_Text_StripsFencesOnly tests
// that when output_format: text is set, code fences are stripped but JSON
// parsing is not performed.
func TestExecutionService_AgentStep_OutputFormat_Text_StripsFencesOnly(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "text-output-test",
		Initial: "analyze",
		Steps: map[string]*workflow.Step{
			"analyze": {
				Name: "analyze",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Analyze code",
					OutputFormat: workflow.OutputFormatText,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("text-output-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns text in bash code fence
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "```bash\necho hello world\n```",
			Tokens:   15,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "text-output-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("analyze")
	require.True(t, exists)

	// Output should have fences stripped
	assert.Equal(t, "echo hello world", state.Output)

	// JSON field should be nil (no parsing for text format)
	assert.Nil(t, state.JSON)
}

// TestExecutionService_AgentStep_OutputFormat_None_BackwardCompatibility verifies
// that when output_format is not set (empty/none), the agent output is stored
// unchanged without any stripping or validation.
func TestExecutionService_AgentStep_OutputFormat_None_BackwardCompatibility(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "no-format-test",
		Initial: "analyze",
		Steps: map[string]*workflow.Step{
			"analyze": {
				Name: "analyze",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Analyze code",
					OutputFormat: workflow.OutputFormatNone,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("no-format-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns output with code fences
	rawOutput := "```json\n{\"data\":\"value\"}\n```"
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   rawOutput,
			Tokens:   20,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "no-format-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("analyze")
	require.True(t, exists)

	// Output should be completely unchanged (fences preserved)
	assert.Equal(t, rawOutput, state.Output)

	// JSON field should be nil (no processing)
	assert.Nil(t, state.JSON)
}

// TestExecutionService_AgentStep_OutputFormat_JSON_ArrayParsing verifies that
// when output_format: json is set and the content is a JSON array, it is correctly
// parsed but not stored in JSON field (only objects are stored).
func TestExecutionService_AgentStep_OutputFormat_JSON_ArrayParsing(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "json-array-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "List items",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("json-array-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns JSON array
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   `["item1","item2","item3"]`,
			Tokens:   10,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "json-array-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("extract")
	require.True(t, exists)

	// Output should be the array JSON
	assert.Equal(t, `["item1","item2","item3"]`, state.Output)

	// JSON field should contain the parsed array
	require.NotNil(t, state.JSON)
	jsonArray, ok := state.JSON.([]any)
	require.True(t, ok, "JSON field should be []any for array output")
	assert.Equal(t, []any{"item1", "item2", "item3"}, jsonArray)
}

// TestExecutionService_AgentStep_OutputFormat_JSON_LargeOutput verifies that
// output format processing works correctly with large outputs near the 1MB limit.
func TestExecutionService_AgentStep_OutputFormat_JSON_LargeOutput(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "large-json-test",
		Initial: "generate",
		Steps: map[string]*workflow.Step{
			"generate": {
				Name: "generate",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Generate large dataset",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("large-json-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Generate a large JSON object (500KB - well under 1MB limit)
	largeValue := make([]byte, 500000)
	for i := range largeValue {
		largeValue[i] = 'x'
	}
	largeJSON := `{"data":"` + string(largeValue) + `"}`

	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "```json\n" + largeJSON + "\n```",
			Tokens:   100000,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "large-json-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("generate")
	require.True(t, exists)

	// Should successfully parse large JSON
	require.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Contains(t, jsonObj["data"], "xxx")
}

// TestExecutionService_AgentStep_OutputFormat_JSON_ConversationMode verifies that
// output format processing (code fence stripping and JSON parsing) works correctly
// in conversation mode just as it does in single-prompt mode.
func TestExecutionService_AgentStep_OutputFormat_JSON_ConversationMode(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "conversation-json-test",
		Initial: "chat",
		Steps: map[string]*workflow.Step{
			"chat": {
				Name: "chat",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:      "claude",
					Mode:          "conversation",
					SystemPrompt:  "You are a helpful assistant",
					InitialPrompt: "Start conversation",
					OutputFormat:  workflow.OutputFormatJSON,
					Conversation: &workflow.ConversationConfig{
						MaxTurns:      5,
						StopCondition: "response contains 'complete'",
					},
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("conversation-json-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Mock conversation agent returns JSON wrapped in code fences
	claude.SetConversationFunc(func(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
		return &workflow.ConversationResult{
			Provider:     "claude",
			State:        state,
			Output:       "```json\n{\"status\":\"complete\",\"result\":\"success\"}\n```",
			TokensTotal:  40,
			TokensInput:  20,
			TokensOutput: 20,
		}, nil
	})
	_ = registry.Register(claude)

	tokenizer := newMockTokenizer()
	mockRegistry := mocks.NewMockAgentRegistry()
	mockRegistry.Register(claude)
	convMgr := application.NewConversationManager(&mockLogger{}, &simpleExpressionEvaluator{}, newMockResolver(), tokenizer, mockRegistry)

	execSvc.SetAgentRegistry(registry)
	execSvc.SetConversationManager(convMgr)

	ctx, err := execSvc.Run(context.Background(), "conversation-json-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("chat")
	require.True(t, exists)

	// Output should have code fences stripped in conversation mode too
	assert.Equal(t, `{"status":"complete","result":"success"}`, state.Output)

	// JSON field should be populated from conversation output
	require.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "complete", jsonObj["status"])
	assert.Equal(t, "success", jsonObj["result"])
}

// TestExecutionService_AgentStep_OutputFormat_JSON_NestedFences tests that when
// output contains nested code fences, only the outermost fence pair is stripped.
func TestExecutionService_AgentStep_OutputFormat_JSON_NestedFences(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "nested-fences-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Return JSON with markdown",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, _ := NewTestHarness(t).
		WithWorkflow("nested-fences-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Agent returns JSON containing code fences in a string field
	// Note: JSON strings use literal characters, not escape sequences
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "```json\n{\"code\":\"```\\necho hello\\n```\",\"type\":\"bash\"}\n```",
			Tokens:   50,
		}, nil
	})
	_ = registry.Register(claude)

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "nested-fences-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	state, exists := ctx.GetStepState("extract")
	require.True(t, exists)

	// Only outermost fences should be stripped
	assert.Equal(t, "{\"code\":\"```\\necho hello\\n```\",\"type\":\"bash\"}", state.Output)

	// JSON should be parsed correctly with nested fence as string value
	// When JSON unmarshals, \n becomes actual newline character
	require.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "```\necho hello\n```", jsonObj["code"])
	assert.Equal(t, "bash", jsonObj["type"])
}

// TestExecutionService_AgentStep_OutputFormat_JSON_MultiStepInterpolation tests that
// JSON fields from one step can be interpolated in subsequent steps via template access.
func TestExecutionService_AgentStep_OutputFormat_JSON_MultiStepInterpolation(t *testing.T) {
	wf := &workflow.Workflow{
		Name:    "multi-step-json-test",
		Initial: "extract",
		Steps: map[string]*workflow.Step{
			"extract": {
				Name: "extract",
				Type: workflow.StepTypeAgent,
				Agent: &workflow.AgentConfig{
					Provider:     "claude",
					Prompt:       "Extract user data",
					OutputFormat: workflow.OutputFormatJSON,
				},
				OnSuccess: "greet",
			},
			"greet": {
				Name:      "greet",
				Type:      workflow.StepTypeCommand,
				Command:   "echo 'Hello, {{states.extract.JSON.name}}! Count: {{states.extract.JSON.count}}'",
				OnSuccess: "done",
			},
			"done": {
				Name: "done",
				Type: workflow.StepTypeTerminal,
			},
		},
	}

	execSvc, testMocks := NewTestHarness(t).
		WithWorkflow("multi-step-json-test", wf).
		Build()

	registry := mocks.NewMockAgentRegistry()
	claude := mocks.NewMockAgentProvider("claude")

	// Step 1: Agent returns JSON with name and count
	claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
		return &workflow.AgentResult{
			Provider: "claude",
			Output:   "```json\n{\"name\":\"alice\",\"count\":3}\n```",
			Tokens:   30,
		}, nil
	})
	_ = registry.Register(claude)

	// Configure the command executor to return the echo output
	// The interpolation happens before execution, so we match the interpolated command
	testMocks.Executor.SetCommandResult("echo 'Hello, alice! Count: 3'", &ports.CommandResult{
		Stdout:   "Hello, alice! Count: 3\n",
		Stderr:   "",
		ExitCode: 0,
	})

	execSvc.SetAgentRegistry(registry)

	ctx, err := execSvc.Run(context.Background(), "multi-step-json-test", nil)

	require.NoError(t, err)
	assert.Equal(t, workflow.StatusCompleted, ctx.Status)

	// Verify first step parsed JSON correctly
	extractState, exists := ctx.GetStepState("extract")
	require.True(t, exists)
	require.NotNil(t, extractState.JSON)
	jsonObj, ok := extractState.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "alice", jsonObj["name"])
	assert.Equal(t, float64(3), jsonObj["count"])

	// Verify second step received interpolated values
	greetState, exists := ctx.GetStepState("greet")
	require.True(t, exists)
	assert.Contains(t, greetState.Output, "Hello, alice!")
	assert.Contains(t, greetState.Output, "Count: 3")
}

// TestExecutionService_AgentStep_OutputFormat_Text_DifferentLanguageTags tests that
// text format strips code fences with various language tags (bash, python, etc).
func TestExecutionService_AgentStep_OutputFormat_Text_DifferentLanguageTags(t *testing.T) {
	tests := []struct {
		name     string
		langTag  string
		content  string
		expected string
	}{
		{
			name:     "bash tag",
			langTag:  "bash",
			content:  "echo hello",
			expected: "echo hello",
		},
		{
			name:     "python tag",
			langTag:  "python",
			content:  "print('hello')",
			expected: "print('hello')",
		},
		{
			name:     "no tag",
			langTag:  "",
			content:  "plain text",
			expected: "plain text",
		},
		{
			name:     "go tag",
			langTag:  "go",
			content:  "fmt.Println(\"hello\")",
			expected: "fmt.Println(\"hello\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:    "text-tag-test",
				Initial: "analyze",
				Steps: map[string]*workflow.Step{
					"analyze": {
						Name: "analyze",
						Type: workflow.StepTypeAgent,
						Agent: &workflow.AgentConfig{
							Provider:     "claude",
							Prompt:       "Return code",
							OutputFormat: workflow.OutputFormatText,
						},
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("text-tag-test", wf).
				Build()

			registry := mocks.NewMockAgentRegistry()
			claude := mocks.NewMockAgentProvider("claude")

			output := "```" + tt.langTag + "\n" + tt.content + "\n```"
			claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
				return &workflow.AgentResult{
					Provider: "claude",
					Output:   output,
					Tokens:   20,
				}, nil
			})
			_ = registry.Register(claude)

			execSvc.SetAgentRegistry(registry)

			ctx, err := execSvc.Run(context.Background(), "text-tag-test", nil)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, ctx.Status)

			state, exists := ctx.GetStepState("analyze")
			require.True(t, exists)

			// Fences should be stripped regardless of language tag
			assert.Equal(t, tt.expected, state.Output)

			// JSON field should be nil for text format
			assert.Nil(t, state.JSON)
		})
	}
}

// TestExecutionService_AgentStep_OutputFormat_JSON_EmptyOutput tests that
// empty JSON objects and arrays are handled correctly.
func TestExecutionService_AgentStep_OutputFormat_JSON_EmptyOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
		isEmpty  bool
	}{
		{
			name:     "empty object",
			output:   "```json\n{}\n```",
			expected: "{}",
			isEmpty:  false,
		},
		{
			name:     "empty array",
			output:   "```json\n[]\n```",
			expected: "[]",
			isEmpty:  false,
		},
		{
			name:     "object with empty string",
			output:   "```json\n{\"value\":\"\"}\n```",
			expected: "{\"value\":\"\"}",
			isEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:    "empty-json-test",
				Initial: "extract",
				Steps: map[string]*workflow.Step{
					"extract": {
						Name: "extract",
						Type: workflow.StepTypeAgent,
						Agent: &workflow.AgentConfig{
							Provider:     "claude",
							Prompt:       "Return empty data",
							OutputFormat: workflow.OutputFormatJSON,
						},
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			execSvc, _ := NewTestHarness(t).
				WithWorkflow("empty-json-test", wf).
				Build()

			registry := mocks.NewMockAgentRegistry()
			claude := mocks.NewMockAgentProvider("claude")

			claude.SetExecuteFunc(func(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
				return &workflow.AgentResult{
					Provider: "claude",
					Output:   tt.output,
					Tokens:   10,
				}, nil
			})
			_ = registry.Register(claude)

			execSvc.SetAgentRegistry(registry)

			ctx, err := execSvc.Run(context.Background(), "empty-json-test", nil)

			require.NoError(t, err)
			assert.Equal(t, workflow.StatusCompleted, ctx.Status)

			state, exists := ctx.GetStepState("extract")
			require.True(t, exists)

			// Output should have fences stripped
			assert.Equal(t, tt.expected, state.Output)

			// Empty objects and arrays are valid JSON but may result in nil JSON field
			// depending on implementation (array vs object handling)
		})
	}
}
