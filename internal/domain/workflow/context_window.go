package workflow

import "errors"

// ContextWindowManager manages context window limits and truncation strategies.
type ContextWindowManager interface {
	// ApplyStrategy applies the configured strategy to keep conversation within token limits.
	// Returns truncated turns and whether truncation occurred.
	ApplyStrategy(turns []Turn, maxTokens int, strategy ContextWindowStrategy) ([]Turn, bool, error)

	// PreserveSystemPrompt ensures the system prompt (first turn with role=system) is always kept.
	PreserveSystemPrompt(turns []Turn) (*Turn, []Turn)

	// CalculateTotalTokens returns the sum of tokens across all turns.
	CalculateTotalTokens(turns []Turn) int

	// EstimateTokens estimates token count for content when tokenizer is unavailable.
	EstimateTokens(content string) int
}

// contextWindowManager is the default implementation of ContextWindowManager.
type contextWindowManager struct{}

// NewContextWindowManager creates a new context window manager.
func NewContextWindowManager() ContextWindowManager {
	return &contextWindowManager{}
}

// ApplyStrategy applies the configured truncation strategy.
func (m *contextWindowManager) ApplyStrategy(turns []Turn, maxTokens int, strategy ContextWindowStrategy) ([]Turn, bool, error) {
	// Validate inputs
	if maxTokens <= 0 {
		return nil, false, errors.New("max_tokens must be positive")
	}

	// Validate strategy
	switch strategy {
	case StrategySlidingWindow:
		s := NewSlidingWindowStrategy()
		return s.Apply(turns, maxTokens)
	case StrategySummarize:
		return nil, false, errors.New("summarize strategy not implemented yet")
	case StrategyTruncateMiddle:
		return nil, false, errors.New("truncate_middle strategy not implemented yet")
	case StrategyNone:
		// No strategy, return as-is
		return turns, false, nil
	default:
		return nil, false, errors.New("invalid strategy")
	}
}

// PreserveSystemPrompt extracts system prompt from turns.
// Returns the first system turn found and all other turns.
func (m *contextWindowManager) PreserveSystemPrompt(turns []Turn) (*Turn, []Turn) {
	if len(turns) == 0 {
		return nil, []Turn{}
	}

	var systemTurn *Turn
	otherTurns := make([]Turn, 0, len(turns))

	for i := range turns {
		if turns[i].Role == TurnRoleSystem && systemTurn == nil {
			// Found first system prompt
			systemTurn = &turns[i]
		} else {
			otherTurns = append(otherTurns, turns[i])
		}
	}

	return systemTurn, otherTurns
}

// CalculateTotalTokens sums tokens across all turns.
func (m *contextWindowManager) CalculateTotalTokens(turns []Turn) int {
	total := 0
	for _, turn := range turns {
		total += turn.Tokens
	}
	return total
}

// EstimateTokens provides character-based token estimation.
// Uses a rough approximation of 1 token ≈ 4 characters.
func (m *contextWindowManager) EstimateTokens(content string) int {
	return len(content) / 4
}

// SlidingWindowStrategy implements FIFO truncation while preserving system prompt.
type SlidingWindowStrategy struct{}

// NewSlidingWindowStrategy creates a sliding window truncation strategy.
func NewSlidingWindowStrategy() *SlidingWindowStrategy {
	return &SlidingWindowStrategy{}
}

// Apply removes oldest turns until within token limit.
// System prompt is always preserved.
func (s *SlidingWindowStrategy) Apply(turns []Turn, maxTokens int) ([]Turn, bool, error) {
	// Validate maxTokens
	if maxTokens <= 0 {
		return nil, false, errors.New("max_tokens must be positive")
	}

	// Handle empty turns
	if len(turns) == 0 {
		return []Turn{}, false, nil
	}

	// Separate system prompt from other turns
	var systemTurn *Turn
	otherTurns := make([]Turn, 0, len(turns))

	for i := range turns {
		if turns[i].Role == TurnRoleSystem && systemTurn == nil {
			systemTurn = &turns[i]
		} else {
			otherTurns = append(otherTurns, turns[i])
		}
	}

	// Calculate initial total
	totalTokens := 0
	if systemTurn != nil {
		totalTokens += systemTurn.Tokens
	}
	for _, turn := range otherTurns {
		totalTokens += turn.Tokens
	}

	// If already within limit, no truncation needed
	if totalTokens <= maxTokens {
		return turns, false, nil
	}

	// Build result: system prompt + most recent turns that fit
	result := make([]Turn, 0, len(turns))
	currentTokens := 0

	if systemTurn != nil {
		result = append(result, *systemTurn)
		currentTokens = systemTurn.Tokens
	}

	// Find starting index where remaining turns fit in budget
	startIdx := 0
	for i := 0; i < len(otherTurns); i++ {
		totalTokens := currentTokens
		for j := i; j < len(otherTurns); j++ {
			totalTokens += otherTurns[j].Tokens
		}
		if totalTokens <= maxTokens {
			startIdx = i
			break
		}
	}

	// Append turns from startIdx onward (already in chronological order)
	result = append(result, otherTurns[startIdx:]...)

	wasTruncated := len(result) < len(turns)
	return result, wasTruncated, nil
}

// SummarizeStrategy compresses old turns via LLM summarization.
// Deferred to post-MVP.
type SummarizeStrategy struct{}

// NewSummarizeStrategy creates a summarization-based truncation strategy.
func NewSummarizeStrategy() *SummarizeStrategy {
	return &SummarizeStrategy{}
}

// Apply generates a summary of older turns.
func (s *SummarizeStrategy) Apply(turns []Turn, maxTokens int) ([]Turn, bool, error) {
	return nil, false, errors.New("not implemented")
}

// TruncateMiddleStrategy preserves first and last turns, removes middle.
// Deferred to post-MVP.
type TruncateMiddleStrategy struct{}

// NewTruncateMiddleStrategy creates a middle-truncation strategy.
func NewTruncateMiddleStrategy() *TruncateMiddleStrategy {
	return &TruncateMiddleStrategy{}
}

// Apply keeps beginning and end of conversation, drops middle turns.
func (t *TruncateMiddleStrategy) Apply(turns []Turn, maxTokens int) ([]Turn, bool, error) {
	return nil, false, errors.New("not implemented")
}

// ContextWindowState tracks the state of context window management.
type ContextWindowState struct {
	Strategy        ContextWindowStrategy // strategy used
	TruncationCount int                   // number of times truncation was applied
	TurnsDropped    int                   // total number of turns dropped
	TokensDropped   int                   // total number of tokens dropped
	LastTruncatedAt int                   // turn number when last truncation occurred
}

// NewContextWindowState creates a new context window state.
func NewContextWindowState(strategy ContextWindowStrategy) *ContextWindowState {
	return &ContextWindowState{
		Strategy: strategy,
	}
}

// RecordTruncation updates state after truncation is applied.
func (s *ContextWindowState) RecordTruncation(turnsDropped int, tokensDropped int, currentTurn int) {
	s.TruncationCount++
	s.TurnsDropped += turnsDropped
	s.TokensDropped += tokensDropped
	s.LastTruncatedAt = currentTurn
}

// WasTruncated returns true if any truncation has occurred.
func (s *ContextWindowState) WasTruncated() bool {
	return s.TruncationCount > 0
}
