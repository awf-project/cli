package transcript_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/awf-project/cli/internal/domain/transcript"
)

// TestBlockTypeEnumCoverage verifies all BlockType constants are defined with correct values
func TestBlockTypeEnumCoverage(t *testing.T) {
	assert.Equal(t, transcript.BlockType("text"), transcript.BlockTypeText)
	assert.Equal(t, transcript.BlockType("thinking"), transcript.BlockTypeThinking)
	assert.Equal(t, transcript.BlockType("tool_use"), transcript.BlockTypeToolUse)
	assert.Equal(t, transcript.BlockType("tool_result"), transcript.BlockTypeToolResult)
	assert.Equal(t, transcript.BlockType("command"), transcript.BlockTypeCommand)
	assert.Equal(t, transcript.BlockType("stream"), transcript.BlockTypeStream)
}

// TestFidelityEnumCoverage verifies all Fidelity constants are defined with correct values
func TestFidelityEnumCoverage(t *testing.T) {
	assert.Equal(t, transcript.Fidelity("router"), transcript.FidelityRouter)
	assert.Equal(t, transcript.Fidelity("agent_emitted"), transcript.FidelityAgentEmitted)
}

// TestValidBlockType_AllValidTypes verifies ValidBlockType returns true for all valid BlockType values
func TestValidBlockType_AllValidTypes(t *testing.T) {
	tests := []struct {
		name      string
		blockType transcript.BlockType
	}{
		{name: "text", blockType: transcript.BlockTypeText},
		{name: "thinking", blockType: transcript.BlockTypeThinking},
		{name: "tool_use", blockType: transcript.BlockTypeToolUse},
		{name: "tool_result", blockType: transcript.BlockTypeToolResult},
		{name: "command", blockType: transcript.BlockTypeCommand},
		{name: "stream", blockType: transcript.BlockTypeStream},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, transcript.ValidBlockType(tt.blockType))
		})
	}
}

// TestValidBlockType_InvalidType verifies ValidBlockType returns false for unknown BlockType
func TestValidBlockType_InvalidType(t *testing.T) {
	assert.False(t, transcript.ValidBlockType(transcript.BlockType("invalid")))
	assert.False(t, transcript.ValidBlockType(transcript.BlockType("")))
	assert.False(t, transcript.ValidBlockType(transcript.BlockType("unknown.type")))
}

// TestValidFidelity_AllValidTypes verifies ValidFidelity returns true for all valid Fidelity values
func TestValidFidelity_AllValidTypes(t *testing.T) {
	tests := []struct {
		name     string
		fidelity transcript.Fidelity
	}{
		{name: "router", fidelity: transcript.FidelityRouter},
		{name: "agent_emitted", fidelity: transcript.FidelityAgentEmitted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, transcript.ValidFidelity(tt.fidelity))
		})
	}
}

// TestValidFidelity_InvalidType verifies ValidFidelity returns false for unknown Fidelity
func TestValidFidelity_InvalidType(t *testing.T) {
	assert.False(t, transcript.ValidFidelity(transcript.Fidelity("invalid")))
	assert.False(t, transcript.ValidFidelity(transcript.Fidelity("")))
	assert.False(t, transcript.ValidFidelity(transcript.Fidelity("unknown.fidelity")))
}

// TestContentBlockFields verifies all ContentBlock fields are accessible
func TestContentBlockFields(t *testing.T) {
	block := transcript.ContentBlock{
		Type:        transcript.BlockTypeToolUse,
		Fidelity:    transcript.FidelityRouter,
		Text:        "sample text",
		Thinking:    "sample thinking",
		ToolName:    "bash",
		ToolID:      "tool-123",
		ToolInput:   map[string]string{"cmd": "ls"},
		ToolContent: "output",
		Command:     "bash -c 'ls'",
		Chunk:       "chunk-1",
	}

	assert.Equal(t, transcript.BlockTypeToolUse, block.Type)
	assert.Equal(t, transcript.FidelityRouter, block.Fidelity)
	assert.Equal(t, "sample text", block.Text)
	assert.Equal(t, "sample thinking", block.Thinking)
	assert.Equal(t, "bash", block.ToolName)
	assert.Equal(t, "tool-123", block.ToolID)
	assert.Equal(t, map[string]string{"cmd": "ls"}, block.ToolInput)
	assert.Equal(t, "output", block.ToolContent)
	assert.Equal(t, "bash -c 'ls'", block.Command)
	assert.Equal(t, "chunk-1", block.Chunk)
}

// TestContentBlockMarshalJSON_DeterministicFieldOrder verifies ContentBlock.MarshalJSON emits type,fidelity first
func TestContentBlockMarshalJSON_DeterministicFieldOrder(t *testing.T) {
	block := transcript.ContentBlock{
		Type:     transcript.BlockTypeText,
		Fidelity: transcript.FidelityRouter,
		Text:     "hello world",
	}

	data, err := json.Marshal(block)
	require.NoError(t, err)

	jsonStr := string(data)
	typePos := indexOfKey(jsonStr, "type")
	fidelityPos := indexOfKey(jsonStr, "fidelity")
	textPos := indexOfKey(jsonStr, "text")

	assert.True(t, typePos < fidelityPos, "type should come before fidelity")
	assert.True(t, fidelityPos < textPos, "fidelity should come before text")
}

// TestContentBlockMarshalJSON_AllBlockTypes verifies ContentBlock marshals all BlockType variants
func TestContentBlockMarshalJSON_AllBlockTypes(t *testing.T) {
	tests := []struct {
		name  string
		block transcript.ContentBlock
	}{
		{
			name: "text block",
			block: transcript.ContentBlock{
				Type:     transcript.BlockTypeText,
				Fidelity: transcript.FidelityRouter,
				Text:     "sample text",
			},
		},
		{
			name: "thinking block",
			block: transcript.ContentBlock{
				Type:     transcript.BlockTypeThinking,
				Fidelity: transcript.FidelityAgentEmitted,
				Thinking: "internal reasoning",
			},
		},
		{
			name: "tool_use block",
			block: transcript.ContentBlock{
				Type:      transcript.BlockTypeToolUse,
				Fidelity:  transcript.FidelityRouter,
				ToolName:  "bash",
				ToolID:    "call-1",
				ToolInput: map[string]string{"cmd": "echo test"},
			},
		},
		{
			name: "tool_result block",
			block: transcript.ContentBlock{
				Type:        transcript.BlockTypeToolResult,
				Fidelity:    transcript.FidelityRouter,
				ToolContent: "result data",
			},
		},
		{
			name: "command block",
			block: transcript.ContentBlock{
				Type:     transcript.BlockTypeCommand,
				Fidelity: transcript.FidelityRouter,
				Command:  "ls -la",
			},
		},
		{
			name: "stream block",
			block: transcript.ContentBlock{
				Type:     transcript.BlockTypeStream,
				Fidelity: transcript.FidelityAgentEmitted,
				Chunk:    "streaming data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.block)
			require.NoError(t, err)
			assert.NotNil(t, data)
		})
	}
}

// TestContentBlockRoundTrip verifies json.Marshal → json.Unmarshal recovers equality
func TestContentBlockRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		block transcript.ContentBlock
	}{
		{
			name: "text block round-trip",
			block: transcript.ContentBlock{
				Type:     transcript.BlockTypeText,
				Fidelity: transcript.FidelityRouter,
				Text:     "hello",
			},
		},
		{
			name: "tool_use block round-trip",
			block: transcript.ContentBlock{
				Type:      transcript.BlockTypeToolUse,
				Fidelity:  transcript.FidelityAgentEmitted,
				ToolName:  "bash",
				ToolID:    "id-123",
				ToolInput: map[string]interface{}{"arg": "value"},
			},
		},
		{
			name: "minimal block round-trip",
			block: transcript.ContentBlock{
				Type:     transcript.BlockTypeThinking,
				Fidelity: transcript.FidelityRouter,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.block)
			require.NoError(t, err)

			var recovered transcript.ContentBlock
			err = json.Unmarshal(data, &recovered)
			require.NoError(t, err)

			assert.Equal(t, tt.block, recovered)
		})
	}
}

// TestNULBytePreservation verifies NUL-byte survives JSON round-trip via standard escape
func TestNULBytePreservation(t *testing.T) {
	nulText := "hello" + string(byte(0)) + "world"
	block := transcript.ContentBlock{
		Type:     transcript.BlockTypeText,
		Fidelity: transcript.FidelityRouter,
		Text:     nulText,
	}

	data, err := json.Marshal(block)
	require.NoError(t, err)

	var recovered transcript.ContentBlock
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	assert.Equal(t, nulText, recovered.Text)
}

// TestMessagePayloadFields verifies all MessagePayload fields are accessible
func TestMessagePayloadFields(t *testing.T) {
	blocks := []transcript.ContentBlock{
		{
			Type:     transcript.BlockTypeText,
			Fidelity: transcript.FidelityRouter,
			Text:     "message text",
		},
	}
	payload := transcript.MessagePayload{
		Role:   "user",
		Blocks: blocks,
	}

	assert.Equal(t, "user", payload.Role)
	assert.Equal(t, blocks, payload.Blocks)
}

// TestMessagePayloadMarshalUnmarshal verifies MessagePayload round-trip
func TestMessagePayloadMarshalUnmarshal(t *testing.T) {
	payload := transcript.MessagePayload{
		Role: "assistant",
		Blocks: []transcript.ContentBlock{
			{
				Type:     transcript.BlockTypeText,
				Fidelity: transcript.FidelityAgentEmitted,
				Text:     "response",
			},
		},
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var recovered transcript.MessagePayload
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	assert.Equal(t, payload.Role, recovered.Role)
	assert.Len(t, recovered.Blocks, len(payload.Blocks))
}

// TestStepPayloadFields verifies all StepPayload fields are accessible
func TestStepPayloadFields(t *testing.T) {
	payload := transcript.StepPayload{
		Name:   "step-1",
		Kind:   "shell",
		Error:  "failed",
		Result: "error output",
	}

	assert.Equal(t, "step-1", payload.Name)
	assert.Equal(t, "shell", payload.Kind)
	assert.Equal(t, "failed", payload.Error)
	assert.Equal(t, "error output", payload.Result)
}

// TestStepPayloadMarshalUnmarshal verifies StepPayload round-trip with optional fields
func TestStepPayloadMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		payload transcript.StepPayload
	}{
		{
			name: "with error and result",
			payload: transcript.StepPayload{
				Name:   "failing-step",
				Kind:   "command",
				Error:  "exit code 1",
				Result: "failed",
			},
		},
		{
			name: "without optional fields",
			payload: transcript.StepPayload{
				Name: "successful-step",
				Kind: "script",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			var recovered transcript.StepPayload
			err = json.Unmarshal(data, &recovered)
			require.NoError(t, err)

			assert.Equal(t, tt.payload, recovered)
		})
	}
}

// TestToolPayloadFields verifies all ToolPayload fields are accessible
func TestToolPayloadFields(t *testing.T) {
	payload := transcript.ToolPayload{
		Name:     "bash",
		CallID:   "call-123",
		Input:    map[string]string{"cmd": "echo"},
		Output:   "result",
		Fidelity: transcript.FidelityRouter,
	}

	assert.Equal(t, "bash", payload.Name)
	assert.Equal(t, "call-123", payload.CallID)
	assert.Equal(t, map[string]string{"cmd": "echo"}, payload.Input)
	assert.Equal(t, "result", payload.Output)
	assert.Equal(t, transcript.FidelityRouter, payload.Fidelity)
}

// TestToolPayloadMarshalUnmarshal verifies ToolPayload round-trip
func TestToolPayloadMarshalUnmarshal(t *testing.T) {
	payload := transcript.ToolPayload{
		Name:     "grep",
		CallID:   "call-456",
		Input:    map[string]interface{}{"pattern": "error", "file": "app.log"},
		Output:   []string{"line 1", "line 2"},
		Fidelity: transcript.FidelityAgentEmitted,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var recovered transcript.ToolPayload
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	assert.Equal(t, payload.Name, recovered.Name)
	assert.Equal(t, payload.CallID, recovered.CallID)
	assert.Equal(t, payload.Fidelity, recovered.Fidelity)
}

// TestContentBlockUnmarshalJSON_UnknownBlockType verifies unknown BlockType returns ErrUnknownBlockType
func TestContentBlockUnmarshalJSON_UnknownBlockType(t *testing.T) {
	data := []byte(`{
		"type": "unknown.block",
		"fidelity": "router"
	}`)

	var block transcript.ContentBlock
	err := json.Unmarshal(data, &block)
	assert.ErrorIs(t, err, transcript.ErrUnknownBlockType)
}

// TestMessagePayloadWithMultipleBlocks verifies MessagePayload with multiple ContentBlocks
func TestMessagePayloadWithMultipleBlocks(t *testing.T) {
	payload := transcript.MessagePayload{
		Role: "assistant",
		Blocks: []transcript.ContentBlock{
			{
				Type:     transcript.BlockTypeThinking,
				Fidelity: transcript.FidelityAgentEmitted,
				Thinking: "let me think",
			},
			{
				Type:     transcript.BlockTypeText,
				Fidelity: transcript.FidelityAgentEmitted,
				Text:     "here is my response",
			},
			{
				Type:      transcript.BlockTypeToolUse,
				Fidelity:  transcript.FidelityAgentEmitted,
				ToolName:  "bash",
				ToolID:    "id-1",
				ToolInput: map[string]string{"cmd": "ls"},
			},
		},
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var recovered transcript.MessagePayload
	err = json.Unmarshal(data, &recovered)
	require.NoError(t, err)

	assert.Len(t, recovered.Blocks, 3)
	assert.Equal(t, transcript.BlockTypeThinking, recovered.Blocks[0].Type)
	assert.Equal(t, transcript.BlockTypeText, recovered.Blocks[1].Type)
	assert.Equal(t, transcript.BlockTypeToolUse, recovered.Blocks[2].Type)
}
