# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **F041**: Validate Template Interpolation References
  - Static validation of `{{inputs.X}}` and `{{states.X.output}}` at parse time
  - Detect forward references (step A references step B before B runs)
  - Report all template errors in single pass with step context
- **F014**: BadgerDB History
  - Store execution history in BadgerDB (pure Go, no CGO)
  - `awf history` command with `--workflow`, `--status`, `--since` filters
  - Statistics with `--stats` flag
  - 30-day auto-cleanup at startup
- **F013**: Resume Command
  - `awf resume <workflow-id>` to continue interrupted workflows
  - `awf resume --list` shows resumable workflows
  - Input override on resume with `--input` flags
- **F012**: Input Validation
  - Validate workflow inputs at runtime against defined rules
  - Type checking: `string`, `integer`, `boolean`
  - Pattern validation (regex), enum, numeric ranges (`min`/`max`)
  - File validation: `file_exists`, `file_extension`
- **F011**: Retry with Exponential Backoff
  - Automatic retry for failed steps with configurable `max_attempts`
  - Backoff strategies: `exponential`, `linear`, `constant`
  - Jitter support to prevent thundering herd
  - Filter retryable failures by `retryable_exit_codes`
- **F010**: Parallel step execution (errgroup)
  - `type: parallel` state with concurrent step execution
  - Strategies: `all_succeed`, `any_succeed`, `best_effort`
  - `max_concurrent` limit with semaphore control
  - Context cancellation on first failure (all_succeed mode)
- **F009**: State machine with conditional transitions
  - Follow `on_success` transition on exit code 0, `on_failure` on non-zero
  - Terminal states (`type: terminal`) with `status: success|failure`
  - State graph validation: detect cycles, unreachable states, missing terminals
  - `continue_on_error` flag to always follow `on_success` path
- **F039**: Single step execution with `--step` flag for debugging and testing individual workflow steps
  - Execute specific steps: `awf run workflow.yaml --step=step_name`
  - Mock state dependencies: `--mock states.prev_step.output="value"`
  - Step hooks (pre/post) execute normally
- **F037**: Step success feedback for steps with no output in silent/streaming modes
  - Shows `✓ step_name completed successfully` for empty-output steps
- **F036**: CLI init command (`awf init`) to initialize AWF in current directory
  - Creates `.awf/workflows/` and `.awf/prompts/` directories
  - Creates example workflow file
- ParallelStrategy validation: Invalid strategies now fail at validation time
  - Valid values: `all_succeed`, `any_succeed`, `best_effort`, or empty (default)
  - Invalid strategies return clear error message
- Race condition tests for JSONStore (`TestJSONStore_RaceSaveLoad`, `TestJSONStore_RaceSaveDelete`, `TestJSONStore_RaceListSave`)
- CLI integration tests expanded:
  - `TestCLI_Run_FailingCommand_Integration` - workflow with failing command
  - `TestCLI_Validate_InvalidStrategy_Integration` - validates strategy error message
  - `TestCLI_Run_OutputModes_Integration` - tests quiet/verbose/default modes
  - `TestCLI_Run_MultiStep_Integration` - multi-step workflow execution
- Code review report: `docs/code-review-2025-12.md`

### Changed

- YAML parsing now reports all errors instead of silently skipping malformed steps
  - Uses `errors.Join()` to aggregate multiple parsing errors
  - Users get detailed feedback on which states failed to parse

### Fixed

- **Race condition in JSONStore**: Concurrent `Save` operations to the same workflow ID could corrupt the state file. Fixed by using unique temp file names with PID and nanosecond timestamp.

### Removed

- **BREAKING**: `Args` field removed from `ports.Command` struct
  - This field was never used by `ShellExecutor`
  - Commands are passed as shell strings to `/bin/sh -c`
  - Use `ShellEscape()` from `pkg/interpolation` for user-provided values
