package agents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ToolDefinition is the JSON representation of a tool for the Chat Completions API.
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function toolFunctionSchema `json:"function"`
}

type toolFunctionSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolCallDelta is one SSE chunk contributing to a streamed tool call.
// Multiple deltas with the same index form a single tool call; their
// function.arguments fields must be concatenated in order before parsing.
type ToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolCall is a fully assembled tool call derived from one or more SSE deltas.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// assembleToolCalls merges ToolCallDelta slices into complete ToolCalls.
// Chunks for the same tool are identified by index; argument fragments are
// concatenated in arrival order, then validated as JSON after assembly.
// Out-of-order index values are handled via a map keyed by index.
func assembleToolCalls(deltas []ToolCallDelta) ([]ToolCall, error) {
	type accumulator struct {
		id   string
		name string
		args strings.Builder
	}

	byIndex := make(map[int]*accumulator)
	indices := []int{}

	for _, d := range deltas {
		acc, exists := byIndex[d.Index]
		if !exists {
			acc = &accumulator{}
			byIndex[d.Index] = acc
			indices = append(indices, d.Index)
		}
		if acc.id == "" && d.ID != "" {
			acc.id = d.ID
		}
		if acc.name == "" && d.Function.Name != "" {
			acc.name = d.Function.Name
		}
		acc.args.WriteString(d.Function.Arguments)
	}

	sort.Ints(indices)

	result := make([]ToolCall, 0, len(indices))
	for _, idx := range indices {
		acc := byIndex[idx]
		var args map[string]any
		if err := json.Unmarshal([]byte(acc.args.String()), &args); err != nil {
			return nil, fmt.Errorf("tool call %q has invalid JSON arguments: %w", acc.name, err)
		}
		result = append(result, ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: args,
		})
	}

	return result, nil
}
