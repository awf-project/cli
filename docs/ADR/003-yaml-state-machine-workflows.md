---
title: "003: YAML State Machine for Workflow Definition"
---

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF needs a way for users to define multi-step workflows that orchestrate AI agents and shell commands. The definition format must be human-readable, version-controllable, and expressive enough to handle branching, parallelism, retries, and loops — without requiring programming knowledge.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| YAML state machine | Human-readable, versionable, declarative, familiar syntax | Limited expressiveness, custom DSL to learn |
| Go plugin system | Full language power, type-safe | Requires Go knowledge, compilation step, not portable |
| JSON/TOML config | Machine-friendly, standard parsers | JSON verbose for humans, TOML lacks nesting depth |
| Lua/Starlark scripting | Turing-complete, sandboxed | Extra runtime, security surface, steeper learning curve |

## Decision

Use YAML files with state machine semantics. Each workflow is a directed graph of states with typed transitions:

- State types: `step`, `agent`, `parallel`, `terminal`, `for_each`, `while`, `operation`, `call_workflow`
- Transitions via `on_success`, `on_failure`, and conditional `transitions` list
- Go template interpolation (`{{.inputs.name}}`, `{{.states.prev.Output}}`)
- External files via `script_file` and `prompt_file` for complex content

Template interpolation uses Go template syntax (`{{var}}`) instead of shell syntax (`${var}`) to avoid conflicts with shell commands in step definitions.

## Consequences

**What becomes easier:**
- Non-developers can author workflows
- Workflows are diff-friendly and reviewable in PRs
- Static validation possible (`awf validate`)
- Visualization (`awf diagram`)

**What becomes harder:**
- Complex logic requires workarounds (exit codes as routing signals)
- No native functions/variables — must use shell for computation
- Template syntax learning curve for Go-unfamiliar users

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | YAML parsing in infrastructure, domain defines Workflow entity |
| Minimal Abstraction | Compliant | Declarative over programmatic, only the state types needed |
| Documentation Co-location | Compliant | Workflows self-document via description fields |
