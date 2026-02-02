package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/logger"
)

// ClaudeProvider implements AgentProvider for Claude CLI.
// Invokes: claude -p "prompt" --output-format json
type ClaudeProvider struct {
	logger   ports.Logger
	executor ports.CLIExecutor
}

// NewClaudeProvider creates a new ClaudeProvider.
// If logger is nil, a NopLogger is used.
// If no executor is provided via options, ExecCLIExecutor is used by default.
func NewClaudeProvider(l ...ports.Logger) *ClaudeProvider {
	var log ports.Logger
	if len(l) > 0 && l[0] != nil {
		log = l[0]
	} else {
		log = logger.NopLogger{}
	}
	return &ClaudeProvider{
		logger:   log,
		executor: NewExecCLIExecutor(),
	}
}

// NewClaudeProviderWithOptions creates a new ClaudeProvider with functional options.
func NewClaudeProviderWithOptions(opts ...ClaudeProviderOption) *ClaudeProvider {
	p := &ClaudeProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Execute invokes the Claude CLI with the given prompt and options.
func (p *ClaudeProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	// Validate prompt
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	// Validate options
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("claude provider: %w", err)
	}

	// Build command arguments
	args := []string{"-p", prompt}

	// Apply options (only those supported by Claude CLI)
	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok && outputFormat == "json" {
		args = append(args, "--output-format", "json")
	}
	if allowedTools, ok := getStringOption(options, "allowedTools"); ok && allowedTools != "" {
		args = append(args, "--allowedTools", allowedTools)
	}
	if skipPerms, ok := getBoolOption(options, "dangerouslySkipPermissions"); ok && skipPerms {
		args = append(args, "--dangerously-skip-permissions")
		p.logger.Info("[SECURITY AUDIT] dangerouslySkipPermissions enabled",
			"timestamp", time.Now().Format(time.RFC3339),
			"workflow", getWorkflowID(options),
			"step", getStepName(options))
	}
	// Note: temperature and max_tokens are validated but not passed to CLI

	// Execute command
	stdout, stderr, err := p.executor.Run(ctx, "claude", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

	// Combine stdout and stderr like CombinedOutput()
	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:        "claude",
		Output:          outputStr,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		Tokens:          estimateTokens(outputStr),
		TokensEstimated: true, // using rough estimation, not actual API usage
	}

	// Parse JSON response if output format is JSON
	if options != nil {
		if format, ok := options["output_format"].(string); ok && format == "json" {
			var jsonResp map[string]any
			if err := json.Unmarshal(output, &jsonResp); err != nil {
				return nil, fmt.Errorf("failed to parse JSON output: %w", err)
			}
			result.Response = jsonResp
		}
	}

	return result, nil
}

// ExecuteConversation invokes the Claude CLI with conversation history for multi-turn interactions.
//
//nolint:gocognit // Complexity 31: conversation executor manages multi-turn state, context windows, token limits, retries, streaming. Conversation orchestration requires this.
func (p *ClaudeProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
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
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("claude provider: %w", err)
	}

	// Clone state to avoid modifying original
	workingState := cloneState(state)

	// Add user turn to conversation history
	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	// Build command arguments
	args := []string{"-p", prompt}

	// Apply options (only those supported by Claude CLI)
	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok && outputFormat == "json" {
		args = append(args, "--output-format", "json")
	}
	if allowedTools, ok := getStringOption(options, "allowedTools"); ok && allowedTools != "" {
		args = append(args, "--allowedTools", allowedTools)
	}
	if skipPerms, ok := getBoolOption(options, "dangerouslySkipPermissions"); ok && skipPerms {
		args = append(args, "--dangerously-skip-permissions")
		p.logger.Info("[SECURITY AUDIT] dangerouslySkipPermissions enabled",
			"timestamp", time.Now().Format(time.RFC3339),
			"workflow", getWorkflowID(options),
			"step", getStepName(options))
	}

	// Execute command
	stdout, stderr, err := p.executor.Run(ctx, "claude", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

	// Combine stdout and stderr like CombinedOutput()
	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := string(output)

	// Add assistant turn to conversation history
	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	// Estimate input tokens (all previous turns)
	inputTokens := 0
	for i := 0; i < len(workingState.Turns)-1; i++ {
		if workingState.Turns[i].Tokens == 0 {
			workingState.Turns[i].Tokens = estimateTokens(workingState.Turns[i].Content)
		}
		inputTokens += workingState.Turns[i].Tokens
	}

	// Create result
	result := &workflow.ConversationResult{
		Provider:        "claude",
		State:           workingState,
		Output:          outputStr,
		TokensInput:     inputTokens,
		TokensOutput:    assistantTurn.Tokens,
		TokensTotal:     inputTokens + assistantTurn.Tokens,
		TokensEstimated: true,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
	}

	// Parse JSON response if output format is JSON
	if options != nil {
		if format, ok := options["output_format"].(string); ok && format == "json" {
			var jsonResp map[string]any
			if err := json.Unmarshal(output, &jsonResp); err != nil {
				return nil, fmt.Errorf("failed to parse JSON output: %w", err)
			}
			result.Response = jsonResp
		}
	}

	return result, nil
}

// Name returns the provider identifier.
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// Validate checks if the Claude CLI is installed and accessible.
func (p *ClaudeProvider) Validate() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	return nil
}

// validateOptions validates provider-specific options.
func validateOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	// Validate max_tokens
	if maxTokens, ok := getIntOption(options, "max_tokens"); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	// Validate temperature
	if temp, ok := getFloatOption(options, "temperature"); ok {
		if temp < 0 || temp > 1 {
			return errors.New("temperature must be between 0 and 1")
		}
	}

	// Validate model format
	if model, ok := getStringOption(options, "model"); ok {
		if !isValidClaudeModel(model) {
			return fmt.Errorf("invalid model format: %s (must be an alias or start with 'claude-')", model)
		}
	}

	return nil
}

// isValidClaudeModel checks if the model is a valid alias or Claude model name.
func isValidClaudeModel(model string) bool {
	aliases := []string{"sonnet", "opus", "haiku"}
	return slices.Contains(aliases, model) || strings.HasPrefix(model, "claude-")
}
