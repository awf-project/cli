package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/logger"
)

// CopilotProvider implements AgentProvider for GitHub Copilot CLI.
// Invokes: copilot -p "prompt" --output-format=json --silent
type CopilotProvider struct {
	base     *baseCLIProvider
	logger   ports.Logger
	executor ports.CLIExecutor
}

func NewCopilotProvider() *CopilotProvider {
	p := &CopilotProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewCopilotProviderWithOptions(opts ...CopilotProviderOption) *CopilotProvider {
	p := &CopilotProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *CopilotProvider) newBase() *baseCLIProvider {
	return newBaseCLIProvider("github_copilot", "copilot", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildCopilotExecuteArgs,
		buildConversationArgs: p.buildCopilotConversationArgs,
		extractSessionID:      p.extractCopilotSessionID,
		extractTextContent:    p.extractCopilotTextContent,
		validateOptions:       validateCopilotOptions,
		parseDisplayEvents:    p.parseCopilotDisplayEvents,
	})
}

func (p *CopilotProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	if extracted := p.extractCopilotTextContent(rawOutput); extracted != "" {
		result.Output = extracted
		result.Tokens = estimateTokens(extracted)
	}
	return result, nil
}

func (p *CopilotProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, _, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *CopilotProvider) Name() string {
	return "github_copilot"
}

func (p *CopilotProvider) Validate() error {
	_, err := exec.LookPath("copilot")
	if err != nil {
		return fmt.Errorf("copilot CLI not found in PATH: %w", err)
	}
	return nil
}

func (p *CopilotProvider) buildCopilotExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt, "--output-format=json", "--silent"}
	args = appendCopilotOptions(args, options)
	return args, nil
}

func (p *CopilotProvider) buildCopilotConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	var args []string
	if state.SessionID != "" {
		args = []string{"--resume=" + state.SessionID, "-p", prompt, "--output-format=json", "--silent"}
	} else {
		effectivePrompt := buildCopilotFirstTurnPrompt(prompt, options)
		args = []string{"-p", effectivePrompt, "--output-format=json", "--silent"}
	}
	args = appendCopilotOptions(args, options)
	return args, nil
}

// buildCopilotFirstTurnPrompt prepends an optional system prompt.
// Copilot CLI has no --system-prompt flag; the system context must be embedded in the message.
func buildCopilotFirstTurnPrompt(userPrompt string, options map[string]any) string {
	if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
		return sysPrompt + "\n\n" + userPrompt
	}
	return userPrompt
}

// appendCopilotOptions appends Copilot CLI flags from options; unknown keys are silently ignored.
func appendCopilotOptions(args []string, options map[string]any) []string {
	if model, ok := getStringOption(options, "model"); ok && model != "" {
		args = append(args, "--model="+model)
	}
	if mode, ok := getStringOption(options, "mode"); ok && mode != "" {
		args = append(args, "--mode="+mode)
	}
	if effort, ok := getStringOption(options, "effort"); ok && effort != "" {
		args = append(args, "--effort="+effort)
	}
	if tools, ok := options["allowed_tools"]; ok {
		for _, t := range toStringSlice(tools) {
			args = append(args, "--allow-tool="+t)
		}
	}
	if tools, ok := options["denied_tools"]; ok {
		for _, t := range toStringSlice(tools) {
			args = append(args, "--deny-tool="+t)
		}
	}
	if allow, ok := getBoolOption(options, "allow_all"); ok && allow {
		args = append(args, "--allow-all")
	}
	return args
}

func toStringSlice(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func validateCopilotOptions(options map[string]any) error {
	var errs []string

	if mode, ok := getStringOption(options, "mode"); ok && mode != "" {
		switch mode {
		case "interactive", "plan", "autopilot":
		default:
			errs = append(errs, fmt.Sprintf("invalid mode %q: must be one of interactive, plan, autopilot", mode))
		}
	}

	if effort, ok := getStringOption(options, "effort"); ok && effort != "" {
		switch effort {
		case "low", "medium", "high":
		default:
			errs = append(errs, fmt.Sprintf("invalid effort %q: must be one of low, medium, high", effort))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *CopilotProvider) extractCopilotSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := findFirstNDJSONEvent(output, "result")
	if evt == nil {
		return "", errors.New("result event not found")
	}
	// Copilot CLI uses camelCase "sessionId" (not snake_case)
	sessionIDVal, ok := evt["sessionId"]
	if !ok || sessionIDVal == nil {
		return "", errors.New("sessionId missing")
	}
	if str, ok := sessionIDVal.(string); ok && str != "" {
		return str, nil
	}
	return "", errors.New("sessionId is not a non-empty string")
}

func (p *CopilotProvider) parseCopilotDisplayEvents(line []byte) []DisplayEvent {
	var evt struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return nil
	}
	switch evt.Type {
	case "assistant.message_delta":
		if delta, ok := evt.Data["deltaContent"].(string); ok && delta != "" {
			return []DisplayEvent{{Type: evt.Type, Kind: EventText, Text: delta, Delta: true}}
		}
	case "tool.execution_start":
		name, ok := evt.Data["toolName"].(string)
		if !ok {
			name = ""
		}
		return []DisplayEvent{{Type: evt.Type, Kind: EventToolUse, Name: name}}
	}
	return nil
}

// extractCopilotTextContent scans JSONL output for the last assistant.message event
// and returns its data.content field. Falls back to raw output when not found.
func (p *CopilotProvider) extractCopilotTextContent(output string) string {
	var lastContent string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}
		if t, ok := evt["type"].(string); ok && t == "assistant.message" {
			if data, ok := evt["data"].(map[string]any); ok {
				if content, ok := data["content"].(string); ok && content != "" {
					lastContent = content
				}
			}
		}
	}
	if lastContent != "" {
		return lastContent
	}
	return output
}
