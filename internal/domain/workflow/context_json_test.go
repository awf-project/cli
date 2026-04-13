package workflow_test

import (
	"encoding/json"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: T003
// Feature: F065

func TestStepState_JSONField_NilByDefault(t *testing.T) {
	state := workflow.StepState{
		Name:   "test-step",
		Status: workflow.StatusPending,
		Output: "raw output",
	}

	assert.Nil(t, state.JSON, "JSON should be nil when output_format is not specified")
}

func TestStepState_JSONField_ParsedJSONObject(t *testing.T) {
	state := workflow.StepState{
		Name:   "agent-step",
		Status: workflow.StatusCompleted,
		Output: `{"name":"alice","count":3}`,
		JSON: map[string]any{
			"name":  "alice",
			"count": float64(3),
		},
	}

	assert.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	assert.Equal(t, "alice", jsonObj["name"])
	assert.Equal(t, float64(3), jsonObj["count"])
}

func TestStepState_JSONField_ParsedJSONArray(t *testing.T) {
	state := workflow.StepState{
		Name:   "agent-step",
		Status: workflow.StatusCompleted,
		Output: `[1,2,3]`,
		JSON:   []any{float64(1), float64(2), float64(3)},
	}

	assert.NotNil(t, state.JSON)
	arr, ok := state.JSON.([]any)
	require.True(t, ok, "JSON should be []any for array output")
	assert.Len(t, arr, 3)
	assert.Equal(t, float64(1), arr[0])
}

func TestStepState_JSONField_NestedObject(t *testing.T) {
	state := workflow.StepState{
		Name:   "complex-agent",
		Status: workflow.StatusCompleted,
		JSON: map[string]any{
			"user": map[string]any{
				"name": "bob",
				"age":  float64(30),
			},
			"status": "active",
		},
	}

	assert.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any for object output")
	user, ok := jsonObj["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bob", user["name"])
	assert.Equal(t, float64(30), user["age"])
}

func TestStepState_JSONField_EmptyObject(t *testing.T) {
	state := workflow.StepState{
		Name:   "empty-json",
		Status: workflow.StatusCompleted,
		Output: "{}",
		JSON:   map[string]any{},
	}

	assert.NotNil(t, state.JSON)
	assert.Empty(t, state.JSON)
}

func TestStepState_JSONField_JSONSerialization(t *testing.T) {
	original := workflow.StepState{
		Name:   "json-step",
		Status: workflow.StatusCompleted,
		Output: `{"key":"value"}`,
		JSON: map[string]any{
			"key": "value",
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Name, decoded.Name)
	assert.NotNil(t, decoded.JSON)
	jsonObj, ok := decoded.JSON.(map[string]any)
	require.True(t, ok, "JSON should deserialize as map[string]any")
	assert.Equal(t, "value", jsonObj["key"])
}

func TestStepState_JSONField_NilSerialization(t *testing.T) {
	state := workflow.StepState{
		Name:   "no-json",
		Status: workflow.StatusCompleted,
		Output: "plain text",
		JSON:   nil,
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Nil(t, decoded.JSON)
}

func TestStepState_JSONField_ComplexTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "string", value: "test"},
		{name: "number", value: float64(42)},
		{name: "boolean", value: true},
		{name: "null", value: nil},
		{name: "array", value: []any{"a", "b", "c"}},
		{name: "nested_map", value: map[string]any{"inner": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := workflow.StepState{
				Name:   "type-test",
				Status: workflow.StatusCompleted,
				JSON: map[string]any{
					"field": tt.value,
				},
			}

			assert.NotNil(t, state.JSON)
			jsonObj, ok := state.JSON.(map[string]any)
			require.True(t, ok, "JSON should be map[string]any")
			assert.Equal(t, tt.value, jsonObj["field"])
		})
	}
}

func TestStepState_JSONField_ExecutionContextIntegration(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-id", "test-workflow")

	state := workflow.StepState{
		Name:   "agent-json-step",
		Status: workflow.StatusCompleted,
		Output: `{"result":"success"}`,
		JSON: map[string]any{
			"result": "success",
		},
	}

	ctx.SetStepState("agent-json-step", state)

	retrieved, ok := ctx.GetStepState("agent-json-step")
	require.True(t, ok)
	assert.NotNil(t, retrieved.JSON)
	jsonObj, ok := retrieved.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, "success", jsonObj["result"])
}

func TestStepState_JSONField_MixedStepsInContext(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-id", "mixed-workflow")

	// Step with no JSON (output_format not specified)
	shellStep := workflow.StepState{
		Name:   "shell-command",
		Status: workflow.StatusCompleted,
		Output: "plain output",
		JSON:   nil,
	}

	// Step with JSON (output_format: json)
	jsonStep := workflow.StepState{
		Name:   "agent-json",
		Status: workflow.StatusCompleted,
		Output: `{"status":"ok"}`,
		JSON: map[string]any{
			"status": "ok",
		},
	}

	// Step with text output_format (JSON is nil)
	textStep := workflow.StepState{
		Name:   "agent-text",
		Status: workflow.StatusCompleted,
		Output: "cleaned text output",
		JSON:   nil,
	}

	ctx.SetStepState("shell-command", shellStep)
	ctx.SetStepState("agent-json", jsonStep)
	ctx.SetStepState("agent-text", textStep)

	shell, ok := ctx.GetStepState("shell-command")
	require.True(t, ok)
	assert.Nil(t, shell.JSON)

	agentJSON, ok := ctx.GetStepState("agent-json")
	require.True(t, ok)
	assert.NotNil(t, agentJSON.JSON)
	jsonObj, ok := agentJSON.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, "ok", jsonObj["status"])

	text, ok := ctx.GetStepState("agent-text")
	require.True(t, ok)
	assert.Nil(t, text.JSON)
}

func TestStepState_JSONField_LargeObject(t *testing.T) {
	largeJSON := make(map[string]any)
	for i := 0; i < 100; i++ {
		largeJSON[string(rune('a'+i%26))+string(rune('0'+i/26))] = float64(i)
	}

	state := workflow.StepState{
		Name:   "large-json",
		Status: workflow.StatusCompleted,
		JSON:   largeJSON,
	}

	assert.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Len(t, jsonObj, 100)
}

func TestStepState_JSONField_DeepNesting(t *testing.T) {
	deepJSON := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"level4": map[string]any{
						"value": "deep",
					},
				},
			},
		},
	}

	state := workflow.StepState{
		Name:   "deep-nested",
		Status: workflow.StatusCompleted,
		JSON:   deepJSON,
	}

	assert.NotNil(t, state.JSON)
	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	level1, ok := jsonObj["level1"].(map[string]any)
	require.True(t, ok)
	level2, ok := level1["level2"].(map[string]any)
	require.True(t, ok)
	level3, ok := level2["level3"].(map[string]any)
	require.True(t, ok)
	level4, ok := level3["level4"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "deep", level4["value"])
}

func TestStepState_JSONField_SpecialCharacters(t *testing.T) {
	state := workflow.StepState{
		Name:   "special-chars",
		Status: workflow.StatusCompleted,
		JSON: map[string]any{
			"unicode":   "Hello 世界",
			"emoji":     "🚀",
			"escaped":   "line1\nline2\ttab",
			"quotes":    `"quoted"`,
			"backslash": `path\to\file`,
		},
	}

	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, "Hello 世界", jsonObj["unicode"])
	assert.Equal(t, "🚀", jsonObj["emoji"])
	assert.Equal(t, "line1\nline2\ttab", jsonObj["escaped"])
}

func TestStepState_JSONField_NumberTypes(t *testing.T) {
	state := workflow.StepState{
		Name:   "numbers",
		Status: workflow.StatusCompleted,
		JSON: map[string]any{
			"integer":  float64(42),
			"negative": float64(-100),
			"decimal":  3.14159,
			"zero":     float64(0),
			"large":    float64(1e10),
		},
	}

	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")
	assert.Equal(t, float64(42), jsonObj["integer"])
	assert.Equal(t, float64(-100), jsonObj["negative"])
	assert.Equal(t, 3.14159, jsonObj["decimal"])
	assert.Equal(t, float64(0), jsonObj["zero"])
	assert.Equal(t, float64(1e10), jsonObj["large"])
}

// Component: T001
// Feature: F082
func TestStepState_DisplayOutput_AbsentFromMarshaledJSON(t *testing.T) {
	state := workflow.StepState{
		Name:          "agent-step",
		Status:        workflow.StatusCompleted,
		Output:        `{"type":"result","text":"hello"}`,
		DisplayOutput: "hello",
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.NotContains(t, raw, "DisplayOutput", "DisplayOutput must not appear in marshaled JSON")

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.DisplayOutput, "DisplayOutput must be empty after round-trip JSON deserialization")
}

func TestStepState_JSONField_ArrayTypes(t *testing.T) {
	state := workflow.StepState{
		Name:   "arrays",
		Status: workflow.StatusCompleted,
		JSON: map[string]any{
			"strings": []any{"a", "b", "c"},
			"numbers": []any{float64(1), float64(2), float64(3)},
			"mixed":   []any{"text", float64(42), true, nil},
			"empty":   []any{},
			"nested":  []any{[]any{float64(1), float64(2)}, []any{float64(3), float64(4)}},
		},
	}

	jsonObj, ok := state.JSON.(map[string]any)
	require.True(t, ok, "JSON should be map[string]any")

	strArr, ok := jsonObj["strings"].([]any)
	require.True(t, ok)
	assert.Equal(t, "a", strArr[0])

	mixedArr, ok := jsonObj["mixed"].([]any)
	require.True(t, ok)
	assert.Equal(t, "text", mixedArr[0])
	assert.Equal(t, float64(42), mixedArr[1])
	assert.Equal(t, true, mixedArr[2])
	assert.Nil(t, mixedArr[3])
}

// Component: T001
// Feature: F082
// AgentResult DisplayOutput tests

func TestAgentResult_DisplayOutput_HappyPath(t *testing.T) {
	result := workflow.NewAgentResult("claude")
	result.Output = `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`
	result.DisplayOutput = "hello"

	assert.Equal(t, "hello", result.DisplayOutput)
	assert.NotEmpty(t, result.Output)
	assert.NotEqual(t, result.Output, result.DisplayOutput)
}

func TestAgentResult_DisplayOutput_EmptyWhenNoParser(t *testing.T) {
	result := workflow.NewAgentResult("claude")
	result.Output = `raw ndjson`
	result.DisplayOutput = ""

	assert.Empty(t, result.DisplayOutput)
	assert.NotEmpty(t, result.Output)
}

func TestAgentResult_DisplayOutput_JSONPassthrough(t *testing.T) {
	result := workflow.NewAgentResult("claude")
	result.Output = `{"data": "raw"}`
	result.DisplayOutput = ""

	assert.Empty(t, result.DisplayOutput, "DisplayOutput should be empty for output_format: json")
	assert.Equal(t, `{"data": "raw"}`, result.Output)
}

func TestAgentResult_DisplayOutput_MultilineText(t *testing.T) {
	result := workflow.NewAgentResult("claude")
	result.Output = `line1\nline2\nline3`
	result.DisplayOutput = "line1\nline2\nline3"

	assert.Contains(t, result.DisplayOutput, "line1")
	assert.Contains(t, result.DisplayOutput, "line3")
}

func TestAgentResult_DisplayOutput_Preserved(t *testing.T) {
	tests := []struct {
		name          string
		displayOutput string
	}{
		{name: "short text", displayOutput: "test"},
		{name: "long text", displayOutput: "a very long response from the agent with multiple lines and content"},
		{name: "empty", displayOutput: ""},
		{name: "special chars", displayOutput: "Hello 世界 🚀"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := workflow.NewAgentResult("claude")
			result.DisplayOutput = tt.displayOutput

			assert.Equal(t, tt.displayOutput, result.DisplayOutput)
		})
	}
}

// Component: T001
// Feature: F082
// ConversationResult DisplayOutput tests

func TestConversationResult_DisplayOutput_HappyPath(t *testing.T) {
	result := workflow.NewConversationResult("claude")
	result.Output = `{"type":"content_block_delta"}`
	result.DisplayOutput = "extracted text from conversation"

	assert.Equal(t, "extracted text from conversation", result.DisplayOutput)
	assert.NotEmpty(t, result.Output)
}

func TestConversationResult_DisplayOutput_EmptyByDefault(t *testing.T) {
	result := workflow.NewConversationResult("claude")
	result.Output = "raw response"

	assert.Empty(t, result.DisplayOutput, "DisplayOutput should be empty on creation")
	assert.NotEmpty(t, result.Output)
}

func TestConversationResult_DisplayOutput_CanBeSetToAnyValue(t *testing.T) {
	result := workflow.NewConversationResult("gemini")
	testText := "Multi-line\nconversation\nresponse"
	result.DisplayOutput = testText

	assert.Equal(t, testText, result.DisplayOutput)
}

func TestConversationResult_DisplayOutput_IndependentFromOutput(t *testing.T) {
	result := workflow.NewConversationResult("claude")
	result.Output = `{"ndjson": "format"}`
	result.DisplayOutput = "filtered text"

	assert.NotEqual(t, result.Output, result.DisplayOutput)
	assert.Contains(t, result.Output, "ndjson")
	assert.NotContains(t, result.DisplayOutput, "ndjson")
}

// Component: T001
// Feature: F082
// StepState DisplayOutput serialization tests

func TestStepState_DisplayOutput_EmptyAfterCreation(t *testing.T) {
	state := workflow.StepState{
		Name:   "agent-step",
		Status: workflow.StatusCompleted,
	}

	assert.Empty(t, state.DisplayOutput)
}

func TestStepState_DisplayOutput_CanBePopulated(t *testing.T) {
	state := workflow.StepState{
		Name:          "agent-step",
		Status:        workflow.StatusCompleted,
		Output:        `{"text":"raw"}`,
		DisplayOutput: "filtered text",
	}

	assert.Equal(t, "filtered text", state.DisplayOutput)
	assert.NotEqual(t, state.Output, state.DisplayOutput)
}

func TestStepState_DisplayOutput_PreservedInMemory(t *testing.T) {
	state := workflow.StepState{
		Name:          "test",
		Status:        workflow.StatusRunning,
		DisplayOutput: "test display content",
	}

	assert.Equal(t, "test display content", state.DisplayOutput)
}

func TestStepState_DisplayOutput_NotPersisted_RoundTrip(t *testing.T) {
	original := workflow.StepState{
		Name:          "test-step",
		Status:        workflow.StatusCompleted,
		Output:        "raw output",
		DisplayOutput: "display output that should not persist",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.DisplayOutput, "DisplayOutput must be empty after round-trip (json:\"-\")")
	assert.Equal(t, "raw output", decoded.Output, "Output must be preserved")
}

func TestStepState_DisplayOutput_ExcludedFromJSONKeys(t *testing.T) {
	state := workflow.StepState{
		Name:          "step",
		Status:        workflow.StatusCompleted,
		Output:        "raw",
		DisplayOutput: "display",
		Response:      map[string]any{"key": "value"},
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.NotContains(t, raw, "DisplayOutput", "DisplayOutput must be excluded from JSON")
	assert.Contains(t, raw, "Output", "Output must be included in JSON")
	assert.Contains(t, raw, "Response", "Response must be included in JSON")
}

func TestStepState_DisplayOutput_WithOtherFields(t *testing.T) {
	state := workflow.StepState{
		Name:          "complex-step",
		Status:        workflow.StatusCompleted,
		Output:        "raw output content",
		DisplayOutput: "display content",
		Stderr:        "error logs",
		ExitCode:      0,
		Response: map[string]any{
			"result": "success",
		},
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.DisplayOutput, "DisplayOutput must not persist")
	assert.Equal(t, "raw output content", decoded.Output, "Output must persist")
	assert.Equal(t, "error logs", decoded.Stderr, "Stderr must persist")
	assert.Equal(t, 0, decoded.ExitCode, "ExitCode must persist")
	assert.NotNil(t, decoded.Response)
}

func TestStepState_DisplayOutput_InExecutionContext(t *testing.T) {
	ctx := workflow.NewExecutionContext("test-id", "test-workflow")

	state := workflow.StepState{
		Name:          "agent-step",
		Status:        workflow.StatusCompleted,
		Output:        `raw ndjson`,
		DisplayOutput: "human readable text",
	}

	ctx.SetStepState("agent-step", state)

	retrieved, ok := ctx.GetStepState("agent-step")
	require.True(t, ok)

	assert.Equal(t, "human readable text", retrieved.DisplayOutput)
	assert.Equal(t, `raw ndjson`, retrieved.Output)
}

func TestStepState_DisplayOutput_LargeContent(t *testing.T) {
	largeDisplay := ""
	for i := 0; i < 1000; i++ {
		largeDisplay += "line\n"
	}

	state := workflow.StepState{
		Name:          "large-step",
		Status:        workflow.StatusCompleted,
		DisplayOutput: largeDisplay,
	}

	assert.NotEmpty(t, state.DisplayOutput)
	assert.True(t, len(state.DisplayOutput) > 1000)

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.DisplayOutput)
}

func TestStepState_DisplayOutput_WithConversationState(t *testing.T) {
	state := workflow.StepState{
		Name:          "conversation-step",
		Status:        workflow.StatusCompleted,
		Output:        `conversation raw`,
		DisplayOutput: "conversation display",
		Conversation: &workflow.ConversationState{
			TotalTurns:  2,
			TotalTokens: 100,
		},
	}

	assert.Equal(t, "conversation display", state.DisplayOutput)
	assert.NotNil(t, state.Conversation)

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var decoded workflow.StepState
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.DisplayOutput, "DisplayOutput excluded from JSON")
	assert.NotNil(t, decoded.Conversation, "Conversation state must persist")
}
