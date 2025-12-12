# F020: Interactive Mode

## Metadata
- **Status**: implemented
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: low
- **Estimation**: M

## Description

Add interactive execution mode with step-by-step confirmation. Pause before each step, show details, and prompt for continue/skip/abort. Enable debugging and manual intervention in automated workflows.

## Acceptance Criteria

- [x] `awf run --interactive` enables step-by-step mode
- [x] Pause before each step with prompt
- [x] Options: continue, skip, abort, inspect
- [x] Show step details before execution
- [x] Show output after execution
- [x] Allow modifying inputs during execution
- [x] Support breakpoints on specific states

## Dependencies

- **Blocked by**: F005
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/cli/commands/run.go
internal/application/interactive_executor.go
internal/interfaces/cli/ui/prompt.go
```

## Technical Tasks

- [x] Add --interactive flag to run command
- [x] Add --breakpoint flag for selective pauses
- [x] Implement InteractiveExecutor
  - [x] Wrap standard executor
  - [x] Pause before step
  - [x] Display step info
  - [x] Read user input
  - [x] Handle actions
- [x] Implement interactive prompt
  - [x] [c]ontinue - execute step
  - [x] [s]kip - skip to next state (on_success)
  - [x] [a]bort - stop workflow
  - [x] [i]nspect - show context details
  - [x] [e]dit - modify input value
  - [x] [r]etry - re-run previous step
- [x] Display step details
  - [x] State name and type
  - [x] Resolved command
  - [x] Expected transitions
  - [x] Current context variables
- [x] Display step result
  - [x] Exit code
  - [x] Output (truncated)
  - [x] Duration
- [x] Write integration tests

## Notes

Interactive session example:
```
$ awf run analyze-code --file-path=app.py --interactive

Interactive Mode: analyze-code
==============================

[Step 1/5] validate
Type: step
Command: test -f "app.py" && echo "valid"
Timeout: 5s

[c]ontinue [s]kip [a]bort [i]nspect > c

Executing...
Output: valid
Exit code: 0
Duration: 0.1s

→ Next: extract

[Step 2/5] extract
Type: step
Command: cat "app.py"

[c]ontinue [s]kip [a]bort [i]nspect > i

Context:
  inputs.file_path: app.py
  states.validate.output: valid
  states.validate.exit_code: 0

[c]ontinue [s]kip [a]bort [i]nspect > c
...
```
