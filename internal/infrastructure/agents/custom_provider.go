package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// CustomProvider implements AgentProvider for user-defined command templates.
// Allows users to define their own agent invocation commands.
type CustomProvider struct {
	name            string
	commandTemplate string
}

// NewCustomProvider creates a new CustomProvider with the given name and command template.
func NewCustomProvider(name string, commandTemplate string) *CustomProvider {
	return &CustomProvider{
		name:            name,
		commandTemplate: commandTemplate,
	}
}

// Execute invokes the custom command with the given prompt and options.
func (p *CustomProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	// Validate prompt
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	// Validate command template
	if strings.TrimSpace(p.commandTemplate) == "" {
		return nil, errors.New("command template cannot be empty")
	}

	// Check context before execution
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Parse and execute template
	tmpl, err := template.New("command").
		Funcs(template.FuncMap{
			"escape": interpolation.ShellEscape,
			"raw":    interpolation.NoEscape,
		}).
		Option("missingkey=error").
		Parse(p.commandTemplate)
	if err != nil {
		return nil, fmt.Errorf("template parsing failed: %w", err)
	}

	// Prepare template data
	// By default, prompt is shell-escaped for safety.
	// Use {{.prompt}} in templates (automatically escaped).
	// Use {{raw .prompt}} only if you need the raw, unescaped value.
	// Example safe template: echo {{.prompt}} | agent-command
	// Example opt-out: echo {{raw .prompt}} | agent-command
	data := map[string]any{
		"prompt":  interpolation.ShellEscape(prompt),
		"options": options,
	}

	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, fmt.Errorf("template variable error: %w", err)
	}

	command := buf.String()

	// Execute command
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	output, err := cmd.CombinedOutput()
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("command execution failed: %w", err)
	}

	outputStr := string(output)
	result := &workflow.AgentResult{
		Provider:        p.name,
		Output:          outputStr,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		Tokens:          estimateCustomTokens(outputStr),
		TokensEstimated: true, // using rough estimation, not actual API usage
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

// ExecuteConversation invokes the custom command with conversation history for multi-turn interactions.
func (p *CustomProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	return nil, errors.New("not implemented")
}

// Name returns the provider identifier.
func (p *CustomProvider) Name() string {
	return p.name
}

// Validate checks if the custom command is properly configured.
func (p *CustomProvider) Validate() error {
	if strings.TrimSpace(p.commandTemplate) == "" {
		return errors.New("command template cannot be empty")
	}

	// Try to parse the template to check syntax
	_, err := template.New("command").Parse(p.commandTemplate)
	if err != nil {
		return fmt.Errorf("command template is invalid: %w", err)
	}

	return nil
}

// estimateCustomTokens provides a rough token count estimation.
func estimateCustomTokens(output string) int {
	// Rough estimation: ~4 characters per token
	return len(output) / 4
}
