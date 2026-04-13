package agents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
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

func (p *GeminiProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	startedAt := time.Now()

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

	args := []string{"-p", prompt}

	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	if outputFormat, ok := getStringOption(options, "output_format"); ok {
		if outputFormat == "json" {
			outputFormat = "stream-json"
		}
		args = append([]string{"--output-format", outputFormat}, args...)
	}
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append([]string{"--approval-mode=yolo"}, args...)
	}

	stdoutBytes, stderrBytes, err := p.executor.Run(ctx, "gemini", stdout, stderr, args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("gemini execution failed: %w", err)
	}

	// Combine stdout and stderr like CombinedOutput()
	output := make([]byte, 0, len(stdoutBytes)+len(stderrBytes))
	output = append(output, stdoutBytes...)
	output = append(output, stderrBytes...)
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

func (p *GeminiProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	startedAt := time.Now()

	if state == nil {
		return nil, errors.New("conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("gemini provider: %w", err)
	}

	workingState := cloneState(state)

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	if err := workingState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("failed to add user turn: %w", err)
	}

	// Gemini CLI has no --system-prompt flag; inline the system prompt into
	// the first turn's message so it reaches the model. Subsequent turns
	// rely on --resume and must not re-send the system prompt.
	effectivePrompt := prompt
	var args []string
	if workingState.SessionID != "" {
		args = []string{"--resume", workingState.SessionID, "-p", effectivePrompt}
	} else {
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			effectivePrompt = sysPrompt + "\n\n" + prompt
		}
		args = []string{"-p", effectivePrompt}
	}

	if model, ok := getStringOption(options, "model"); ok {
		args = append([]string{"--model", model}, args...)
	}
	// Force stream-json unconditionally for reliable session ID extraction.
	args = append([]string{"--output-format", "stream-json"}, args...)
	if skipPerms, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skipPerms {
		args = append([]string{"--approval-mode=yolo"}, args...)
	}

	stdoutBytes, stderrBytes, err := p.executor.Run(ctx, "gemini", stdout, stderr, args...)
	completedAt := time.Now()

	if err != nil {
		return nil, fmt.Errorf("gemini execution failed: %w", err)
	}

	output := make([]byte, 0, len(stdoutBytes)+len(stderrBytes))
	output = append(output, stdoutBytes...)
	output = append(output, stderrBytes...)
	outputStr := string(output)
	if outputStr == "" {
		outputStr = " "
	}

	// Extract session ID for future resume turns; continue stateless on failure.
	if sessionID, err := p.extractSessionID(outputStr); err == nil {
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

	return result, nil
}

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

func (p *GeminiProvider) extractInitEvent(output string) map[string]any {
	return findFirstNDJSONEvent(output, "init")
}

func (p *GeminiProvider) extractSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := p.extractInitEvent(output)
	if evt == nil {
		return "", errors.New("init event not found")
	}
	sessionIDVal, ok := evt["session_id"]
	if !ok || sessionIDVal == nil {
		return "", errors.New("session_id missing")
	}
	str, ok := sessionIDVal.(string)
	if !ok {
		return "", errors.New("session_id is not a string")
	}
	if str == "" {
		return "", errors.New("session_id is empty")
	}
	return str, nil
}
