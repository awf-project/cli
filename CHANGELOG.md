# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
