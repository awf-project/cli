package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDisplayEventStructure verifies DisplayEvent has Type and Text fields
func TestDisplayEventStructure(t *testing.T) {
	evt := DisplayEvent{
		Type: "message",
		Text: "hello world",
	}

	assert.Equal(t, "message", evt.Type)
	assert.Equal(t, "hello world", evt.Text)
}

// TestDisplayEventZeroValueHasEmptyText verifies empty Text signals line should be skipped
func TestDisplayEventZeroValueHasEmptyText(t *testing.T) {
	evt := DisplayEvent{}

	assert.Empty(t, evt.Type)
	assert.Empty(t, evt.Text)
}

// TestDisplayEventParserTypeSignature verifies DisplayEventParser has correct signature
func TestDisplayEventParserTypeSignature(t *testing.T) {
	parser := DisplayEventParser(func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Type: "test", Text: "parsed"}}
	})

	result := parser([]byte("input"))

	require.Len(t, result, 1)
	assert.Equal(t, "test", result[0].Type)
	assert.Equal(t, "parsed", result[0].Text)
}

// TestDisplayEventParserReturnsNilForNonDisplayableContent verifies
// implementations can return nil to indicate no displayable content
func TestDisplayEventParserReturnsNilForNonDisplayableContent(t *testing.T) {
	parser := DisplayEventParser(func(line []byte) []DisplayEvent {
		// Parser returns nil to indicate line carries no displayable content
		return nil
	})

	result := parser([]byte("input"))

	assert.Empty(t, result)
}

// TestDisplayEventParserWithDifferentInputTypes tests parser with various input
func TestDisplayEventParserWithDifferentInputTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		parser   DisplayEventParser
		wantLen  int
		wantType string
		wantText string
	}{
		{
			name:  "parser with non-empty result",
			input: []byte("test input"),
			parser: func(line []byte) []DisplayEvent {
				return []DisplayEvent{{Type: "output", Text: "processed"}}
			},
			wantLen:  1,
			wantType: "output",
			wantText: "processed",
		},
		{
			name:  "parser with empty result for empty input",
			input: []byte(""),
			parser: func(line []byte) []DisplayEvent {
				if len(line) == 0 {
					return nil
				}
				return []DisplayEvent{{Type: "output", Text: "data"}}
			},
			wantLen:  0,
			wantType: "",
			wantText: "",
		},
		{
			name:  "parser extracting text from input",
			input: []byte("some output"),
			parser: func(line []byte) []DisplayEvent {
				return []DisplayEvent{{Type: "data", Text: string(line)}}
			},
			wantLen:  1,
			wantType: "data",
			wantText: "some output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.parser(tt.input)

			assert.Len(t, result, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantType, result[0].Type)
				assert.Equal(t, tt.wantText, result[0].Text)
			}
		})
	}
}

// TestDisplayEventParserImplementationAssignment verifies various implementations can be assigned
func TestDisplayEventParserImplementationAssignment(t *testing.T) {
	// Create multiple implementations
	simpleParser := func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Type: "simple", Text: "data"}}
	}

	conditionalParser := func(line []byte) []DisplayEvent {
		if len(line) > 0 {
			return []DisplayEvent{{Type: "conditional", Text: string(line)}}
		}
		return nil
	}

	// Verify both can be assigned to DisplayEventParser
	var p1 DisplayEventParser = simpleParser
	var p2 DisplayEventParser = conditionalParser

	require.NotNil(t, p1)
	require.NotNil(t, p2)

	result1 := p1([]byte("test"))
	result2 := p2([]byte("test"))

	require.Len(t, result1, 1)
	require.Len(t, result2, 1)
	assert.Equal(t, "simple", result1[0].Type)
	assert.Equal(t, "conditional", result2[0].Type)
}

// TestDisplayEventParserReturnsMutableCopy verifies parser return can be modified
func TestDisplayEventParserReturnsMutableCopy(t *testing.T) {
	parser := DisplayEventParser(func(line []byte) []DisplayEvent {
		return []DisplayEvent{{Type: "original", Text: "data"}}
	})

	result := parser([]byte("input"))
	require.Len(t, result, 1)
	originalType := result[0].Type

	result[0].Type = "modified"

	assert.Equal(t, "original", originalType)
	assert.Equal(t, "modified", result[0].Type)
}
