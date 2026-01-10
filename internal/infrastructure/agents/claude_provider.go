package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/logger"
)

// ClaudeProvider implements AgentProvider for Claude CLI.
// Invokes: claude -p "prompt" --output-format json
type ClaudeProvider struct {
	logger ports.Logger
}

// NewClaudeProvider creates a new ClaudeProvider.
// If logger is nil, a NopLogger is used.
func NewClaudeProvider(l ...ports.Logger) *ClaudeProvider {
	var log ports.Logger
	if len(l) > 0 && l[0] != nil {
		log = l[0]
	} else {
		log = logger.NopLogger{}
	}
	return &ClaudeProvider{
		logger: log,
	}
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
	if options != nil {
		if model, ok := options["model"].(string); ok {
			args = append(args, "--model", model)
		}
		if outputFormat, ok := options["output_format"].(string); ok {
			if outputFormat == "json" {
				args = append(args, "--output-format", "json")
			}
		}
		// allowedTools - pass tool list to Claude CLI for agentic workflows
		if allowedTools, ok := options["allowedTools"].(string); ok && allowedTools != "" {
			args = append(args, "--allowedTools", allowedTools)
		}
		// dangerouslySkipPermissions - skip permission prompts for automated execution
		if skipPerms, ok := options["dangerouslySkipPermissions"].(bool); ok && skipPerms {
			args = append(args, "--dangerously-skip-permissions")
			// Audit log for security compliance
			p.logger.Info("[SECURITY AUDIT] dangerouslySkipPermissions enabled",
				"timestamp", time.Now().Format(time.RFC3339),
				"workflow", getWorkflowID(options),
				"step", getStepName(options))
		}
		// Note: temperature and max_tokens are validated but not passed to CLI
		// as the Claude CLI does not support these options directly
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

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
	if options != nil {
		if model, ok := options["model"].(string); ok {
			args = append(args, "--model", model)
		}
		if outputFormat, ok := options["output_format"].(string); ok {
			if outputFormat == "json" {
				args = append(args, "--output-format", "json")
			}
		}
		// allowedTools - pass tool list to Claude CLI for agentic workflows
		if allowedTools, ok := options["allowedTools"].(string); ok && allowedTools != "" {
			args = append(args, "--allowedTools", allowedTools)
		}
		// dangerouslySkipPermissions - skip permission prompts for automated execution
		if skipPerms, ok := options["dangerouslySkipPermissions"].(bool); ok && skipPerms {
			args = append(args, "--dangerously-skip-permissions")
			// Audit log for security compliance
			p.logger.Info("[SECURITY AUDIT] dangerouslySkipPermissions enabled",
				"timestamp", time.Now().Format(time.RFC3339),
				"workflow", getWorkflowID(options),
				"step", getStepName(options))
		}
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

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

// cloneState creates a shallow copy of ConversationState.
func cloneState(state *workflow.ConversationState) *workflow.ConversationState {
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

	// Validate max_tokens type and value
	if val, exists := options["max_tokens"]; exists {
		maxTokens, ok := val.(int)
		if !ok {
			return errors.New("max_tokens must be an integer")
		}
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	// Validate temperature type and value
	if val, exists := options["temperature"]; exists {
		temp, ok := val.(float64)
		if !ok {
			return errors.New("temperature must be a number")
		}
		if temp < 0 || temp > 1 {
			return errors.New("temperature must be between 0 and 1")
		}
	}

	// Validate model format (aliases or claude-* models supported by Claude CLI)
	if model, ok := options["model"].(string); ok {
		// Accept known aliases
		aliases := []string{"sonnet", "opus", "haiku"}
		isAlias := false
		for _, alias := range aliases {
			if model == alias {
				isAlias = true
				break
			}
		}
		// Accept models starting with 'claude-'
		if !isAlias && !strings.HasPrefix(model, "claude-") {
			return fmt.Errorf("invalid model format: %s (must be an alias or start with 'claude-')", model)
		}
	}

	return nil
}

// estimateTokens provides an approximate token count estimation based on output length.
// NOTE: This is a rough approximation (~4 characters per token) and may not reflect
// actual token usage. For accurate token counts, parse the usage data from Claude CLI
// JSON output when available (requires --output-format json).
func estimateTokens(output string) int {
	// Rough estimation: ~4 characters per token
	return len(output) / 4
}

// getWorkflowID extracts workflow ID from options for structured logging.
func getWorkflowID(options map[string]any) string {
	if options == nil {
		return "unknown"
	}
	if id, ok := options["workflowID"].(string); ok {
		return id
	}
	return "unknown"
}

// getStepName extracts step name from options for structured logging.
func getStepName(options map[string]any) string {
	if options == nil {
		return "unknown"
	}
	if name, ok := options["stepName"].(string); ok {
		return name
	}
	return "unknown"
}
