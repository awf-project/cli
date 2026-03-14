---
title: "002: Error Taxonomy with Exit Codes"
---

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF is a CLI tool used in scripts and CI pipelines. Consumers need to programmatically distinguish error categories to decide retry strategy, user messaging, and escalation. A single non-zero exit code forces callers to parse stderr text, which is fragile and locale-dependent.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| Fixed exit code taxonomy (1-4) | Scriptable, simple, universal | Limited to 4 categories, no sub-codes |
| Structured JSON error output | Rich metadata, extensible | Requires parsing, breaks simple `$?` checks |
| Single non-zero exit (1) | Simplest implementation | No differentiation, useless for automation |

## Decision

Map exit codes to four error categories:

| Code | Category | Examples |
|------|----------|----------|
| `1` | User error | Bad input, missing file, invalid workflow name |
| `2` | Configuration error | Invalid config, missing dependency, bad YAML |
| `3` | Execution error | Command failed, timeout, agent error |
| `4` | System error | IO failure, permissions, disk full |

Rules:
- Every error path must produce exactly one of these codes
- Error types defined in domain layer
- Infrastructure maps native errors to taxonomy
- Exit code 0 = success, always

## Consequences

**What becomes easier:**
- Scripting: `awf run X || case $? in 1) ... 2) ... esac`
- CI pipelines can distinguish retryable (3) from fatal (1, 2, 4) errors
- Consistent user feedback across all commands

**What becomes harder:**
- Adding new categories requires semver major bump
- Infrastructure errors must be correctly classified (not just "exit 1")

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Error Taxonomy | Compliant | This ADR defines the principle |
| Go Idioms | Compliant | Custom error types with Error() method |
| Security First | Compliant | Error messages never leak secrets |
