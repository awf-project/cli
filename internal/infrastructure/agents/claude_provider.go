package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// ClaudeProvider implements AgentProvider for Claude CLI.
// Invokes: claude -p "prompt" --output-format json
type ClaudeProvider struct{}

// NewClaudeProvider creates a new ClaudeProvider.
func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
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
		return nil, err
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
			log.Printf("[SECURITY AUDIT] dangerouslySkipPermissions=true enabled at %s", time.Now().Format(time.RFC3339))
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
	if maxTokens, ok := options["max_tokens"].(int); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	// Validate temperature
	if temp, ok := options["temperature"].(float64); ok {
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
