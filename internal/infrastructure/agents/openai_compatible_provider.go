package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/httpx"
)

var _ ports.AgentProvider = (*OpenAICompatibleProvider)(nil)

// OpenAICompatibleProvider implements AgentProvider via the Chat Completions HTTP API.
// Compatible with OpenAI, Ollama, vLLM, Groq, and any OpenAI-compatible backend.
//
// MCP proxy integration — divergence from CLI providers:
//
// The CLI providers (Claude, Codex, Gemini, Opencode) wire MCP through a
// `mcpInjector` hook on baseCLIProvider, which appends provider-specific
// flags to the subprocess invocation (e.g. `--mcp-config <path>`). That path
// does not apply here: this provider speaks the Chat Completions HTTP API
// directly and has no child process to inject flags into.
//
// Instead, MCP integration is HTTP-native and lives entirely in this file:
//
//   - SetToolRouter installs an application/tools.Router implementation;
//   - buildToolList reads the MCPProxyConfig from options and advertises
//     tools (respecting cfg.InterceptBuiltins) in the `tools` request field;
//   - dispatchToolCall routes the model's tool_calls back through the
//     Router and feeds tool results into the next turn.
//
// This is the documented HTTP-native MCP path; the absence of an
// mcpInjector here is intentional, not a missing implementation.
type OpenAICompatibleProvider struct {
	httpClient *httpx.Client
	toolRouter ports.ToolRouter
}

// maxResponseBodyBytes limits response reading to prevent memory exhaustion.
const maxResponseBodyBytes = 10 * 1024 * 1024 // 10MB

// chatToolCall represents a tool call in an assistant message.
type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
}

type chatCompletionsRequest struct {
	Model               string           `json:"model"`
	Messages            []chatMessage    `json:"messages"`
	Temperature         *float64         `json:"temperature,omitempty"`
	MaxCompletionTokens *int             `json:"max_completion_tokens,omitempty"`
	TopP                *float64         `json:"top_p,omitempty"`
	Tools               []ToolDefinition `json:"tools,omitempty"`
	ToolChoice          string           `json:"tool_choice,omitempty"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Index        int         `json:"index"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatCompletionsResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type parsedOptions struct {
	baseURL             string
	model               string
	apiKey              string
	systemPrompt        string
	temperature         *float64
	maxCompletionTokens *int
	topP                *float64
}

func NewOpenAICompatibleProvider(opts ...OpenAICompatibleProviderOption) *OpenAICompatibleProvider {
	p := &OpenAICompatibleProvider{
		httpClient: httpx.NewClient(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *OpenAICompatibleProvider) SetToolRouter(r ports.ToolRouter) {
	p.toolRouter = r
}

func (p *OpenAICompatibleProvider) Name() string {
	return "openai_compatible"
}

func (p *OpenAICompatibleProvider) Validate() error {
	return nil
}

// maxToolCallIterations is the hard cap on multi-turn tool-call loops.
// Prevents runaway loops even when the model continually returns valid tool_calls.
const maxToolCallIterations = 25

func (p *OpenAICompatibleProvider) Execute(ctx context.Context, prompt string, options map[string]any, stdout, _ io.Writer) (*workflow.AgentResult, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	opts, err := p.parseAndValidateOptions(options)
	if err != nil {
		return nil, err
	}

	messages := []chatMessage{
		{Role: "user", Content: prompt},
	}

	// When a system prompt is configured, prepend it.
	if opts.systemPrompt != "" {
		messages = append([]chatMessage{{Role: "system", Content: opts.systemPrompt}}, messages...)
	}

	result := workflow.NewAgentResult("openai_compatible")

	// Resolve MCP proxy config from options if present; nil cfg is safe (buildToolList skips proxy tools).
	cfg, _ := options[workflow.MCPProxyConfigKey].(*workflow.MCPProxyConfig) //nolint:errcheck // comma-ok type assertion; false ok means key absent or wrong type, cfg=nil is the correct fallback

	// Build tool list when MCP proxy is enabled.
	tools, toolChoice, toolErr := p.buildToolList(ctx, cfg)
	if toolErr != nil {
		return nil, toolErr
	}

	loopResult, loopErr := p.runToolCallLoop(ctx, &opts, messages, tools, toolChoice, stdout)
	if loopErr != nil {
		return nil, loopErr
	}

	result.Output = loopResult.output
	result.Tokens = loopResult.totalTokens
	result.TokensEstimated = false
	result.CompletedAt = time.Now()

	if outputFormat, ok := options["output_format"]; ok && outputFormat == "json" {
		parsed, parseErr := p.parseJSONResponse(loopResult.output)
		if parseErr != nil {
			return nil, parseErr
		}
		result.Response = parsed
	}
	return result, nil
}

// toolCallLoopResult holds the outcome of a runToolCallLoop execution.
type toolCallLoopResult struct {
	output       string
	totalTokens  int
	inputTokens  int // prompt tokens from the final (stop) response
	outputTokens int // completion tokens from the final (stop) response
}

// runToolCallLoop executes the multi-turn POST → tool_calls → POST loop until
// finish_reason is "stop" (or equivalent), or the hard cap of maxToolCallIterations
// is reached. It returns the final assistant text output and accumulated token counts.
//
// Both Execute and ExecuteConversation delegate their tool-call handling here so the
// loop semantics are always identical regardless of entry point.
func (p *OpenAICompatibleProvider) runToolCallLoop(
	ctx context.Context,
	opts *parsedOptions,
	messages []chatMessage,
	tools []ToolDefinition,
	toolChoice string,
	stdout io.Writer,
) (toolCallLoopResult, error) {
	var res toolCallLoopResult

	// Multi-turn loop: POST → handle tool_calls → POST again, up to maxToolCallIterations.
	for iter := range maxToolCallIterations {
		resp, callErr := p.callChatCompletionsWithTools(ctx, opts, messages, tools, toolChoice)
		if callErr != nil {
			return res, callErr
		}

		if len(resp.Choices) == 0 {
			return res, fmt.Errorf("openai_compatible: API returned no choices")
		}

		choice := resp.Choices[0]
		res.totalTokens += resp.Usage.TotalTokens

		switch choice.FinishReason {
		case "stop", "":
			// Normal completion — return the assistant content.
			res.output = choice.Message.Content
			res.inputTokens = resp.Usage.PromptTokens
			res.outputTokens = resp.Usage.CompletionTokens
			p.writeDisplayOutput(stdout, res.output)
			return res, nil

		case "tool_calls":
			// Infinite-loop guard: finish_reason is tool_calls but no tool calls emitted.
			if len(choice.Message.ToolCalls) == 0 {
				return res, domerrors.NewUserError(
					domerrors.ErrorCodeUserMCPProxyInfiniteLoopGuard,
					"openai_compatible: finish_reason=tool_calls but no tool_calls in response (infinite loop guard)",
					map[string]any{"iteration": iter},
					nil,
				)
			}

			// Append the assistant turn (with tool_calls) to the message history.
			messages = append(messages, choice.Message)

			// Dispatch each tool call and append the tool result message.
			for _, tc := range choice.Message.ToolCalls {
				toolResult, callToolErr := p.dispatchToolCall(ctx, tc)
				messages = append(messages, chatMessage{
					Role:       "tool",
					Content:    toolResult,
					ToolCallID: tc.ID,
				})
				if callToolErr != nil {
					// Log but continue — tool error is conveyed via content.
					_ = callToolErr //nolint:errcheck // tool error is surfaced to the model via the tool result message
				}
			}
			// Loop: POST with updated history.

		case "length":
			// Context-length truncation — return what we have with a note.
			res.output = choice.Message.Content
			res.inputTokens = resp.Usage.PromptTokens
			res.outputTokens = resp.Usage.CompletionTokens
			p.writeDisplayOutput(stdout, res.output)
			return res, nil

		default:
			// Unknown finish_reason — treat as completion.
			res.output = choice.Message.Content
			res.inputTokens = resp.Usage.PromptTokens
			res.outputTokens = resp.Usage.CompletionTokens
			p.writeDisplayOutput(stdout, res.output)
			return res, nil
		}
	}

	// Hard cap reached — 25 iterations with tool_calls each time.
	return res, domerrors.NewUserError(
		domerrors.ErrorCodeUserMCPProxyInfiniteLoopGuard,
		fmt.Sprintf("openai_compatible: tool-call loop exceeded %d iterations (hard cap)", maxToolCallIterations),
		map[string]any{"iterations": maxToolCallIterations},
		nil,
	)
}

// buildToolList constructs the Tools slice and ToolChoice value for a chat completions request
// based on the active MCPProxyConfig. Returns empty slice and empty choice when proxy is disabled.
func (p *OpenAICompatibleProvider) buildToolList(ctx context.Context, cfg *workflow.MCPProxyConfig) ([]ToolDefinition, string, error) {
	if cfg == nil || !cfg.Enable || p.toolRouter == nil {
		return nil, "", nil
	}

	portTools, err := p.toolRouter.ListTools(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("openai_compatible: list tools: %w", err)
	}

	var tools []ToolDefinition
	for _, t := range portTools {
		// When intercept_builtins=false, only expose plugin-sourced tools (source != "builtin").
		if !cfg.InterceptBuiltins && t.Source == "builtin" {
			continue
		}
		td := ToolDefinition{
			Type: "function",
			Function: toolFunctionSchema{
				Name:        t.Name,
				Description: t.Description,
			},
		}
		if t.InputSchema != nil {
			td.Function.Parameters = t.InputSchema
		}
		tools = append(tools, td)
	}

	if len(tools) == 0 {
		return nil, "", nil
	}
	return tools, "auto", nil
}

// dispatchToolCall invokes the ToolRouter for a single tool call and returns the result content.
// On error, returns an error message string so the model can see the failure.
// Tool names and sources are logged; arguments are not (may contain secrets per NFR-002).
func (p *OpenAICompatibleProvider) dispatchToolCall(ctx context.Context, tc chatToolCall) (string, error) {
	if p.toolRouter == nil {
		return "error: no tool router configured", fmt.Errorf("no tool router")
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("error: invalid tool arguments for %s", tc.Function.Name), err
	}

	result, err := p.toolRouter.CallTool(ctx, tc.Function.Name, args)
	if err != nil {
		return fmt.Sprintf("error calling tool %s: %s", tc.Function.Name, err.Error()), err
	}

	// Assemble tool result content.
	var parts []string
	for _, c := range result.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	if result.IsError {
		return "error: " + strings.Join(parts, "\n"), nil
	}
	return strings.Join(parts, "\n"), nil
}

// callChatCompletionsWithTools posts a chat completions request with optional tools.
func (p *OpenAICompatibleProvider) callChatCompletionsWithTools(ctx context.Context, opts *parsedOptions, messages []chatMessage, tools []ToolDefinition, toolChoice string) (*chatCompletionsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	endpoint := opts.baseURL + "/chat/completions"

	reqBody := chatCompletionsRequest{
		Model:               opts.model,
		Messages:            messages,
		Temperature:         opts.temperature,
		MaxCompletionTokens: opts.maxCompletionTokens,
		TopP:                opts.topP,
		Tools:               tools,
		ToolChoice:          toolChoice,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai_compatible: failed to serialize request: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if opts.apiKey != "" {
		// API key sent as Bearer token; never included in error messages (NFR-002).
		headers["Authorization"] = "Bearer " + opts.apiKey
	}

	httpResp, err := p.httpClient.Post(ctx, endpoint, headers, string(bodyBytes), maxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	if err := mapHTTPError(httpResp); err != nil {
		return nil, err
	}

	var resp chatCompletionsResponse
	if err := json.Unmarshal([]byte(httpResp.Body), &resp); err != nil {
		return nil, fmt.Errorf("openai_compatible: failed to parse response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai_compatible: API returned no choices")
	}

	return &resp, nil
}

func (p *OpenAICompatibleProvider) parseJSONResponse(output string) (map[string]any, error) {
	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("openai_compatible: response is empty, cannot parse as json")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("openai_compatible: failed to parse response as json: %w", err)
	}

	return parsed, nil
}

func (p *OpenAICompatibleProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any, stdout, _ io.Writer) (*workflow.ConversationResult, error) {
	if state == nil {
		return nil, fmt.Errorf("openai_compatible: conversation state cannot be nil")
	}

	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	opts, err := p.parseAndValidateOptions(options)
	if err != nil {
		return nil, err
	}

	// Clone state before modification to preserve caller's original.
	newState := cloneState(state)

	messages := make([]chatMessage, 0, len(newState.Turns)+2)
	if opts.systemPrompt != "" {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: opts.systemPrompt,
		})
	}
	for _, turn := range newState.Turns {
		messages = append(messages, chatMessage{
			Role:    string(turn.Role),
			Content: turn.Content,
		})
	}
	messages = append(messages, chatMessage{Role: "user", Content: prompt})

	result := workflow.NewConversationResult("openai_compatible")
	result.StartedAt = time.Now()

	// Resolve MCP proxy config from options if present; nil cfg is safe (buildToolList skips proxy tools).
	cfg, _ := options[workflow.MCPProxyConfigKey].(*workflow.MCPProxyConfig) //nolint:errcheck // comma-ok type assertion; false ok means key absent or wrong type, cfg=nil is the correct fallback
	tools, toolChoice, toolErr := p.buildToolList(ctx, cfg)
	if toolErr != nil {
		return nil, toolErr
	}

	// Use the shared tool-call loop so MCP tool_calls are dispatched in conversation
	// mode just as they are in Execute. Without this, MCP is silently inactive when
	// the model returns finish_reason=tool_calls during a conversation turn.
	loopResult, loopErr := p.runToolCallLoop(ctx, &opts, messages, tools, toolChoice, stdout)
	if loopErr != nil {
		return nil, loopErr
	}

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	userTurn.Tokens = loopResult.inputTokens
	if err := newState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, loopResult.output)
	assistantTurn.Tokens = loopResult.outputTokens
	if err := newState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	result.Output = loopResult.output
	result.State = newState
	result.TokensInput = loopResult.inputTokens
	result.TokensOutput = loopResult.outputTokens
	result.TokensTotal = loopResult.totalTokens
	result.TokensEstimated = false
	result.CompletedAt = time.Now()

	return result, nil
}

func (p *OpenAICompatibleProvider) writeDisplayOutput(w io.Writer, output string) {
	if w == nil {
		return
	}
	if displayText := extractDisplayTextFromEvents(output, p.translateOpenAICompatibleDisplayEvents); displayText != "" {
		_, _ = io.WriteString(w, displayText) //nolint:gosec,errcheck // best-effort display output to stdout
	}
}

func (p *OpenAICompatibleProvider) parseAndValidateOptions(options map[string]any) (parsedOptions, error) {
	var opts parsedOptions

	rawBaseURL, ok := options["base_url"]
	if ok {
		opts.baseURL = strings.TrimSuffix(fmt.Sprintf("%v", rawBaseURL), "/")
	} else if envURL := os.Getenv("OPENAI_BASE_URL"); envURL != "" {
		opts.baseURL = strings.TrimSuffix(envURL, "/")
	} else {
		return opts, fmt.Errorf("openai_compatible: base_url is required")
	}

	rawModel, ok := options["model"]
	if ok {
		opts.model = fmt.Sprintf("%v", rawModel)
	} else if envModel := os.Getenv("OPENAI_MODEL"); envModel != "" {
		opts.model = envModel
	} else {
		return opts, fmt.Errorf("openai_compatible: model is required")
	}

	if rawKey, ok := options["api_key"]; ok {
		opts.apiKey = fmt.Sprintf("%v", rawKey)
	} else {
		opts.apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if rawSP, ok := options["system_prompt"]; ok {
		opts.systemPrompt = fmt.Sprintf("%v", rawSP)
	}

	temp, err := parseTemperatureOption(options)
	if err != nil {
		return opts, err
	}
	opts.temperature = temp

	maxTok, err := parseMaxCompletionTokensOption(options)
	if err != nil {
		return opts, err
	}
	opts.maxCompletionTokens = maxTok

	topP, err := parseTopPOption(options)
	if err != nil {
		return opts, err
	}
	opts.topP = topP

	return opts, nil
}

func parseTemperatureOption(options map[string]any) (*float64, error) {
	raw, ok := options["temperature"]
	if !ok {
		return nil, nil
	}
	var v float64
	switch t := raw.(type) {
	case float64:
		v = t
	case int:
		v = float64(t)
	default:
		return nil, fmt.Errorf("openai_compatible: temperature must be a number, got %T", raw)
	}
	if v < 0 || v > 2 {
		return nil, fmt.Errorf("openai_compatible: temperature must be between 0 and 2, got %v", v)
	}
	return &v, nil
}

func parseMaxCompletionTokensOption(options map[string]any) (*int, error) {
	// Prefer max_completion_tokens, fall back to max_tokens for legacy support.
	raw, ok := options["max_completion_tokens"]
	if !ok {
		raw, ok = options["max_tokens"]
		if !ok {
			return nil, nil
		}
	}

	switch v := raw.(type) {
	case int:
		if v < 0 {
			return nil, fmt.Errorf("openai_compatible: max_completion_tokens must be non-negative, got %d", v)
		}
		return &v, nil
	case float64:
		iv := int(v)
		if iv < 0 {
			return nil, fmt.Errorf("openai_compatible: max_completion_tokens must be non-negative, got %d", iv)
		}
		return &iv, nil
	}
	return nil, nil
}

func parseTopPOption(options map[string]any) (*float64, error) {
	raw, ok := options["top_p"]
	if !ok {
		return nil, nil
	}
	var v float64
	switch t := raw.(type) {
	case float64:
		v = t
	case int:
		v = float64(t)
	default:
		return nil, fmt.Errorf("openai_compatible: top_p must be a number, got %T", raw)
	}
	if v < 0 || v > 1 {
		return nil, fmt.Errorf("openai_compatible: top_p must be between 0 and 1, got %v", v)
	}
	return &v, nil
}

func mapHTTPError(resp *httpx.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("openai_compatible: authentication failed (HTTP 401)")
	case resp.StatusCode == http.StatusTooManyRequests:
		if retryAfter := resp.Headers["Retry-After"]; retryAfter != "" {
			return fmt.Errorf("openai_compatible: rate limited, retry after %s (HTTP 429)", retryAfter)
		}
		return fmt.Errorf("openai_compatible: rate limited (HTTP 429)")
	case resp.StatusCode >= 500:
		return fmt.Errorf("openai_compatible: server error (HTTP %d)", resp.StatusCode)
	case resp.StatusCode >= 400 && resp.StatusCode != http.StatusTeapot:
		return fmt.Errorf("openai_compatible: bad request (HTTP %d)", resp.StatusCode)
	default:
		return fmt.Errorf("openai_compatible: unexpected status (HTTP %d)", resp.StatusCode)
	}
}

func (p *OpenAICompatibleProvider) translateOpenAICompatibleDisplayEvents(line []byte) []DisplayEvent {
	type toolCall struct {
		ID       string `json:"id"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	type deltaOrMessage struct {
		Content   string     `json:"content"`
		ToolCalls []toolCall `json:"tool_calls"`
	}
	var chunk struct {
		Object  string `json:"object"`
		Choices []struct {
			Delta   deltaOrMessage `json:"delta"`
			Message deltaOrMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(line, &chunk); err != nil {
		return nil
	}
	if chunk.Object == "" {
		return nil
	}
	if len(chunk.Choices) == 0 {
		return nil
	}

	// Prefer delta (streaming SSE) over message (complete response).
	source := chunk.Choices[0].Delta
	if source.Content == "" && len(source.ToolCalls) == 0 {
		source = chunk.Choices[0].Message
	}

	var events []DisplayEvent

	if source.Content != "" {
		events = append(events, DisplayEvent{Type: chunk.Object, Kind: EventText, Text: source.Content})
	}

	for _, tc := range source.ToolCalls {
		events = append(events, DisplayEvent{
			Type: chunk.Object,
			Kind: EventToolUse,
			Name: tc.Function.Name,
			Arg:  extractArgPreview(tc.Function.Arguments),
			ID:   tc.ID,
		})
	}

	if len(events) == 0 {
		return nil
	}
	return events
}
