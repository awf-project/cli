# F020: Interactive Mode

## Metadata
- **Status**: backlog
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priority**: low
- **Estimation**: M

## Description

Add interactive execution mode with step-by-step confirmation. Pause before each step, show details, and prompt for continue/skip/abort. Enable debugging and manual intervention in automated workflows.

## Acceptance Criteria

- [ ] `awf run --interactive` enables step-by-step mode
- [ ] Pause before each step with prompt
- [ ] Options: continue, skip, abort, inspect
- [ ] Show step details before execution
- [ ] Show output after execution
- [ ] Allow modifying inputs during execution
- [ ] Support breakpoints on specific states

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

- [ ] Add --interactive flag to run command
- [ ] Add --breakpoint flag for selective pauses
- [ ] Implement InteractiveExecutor
  - [ ] Wrap standard executor
  - [ ] Pause before step
  - [ ] Display step info
  - [ ] Read user input
  - [ ] Handle actions
- [ ] Implement interactive prompt
  - [ ] [c]ontinue - execute step
  - [ ] [s]kip - skip to next state (on_success)
  - [ ] [a]bort - stop workflow
  - [ ] [i]nspect - show context details
  - [ ] [e]dit - modify input value
  - [ ] [r]etry - re-run previous step
- [ ] Display step details
  - [ ] State name and type
  - [ ] Resolved command
  - [ ] Expected transitions
  - [ ] Current context variables
- [ ] Display step result
  - [ ] Exit code
  - [ ] Output (truncated)
  - [ ] Duration
- [ ] Write integration tests

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
