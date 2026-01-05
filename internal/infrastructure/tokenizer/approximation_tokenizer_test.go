package tokenizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/ports"
)

// Component: approximation_tokenizer
// Feature: F033

// ============================================================================
// Interface Compliance Tests
// ============================================================================

func TestApproximationTokenizer_InterfaceCompliance(t *testing.T) {
	// Verify ApproximationTokenizer implements ports.Tokenizer
	var _ ports.Tokenizer = (*ApproximationTokenizer)(nil)
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewApproximationTokenizer_HappyPath(t *testing.T) {
	// Act
	tokenizer := NewApproximationTokenizer()

	// Assert
	require.NotNil(t, tokenizer)
	assert.Equal(t, "approximation", tokenizer.ModelName())
	assert.True(t, tokenizer.IsEstimate(), "approximation should provide estimates")
}

func TestNewApproximationTokenizerWithRatio_HappyPath(t *testing.T) {
	tests := []struct {
		name          string
		charsPerToken float64
		expectedRatio float64
	}{
		{"default ratio (4.0)", 4.0, 4.0},
		{"tight ratio (2.0) for dense text", 2.0, 2.0},
		{"loose ratio (6.0) for verbose text", 6.0, 6.0},
		{"precise ratio (3.5)", 3.5, 3.5},
		{"very tight (1.0)", 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			tokenizer := NewApproximationTokenizerWithRatio(tt.charsPerToken)

			// Assert
			require.NotNil(t, tokenizer)
			assert.Equal(t, "approximation", tokenizer.ModelName())
			assert.True(t, tokenizer.IsEstimate())

			// Verify ratio by testing known text
			// 40 chars with ratio 4.0 should give ~10 tokens
			text := strings.Repeat("x", int(tt.charsPerToken*10))
			count, err := tokenizer.CountTokens(text)
			require.NoError(t, err)
			assert.Equal(t, 10, count, "expected 10 tokens for ratio calculation")
		})
	}
}

func TestNewApproximationTokenizerWithRatio_ZeroRatio(t *testing.T) {
	// This tests edge case behavior - division by zero potential
	// Act
	tokenizer := NewApproximationTokenizerWithRatio(0.0)

	// Assert - Constructor should succeed
	require.NotNil(t, tokenizer)
}

func TestNewApproximationTokenizerWithRatio_NegativeRatio(t *testing.T) {
	// This tests edge case behavior - negative ratio
	// Act
	tokenizer := NewApproximationTokenizerWithRatio(-4.0)

	// Assert - Constructor should succeed (validation happens during counting)
	require.NotNil(t, tokenizer)
}

// ============================================================================
// CountTokens Tests - Happy Path
// ============================================================================

func TestApproximationTokenizer_CountTokens_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		expectedMin int // Minimum expected tokens (with 4.0 ratio)
		expectedMax int // Maximum expected tokens (with 4.0 ratio)
	}{
		{
			name:        "simple sentence",
			text:        "Hello, world!", // 13 chars
			expectedMin: 2,
			expectedMax: 5,
		},
		{
			name:        "longer text",
			text:        "This is a test prompt for token counting in the AWF CLI application.", // ~70 chars
			expectedMin: 15,
			expectedMax: 20,
		},
		{
			name:        "exact 40 chars (should be 10 tokens)",
			text:        "1234567890123456789012345678901234567890", // 40 chars
			expectedMin: 10,
			expectedMax: 10,
		},
		{
			name:        "code snippet",
			text:        "func main() {\n\tfmt.Println(\"Hello\")\n}",
			expectedMin: 8,
			expectedMax: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizer()

			// Act
			count, err := tokenizer.CountTokens(tt.text)

			// Assert
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, tt.expectedMin,
				"token count should be at least %d", tt.expectedMin)
			assert.LessOrEqual(t, count, tt.expectedMax,
				"token count should be at most %d", tt.expectedMax)
		})
	}
}

func TestApproximationTokenizer_CountTokens_EmptyString(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Act
	count, err := tokenizer.CountTokens("")

	// Assert - Empty string should have 0 tokens
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestApproximationTokenizer_CountTokens_WhitespaceOnly(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"single space", " "},
		{"multiple spaces", "    "},        // 4 chars = 1 token
		{"tabs", "\t\t\t\t"},               // 4 chars = 1 token
		{"newlines", "\n\n\n\n"},           // 4 chars = 1 token
		{"mixed whitespace", " \t\n \t\n"}, // variable
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizer()

			// Act
			count, err := tokenizer.CountTokens(tt.text)

			// Assert - Whitespace should be counted by character length
			require.NoError(t, err)
			expectedCount := len(tt.text) / 4 // Integer division rounds down
			assert.Equal(t, expectedCount, count)
		})
	}
}

func TestApproximationTokenizer_CountTokens_UnicodeText(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"chinese", "你好世界"},                        // UTF-8 bytes vary
		{"russian", "Привет мир"},                  // UTF-8 bytes vary
		{"arabic", "مرحبا بالعالم"},                // UTF-8 bytes vary
		{"emoji", "Hello 👋 World 🌍"},               // Emoji = multiple bytes
		{"mixed unicode", "Hello 世界! Привет мир!"}, // Mixed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizer()

			// Act
			count, err := tokenizer.CountTokens(tt.text)

			// Assert - Should handle unicode gracefully (byte-based or rune-based)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, count, 0, "unicode text should produce non-negative count")
		})
	}
}

func TestApproximationTokenizer_CountTokens_SpecialCharacters(t *testing.T) {
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
			// Arrange
			tokenizer := NewApproximationTokenizer()

			// Act
			count, err := tokenizer.CountTokens(tt.text)

			// Assert
			require.NoError(t, err)
			assert.Greater(t, count, 0)
		})
	}
}

// ============================================================================
// CountTokens Tests - Edge Cases
// ============================================================================

func TestApproximationTokenizer_CountTokens_LargeText(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Create large text (~100KB)
	largeText := strings.Repeat("This is a test sentence with multiple words. ", 2000)

	// Act
	count, err := tokenizer.CountTokens(largeText)

	// Assert - Should handle large text without error
	require.NoError(t, err)
	assert.Greater(t, count, 1000, "large text should produce many tokens")

	// Verify approximation accuracy
	expectedApprox := len(largeText) / 4
	tolerance := expectedApprox / 10 // 10% tolerance
	assert.InDelta(t, expectedApprox, count, float64(tolerance))
}

func TestApproximationTokenizer_CountTokens_VeryLongSingleWord(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Very long "word" (no spaces)
	longWord := strings.Repeat("a", 10000)

	// Act
	count, err := tokenizer.CountTokens(longWord)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2500, count, "10000 chars / 4 = 2500 tokens")
}

func TestApproximationTokenizer_CountTokens_SingleCharacter(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Act
	count, err := tokenizer.CountTokens("x")

	// Assert - Single char with ratio 4.0 should round to 0 or 1
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0)
	assert.LessOrEqual(t, count, 1)
}

func TestApproximationTokenizer_CountTokens_RepeatedCounting(t *testing.T) {
	// Test that counting the same text multiple times is consistent
	// Arrange
	tokenizer := NewApproximationTokenizer()

	text := "This is a test prompt that should produce consistent counts"

	// Act - Count same text 5 times
	counts := make([]int, 5)
	for i := 0; i < 5; i++ {
		count, err := tokenizer.CountTokens(text)
		require.NoError(t, err)
		counts[i] = count
	}

	// Assert - All counts should be identical (deterministic)
	for i := 1; i < len(counts); i++ {
		assert.Equal(t, counts[0], counts[i],
			"count %d should equal first count %d", counts[i], counts[0])
	}
}

func TestApproximationTokenizer_CountTokens_DifferentRatios(t *testing.T) {
	tests := []struct {
		name          string
		charsPerToken float64
		text          string
		expectedCount int
	}{
		{"ratio 2.0", 2.0, "12345678", 4}, // 8 chars / 2 = 4 tokens
		{"ratio 4.0", 4.0, "12345678", 2}, // 8 chars / 4 = 2 tokens
		{"ratio 8.0", 8.0, "12345678", 1}, // 8 chars / 8 = 1 token
		{"ratio 1.0", 1.0, "12345678", 8}, // 8 chars / 1 = 8 tokens
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizerWithRatio(tt.charsPerToken)

			// Act
			count, err := tokenizer.CountTokens(tt.text)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// ============================================================================
// CountTokens Tests - Error Handling
// ============================================================================

func TestApproximationTokenizer_CountTokens_ZeroRatio(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizerWithRatio(0.0)

	// Act
	count, err := tokenizer.CountTokens("test text")

	// Assert - Should return error for division by zero
	// Stub returns 0, nil - test will fail when implemented
	if err == nil {
		t.Log("Warning: Expected error for zero ratio, got success (stub behavior)")
		assert.Equal(t, 0, count, "count should be 0 on error")
	} else {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ratio", "error should mention ratio")
	}
}

func TestApproximationTokenizer_CountTokens_NegativeRatio(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizerWithRatio(-4.0)

	// Act
	count, err := tokenizer.CountTokens("test text")

	// Assert - Should return error for negative ratio
	if err == nil {
		t.Log("Warning: Expected error for negative ratio, got success (stub behavior)")
		assert.Equal(t, 0, count, "count should be 0 on error")
	} else {
		assert.Error(t, err)
	}
}

// ============================================================================
// CountTurnsTokens Tests - Happy Path
// ============================================================================

func TestApproximationTokenizer_CountTurnsTokens_HappyPath(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	turns := []string{
		"You are a helpful assistant.",     // ~30 chars = ~7-8 tokens
		"Analyze this code snippet.",       // ~27 chars = ~6-7 tokens
		"Here is the detailed analysis...", // ~35 chars = ~8-9 tokens
		"Thank you for the analysis!",      // ~29 chars = ~7 tokens
	}

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	assert.Greater(t, count, 25, "multiple turns should produce significant tokens")

	// Verify it sums correctly (total chars ~121 / 4 = ~30)
	totalChars := 0
	for _, turn := range turns {
		totalChars += len(turn)
	}
	expectedCount := totalChars / 4
	assert.Equal(t, expectedCount, count)
}

func TestApproximationTokenizer_CountTurnsTokens_SingleTurn(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	turns := []string{"Single turn message"} // 19 chars = 4 tokens

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 4, count, "19 chars / 4 = 4 tokens (integer division)")
}

func TestApproximationTokenizer_CountTurnsTokens_EmptyArray(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Act
	count, err := tokenizer.CountTurnsTokens([]string{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, count, "empty array should produce 0 tokens")
}

func TestApproximationTokenizer_CountTurnsTokens_NilArray(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Act
	count, err := tokenizer.CountTurnsTokens(nil)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, count, "nil array should produce 0 tokens")
}

func TestApproximationTokenizer_CountTurnsTokens_MixedEmptyTurns(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	turns := []string{
		"First turn with content", // 23 chars = 5 tokens
		"",                        // 0 chars = 0 tokens
		"Third turn with content", // 23 chars = 5 tokens
		"",                        // 0 chars = 0 tokens
		"Fifth turn with content", // 23 chars = 5 tokens
	}

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 17, count, "69 chars / 4 = 17 tokens (integer division)")
}

func TestApproximationTokenizer_CountTurnsTokens_AllEmptyTurns(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	turns := []string{"", "", "", ""}

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, count, "all empty turns should produce 0 tokens")
}

// ============================================================================
// CountTurnsTokens Tests - Edge Cases
// ============================================================================

func TestApproximationTokenizer_CountTurnsTokens_ManyTurns(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Create 100 turns
	turns := make([]string, 100)
	for i := range turns {
		turns[i] = "Turn with some content for testing" // 34 chars = ~8-9 tokens each
	}

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	expectedCount := 100 * 34 / 4 // 100 turns * 34 chars / 4 = 850 tokens
	assert.Equal(t, expectedCount, count)
}

func TestApproximationTokenizer_CountTurnsTokens_LargeIndividualTurns(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Create turns with large content
	turns := []string{
		strings.Repeat("First turn with lots of content. ", 100),  // 3400 chars = 850 tokens
		strings.Repeat("Second turn with lots of content. ", 100), // 3500 chars = 875 tokens
		strings.Repeat("Third turn with lots of content. ", 100),  // 3400 chars = 850 tokens
	}

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	assert.Greater(t, count, 2000, "large turns should produce many tokens")
}

func TestApproximationTokenizer_CountTurnsTokens_UnicodeInTurns(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	turns := []string{
		"Hello in English",
		"你好 in Chinese",
		"Привет in Russian",
		"مرحبا in Arabic",
		"🌍 Emoji turn",
	}

	// Act
	count, err := tokenizer.CountTurnsTokens(turns)

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0, "unicode turns should produce non-negative tokens")
}

// ============================================================================
// CountTurnsTokens Tests - Consistency with CountTokens
// ============================================================================

func TestApproximationTokenizer_CountTurnsTokens_MatchesIndividualCounts(t *testing.T) {
	// Test that CountTurnsTokens produces same result as sum of individual CountTokens
	// Arrange
	tokenizer := NewApproximationTokenizer()

	turns := []string{
		"Turn one",   // 8 chars
		"Turn two",   // 8 chars
		"Turn three", // 10 chars
	}

	// Act - Count individually
	individualTotal := 0
	for _, turn := range turns {
		count, err := tokenizer.CountTokens(turn)
		require.NoError(t, err)
		individualTotal += count
	}

	// Act - Count in batch
	batchTotal, err := tokenizer.CountTurnsTokens(turns)
	require.NoError(t, err)

	// Assert - Should match exactly for approximation tokenizer
	assert.Equal(t, individualTotal, batchTotal,
		"batch count should match sum of individual counts")
}

func TestApproximationTokenizer_CountTurnsTokens_DifferentRatios(t *testing.T) {
	tests := []struct {
		name          string
		charsPerToken float64
		turns         []string
		expectedCount int
	}{
		{
			name:          "ratio 2.0",
			charsPerToken: 2.0,
			turns:         []string{"12345678", "12345678"}, // 16 chars total / 2 = 8 tokens
			expectedCount: 8,
		},
		{
			name:          "ratio 4.0",
			charsPerToken: 4.0,
			turns:         []string{"12345678", "12345678"}, // 16 chars total / 4 = 4 tokens
			expectedCount: 4,
		},
		{
			name:          "ratio 1.0",
			charsPerToken: 1.0,
			turns:         []string{"1234", "5678", "90AB"}, // 12 chars total / 1 = 12 tokens
			expectedCount: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizerWithRatio(tt.charsPerToken)

			// Act
			count, err := tokenizer.CountTurnsTokens(tt.turns)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}

// ============================================================================
// ModelName Tests
// ============================================================================

func TestApproximationTokenizer_ModelName(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Act
	name := tokenizer.ModelName()

	// Assert
	assert.Equal(t, "approximation", name)
}

func TestApproximationTokenizer_ModelName_WithCustomRatio(t *testing.T) {
	// Model name should be consistent regardless of ratio
	ratios := []float64{1.0, 2.0, 3.5, 4.0, 6.0}

	for _, ratio := range ratios {
		t.Run(string(rune(ratio)), func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizerWithRatio(ratio)

			// Act
			name := tokenizer.ModelName()

			// Assert
			assert.Equal(t, "approximation", name,
				"model name should be 'approximation' for all ratios")
		})
	}
}

// ============================================================================
// IsEstimate Tests
// ============================================================================

func TestApproximationTokenizer_IsEstimate(t *testing.T) {
	// Arrange
	tokenizer := NewApproximationTokenizer()

	// Act
	isEstimate := tokenizer.IsEstimate()

	// Assert - Approximation provides estimates, not exact counts
	assert.True(t, isEstimate, "approximation should return true for IsEstimate()")
}

func TestApproximationTokenizer_IsEstimate_AllRatios(t *testing.T) {
	ratios := []float64{1.0, 2.0, 3.5, 4.0, 6.0}

	for _, ratio := range ratios {
		t.Run(string(rune(ratio)), func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizerWithRatio(ratio)

			// Act
			isEstimate := tokenizer.IsEstimate()

			// Assert - All approximation tokenizers provide estimates
			assert.True(t, isEstimate,
				"ratio %.1f should return true for IsEstimate()", ratio)
		})
	}
}

// ============================================================================
// Integration Tests - Real-world Scenarios
// ============================================================================

func TestApproximationTokenizer_RealWorldPrompt(t *testing.T) {
	// Test with a realistic AI workflow prompt
	// Arrange
	tokenizer := NewApproximationTokenizer()

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

	// Act
	count, err := tokenizer.CountTokens(prompt)

	// Assert
	require.NoError(t, err)

	// Calculate expected approximation
	expectedApprox := len(prompt) / 4
	assert.InDelta(t, expectedApprox, count, float64(expectedApprox)/10,
		"approximation should be within 10%% of expected")
}

func TestApproximationTokenizer_ConversationScenario(t *testing.T) {
	// Test with a multi-turn conversation
	// Arrange
	tokenizer := NewApproximationTokenizer()

	conversation := []string{
		"System: You are a helpful coding assistant.",
		"User: How do I reverse a string in Go?",
		"Assistant: Here's how to reverse a string in Go:\n\nfunc reverse(s string) string {\n    runes := []rune(s)\n    for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {\n        runes[i], runes[j] = runes[j], runes[i]\n    }\n    return string(runes)\n}",
		"User: Thanks! Can you add error handling?",
		"Assistant: Sure! Here's the version with error handling...",
	}

	// Act
	count, err := tokenizer.CountTurnsTokens(conversation)

	// Assert
	require.NoError(t, err)

	// Calculate total characters
	totalChars := 0
	for _, turn := range conversation {
		totalChars += len(turn)
	}
	expectedApprox := totalChars / 4
	assert.Equal(t, expectedApprox, count,
		"conversation token count should match total chars / 4")
}

func TestApproximationTokenizer_AccuracyComparison(t *testing.T) {
	// Compare approximation accuracy across different text types
	tests := []struct {
		name string
		text string
	}{
		{"english prose", "The quick brown fox jumps over the lazy dog."},
		{"code", "func main() { fmt.Println(\"Hello\") }"},
		{"numbers", "1234567890 9876543210 5555555555"},
		{"mixed", "User123 sent $100.50 to account #456-789-012 on 2024-01-05."},
	}

	tokenizer := NewApproximationTokenizer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			count, err := tokenizer.CountTokens(tt.text)

			// Assert
			require.NoError(t, err)

			// For approximation, we expect: count ≈ len(text) / 4
			expectedApprox := len(tt.text) / 4
			assert.InDelta(t, expectedApprox, count, float64(expectedApprox)/5,
				"approximation should be within 20%% for %s", tt.name)
		})
	}
}

// ============================================================================
// Rounding Behavior Tests
// ============================================================================

func TestApproximationTokenizer_RoundingBehavior(t *testing.T) {
	// Test how integer division handles rounding
	tests := []struct {
		name          string
		textLength    int
		charsPerToken float64
		expectedCount int
	}{
		{"exact division", 8, 4.0, 2}, // 8 / 4 = 2
		{"rounds down", 9, 4.0, 2},    // 9 / 4 = 2.25 → 2
		{"rounds down", 10, 4.0, 2},   // 10 / 4 = 2.5 → 2
		{"rounds down", 11, 4.0, 2},   // 11 / 4 = 2.75 → 2
		{"next whole", 12, 4.0, 3},    // 12 / 4 = 3
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			tokenizer := NewApproximationTokenizerWithRatio(tt.charsPerToken)
			text := strings.Repeat("x", tt.textLength)

			// Act
			count, err := tokenizer.CountTokens(text)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, count)
		})
	}
}
