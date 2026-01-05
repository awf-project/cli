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
		return nil, err
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

// estimateCodexTokens provides a rough token count estimation.
func estimateCodexTokens(output string) int {
	// Rough estimation: ~4 characters per token
	return len(output) / 4
}
