---
title: "009. Breaking Removal of Custom Provider at v0.4.0"
---

Date: 2026-02-21
Status: Accepted
Issue: F070

## Context

`AgentConfig.Command` and `CustomProvider` are the only code path for shell-based agent invocation. With `type: step` covering deterministic CLI tools and `openai_compatible` covering LLM APIs, `custom` has no unique purpose and its shell-template model conflates execution paradigms. Keeping dead code violates Principle 6 (Minimal Abstraction).

## Decision

Delete `custom_provider.go` and its test files, remove `AgentConfig.Command` field from the domain model, and add a validation error for `provider: custom` with migration guidance pointing users to `type: step` or `provider: openai_compatible`. Released as a breaking change at v0.4.0.

Alternatives rejected:
- **Deprecation warning first** — v0.4.0 is already a breaking version boundary; parallel maintenance adds cost with no benefit.
- **Keep `Command` as unused field** — dead fields violate Minimal Abstraction and confuse future readers.

## Consequences

### Positive
- Eliminates ~820 LOC of dead code and associated maintenance burden.
- Enforces strict semantic separation: CLI execution vs. LLM API invocation.
- Validation error provides actionable migration guidance at configuration time.

### Negative
- Users with `provider: custom` workflows get a hard failure requiring migration.
- Breaking change must be prominently documented in CHANGELOG and migration guide.
