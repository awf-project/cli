package workflow

import (
	"errors"
	"time"
)

// ExpressionCompiler is a function type for validating expression syntax.
// This type mirrors the ports.ExpressionValidator.Compile signature but is defined
// in the workflow package to avoid import cycles while maintaining hexagonal architecture.
// Returns nil if the expression is syntactically valid, error otherwise.
type ExpressionCompiler func(expression string) error

// StepTypeChecker is a function type for querying whether a step type name is registered
// by a plugin provider. Defined in the workflow package to avoid import cycles.
// Returns true if the type is known (custom step type accepted), false otherwise.
type StepTypeChecker func(typeName string) bool

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
	Role    TurnRole
	Content string
	Tokens  int
}

// NewTurn creates a new Turn with the given role and content.
func NewTurn(role TurnRole, content string) *Turn {
	return &Turn{
		Role:    role,
		Content: content,
	}
}

// Validate checks if the turn is valid.
func (t *Turn) Validate() error {
	if t.Role == "" {
		return errors.New("turn role cannot be empty")
	}
	if t.Role != TurnRoleSystem && t.Role != TurnRoleUser && t.Role != TurnRoleAssistant {
		return errors.New("invalid turn role")
	}
	if t.Content == "" {
		return errors.New("turn content cannot be empty")
	}
	if t.Tokens < 0 {
		return errors.New("turn tokens cannot be negative")
	}
	return nil
}

// ConversationConfig holds configuration for conversation mode execution.
type ConversationConfig struct {
	ContinueFrom string // step name to continue conversation from
}

// Validate checks if the conversation configuration is valid.
func (c *ConversationConfig) Validate() error {
	return nil
}

// StopReason indicates why a conversation stopped.
type StopReason string

const (
	StopReasonUserExit StopReason = "user_exit"
	StopReasonError    StopReason = "error"
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
	DisplayOutput   string             // filtered human-readable output for display (empty when output_format=json or no parser)
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
