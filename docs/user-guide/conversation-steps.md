---
title: "Conversation Mode & Session Tracking"
---

AWF supports two distinct conversation-related features on agent steps:

1. **Interactive conversation mode** (`mode: conversation`) — a live, user-driven chat loop where the user types messages at each turn and ends the session with `exit`, `quit`, or an empty line.
2. **Cross-step session tracking** (`conversation:` sub-struct on any agent step) — opts a single-turn agent step into session tracking so another step can resume its conversation via `continue_from`.

These two features are independent. You can use either, both, or neither.

## When to Use Which

| You want to... | Use |
|---|---|
| Chat with an agent from a terminal, one question at a time, until you decide to stop | `mode: conversation` |
| Run a single-turn agent call whose session a later step will resume | `mode: single` + `conversation: {}` |
| Run a single-turn agent call that resumes a prior step's session | `mode: single` + `conversation: {continue_from: prior_step}` |
| Run a plain one-shot agent call with no session at all | plain agent step, no `conversation:` sub-struct |

## Interactive Conversation Mode

`mode: conversation` spawns an interactive loop: the agent replies, AWF prints a `> ` prompt, you type your next message, repeat. The session ends when you submit an empty line, `exit`, or `quit`.

### Example

```yaml
name: interactive-clarify
version: "1.0.0"

inputs:
  - name: topic
    type: string
    default: "explain Go channels"

states:
  initial: chat

  chat:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a concise technical assistant. Ask one clarifying question at a time.
    prompt: |
      {{.inputs.topic}}
    options:
      model: claude-haiku-4-5
    timeout: 600
    on_success: done

  done:
    type: terminal
    status: success
```

Run it:

```bash
awf run interactive-clarify
```

You'll see the agent's first reply, then a `> ` prompt where you can type. When you're done, press Enter on an empty line or type `exit`.

### Required and Optional Fields

| Field | Required | Description |
|---|---|---|
| `provider` | Yes | Agent provider (`claude`, `gemini`, `codex`, `opencode`, `openai_compatible`) |
| `mode` | Yes | Must be `conversation` |
| `prompt` | Yes | First user message — sent automatically as turn 1 |
| `system_prompt` | No | System message preserved for the whole session |
| `options` | No | Provider-specific options (model, allowed_tools, etc.) |
| `timeout` | No | Per-turn timeout in seconds (default: 300) |

### Terminal Requirement

`mode: conversation` reads from `os.Stdin`. It requires a TTY or piped stdin. In CI/headless runs, pipe an empty input to exit immediately after turn 1:

```bash
echo "" | awf run interactive-clarify
```

For fully non-interactive workflows, prefer [cross-step session tracking](#cross-step-session-tracking) below.

### Exit Signals

The conversation ends when:
- The user submits an **empty line**
- The user types `exit` or `quit` (case-insensitive)
- `stdin` returns EOF (e.g., a closed pipe)
- The step timeout fires

The step completes with `stopped_by: user_exit` in the recorded state.

## Cross-Step Session Tracking

For automated workflows, you rarely want an interactive loop. You want step A to establish a conversation with an agent, and step B (later in the workflow) to resume that same session with additional context — without a human in the loop.

This is what the `conversation:` sub-struct on a **single-mode** agent step provides.

### Example: Seed and Recall

```yaml
name: session-resume-demo
version: "1.0.0"

states:
  initial: seed

  seed:
    type: agent
    provider: claude
    system_prompt: "You are a memory test assistant."
    prompt: |
      Remember this secret: the magic word is BANANA42.
      Reply with exactly: "stored".
    conversation: {}              # opt into session tracking
    options:
      model: claude-haiku-4-5
    on_success: recall

  recall:
    type: agent
    provider: claude
    prompt: |
      What was the magic word I told you to remember?
    conversation:
      continue_from: seed         # resume seed's session
    on_success: done

  done:
    type: terminal
    status: success
```

Both steps run as `mode: single` (the default — no `mode:` line needed). There is no interactive loop. Each step runs exactly one agent turn.

- `seed` has `conversation: {}`. This marks the step as session-tracked: AWF calls `provider.ExecuteConversation` (instead of `provider.Execute`), the provider runs one turn, and the session ID returned by the CLI is captured into `state.conversation.session_id`.
- `recall` has `conversation: {continue_from: seed}`. AWF clones the conversation state from `seed` (session ID + turn history) and passes it to the provider, which resumes the session via its native flag (`claude -r <id>`, `gemini --resume <id>`, `codex resume <id>`, `opencode -s <id>`).

### Why the Empty `conversation: {}`?

The presence of the `conversation:` sub-struct — even empty — is the marker that opts the step into session tracking. Without it, a single-mode agent step uses `provider.Execute` and produces no session state, so no other step can ever resume it.

Think of `conversation:` as a **flag** meaning *"track this step's session"*, not as "enable multi-turn mode". The field name is historical; its F083 meaning is session metadata.

### ContinueFrom Rules

`continue_from` references another step by name. At runtime, AWF enforces:

1. The referenced step must have **already executed** in the current run (forward references fail).
2. Its `state.conversation` must be non-nil — i.e., it must itself have been session-tracked (either `mode: conversation` or `mode: single` + `conversation: {}`).
3. The conversation state must have a non-empty `session_id` **or** at least one recorded turn.
4. For `provider: openai_compatible` (HTTP-based), at least one recorded turn is required since there is no server-side session.

Violating any of these produces a clear error: `continue_from: step "X" has no session ID or conversation history to resume`.

### Cross-Provider Session Chains

Each provider has its own session identifier format and CLI flag. A session established by Claude cannot be resumed by Gemini. Keep `provider` consistent across the seed and recall steps — or use distinct seed/recall pairs per provider, as in [`test-resume.yaml`](https://github.com/awf-project/cli/blob/main/.awf/workflows/test-resume.yaml).

### Session Tracking vs. State Passing

AWF has always supported chaining agent steps via template interpolation:

```yaml
step2:
  type: agent
  prompt: |
    Based on: {{.states.step1.Output}}
    Now answer: ...
```

This is **state passing** — step2 gets step1's textual output but the agent has no memory of step1's conversation. Every step is stateless from the provider's perspective.

Session tracking is different: the provider itself retains the conversation (via its CLI's session store), so step2 resumes as if the agent never stopped. Benefits:

- Large prior context doesn't need to be re-sent in the prompt (token savings)
- The agent can reference earlier parts of the session implicitly
- System prompt and tool state are retained by the provider

Downsides:
- Coupled to the provider's session store (opaque, may expire)
- Only works within a single workflow run (sessions aren't persisted across runs)
- Fails gracefully to stateless if session ID extraction fails

Use state passing for simple chaining; use session tracking when the agent needs semantic continuity.

## Common Configuration

### Fields Removed in F083

If you're upgrading from an earlier AWF version, these fields no longer exist:

| Removed Field | Replacement |
|---|---|
| `initial_prompt` | Use `prompt` — it serves as the first user message |
| `conversation.max_turns` | The user drives turn count in interactive mode; `mode: single` is always one turn |
| `conversation.max_context_tokens` | Removed — context window management is deferred to the provider |
| `conversation.strategy` | Removed — no automatic truncation |
| `conversation.stop_condition` | Removed — user types `exit`/`quit` to stop interactive mode |
| `conversation.inject_context` | Removed — compose prompts with standard `{{.states.*}}` interpolation |

Workflows using any of these fields will silently ignore them (YAML lenient mode), which may produce unexpected behavior. **Remove them explicitly.**

## Observability

Both conversation features populate `state.conversation` on the step state with:

| Field | Description |
|---|---|
| `session_id` | Provider-assigned session identifier (empty if extraction failed) |
| `turns` | List of user and assistant messages recorded during the step |
| `total_turns` | Turn counter |
| `total_tokens` | Estimated token usage across the session |
| `stopped_by` | `user_exit` (user typed exit/quit/empty line) or `error` |

These fields are visible in `awf history <workflow-id>` output and in the step state files under `storage/states/`.

## Complete Examples

### Interactive Clarification Loop

A runnable example of `mode: conversation`. Paste into a file, run with `awf run <filename>`, answer the prompts, and type `exit` when done.

```yaml
name: clarify
version: "1.0.0"
description: Interactive specification clarification session

inputs:
  - name: topic
    type: string
    default: "explain Go channels in one sentence"

states:
  initial: chat

  chat:
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a concise technical assistant. Ask one clarifying question
      at a time and wait for the user's answer before continuing. When
      the user types "exit", produce a final summary.
    prompt: |
      {{.inputs.topic}}
    options:
      model: claude-haiku-4-5
    timeout: 600
    on_success: done

  done:
    type: terminal
    status: success
```

### Cross-Step Session Resume Across Providers

A non-interactive example exercising session tracking across Claude, Gemini, and OpenCode. Each provider gets a seed step (establishes a session with a secret) and a recall step (resumes the session and retrieves the secret).

```yaml
name: session-resume-demo
version: "1.0.0"
description: Cross-step session resume across Claude, Gemini, and OpenCode

states:
  initial: claude_seed

  claude_seed:
    type: agent
    provider: claude
    system_prompt: "You are a memory test assistant. Answer briefly."
    prompt: |
      Remember this secret: the magic word is BANANA42.
      Reply with exactly: "stored".
    conversation: {}
    options:
      dangerously_skip_permissions: true
    timeout: 60
    on_success: claude_recall

  claude_recall:
    type: agent
    provider: claude
    prompt: "What is the magic word I told you to remember?"
    conversation:
      continue_from: claude_seed
    options:
      dangerously_skip_permissions: true
    timeout: 60
    on_success: gemini_seed

  gemini_seed:
    type: agent
    provider: gemini
    system_prompt: "You are a memory test assistant. Answer briefly."
    prompt: |
      Remember this secret: the magic word is MANGO17.
      Reply with exactly: "stored".
    conversation: {}
    options:
      dangerously_skip_permissions: true
    timeout: 60
    on_success: gemini_recall

  gemini_recall:
    type: agent
    provider: gemini
    prompt: "What is the magic word I told you to remember?"
    conversation:
      continue_from: gemini_seed
    options:
      dangerously_skip_permissions: true
    timeout: 60
    on_success: verify

  verify:
    type: step
    command: |
      echo "claude expected BANANA42 -> {{.states.claude_recall.Output}}"
      echo "gemini expected MANGO17  -> {{.states.gemini_recall.Output}}"
    continue_on_error: true
    on_success: done

  done:
    type: terminal
    status: success
```

Run with `awf run session-resume-demo`. Each agent step runs exactly one turn; the recall steps prove the provider retained the seed step's session by recalling the secret without being re-told.

## See Also

- [Agent Steps](agent-steps.md) — complete reference for `type: agent`, including provider options, output formats, and single-turn usage
- [Workflow Syntax](workflow-syntax.md) — full YAML reference
