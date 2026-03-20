---
title: "Agent Conversation Mode"
---

Enable multi-turn conversations with AI agents, featuring automatic context window management, token counting, and stop conditions.

## Overview

While [agent steps](agent-steps.md) invoke agents once per step, **conversation mode** maintains conversation history across multiple turns within a single step. This allows iterative refinement, clarification loops, and complex reasoning without chaining multiple workflow steps.

**Key features:**
- **Automatic turn management** - Maintain conversation history without explicit state passing
- **Context window handling** - Automatically truncate old turns when reaching token limits
- **Stop conditions** - Exit conversation early when a condition is met
- **Token tracking** - Monitor input/output token usage across turns
- **System prompt preservation** - Protect system instructions during truncation

## When to Use

Use conversation mode when:
- **Iterative refinement** — Multiple rounds of feedback on generated content
- **Clarification loops** — Agent asks questions, workflow provides answers
- **Complex reasoning** — Chain-of-thought across multiple turns
- **Interactive workflows** — Step-by-step problem solving with the agent

For simple single-turn interactions, use standard [agent steps](agent-steps.md) instead.

## Basic Syntax

```yaml
name: code-review-conversation
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true

states:
  initial: refine_code

  refine_code:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a code reviewer. Iterate on the code until it meets quality standards.
      Say "APPROVED" when done.
    initial_prompt: |
      Review this code:
      {{.inputs.code}}
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 4096
    conversation:
      max_turns: 10
      max_context_tokens: 100000
      strategy: sliding_window
      stop_condition: "response contains 'APPROVED'"
    on_success: done

  done:
    type: terminal
```

## Configuration

### Step-Level Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `type` | string | Yes | — | Must be `agent` |
| `mode` | string | No | `step` | Set to `conversation` to enable multi-turn mode |
| `provider` | string | Yes | — | Agent provider: `claude`, `codex`, `gemini`, `opencode`, `openai_compatible` |
| `system_prompt` | string | No | — | System message for the entire conversation (preserved during truncation) |
| `initial_prompt` | string | Yes | — | Initial user message to start the conversation |
| `prompt` | string | No | — | Used when injecting context mid-conversation (see [Injecting Context](#injecting-context)) |
| `options` | object | No | — | Provider-specific options (model, max_tokens, temperature, etc.) |
| `timeout` | duration | No | `300s` | Timeout for each turn |
| `on_success` | string | Yes | — | Next state on successful completion |
| `on_failure` | string | No | — | Next state on error |

### Conversation Configuration

```yaml
conversation:
  max_turns: 10                    # Maximum number of turns (default: 10)
  max_context_tokens: 100000       # Token budget for conversation (default: model limit)
  strategy: sliding_window         # Context window strategy (only sliding_window supported)
  stop_condition: "expression"     # Exit condition (optional)
```

#### max_turns

Maximum number of conversation turns before automatic termination.

```yaml
conversation:
  max_turns: 5  # Conversation stops after 5 turns
```

Useful for preventing runaway conversations and controlling costs.

#### max_context_tokens

Token budget for the entire conversation. When approaching this limit, context window strategy applies.

```yaml
conversation:
  max_context_tokens: 100000  # Limit total tokens to 100k
```

Prevents exceeding model token limits. When exceeded, old turns are dropped using the configured strategy.

#### strategy

Context window strategy when token limit is reached:

- **`sliding_window`** - Drop oldest turns, preserving system prompt and most recent context. If omitted, no context management is applied.

```yaml
strategy: sliding_window
# Conversation with 5 turns reaches token limit
# Drop: Turn 1, Keep: System prompt, Turn 2, Turn 3, Turn 4, Turn 5
```

Future strategies: `summarize` (compress old turns), `truncate_middle` (keep first and last turns). These strategies are validated but not yet implemented — using them will produce a validation error suggesting `sliding_window` instead.

#### stop_condition

Expression to evaluate after each turn. When true, conversation exits.

```yaml
stop_condition: "response contains 'APPROVED'"
```

**Expression Syntax**: Supports comparison operators and string functions:
- `response contains 'text'` — Check if response contains substring
- `response matches 'regex'` — Match against regex pattern
- `turn_count >= 5` — Check number of turns
- `tokens_used > 50000` — Check token consumption

See [Stop Condition Expressions](#stop-condition-expressions) for examples.

## Accessing Conversation State

Conversation state is stored in the step state and accessible in subsequent steps:

```yaml
analyze_conversation:
  type: agent
  provider: claude
  mode: conversation
  initial_prompt: "Start conversation"
  on_success: review_conversation

review_conversation:
  type: step
  command: |
    echo "Conversation lasted {{.states.analyze_conversation.conversation.total_turns}} turns"
    echo "Total tokens: {{.states.analyze_conversation.conversation.total_tokens}}"
  on_success: done
```

### Conversation State Structure

> **Note:** The `role` field is read-only execution metadata assigned by AWF during execution. `system_prompt` maps to `system`, `initial_prompt`/`prompt` to `user`, and CLI responses to `assistant`. You do not write roles in workflow YAML — they appear only in the runtime state output.

```yaml
states:
  analyze_conversation:
    output: "Final response text"
    conversation:
      turns:
        - role: system
          content: "You are a code reviewer..."
          tokens: 50
        - role: user
          content: "Review this code..."
          tokens: 500
        - role: assistant
          content: "I found these issues..."
          tokens: 800
        - role: user
          content: "Fix the issues"
          tokens: 20
        - role: assistant
          content: "Here's the fixed code... APPROVED"
          tokens: 600
      total_turns: 5
      total_tokens: 1970
      stopped_by: "condition"  # or "max_turns", "max_tokens"
```

## Stop Condition Expressions

Exit conversations early with programmatic conditions.

### String Matching

```yaml
# Exit when response contains exact text
stop_condition: "response contains 'DONE'"

# Exit when response matches pattern
stop_condition: "response matches 'APPROVED|COMPLETE'"

# Case-insensitive matching
stop_condition: "response contains 'done' || response contains 'finished'"
```

### Token-Based Conditions

```yaml
# Exit when token count reaches threshold
stop_condition: "tokens_used > 80000"

# Exit when approaching context limit
stop_condition: "tokens_used > max_context_tokens * 0.9"
```

### Turn-Based Conditions

```yaml
# Exit after specific number of turns
stop_condition: "turn_count >= 5"

# Complex: exit after 5 turns OR if done
stop_condition: "turn_count >= 5 || response contains 'APPROVED'"
```

### Advanced Examples

```yaml
# Code review: approve after syntax fixes
stop_condition: "response contains 'All issues fixed' && turn_count >= 2"

# Research loop: gather enough sources
stop_condition: "response contains 'sufficient sources' || turn_count >= 10"

# Conversation: stop on topic completion or token budget
stop_condition: "response contains 'Summary:' || tokens_used > 90000"
```

## Injecting Context Mid-Conversation (Planned)

> **Not yet implemented** — `continue_from` and `inject_context` are parsed but rejected at validation. See [Limitations](#limitations) for details.
>
> **Dependencies:** `continue_from` requires session resume support (F073) to function — it hands off the session ID between steps rather than re-serializing history. `inject_context` is a separate concept (context enrichment mid-conversation) with its own design requirements.

The following example shows the planned behavior for resuming a conversation from a previous step:

```yaml
states:
  initial: refine_code

  refine_code:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: "You are a code reviewer."
    initial_prompt: |
      Review this code:
      {{.inputs.code}}
    conversation:
      max_turns: 5
    on_success: add_requirements

  # Planned: continue previous conversation
  add_requirements:
    type: agent
    provider: claude
    mode: conversation
    continue_from: refine_code  # Not yet implemented — will error at validation
    prompt: |
      Also consider these requirements:
      {{.inputs.additional_requirements}}
    conversation:
      max_turns: 3
    on_success: done

  done:
    type: terminal
```

When implemented, the `continue_from` field will resume the previous step's session using the stored `SessionID`, enabling native context continuity without history re-serialization.

## Examples

### Iterative Code Review

```yaml
name: iterative-code-review
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true
  - name: requirements
    type: string
    required: true

states:
  initial: review

  review:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are an expert code reviewer. Help improve the code quality step by step.
      After each suggestion, wait for the user's response or revision.
      Say "Code review complete!" when satisfied.
    initial_prompt: |
      Review this code and suggest the first improvement:

      {{.inputs.code}}

      Requirements to consider:
      {{.inputs.requirements}}
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 2048
    conversation:
      max_turns: 10
      max_context_tokens: 100000
      strategy: sliding_window
      stop_condition: "response contains 'Code review complete!'"
    on_success: done

  done:
    type: terminal
```

### Multi-Stage Problem Solving

```yaml
name: problem-solver
version: "1.0.0"

inputs:
  - name: problem
    type: string
    required: true

states:
  initial: analyze

  analyze:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a problem-solving expert. Work through problems systematically.
      Use the following stages:
      1. ANALYZE: Break down the problem
      2. PLAN: Outline approach
      3. IMPLEMENT: Provide solution
      4. VERIFY: Check solution

      End with "COMPLETE" when done.
    initial_prompt: |
      {{.inputs.problem}}
    conversation:
      max_turns: 8
      max_context_tokens: 50000
      stop_condition: "response contains 'COMPLETE'"
    on_success: done

  done:
    type: terminal
```

### Document Refinement Loop

```yaml
name: document-refinement
version: "1.0.0"

inputs:
  - name: document
    type: string
    required: true

states:
  initial: refine

  refine:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a professional editor. Improve the document iteratively.
      Respond with the refined version and ask what specific improvements to focus on next.
    initial_prompt: |
      {{.inputs.document}}
    conversation:
      max_turns: 5
      max_context_tokens: 80000
      stop_condition: "turn_count >= 3"
    on_success: summary

  summary:
    type: step
    command: |
      echo "Refinement complete."
      echo "Turns: {{.states.refine.conversation.total_turns}}"
      echo "Tokens: {{.states.refine.conversation.total_tokens}}"
    on_success: done

  done:
    type: terminal
```

## Best Practices

### 1. Set Reasonable Turn Limits

Always set `max_turns` to prevent runaway conversations:

```yaml
conversation:
  max_turns: 10  # Prevent infinite loops
```

### 2. Define Clear Stop Conditions

Use specific, unambiguous conditions:

```yaml
# ✅ Good: Specific completion signal
stop_condition: "response contains 'APPROVED'"

# ❌ Vague: Could match unintended text
stop_condition: "response contains 'done'"
```

### 3. Monitor Token Usage

Set appropriate `max_context_tokens`:

```yaml
conversation:
  max_context_tokens: 100000  # Model limit for Claude 3 Sonnet
```

Check token usage in subsequent steps:

```yaml
log_tokens:
  type: step
  command: echo "Tokens used: {{.states.analyze.conversation.total_tokens}}"
  on_success: done
```

### 4. Use System Prompt Effectively

System prompt guides the agent's behavior across all turns. For CLI providers, the `--system-prompt` flag is passed on the first conversation turn. On resumed turns (2+), the provider retains the system prompt from the session.

```yaml
system_prompt: |
  You are a code reviewer.
  Focus on security, performance, and readability.
  Keep responses concise.
  Use JSON format for structured feedback.
```

### 5. Test Stop Conditions

Verify stop conditions work as expected:

```bash
awf run workflow --dry-run
# Review the prompt and stop condition
```

### 6. Handle Errors Gracefully

Add error handling for conversation failures:

```yaml
refine:
  type: agent
  mode: conversation
  initial_prompt: "Review this code"
  on_success: done
  on_failure: error
  timeout: 120

error:
  type: terminal
  status: failure
```

## Troubleshooting

### Conversation Runs Longer Than Expected

**Problem**: Conversation continues past expected point

**Solutions**:
- Review stop condition: `awf run workflow --dry-run`
- Lower `max_turns` if using as fallback
- Make stop condition more specific (avoid ambiguous phrases)
- Check provider CLI is matching expected output

### Token Count Exceeds Limit

**Problem**: Context window strategy truncates important turns

**Solutions**:
- Increase `max_context_tokens` if model supports it
- Reduce initial prompt size
- Use shorter system prompt
- Lower `max_turns` to reduce conversation length

### Conversation Fails After First Turn

**Problem**: Error "prompt cannot be empty" on second turn

**Solution**: This was a bug in versions prior to F051 (fixed in v0.1.0+). The implementation incorrectly set prompts to empty strings for subsequent conversation turns.

**Workaround** (if on older version):
- Upgrade to latest version with `go install github.com/awf-project/cli/cmd/awf@latest`
- Or use single-turn agent steps with explicit state chaining instead

**Fixed in**: F051 (See CHANGELOG.md for details)

### CLI Providers Execute Turns Independently

**Problem**: Agent doesn't maintain history across turns with CLI providers

**Explanation**: CLI-based providers (`claude`, `codex`, `gemini`, `opencode`) execute each conversation turn as an independent process invocation. AWF passes only the current turn's prompt via `-p` (not serialized history). With session resume support (F073), providers use native `--resume` flags to maintain context across turns. Without session resume, each turn starts fresh with no context from previous turns. This means:
- Each turn starts a fresh CLI process
- Context continuity relies on native session resume via provider-specific flags (`-r`, `resume`, `--resume`, `-s`)
- If session ID extraction fails, the provider falls back to stateless mode (each turn independent)

**Solution**: CLI providers now support session resume natively. If you need HTTP-based multi-turn conversation with full message history, use the `openai_compatible` provider, which sends the complete message history via the Chat Completions API:

```yaml
refine:
  type: agent
  provider: openai_compatible
  mode: conversation
  options:
    base_url: https://api.openai.com/v1
    model: gpt-4
  initial_prompt: "Review this code"
  conversation:
    max_turns: 10
    strategy: sliding_window
  on_success: done
```

## Limitations

### Provider Compatibility

All providers support conversation mode with context continuity:

- **`openai_compatible`** maintains full conversation history via the Chat Completions API (messages array).
- **CLI providers** (`claude`, `codex`, `gemini`, `opencode`) use native session resume flags (`-r`, `resume`, `--resume`, `-s`) to maintain context across turns. Session IDs are extracted from CLI output after the first turn and passed on subsequent turns. If session ID extraction fails, the provider falls back to stateless mode gracefully.

### Current Implementation

- Only `sliding_window` strategy implemented (dropping oldest turns); `summarize` and `truncate_middle` are rejected at validation
- `continue_from` and `inject_context` fields are parsed but not yet implemented; using them produces a validation error
- Stop conditions limited to string/token/turn expressions
- No branching conversations (single linear path)

### Future Enhancements

- `summarize` strategy (LLM-based compression of old turns)
- `truncate_middle` strategy (keep first and last turns)
- `continue_from` for resuming conversations across steps (unblocked by F073 session resume)
- `inject_context` for adding context mid-conversation
- Conversation branching (explore multiple paths)

## See Also

- [Agent Steps Guide](agent-steps.md) - Standard single-turn agent invocation
- [Workflow Syntax Reference](workflow-syntax.md#agent-state) - Complete agent step options
- [Template Variables](../reference/interpolation.md) - Available variables in prompts
- [Examples](examples.md) - More workflow examples
