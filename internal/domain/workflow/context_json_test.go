package workflow_test

import (
	"encoding/json"
	"testing"

	"github.com/awf-project/awf/internal/domain/workflow"
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
