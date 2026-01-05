package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// OpenCodeProvider implements AgentProvider for OpenCode CLI.
// Invokes: opencode run "prompt"
type OpenCodeProvider struct{}

// NewOpenCodeProvider creates a new OpenCodeProvider.
func NewOpenCodeProvider() *OpenCodeProvider {
	return &OpenCodeProvider{}
}

// Execute invokes the OpenCode CLI with the given prompt and options.
func (p *OpenCodeProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	// Validate prompt
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	// Validate options
	if err := validateOpenCodeOptions(options); err != nil {
		return nil, err
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Build command arguments
	args := []string{"run", prompt}

	// Apply options
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

	// Execute command
	cmd := exec.CommandContext(ctx, "opencode", args...)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("opencode execution failed: %w", err)
	}

	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:    "opencode",
		Output:      outputStr,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		Tokens:      estimateOpenCodeTokens(outputStr),
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

	// Validate output_dir type
	if outputDir, exists := options["output_dir"]; exists {
		if _, ok := outputDir.(string); !ok {
			return errors.New("output_dir must be a string")
		}
	}

	// Validate verbose type
	if verbose, exists := options["verbose"]; exists {
		if _, ok := verbose.(bool); !ok {
			return errors.New("verbose must be a boolean")
		}
	}

	return nil
}

// estimateOpenCodeTokens provides a rough token count estimation.
func estimateOpenCodeTokens(output string) int {
	// Rough estimation: ~4 characters per token
	return len(output) / 4
}
