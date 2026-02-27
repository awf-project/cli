# 0008. OpenAI-Compatible Provider as HTTP Infrastructure Adapter

Date: 2026-02-21
Status: Accepted
Issue: F070

## Context

The `custom` provider executes LLM invocations via shell commands (`/bin/sh -c`), blurring the boundary between deterministic CLI steps (`type: step`) and probabilistic AI model calls (`type: agent`). A replacement must establish a clean HTTP-native pattern for API-compatible providers and enable native multi-turn conversations (previously unimplemented in custom provider).

## Decision

Implement `OpenAICompatibleProvider` in `internal/infrastructure/agents/` using `pkg/httpx.Client` (the `HTTPDoer` interface from F058) for HTTP transport. Chat Completions request/response types are unexported and co-located in the provider file. No new packages or public types are introduced.

Alternatives rejected:
- **New `pkg/openai` package** — YAGNI: no second consumer exists yet; extraction deferred until needed.
- **Generic HTTP provider base struct** — premature abstraction with no concrete second use case.

## Consequences

### Positive
- Reuses existing `HTTPDoer` abstraction — no new transport infrastructure needed.
- Token counts come from API `usage` field (accurate) instead of `len(output)/4` estimation.
- Native multi-turn conversations via `ExecuteConversation` (previously returned "not implemented").
- Backend-agnostic: works with OpenAI, Ollama, vLLM, Groq via standard Chat Completions protocol.

### Negative
- If a second Chat Completions provider is added later, types must be extracted to a shared location.
- Provider configuration (base_url, model, api_key) via options map rather than typed struct fields.
