# F019: Dry-Run Mode

## Metadata
- **Statut**: backlog
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priorité**: medium
- **Estimation**: S

## Description

Add dry-run mode that shows what would be executed without actually running commands. Display resolved commands, transitions, and interpolated values. Useful for debugging and validation before actual execution.

## Critères d'Acceptance

- [ ] `awf run --dry-run` shows execution plan
- [ ] Display resolved commands with interpolation
- [ ] Show state transitions
- [ ] Validate workflow without execution
- [ ] Show hooks that would run
- [ ] No side effects (no files written, commands run)

## Dépendances

- **Bloqué par**: F005, F007
- **Débloque**: _none_

## Fichiers Impactés

```
internal/interfaces/cli/commands/run.go
internal/application/dry_run_executor.go
internal/interfaces/cli/ui/dry_run_formatter.go
```

## Tâches Techniques

- [ ] Add --dry-run flag to run command
- [ ] Implement DryRunExecutor
  - [ ] Walk through states without execution
  - [ ] Resolve interpolations
  - [ ] Collect execution plan
  - [ ] Return structured plan
- [ ] Implement DryRunFormatter
  - [ ] Display step-by-step plan
  - [ ] Show resolved commands
  - [ ] Show variable values
  - [ ] Highlight hooks
- [ ] Handle conditional transitions
  - [ ] Show all possible paths
  - [ ] Or require mock values
- [ ] Handle parallel states
  - [ ] Show all parallel branches
- [ ] Add --dry-run-inputs for mock values
- [ ] Write unit tests

## Notes

Dry-run output example:
```bash
$ awf run analyze-code --file-path=app.py --dry-run

Dry Run: analyze-code
======================

Inputs:
  file_path: app.py
  output_format: markdown (default)

Execution Plan:
---------------

[1] validate
    Hook (pre): log "Validating file: app.py"
    Command: test -f "app.py" && echo "valid"
    Timeout: 5s
    → on_success: extract
    → on_failure: error

[2] extract
    Command: cat "app.py"
    Capture: stdout → file_content
    Timeout: 10s
    → on_success: analyze

[3] analyze
    Hook (pre): log "Analyzing with Claude..."
    Command: claude -c "Analyze this code..."
    Retry: 3 attempts, exponential backoff
    → on_success: format_output
    → on_failure: try_gemini

...

No commands will be executed (dry-run mode).
```
