package tokenizer

import (
	"fmt"

	tiktoken "github.com/pkoukk/tiktoken-go"
	"github.com/vanoix/awf/internal/domain/ports"
)

// TiktokenTokenizer implements ports.Tokenizer using pkoukk/tiktoken-go library.
// Provides accurate token counting for OpenAI-compatible models.
type TiktokenTokenizer struct {
	modelName string
}

// NewTiktokenTokenizer creates a new TiktokenTokenizer for the specified model.
// Common models: "cl100k_base" (GPT-4, GPT-3.5-turbo), "p50k_base" (Codex), "r50k_base" (GPT-3).
func NewTiktokenTokenizer(modelName string) (ports.Tokenizer, error) {
	return &TiktokenTokenizer{
		modelName: modelName,
	}, nil
}

// CountTokens returns the exact number of tokens in the given text.
func (t *TiktokenTokenizer) CountTokens(text string) (int, error) {
	if text == "" {
		return 0, nil
	}

	// Get encoding for the model
	encoding, err := tiktoken.GetEncoding(t.modelName)
	if err != nil {
		return 0, fmt.Errorf("failed to get encoding for model %s: %w", t.modelName, err)
	}

	// Encode the text to get tokens
	tokens := encoding.Encode(text, nil, nil)
	return len(tokens), nil
}

// CountTurnsTokens returns the total token count across multiple conversation turns.
func (t *TiktokenTokenizer) CountTurnsTokens(turns []string) (int, error) {
	if len(turns) == 0 {
		return 0, nil
	}

	totalTokens := 0
	for _, turn := range turns {
		count, err := t.CountTokens(turn)
		if err != nil {
			return 0, err
		}
		totalTokens += count
	}

	return totalTokens, nil
}

// IsEstimate returns false because tiktoken provides exact counts.
func (t *TiktokenTokenizer) IsEstimate() bool {
	return false
}

// ModelName returns the tiktoken model identifier.
func (t *TiktokenTokenizer) ModelName() string {
	return t.modelName
}
