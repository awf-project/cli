package agents

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// CodexProvider implements AgentProvider for Codex CLI.
// Invokes: codex --prompt "prompt" --quiet
type CodexProvider struct{}

// NewCodexProvider creates a new CodexProvider.
func NewCodexProvider() *CodexProvider {
	return &CodexProvider{}
}

// Execute invokes the Codex CLI with the given prompt and options.
func (p *CodexProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	// Validate prompt
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	// Validate options
	if err := validateCodexOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("codex provider: %w", err)
	}

	// Build command arguments
	args := []string{"--prompt", prompt}

	// Apply options
	if options != nil {
		if language, ok := options["language"].(string); ok {
			args = append(args, "--language", language)
		}
		if maxTokens, ok := options["max_tokens"].(int); ok {
			args = append(args, "--max-tokens", fmt.Sprintf("%d", maxTokens))
		}
		if quiet, ok := options["quiet"].(bool); ok && quiet {
			args = append(args, "--quiet")
		}
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "codex", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("codex execution failed: %w", err)
	}

	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:    "codex",
		Output:      outputStr,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Tokens:      estimateCodexTokens(outputStr),
	}

	return result, nil
}

// ExecuteConversation invokes the Codex CLI with conversation history for multi-turn interactions.
func (p *CodexProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	// Validate state
	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	// Validate prompt
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	// Validate options
	if err := validateCodexConversationOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("codex provider: %w", err)
	}

	// Clone state to avoid modifying original
	workingState := cloneCodexState(state)

	// Add user turn to conversation history
	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	// Build command arguments
	args := []string{"--prompt", prompt}

	// Apply options
	if options != nil {
		if model, ok := options["model"].(string); ok {
			args = append(args, "--model", model)
		}
		if language, ok := options["language"].(string); ok {
			args = append(args, "--language", language)
		}
		if maxTokens, ok := options["max_tokens"].(int); ok {
			args = append(args, "--max-tokens", fmt.Sprintf("%d", maxTokens))
		}
		if temperature, ok := options["temperature"].(float64); ok {
			args = append(args, "--temperature", fmt.Sprintf("%.2f", temperature))
		}
		if quiet, ok := options["quiet"].(bool); ok && quiet {
			args = append(args, "--quiet")
		}
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "codex", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("codex execution failed: %w", err)
	}

	outputStr := string(output)

	// Add assistant turn to conversation history
	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateCodexTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	// Estimate input tokens (all previous turns)
	inputTokens := 0
	for i := 0; i < len(workingState.Turns)-1; i++ {
		if workingState.Turns[i].Tokens == 0 {
			workingState.Turns[i].Tokens = estimateCodexTokens(workingState.Turns[i].Content)
		}
		inputTokens += workingState.Turns[i].Tokens
	}

	// Create result
	result := &workflow.ConversationResult{
		Provider:        "codex",
		State:           workingState,
		Output:          outputStr,
		TokensInput:     inputTokens,
		TokensOutput:    assistantTurn.Tokens,
		TokensTotal:     inputTokens + assistantTurn.Tokens,
		TokensEstimated: true,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
	}

	return result, nil
}

// Name returns the provider identifier.
func (p *CodexProvider) Name() string {
	return "codex"
}

// Validate checks if the Codex CLI is installed and accessible.
func (p *CodexProvider) Validate() error {
	_, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("codex CLI not found in PATH: %w", err)
	}
	return nil
}

// validateCodexOptions validates provider-specific options.
func validateCodexOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	// Validate max_tokens
	if maxTokens, ok := options["max_tokens"].(int); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	// Validate language
	if language, ok := options["language"]; ok {
		if _, isString := language.(string); !isString {
			return errors.New("language must be a string")
		}
	}

	return nil
}

// validateCodexConversationOptions validates conversation-specific options.
func validateCodexConversationOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	// Validate temperature type and value
	if val, exists := options["temperature"]; exists {
		temp, ok := val.(float64)
		if !ok {
			return errors.New("temperature must be a number")
		}
		if temp < 0 || temp > 2 {
			return errors.New("temperature must be between 0 and 2")
		}
	}

	// Validate max_tokens
	if maxTokens, ok := options["max_tokens"].(int); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	// Validate language
	if language, ok := options["language"]; ok {
		if _, isString := language.(string); !isString {
			return errors.New("language must be a string")
		}
	}

	return nil
}

// cloneCodexState creates a shallow copy of ConversationState.
func cloneCodexState(state *workflow.ConversationState) *workflow.ConversationState {
	if state == nil {
		return nil
	}

	// Create new state with copied turns slice
	turns := make([]workflow.Turn, len(state.Turns))
	copy(turns, state.Turns)

	return &workflow.ConversationState{
		Turns:       turns,
		TotalTurns:  state.TotalTurns,
		TotalTokens: state.TotalTokens,
		StoppedBy:   state.StoppedBy,
	}
}

// estimateCodexTokens provides a rough token count estimation.
func estimateCodexTokens(output string) int {
	// Rough estimation: ~4 characters per token
	return len(output) / 4
}
