package workflow

import "errors"

type ContextWindowManager interface {
	ApplyStrategy(turns []Turn, maxTokens int, strategy ContextWindowStrategy) ([]Turn, bool, error)
	PreserveSystemPrompt(turns []Turn) (*Turn, []Turn)
	CalculateTotalTokens(turns []Turn) int
	EstimateTokens(content string) int
}

type contextWindowManager struct{}

func NewContextWindowManager() ContextWindowManager {
	return &contextWindowManager{}
}

func (m *contextWindowManager) ApplyStrategy(turns []Turn, maxTokens int, strategy ContextWindowStrategy) ([]Turn, bool, error) {
	if maxTokens <= 0 {
		return nil, false, errors.New("max_tokens must be positive")
	}

	switch strategy {
	case StrategySlidingWindow:
		s := NewSlidingWindowStrategy()
		return s.Apply(turns, maxTokens)
	case StrategySummarize:
		return nil, false, errors.New("summarize strategy not implemented yet")
	case StrategyTruncateMiddle:
		return nil, false, errors.New("truncate_middle strategy not implemented yet")
	case StrategyNone:
		return turns, false, nil
	default:
		return nil, false, errors.New("invalid strategy")
	}
}

func (m *contextWindowManager) PreserveSystemPrompt(turns []Turn) (systemPrompt *Turn, remaining []Turn) {
	if len(turns) == 0 {
		return nil, []Turn{}
	}

	var systemTurn *Turn
	otherTurns := make([]Turn, 0, len(turns))

	for i := range turns {
		if turns[i].Role == TurnRoleSystem && systemTurn == nil {
			systemTurn = &turns[i]
		} else {
			otherTurns = append(otherTurns, turns[i])
		}
	}

	return systemTurn, otherTurns
}

func (m *contextWindowManager) CalculateTotalTokens(turns []Turn) int {
	total := 0
	for _, turn := range turns {
		total += turn.Tokens
	}
	return total
}

// Uses a rough approximation of 1 token ≈ 4 characters.
func (m *contextWindowManager) EstimateTokens(content string) int {
	return len(content) / 4
}

type SlidingWindowStrategy struct{}

func NewSlidingWindowStrategy() *SlidingWindowStrategy {
	return &SlidingWindowStrategy{}
}

func (s *SlidingWindowStrategy) Apply(turns []Turn, maxTokens int) ([]Turn, bool, error) {
	if maxTokens <= 0 {
		return nil, false, errors.New("max_tokens must be positive")
	}

	if len(turns) == 0 {
		return []Turn{}, false, nil
	}

	var systemTurn *Turn
	otherTurns := make([]Turn, 0, len(turns))

	for i := range turns {
		if turns[i].Role == TurnRoleSystem && systemTurn == nil {
			systemTurn = &turns[i]
		} else {
			otherTurns = append(otherTurns, turns[i])
		}
	}

	totalTokens := 0
	if systemTurn != nil {
		totalTokens += systemTurn.Tokens
	}
	for _, turn := range otherTurns {
		totalTokens += turn.Tokens
	}

	if totalTokens <= maxTokens {
		return turns, false, nil
	}

	result := make([]Turn, 0, len(turns))
	currentTokens := 0

	if systemTurn != nil {
		result = append(result, *systemTurn)
		currentTokens = systemTurn.Tokens
	}

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

	result = append(result, otherTurns[startIdx:]...)

	wasTruncated := len(result) < len(turns)
	return result, wasTruncated, nil
}

type SummarizeStrategy struct{}

func NewSummarizeStrategy() *SummarizeStrategy {
	return &SummarizeStrategy{}
}

func (s *SummarizeStrategy) Apply(turns []Turn, maxTokens int) ([]Turn, bool, error) {
	return nil, false, errors.New("not implemented")
}

type TruncateMiddleStrategy struct{}

func NewTruncateMiddleStrategy() *TruncateMiddleStrategy {
	return &TruncateMiddleStrategy{}
}

func (t *TruncateMiddleStrategy) Apply(turns []Turn, maxTokens int) ([]Turn, bool, error) {
	return nil, false, errors.New("not implemented")
}

type ContextWindowState struct {
	Strategy        ContextWindowStrategy
	TruncationCount int
	TurnsDropped    int
	TokensDropped   int
	LastTruncatedAt int
}

func NewContextWindowState(strategy ContextWindowStrategy) *ContextWindowState {
	return &ContextWindowState{
		Strategy: strategy,
	}
}

func (s *ContextWindowState) RecordTruncation(turnsDropped, tokensDropped, currentTurn int) {
	s.TruncationCount++
	s.TurnsDropped += turnsDropped
	s.TokensDropped += tokensDropped
	s.LastTruncatedAt = currentTurn
}

func (s *ContextWindowState) WasTruncated() bool {
	return s.TruncationCount > 0
}
