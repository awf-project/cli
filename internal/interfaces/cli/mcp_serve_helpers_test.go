package cli

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPortSchemaToMCP covers the conversion from a ports.ToolDefinition.InputSchema
// (map[string]any) to a mcpserver.InputSchema struct. These are the cases most
// likely to produce zero values or panics in production.
func TestPortSchemaToMCP(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		wantType string
	}{
		{
			name:     "nil schema defaults to object",
			input:    nil,
			wantType: "object",
		},
		{
			name:     "empty schema defaults to object",
			input:    map[string]any{},
			wantType: "object",
		},
		{
			name:     "empty Type field defaults to object",
			input:    map[string]any{"type": ""},
			wantType: "object",
		},
		{
			name:     "explicit object type preserved",
			input:    map[string]any{"type": "object"},
			wantType: "object",
		},
		{
			name:     "schema with properties round-trips type",
			input:    map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}},
			wantType: "object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := portSchemaToMCP(tt.input)
			assert.Equal(t, tt.wantType, got.Type,
				"portSchemaToMCP(%v).Type = %q, want %q", tt.input, got.Type, tt.wantType)
		})
	}
}

// TestPortResultToMCP covers the conversion from *ports.ToolResult to mcpserver.Result.
func TestPortResultToMCP(t *testing.T) {
	tests := []struct {
		name        string
		input       *ports.ToolResult
		wantIsError bool
		wantLen     int
	}{
		{
			name:        "nil Content slice produces empty result",
			input:       &ports.ToolResult{Content: nil, IsError: false},
			wantIsError: false,
			wantLen:     0,
		},
		{
			name:        "empty Content slice produces empty result",
			input:       &ports.ToolResult{Content: []ports.ToolContent{}, IsError: false},
			wantIsError: false,
			wantLen:     0,
		},
		{
			name: "IsError true propagated",
			input: &ports.ToolResult{
				Content: []ports.ToolContent{{Type: "text", Text: "boom"}},
				IsError: true,
			},
			wantIsError: true,
			wantLen:     1,
		},
		{
			name: "multiple content blocks",
			input: &ports.ToolResult{
				Content: []ports.ToolContent{
					{Type: "text", Text: "first"},
					{Type: "text", Text: "second"},
				},
				IsError: false,
			},
			wantIsError: false,
			wantLen:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := portResultToMCP(tt.input)
			assert.Equal(t, tt.wantIsError, got.IsError)
			require.Len(t, got.Content, tt.wantLen)

			// Verify each ContentBlock is correctly mapped.
			for i, c := range tt.input.Content {
				assert.Equal(t, c.Type, got.Content[i].Type, "content[%d].Type mismatch", i)
				assert.Equal(t, c.Text, got.Content[i].Text, "content[%d].Text mismatch", i)
			}
		})
	}
}

// TestPortResultToMCP_IsErrorAndError verifies the combination of IsError:true
// and a non-empty Content field is correctly mapped.
func TestPortResultToMCP_IsErrorAndError(t *testing.T) {
	input := &ports.ToolResult{
		Content: []ports.ToolContent{{Type: "text", Text: "something failed"}},
		IsError: true,
	}
	got := portResultToMCP(input)

	assert.True(t, got.IsError, "IsError must be preserved")
	require.Len(t, got.Content, 1)
	assert.Equal(t, "text", got.Content[0].Type)
	assert.Equal(t, "something failed", got.Content[0].Text)
}
