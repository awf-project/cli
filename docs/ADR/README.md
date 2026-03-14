# Architecture Decision Records

This directory contains the Architecture Decision Records (ADRs) for this project.

## Format

Each ADR follows this structure:

```markdown
# NNNN: Title

**Status**: Proposed | Accepted | Superseded | Deprecated
**Date**: YYYY-MM-DD

## Context       — What is the issue motivating this decision?
## Candidates    — Options considered with trade-offs
## Decision      — What we chose and why
## Consequences  — What becomes easier/harder
## Constitution Compliance — Mapping to project principles
```

## Numbering Convention

ADRs are numbered sequentially: `001`, `002`, etc.
Numbers are never reused. If a decision is reversed, the original ADR is marked "Superseded" and a new ADR is created with a reference.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [001](001-hexagonal-architecture.md) | Hexagonal Architecture | Accepted |
| [002](002-error-taxonomy-exit-codes.md) | Error Taxonomy with Exit Codes | Accepted |
| [003](003-yaml-state-machine-workflows.md) | YAML State Machine for Workflow Definition | Accepted |
| [004](004-domain-operation-registry.md) | Domain Operation Registry with Infrastructure Coexistence | Accepted |
| [005](005-atomic-file-writes.md) | Atomic File Writes for State Persistence | Accepted |
| [006](006-xdg-path-resolution.md) | XDG-Compliant Path Resolution | Accepted |
| [007](007-agent-prompt-xor-constraint.md) | Agent Prompt XOR Constraint | Accepted |
| [008](008-openai-compatible-provider-http-adapter.md) | OpenAI-Compatible Provider: HTTP Adapter Integration | Accepted |
| [009](009-breaking-removal-custom-provider-v0.4.0.md) | Breaking: Removal of Custom Provider in v0.4.0 | Accepted |
| [010](010-paired-jsonl-audit-trail-with-atomic-append.md) | Paired JSONL Audit Trail with Atomic Append | Accepted |
| [011](011-application-layer-secret-masking-for-audit-events.md) | Application-Layer Secret Masking for Audit Events | Accepted |
| [012](012-runtime-shell-detection.md) | Runtime Shell Detection with $SHELL Environment Variable | Accepted |
| [013](013-context-aware-input-ports.md) | Context-Aware Input Ports | Accepted |
| [014](014-shebang-execution-for-script-files.md) | Shebang Execution for Script Files | Accepted |

## Creating a New ADR

1. Find the next number: `ls docs/ADR/ | grep -oP '^\d+' | sort -n | tail -1` + 1
2. Copy the template: `cp docs/ADR/.template.md docs/ADR/NNNN-short-name.md`
3. Fill in all sections
4. Update this index
5. Submit for review

## Pre-Merge Checklist

Before merging any new or modified ADR:

- [ ] **Cross-references**: All `[ADR-NNNN]` links resolve to existing files
- [ ] **Supersession**: If changing a prior decision, both ADRs have `Supersedes`/`Superseded by` metadata
- [ ] **Constitution**: Compliance section maps to current constitution version
- [ ] **Candidates**: At least 2 alternatives documented with trade-offs
