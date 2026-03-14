---
title: "013. Context-Aware Port Interfaces for User Input"
---

Date: 2026-03-03
Status: Accepted
Issue: B008

## Context

`InputCollector.PromptForInput`, `UserInteraction.PromptAction`, and `UserInteraction.EditInput` use blocking `bufio.Reader.ReadString('\n')` that ignores context cancellation. SIGINT cancels the context but the blocked read prevents process termination, causing a hang.

## Decision

Add `ctx context.Context` as the first parameter to all three port methods. Implementations use a goroutine+select+channel pattern: `ReadString` runs in a goroutine, result sent on a buffered channel (capacity 1), `select` races the channel against `ctx.Done()`. A shared `readLineWithContext(ctx, reader)` helper is extracted to avoid duplicating the pattern across three call sites. Context cancellation returns `fmt.Errorf("input cancelled: %w", ctx.Err())` so callers can use `errors.Is(err, context.Canceled)`.

Alternatives rejected:
- **`SetReadDeadline` polling**: Requires `*os.File`, breaks `io.Reader` test mocking with `strings.NewReader`
- **Close stdin on signal**: Irreversible side effect; affects all code referencing stdin; hard to test
- **`signal.Reset(syscall.SIGINT)` during reads**: Bypasses signal handler, prevents graceful cleanup of other resources

## Consequences

### Positive
- Ctrl+C during any input prompt exits cleanly (no hang)
- Follows Go convention: `context.Context` as first parameter matches `CommandExecutor.Execute(ctx, cmd)`
- `readLineWithContext` reuses the goroutine+channel pattern already in `CLIInputCollector.validate()`
- Fully testable with `strings.NewReader` — no `*os.File` dependency

### Negative
- Breaking port change: all callers and mock implementations must be updated (compiler-guided, ~50 call sites)
- One goroutine leaks per cancelled read (~2KB stack); cleaned up at process exit since cancellation precedes termination
