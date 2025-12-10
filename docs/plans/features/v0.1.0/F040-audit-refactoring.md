# F040: Audit-Driven Refactoring

## Metadata
- **Status**: implemented
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: high
- **Estimation**: L

## Description

Organize and execute refactoring tasks identified in the project audit report. The audit revealed a project health score of 8/10 with test coverage at 70.9% (below the 80% target). This feature addresses critical issues including CLI layer undertesting (53% coverage), documentation desynchronization, and missing CHANGELOG entries.

## Acceptance Criteria

### Coverage Targets
- [ ] Overall test coverage reaches 80%+
- [ ] CLI layer (`internal/interfaces/cli`) coverage reaches 80%+
- [ ] `runWorkflow` function coverage exceeds 80% (currently 46.4%)
- [ ] `ExecuteSingleStep` coverage exceeds 80% (currently 63.5%)
- [ ] Store layer coverage reaches 80%+ (currently 68.5%)

### Documentation Sync
- [ ] F038 spec status corrected from "implemented" to "planned"
- [ ] F039 marked as Done in Serena `feature_roadmap` memory
- [ ] F039 acceptance criteria checked off in spec
- [ ] CHANGELOG updated with F036, F037, F039 entries

### Code Quality
- [ ] `pkg/interpolation` error types have tests
- [ ] Status command display logic tested
- [ ] All `go vet` and `golangci-lint` issues remain at 0

## Dependencies

- **Blocked by**: _none_
- **Unblocks**: v0.1.0 release readiness

## Impacted Files

```
internal/interfaces/cli/run.go
internal/interfaces/cli/run_test.go
internal/interfaces/cli/status.go
internal/interfaces/cli/status_test.go
internal/application/single_step.go
internal/application/single_step_test.go
internal/infrastructure/store/json_store.go
internal/infrastructure/store/json_store_test.go
pkg/interpolation/errors.go
pkg/interpolation/errors_test.go
docs/plans/features/v0.1.0/F038-prompt-storage.md
docs/plans/features/v0.1.0/F039-run-single-step.md
CHANGELOG.md
.serena/memories/feature_roadmap.md
```

## Technical Tasks

### P0 — Critical (Must Fix)
- [ ] Add tests for CLI `runWorkflow` function
  - [ ] Test `showExecutionDetails` (line 310)
  - [ ] Test `showStepOutputs` (line 321)
  - [ ] Test `showEmptyStepFeedback` (line 341)
  - [ ] Test `buildStepInfos` (line 350)
  - [ ] Test `ExitCode` (line 396)
  - [ ] Test `WithContext` (line 438)
- [ ] Add tests for `ExecuteSingleStep` error paths
  - [ ] Test step not found scenario
  - [ ] Test workflow not found scenario
  - [ ] Test execution failure scenario
- [ ] Fix F038 spec status (change "implemented" to "planned")

### P1 — High Priority
- [ ] Increase store layer coverage to 80%
  - [ ] Test `Save` failure paths (disk full, permission denied)
  - [ ] Mock `syscall.Flock` failures
- [ ] Update Serena `feature_roadmap` memory for F039
- [ ] Check off F039 acceptance criteria in spec
- [ ] Update CHANGELOG with F036, F037, F039 entries
- [ ] Test interpolation error types
  - [ ] Add `TestVariableError_Error`
  - [ ] Add `TestExecutionError_ErrorAndUnwrap`
- [ ] Test status command display logic
  - [ ] Test `toExecutionInfo`
  - [ ] Test `displayStatus`

### P2 — Medium Priority
- [ ] Test text output modes in `ui/output.go`
  - [ ] Test `writeExecutionText`
  - [ ] Test `writeRunResultText`
  - [ ] Test `writeValidationText`
  - [ ] Test `calculateDuration`
  - [ ] Test `ParseOutputFormat`
- [ ] Test `NewApp` entry point (`root.go:24`)
- [ ] Consider marking noOpLogger methods as coverage-excluded

### P3 — Low Priority
- [ ] Clarify parallel state in README (type exists but execution not implemented)
- [ ] Review CLAUDE.md dependencies (remove errgroup mention until F010)
- [ ] Delete or test `Source.String()` method if unused

### Documentation
- [ ] Update documentation after all changes
- [ ] Verify all cross-references between specs, memories, and README

## Notes

### Audit Summary
- **Overall Score**: 8/10
- **Architecture Score**: 9/10 (exemplary hexagonal implementation)
- **Test Coverage**: 70.9% (target: 80%)
- **CLI Coverage**: 53% (biggest gap)

### Estimated Effort
- Test writing: ~10 hours
- Documentation fixes: ~1.5 hours

### Key Metrics to Track
| Layer | Current | Target |
|-------|---------|--------|
| Domain | 92.2% | 80% ✅ |
| Infrastructure | 83-100% | 80% ✅ |
| Application | 80.4% | 80% ✅ |
| CLI | 53.0% | 80% ❌ |
| Store | 68.5% | 80% ❌ |
| Overall | 70.9% | 80% ❌ |

### Reference
- Full audit report: `audit-report.md`
- Branch: `feature/F039-run-single-step`
