package ports

// Tokenizer defines the contract for counting tokens in text.
// Implementations provide model-specific or approximation-based token counting
// for context window management.
type Tokenizer interface {
	// CountTokens returns the number of tokens in the given text.
	// The count may be exact (tiktoken) or approximate (character-based)
	// depending on the implementation.
	CountTokens(text string) (int, error)

	// CountTurnsTokens returns the total token count across multiple conversation turns.
	// This allows optimizations for batch counting in some implementations.
	CountTurnsTokens(turns []string) (int, error)

	// IsEstimate returns true if this tokenizer produces approximate counts.
	// Used to set TokensEstimated flag in conversation results.
	IsEstimate() bool

	// ModelName returns the tokenizer model identifier (e.g., "cl100k_base", "approximation").
	// Used for debugging and logging.
	ModelName() string
}
