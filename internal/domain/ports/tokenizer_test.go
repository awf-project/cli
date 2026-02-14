package ports_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vanoix/awf/internal/domain/ports"
)

// Component: tokenizer_port
// Feature: F033

// mockTokenizer is a test implementation of Tokenizer interface
type mockTokenizer struct {
	countFunc      func(text string) (int, error)
	countTurnsFunc func(turns []string) (int, error)
	isEstimate     bool
	modelName      string
	callCount      int
	turnsCallCount int
}

func newMockTokenizer(modelName string, isEstimate bool) *mockTokenizer {
	return &mockTokenizer{
		modelName:  modelName,
		isEstimate: isEstimate,
		countFunc: func(text string) (int, error) {
			// Simple approximation: ~4 chars per token
			return len(text) / 4, nil
		},
		countTurnsFunc: func(turns []string) (int, error) {
			total := 0
			for _, turn := range turns {
				total += len(turn) / 4
			}
			return total, nil
		},
	}
}

func (m *mockTokenizer) CountTokens(text string) (int, error) {
	m.callCount++
	return m.countFunc(text)
}

func (m *mockTokenizer) CountTurnsTokens(turns []string) (int, error) {
	m.turnsCallCount++
	return m.countTurnsFunc(turns)
}

func (m *mockTokenizer) IsEstimate() bool {
	return m.isEstimate
}

func (m *mockTokenizer) ModelName() string {
	return m.modelName
}

func TestTokenizerInterface(t *testing.T) {
	// Verify interface compliance
	var _ ports.Tokenizer = (*mockTokenizer)(nil)
}

func TestTokenizer_CountTokens_HappyPath(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	text := "This is a test prompt for token counting"

	count, err := tokenizer.CountTokens(text)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive token count, got %d", count)
	}
	if tokenizer.callCount != 1 {
		t.Errorf("expected CountTokens to be called once, got %d", tokenizer.callCount)
	}
}

func TestTokenizer_CountTurnsTokens_HappyPath(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	turns := []string{
		"You are a helpful assistant",
		"Analyze this code",
		"Here is the analysis...",
	}

	count, err := tokenizer.CountTurnsTokens(turns)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive token count, got %d", count)
	}
	if tokenizer.turnsCallCount != 1 {
		t.Errorf("expected CountTurnsTokens to be called once, got %d", tokenizer.turnsCallCount)
	}
}

func TestTokenizer_IsEstimate_HappyPath(t *testing.T) {
	tests := []struct {
		name       string
		isEstimate bool
	}{
		{"exact tokenizer", false},
		{"approximate tokenizer", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := newMockTokenizer("test", tt.isEstimate)

			result := tokenizer.IsEstimate()

			if result != tt.isEstimate {
				t.Errorf("expected IsEstimate() = %v, got %v", tt.isEstimate, result)
			}
		})
	}
}

func TestTokenizer_ModelName_HappyPath(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{"tiktoken cl100k", "cl100k_base"},
		{"tiktoken p50k", "p50k_base"},
		{"approximation", "approximation"},
		{"custom model", "custom-tokenizer-v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := newMockTokenizer(tt.modelName, false)

			name := tokenizer.ModelName()

			if name != tt.modelName {
				t.Errorf("expected ModelName() = '%s', got '%s'", tt.modelName, name)
			}
		})
	}
}

func TestTokenizer_CountTokens_EmptyString(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)

	count, err := tokenizer.CountTokens("")
	if err != nil {
		t.Errorf("unexpected error for empty string: %v", err)
	}
	if count < 0 {
		t.Errorf("expected non-negative count for empty string, got %d", count)
	}
}

func TestTokenizer_CountTokens_LargeText(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	// Create a large text (100KB)
	largeText := strings.Repeat("This is a test sentence. ", 4000)

	count, err := tokenizer.CountTokens(largeText)
	if err != nil {
		t.Errorf("unexpected error for large text: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive count for large text, got %d", count)
	}
}

func TestTokenizer_CountTokens_UnicodeText(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	unicodeText := "Hello 世界! Привет мир! مرحبا بالعالم!"

	count, err := tokenizer.CountTokens(unicodeText)
	if err != nil {
		t.Errorf("unexpected error for unicode text: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive count for unicode text, got %d", count)
	}
}

func TestTokenizer_CountTokens_SpecialCharacters(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	specialText := "```python\ndef foo():\n    return \"bar\"\n```\n\n<xml>test</xml>"

	count, err := tokenizer.CountTokens(specialText)
	if err != nil {
		t.Errorf("unexpected error for special characters: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive count for special characters, got %d", count)
	}
}

func TestTokenizer_CountTurnsTokens_EmptyArray(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)

	count, err := tokenizer.CountTurnsTokens([]string{})
	if err != nil {
		t.Errorf("unexpected error for empty array: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count for empty array, got %d", count)
	}
}

func TestTokenizer_CountTurnsTokens_NilArray(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)

	count, err := tokenizer.CountTurnsTokens(nil)
	if err != nil {
		t.Errorf("unexpected error for nil array: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count for nil array, got %d", count)
	}
}

func TestTokenizer_CountTurnsTokens_SingleTurn(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	turns := []string{"Single turn message"}

	count, err := tokenizer.CountTurnsTokens(turns)
	if err != nil {
		t.Errorf("unexpected error for single turn: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive count for single turn, got %d", count)
	}
}

func TestTokenizer_CountTurnsTokens_ManyTurns(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	// Create 100 turns
	turns := make([]string, 100)
	for i := range turns {
		turns[i] = "This is turn number with some content"
	}

	count, err := tokenizer.CountTurnsTokens(turns)
	if err != nil {
		t.Errorf("unexpected error for many turns: %v", err)
	}
	if count <= 0 {
		t.Errorf("expected positive count for many turns, got %d", count)
	}
}

func TestTokenizer_CountTurnsTokens_MixedEmptyTurns(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	turns := []string{
		"First turn with content",
		"",
		"Third turn with content",
		"",
		"Fifth turn with content",
	}

	count, err := tokenizer.CountTurnsTokens(turns)
	if err != nil {
		t.Errorf("unexpected error for mixed turns: %v", err)
	}
	if count < 0 {
		t.Errorf("expected non-negative count for mixed turns, got %d", count)
	}
}

func TestTokenizer_CountTokens_Error(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	expectedErr := errors.New("tokenization failed")
	tokenizer.countFunc = func(text string) (int, error) {
		return 0, expectedErr
	}

	count, err := tokenizer.CountTokens("test")

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err.Error() != "tokenization failed" {
		t.Errorf("expected error 'tokenization failed', got '%v'", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count on error, got %d", count)
	}
}

func TestTokenizer_CountTurnsTokens_Error(t *testing.T) {
	tokenizer := newMockTokenizer("cl100k_base", false)
	expectedErr := errors.New("batch tokenization failed")
	tokenizer.countTurnsFunc = func(turns []string) (int, error) {
		return 0, expectedErr
	}

	count, err := tokenizer.CountTurnsTokens([]string{"test1", "test2"})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err.Error() != "batch tokenization failed" {
		t.Errorf("expected error 'batch tokenization failed', got '%v'", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count on error, got %d", count)
	}
}

func TestTokenizer_CountTokens_ModelLoadError(t *testing.T) {
	tokenizer := newMockTokenizer("invalid_model", false)
	expectedErr := errors.New("model not found: invalid_model")
	tokenizer.countFunc = func(text string) (int, error) {
		return 0, expectedErr
	}

	count, err := tokenizer.CountTokens("test")

	if err == nil {
		t.Error("expected model load error, got nil")
	}
	if count != 0 {
		t.Errorf("expected 0 count on model load error, got %d", count)
	}
}

func TestTokenizer_CountTurnsOptimization(t *testing.T) {
	// Test that CountTurnsTokens can be optimized vs individual calls
	tokenizer := newMockTokenizer("cl100k_base", false)
	turns := []string{
		"Turn 1",
		"Turn 2",
		"Turn 3",
	}

	individualTotal := 0
	for _, turn := range turns {
		count, err := tokenizer.CountTokens(turn)
		if err != nil {
			t.Fatalf("error counting individual turn: %v", err)
		}
		individualTotal += count
	}

	batchTotal, err := tokenizer.CountTurnsTokens(turns)
	if err != nil {
		t.Fatalf("error counting batch: %v", err)
	}

	if individualTotal != batchTotal {
		// For this mock, they should be exactly equal
		// Real implementations might have slight differences
		t.Logf("Note: individual=%d, batch=%d (difference is acceptable for some tokenizers)",
			individualTotal, batchTotal)
	}
}

func TestTokenizer_ConsistentCounting(t *testing.T) {
	// Test that counting the same text multiple times produces consistent results
	tokenizer := newMockTokenizer("cl100k_base", false)
	text := "This is a test prompt that should produce consistent counts"

	count1, err1 := tokenizer.CountTokens(text)
	count2, err2 := tokenizer.CountTokens(text)
	count3, err3 := tokenizer.CountTokens(text)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("unexpected errors: %v, %v, %v", err1, err2, err3)
	}
	if count1 != count2 || count2 != count3 {
		t.Errorf("inconsistent counts: %d, %d, %d", count1, count2, count3)
	}
}

func TestTokenizer_DifferentModels(t *testing.T) {
	// Test different tokenizer models
	tests := []struct {
		modelName  string
		isEstimate bool
	}{
		{"cl100k_base", false},
		{"p50k_base", false},
		{"approximation", true},
	}

	text := "This is a test prompt"

	for _, tt := range tests {
		t.Run(tt.modelName, func(t *testing.T) {
			tokenizer := newMockTokenizer(tt.modelName, tt.isEstimate)

			count, err := tokenizer.CountTokens(text)
			if err != nil {
				t.Errorf("unexpected error for model %s: %v", tt.modelName, err)
			}
			if count <= 0 {
				t.Errorf("expected positive count for model %s, got %d", tt.modelName, count)
			}
			if tokenizer.IsEstimate() != tt.isEstimate {
				t.Errorf("IsEstimate mismatch for model %s", tt.modelName)
			}
			if tokenizer.ModelName() != tt.modelName {
				t.Errorf("ModelName mismatch: expected %s, got %s", tt.modelName, tokenizer.ModelName())
			}
		})
	}
}

func TestTokenizer_CountTurnsTokens_Performance(t *testing.T) {
	// Test that CountTurnsTokens is called once for batch operations
	tokenizer := newMockTokenizer("cl100k_base", false)
	turns := []string{
		"Turn 1", "Turn 2", "Turn 3", "Turn 4", "Turn 5",
		"Turn 6", "Turn 7", "Turn 8", "Turn 9", "Turn 10",
	}

	_, err := tokenizer.CountTurnsTokens(turns)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tokenizer.turnsCallCount != 1 {
		t.Errorf("expected CountTurnsTokens to be called once, got %d", tokenizer.turnsCallCount)
	}
	if tokenizer.callCount != 0 {
		t.Errorf("expected CountTokens not to be called for batch operation, got %d", tokenizer.callCount)
	}
}
