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

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// GeminiProvider implements AgentProvider for Gemini CLI.
// Invokes: gemini -p "prompt"
type GeminiProvider struct {
	executor ports.CLIExecutor
}

func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{
		executor: NewExecCLIExecutor(),
	}
}

func NewGeminiProviderWithOptions(opts ...GeminiProviderOption) *GeminiProvider {
	p := &GeminiProvider{
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *GeminiProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateGeminiOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

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

	stdout, stderr, err := p.executor.Run(ctx, "gemini", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("gemini execution failed: %w", err)
	}

	// Combine stdout and stderr like CombinedOutput()
	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
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

func (p *GeminiProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateGeminiOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

	workingState := cloneState(state)

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	var args []string
	if workingState.SessionID != "" {
		args = []string{"--resume", workingState.SessionID, prompt}
	} else {
		args = []string{prompt}
		// First turn only: pass system prompt if provided
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			args = append([]string{"--system-prompt", sysPrompt}, args...)
		}
	}

	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok {
		args = append([]string{"--output-format", outputFormat}, args...)
	}

	stdout, stderr, err := p.executor.Run(ctx, "gemini", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("gemini execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := string(output)
	if outputStr == "" {
		outputStr = " "
	}

	// Extract session ID for future resume turns; continue stateless on failure.
	if sessionID, extractErr := extractSessionIDFromLines(outputStr); extractErr == nil {
		workingState.SessionID = sessionID
	} else {
		workingState.SessionID = ""
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	inputTokens := estimateInputTokens(workingState.Turns, 1)

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

// extractSessionID parses a session identifier from Gemini CLI output.
// Looks for a "Session: <id>" line and returns the trimmed ID.
// Returns empty string and error if not found (caller falls back to stateless).
func (p *GeminiProvider) Name() string {
	return "gemini"
}

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

	if temp, ok := getFloatOption(options, "temperature"); ok {
		if temp < 0 || temp > 1 {
			return errors.New("temperature must be between 0 and 1")
		}
	}

	if model, ok := getStringOption(options, "model"); ok {
		validModels := []string{"gemini-pro", "gemini-pro-vision", "gemini-ultra"}
		if !slices.Contains(validModels, model) {
			return fmt.Errorf("unknown model: %s", model)
		}
	}

	return nil
}
