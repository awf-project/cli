# 0012. Runtime Shell Detection with $SHELL Environment Variable

**Status**: Accepted
**Date**: 2026-03-02
**Issue**: B006
**Supersedes**: N/A
**Superseded by**: N/A

## Context

`ShellExecutor` hardcodes `/bin/sh -c`, which maps to `dash` on Debian/Ubuntu. Bash-dependent workflow commands (arrays, `[[`, process substitution, brace expansion) fail silently or with cryptic errors. Users on those distros cannot use bash syntax without workarounds.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| **$SHELL Detection + Fallback** | Automatic for users; minimal code; backward-compatible variadic options; matches agents pattern | Silent fallback for incompatible shells (fish); per-step override deferred |
| **Always `/bin/bash`** | Simple, works for most users | Breaks minimal containers (Alpine); not standard POSIX |
| **Per-step `shell:` field** | Flexible, per-step control | Domain model change; larger surface; YAGNI |
| **Document POSIX-only** | Zero code change | Poor UX; doesn't fix the problem; users forced to use workarounds |

## Decision

Detect the user's preferred shell at `ShellExecutor` construction time by reading the `$SHELL` environment variable. Validate the path is absolute and exists on disk; fall back to `/bin/sh` if unset, relative, or invalid. Expose a `WithShell(path)` functional option for explicit override and test injection. Change is isolated to `internal/infrastructure/executor/shell_executor.go` — zero domain or port modifications.

**Implementation:**
- Add `shellPath string` field to `ShellExecutor`
- Add `detectShell()` helper: reads `$SHELL`, validates path, returns fallback on error
- Add `WithShell(path)` functional option matching `agents/options.go` pattern
- Update `Execute()` to use `e.shellPath` instead of hardcoded `/bin/sh`
- Update documentation: `doc.go`, `ports/executor.go`, `ports/doc.go`, `SECURITY.md`, `architecture.md`

## Consequences

**What becomes easier:**
- Bash-dependent commands work automatically for users with `$SHELL=/bin/bash`
- Workflows written on macOS/Arch (bash) now run on Debian/Ubuntu without modification
- Test injection via `WithShell()` without environment variable manipulation
- Zero call-site changes — backward compatibility maintained via variadic options

**What becomes harder:**
- Users with exotic shells (fish, nushell) without `-c` flag will silently fall back to POSIX `/bin/sh`
- Per-step shell override deferred — all steps use the same detected shell (can be added later if needed)

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| **P1: Hexagonal Architecture** | Compliant | Infrastructure-only change; zero domain/port modifications |
| **P2: Go Idioms** | Compliant | Functional options pattern, explicit error handling, `os.Getenv` |
| **P3: Minimal Abstraction** | Compliant | Single field + single option; no new types/interfaces |
| **P4: Error Taxonomy** | Compliant | Silent fallback is not an error condition (matches B005 precedent) |
| **P5: Security First** | Compliant | Validates shell path is absolute and exists; gosec G204 documented |
| **P6: Test-Driven Development** | Compliant | Table-driven tests for all scenarios |
