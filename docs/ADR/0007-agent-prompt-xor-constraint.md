# 0007: Agent Prompt XOR Constraint

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

Agent steps accept prompts in two forms: inline `prompt` field (for short prompts) and `prompt_file` field (for external markdown files with template interpolation). If both are set, behavior is ambiguous — which one wins? Silent precedence rules cause subtle bugs in workflow authoring.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| XOR constraint (exactly one) | Unambiguous, fails fast, clear error | Slightly less flexible |
| Precedence rule (file wins) | Allows both, predictable | Silent override, confusing when inline is ignored |
| Merge (inline as fallback) | Maximum flexibility | Complex semantics, hard to debug |

## Decision

Enforce mutual exclusivity at validation time:

- `AgentConfig.Validate()` rejects configurations where both `Prompt` and `PromptFile` are set
- Error message explicitly names both fields and states the constraint
- `awf validate` catches this statically before execution
- `prompt_file` supports full Go template interpolation and resolves via ADR-0006 path resolution
- 1MB size limit on loaded prompt files via `io.LimitReader`

Rules:
- Exactly one of `prompt` or `prompt_file` must be set on agent steps
- Validation in domain layer (`AgentConfig.Validate()`)
- Same constraint applies: never both, never neither

## Consequences

**What becomes easier:**
- Workflow authors get immediate feedback on misconfiguration
- No ambiguity about which prompt is used
- Debugging agent behavior — single source of truth

**What becomes harder:**
- Can't use inline prompt as fallback for missing file (must handle in workflow logic)

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Go Idioms | Compliant | Validation at parse time, explicit error handling |
| Minimal Abstraction | Compliant | Simple boolean check, no complex merge logic |
| Error Taxonomy | Compliant | Validation error = exit code 2 (configuration error) |
