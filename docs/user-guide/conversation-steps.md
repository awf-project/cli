# Agent Conversation Mode

Enable multi-turn agent execution with automatic stop conditions and token tracking.

## Overview

While [agent steps](agent-steps.md) invoke agents once per step, **conversation mode** executes the same prompt repeatedly until a stop condition is met. This is useful for:

- **Iterative generation** - Agent refines output over multiple turns
- **Autonomous reasoning** - Chain-of-thought across turns until completion signal
- **Controlled execution** - Stop after N turns or when specific output detected

> **Important**: Conversation mode does NOT support interactive back-and-forth with user input between turns. Each turn executes the same prompt. For interactive workflows, use multiple separate agent steps.

## Basic Syntax

```yaml
name: autonomous-review
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true

states:
  initial: review

  review:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a code reviewer. Iterate on improvements.
      Say "APPROVED" when the code meets quality standards.
    prompt: |
      Review this code:
      {{.inputs.code}}
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 4096
    conversation:
      max_turns: 10
      max_context_tokens: 100000
      strategy: sliding_window
      stop_condition: "inputs.response contains 'APPROVED'"
    on_success: done

  done:
    type: terminal
```

## Configuration

### Step-Level Options

| Option | Type | Required | Default | Description |
|--------|------|----------|---------|-------------|
| `type` | string | Yes | — | Must be `agent` |
| `mode` | string | No | `single` | Set to `conversation` for multi-turn |
| `provider` | string | Yes | — | Agent provider: `claude`, `codex`, `gemini`, `opencode` |
| `system_prompt` | string | No | — | System message preserved across turns |
| `prompt` | string | Yes | — | User message (executed each turn) |
| `options` | object | No | — | Provider-specific options |
| `timeout` | int | No | `300` | Timeout per turn in seconds |
| `on_success` | string | Yes | — | Next state on completion |
| `on_failure` | string | No | — | Next state on error |

### Conversation Configuration

```yaml
conversation:
  max_turns: 10                    # Maximum turns (default: 10)
  max_context_tokens: 100000       # Token budget (default: unlimited)
  strategy: sliding_window         # Only supported strategy
  stop_condition: "expression"     # Exit condition (optional)
```

#### max_turns

Maximum conversation turns before automatic termination. Default is 10.

```yaml
conversation:
  max_turns: 5  # Stop after 5 turns regardless of stop_condition
```

#### max_context_tokens

Token budget for the conversation. When exceeded, oldest turns are dropped (sliding window).

```yaml
conversation:
  max_context_tokens: 50000
```

#### strategy

Context window strategy when token limit is reached:

- **`sliding_window`** (only implemented) - Drop oldest turns, preserve system prompt

```yaml
strategy: sliding_window
```

> **Not Yet Implemented**: `summarize`, `truncate_middle`

#### stop_condition

Expression evaluated after each turn. When true, conversation exits early.

```yaml
stop_condition: "inputs.response contains 'APPROVED'"
```

## Stop Condition Expressions

Stop conditions use the expression evaluator with these variables:

| Variable | Type | Description |
|----------|------|-------------|
| `inputs.response` | string | Last assistant response |
| `inputs.turn_count` | int | Number of completed turns |

### Examples

```yaml
# Exit when response contains keyword
stop_condition: "inputs.response contains 'DONE'"

# Exit after N turns
stop_condition: "inputs.turn_count >= 5"

# Complex: exit on keyword OR turn limit
stop_condition: "inputs.response contains 'APPROVED' || inputs.turn_count >= 10"

# Multiple keywords
stop_condition: "inputs.response contains 'COMPLETE' || inputs.response contains 'FINISHED'"
```

> **Important**: Variables must be prefixed with `inputs.` (e.g., `inputs.response`, not just `response`).

## Accessing Conversation State

After execution, conversation state is available in step state:

```yaml
show_result:
  type: step
  command: |
    echo "Final response: {{.states.review.Output}}"
    echo "Turns: {{.states.review.Conversation.TotalTurns}}"
    echo "Tokens: {{.states.review.TokensUsed}}"
  on_success: done
```

### State Structure

```yaml
states:
  review:
    Output: "Final response text..."
    Status: completed
    Conversation:
      Turns:
        - Role: system
          Content: "You are a code reviewer..."
          Tokens: 50
        - Role: user
          Content: "Review this code..."
          Tokens: 500
        - Role: assistant
          Content: "I found issues... APPROVED"
          Tokens: 800
      TotalTurns: 3
      TotalTokens: 1350
      StoppedBy: condition  # or "max_turns", "max_tokens"
    TokensUsed: 1350
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                    Conversation Loop                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Turn 1: Execute prompt → Response A                         │
│          Check stop_condition → false                        │
│                                                              │
│  Turn 2: Execute prompt → Response B                         │
│          Check stop_condition → false                        │
│                                                              │
│  Turn 3: Execute prompt → Response C (contains "APPROVED")   │
│          Check stop_condition → true → EXIT                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**Key point**: The same `prompt` is sent each turn. The conversation history accumulates in `state`, but the provider CLI is invoked with the same prompt repeatedly.

## Examples

### Autonomous Code Review

```yaml
name: code-review
version: "1.0.0"

inputs:
  - name: code
    type: string
    required: true

states:
  initial: review

  review:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a code reviewer. Analyze the code and suggest improvements.
      After each review iteration, either:
      - Suggest another improvement
      - Say "APPROVED" if the code is good
    prompt: |
      Review this code:
      {{.inputs.code}}
    options:
      model: claude-sonnet-4-20250514
    conversation:
      max_turns: 5
      stop_condition: "inputs.response contains 'APPROVED'"
    on_success: done

  done:
    type: terminal
```

### Turn-Limited Generation

```yaml
name: brainstorm
version: "1.0.0"

inputs:
  - name: topic
    type: string
    required: true

states:
  initial: generate

  generate:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: "Generate creative ideas. One idea per turn."
    prompt: "Generate ideas about: {{.inputs.topic}}"
    conversation:
      max_turns: 5
      stop_condition: "inputs.turn_count >= 3"
    on_success: done

  done:
    type: terminal
```

## Limitations

### Current Implementation

- **Single prompt per conversation** - Same prompt executed each turn
- **No interactive input** - Cannot inject user messages between turns
- **Only `sliding_window` strategy** - Other strategies not implemented
- **No conversation continuation** - `continue_from` not implemented
- **No branching** - Single linear path only

### Not Yet Implemented

| Feature | Status | Description |
|---------|--------|-------------|
| `continue_from` | Not implemented | Resume conversation from previous step |
| `summarize` strategy | Not implemented | LLM-based compression of old turns |
| `truncate_middle` strategy | Not implemented | Keep first and last turns |
| `on_error` mapping | Not implemented | Route to specific states by error type |
| Interactive input | Not implemented | User input between turns |

## Best Practices

### 1. Always Set Turn Limits

Prevent runaway conversations:

```yaml
conversation:
  max_turns: 10  # Hard limit
  stop_condition: "inputs.response contains 'DONE'"
```

### 2. Use Specific Stop Keywords

Make stop conditions unambiguous:

```yaml
# Good: Specific signal
stop_condition: "inputs.response contains 'TASK_COMPLETE'"

# Bad: Could match unintended text
stop_condition: "inputs.response contains 'done'"
```

### 3. Instruct Agent About Completion

Include completion signal in system prompt:

```yaml
system_prompt: |
  Complete the task step by step.
  Say "FINISHED" when you're done.
```

### 4. Use Fallback Turn Limits

Combine keyword and turn limit:

```yaml
stop_condition: "inputs.response contains 'DONE' || inputs.turn_count >= 10"
```

## Troubleshooting

### Stop Condition Not Triggering

**Problem**: Conversation runs to max_turns

**Check**:
1. Expression syntax uses `inputs.` prefix
2. Keyword matches exactly (case-sensitive)
3. Agent system prompt instructs completion signal

```yaml
# Wrong
stop_condition: "response contains 'DONE'"

# Correct
stop_condition: "inputs.response contains 'DONE'"
```

### Same Response Each Turn

**Expected behavior**: Conversation mode executes the same prompt each turn. The response may vary due to model non-determinism, but input is identical.

For different inputs each turn, use multiple agent steps instead.

### Context Window Exceeded

**Problem**: Old turns dropped, losing context

**Solutions**:
- Increase `max_context_tokens`
- Reduce `max_turns`
- Use shorter system prompt

## See Also

- [Agent Steps Guide](agent-steps.md) - Single-turn agent execution
- [Workflow Syntax Reference](workflow-syntax.md#agent-state) - Complete options
- [Template Variables](../reference/interpolation.md) - Variable interpolation
