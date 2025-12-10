# F039: Run Single Step

## Metadata
- **Status**: implemented
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
- **Estimation**: S

## Description

Add the ability to execute only a specific step from a workflow using a `--step` flag. This is useful for:
- Debugging a specific step without running the entire workflow
- Re-running a failed step after fixing an issue
- Testing step behavior in isolation

## Acceptance Criteria

- [x] `awf run workflow.yaml --step=step_name` executes only that step
- [x] Step inputs can be provided via `--input` flags
- [x] Step dependencies (previous state outputs) can be mocked via `--mock` or `--state` flags
- [x] Clear error message if step doesn't exist
- [x] Step hooks (pre/post) are executed
- [x] Output is captured and displayed normally

## Dependencies

- **Blocked by**: F001 (Core State Machine)
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/cli/run.go          # Add --step flag
internal/application/execution_service.go  # Single step execution logic
internal/domain/workflow/workflow.go    # GetStep() method if missing
```

## Technical Tasks

- [ ] Add `--step` flag to `run` command
- [ ] Add `--mock` flag for injecting state values
- [ ] Implement `ExecuteSingleStep()` in ExecutionService
- [ ] Add step validation (check step exists)
- [ ] Handle step output capture
- [ ] Write unit tests
- [ ] Update CLI help/documentation

## Notes

- Single step execution skips state machine transitions
- Consider adding `--dry-run` combination to show what would execute
- Mock data format: `--mock states.step_name.output="value"`
