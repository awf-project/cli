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

// CursorProvider implements AgentProvider for Cursor CLI.
// Invokes: agent -p --output-format stream-json "prompt"
type CursorProvider struct {
	base     *baseCLIProvider
	logger   ports.Logger
	executor ports.CLIExecutor
}

func NewCursorProvider() *CursorProvider {
	p := &CursorProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	p.base = p.newBase()
	return p
}

func NewCursorProviderWithOptions(opts ...CursorProviderOption) *CursorProvider {
	p := &CursorProvider{
		logger:   logger.NopLogger{},
		executor: NewExecCLIExecutor(),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.base = p.newBase()
	return p
}

func (p *CursorProvider) newBase() *baseCLIProvider {
	return newBaseCLIProvider("cursor", "agent", p.executor, p.logger, cliProviderHooks{
		buildExecuteArgs:      p.buildExecuteArgs,
		buildConversationArgs: p.buildConversationArgs,
		extractSessionID:      p.extractSessionID,
		extractTextContent:    p.extractTextFromJSON,
		validateOptions:       validateCursorOptions,
		parseStreamLine:       p.parseCursorStreamLine,
	})
}

func (p *CursorProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.AgentResult, error) {
	result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	userFormat, _ := getStringOption(options, "output_format")
	if userFormat == "json" || userFormat == "stream-json" {
		if jsonResp := p.extractResultEvent(rawOutput); jsonResp != nil {
			result.Response = jsonResp
		}
	} else {
		if extracted := p.extractTextFromJSON(rawOutput); extracted != "" {
			result.Output = extracted
			result.Tokens = estimateTokens(extracted)
		}
	}

	return result, nil
}

func (p *CursorProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, stderr io.Writer) (*workflow.ConversationResult, error) {
	result, rawOutput, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
	if err != nil {
		return nil, err
	}

	// Keep behavior aligned with Claude provider: only expose raw final result
	// wrapper when explicitly requested.
	userFormat, userFormatSet := getStringOption(options, "output_format")
	if userFormatSet && userFormat == "json" {
		if jsonResp := p.extractResultEvent(rawOutput); jsonResp != nil {
			result.Response = jsonResp
		}
	}

	return result, nil
}

func (p *CursorProvider) Name() string {
	return "cursor"
}

func (p *CursorProvider) Validate() error {
	_, err := exec.LookPath("agent")
	if err != nil {
		return fmt.Errorf("cursor CLI not found in PATH (expected binary 'agent'): %w", err)
	}
	return nil
}

func (p *CursorProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
	args := []string{"-p", prompt, "--output-format", "stream-json"}
	return appendCursorOptions(args, options), nil
}

func (p *CursorProvider) buildConversationArgs(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error) {
	effectivePrompt := prompt
	if state != nil && state.SessionID == "" {
		// Cursor CLI has no dedicated system prompt flag. Inline it only for first turn.
		if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
			effectivePrompt = sysPrompt + "\n\n" + prompt
		}
	}

	args := []string{"-p", effectivePrompt, "--output-format", "stream-json"}
	if state != nil && state.SessionID != "" {
		args = append(args, "--resume", state.SessionID)
	}
	return appendCursorOptions(args, options), nil
}

func appendCursorOptions(args []string, options map[string]any) []string {
	if model, ok := getStringOption(options, "model"); ok && model != "" {
		args = append(args, "--model", model)
	}
	if mode, ok := getStringOption(options, "mode"); ok && mode != "" {
		args = append(args, "--mode", mode)
	}
	if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
		args = append(args, "--force")
	}
	if sandbox, ok := getStringOption(options, "sandbox"); ok && sandbox != "" {
		args = append(args, "--sandbox", sandbox)
	}
	return args
}

func validateCursorOptions(options map[string]any) error {
	if options == nil {
		return nil
	}

	if mode, ok := getStringOption(options, "mode"); ok {
		if mode != "plan" && mode != "ask" {
			return fmt.Errorf("invalid mode: %s (must be 'plan' or 'ask')", mode)
		}
	}

	if sandbox, ok := getStringOption(options, "sandbox"); ok {
		if sandbox != "enabled" && sandbox != "disabled" {
			return fmt.Errorf("invalid sandbox: %s (must be 'enabled' or 'disabled')", sandbox)
		}
	}

	return nil
}

func (p *CursorProvider) extractResultEvent(output string) map[string]any {
	return findFirstNDJSONEvent(output, "result")
}

func (p *CursorProvider) extractInitEvent(output string) map[string]any {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		eventType, _ := evt["type"].(string)
		subtype, _ := evt["subtype"].(string)
		if eventType == "system" && subtype == "init" {
			return evt
		}
	}
	return nil
}

func extractStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func (p *CursorProvider) extractSessionID(output string) (string, error) {
	if output == "" {
		return "", errors.New("empty output")
	}
	evt := p.extractInitEvent(output)
	if evt == nil {
		return "", errors.New("system init event not found")
	}

	sessionID := extractStringValue(evt,
		"chat_id", "chatId",
		"session_id", "sessionId",
		"conversation_id", "conversationId",
		"thread_id", "threadId",
		"id",
	)
	if sessionID == "" {
		return "", errors.New("session identifier missing")
	}
	return sessionID, nil
}

func (p *CursorProvider) extractTextFromJSON(output string) string {
	evt := p.extractResultEvent(output)
	if evt == nil {
		return ""
	}

	if result, ok := evt["result"].(string); ok && result != "" {
		return result
	}
	if message, ok := evt["message"].(string); ok && message != "" {
		return message
	}

	return extractDisplayText(output, p.parseCursorStreamLine)
}

// parseCursorStreamLine extracts displayable text from Cursor CLI stream-json
// lines. It surfaces assistant message text blocks and ignores tool/system events.
func (p *CursorProvider) parseCursorStreamLine(line []byte) string {
	var evt struct {
		Type    string `json:"type"`
		Message *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return ""
	}
	if evt.Type != "assistant" || evt.Message == nil {
		return ""
	}

	var out strings.Builder
	for _, block := range evt.Message.Content {
		if block.Type == "text" && block.Text != "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(block.Text)
		}
	}
	return out.String()
}
