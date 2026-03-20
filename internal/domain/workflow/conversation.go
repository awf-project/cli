package workflow

import (
	"errors"
	"fmt"
	"time"
)

// ExpressionCompiler is a function type for validating expression syntax.
// This type mirrors the ports.ExpressionValidator.Compile signature but is defined
// in the workflow package to avoid import cycles while maintaining hexagonal architecture.
// Returns nil if the expression is syntactically valid, error otherwise.
type ExpressionCompiler func(expression string) error

// Conversation errors
var (
	ErrNilTurn = errors.New("cannot add nil turn")
)

// TurnRole represents the role of a conversation turn participant.
type TurnRole string

const (
	TurnRoleSystem    TurnRole = "system"
	TurnRoleUser      TurnRole = "user"
	TurnRoleAssistant TurnRole = "assistant"
)

// Turn represents a single message in a conversation.
type Turn struct {
	Role    TurnRole // system, user, or assistant
	Content string   // message content
	Tokens  int      // token count for this turn
}

// NewTurn creates a new Turn with the given role and content.
func NewTurn(role TurnRole, content string) *Turn {
	return &Turn{
		Role:    role,
		Content: content,
		Tokens:  0, // Will be filled by tokenizer
	}
}

// Validate checks if the turn is valid.
func (t *Turn) Validate() error {
	// Validate role
	if t.Role == "" {
		return errors.New("turn role cannot be empty")
	}
	if t.Role != TurnRoleSystem && t.Role != TurnRoleUser && t.Role != TurnRoleAssistant {
		return errors.New("invalid turn role")
	}

	// Validate content
	if t.Content == "" {
		return errors.New("turn content cannot be empty")
	}

	// Validate tokens
	if t.Tokens < 0 {
		return errors.New("turn tokens cannot be negative")
	}

	return nil
}

// ContextWindowStrategy defines the strategy for managing context window limits.
type ContextWindowStrategy string

const (
	StrategyNone           ContextWindowStrategy = ""
	StrategySlidingWindow  ContextWindowStrategy = "sliding_window"
	StrategySummarize      ContextWindowStrategy = "summarize"
	StrategyTruncateMiddle ContextWindowStrategy = "truncate_middle"
)

// ConversationConfig holds configuration for conversation mode execution.
type ConversationConfig struct {
	MaxTurns         int                   // maximum number of turns (default 10, max 100)
	MaxContextTokens int                   // maximum tokens in context window (0 = provider default)
	Strategy         ContextWindowStrategy // context window management strategy
	StopCondition    string                // expression to evaluate for early exit
	ContinueFrom     string                // step name to continue conversation from
	InjectContext    string                // additional context to inject mid-conversation
}

// Validate checks if the conversation configuration is valid.
// The validator parameter is used to check stop condition expression syntax.
func (c *ConversationConfig) Validate(validator ExpressionCompiler) error {
	// Validate MaxTurns (0 is allowed and means use default)
	if c.MaxTurns < 0 {
		return errors.New("max_turns must be non-negative")
	}
	if c.MaxTurns > 100 {
		return errors.New("max_turns cannot exceed 100")
	}

	// Validate MaxContextTokens if set
	if c.MaxContextTokens < 0 {
		return errors.New("max_context_tokens must be non-negative")
	}

	// Validate Strategy if set
	if c.Strategy != "" {
		switch c.Strategy {
		case StrategySlidingWindow:
			// Valid strategies
		case StrategySummarize, StrategyTruncateMiddle:
			return fmt.Errorf("strategy %q is not yet implemented; use sliding_window", c.Strategy)
		default:
			return errors.New("invalid context window strategy")
		}
	}

	// Validate StopCondition if set (compile-time syntax check)
	if c.StopCondition != "" && validator != nil {
		if err := validator(c.StopCondition); err != nil {
			return fmt.Errorf("invalid stop_condition expression: %w", err)
		}
	}

	return nil
}

// GetMaxTurns returns the effective max turns with default fallback.
func (c *ConversationConfig) GetMaxTurns() int {
	if c.MaxTurns == 0 {
		return 10 // default
	}
	return c.MaxTurns
}

// StopReason indicates why a conversation stopped.
type StopReason string

const (
	StopReasonCondition StopReason = "condition"
	StopReasonMaxTurns  StopReason = "max_turns"
	StopReasonMaxTokens StopReason = "max_tokens"
	StopReasonError     StopReason = "error"
)

// ConversationState represents the state of an ongoing or completed conversation.
type ConversationState struct {
	SessionID   string     // provider-assigned session identifier for resume capability
	Turns       []Turn     // ordered array of conversation turns
	TotalTurns  int        // total number of turns executed
	TotalTokens int        // cumulative token count across all turns
	StoppedBy   StopReason // reason the conversation stopped
}

// NewConversationState creates a new conversation state with system prompt.
func NewConversationState(systemPrompt string) *ConversationState {
	state := &ConversationState{
		Turns:       make([]Turn, 0),
		TotalTurns:  0,
		TotalTokens: 0,
		StoppedBy:   "",
	}

	// Add system prompt as first turn if provided
	if systemPrompt != "" {
		systemTurn := NewTurn(TurnRoleSystem, systemPrompt)
		state.Turns = append(state.Turns, *systemTurn)
		state.TotalTurns = 1
		state.TotalTokens = systemTurn.Tokens
	}

	return state
}

// AddTurn appends a turn to the conversation history.
// Returns an error if the turn is nil.
func (s *ConversationState) AddTurn(turn *Turn) error {
	if turn == nil {
		return ErrNilTurn
	}
	s.Turns = append(s.Turns, *turn)
	s.TotalTurns++
	s.TotalTokens += turn.Tokens
	return nil
}

// GetTotalTokens returns the sum of tokens across all turns.
func (s *ConversationState) GetTotalTokens() int {
	return s.TotalTokens
}

// GetLastTurn returns the most recent turn, or nil if no turns exist.
func (s *ConversationState) GetLastTurn() *Turn {
	if len(s.Turns) == 0 {
		return nil
	}
	return &s.Turns[len(s.Turns)-1]
}

// GetLastAssistantResponse returns the content of the last assistant turn.
func (s *ConversationState) GetLastAssistantResponse() string {
	// Iterate backwards to find last assistant turn
	for i := len(s.Turns) - 1; i >= 0; i-- {
		if s.Turns[i].Role == TurnRoleAssistant {
			return s.Turns[i].Content
		}
	}
	return ""
}

// IsStopped returns true if the conversation has a stop reason set.
func (s *ConversationState) IsStopped() bool {
	return s.StoppedBy != ""
}

// ConversationResult holds the result of a conversation execution.
type ConversationResult struct {
	Provider        string             // provider name used
	State           *ConversationState // final conversation state
	Output          string             // final assistant response (last turn)
	Response        map[string]any     // parsed JSON response from last turn (if applicable)
	TokensInput     int                // total input tokens across all turns
	TokensOutput    int                // total output tokens across all turns
	TokensTotal     int                // sum of input + output tokens
	TokensEstimated bool               // true if token counts are estimates
	Error           error              // execution error, if any
	StartedAt       time.Time
	CompletedAt     time.Time
}

// NewConversationResult creates a new ConversationResult with initialized values.
func NewConversationResult(provider string) *ConversationResult {
	return &ConversationResult{
		Provider:        provider,
		State:           NewConversationState(""), // Empty state, no system prompt by default
		Output:          "",
		Response:        make(map[string]any),
		TokensInput:     0,
		TokensOutput:    0,
		TokensTotal:     0,
		TokensEstimated: false,
		Error:           nil,
		StartedAt:       time.Now(),
		CompletedAt:     time.Time{},
	}
}

// Duration returns the total execution time of the conversation.
func (r *ConversationResult) Duration() time.Duration {
	if r.StartedAt.IsZero() || r.CompletedAt.IsZero() {
		return 0
	}
	return r.CompletedAt.Sub(r.StartedAt)
}

// Success returns true if the conversation completed without error.
func (r *ConversationResult) Success() bool {
	return r.Error == nil
}

// HasJSONResponse returns true if a JSON response was successfully parsed.
func (r *ConversationResult) HasJSONResponse() bool {
	return len(r.Response) > 0
}

// TurnCount returns the number of turns in the conversation.
func (r *ConversationResult) TurnCount() int {
	if r.State == nil {
		return 0
	}
	return r.State.TotalTurns
}
