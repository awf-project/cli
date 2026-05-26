package agents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssembleToolCalls_SingleToolCallSingleChunk(t *testing.T) {
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-123",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "get_weather",
				Arguments: `{"city":"London"}`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "call-123", result[0].ID)
	assert.Equal(t, "get_weather", result[0].Name)
	assert.Equal(t, map[string]any{"city": "London"}, result[0].Arguments)
}

func TestAssembleToolCalls_SingleToolCallMultipleChunks(t *testing.T) {
	// Simulate tool arguments split across 3 chunks: {"path": "/tmp/foo"}
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-456",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "read_file",
				Arguments: `{"path": "/`,
			},
		},
		{
			Index: 0,
			ID:    "call-456",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "read_file",
				Arguments: `tmp/foo`,
			},
		},
		{
			Index: 0,
			ID:    "call-456",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "read_file",
				Arguments: `"}`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "call-456", result[0].ID)
	assert.Equal(t, "read_file", result[0].Name)
	expectedArgs := map[string]any{"path": "/tmp/foo"}
	assert.Equal(t, expectedArgs, result[0].Arguments)
}

func TestAssembleToolCalls_MultipleParallelToolCalls(t *testing.T) {
	// Two tool calls arriving in mixed order (indices 0 and 1)
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-001",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "add",
				Arguments: `{"a":2,"b":3}`,
			},
		},
		{
			Index: 1,
			ID:    "call-002",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "multiply",
				Arguments: `{"x":4,"y":5}`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "call-001", result[0].ID)
	assert.Equal(t, "add", result[0].Name)
	assert.Equal(t, map[string]any{"a": float64(2), "b": float64(3)}, result[0].Arguments)

	assert.Equal(t, "call-002", result[1].ID)
	assert.Equal(t, "multiply", result[1].Name)
	assert.Equal(t, map[string]any{"x": float64(4), "y": float64(5)}, result[1].Arguments)
}

func TestAssembleToolCalls_OutOfOrderIndices(t *testing.T) {
	// Deltas arriving out of order: index 1, then 0
	deltas := []ToolCallDelta{
		{
			Index: 1,
			ID:    "call-b",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "b_func",
				Arguments: `{"b":2}`,
			},
		},
		{
			Index: 0,
			ID:    "call-a",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "a_func",
				Arguments: `{"a":1}`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 2)

	// Results should be keyed by index, so index 0 comes first
	assert.Equal(t, "call-a", result[0].ID)
	assert.Equal(t, "a_func", result[0].Name)

	assert.Equal(t, "call-b", result[1].ID)
	assert.Equal(t, "b_func", result[1].Name)
}

func TestAssembleToolCalls_InvalidJSONArguments(t *testing.T) {
	// Assembled arguments are not valid JSON
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-bad",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "bad_func",
				Arguments: `{invalid json]`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad_func", "error should identify the offending tool call by name")
	assert.Nil(t, result)
}

func TestAssembleToolCalls_InvalidJSONAfterMultiChunkAssembly(t *testing.T) {
	// Valid chunks individually but invalid JSON when assembled
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-bad",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "bad_func",
				Arguments: `{"key": "val`,
			},
		},
		{
			Index: 0,
			ID:    "call-bad",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "bad_func",
				Arguments: `ue"]]`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad_func", "error should identify the offending tool call by name")
	assert.Nil(t, result)
}

func TestAssembleToolCalls_EmptyDeltas(t *testing.T) {
	deltas := []ToolCallDelta{}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestAssembleToolCalls_ComplexNestedArguments(t *testing.T) {
	complexArgs := map[string]any{
		"config": map[string]any{
			"nested": map[string]any{
				"value": "deep",
			},
			"list": []any{"a", "b", "c"},
		},
		"count": float64(42),
	}
	argsJSON, _ := json.Marshal(complexArgs)

	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-complex",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "complex_func",
				Arguments: string(argsJSON),
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "call-complex", result[0].ID)
	assert.Equal(t, "complex_func", result[0].Name)
	assert.Equal(t, complexArgs, result[0].Arguments)
}

func TestAssembleToolCalls_EmptyArgumentsString(t *testing.T) {
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-empty",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "no_args_func",
				Arguments: `{}`,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, map[string]any{}, result[0].Arguments)
}

func TestAssembleToolCalls_ToolCallDeltaWithPartialName(t *testing.T) {
	// Function name also split across chunks (though less common)
	// First delta has the main name portion, subsequent deltas supplement it
	deltas := []ToolCallDelta{
		{
			Index: 0,
			ID:    "call-split-name",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "get_info",
				Arguments: `{"id":"123"}`,
			},
		},
		{
			Index: 0,
			ID:    "call-split-name",
			Type:  "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "",
				Arguments: ``,
			},
		},
	}

	result, err := assembleToolCalls(deltas)

	require.NoError(t, err)
	require.Len(t, result, 1)

	// The name from the first occurrence should be preserved
	assert.Equal(t, "get_info", result[0].Name)
}
