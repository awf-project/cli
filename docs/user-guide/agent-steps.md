# Agent Steps Guide

Invoke AI agents (Claude, Codex, Gemini, OpenCode) in your workflows with structured prompts and response parsing.

## Overview

Agent steps allow you to integrate AI CLI tools into AWF workflows. Instead of shell commands, you define prompts as templates that get interpolated with workflow context and executed through provider-specific CLIs.

**Benefits:**
- Non-interactive: Suitable for CI/CD automation
- Stateless: Multi-turn conversations via state passing between steps
- Structured output: Automatic JSON parsing and token tracking
- Template interpolation: Access workflow context in prompts

## Basic Usage

```yaml
states:
  initial: analyze

  analyze:
    type: agent
    provider: claude
    prompt: "Analyze this code: {{.inputs.code}}"
    on_success: done

  done:
    type: terminal
```

```bash
awf run workflow --input code="$(cat main.py)"
```

## Supported Providers

### Claude (Anthropic)

Requires the `claude` CLI tool installed.

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Code review: {{.inputs.file_content}}"
  options:
    model: claude-sonnet-4-20250514
    max_tokens: 4096
    temperature: 0.7
  timeout: 120
  on_success: next
```

**Provider-Specific Options:**
- `model`: Claude model identifier
- `max_tokens`: Maximum response tokens
- `temperature`: Creativity level (0-1)

### Codex (OpenAI)

Requires the `codex` CLI tool installed.

```yaml
generate:
  type: agent
  provider: codex
  prompt: "Generate a function to: {{.inputs.requirement}}"
  options:
    max_tokens: 2048
    temperature: 0.8
  timeout: 60
  on_success: next
```

### Gemini (Google)

Requires the `gemini` CLI tool installed.

```yaml
summarize:
  type: agent
  provider: gemini
  prompt: "Summarize: {{.inputs.text}}"
  options:
    model: gemini-pro
  timeout: 60
  on_success: next
```

### OpenCode

Requires the `opencode` CLI tool installed.

```yaml
refactor:
  type: agent
  provider: opencode
  prompt: "Refactor this code for readability: {{.inputs.code}}"
  timeout: 120
  on_success: next
```

### Custom Provider

For unsupported AI CLIs, use the `custom` provider with a command template.

```yaml
my_ai:
  type: agent
  provider: custom
  command: "my-ai-tool --prompt={{prompt}} --json --timeout=30"
  prompt: "Analyze: {{.inputs.data}}"
  timeout: 60
  on_success: next
```

The `{{prompt}}` placeholder is replaced with the resolved prompt. **Security Warning**: The prompt is NOT automatically shell-escaped. To prevent command injection, use one of these approaches:

1. **Heredoc syntax** (recommended):
   ```yaml
   command: |
     my-ai-tool --json <<'EOF'
     {{prompt}}
     EOF
   ```

2. **Manual escaping**:
   ```yaml
   command: "my-ai-tool --prompt={{escape .prompt}} --json"
   ```

This allows you to integrate:
- Local LLMs (Ollama, LM Studio, etc.)
- Proprietary CLI tools
- Custom scripts or wrappers

## Prompt Templates

Prompts support full variable interpolation with access to workflow context:

```yaml
review:
  type: agent
  provider: claude
  prompt: |
    Review this code file:
    Path: {{.inputs.file_path}}
    Language: {{.inputs.language}}

    File content:
    {{.inputs.file_content}}

    Focus on:
    - Performance issues
    - Security vulnerabilities
    - Code style violations
  on_success: generate_report
```

**Available Variables:**
- `{{.inputs.*}}` - Workflow input values
- `{{.states.step_name.Output}}` - Previous step raw output
- `{{.states.step_name.Response}}` - Previous step parsed JSON
- `{{.env.VAR_NAME}}` - Environment variables
- `{{.workflow.id}}` - Workflow execution ID
- `{{.workflow.name}}` - Workflow name

See [Variable Interpolation Reference](../reference/interpolation.md) for complete details.

## Capturing Responses

Agent responses are automatically captured in the execution state:

| Field | Type | Description |
|-------|------|-------------|
| `{{.states.step_name.Output}}` | string | Raw response text from the agent |
| `{{.states.step_name.Response}}` | object | Parsed JSON response (if valid) |
| `{{.states.step_name.TokensUsed}}` | int | Tokens consumed by this step |
| `{{.states.step_name.ExitCode}}` | int | 0 for success, non-zero for failure |

### Accessing Raw Output

```yaml
report_results:
  type: step
  command: echo "Agent said: {{.states.analyze.Output}}"
  on_success: done
```

### Parsing JSON Responses

If an agent returns valid JSON, it's automatically parsed:

```yaml
# Agent returns: {"issues": ["bug1", "bug2"], "severity": "high"}

process_response:
  type: step
  command: echo "Found {{.states.analyze.Response.issues}} issues"
  on_success: done
```

## Multi-Turn Conversations

There are two approaches for multi-turn conversations:

### Chaining Steps (Manual State Passing)

For simple multi-turn workflows, chain agent steps with state passing:

```yaml
name: code-review-conversation
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true

states:
  initial: initial_review

  initial_review:
    type: agent
    provider: claude
    prompt: |
      Review this code for issues:
      {{.inputs.code}}
    on_success: ask_about_performance

  ask_about_performance:
    type: agent
    provider: claude
    prompt: |
      Based on your previous analysis:
      {{.states.initial_review.Output}}

      Can you elaborate on performance concerns?
    on_success: suggest_improvements

  suggest_improvements:
    type: agent
    provider: claude
    prompt: |
      Based on the previous discussion, suggest 3 specific improvements to:
      {{.inputs.code}}
    on_success: done

  done:
    type: terminal
```

Each step can reference previous agent outputs and build on the conversation without maintaining session state.

### Conversation Mode (Built-In Multi-Turn)

For iterative refinement within a single step, use **conversation mode** with automatic context window management:

```yaml
refine_code:
  type: agent
  provider: claude
  mode: conversation
  system_prompt: "You are a code reviewer. Iterate until code is approved."
  initial_prompt: |
    Review this code:
    {{.inputs.code}}
  conversation:
    max_turns: 10
    max_context_tokens: 100000
    stop_condition: "response contains 'APPROVED'"
  on_success: done
```

**Key differences:**
- **Automatic turn management** — No need to manually chain steps
- **Context window handling** — Automatically truncates old turns when token limit approached
- **Stop conditions** — Exit conversation early when specific condition met
- **Single step** — Simpler workflows for iterative refinement

See [Conversation Mode Guide](conversation-steps.md) for detailed documentation, examples, and best practices.

## Error Handling

Agent steps follow standard error handling:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  timeout: 120
  on_success: success_path
  on_failure: error_path
  retry:
    max_attempts: 3
    backoff: exponential
    initial_delay: 2s

success_path:
  type: terminal

error_path:
  type: terminal
  status: failure
```

### Common Error Scenarios

| Error | Cause | Solution |
|-------|-------|----------|
| Provider not found | CLI tool not installed | Install required CLI (e.g., `claude install`) |
| Timeout | Agent response took too long | Increase timeout or reduce prompt complexity |
| Invalid provider | Unsupported provider | Use `claude`, `codex`, `gemini`, `opencode`, or `custom` |
| Command failed | Provider CLI returned error | Check provider configuration and logs |

### Debugging

Use `--dry-run` to preview resolved prompts without execution:

```bash
awf run workflow --dry-run
# Shows: [DRY RUN] Agent: claude
# Prompt: <resolved prompt text>
```

## Parallel Agent Execution

Run multiple agents concurrently:

```yaml
parallel_analysis:
  type: parallel
  parallel:
    - claude_review
    - codex_suggest
  strategy: all_succeed
  on_success: aggregate

claude_review:
  type: agent
  provider: claude
  prompt: "Analyze for security: {{.inputs.code}}"

codex_suggest:
  type: agent
  provider: codex
  prompt: "Optimize performance: {{.inputs.code}}"

aggregate:
  type: step
  command: echo "Claude: {{.states.claude_review.Output}}\nCodex: {{.states.codex_suggest.Output}}"
  on_success: done
```

## Token Tracking

Some providers report token usage (useful for cost tracking):

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  options:
    model: claude-sonnet-4-20250514
  on_success: log_tokens

log_tokens:
  type: step
  command: echo "Tokens used: {{.states.analyze.TokensUsed}}"
  on_success: done
```

**Note**: All agent providers (Claude, Gemini, Codex) report token usage in the `TokensUsed` field.

## Best Practices

### 1. Keep Prompts Focused

Long, complex prompts may hit token limits or timeout. Break into multiple steps:

```yaml
# ❌ Too much
ask_everything:
  type: agent
  provider: claude
  prompt: |
    Review code for security, performance, style, and suggest
    improvements, then estimate refactoring effort...

# ✅ Better
security_review:
  type: agent
  provider: claude
  prompt: "Security review: {{.inputs.code}}"
  on_success: performance_review

performance_review:
  type: agent
  provider: claude
  prompt: |
    After this security review:
    {{.states.security_review.Output}}

    Now analyze performance: {{.inputs.code}}
  on_success: done
```

### 2. Use Consistent Formatting

Request structured output when relevant:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: |
    Analyze code and respond in JSON format:
    {
      "issues": [...],
      "severity": "high|medium|low",
      "estimate_hours": number
    }

    Code: {{.inputs.code}}
  on_success: process_response
```

### 3. Add Timeouts

Always set reasonable timeouts:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Review: {{.inputs.code}}"
  timeout: 120  # 2 minutes
  on_success: next
```

### 4. Test with Dry-Run

Preview prompts before running:

```bash
awf run my-workflow --dry-run --input file=/path/to/file
```

### 5. Handle Missing Providers

Test that required providers are installed:

```yaml
states:
  initial: check_claude

  check_claude:
    type: step
    command: which claude
    on_success: analyze
    on_failure: install_claude

  analyze:
    type: agent
    provider: claude
    prompt: "Review: {{.inputs.code}}"
    on_success: done

  install_claude:
    type: terminal
    status: failure

  done:
    type: terminal
```

## See Also

- [Workflow Syntax Reference](workflow-syntax.md#agent-state) - Complete agent step options
- [Template Variables](../reference/interpolation.md) - Available interpolation variables
- [Examples](examples.md) - More workflow examples
