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

ADRs are numbered sequentially: `0001`, `0002`, etc.
Numbers are never reused. If a decision is reversed, the original ADR is marked "Superseded" and a new ADR is created with a reference.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-hexagonal-architecture.md) | Hexagonal Architecture | Accepted |
| [0002](0002-error-taxonomy-exit-codes.md) | Error Taxonomy with Exit Codes | Accepted |
| [0003](0003-yaml-state-machine-workflows.md) | YAML State Machine for Workflow Definition | Accepted |
| [0004](0004-domain-operation-registry.md) | Domain Operation Registry with Infrastructure Coexistence | Accepted |
| [0005](0005-atomic-file-writes.md) | Atomic File Writes for State Persistence | Accepted |
| [0006](0006-xdg-path-resolution.md) | XDG-Compliant Path Resolution | Accepted |
| [0007](0007-agent-prompt-xor-constraint.md) | Agent Prompt XOR Constraint | Accepted |
| [0012](0012-runtime-shell-detection.md) | Runtime Shell Detection with $SHELL Environment Variable | Accepted |

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
