# F037: Step Success Feedback

## Metadata
- **Status**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: low
- **Estimation**: XS

## Description

When a step completes successfully but produces no output, display a success message to the user. Currently, steps without output leave the user uncertain whether execution actually occurred. This feature ensures consistent user feedback for all step completions.

## Acceptance Criteria

- [ ] Steps with empty stdout/stderr display "Step completed successfully" (or similar)
- [ ] Steps with actual output display only the output (no extra message)
- [ ] Success message respects `--quiet` flag (hidden when quiet)
- [ ] Success message uses appropriate color (green) when colors enabled
- [ ] Message includes step ID for clarity: `✓ step_id: completed successfully`

## Dependencies

- **Blocked by**: F003, F029
- **Unblocks**: _none_

## Impacted Files

```
internal/application/execution_service.go
internal/interfaces/cli/ui/formatter.go
internal/interfaces/cli/run.go
tests/integration/run_test.go
```

## Technical Tasks

- [ ] Detect empty output after step execution
- [ ] Add success message formatting in UI formatter
- [ ] Integrate with existing output streaming (F029)
- [ ] Respect quiet mode flag
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

Example output:

```
Running workflow: deploy-app

▶ build
  Building application...
  Build complete.

▶ test
  ✓ test: completed successfully

▶ deploy
  Deployed to production.

✓ Workflow completed in 12.3s
```

The success feedback should be subtle - not overwhelming when many steps have no output.
