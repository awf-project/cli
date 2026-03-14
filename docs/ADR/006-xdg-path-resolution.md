---
title: "006: XDG-Compliant Path Resolution"
---

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF has three categories of files: configuration (workflows, templates), runtime state (execution logs, history), and cache. These files need well-defined locations that work across Linux, macOS, and respect user customization.

Additionally, workflows reference external files (`script_file`, `prompt_file`) that must resolve correctly regardless of where `awf` is invoked from. Resolution must handle: workflow-relative paths, project-local overrides, and global XDG paths.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| XDG Base Directory Specification | Standard on Linux, predictable, user-overridable | macOS convention differs (~Library/), verbose paths |
| Single ~/.awf directory | Simple, discoverable | Mixes config/state/cache, no standard |
| Binary-relative paths | Portable, self-contained | Breaks on system install, no user customization |

## Decision

Follow XDG Base Directory Specification:

| Variable | Default | AWF Usage |
|----------|---------|-----------|
| `XDG_CONFIG_HOME` | `~/.config` | `~/.config/awf/` — workflows, templates, prompts |
| `XDG_STATE_HOME` | `~/.local/state` | `~/.local/state/awf/` — logs, history.db |
| `XDG_CACHE_HOME` | `~/.cache` | `~/.cache/awf/` — temporary downloads |

Path resolution order for `script_file` / `prompt_file`:
1. Resolve relative to `workflow.SourceDir`
2. Check for local project override via `resolveLocalOverGlobal()`
3. Fall back to global XDG path

Rules:
- Application layer populates AWF context variables (`.awf.config_dir`, `.awf.cache_dir`)
- Local XDG overrides restricted to `scripts_dir` and `prompts_dir` only (allowlist)
- Never import infrastructure modules from application layer for path resolution
- Inject XDG paths via `SetAWFPaths()` pattern in application layer

## Consequences

**What becomes easier:**
- Users can override all paths via environment variables
- System packages can separate config from state
- Project-local scripts/prompts override globals transparently

**What becomes harder:**
- Path resolution logic is multi-step (3 levels of fallback)
- Testing requires mocking XDG environment variables
- allowlist must be maintained when adding new overridable paths

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | Path resolution in application, file access in infrastructure |
| Go Idioms | Compliant | os.Getenv for XDG, no magic globals |
| Security First | Compliant | Allowlist prevents unintended path resolution |
