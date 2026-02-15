package agents

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
)

// CodexProvider implements AgentProvider for Codex CLI.
// Invokes: codex --prompt "prompt" --quiet
type CodexProvider struct {
	executor ports.CLIExecutor
}

func NewCodexProvider() *CodexProvider {
	return &CodexProvider{
		executor: NewExecCLIExecutor(),
	}
}

func NewCodexProviderWithOptions(opts ...CodexProviderOption) *CodexProvider {
	p := &CodexProvider{
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *CodexProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateCodexOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("codex provider: %w", err)
	}

	args := []string{"--prompt", prompt}

	if language, ok := getStringOption(options, "language"); ok {
		args = append(args, "--language", language)
	}
	if maxTokens, ok := getIntOption(options, "max_tokens"); ok {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", maxTokens))
	}
	if quiet, ok := getBoolOption(options, "quiet"); ok && quiet {
		args = append(args, "--quiet")
	}

	stdout, stderr, err := p.executor.Run(ctx, "codex", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("codex execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:    "codex",
		Output:      outputStr,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Tokens:      estimateTokens(outputStr),
	}

	return result, nil
}

func (p *CodexProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateCodexConversationOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("codex provider: %w", err)
	}

	workingState := cloneState(state)

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	args := []string{"--prompt", prompt}

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}
	if language, ok := getStringOption(options, "language"); ok {
		args = append(args, "--language", language)
	}
	if maxTokens, ok := getIntOption(options, "max_tokens"); ok {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", maxTokens))
	}
	if temperature, ok := getFloatOption(options, "temperature"); ok {
		args = append(args, "--temperature", fmt.Sprintf("%.2f", temperature))
	}
	if quiet, ok := getBoolOption(options, "quiet"); ok && quiet {
		args = append(args, "--quiet")
	}

	stdout, stderr, err := p.executor.Run(ctx, "codex", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("codex execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := string(output)

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	inputTokens := 0
	for i := 0; i < len(workingState.Turns)-1; i++ {
		if workingState.Turns[i].Tokens == 0 {
			workingState.Turns[i].Tokens = estimateTokens(workingState.Turns[i].Content)
		}
		inputTokens += workingState.Turns[i].Tokens
	}

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
	if maxTokens, ok := getIntOption(options, "max_tokens"); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	return nil
}

// validateCodexConversationOptions validates conversation-specific options.
func validateCodexConversationOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	// Validate temperature
	if temp, ok := getFloatOption(options, "temperature"); ok {
		if temp < 0 || temp > 2 {
			return errors.New("temperature must be between 0 and 2")
		}
	}

	// Validate max_tokens
	if maxTokens, ok := getIntOption(options, "max_tokens"); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	return nil
}
