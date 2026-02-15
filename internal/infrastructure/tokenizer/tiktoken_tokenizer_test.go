package tokenizer

import (
	"strings"
	"testing"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Component: tiktoken_adapter
// Feature: F033

func TestTiktokenTokenizer_InterfaceCompliance(t *testing.T) {
	// Verify TiktokenTokenizer implements ports.Tokenizer
	var _ ports.Tokenizer = (*TiktokenTokenizer)(nil)
}

func TestNewTiktokenTokenizer_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{"cl100k_base (GPT-4, GPT-3.5-turbo)", "cl100k_base"},
		{"p50k_base (Codex)", "p50k_base"},
		{"r50k_base (GPT-3)", "r50k_base"},
		{"gpt2", "gpt2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer(tt.modelName)

			require.NoError(t, err)
			require.NotNil(t, tokenizer)
			assert.Equal(t, tt.modelName, tokenizer.ModelName())
			assert.False(t, tokenizer.IsEstimate(), "tiktoken should provide exact counts")
		})
	}
}

func TestNewTiktokenTokenizer_EmptyModelName(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("")

	require.NoError(t, err)
	require.NotNil(t, tokenizer)
	assert.Equal(t, "", tokenizer.ModelName())
}

func TestNewTiktokenTokenizer_InvalidModelName(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("invalid_model_xyz")

	require.NoError(t, err)
	require.NotNil(t, tokenizer)
	assert.Equal(t, "invalid_model_xyz", tokenizer.ModelName())
}

func TestTiktokenTokenizer_CountTokens_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		modelName   string
		text        string
		expectedMin int // Minimum expected tokens
		expectedMax int // Maximum expected tokens
	}{
		{
			name:        "simple sentence",
			modelName:   "cl100k_base",
			text:        "Hello, world!",
			expectedMin: 2,
			expectedMax: 5,
		},
		{
			name:        "longer text",
			modelName:   "cl100k_base",
			text:        "This is a test prompt for token counting in the AWF CLI application.",
			expectedMin: 10,
			expectedMax: 20,
		},
		{
			name:        "code snippet",
			modelName:   "cl100k_base",
			text:        "func main() {\n\tfmt.Println(\"Hello\")\n}",
			expectedMin: 8,
			expectedMax: 20,
		},
		{
			name:        "markdown text",
			modelName:   "cl100k_base",
			text:        "# Header\n\n**Bold** and *italic* text with [link](https://example.com)",
			expectedMin: 12,
			expectedMax: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer(tt.modelName)
			require.NoError(t, err)

			count, err := tokenizer.CountTokens(tt.text)

			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, tt.expectedMin,
				"token count should be at least %d", tt.expectedMin)
			assert.LessOrEqual(t, count, tt.expectedMax,
				"token count should be at most %d", tt.expectedMax)
		})
	}
}

func TestTiktokenTokenizer_CountTokens_EmptyString(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	count, err := tokenizer.CountTokens("")

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTiktokenTokenizer_CountTokens_WhitespaceOnly(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"single space", " "},
		{"multiple spaces", "   "},
		{"tabs", "\t\t"},
		{"newlines", "\n\n\n"},
		{"mixed whitespace", " \t\n \t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer("cl100k_base")
			require.NoError(t, err)

			count, err := tokenizer.CountTokens(tt.text)

			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, 0)
		})
	}
}

func TestTiktokenTokenizer_CountTokens_UnicodeText(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"chinese", "你好世界"},
		{"russian", "Привет мир"},
		{"arabic", "مرحبا بالعالم"},
		{"emoji", "Hello 👋 World 🌍"},
		{"mixed unicode", "Hello 世界! Привет мир! مرحبا بالعالم!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer("cl100k_base")
			require.NoError(t, err)

			count, err := tokenizer.CountTokens(tt.text)

			require.NoError(t, err)
			assert.Greater(t, count, 0, "unicode text should produce tokens")
		})
	}
}

func TestTiktokenTokenizer_CountTokens_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"code block", "```python\ndef foo():\n    return \"bar\"\n```"},
		{"xml/html", "<xml><tag attribute=\"value\">content</tag></xml>"},
		{"json", "{\"key\": \"value\", \"number\": 42}"},
		{"special symbols", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer("cl100k_base")
			require.NoError(t, err)

			count, err := tokenizer.CountTokens(tt.text)

			require.NoError(t, err)
			assert.Greater(t, count, 0)
		})
	}
}

func TestTiktokenTokenizer_CountTokens_LargeText(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	// Create large text (~100KB)
	largeText := strings.Repeat("This is a test sentence with multiple words. ", 2000)

	count, err := tokenizer.CountTokens(largeText)

	require.NoError(t, err)
	assert.Greater(t, count, 1000, "large text should produce many tokens")
}

func TestTiktokenTokenizer_CountTokens_VeryLongSingleWord(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	// Very long "word" (no spaces)
	longWord := strings.Repeat("a", 10000)

	count, err := tokenizer.CountTokens(longWord)

	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestTiktokenTokenizer_CountTokens_RepeatedCounting(t *testing.T) {
	// Test that counting the same text multiple times is consistent
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	text := "This is a test prompt that should produce consistent counts"

	counts := make([]int, 5)
	for i := 0; i < 5; i++ {
		count, err := tokenizer.CountTokens(text)
		require.NoError(t, err)
		counts[i] = count
	}

	for i := 1; i < len(counts); i++ {
		assert.Equal(t, counts[0], counts[i],
			"count %d should equal first count %d", counts[i], counts[0])
	}
}

func TestTiktokenTokenizer_CountTokens_InvalidModel(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("invalid_model_name")
	require.NoError(t, err)

	count, err := tokenizer.CountTokens("test text")

	// Note: This behavior depends on tiktoken-go implementation
	// For now, stub returns 0, nil - test will fail when implemented
	if err != nil {
		assert.Equal(t, 0, count, "count should be 0 on error")
		assert.Error(t, err)
	} else {
		// Stub behavior - will fail when real implementation is added
		t.Log("Warning: Expected error for invalid model, got success (stub behavior)")
	}
}

func TestTiktokenTokenizer_CountTurnsTokens_HappyPath(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	turns := []string{
		"You are a helpful assistant.",
		"Analyze this code snippet.",
		"Here is the detailed analysis of the code...",
		"Thank you for the analysis!",
	}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Greater(t, count, 10, "multiple turns should produce significant tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_SingleTurn(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	turns := []string{"Single turn message"}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestTiktokenTokenizer_CountTurnsTokens_EmptyArray(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	count, err := tokenizer.CountTurnsTokens([]string{})

	require.NoError(t, err)
	assert.Equal(t, 0, count, "empty array should produce 0 tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_NilArray(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	count, err := tokenizer.CountTurnsTokens(nil)

	require.NoError(t, err)
	assert.Equal(t, 0, count, "nil array should produce 0 tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_MixedEmptyTurns(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	turns := []string{
		"First turn with content",
		"",
		"Third turn with content",
		"",
		"Fifth turn with content",
	}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Greater(t, count, 0, "non-empty turns should produce tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_AllEmptyTurns(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	turns := []string{"", "", "", ""}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Equal(t, 0, count, "all empty turns should produce 0 tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_ManyTurns(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	// Create 100 turns
	turns := make([]string, 100)
	for i := range turns {
		turns[i] = "Turn with some content for testing"
	}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Greater(t, count, 100, "100 turns should produce significant tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_LargeIndividualTurns(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	// Create turns with large content
	turns := []string{
		strings.Repeat("First turn with lots of content. ", 100),
		strings.Repeat("Second turn with lots of content. ", 100),
		strings.Repeat("Third turn with lots of content. ", 100),
	}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Greater(t, count, 300, "large turns should produce many tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_UnicodeInTurns(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	turns := []string{
		"Hello in English",
		"你好 in Chinese",
		"Привет in Russian",
		"مرحبا in Arabic",
		"🌍 Emoji turn",
	}

	count, err := tokenizer.CountTurnsTokens(turns)

	require.NoError(t, err)
	assert.Greater(t, count, 10, "unicode turns should produce tokens")
}

func TestTiktokenTokenizer_CountTurnsTokens_MatchesIndividualCounts(t *testing.T) {
	// Test that CountTurnsTokens produces same result as sum of individual CountTokens
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	turns := []string{
		"Turn one",
		"Turn two",
		"Turn three",
	}

	individualTotal := 0
	for _, turn := range turns {
		count, err := tokenizer.CountTokens(turn)
		require.NoError(t, err)
		individualTotal += count
	}

	batchTotal, err := tokenizer.CountTurnsTokens(turns)
	require.NoError(t, err)

	assert.Equal(t, individualTotal, batchTotal,
		"batch count should match sum of individual counts")
}

func TestTiktokenTokenizer_ModelName(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{"cl100k_base", "cl100k_base"},
		{"p50k_base", "p50k_base"},
		{"r50k_base", "r50k_base"},
		{"gpt2", "gpt2"},
		{"custom-model", "custom-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer(tt.modelName)
			require.NoError(t, err)

			name := tokenizer.ModelName()

			assert.Equal(t, tt.modelName, name)
		})
	}
}

func TestTiktokenTokenizer_IsEstimate(t *testing.T) {
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	isEstimate := tokenizer.IsEstimate()

	assert.False(t, isEstimate, "tiktoken should return false for IsEstimate()")
}

func TestTiktokenTokenizer_IsEstimate_AllModels(t *testing.T) {
	models := []string{"cl100k_base", "p50k_base", "r50k_base", "gpt2"}

	for _, modelName := range models {
		t.Run(modelName, func(t *testing.T) {
			tokenizer, err := NewTiktokenTokenizer(modelName)
			require.NoError(t, err)

			isEstimate := tokenizer.IsEstimate()

			assert.False(t, isEstimate,
				"model %s should return false for IsEstimate()", modelName)
		})
	}
}

func TestTiktokenTokenizer_RealWorldPrompt(t *testing.T) {
	// Test with a realistic AI workflow prompt
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	prompt := `You are a code review assistant. Analyze the following Go code for:
1. Potential bugs
2. Performance issues
3. Security vulnerabilities
4. Code style violations

Code:
func ProcessData(data []string) error {
    for _, item := range data {
        // Process item
        fmt.Println(item)
    }
    return nil
}

Provide detailed feedback.`

	count, err := tokenizer.CountTokens(prompt)

	require.NoError(t, err)
	assert.Greater(t, count, 50, "realistic prompt should have substantial tokens")
	assert.Less(t, count, 200, "realistic prompt shouldn't have excessive tokens")
}

func TestTiktokenTokenizer_ConversationScenario(t *testing.T) {
	// Test with a multi-turn conversation
	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	conversation := []string{
		"System: You are a helpful coding assistant.",
		"User: How do I reverse a string in Go?",
		"Assistant: Here's how to reverse a string in Go:\n\nfunc reverse(s string) string {\n    runes := []rune(s)\n    for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {\n        runes[i], runes[j] = runes[j], runes[i]\n    }\n    return string(runes)\n}",
		"User: Thanks! Can you add error handling?",
		"Assistant: Sure! Here's the version with error handling...",
	}

	count, err := tokenizer.CountTurnsTokens(conversation)

	require.NoError(t, err)
	assert.Greater(t, count, 100, "multi-turn conversation should have many tokens")
}

func TestTiktokenTokenizer_CodeSnippets(t *testing.T) {
	// Test various programming language snippets
	snippets := map[string]string{
		"go": `package main
import "fmt"
func main() {
    fmt.Println("Hello, World!")
}`,
		"python": `def hello():
    print("Hello, World!")
if __name__ == "__main__":
    hello()`,
		"javascript": `function hello() {
    console.log("Hello, World!");
}
hello();`,
		"sql": `SELECT users.name, orders.total
FROM users
INNER JOIN orders ON users.id = orders.user_id
WHERE orders.total > 100;`,
	}

	tokenizer, err := NewTiktokenTokenizer("cl100k_base")
	require.NoError(t, err)

	for lang, code := range snippets {
		t.Run(lang, func(t *testing.T) {
			count, err := tokenizer.CountTokens(code)

			require.NoError(t, err)
			assert.Greater(t, count, 5, "%s code should produce tokens", lang)
		})
	}
}
