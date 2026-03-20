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
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// ClaudeProvider implements AgentProvider for Claude CLI.
// Invokes: claude -p "prompt" --output-format json
type ClaudeProvider struct {
	logger   ports.Logger
	executor ports.CLIExecutor
}

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

func (p *ClaudeProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("claude provider: %w", err)
	}

	args := []string{"-p", prompt}

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

	stdout, stderr, err := p.executor.Run(ctx, "claude", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

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
		TokensEstimated: true,
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

// ExecuteConversation invokes the Claude CLI with conversation history for multi-turn interactions.
//
//nolint:gocognit // Complexity 31: conversation executor manages multi-turn state, context windows, token limits, retries, streaming. Conversation orchestration requires this.
func (p *ClaudeProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := validateOptions(options); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("claude provider: %w", err)
	}

	workingState := cloneState(state)

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	args := []string{"-p", prompt}

	if model, ok := getStringOption(options, "model"); ok {
		args = append(args, "--model", model)
	}

	// Force JSON output for session ID extraction on all conversation turns
	args = append(args, "--output-format", "json")

	// Resume from previous session if available
	if workingState.SessionID != "" {
		args = append(args, "-r", workingState.SessionID)
	} else {
		// First turn only: pass system prompt if provided
		// Note: system_prompt is always present in options (set by ConversationManager)
		// but only consumed here on turn 1 when SessionID is empty.
		// On turns 2+, the provider retains the system prompt from the resumed session.
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			args = append(args, "--system-prompt", sysPrompt)
		}
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

	stdout, stderr, err := p.executor.Run(ctx, "claude", args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("claude execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdout)+len(stderr))
	output = append(output, stdout...)
	output = append(output, stderr...)

	// Extract text content from JSON wrapper
	// Claude's --output-format json wraps response in JSON: {"session_id":"...", "result":"actual text", ...}
	// We need the clean text for the assistant turn, not the raw JSON.
	// On extraction failure, gracefully fall back to raw output string.
	rawOutputStr := string(output)
	outputStr := p.extractTextFromJSON(rawOutputStr)
	if outputStr == "" {
		// Extraction failed (either non-JSON or missing result field) — use raw output
		outputStr = strings.TrimSpace(rawOutputStr)
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, outputStr)
	assistantTurn.Tokens = estimateTokens(outputStr)
	if err := workingState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("failed to add assistant turn: %w", err)
	}

	// Extract session ID for future turns (uses raw output, not extracted text)
	if sessionID, err := p.extractSessionID(rawOutputStr); err != nil {
		p.logger.Debug("session ID extraction failed, continuing stateless", "error", err)
	} else if sessionID != "" {
		workingState.SessionID = sessionID
	}

	inputTokens := estimateInputTokens(workingState.Turns, 1)

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

	// Populate result.Response only when user explicitly requested output_format: json.
	// When --output-format json is forced internally for session resume, the full JSON wrapper
	// (containing session_id, cost_usd, etc.) must NOT leak into workflow state.
	userRequestedJSON := false
	if userFormat, ok := getStringOption(options, "output_format"); ok && userFormat == "json" {
		userRequestedJSON = true
	}

	if userRequestedJSON {
		var jsonResp map[string]any
		if err := json.Unmarshal(output, &jsonResp); err != nil {
			return nil, fmt.Errorf("failed to parse JSON output: %w", err)
		}
		result.Response = jsonResp
	}

	return result, nil
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Validate() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	return nil
}

func validateOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if maxTokens, ok := getIntOption(options, "max_tokens"); ok {
		if maxTokens < 0 {
			return errors.New("max_tokens must be non-negative")
		}
	}

	if temp, ok := getFloatOption(options, "temperature"); ok {
		if temp < 0 || temp > 1 {
			return errors.New("temperature must be between 0 and 1")
		}
	}

	if model, ok := getStringOption(options, "model"); ok {
		if !isValidClaudeModel(model) {
			return fmt.Errorf("invalid model format: %s (must be an alias or start with 'claude-')", model)
		}
	}

	return nil
}

func isValidClaudeModel(model string) bool {
	aliases := []string{"sonnet", "opus", "haiku"}
	return slices.Contains(aliases, model) || strings.HasPrefix(model, "claude-")
}

func (p *ClaudeProvider) extractSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return "", fmt.Errorf("output is not valid JSON: %w", err)
	}

	sessionIDVal, ok := data["session_id"]
	if !ok {
		return "", errors.New("session_id field not found")
	}

	// Handle null value
	if sessionIDVal == nil {
		return "", errors.New("session_id is null")
	}

	// Extract string value
	if str, ok := sessionIDVal.(string); ok {
		return str, nil
	}

	return "", errors.New("session_id is not a string")
}

func (p *ClaudeProvider) extractTextFromJSON(output string) string {
	if output == "" {
		return ""
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return ""
	}

	result, ok := data["result"]
	if !ok {
		return ""
	}

	// Handle null value
	if result == nil {
		return ""
	}

	// Handle string values
	if str, ok := result.(string); ok {
		return str
	}

	// Handle numeric values - convert to string
	if num, ok := result.(float64); ok {
		// Check if it's an integer
		if num == float64(int64(num)) {
			return fmt.Sprintf("%.0f", num)
		}
		return fmt.Sprint(num)
	}

	// Unexpected types (boolean, array, object, etc.) - return empty string
	return ""
}
