# F029: Output Streaming

## Metadata
- **Statut**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: medium
- **Estimation**: M

## Description

Display command output (stdout/stderr) during workflow execution. Currently, output is captured silently and only stored in step state. This feature allows users to see what commands produce in real-time or after each step completes.

Three output modes:
1. **silent** (default): Current behavior, no output displayed
2. **streaming**: Real-time output as commands execute (like `docker build`)
3. **buffered**: Show complete output after each step finishes

Output streams are prefixed to distinguish them:
- `[OUT]` for stdout
- `[ERR]` for stderr

## Critères d'Acceptance

- [x] `awf run workflow --output=streaming` streams output in real-time
- [x] `awf run workflow --output=buffered` shows output after each step
- [x] `awf run workflow` (no flag) keeps current silent behavior
- [x] stdout prefixed with `[OUT]` in cyan
- [x] stderr prefixed with `[ERR]` in red
- [x] `--quiet` flag overrides and suppresses all output
- [x] Parallel step output is interleaved with step name prefix
- [x] Output capture for `{{states.step.output}}` still works in all modes

## Dépendances

- **Bloqué par**: F001, F003, F005
- **Débloque**: _none_

## Fichiers Impactés

```
internal/domain/ports/executor.go          # Add OutputWriter to Command
internal/infrastructure/executor/shell_executor.go  # Implement streaming
internal/interfaces/cli/run.go             # Add --output flag
internal/interfaces/cli/ui/output_writer.go  # New: prefixed writer
internal/application/execution_service.go  # Pass writer to executor
```

## Tâches Techniques

- [ ] Add OutputMode type and flag
  - [ ] Define OutputMode enum (silent, streaming, buffered)
  - [ ] Add `--output` flag to run command
  - [ ] Parse and validate flag value
- [ ] Create OutputWriter component
  - [ ] Implement io.Writer interface
  - [ ] Add prefix support ([OUT], [ERR])
  - [ ] Add color support (cyan for stdout, red for stderr)
  - [ ] Add step name prefix for parallel execution
- [ ] Modify executor port
  - [ ] Add optional Stdout/Stderr io.Writer to Command struct
  - [ ] Maintain backward compatibility (nil = buffer only)
- [ ] Update ShellExecutor
  - [ ] Use io.MultiWriter to write to both buffer and writer
  - [ ] Handle nil writers gracefully
- [ ] Integrate in ExecutionService
  - [ ] Create writers based on output mode
  - [ ] Pass writers through Command struct
- [ ] Handle buffered mode
  - [ ] Store output in buffer
  - [ ] Print after step completion
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Update CLI help documentation

## Notes

- Use `io.MultiWriter` to capture output for state AND stream to terminal simultaneously
- For parallel execution, consider mutex or channel-based serialization to avoid garbled output
- Streaming mode may impact performance slightly due to syscall overhead per line
