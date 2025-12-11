# F033: Agent Conversations

## Metadata
- **Status**: backlog
- **Phase**: 3-AI
- **Version**: v0.4.0
- **Priority**: medium
- **Estimation**: L

## Description

Extend the `agent` step type (F032) to support managed multi-turn conversations with automatic context window handling. While F032 treats each agent invocation as stateless, F033 introduces a `conversation` mode that maintains conversation history across turns.

```
┌─────────────────────────────────────────────────────────────┐
│                   CONVERSATION STEP                         │
│  type: agent                                                │
│  mode: conversation                                         │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐  │
│  │                 CONVERSATION STATE                    │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐ │  │
│  │  │ Turn 1  │─▶│ Turn 2  │─▶│ Turn 3  │─▶│ Turn N  │ │  │
│  │  │ (user)  │  │ (asst)  │  │ (user)  │  │  ...    │ │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘ │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                  │
│                          ▼                                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              CONTEXT WINDOW MANAGER                   │  │
│  │  • Token counting                                     │  │
│  │  • Truncation strategy (sliding window / summary)    │  │
│  │  • System prompt preservation                         │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

**Use cases:**
1. **Iterative refinement** — Multiple rounds of feedback on generated content
2. **Clarification loops** — Agent asks questions, workflow provides answers
3. **Complex reasoning** — Chain-of-thought across multiple turns

## Acceptance Criteria

- [ ] New `mode: conversation` option for agent steps
- [ ] Conversation history maintained in step state
- [ ] Automatic context window management with configurable strategy
- [ ] Token counting for supported providers
- [ ] System prompt preserved across truncations
- [ ] `max_turns` limit to prevent infinite loops
- [ ] `stop_condition` expression to exit conversation early
- [ ] Conversation state accessible via `{{states.step.conversation}}`
- [ ] Support for injecting context mid-conversation
- [ ] Works with streaming output (F029)

## Dependencies

- **Blocked by**: F032 (Agent Step Type)
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/workflow/conversation.go        # New: conversation model
internal/domain/workflow/context_window.go      # New: token management
internal/infrastructure/agents/base_provider.go # Add conversation support
internal/application/conversation_manager.go    # New: conversation orchestration
pkg/tokenizer/                                  # New: token counting utilities
```

## Technical Tasks

- [ ] Define Conversation domain model
  - [ ] Turn struct (role, content, tokens)
  - [ ] ConversationState (turns, total_tokens, metadata)
  - [ ] ConversationConfig (max_turns, max_tokens, strategy)
- [ ] Implement context window strategies
  - [ ] `sliding_window` — Drop oldest turns
  - [ ] `summarize` — Compress old turns via LLM
  - [ ] `truncate_middle` — Keep first and last turns
- [ ] Add token counting
  - [ ] Tiktoken integration for OpenAI-compatible
  - [ ] Approximation for other providers
- [ ] Extend AgentProvider interface
  - [ ] Add ConversationExecute method
  - [ ] Pass conversation history
- [ ] Update ExecutionService
  - [ ] Detect conversation mode
  - [ ] Manage conversation state
  - [ ] Apply context window strategy
- [ ] Add stop conditions
  - [ ] Expression evaluation (e.g., `response contains "DONE"`)
  - [ ] Max turns reached
  - [ ] Token budget exhausted
- [ ] Write unit tests
- [ ] Write integration tests

## YAML Syntax

```yaml
steps:
  - name: refine_code
    type: agent
    provider: claude
    mode: conversation
    system_prompt: |
      You are a code reviewer. Iterate on the code until it meets quality standards.
      Say "APPROVED" when done.
    initial_prompt: |
      Review this code:
      {{inputs.code}}
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 4096
    conversation:
      max_turns: 10
      max_context_tokens: 100000
      strategy: sliding_window
      stop_condition: "response contains 'APPROVED'"

  # Inject context mid-conversation
  - name: add_requirements
    type: agent
    provider: claude
    mode: conversation
    continue_from: refine_code  # Continue previous conversation
    prompt: |
      Also consider these requirements:
      {{inputs.additional_requirements}}
```

## State Structure

```yaml
states:
  refine_code:
    output: "Final response text"
    response: { ... }  # Parsed JSON if applicable
    tokens:
      input: 15000
      output: 2000
      total: 17000
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

## Notes

- **Reproducibility concern** — Conversations are inherently less reproducible than single-shot prompts
- **Cost awareness** — Multi-turn conversations can accumulate significant token usage
- **Provider differences** — Some CLIs may not support conversation mode natively; may need to serialize history in prompt
- **Future enhancement** — Could support branching conversations (explore multiple paths)
