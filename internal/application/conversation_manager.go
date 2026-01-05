package application

import (
	"context"
	"errors"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// ConversationManager orchestrates multi-turn agent conversations with automatic
// context window management, token counting, and stop condition evaluation.
//
// Following the LoopExecutor pattern, ConversationManager:
// - Manages turn iteration (analogous to loop iterations)
// - Evaluates stop conditions (analogous to break conditions)
// - Maintains conversation state (analogous to loop context)
// - Integrates with AgentProvider for turn execution
type ConversationManager struct {
	logger        ports.Logger
	evaluator     ExpressionEvaluator
	resolver      interpolation.Resolver
	tokenizer     ports.Tokenizer
	agentRegistry ports.AgentRegistry
}

// NewConversationManager creates a new ConversationManager with required dependencies.
// Dependencies follow application layer DI pattern:
// - logger: structured logging for turn execution
// - evaluator: expression evaluation for stop conditions
// - resolver: template interpolation for prompts
// - tokenizer: token counting for context window management
// - agentRegistry: provider lookup for agent execution
func NewConversationManager(
	logger ports.Logger,
	evaluator ExpressionEvaluator,
	resolver interpolation.Resolver,
	tokenizer ports.Tokenizer,
	agentRegistry ports.AgentRegistry,
) *ConversationManager {
	return &ConversationManager{
		logger:        logger,
		evaluator:     evaluator,
		resolver:      resolver,
		tokenizer:     tokenizer,
		agentRegistry: agentRegistry,
	}
}

// ExecuteConversation orchestrates a multi-turn conversation according to the
// configuration in the agent step's conversation settings.
//
// Flow:
//  1. Initialize conversation state with system prompt (if provided)
//  2. Execute initial user prompt to start conversation
//  3. For each turn:
//     a. Execute agent provider with conversation history
//     b. Add agent response to conversation state
//     c. Count tokens and apply context window strategy if needed
//     d. Evaluate stop condition
//     e. Check max turns/tokens limits
//     f. If continuing, prepare next user prompt
//  4. Return final ConversationResult
//
// Parameters:
// - ctx: context for cancellation and timeout
// - step: agent step configuration with conversation settings
// - config: conversation configuration (max_turns, strategy, stop_condition)
// - execCtx: execution context with state and inputs
// - buildContext: function to build interpolation context for template resolution
//
// Returns:
// - ConversationResult with final state, output, token counts, and stop reason
// - error if conversation execution fails
func (m *ConversationManager) ExecuteConversation(
	ctx context.Context,
	step *workflow.Step,
	config *workflow.ConversationConfig,
	execCtx *workflow.ExecutionContext,
	buildContext ContextBuilderFunc,
) (*workflow.ConversationResult, error) {
	return nil, errors.New("not implemented")
}
