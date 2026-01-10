package tokenizer

import (
	"fmt"

	"github.com/vanoix/awf/internal/domain/ports"
)

// ApproximationTokenizer implements ports.Tokenizer using character-based estimation.
// Provides fast, approximate token counting as a fallback when exact tokenization
// is not available or not needed. Uses a simple heuristic: ~4 characters per token.
type ApproximationTokenizer struct {
	// charsPerToken defines the character-to-token ratio for estimation.
	// Default: 4.0 (typical for English text)
	charsPerToken float64
}

// NewApproximationTokenizer creates a new ApproximationTokenizer with default settings.
// Uses 4.0 characters per token as the default ratio.
func NewApproximationTokenizer() ports.Tokenizer {
	return &ApproximationTokenizer{
		charsPerToken: 4.0,
	}
}

// NewApproximationTokenizerWithRatio creates a new ApproximationTokenizer with a custom
// characters-per-token ratio. Useful for tuning accuracy for specific languages or domains.
func NewApproximationTokenizerWithRatio(charsPerToken float64) ports.Tokenizer {
	return &ApproximationTokenizer{
		charsPerToken: charsPerToken,
	}
}

// CountTokens returns an approximate token count based on character length.
// Formula: token_count ≈ len(text) / charsPerToken
func (a *ApproximationTokenizer) CountTokens(text string) (int, error) {
	// Validate ratio
	if a.charsPerToken <= 0 {
		return 0, fmt.Errorf("invalid ratio: charsPerToken must be greater than 0")
	}

	// Empty text = 0 tokens
	if text == "" {
		return 0, nil
	}

	// Integer division rounds down (e.g., 9 chars / 4.0 = 2 tokens, not 3)
	return int(float64(len(text)) / a.charsPerToken), nil
}

// CountTurnsTokens returns the total approximate token count across multiple conversation turns.
// Sums the estimated token counts for each turn.
func (a *ApproximationTokenizer) CountTurnsTokens(turns []string) (int, error) {
	// Validate ratio
	if a.charsPerToken <= 0 {
		return 0, fmt.Errorf("invalid ratio: charsPerToken must be greater than 0")
	}

	// Empty array = 0 tokens
	if len(turns) == 0 {
		return 0, nil
	}

	// Sum all turn lengths
	totalChars := 0
	for _, turn := range turns {
		totalChars += len(turn)
	}

	// Integer division rounds down
	return int(float64(totalChars) / a.charsPerToken), nil
}

// IsEstimate returns true because this tokenizer produces approximate counts.
func (a *ApproximationTokenizer) IsEstimate() bool {
	return true
}

// ModelName returns the identifier for this approximation method.
func (a *ApproximationTokenizer) ModelName() string {
	return "approximation"
}
