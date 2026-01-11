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

	"github.com/vanoix/awf/internal/domain/workflow"
)

// GeminiProvider implements AgentProvider for Gemini CLI.
// Invokes: gemini -p "prompt"
type GeminiProvider struct{}

// NewGeminiProvider creates a new GeminiProvider.
func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{}
}

// Execute invokes the Gemini CLI with the given prompt and options.
func (p *GeminiProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	// Validate prompt
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	// Validate options
	if err := validateGeminiOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

	// Build command arguments
	// Note: -p/--prompt is deprecated, use positional argument instead
	args := []string{prompt}

	// Apply options (only those supported by Gemini CLI)
	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok {
		args = append([]string{"--output-format", outputFormat}, args...)
	}
	// Note: temperature and safety_settings are validated but not passed to CLI

	// Execute command
	cmd := exec.CommandContext(ctx, "gemini", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("gemini execution failed: %w", err)
	}

	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:    "gemini",
		Output:      outputStr,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Tokens:      estimateTokens(outputStr),
	}

	// Try to parse JSON response if output looks like JSON
	if jsonResp := tryParseJSONResponse(outputStr); jsonResp != nil {
		result.Response = jsonResp
	}

	return result, nil
}

// ExecuteConversation invokes the Gemini CLI with conversation history for multi-turn interactions.
func (p *GeminiProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
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
	if err := validateGeminiOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

	// Clone state to avoid modifying original
	workingState := cloneState(state)

	// Add user turn to conversation history
	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	// Build command arguments
	args := []string{prompt}

	// Apply options (only those supported by Gemini CLI)
	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok {
		args = append([]string{"--output-format", outputFormat}, args...)
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "gemini", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("gemini execution failed: %w", err)
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
		Provider:        "gemini",
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
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Validate checks if the Gemini CLI is installed and accessible.
func (p *GeminiProvider) Validate() error {
	_, err := exec.LookPath("gemini")
	if err != nil {
		return fmt.Errorf("gemini CLI not found in PATH: %w", err)
	}
	return nil
}

// validateGeminiOptions validates provider-specific options.
func validateGeminiOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	// Validate temperature
	if temp, ok := getFloatOption(options, "temperature"); ok {
		if temp < 0 || temp > 1 {
			return errors.New("temperature must be between 0 and 1")
		}
	}

	// Validate model
	if model, ok := getStringOption(options, "model"); ok {
		validModels := []string{"gemini-pro", "gemini-pro-vision", "gemini-ultra"}
		if !slices.Contains(validModels, model) {
			return fmt.Errorf("unknown model: %s", model)
		}
	}

	return nil
}
