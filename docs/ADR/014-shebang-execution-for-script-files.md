---
title: "014: Shebang Execution for Script Files"
---

**Status**: Accepted
**Date**: 2026-03-04
**Fixes**: B009
**Related**: [ADR-0012 - Runtime Shell Detection](012-runtime-shell-detection.md)

## Context

`script_file` steps load shell script content and pass it to `$SHELL -c`, ignoring the script's shebang line. This forces all scripts through the user's login shell, breaking Python, Ruby, Perl, and other non-shell scripts silently. It also fails for shell scripts written in an incompatible variant (e.g., bash script under zsh, or vice versa).

Additionally, passing large script content as a `-c` argument risks hitting `ARG_MAX` limits (128KB–2MB depending on OS), causing silent truncation or exec failures.

The root cause: `resolveStepCommand()` reads script file content as a string, and `ShellExecutor.Execute()` unconditionally wraps it in `exec.CommandContext(ctx, e.shellPath, "-c", cmd.Program)`, bypassing the kernel's shebang mechanism entirely.

## Candidates

| Option | Implementation | Pros | Cons |
|--------|---|---|---|
| **A: Temp file + direct exec** | Write to temp file, `chmod +x`, execute directly when shebang detected; fallback to `$SHELL -c` for no-shebang | Delegates to kernel; handles edge cases (`#!/usr/bin/env -S python3 -u`); solves `ARG_MAX`; backward-compatible | Temp file I/O overhead per execution |
| **B: Parse shebang in application** | Extract interpreter from `#!` line, call `exec.CommandContext(interpreter, tmpFile)` | No temp file I/O overhead | Reinvents kernel shebang handling; fragile for edge cases; still needs temp file for `ARG_MAX` |
| **C: Always write temp file** | Always write `script_file` content to temp file regardless of shebang | Solves `ARG_MAX` for all cases; simpler logic | Wasteful I/O for no-shebang scripts; not necessary |

**Selected: Option A (Temp file + direct exec)**

**Rationale**: The kernel's `execve()` already handles shebang parsing perfectly, including edge cases like `#!/usr/bin/env -S python3 -u` and multi-argument shebangs. Delegating to it avoids reimplementing fragile parsing logic. The temp file solution is required anyway to handle `ARG_MAX` limits for large scripts. Option B would duplicate kernel logic poorly. Option C wastes I/O for backward-compatible no-shebang cases.

## Decision

**We will:** Detect shebangs at execution time and execute script files directly via the kernel's shebang dispatch mechanism, while maintaining backward compatibility for shell-only workflows.

### Implementation Details

1. **Domain Layer** (`internal/domain/ports/executor.go`):
   - Add `IsScriptFile bool` field to `ports.Command` struct
   - Document that `IsScriptFile` signals infrastructure to attempt shebang-based execution

2. **Application Layer** (`internal/application/execution_service.go`):
   - In `resolveStepCommand()`, set `cmd.IsScriptFile = true` when `step.ScriptFile` is non-empty
   - Inline `command` steps get `IsScriptFile = false` (existing behavior)

3. **Infrastructure Layer** (`internal/infrastructure/executor/shell_executor.go`):
   - In `ShellExecutor.Execute()`, when `cmd.IsScriptFile = true`:
     - Check if content starts with `#!` via `strings.HasPrefix(content, "#!")`
     - If shebang found:
       - Write content to temp file via `os.CreateTemp("", "awf-script-*")`
       - Set permissions to `0o700` (owner-executable, secure)
       - `defer os.Remove()` for guaranteed cleanup
       - Execute directly via `exec.CommandContext(ctx, tmpFile)`
       - Let kernel dispatch to correct interpreter
     - If no shebang: fall back to `$SHELL -c` (backward-compatible)
   - All other logic (env propagation, working directory, exit code capture) unchanged

### Testing Strategy

**Unit Tests** (`internal/infrastructure/executor/shell_executor_script_file_test.go`):
- `TestShellExecutor_ScriptFile_ShebangPython` — Python shebang executes via Python
- `TestShellExecutor_ScriptFile_ShebangBash` — Bash shebang works even if `$SHELL` is zsh
- `TestShellExecutor_ScriptFile_NoShebang` — No shebang falls back to `$SHELL -c`
- `TestShellExecutor_ScriptFile_TempFileCleanup` — Cleanup on success and failure
- `TestShellExecutor_ScriptFile_ContextCancellation` — Cleanup on cancellation
- `TestShellExecutor_ScriptFile_EnvPropagation` — Environment variables available
- `TestShellExecutor_ScriptFile_SecretMasking` — Secret masking works

**Application Tests** (`internal/application/execution_service_script_file_test.go`):
- `TestResolveStepCommand_ScriptFile_SetsIsScriptFile` — `step.ScriptFile` → `IsScriptFile=true`
- `TestResolveStepCommand_InlineCommand_IsScriptFileFalse` — `step.Command` → `IsScriptFile=false`

**Integration Tests** (`tests/integration/cli/run_script_file_shebang_test.go`):
- End-to-end: Python shebang, bash shebang, no-shebang, inline command unchanged
- Fixtures: `tests/fixtures/scripts/{shebang_python.py, shebang_bash.sh, no_shebang.sh}`

## Consequences

### What Becomes Easier

- **Polyglot workflows**: Users can mix shell, Python, Ruby, Perl scripts in the same workflow
- **Correct interpreter dispatch**: Scripts execute via their declared interpreter, not the user's login shell
- **Large script support**: Scripts larger than `ARG_MAX` (128KB–2MB) now work reliably
- **Shell variant compatibility**: Bash scripts work on zsh systems and vice versa
- **Minimal code change**: One bool field + one execution branch; backward-compatible zero value

### What Becomes Harder

- **Temp file lifecycle**: Must guarantee cleanup even on panic/cancel (mitigated by `defer`)
- **CI environment constraints**: Some CI runners have `/tmp` mounted `noexec`; mitigated by `t.Skip()` when interpreter unavailable

### Backward Compatibility

✅ **Fully backward-compatible:**
- Scripts without shebang fall back to `$SHELL -c` (existing behavior)
- Inline `command` field unchanged (`IsScriptFile=false`)
- Zero-value `IsScriptFile=false` preserves all existing behavior

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Hexagonal Architecture | Compliant | Domain adds field, application sets flag, infrastructure implements behavior; clean separation |
| Go Idioms | Compliant | `context.Context` propagation, explicit errors, `defer` cleanup; no shortcuts |
| Test-Driven Development | Compliant | Unit tests for shebang detection + temp file execution; integration tests for end-to-end |
| Error Taxonomy | Compliant | Temp file creation failures → system error (exit 4); script execution failures → execution error (exit 3) |
| Security First | Compliant | Temp file `0o700` permissions; `defer` cleanup; secret masking unchanged; no new injection vectors |
| Minimal Abstraction | Compliant | No new interfaces, types, or abstractions — one bool field + one conditional branch |
| Documentation Co-location | Compliant | Updated `Command` struct comment; user guide updated (workflow-syntax.md) |

## Implementation Status

- ✅ Domain: `IsScriptFile` added to `ports.Command`
- ✅ Application: `resolveStepCommand()` sets flag
- ✅ Infrastructure: `ShellExecutor` implements shebang execution
- ✅ Tests: Unit, application, and integration tests passing
- ✅ Fixtures: Script files with various shebangs created
- ✅ Documentation: Backlog marked as implemented; workflow-syntax.md updated
