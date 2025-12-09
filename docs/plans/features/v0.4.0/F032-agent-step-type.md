# F032: Agent Step Type

## Metadata
- **Statut**: backlog
- **Phase**: 3-AI
- **Version**: v0.4.0
- **Priorité**: high
- **Estimation**: L

## Description

Introduce a dedicated `agent` step type for invoking AI CLI agents (Claude Code, Codex, Gemini CLI, OpenCode, etc.) in a structured, non-interactive way. This replaces ad-hoc shell commands with a first-class abstraction that handles prompt injection, response parsing, and multi-turn conversations.

```
┌─────────────────────────────────────────────────────────────┐
│                      WORKFLOW STEP                          │
│  type: agent                                                │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────┐    ┌──────────────┐    ┌─────────────────┐    │
│  │ Prompt  │───▶│ Agent Adapter│───▶│ CLI Invocation  │    │
│  │ Template│    │ (provider)   │    │ (non-interactive│    │
│  └─────────┘    └──────────────┘    └────────┬────────┘    │
│                                              │              │
│                 ┌──────────────┐             │              │
│                 │ Response     │◀────────────┘              │
│                 │ Parser       │                            │
│                 └──────┬───────┘                            │
│                        │                                    │
│                        ▼                                    │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ states.step_name.output   (raw text)                    ││
│  │ states.step_name.response (parsed if JSON)              ││
│  │ states.step_name.tokens   (usage if available)          ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

**Design principles:**
1. **Non-interactive by design** — No stdin passthrough, prompts are inputs
2. **Provider abstraction** — Same YAML syntax, different AI backends
3. **Structured output** — Parse JSON responses when possible
4. **Stateless invocations** — Multi-turn = multiple steps with state passing

## Critères d'Acceptance

- [ ] New step type `agent` recognized by workflow parser
- [ ] Support for providers: `claude`, `codex`, `gemini`, `opencode`, `custom`
- [ ] Prompt interpolation with `{{inputs.*}}` and `{{states.*}}`
- [ ] Response captured in `states.step_name.output`
- [ ] JSON responses auto-parsed into `states.step_name.response`
- [ ] Token usage captured in `states.step_name.tokens` (when available)
- [ ] Provider-specific options (model, temperature, max_tokens)
- [ ] Timeout handling per step
- [ ] `--dry-run` shows the resolved prompt without invoking
- [ ] Error handling: provider not found, CLI not installed, timeout
- [ ] Works with parallel steps (multiple agents in parallel)

## Dépendances

- **Bloqué par**: F001, F003, F005, F029
- **Débloque**: F033 (Agent Conversations), F034 (Agent Tools)

## Fichiers Impactés

```
internal/domain/workflow/step.go              # Add AgentStep type
internal/domain/workflow/agent_config.go      # New: agent configuration
internal/domain/ports/agent_provider.go       # New: provider interface
internal/infrastructure/agents/               # New: provider implementations
├── claude_provider.go
├── codex_provider.go
├── gemini_provider.go
├── opencode_provider.go
└── custom_provider.go
internal/application/execution_service.go    # Handle agent step execution
internal/interfaces/cli/run.go               # --dry-run for agent prompts
pkg/validation/agent_validation.go           # Validate agent config
```

## Tâches Techniques

- [ ] Define AgentStep domain model
  - [ ] Add `type: agent` to step type enum
  - [ ] Define AgentConfig struct (provider, model, prompt, options)
  - [ ] Add provider-specific option structs
- [ ] Create AgentProvider port (interface)
  - [ ] Define Execute(ctx, prompt, options) (output, tokens, error)
  - [ ] Define provider registry pattern
- [ ] Implement provider adapters
  - [ ] ClaudeProvider: `claude -p "prompt" --output-format json`
  - [ ] CodexProvider: `codex --prompt "prompt" --quiet`
  - [ ] GeminiProvider: `gemini -p "prompt"`
  - [ ] OpenCodeProvider: `opencode run "prompt"`
  - [ ] CustomProvider: user-defined command template
- [ ] Update YAML parser for agent steps
  - [ ] Parse agent-specific fields
  - [ ] Validate provider is known or custom
- [ ] Integrate in ExecutionService
  - [ ] Detect agent step type
  - [ ] Resolve provider from registry
  - [ ] Execute and capture response
  - [ ] Parse JSON if applicable
  - [ ] Store in state (output, response, tokens)
- [ ] Add --dry-run support
  - [ ] Show resolved prompt
  - [ ] Show command that would be executed
- [ ] Write unit tests
- [ ] Write integration tests with mock providers
- [ ] Update CLI help and documentation

## YAML Syntax

```yaml
name: code-review-workflow
version: "1.0"

inputs:
  code_path:
    type: string
    required: true

steps:
  - name: analyze
    type: agent
    provider: claude
    prompt: |
      Analyze this code for potential issues:
      {{inputs.code_path}}
    options:
      model: claude-sonnet-4-20250514
      max_tokens: 4096
      output_format: json

  - name: suggest_fixes
    type: agent
    provider: claude
    prompt: |
      Based on this analysis:
      {{states.analyze.response}}

      Suggest specific fixes.
    options:
      model: claude-sonnet-4-20250514

  - name: apply_with_codex
    type: agent
    provider: codex
    prompt: "Apply these fixes: {{states.suggest_fixes.output}}"
    timeout: 300s

  # Custom provider example
  - name: custom_llm
    type: agent
    provider: custom
    command: "my-llm --prompt {{prompt}} --json"
    prompt: "Summarize: {{states.analyze.output}}"
```

## Provider Configuration

Global provider config in `~/.awf/config.yaml`:

```yaml
agents:
  claude:
    binary: claude
    default_model: claude-sonnet-4-20250514
    default_options:
      output_format: json

  codex:
    binary: codex
    default_options:
      quiet: true

  gemini:
    binary: gemini
    default_model: gemini-pro

  opencode:
    binary: opencode
```

## Notes

- **No stdin passthrough** — Workflows must remain non-interactive and reproducible
- **Multi-turn conversations** — Use multiple steps with `{{states.prev.output}}`, not session persistence
- **Token tracking** — Useful for cost estimation and rate limiting in future
- **Custom provider** — Escape hatch for unsupported CLIs or local models
- Future F033 could add `conversation` type for managed multi-turn with context window handling
