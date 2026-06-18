---
title: "Creating an Agent Provider"
---

Guide for implementing a new agent provider in AWF. Covers the domain contract, infrastructure base layer, hooks, options, display events, session management, and registration.

## Architecture

Agent providers live in the **infrastructure layer** and implement the `ports.AgentProvider` interface defined in the **domain layer**. The base infrastructure handles execution orchestration, token counting, state cloning, and stream filtering. Each provider only implements the provider-specific parts via hooks.

```
Domain Layer (ports)                Infrastructure Layer (agents)
┌───────────────────────┐          ┌──────────────────────────────────┐
│ AgentProvider          │◄────────│ baseCLIProvider                  │
│ CLIExecutor            │         │   ├── execute()                  │
│ Tokenizer              │         │   ├── executeConversation()      │
│ Logger                 │         │   └── cliProviderHooks{...}      │
└───────────────────────┘          │                                  │
                                   │ YourProvider                     │
                                   │   ├── newBase() → hooks wiring   │
                                   │   ├── buildExecuteArgs()         │
                                   │   ├── buildConversationArgs()    │
                                   │   ├── extractSessionID()         │
                                   │   ├── parseDisplayEvents()       │
                                   │   └── validateOptions()          │
                                   └──────────────────────────────────┘
```

## Domain Contract

### AgentProvider Interface

**File:** `internal/domain/ports/agent_provider.go`

```go
type AgentProvider interface {
    Execute(ctx context.Context, prompt string, options map[string]any,
        stdout, stderr io.Writer) (*workflow.AgentResult, error)
    ExecuteConversation(ctx context.Context, state *workflow.ConversationState,
        prompt string, options map[string]any,
        stdout, stderr io.Writer) (*workflow.ConversationResult, error)
    Name() string
    Validate() error
}
```

| Method | Purpose |
|--------|---------|
| `Execute` | Single-turn prompt execution. Returns `AgentResult` with output, tokens, timing. |
| `ExecuteConversation` | Multi-turn execution with conversation state. Returns `ConversationResult` with updated state. |
| `Name` | Unique provider identifier used in workflow YAML (`provider: your_name`). |
| `Validate` | Pre-flight check (binary in PATH, API key set, etc.). Called before first execution. |

### AgentResult

**File:** `internal/domain/workflow/agent_config.go`

```go
type AgentResult struct {
    Provider        string
    Output          string         // extracted text output
    DisplayOutput   string         // filtered output for terminal display
    Response        map[string]any // parsed JSON response (optional)
    Tokens          int
    TokensEstimated bool
    Error           error
    StartedAt       time.Time
    CompletedAt     time.Time
}
```

### ConversationResult

**File:** `internal/domain/workflow/conversation.go`

```go
type ConversationResult struct {
    Provider        string
    State           *ConversationState // updated state with new turns
    Output          string             // last assistant response
    DisplayOutput   string
    Response        map[string]any
    TokensInput     int
    TokensOutput    int
    TokensTotal     int
    TokensEstimated bool
    Error           error
    StartedAt       time.Time
    CompletedAt     time.Time
}
```

### ConversationState

```go
type ConversationState struct {
    SessionID   string
    Turns       []Turn
    TotalTurns  int
    TotalTokens int
    StoppedBy   StopReason
}

type Turn struct {
    Role    TurnRole // "system", "user", "assistant"
    Content string
    Tokens  int
}
```

## Base Layer: baseCLIProvider

All CLI-based providers delegate to `baseCLIProvider`, which handles:

- Prompt validation
- CLI binary execution via `CLIExecutor`
- Stream filtering and display event rendering
- Token counting via injected `Tokenizer`
- Conversation state cloning and turn management
- Timing (StartedAt / CompletedAt)

### Hooks

Provider-specific behavior is injected via `cliProviderHooks`:

```go
type cliProviderHooks struct {
    buildExecuteArgs      func(prompt string, options map[string]any) ([]string, error)
    buildConversationArgs func(state *workflow.ConversationState, prompt string, options map[string]any) ([]string, error)
    extractSessionID      func(output string) (string, error)
    extractTextContent    func(output string) string       // optional
    validateOptions       func(options map[string]any) error // optional
    parseDisplayEvents    DisplayEventParser                 // optional
    extractTokenUsage     func(rawOutput string) *tokenUsage // optional
    mcpInjector           func(ctx context.Context, args []string, cfg *workflow.MCPProxyConfig,
                              mcpConfigPath string, options map[string]any) (
                              newArgs []string, newOptions map[string]any,
                              cleanup func() error, err error) // optional
}
```

| Hook | Required | Purpose |
|------|----------|---------|
| `buildExecuteArgs` | **yes** | Construct CLI argv for single-turn execution. |
| `buildConversationArgs` | **yes** | Construct CLI argv for multi-turn execution (session resume). |
| `extractSessionID` | **yes** | Parse session/thread ID from CLI output for conversation resume. |
| `extractTextContent` | no | Extract human-readable text from structured output (e.g., JSON wrapper). Falls back to raw output if nil. |
| `validateOptions` | no | Validate provider-specific options before execution. Return error to reject. |
| `parseDisplayEvents` | no | Parse a single NDJSON line into `[]DisplayEvent` for real-time terminal display. |
| `extractTokenUsage` | no | Parse exact input/output/total token counts from a structured CLI event (e.g. Gemini `result.stats`, Claude `usage`). When set, the base layer uses these counts instead of estimating via the tokenizer and clears `TokensEstimated`. |
| `mcpInjector` | no | Provider-specific MCP proxy injection: appends MCP flags to args, optionally mutates options (e.g. prefixes `system_prompt` in coexistence mode), and returns a cleanup that runs after the CLI exits. See [MCP Proxy Integration](#mcp-proxy-integration). |

### What baseCLIProvider Does For You

**In `execute()`:**
1. Rejects empty prompts
2. Calls `validateOptions` hook (if set)
3. Calls `buildExecuteArgs` hook to get CLI arguments
4. Runs binary via `CLIExecutor.Run()`
5. Filters output through `StreamFilterWriter` (if `parseDisplayEvents` set)
6. Counts output tokens: `b.tokenizer.CountTokens(output)`
7. Builds and returns `AgentResult`

**In `executeConversation()`:**
1. Clones conversation state (caller's original is never mutated)
2. Appends user turn to cloned state
3. Calls `validateOptions` and `buildConversationArgs` hooks
4. Runs binary
5. Calls `extractSessionID` hook, updates state
6. Appends assistant turn to state
7. Counts input tokens (`CountTurnsTokens`) and output tokens (`CountTokens`)
8. Builds and returns `ConversationResult`

## Step-by-Step Implementation

### 1. Create the provider file

**File:** `internal/infrastructure/agents/myprovider_provider.go`

```go
package agents

import (
    "context"
    "fmt"
    "io"
    "os/exec"

    "github.com/awf-project/cli/internal/domain/ports"
    "github.com/awf-project/cli/internal/domain/workflow"
    "github.com/awf-project/cli/internal/infrastructure/logger"
)

type MyProviderProvider struct {
    base      *baseCLIProvider
    logger    ports.Logger
    executor  ports.CLIExecutor
    tokenizer ports.Tokenizer
}
```

### 2. Add constructors

Two constructors are required: a zero-config default and a functional-options variant.

```go
func NewMyProviderProvider() *MyProviderProvider {
    p := &MyProviderProvider{
        logger:   logger.NopLogger{},
        executor: NewExecCLIExecutor(),
    }
    p.base = p.newBase()
    return p
}

func NewMyProviderProviderWithOptions(opts ...MyProviderProviderOption) *MyProviderProvider {
    p := &MyProviderProvider{
        logger:   logger.NopLogger{},
        executor: NewExecCLIExecutor(),
    }
    for _, opt := range opts {
        opt(p)
    }
    p.base = p.newBase()
    return p
}
```

> **Important:** `p.newBase()` must be called **after** applying options, since options may set the executor, logger, or tokenizer that `newBase` forwards.

### 3. Wire the hooks via newBase()

```go
func (p *MyProviderProvider) newBase() *baseCLIProvider {
    b := newBaseCLIProvider("myprovider", "myprovider-cli", p.executor, p.logger, cliProviderHooks{
        buildExecuteArgs:      p.buildExecuteArgs,
        buildConversationArgs: p.buildConversationArgs,
        extractSessionID:      p.extractSessionID,
        validateOptions:       validateMyProviderOptions,
        parseDisplayEvents:    p.parseMyProviderDisplayEvents,
    })
    if p.tokenizer != nil {
        b.tokenizer = p.tokenizer
    }
    return b
}
```

**Parameters to `newBaseCLIProvider`:**

| Parameter | Value |
|-----------|-------|
| `name` | Provider identifier returned by `Name()`. Used in `AgentResult.Provider`. Must match the value users write in `provider:` YAML field. |
| `binary` | CLI binary name looked up in `$PATH`. |
| `executor` | The `CLIExecutor` to run the binary. Always forward `p.executor`. |
| `log` | Logger. Nil-defaults to `NopLogger`. |
| `hooks` | Provider-specific hooks (see table above). |

### 4. Implement the required hooks

#### buildExecuteArgs

Construct the CLI arguments for a single-turn call.

```go
func (p *MyProviderProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
    args := []string{"run", "--prompt", prompt, "--format", "json"}

    if model, ok := getStringOption(options, "model"); ok {
        args = append(args, "--model", model)
    }
    if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
        args = append(args, "--yes")
    }

    return args, nil
}
```

**Available helpers:** `getStringOption(options, key)`, `getBoolOption(options, key)` — type-safe extraction from `map[string]any`.

#### buildConversationArgs

Construct CLI arguments for multi-turn. Must handle session resume vs first turn.

```go
func (p *MyProviderProvider) buildConversationArgs(
    state *workflow.ConversationState, prompt string, options map[string]any,
) ([]string, error) {
    var args []string
    if state.SessionID != "" {
        args = []string{"resume", state.SessionID, "--prompt", prompt, "--format", "json"}
    } else {
        effectivePrompt := buildFirstTurnPrompt(prompt, options)
        args = []string{"run", "--prompt", effectivePrompt, "--format", "json"}
    }

    if model, ok := getStringOption(options, "model"); ok {
        args = append(args, "--model", model)
    }

    return args, nil
}
```

**Key patterns:**
- Use `state.SessionID` to detect resume vs new conversation.
- Use `buildFirstTurnPrompt(prompt, options)` to inline `system_prompt` into the first message when the CLI has no native `--system-prompt` flag.
- Always force a structured output format (JSON/NDJSON) for reliable parsing.

#### extractSessionID

Parse the session identifier from CLI output so subsequent turns can resume.

```go
func (p *MyProviderProvider) extractSessionID(output string) (string, error) {
    if output == "" {
        return "", errors.New("empty output")
    }
    evt := findFirstNDJSONEvent(output, "session_start")
    if evt == nil {
        return "", errors.New("session_start event not found")
    }
    id, ok := evt["session_id"].(string)
    if !ok || id == "" {
        return "", errors.New("session_id missing or empty")
    }
    return id, nil
}
```

**Available helper:** `findFirstNDJSONEvent(output, eventType)` — scans NDJSON output line-by-line for the first `{"type": eventType, ...}` event and returns it as `map[string]any`.

> Session ID extraction errors are **non-fatal**. The base layer logs the error and continues in stateless mode. The conversation still works; it just cannot resume on the next turn.

### 5. Implement the optional hooks

#### validateOptions

Reject invalid option combinations before execution.

```go
func validateMyProviderOptions(options map[string]any) error {
    if options == nil {
        return nil
    }
    if model, ok := getStringOption(options, "model"); ok {
        if !strings.HasPrefix(model, "myprovider-") {
            return fmt.Errorf("invalid model: %s (must start with 'myprovider-')", model)
        }
    }
    return nil
}
```

#### parseDisplayEvents

Parse a single NDJSON line into display events for real-time terminal rendering.

```go
func (p *MyProviderProvider) parseMyProviderDisplayEvents(line []byte) []DisplayEvent {
    var evt struct {
        Type    string `json:"type"`
        Content string `json:"content"`
        Tool    string `json:"tool_name"`
    }
    if err := json.Unmarshal(line, &evt); err != nil {
        return nil
    }

    switch evt.Type {
    case "text":
        return []DisplayEvent{{Kind: EventText, Text: evt.Content}}
    case "tool_call":
        return []DisplayEvent{{Kind: EventToolUse, Name: evt.Tool}}
    }
    return nil
}
```

**Display event kinds:**

| Constant | Purpose |
|----------|---------|
| `EventText` | Text content from the assistant. Aggregated for `DisplayOutput`. |
| `EventToolUse` | Tool invocation. Rendered as tool name + argument preview. |

**DisplayEvent fields:**

| Field | Required | Purpose |
|-------|----------|---------|
| `Kind` | **yes** | `EventText` or `EventToolUse` |
| `Text` | for text | The text content |
| `Name` | for tools | Tool name |
| `Arg` | no | Truncated argument preview. Use `extractArgPreviewFromMap(args)` or `extractArgPreview(jsonStr)`. |
| `ID` | no | Tool call ID (empty if provider doesn't emit one) |
| `Delta` | no | `true` for streaming deltas (partial text chunks) |
| `Type` | no | Raw event type from provider output (for debugging) |

#### extractTokenUsage

Parse exact token counts from a structured CLI event so the base layer can skip its estimator.

```go
type tokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
    CostUSD      float64
}

func (p *MyProviderProvider) extractMyProviderTokenUsage(rawOutput string) *tokenUsage {
    evt := findFirstNDJSONEvent(rawOutput, "result")
    if evt == nil {
        return nil
    }
    stats, ok := evt["stats"].(map[string]any)
    if !ok {
        return nil
    }
    return &tokenUsage{
        InputTokens:  intFromMap(stats, "input_tokens"),
        OutputTokens: intFromMap(stats, "output_tokens"),
        TotalTokens:  intFromMap(stats, "total_tokens"),
    }
}
```

**Available helper:** `intFromMap(m, key)` — extracts an `int` from `map[string]any` regardless of whether the source value is `int`, `int64`, `float64`, or a numeric string.

**When to set this hook.** Only when the CLI emits exact token counts in its structured output. If the hook is omitted (or returns `nil`), the base layer falls back to estimating via `Tokenizer.CountTokens` and sets `result.TokensEstimated = true`. Returning a non-nil `*tokenUsage` overrides the estimate and clears `TokensEstimated`.

Reference implementations: `extractClaudeTokenUsage`, `extractGeminiTokenUsage`, `extractCodexTokenUsage`, `extractOpenCodeTokenUsage`.

### 6. Implement the AgentProvider interface methods

#### Execute

Delegate to `p.base.execute()`, then apply provider-specific post-processing.

```go
func (p *MyProviderProvider) Execute(
    ctx context.Context, prompt string, options map[string]any, stdout, stderr io.Writer,
) (*workflow.AgentResult, error) {
    result, rawOutput, err := p.base.execute(ctx, prompt, options, stdout, stderr)
    if err != nil {
        return nil, err
    }

    // Post-processing: extract text from structured output
    if extracted := extractDisplayTextFromEvents(rawOutput, p.parseMyProviderDisplayEvents); extracted != "" {
        result.Output = extracted
        tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
        result.Tokens = tokens
    }

    // Optional: parse JSON response
    userFormat, _ := getStringOption(options, "output_format")
    if userFormat == "json" || userFormat == "stream-json" {
        if jsonResp := tryParseJSONResponse(rawOutput); jsonResp != nil {
            result.Response = jsonResp
        }
    }

    return result, nil
}
```

**Why post-process?** When the CLI outputs NDJSON (events), the raw output is not human-readable. Post-processing extracts the actual assistant text and re-counts tokens on the extracted content.

#### ExecuteConversation

Most providers simply delegate without post-processing:

```go
func (p *MyProviderProvider) ExecuteConversation(
    ctx context.Context, state *workflow.ConversationState, prompt string,
    options map[string]any, stdout, stderr io.Writer,
) (*workflow.ConversationResult, error) {
    result, _, err := p.base.executeConversation(ctx, state, prompt, options, stdout, stderr)
    if err != nil {
        return nil, err
    }
    return result, nil
}
```

#### Name and Validate

```go
func (p *MyProviderProvider) Name() string {
    return "myprovider"
}

func (p *MyProviderProvider) Validate() error {
    _, err := exec.LookPath("myprovider-cli")
    if err != nil {
        return fmt.Errorf("myprovider-cli not found in PATH: %w", err)
    }
    return nil
}
```

### 7. Add functional options

**File:** `internal/infrastructure/agents/options.go`

```go
type MyProviderProviderOption func(*MyProviderProvider)

func WithMyProviderExecutor(executor ports.CLIExecutor) MyProviderProviderOption {
    return func(p *MyProviderProvider) {
        p.executor = executor
    }
}

func WithMyProviderTokenizer(tok ports.Tokenizer) MyProviderProviderOption {
    return func(p *MyProviderProvider) {
        p.tokenizer = tok
    }
}

func WithMyProviderLogger(l ports.Logger) MyProviderProviderOption {
    return func(p *MyProviderProvider) {
        p.logger = l
    }
}
```

### 8. Register in the registry

**File:** `internal/infrastructure/agents/registry.go`

Add to `RegisterDefaults()`:

```go
func (r *AgentRegistry) RegisterDefaults() error {
    defaults := []ports.AgentProvider{
        NewClaudeProvider(),
        NewCodexProvider(),
        NewGeminiProvider(),
        NewOpenAICompatibleProvider(),
        NewOpenCodeProvider(),
        NewCopilotProvider(),
        NewMyProviderProvider(), // <-- add here
    }
    // ...
}
```

## Testing

### Option tests

**File:** `internal/infrastructure/agents/provider_options_test.go`

```go
func TestWithMyProviderTokenizer(t *testing.T) {
    tok := &mockTokenizer{countTokensResult: 99}
    provider := NewMyProviderProviderWithOptions(
        WithMyProviderExecutor(mocks.NewMockCLIExecutor()),
        WithMyProviderTokenizer(tok),
    )
    assert.Equal(t, tok, provider.base.tokenizer)
}
```

### Argument construction tests

Test that `buildExecuteArgs` and `buildConversationArgs` produce correct CLI arguments for all option combinations.

```go
func TestMyProvider_BuildExecuteArgs(t *testing.T) {
    tests := []struct {
        name     string
        prompt   string
        options  map[string]any
        wantArgs []string
        wantErr  bool
    }{
        {
            name:     "basic prompt",
            prompt:   "hello",
            options:  nil,
            wantArgs: []string{"run", "--prompt", "hello", "--format", "json"},
        },
        {
            name:     "with model",
            prompt:   "hello",
            options:  map[string]any{"model": "myprovider-large"},
            wantArgs: []string{"run", "--prompt", "hello", "--format", "json", "--model", "myprovider-large"},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := NewMyProviderProvider()
            args, err := p.buildExecuteArgs(tt.prompt, tt.options)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.wantArgs, args)
        })
    }
}
```

### Session ID extraction tests

```go
func TestMyProvider_ExtractSessionID(t *testing.T) {
    tests := []struct {
        name    string
        output  string
        wantID  string
        wantErr bool
    }{
        {
            name:   "valid session",
            output: `{"type":"session_start","session_id":"abc-123"}`,
            wantID: "abc-123",
        },
        {
            name:    "missing event",
            output:  `{"type":"text","content":"hello"}`,
            wantErr: true,
        },
        {
            name:    "empty output",
            output:  "",
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := NewMyProviderProvider()
            id, err := p.extractSessionID(tt.output)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.wantID, id)
        })
    }
}
```

### Display event parser tests

```go
func TestMyProvider_ParseDisplayEvents(t *testing.T) {
    p := NewMyProviderProvider()

    t.Run("text event", func(t *testing.T) {
        events := p.parseMyProviderDisplayEvents([]byte(`{"type":"text","content":"hello"}`))
        require.Len(t, events, 1)
        assert.Equal(t, EventText, events[0].Kind)
        assert.Equal(t, "hello", events[0].Text)
    })

    t.Run("tool event", func(t *testing.T) {
        events := p.parseMyProviderDisplayEvents([]byte(`{"type":"tool_call","tool_name":"read_file"}`))
        require.Len(t, events, 1)
        assert.Equal(t, EventToolUse, events[0].Kind)
        assert.Equal(t, "read_file", events[0].Name)
    })

    t.Run("unknown event returns nil", func(t *testing.T) {
        events := p.parseMyProviderDisplayEvents([]byte(`{"type":"unknown"}`))
        assert.Nil(t, events)
    })

    t.Run("invalid JSON returns nil", func(t *testing.T) {
        events := p.parseMyProviderDisplayEvents([]byte(`not json`))
        assert.Nil(t, events)
    })
}
```

### Option validation tests

```go
func TestMyProvider_ValidateOptions(t *testing.T) {
    tests := []struct {
        name    string
        options map[string]any
        wantErr bool
    }{
        {name: "nil options", options: nil},
        {name: "valid model", options: map[string]any{"model": "myprovider-large"}},
        {name: "invalid model", options: map[string]any{"model": "gpt-4"}, wantErr: true},
        {name: "unknown option ignored", options: map[string]any{"unknown": "value"}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateMyProviderOptions(tt.options)
            if tt.wantErr {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## Mandatory Cross-Provider Conventions

Every provider **must** handle these patterns. Omitting any of them creates inconsistency for users who switch between providers in their workflows.

### Force structured output format

All CLI providers force NDJSON/JSON output at the CLI level, regardless of what the user requests. This ensures consistent session ID extraction, display event filtering, and text extraction.

```go
// The user's output_format preference controls post-processing (display vs raw),
// but the wire format is always NDJSON.
func (p *MyProviderProvider) buildExecuteArgs(prompt string, options map[string]any) ([]string, error) {
    args := []string{"run", "--prompt", prompt}
    args = append(args, "--format", "json") // always force structured output
    // ...
}
```

How each provider does it:

| Provider | Forced flag |
|----------|-------------|
| Claude | `--output-format stream-json --verbose` |
| Gemini | `--output-format stream-json` |
| Codex | `exec --json` |
| Copilot | `--output-format=json --silent` |
| OpenCode | `--format json` |

### Handle `dangerously_skip_permissions`

This option is **cross-provider** — users expect it to work in any workflow regardless of provider. Each CLI maps it to its own flag:

```go
// In buildExecuteArgs and buildConversationArgs:
if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
    args = append(args, "--your-cli-equivalent-flag")
}
```

| Provider | CLI flag |
|----------|----------|
| Claude | `--dangerously-skip-permissions` |
| Gemini | `--approval-mode=yolo` |
| Codex | `--dangerously-bypass-approvals-and-sandbox` |
| Copilot | `--allow-all` |
| OpenCode | `--dangerously-skip-permissions` |

If your CLI has no equivalent, log at debug level and ignore the option:

```go
if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
    p.logger.Debug("dangerously_skip_permissions is not supported by myprovider and will be ignored")
}
```

Before assuming "not supported", run `your-cli run --help` against the installed binary — CLI vendors occasionally add the flag in a minor release without changing the help summary.

### Handle `system_prompt`

Only Claude has a native `--system-prompt` flag. All other providers inline it into the first turn's message using the shared helper:

```go
// In buildConversationArgs, for the first turn (no session ID):
effectivePrompt := buildFirstTurnPrompt(prompt, options)
// Returns: "system prompt content\n\nuserPrompt" or just "userPrompt" if no system_prompt
```

If your CLI has a native system prompt flag, use it directly instead:

```go
if sysPrompt, ok := getStringOption(options, "system_prompt"); ok && sysPrompt != "" {
    args = append(args, "--system-prompt", sysPrompt)
}
```

System prompt must only be applied on the **first turn**. On subsequent turns (when `state.SessionID != ""`), the provider's session already retains the system context.

### Handle `model`

Every provider must support the `model` option. Validate the model name in `validateOptions` to reject models incompatible with your CLI:

```go
func validateMyProviderOptions(options map[string]any) error {
    if options == nil {
        return nil
    }
    if model, ok := getStringOption(options, "model"); ok {
        if !strings.HasPrefix(model, "myprovider-") {
            return fmt.Errorf("invalid model: %s (must start with 'myprovider-')", model)
        }
    }
    return nil
}
```

### Handle `output_format` for response parsing

The `output_format` option controls what the user sees. When the user requests `json` or `stream-json`, expose the parsed JSON response in `result.Response`:

```go
// In Execute(), after text extraction:
userFormat, _ := getStringOption(options, "output_format")
if userFormat == "json" || userFormat == "stream-json" {
    if jsonResp := tryParseJSONResponse(rawOutput); jsonResp != nil {
        result.Response = jsonResp
    }
}
```

### Ignore unknown options silently

Go's `map[string]any` behavior means unsupported option keys are simply not looked up. Never iterate over options to reject unknown keys — this allows cross-provider workflows to pass provider-specific options that only apply to certain providers.

### Token counting pattern

Every `CountTokens` call in provider code must use the `//nolint:errcheck` directive with an explanatory comment. This is enforced by `golangci-lint` with `check-blank: true`:

```go
tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck // ApproximationTokenizer never errors with a valid ratio
result.Tokens = tokens
```

### NUL byte sanitization in display event parsers

CLI tools may output NUL bytes (`0x00`) that break `json.Unmarshal`. Sanitize before parsing:

```go
func (p *MyProviderProvider) parseMyProviderDisplayEvents(line []byte) []DisplayEvent {
    // Escape NUL bytes to valid JSON unicode sequences
    sanitized := bytes.ReplaceAll(line, []byte{0x00}, []byte(`\u0000`))

    var evt struct { /* ... */ }
    if err := json.Unmarshal(sanitized, &evt); err != nil {
        return nil
    }
    // ...
}
```

Codex and OpenCode use this escape pattern. Claude replaces NUL with spaces instead.

### Error handling conventions

| Scenario | Handling |
|----------|----------|
| `Validate()` — binary not found | Return `fmt.Errorf("binary not found in PATH: %w", err)` |
| `extractSessionID` fails | **Non-fatal.** Base layer logs at debug and continues stateless. |
| JSON parsing fails in `Execute()` | **Non-fatal.** `result.Response` stays nil. |
| `validateOptions` returns error | **Fatal.** Execution is aborted before running the CLI. |
| Empty output from CLI | Base layer substitutes `" "` (single space) to prevent zero-length issues. |

### Apply `dangerously_skip_permissions` in both arg builders

The `buildExecuteArgs` and `buildConversationArgs` hooks must **both** handle `dangerously_skip_permissions` (and `model`, etc.). Users don't know which execution path their workflow triggers — missing the option in one path creates hard-to-debug inconsistencies.

### extractTextContent vs extractDisplayTextFromEvents

Two mechanisms exist for extracting human-readable text from structured output:

| Mechanism | When to use |
|-----------|-------------|
| `extractTextContent` hook | Your CLI wraps the final answer in a specific JSON envelope (e.g., Claude's `result` event, Copilot's `assistant.message` event). Set this hook to extract from that envelope. |
| `extractDisplayTextFromEvents()` | Your CLI outputs NDJSON events where text is spread across multiple `EventText` events. This helper aggregates all text events via your `parseDisplayEvents` hook. |

Most providers use `extractDisplayTextFromEvents` in their `Execute()` post-processing. Only set `extractTextContent` if your provider needs a different extraction strategy for `executeConversation`.

## MCP Proxy Integration

The `mcp_proxy:` workflow block lets users route an agent's tool calls through an AWF-managed local MCP server, exposing built-in `Read`/`Write`/`Edit`/`Bash`/`Glob`/`Grep` tools and/or AWF gRPC plugins as MCP tools. See [docs/user-guide/mcp-proxy.md](../user-guide/mcp-proxy.md) for the user-facing contract and [ADR 017](../ADR/017-mcp-proxy-stdio-subprocess-for-tool-interception.md) for the protocol/topology rationale.

To support `mcp_proxy:` in your provider, implement the `mcpInjector` hook. It is the only extension point — the base layer handles spawning `awf mcp-serve` and tearing it down. Providers that omit the hook get a clean "MCP proxy not supported" error at validation time; you can also ship the provider with no MCP support initially and add the hook later.

### The mcpInjector hook

```go
mcpInjector func(
    ctx           context.Context,
    args          []string,
    cfg           *workflow.MCPProxyConfig,
    mcpConfigPath string,
    options       map[string]any,
) (
    newArgs    []string,
    newOptions map[string]any,
    cleanup    func() error,
    err        error,
)
```

**Inputs:**

| Parameter | Purpose |
|-----------|---------|
| `ctx` | Parent context of the agent execution. Use it for any sub-process spawned during injection (e.g., `gemini mcp add`) so cancellation propagates. Do NOT pass it to the returned cleanup closure — cleanup must run after parent cancellation; use `context.Background()` inside the closure. |
| `args` | The CLI argv built by `buildExecuteArgs` / `buildConversationArgs`. Never mutate this slice; always copy it into `newArgs`. |
| `cfg` | The `mcp_proxy:` block from the workflow YAML. **Always nil-check first** — when nil, return `(args, options, noopMCPCleanup, nil)`. |
| `mcpConfigPath` | Path to a tmp JSON file that the spawned `awf mcp-serve` reads to learn which built-ins to expose and which plugin operations to route. Owned by `ToolProxyService`; do NOT delete or modify it. |
| `options` | The workflow options map. Clone before mutating (see "Coexistence mode" below). |

**Outputs:**

| Return | Purpose |
|--------|---------|
| `newArgs` | A new slice (never the input slice) with provider-specific MCP flags appended. |
| `newOptions` | Either the original `options` or a clone with mutations. The base layer replaces its local map with this value. |
| `cleanup` | Invoked AFTER the agent process exits. Must be idempotent (`sync.Once`) and use `context.Background()` for any teardown subprocess. Return `noopMCPCleanup` when there is nothing to undo. |
| `err` | Non-nil aborts the agent execution before spawning the CLI. Wrap with `%w`. |

The base layer calls `mcpInjector` only when `cfg != nil && cfg.Enable && hooks.mcpInjector != nil`. Both `execute()` and `executeConversation()` invoke it on the same args — there is no separate hook for conversation.

### Five integration patterns

Five distinct strategies exist in the codebase, dictated by what each CLI supports. Pick the one matching your CLI's MCP API surface. Do not infer support from another provider: verify the target CLI's `--help`, config discovery rules, tool naming, and non-interactive approval behavior.

#### Pattern A: Wrapper config file (Claude)

Use when the CLI accepts a flag like `--mcp-config <path>` pointing to a config file in a CLI-native shape (different from AWF's internal `mcpConfigPath` shape). Write a small wrapper file mapping a server name to `awf mcp-serve --config=<internal>`, pass the wrapper path to the CLI flag, and clean up the wrapper file after the CLI exits.

```go
func claudeMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig,
    mcpConfigPath string, options map[string]any) (
    newArgs []string, newOptions map[string]any, cleanup func() error, err error) {

    if cfg == nil {
        return args, options, noopMCPCleanup, nil
    }
    wrapperPath, wrapperCleanup, werr := writeClaudeMCPWrapper(mcpConfigPath)
    if werr != nil {
        return nil, options, noopMCPCleanup, werr
    }
    newArgs = make([]string, len(args), len(args)+4)
    copy(newArgs, args)
    newArgs = append(newArgs, "--mcp-config", wrapperPath)
    if cfg.InterceptBuiltins {
        newArgs = append(newArgs, "--tools", "", "--strict-mcp-config")
    }
    return newArgs, options, wrapperCleanup, nil
}
```

**Cleanup:** removes the wrapper file (the internal config at `mcpConfigPath` is owned by `ToolProxyService`).

#### Pattern B: Persistent subcommand registration (Gemini)

Use when the CLI exposes a CRUD subcommand (`<cli> mcp add <name> <cmd>` / `<cli> mcp remove <name>`) that writes to the CLI's own settings file. Each injector call registers a uniquely-named server, the cleanup unregisters that same name.

```go
func (p *GeminiProvider) geminiMCPInjector(ctx context.Context, args []string, cfg *workflow.MCPProxyConfig,
    mcpConfigPath string, options map[string]any) (
    newArgs []string, newOptions map[string]any, cleanup func() error, err error) {

    if cfg == nil {
        return args, options, noopMCPCleanup, nil
    }
    name := mcpProxyNamePrefix + randShortID(8)
    serveCmd := mcpServeCommand(mcpConfigPath)
    addProgram := "gemini mcp add " + name + " " + strings.Join(serveCmd, " ")

    addCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if _, err := p.cmdExecutor.Execute(addCtx, &ports.Command{Program: addProgram}); err != nil {
        return nil, options, noopMCPCleanup, fmt.Errorf("gemini mcp add: %w", err)
    }

    newArgs = make([]string, len(args), len(args)+2)
    copy(newArgs, args)
    if cfg.InterceptBuiltins {
        newArgs = append(newArgs, "--allowed-mcp-server-names", name)
    }

    var once sync.Once
    var removeErr error
    return newArgs, options, func() error {
        once.Do(func() {
            _, removeErr = p.cmdExecutor.Execute(context.Background(), &ports.Command{
                Program: "gemini mcp remove " + name,
            })
        })
        return removeErr
    }, nil
}
```

**Uniqueness invariant:** `mcpProxyNamePrefix + randShortID(8)` guarantees concurrent AWF processes don't collide on a single shared server name. The cleanup closure captures `name`, so it removes only its own registration.

#### Pattern C: Workspace config file with flock (OpenCode)

Use when the CLI has neither a per-invocation `--mcp-config` flag nor a scriptable `mcp add` command, but reads `./opencode.json` (or equivalent) from the working directory at startup. Write our server entry into the workspace config, take an `LOCK_EX` flock on a sidecar file so concurrent AWF processes serialize their read-modify-write cycles, and delete the file in cleanup when we created it from scratch.

```go
func (p *OpenCodeProvider) opencodeMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig,
    mcpConfigPath string, options map[string]any) (
    newArgs []string, newOptions map[string]any, cleanup func() error, err error) {

    if cfg == nil {
        return args, options, noopMCPCleanup, nil
    }
    workspaceDir, wdErr := os.Getwd()
    if wdErr != nil {
        return nil, options, noopMCPCleanup, fmt.Errorf("opencode mcp: getwd: %w", wdErr)
    }
    name := mcpProxyNamePrefix + randShortID(8)
    addCleanup, addErr := addOpenCodeMCPServer(workspaceDir, name, mcpServeCommand(mcpConfigPath))
    if addErr != nil {
        return nil, options, noopMCPCleanup, addErr
    }
    newArgs = make([]string, len(args))
    copy(newArgs, args)
    return newArgs, options, addCleanup, nil
}
```

**Implementation details** are factored into `addOpenCodeMCPServer` (`opencode_workspace_config.go`):
- Lock target lives in `os.TempDir()` keyed by `sha256(workspaceDir)[:8]` so the workspace stays free of sidecar files.
- Atomic write: marshal in memory → write to `*.tmp` → `os.Rename` over the final path.
- Cleanup is idempotent via `sync.Once`, removes only the named entry, and deletes the workspace file when we created it from scratch (even if the CLI itself later annotated the file with `$schema` or similar).

#### Pattern D: Inline `-c key=value` config flags (Codex)

Use when the CLI exposes an inline config-injection flag (`-c <key>=<value>`) that maps to the CLI's internal config schema. No external file is needed; the MCP server config is encoded directly in the argv. Cleanup is a no-op.

```go
func (p *CodexProvider) codexMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig,
    mcpConfigPath string, options map[string]any) (
    newArgs []string, newOptions map[string]any, cleanup func() error, err error) {

    if cfg == nil {
        return args, options, noopMCPCleanup, nil
    }
    exe := resolvedExecutable()
    commandArg := fmt.Sprintf("mcp_servers.awf-proxy.command=%q", exe)
    argsArg := fmt.Sprintf(`mcp_servers.awf-proxy.args=["mcp-serve", "--config=%s"]`, mcpConfigPath)

    newArgs = make([]string, len(args), len(args)+6)
    copy(newArgs, args)
    newArgs = append(newArgs, "-c", commandArg, "-c", argsArg)
    return newArgs, options, noopMCPCleanup, nil
}
```

#### Pattern E: Temporary CLI home/config via environment override (Mistral Vibe)

Use when the CLI has no per-run MCP flag and no reliable `mcp add` subcommand, but reads a user-level config file from a home directory controlled by an environment variable. Mistral Vibe is the reference implementation: AWF creates a temporary `VIBE_HOME`, writes `config.toml` containing an `[[mcp_servers]]` stdio entry, copies secrets such as `.env`, launches `vibe` with `VIBE_HOME=<tmp>`, and deletes the temporary home in cleanup.

This pattern needs two pieces:

1. A provider environment overlay. `baseCLIProvider` reads an internal environment option and calls `ExecCLIExecutor.RunWithEnv`. Do not shell-prefix the command with `env`; keep execution direct so argument escaping and process-group cancellation stay consistent.
2. A sanitized config copy. If the user's existing config is copied, remove any existing `mcp_servers` table or assignment before appending AWF's entry. TOML does not allow mutating a namespace after it has been defined in an incompatible form.

```go
func (p *MyProviderProvider) myProviderMCPInjector(_ context.Context, args []string, cfg *workflow.MCPProxyConfig,
    mcpConfigPath string, options map[string]any) (
    newArgs []string, newOptions map[string]any, cleanup func() error, err error) {

    if cfg == nil {
        return args, options, noopMCPCleanup, nil
    }

    cliHome, cleanup, err := writeMyProviderMCPHome(mcpConfigPath)
    if err != nil {
        return nil, options, noopMCPCleanup, err
    }

    newArgs = make([]string, len(args), len(args)+8)
    copy(newArgs, args)
    for _, toolName := range myProviderAllowedMCPTools(cfg) {
        newArgs = append(newArgs, "--enabled-tools", toolName)
    }

    newOpts := make(map[string]any, len(options)+1)
    for k, v := range options {
        newOpts[k] = v
    }
    newOpts[cliProviderEnvOptionKey] = map[string]string{"MY_PROVIDER_HOME": cliHome}

    return newArgs, newOpts, cleanup, nil
}
```

**Tool names may be provider-specific.** Vibe publishes MCP tools as `<server-alias>_<remote-tool-name>`. AWF's plugin tools are already named `<plugin>_<operation>`, so a Vibe workflow with server alias `awf-proxy` must call `awf-proxy_awf-plugin-time_time`, not `awf-plugin-time_time`. Document and test the provider's published names with a real plugin-tool workflow.

**Non-interactive permission mode matters.** Some CLIs discover MCP tools but still refuse to execute them unless an approval callback or auto-approval profile is active. For Mistral Vibe, real non-interactive MCP workflows require `agent_profile: auto-approve` and `trust: true`; otherwise the model may return `Tool execution not permitted` even though MCP injection is correct. Add a provider-specific workflow fixture covering this path.

### HTTP providers

`OpenAICompatibleProvider` intentionally **does not** implement `mcpInjector`. MCP tools are delivered in-process via `ports.ToolRouter` and injected as `tools[]` in the Chat Completions request payload. See [Non-CLI Provider (HTTP API)](#non-cli-provider-http-api) for details. The absence of an `mcpInjector` on the HTTP provider is the documented path, not a missing implementation.

### Coexistence mode

Codex and OpenCode CLIs cannot fully disable their native built-in tools — they have no `--tools ""` equivalent. When users request `intercept_builtins: true` on these providers, the injector runs in **coexistence mode**:

1. Emit a startup `WARN` log so operators see that strict isolation is impossible:
   ```go
   p.logger.Warn("mcp_proxy on provider=opencode runs in coexistence mode; built-in tools are not blocked")
   ```
2. Prefix the user's `system_prompt` (or set it if empty) to steer the model toward MCP:
   ```go
   newOpts := make(map[string]any, len(options)+1)
   for k, v := range options {
       newOpts[k] = v
   }
   const mcpOnlyPrefix = "Use only MCP tools, never built-in tools. "
   existing, _ := getStringOption(newOpts, "system_prompt")
   newOpts["system_prompt"] = mcpOnlyPrefix + existing
   return newArgs, newOpts, cleanupFn, nil
   ```
3. Document the limitation in your provider's row of [docs/user-guide/mcp-proxy.md](../user-guide/mcp-proxy.md) "Supported Providers" table.

Apply this pattern only when `cfg.InterceptBuiltins == true`. For `intercept_builtins: false` (additive mode), no system-prompt mutation is needed — both native and MCP tools are intentionally exposed.

### Common helpers

Defined in the `agents` package; reuse instead of reinventing:

| Helper | Purpose |
|--------|---------|
| `mcpProxyNamePrefix` | Constant `"awf-proxy-"`. Use as the namespace prefix for any persistent CLI registration so the purge routine can find orphans from crashed prior runs. |
| `randShortID(n int) string` | Crypto-random hex (length `2*n`). Use `randShortID(8)` to generate a 16-char suffix unique enough to prevent concurrent-run collisions. |
| `mcpServeCommand(configPath string) []string` | Returns `[<resolved-awf-bin>, "mcp-serve", "--config=" + configPath]` — the exact argv to invoke the local MCP server. |
| `resolvedExecutable() string` | Symlink-resolved absolute path to the current AWF binary, cached after first call. Use whenever you must capture a stable path for the MCP server child process. |
| `cliProviderEnvOptionKey` | Internal options-map key used by provider injectors that need per-run environment overrides (Pattern E). Set it to `map[string]string`; the base layer will call `RunWithEnv`. |
| `noopMCPCleanup() error { return nil }` | Default cleanup for nil-config or no-side-effect injectors. |

### Wiring the hook

Plug the injector into `cliProviderHooks` in `newBase()`:

```go
func (p *MyProviderProvider) newBase() *baseCLIProvider {
    return newBaseCLIProvider("myprovider", "myprovider-cli", p.executor, p.logger, cliProviderHooks{
        buildExecuteArgs:      p.buildExecuteArgs,
        buildConversationArgs: p.buildConversationArgs,
        extractSessionID:      p.extractSessionID,
        // ...
        mcpInjector: p.myproviderMCPInjector,
    })
}
```

### Tests to write

- **Nil-config nil-effect.** `cfg == nil` returns the input args unchanged, an unchanged options map, `noopMCPCleanup`, and no error. No side effects (no sub-process spawn, no file write).
- **Happy path with `InterceptBuiltins=false`.** Injector produces correct args and a working cleanup. For Pattern B/C, assert that the registered name matches `mcpProxyNameRE` (`^awf-proxy-[0-9a-f]{16}$`).
- **`InterceptBuiltins=true` behavior.** Strict-mode flags / coexistence warning / system_prompt mutation as applicable.
- **Cleanup idempotency.** Second call returns nil and performs no additional side effects (verify via mock executor call count or file inode timestamp).
- **Cleanup name consistency** (Pattern B/C). The name passed to "remove" equals the name passed to "add", proving each run owns exactly one registration and never touches another.
- **Concurrency** (Pattern C). N goroutines each adding a uniquely-named entry → all entries present; N cleanups → file/state restored to pre-test condition.

Reference test files: `claude_provider_mcp_test.go`, `gemini_provider_mcp_test.go`, `codex_provider_mcp_test.go`, `opencode_provider_mcp_test.go`, `opencode_workspace_config_test.go`, `mistral_vibe_provider_unit_test.go`.

## Existing Providers Reference

| Provider | Binary | Name | Session Event | Session Field | Resume Flag | System Prompt |
|----------|--------|------|---------------|---------------|-------------|---------------|
| Claude | `claude` | `claude` | `result` | `session_id` | `-r ID` | `--system-prompt` (native) |
| Gemini | `gemini` | `gemini` | `init` | `session_id` | `--resume ID` | Inlined in first turn |
| Codex | `codex` | `codex` | `thread.started` | `thread_id` | `resume ID` (subcommand) | Inlined in first turn |
| Copilot | `copilot` | `github_copilot` | `result` | `sessionId` (camelCase) | `--resume=ID` | Inlined in first turn |
| OpenCode | `opencode` | `opencode` | `step_start` | `sessionID` | `-s ID` / `-c` (fallback) | Inlined in first turn |
| Mistral Vibe | `vibe` | `mistral_vibe` | unsupported | N/A | unsupported | Inlined in first turn |
| OpenAI-Compatible | HTTP API | `openai_compatible` | API response | N/A | Messages array | `system` role message |

### MCP proxy support per provider

| Provider | mcpInjector | Pattern | Strict isolation? |
|----------|-------------|---------|-------------------|
| Claude | `claudeMCPInjector` | Wrapper file + `--mcp-config` (Pattern A) | Yes (`--tools "" --strict-mcp-config`) |
| Gemini | `geminiMCPInjector` | `gemini mcp add` subcommand (Pattern B) | Yes (`--allowed-mcp-server-names` + `--policy`) |
| Codex | `codexMCPInjector` | Inline `-c mcp_servers.*` (Pattern D) | Coexistence only (`-s read-only` best-effort) |
| Copilot | `copilotMCPInjector` | Wrapper file + `--additional-mcp-config @<file>` (Pattern A variant) | Coexistence only (`--disable-builtin-mcps` best-effort) |
| OpenCode | `opencodeMCPInjector` | Workspace `./opencode.json` (Pattern C) | Coexistence only |
| Mistral Vibe | `mistralVibeMCPInjector` | Temporary `VIBE_HOME/config.toml` (Pattern E) | Tool allowlist + auto-approve/trust required for non-interactive runs |
| OpenAI-Compatible | _intentional no-op_ | In-process `ToolRouter` + HTTP `tools[]` | Yes |

## Non-CLI Provider (HTTP API)

`OpenAICompatibleProvider` follows a completely different path from CLI-based providers. It implements `AgentProvider` **directly** without using `baseCLIProvider`, hooks, or any of the CLI infrastructure.

### What changes vs CLI providers

| Aspect | CLI providers | HTTP provider (OpenAI-Compatible) |
|--------|--------------|----------------------------------|
| Execution | `CLIExecutor.Run()` → binary subprocess | `httpx.Client` → HTTP POST to `/chat/completions` |
| Token counting | `ports.Tokenizer` → estimation (`len/4`), `TokensEstimated: true` | API response `usage` field → exact counts, `TokensEstimated: false` |
| Session management | Extract session ID from NDJSON, resume via CLI flag | No session ID — full messages array sent each turn |
| System prompt | Inlined in first turn or native CLI flag | `system` role message in messages array |
| Display events | NDJSON stream filtering via `DisplayEventParser` | Direct write to stdout, no parsing needed |
| State cloning | Done by `baseCLIProvider.executeConversation()` | Must call `cloneState()` manually |
| Base struct | `base *baseCLIProvider` field | No base — flat struct with `httpClient *httpx.Client` |

### Token counting: the key difference

CLI providers estimate tokens because CLI tools don't report token usage:

```go
// CLI provider pattern — estimation
tokens, _ := p.base.tokenizer.CountTokens(extracted) //nolint:errcheck
result.Tokens = tokens
result.TokensEstimated = true // set by tokenizer.IsEstimate()
```

The HTTP provider gets exact counts from the API response:

```go
// HTTP provider pattern — exact counts from API
result.Tokens = resp.Usage.TotalTokens
result.TokensEstimated = false

// In ExecuteConversation, input/output are separated:
result.TokensInput = resp.Usage.PromptTokens
result.TokensOutput = resp.Usage.CompletionTokens
result.TokensTotal = resp.Usage.TotalTokens
```

No `Tokenizer` port is used. No `//nolint:errcheck` is needed.

### Conversation: messages array vs session resume

CLI providers maintain a session ID and pass it as a CLI flag to resume:

```go
// CLI: resume with session ID
args = []string{"--resume", state.SessionID, "-p", prompt}
```

The HTTP provider reconstructs the full messages array from conversation state on every turn:

```go
// HTTP: rebuild messages from turns
messages := make([]chatMessage, 0, len(state.Turns)+2)
if opts.systemPrompt != "" {
    messages = append(messages, chatMessage{Role: "system", Content: opts.systemPrompt})
}
for _, turn := range state.Turns {
    messages = append(messages, chatMessage{Role: string(turn.Role), Content: turn.Content})
}
messages = append(messages, chatMessage{Role: "user", Content: prompt})
```

### Struct and constructor

```go
type OpenAICompatibleProvider struct {
    httpClient *httpx.Client // no base, no logger, no executor, no tokenizer
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
```

### Option handling

Options are parsed into a dedicated `parsedOptions` struct with env var fallbacks:

```go
type parsedOptions struct {
    baseURL             string   // required — env: OPENAI_BASE_URL
    model               string   // required — env: OPENAI_MODEL
    apiKey              string   // optional — env: OPENAI_API_KEY
    systemPrompt        string
    temperature         *float64 // 0.0–2.0
    maxCompletionTokens *int
    topP                *float64 // 0.0–1.0
}
```

### When to use this pattern

Use the HTTP provider pattern (not `baseCLIProvider`) when:
- Your provider is an HTTP API, not a CLI binary
- The API returns exact token counts in its response
- Conversation is managed via a messages array, not session IDs
- There is no NDJSON stream to parse

Use `OpenAICompatibleProvider` as your reference implementation.

## Checklist

### Structure
- [ ] Provider struct with `base`, `logger`, `executor`, `tokenizer` fields
- [ ] `NewXxxProvider()` zero-config constructor
- [ ] `NewXxxProviderWithOptions()` functional-options constructor
- [ ] `newBase()` called **after** options, wires all hooks, forwards tokenizer with nil-check
- [ ] Option types added to `options.go` (`WithXxxExecutor`, `WithXxxTokenizer`, `WithXxxLogger`)
- [ ] Provider registered in `registry.go` `RegisterDefaults()`

### Hooks (required)
- [ ] `buildExecuteArgs` forces structured output format (JSON/NDJSON)
- [ ] `buildConversationArgs` handles first turn vs session resume
- [ ] `extractSessionID` parses session ID from provider-specific event

### Cross-provider options (mandatory)
- [ ] `model` handled in both `buildExecuteArgs` and `buildConversationArgs`
- [ ] `model` validated in `validateOptions` (prefix check or allowlist)
- [ ] `dangerously_skip_permissions` mapped to CLI-specific flag (or logged + ignored if unsupported)
- [ ] `system_prompt` handled via native flag or `buildFirstTurnPrompt()` on first turn only
- [ ] `output_format` checked in `Execute()` to conditionally expose `result.Response`
- [ ] Unknown options silently ignored (never iterate to reject)
- [ ] All options applied in **both** `buildExecuteArgs` and `buildConversationArgs`

### Execute post-processing
- [ ] Text extracted from NDJSON via `extractDisplayTextFromEvents` or `extractTextContent`
- [ ] Tokens re-counted on extracted text (not raw output)
- [ ] `//nolint:errcheck` with explanatory comment on every `CountTokens` call
- [ ] JSON response parsed when `output_format` is `json` or `stream-json`

### Interface methods
- [ ] `Execute` delegates to `p.base.execute()` with post-processing
- [ ] `ExecuteConversation` delegates to `p.base.executeConversation()`
- [ ] `Name()` returns unique provider identifier (matches `provider:` YAML field)
- [ ] `Validate()` checks binary via `exec.LookPath` with `%w` error wrapping

### Display events
- [ ] `parseDisplayEvents` handles text events (`EventText`) and tool events (`EventToolUse`)
- [ ] NUL bytes sanitized before `json.Unmarshal`
- [ ] Unknown/malformed events return `nil` (never error)

### Token usage (if CLI emits exact counts)
- [ ] `extractTokenUsage` returns `*tokenUsage` from the CLI's structured token-stats event
- [ ] Returns `nil` when the event is absent so the base layer falls back to estimation
- [ ] No `//nolint:errcheck` needed — exact counts mean `TokensEstimated` is cleared automatically

### MCP proxy (if supported)
- [ ] `mcpInjector` wired in `cliProviderHooks` via `newBase()`
- [ ] Nil-config short-circuit: `cfg == nil` returns `(args, options, noopMCPCleanup, nil)` with no side effects
- [ ] `newArgs` is always a fresh slice (input `args` never mutated)
- [ ] Unique server name via `mcpProxyNamePrefix + randShortID(8)` for any persistent registration (Patterns B/C)
- [ ] Cleanup closure is idempotent (`sync.Once`) and uses `context.Background()` so it survives parent cancellation
- [ ] Cleanup removes only this run's registration — never touches entries from concurrent AWF processes
- [ ] When `cfg.InterceptBuiltins == true` and the CLI cannot disable native tools: emit a coexistence `WARN` log AND prefix `system_prompt` with `"Use only MCP tools, never built-in tools. "` (cloned options map)
- [ ] When `cfg.InterceptBuiltins == true` and the CLI CAN disable natives: append the appropriate strict-mode flag (`--strict-mcp-config`, `--allowed-mcp-server-names`, etc.)
- [ ] If the CLI discovers MCP from a config file, verify whether config is per-run, workspace-global, or user-global; never mutate user-global config without a cleanup path
- [ ] If using an environment override (Pattern E), copy secrets/config selectively, sanitize existing MCP entries before appending AWF's server, set `cliProviderEnvOptionKey`, and test cleanup removes the temporary home
- [ ] Verify the provider's published MCP tool names; if it prefixes tools with the server alias, update workflow fixtures and docs to use the published name
- [ ] Verify non-interactive permission behavior with a real plugin tool; add required provider options such as auto-approve/trust to fixtures
- [ ] Provider row added to "Supported Providers" table in `docs/user-guide/mcp-proxy.md`
- [ ] Provider row added to "MCP proxy support per provider" table in this document

### Tests
- [ ] Option injection tests (`TestWithXxxTokenizer`, `TestWithXxxExecutor`)
- [ ] `buildExecuteArgs` table-driven tests (basic, with model, with permissions)
- [ ] `buildConversationArgs` tests (first turn with system_prompt, resume with session ID)
- [ ] `extractSessionID` tests (valid, missing event, empty output)
- [ ] `parseDisplayEvents` tests (text, tool, unknown, invalid JSON)
- [ ] `validateOptions` tests (nil, valid, invalid model, unknown option)
- [ ] `extractTokenUsage` tests (valid event, missing event, malformed stats) — if hook is set
- [ ] `mcpInjector` tests — if hook is set:
  - [ ] Nil-config short-circuit (no side effects)
  - [ ] Happy path with `InterceptBuiltins=false`
  - [ ] `InterceptBuiltins=true` flags / coexistence WARN / system_prompt mutation
  - [ ] Cleanup idempotency (second call is no-op)
  - [ ] Cleanup name consistency (Pattern B/C: remove uses same name as add)
  - [ ] Concurrent safety with `errgroup` (Pattern C: N parallel adds + N parallel cleanups)
  - [ ] Config sanitization and environment overlay (Pattern E)
- [ ] End-to-end workflow: from a clean state (no leftover config / registration), `awf run test-mcp-proxy-<provider>-plugin-tools` must complete with status `success` AND leave no orphan files / registrations behind

### Final verification
- [ ] `make build` passes
- [ ] `make lint` passes with zero violations
- [ ] `make test` passes
- [ ] `grep -rn "dangerously_skip_permissions" your_provider.go` returns at least one match
- [ ] `grep -rn "mcpInjector" your_provider.go` returns a match if MCP is supported, OR a comment in the file explaining why it is intentionally omitted (HTTP path / unsupported CLI)
