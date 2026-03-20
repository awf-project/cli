package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// OpenCodeProvider implements AgentProvider for OpenCode CLI.
// Invokes: opencode run "prompt"
type OpenCodeProvider struct {
	executor ports.CLIExecutor
}

// NewOpenCodeProvider creates a new OpenCodeProvider.
// If no executor is provided, ExecCLIExecutor is used by default.
func NewOpenCodeProvider() *OpenCodeProvider {
	return &OpenCodeProvider{
		executor: NewExecCLIExecutor(),
	}
}

// NewOpenCodeProviderWithOptions creates a new OpenCodeProvider with functional options.
func NewOpenCodeProviderWithOptions(opts ...OpenCodeProviderOption) *OpenCodeProvider {
	p := &OpenCodeProvider{
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Execute invokes the OpenCode CLI with the given prompt and options.
func (p *OpenCodeProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateOpenCodeOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opencode provider: %w", err)
	}

	args := []string{"run", prompt}

	if options != nil {
		if framework, ok := options["framework"].(string); ok {
			args = append(args, "--framework", framework)
		}
		if verbose, ok := options["verbose"].(bool); ok && verbose {
			args = append(args, "--verbose")
		}
		if outputDir, ok := options["output_dir"].(string); ok {
			args = append(args, "--output", outputDir)
		}
	}

	stdout, stderr, err := p.executor.Run(ctx, "opencode", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("opencode execution failed: %w", err)
	}

	// Combine stdout and stderr like CombinedOutput()
	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:    "opencode",
		Output:      outputStr,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Tokens:      estimateTokens(outputStr),
	}

	// Try to parse JSON response if output looks like JSON
	trimmedOutput := strings.TrimSpace(outputStr)
	if strings.HasPrefix(trimmedOutput, "{") && strings.HasSuffix(trimmedOutput, "}") {
		var jsonResp map[string]any
		if err := json.Unmarshal([]byte(trimmedOutput), &jsonResp); err == nil {
			result.Response = jsonResp
		}
	}

	return result, nil
}

// ExecuteConversation invokes the OpenCode CLI with conversation history for multi-turn interactions.
func (p *OpenCodeProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateOpenCodeOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("opencode provider: %w", err)
	}

	workingState := cloneState(state)

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	args := []string{"run", prompt}

	// Resume from previous session if available
	if workingState.SessionID != "" {
		args = append(args, "-s", workingState.SessionID)
	} else {
		// First turn only: pass system prompt if provided
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			args = append(args, "--system-prompt", sysPrompt)
		}
	}

	if options != nil {
		if framework, ok := options["framework"].(string); ok {
			args = append(args, "--framework", framework)
		}
		if verbose, ok := options["verbose"].(bool); ok && verbose {
			args = append(args, "--verbose")
		}
		if outputDir, ok := options["output_dir"].(string); ok {
			args = append(args, "--output", outputDir)
		}
	}

	stdout, stderr, err := p.executor.Run(ctx, "opencode", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("opencode execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)
	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		outputStr = " "
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	// Extract session ID for future turns - gracefully fall back to empty SessionID on error
	if sessionID, err := extractSessionIDFromLines(outputStr); err == nil && sessionID != "" {
		workingState.SessionID = sessionID
	} else {
		// If extraction fails, clear SessionID for stateless fallback
		workingState.SessionID = ""
	}

	inputTokens := estimateInputTokens(workingState.Turns, 1)

	result := &workflow.ConversationResult{
		Provider:        "opencode",
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
func (p *OpenCodeProvider) Name() string {
	return "opencode"
}

// Validate checks if the OpenCode CLI is installed and accessible.
func (p *OpenCodeProvider) Validate() error {
	_, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode CLI not found in PATH: %w", err)
	}
	return nil
}

// validateOpenCodeOptions validates provider-specific options.
func validateOpenCodeOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if outputDir, exists := options["output_dir"]; exists {
		if _, ok := outputDir.(string); !ok {
			return errors.New("output_dir must be a string")
		}
	}

	if verbose, exists := options["verbose"]; exists {
		if _, ok := verbose.(bool); !ok {
			return errors.New("verbose must be a boolean")
		}
	}

	return nil
}
