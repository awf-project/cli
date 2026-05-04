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

func cloneConversationState(state *workflow.ConversationState) *workflow.ConversationState {
	if state == nil {
		return nil
	}
	turns := make([]workflow.Turn, len(state.Turns))
	copy(turns, state.Turns)
	return &workflow.ConversationState{
		Turns:       turns,
		TotalTurns:  state.TotalTurns,
		TotalTokens: state.TotalTokens,
		StoppedBy:   state.StoppedBy,
		SessionID:   state.SessionID,
	}
}

// ConversationManager orchestrates interactive user→agent→user conversations.
//
// Each turn: resolve initial prompt → send to provider → stream response →
// read user input → repeat until empty input or context cancellation.
type ConversationManager struct {
	logger          ports.Logger
	resolver        interpolation.Resolver
	agentRegistry   ports.AgentRegistry
	userInputReader ports.UserInputReader
}

func NewConversationManager(
	logger ports.Logger,
	resolver interpolation.Resolver,
	agentRegistry ports.AgentRegistry,
) *ConversationManager {
	return &ConversationManager{
		logger:        logger,
		resolver:      resolver,
		agentRegistry: agentRegistry,
	}
}

// SetUserInputReader wires the optional user input reader for interactive conversations.
// When nil, conversations that require user input will return an error.
func (m *ConversationManager) SetUserInputReader(r ports.UserInputReader) {
	m.userInputReader = r
}

// validateConversationInputs validates step and agent config inputs.
// ConversationConfig is optional — a nil config is treated as an empty config
// (no ContinueFrom reference).
func (m *ConversationManager) validateConversationInputs(
	step *workflow.Step,
	_ *workflow.ConversationConfig,
) error {
	if step == nil || step.Agent == nil {
		return errors.New("step or agent config is nil")
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
	if config != nil && config.ContinueFrom != "" {
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

	intCtx := buildContext(execCtx)
	resolvedPrompt, err := m.resolver.Resolve(step.Agent.Prompt, intCtx)
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

// ExecuteConversation orchestrates an interactive user→agent→user conversation.
//
// Flow:
//  1. Initialize conversation state with system prompt (if provided)
//  2. Execute initial user prompt to start conversation
//  3. Read user input; empty input ends the conversation (StopReasonUserExit)
//  4. Repeat until empty input or context cancellation
//
// Parameters:
// - ctx: context for cancellation and timeout
// - step: agent step configuration with conversation settings
// - config: conversation configuration (continue_from)
// - execCtx: execution context with state and inputs
// - buildContext: function to build interpolation context for template resolution
//
// Returns:
// - ConversationResult with final state, output, and stop reason
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

	// Clone options to preserve immutability of step.Agent.Options,
	// and inject output_format so baseCLIProvider can route display filtering
	// identically between executeAgentStep and conversation mode (F082).
	options := cloneAndInjectOutputFormat(step.Agent.Options, step.Agent.OutputFormat)
	if step.Agent.SystemPrompt != "" {
		options["system_prompt"] = step.Agent.SystemPrompt
	}

	if m.userInputReader == nil {
		return nil, errors.New("conversation mode requires a UserInputReader; none configured")
	}

	var lastResult *workflow.ConversationResult

	for {
		// Push user turn immediately so the TUI poll sees it before the
		// provider responds (gives instant visual feedback).
		previewState := cloneConversationState(state)
		if err := previewState.AddTurn(workflow.NewTurn(workflow.TurnRoleUser, resolvedPrompt)); err != nil {
			return nil, fmt.Errorf("preview turn: %w", err)
		}
		execCtx.SetStepState(step.Name, workflow.StepState{
			Name:         step.Name,
			Status:       workflow.StatusRunning,
			Conversation: previewState,
		})

		result, err := m.executeTurn(ctx, provider, state, resolvedPrompt, options, stdoutW, stderrW)
		if err != nil {
			if lastResult != nil {
				lastResult.State = state
				lastResult.Error = err
			}
			return lastResult, err
		}

		state = result.State
		lastResult = result

		// Push complete state with both user and assistant turns.
		execCtx.SetStepState(step.Name, workflow.StepState{
			Name:         step.Name,
			Status:       workflow.StatusRunning,
			Conversation: state,
			Output:       result.Output,
		})

		userInput, err := m.userInputReader.ReadInput(ctx)
		if err != nil {
			state.StoppedBy = workflow.StopReasonError
			lastResult.State = state
			return lastResult, fmt.Errorf("reading user input: %w", err)
		}

		if strings.TrimSpace(userInput) == "" {
			state.StoppedBy = workflow.StopReasonUserExit
			break
		}

		resolvedPrompt = userInput
	}

	if lastResult != nil {
		lastResult.State = state
		return lastResult, nil
	}

	result := workflow.NewConversationResult(provider.Name())
	result.State = state
	return result, nil
}
