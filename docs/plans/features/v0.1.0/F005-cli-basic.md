# F005: CLI Basique (run, list, status)

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: high
- **Estimation**: M

## Description

Implement core CLI commands using Cobra: `run` to execute workflows, `list` to show available workflows, `status` to check running workflow state. This is the primary user interface for AWF.

## Acceptance Criteria

- [x] `awf run <workflow> --input=value` executes workflow
- [x] `awf list` shows available workflows
- [x] `awf status <workflow-id>` shows execution status
- [x] `awf version` shows version info
- [x] `awf help` shows usage
- [x] Global flags: --verbose, --quiet, --config, --storage, --no-color, --log-level
- [x] Clear error messages with exit codes (0-4)

## Dependencies

- **Blocked by**: F001, F002, F003, F004
- **Unblocks**: F013

## Impacted Files

```
cmd/awf/main.go
internal/interfaces/cli/root.go
internal/interfaces/cli/config.go
internal/interfaces/cli/exitcodes.go
internal/interfaces/cli/run.go
internal/interfaces/cli/list.go
internal/interfaces/cli/status.go
internal/interfaces/cli/validate.go
internal/interfaces/cli/ui/formatter.go
internal/interfaces/cli/ui/colors.go
tests/integration/cli_test.go
```

## Technical Tasks

- [x] Set up Cobra root command
  - [x] Global flags (verbose, quiet, config, storage, no-color, log-level)
  - [x] Version command
  - [x] Help customization
- [x] Implement `run` command
  - [x] Parse workflow name argument
  - [x] Parse input flags dynamically
  - [x] Call ExecutionService.Run()
  - [x] Display progress output
  - [x] Return appropriate exit code
- [x] Implement `list` command
  - [x] List workflows from repository
  - [x] Show name, description, version
  - [x] Table formatting
- [x] Implement `status` command
  - [x] Load state from store
  - [x] Display current state, progress, duration
  - [x] Show completed/pending steps
- [x] Implement `validate` command
  - [x] Validate workflow without executing
  - [x] Display validation errors
- [x] Implement output formatter
  - [x] Color support (fatih/color)
  - [x] Respect --no-color flag
  - [x] Verbose vs quiet modes
- [x] Write integration tests

## Notes

Use Cobra for CLI framework. Flags should map directly to workflow inputs:

```bash
awf run analyze-code --file-path=app.py --max-tokens=2000
```

Exit codes follow error taxonomy:
- 0: Success
- 1: User error
- 2: Workflow error
- 3: Execution error
- 4: System error
