# F019: Dry-Run Mode

## Metadata
- **Status**: implemented
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: medium
- **Estimation**: S

## Description

Add dry-run mode that shows what would be executed without actually running commands. Display resolved commands, transitions, and interpolated values. Useful for debugging and validation before actual execution.

## Acceptance Criteria

- [x] `awf run --dry-run` shows execution plan
- [x] Display resolved commands with interpolation
- [x] Show state transitions
- [x] Validate workflow without execution
- [x] Show hooks that would run
- [x] No side effects (no files written, commands run)

## Dependencies

- **Blocked by**: F005, F007
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/cli/commands/run.go
internal/application/dry_run_executor.go
internal/interfaces/cli/ui/dry_run_formatter.go
```

## Technical Tasks

- [x] Add --dry-run flag to run command
- [x] Implement DryRunExecutor
  - [x] Walk through states without execution
  - [x] Resolve interpolations
  - [x] Collect execution plan
  - [x] Return structured plan
- [x] Implement DryRunFormatter
  - [x] Display step-by-step plan
  - [x] Show resolved commands
  - [x] Show variable values
  - [x] Highlight hooks
- [x] Handle conditional transitions
  - [x] Show all possible paths
  - [x] Or require mock values
- [x] Handle parallel states
  - [x] Show all parallel branches
- [x] Add --dry-run-inputs for mock values
- [x] Write unit tests

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
