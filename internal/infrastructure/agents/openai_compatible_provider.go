package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/httputil"
)

var _ ports.AgentProvider = (*OpenAICompatibleProvider)(nil)

// OpenAICompatibleProvider implements AgentProvider via the Chat Completions HTTP API.
// Compatible with OpenAI, Ollama, vLLM, Groq, and any OpenAI-compatible backend.
type OpenAICompatibleProvider struct {
	httpClient *httputil.Client
}

// maxResponseBodyBytes limits response reading to prevent memory exhaustion.
const maxResponseBodyBytes = 10 * 1024 * 1024 // 10MB

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
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
	baseURL      string
	model        string
	apiKey       string
	systemPrompt string
	temperature  *float64
	maxTokens    *int
	topP         *float64
}

func NewOpenAICompatibleProvider(opts ...OpenAICompatibleProviderOption) *OpenAICompatibleProvider {
	p := &OpenAICompatibleProvider{
		httpClient: httputil.NewClient(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *OpenAICompatibleProvider) Name() string {
	return "openai_compatible"
}

func (p *OpenAICompatibleProvider) Validate() error {
	return nil
}

func (p *OpenAICompatibleProvider) Execute(ctx context.Context, prompt string, options map[string]any) (*workflow.AgentResult, error) {
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

	result := workflow.NewAgentResult("openai_compatible")

	resp, err := p.callChatCompletions(ctx, &opts, messages)
	if err != nil {
		return nil, err
	}

	result.Output = resp.Choices[0].Message.Content
	result.Tokens = resp.Usage.TotalTokens
	result.TokensEstimated = false
	result.CompletedAt = time.Now()

	if outputFormat, ok := options["output_format"]; ok && outputFormat == "json" {
		parsed, err := p.parseJSONResponse(result.Output)
		if err != nil {
			return nil, err
		}
		result.Response = parsed
	}

	return result, nil
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

func (p *OpenAICompatibleProvider) ExecuteConversation(ctx context.Context, state *workflow.ConversationState, prompt string, options map[string]any) (*workflow.ConversationResult, error) {
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

	resp, err := p.callChatCompletions(ctx, &opts, messages)
	if err != nil {
		return nil, err
	}

	assistantContent := resp.Choices[0].Message.Content

	userTurn := workflow.NewTurn(workflow.TurnRoleUser, prompt)
	userTurn.Tokens = resp.Usage.PromptTokens
	if err := newState.AddTurn(userTurn); err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	assistantTurn := workflow.NewTurn(workflow.TurnRoleAssistant, assistantContent)
	assistantTurn.Tokens = resp.Usage.CompletionTokens
	if err := newState.AddTurn(assistantTurn); err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	result.Output = assistantContent
	result.State = newState
	result.TokensInput = resp.Usage.PromptTokens
	result.TokensOutput = resp.Usage.CompletionTokens
	result.TokensTotal = resp.Usage.TotalTokens
	result.TokensEstimated = false
	result.CompletedAt = time.Now()

	return result, nil
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

	maxTok, err := parseMaxTokensOption(options)
	if err != nil {
		return opts, err
	}
	opts.maxTokens = maxTok

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

func parseMaxTokensOption(options map[string]any) (*int, error) {
	raw, ok := options["max_tokens"]
	if !ok {
		return nil, nil
	}
	switch v := raw.(type) {
	case int:
		if v < 0 {
			return nil, fmt.Errorf("openai_compatible: max_tokens must be non-negative, got %d", v)
		}
		return &v, nil
	case float64:
		iv := int(v)
		if iv < 0 {
			return nil, fmt.Errorf("openai_compatible: max_tokens must be non-negative, got %d", iv)
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

func (p *OpenAICompatibleProvider) callChatCompletions(ctx context.Context, opts *parsedOptions, messages []chatMessage) (*chatCompletionsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("openai_compatible: %w", err)
	}

	endpoint := opts.baseURL + "/chat/completions"

	reqBody := chatCompletionsRequest{
		Model:       opts.model,
		Messages:    messages,
		Temperature: opts.temperature,
		MaxTokens:   opts.maxTokens,
		TopP:        opts.topP,
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

func mapHTTPError(resp *httputil.Response) error {
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
