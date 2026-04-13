package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
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
	evaluator     ports.ExpressionEvaluator
	resolver      interpolation.Resolver
	tokenizer     ports.Tokenizer
	agentRegistry ports.AgentRegistry
}

func NewConversationManager(
	logger ports.Logger,
	evaluator ports.ExpressionEvaluator,
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

// validateConversationInputs validates step and config inputs.
func (m *ConversationManager) validateConversationInputs(
	step *workflow.Step,
	config *workflow.ConversationConfig,
) error {
	if step == nil || step.Agent == nil {
		return errors.New("step or agent config is nil")
	}
	if config == nil {
		return errors.New("conversation config is nil")
	}
	return nil
}

// initializeConversationState initializes conversation state with system prompt
// and resolves initial prompt with interpolation. When config.ContinueFrom is set,
// loads prior conversation state from the referenced predecessor step.
func (m *ConversationManager) initializeConversationState(
	step *workflow.Step,
	resolvedProvider string,
	config *workflow.ConversationConfig,
	execCtx *workflow.ExecutionContext,
	buildContext ContextBuilderFunc,
) (*workflow.ConversationState, string, error) {
	var state *workflow.ConversationState
	if config.ContinueFrom != "" {
		priorStepState, ok := execCtx.GetStepState(config.ContinueFrom)
		if !ok {
			return nil, "", fmt.Errorf("continue_from: step %q not found in execution context", config.ContinueFrom)
		}
		if priorStepState.Conversation == nil {
			return nil, "", fmt.Errorf("continue_from: step %q has no conversation state", config.ContinueFrom)
		}
		prior := priorStepState.Conversation
		if prior.SessionID == "" && len(prior.Turns) == 0 {
			return nil, "", fmt.Errorf("continue_from: step %q has no session ID or conversation history to resume", config.ContinueFrom)
		}
		// openai_compatible uses Turns for session resume, not SessionID
		if resolvedProvider == "openai_compatible" && len(prior.Turns) == 0 {
			return nil, "", fmt.Errorf("continue_from: step %q has no conversation turns for HTTP-based provider %q", config.ContinueFrom, resolvedProvider)
		}
		// Clone prior state for the new step
		state = &workflow.ConversationState{
			SessionID:   prior.SessionID,
			Turns:       make([]workflow.Turn, len(prior.Turns)),
			TotalTurns:  prior.TotalTurns,
			TotalTokens: prior.TotalTokens,
		}
		copy(state.Turns, prior.Turns)
	} else {
		systemPrompt := step.Agent.SystemPrompt
		state = workflow.NewConversationState(systemPrompt)
	}

	initialPrompt := step.Agent.Prompt
	if step.Agent.InitialPrompt != "" {
		initialPrompt = step.Agent.InitialPrompt
	}

	intCtx := buildContext(execCtx)
	resolvedPrompt, err := m.resolver.Resolve(initialPrompt, intCtx)
	if err != nil {
		return nil, "", fmt.Errorf("step %s: resolve prompt: %w", step.Name, err)
	}

	return state, resolvedPrompt, nil
}

// executeTurn executes a single conversation turn with the provider.
func (m *ConversationManager) executeTurn(
	ctx context.Context,
	provider ports.AgentProvider,
	state *workflow.ConversationState,
	prompt string,
	options map[string]any,
	stdoutW, stderrW io.Writer,
) (*workflow.ConversationResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result, err := provider.ExecuteConversation(ctx, state, prompt, options, stdoutW, stderrW)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// evaluateTurnCompletion evaluates stop conditions and max tokens,
// returns true if conversation should stop.
func (m *ConversationManager) evaluateTurnCompletion(
	config *workflow.ConversationConfig,
	state *workflow.ConversationState,
	execCtx *workflow.ExecutionContext,
	buildContext ContextBuilderFunc,
) bool {
	if config.StopCondition != "" {
		stopCtx := buildContext(execCtx)
		if stopCtx.Inputs == nil {
			stopCtx.Inputs = make(map[string]any)
		}
		stopCtx.Inputs["response"] = state.GetLastAssistantResponse()
		stopCtx.Inputs["turn_count"] = state.TotalTurns

		shouldStop, err := m.evaluator.EvaluateBool(config.StopCondition, stopCtx)
		if err != nil {
			m.logger.Warn("failed to evaluate stop condition", "error", err)
		} else if shouldStop {
			state.StoppedBy = workflow.StopReasonCondition
			return true
		}
	}

	if config.MaxContextTokens > 0 && state.TotalTokens >= config.MaxContextTokens {
		state.StoppedBy = workflow.StopReasonMaxTokens
		return true
	}

	return false
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
	stdoutW, stderrW io.Writer,
) (*workflow.ConversationResult, error) {
	if err := m.validateConversationInputs(step, config); err != nil {
		return nil, err
	}

	intCtx := buildContext(execCtx)
	resolvedProvider, err := m.resolver.Resolve(step.Agent.Provider, intCtx)
	if err != nil {
		return nil, fmt.Errorf("step %s: resolve provider: %w", step.Name, err)
	}

	provider, err := m.agentRegistry.Get(resolvedProvider)
	if err != nil {
		return nil, fmt.Errorf("step %s: %w", step.Name, err)
	}

	state, resolvedPrompt, err := m.initializeConversationState(step, resolvedProvider, config, execCtx, buildContext)
	if err != nil {
		return nil, err
	}

	// Clone options to preserve FR-009 immutability of step.Agent.Options,
	// and inject output_format so baseCLIProvider can route display filtering
	// identically between executeAgentStep and conversation mode (F082).
	options := cloneAndInjectOutputFormat(step.Agent.Options, step.Agent.OutputFormat)
	if step.Agent.SystemPrompt != "" {
		options["system_prompt"] = step.Agent.SystemPrompt
	}

	maxTurns := config.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 10
	}

	var lastResult *workflow.ConversationResult
	for turnCount := 0; turnCount < maxTurns; turnCount++ {
		result, err := m.executeTurn(ctx, provider, state, resolvedPrompt, options, stdoutW, stderrW)
		if err != nil {
			return nil, err
		}

		state = result.State
		lastResult = result

		if m.evaluateTurnCompletion(config, state, execCtx, buildContext) {
			break
		}

		intCtx := buildContext(execCtx)
		resolvedPrompt, err = m.resolver.Resolve(step.Agent.Prompt, intCtx)
		if err != nil {
			return nil, err
		}

		if config.InjectContext != "" {
			resolvedInjectContext, injectErr := m.resolver.Resolve(config.InjectContext, intCtx)
			if injectErr != nil {
				return nil, fmt.Errorf("inject_context: %w", injectErr)
			}
			if trimmed := strings.TrimSpace(resolvedInjectContext); trimmed != "" {
				resolvedPrompt = resolvedPrompt + "\n\n" + trimmed
			}
		}
	}

	if state.StoppedBy == "" {
		if state.TotalTurns >= maxTurns {
			state.StoppedBy = workflow.StopReasonMaxTurns
		}
	}

	if lastResult != nil {
		lastResult.State = state
		return lastResult, nil
	}

	result := workflow.NewConversationResult(provider.Name())
	result.State = state
	return result, nil
}
