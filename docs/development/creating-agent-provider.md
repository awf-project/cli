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
| OpenCode | Not supported (logged at debug level, silently ignored) |

If your CLI has no equivalent, log a debug message and ignore:

```go
if skip, ok := getBoolOption(options, "dangerously_skip_permissions"); ok && skip {
    p.logger.Debug("dangerously_skip_permissions is not supported by myprovider and will be ignored")
}
```

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
    sanitized := bytes.ReplaceAll(line, []byte{0x00}, []byte(` `))

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

## Existing Providers Reference

| Provider | Binary | Name | Session Event | Session Field | Resume Flag | System Prompt |
|----------|--------|------|---------------|---------------|-------------|---------------|
| Claude | `claude` | `claude` | `result` | `session_id` | `-r ID` | `--system-prompt` (native) |
| Gemini | `gemini` | `gemini` | `init` | `session_id` | `--resume ID` | Inlined in first turn |
| Codex | `codex` | `codex` | `thread.started` | `thread_id` | `resume ID` (subcommand) | Inlined in first turn |
| Copilot | `copilot` | `github_copilot` | `result` | `sessionId` (camelCase) | `--resume=ID` | Inlined in first turn |
| OpenCode | `opencode` | `opencode` | `step_start` | `sessionID` | `-s ID` / `-c` (fallback) | Inlined in first turn |
| OpenAI-Compatible | HTTP API | `openai_compatible` | API response | N/A | Messages array | `system` role message |

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

### Tests
- [ ] Option injection tests (`TestWithXxxTokenizer`, `TestWithXxxExecutor`)
- [ ] `buildExecuteArgs` table-driven tests (basic, with model, with permissions)
- [ ] `buildConversationArgs` tests (first turn with system_prompt, resume with session ID)
- [ ] `extractSessionID` tests (valid, missing event, empty output)
- [ ] `parseDisplayEvents` tests (text, tool, unknown, invalid JSON)
- [ ] `validateOptions` tests (nil, valid, invalid model, unknown option)

### Final verification
- [ ] `make build` passes
- [ ] `make lint` passes with zero violations
- [ ] `make test` passes
- [ ] `grep -rn "dangerously_skip_permissions" your_provider.go` returns at least one match
