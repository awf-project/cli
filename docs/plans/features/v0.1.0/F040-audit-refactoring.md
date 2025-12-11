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
- [x] Overall test coverage reaches 80%+
- [x] CLI layer (`internal/interfaces/cli`) coverage reaches 80%+
- [x] `runWorkflow` function coverage exceeds 80% (currently 46.4%)
- [x] `ExecuteSingleStep` coverage exceeds 80% (currently 63.5%)
- [x] Store layer coverage reaches 80%+ (currently 68.5%)

### Documentation Sync
- [x] F038 spec status corrected from "implemented" to "planned"
- [x] F039 marked as Done in Serena `feature_roadmap` memory
- [x] F039 acceptance criteria checked off in spec
- [x] CHANGELOG updated with F036, F037, F039 entries

### Code Quality
- [x] `pkg/interpolation` error types have tests
- [x] Status command display logic tested
- [x] All `go vet` and `golangci-lint` issues remain at 0

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
- [x] Add tests for CLI `runWorkflow` function
  - [x] Test `showExecutionDetails` (line 310)
  - [x] Test `showStepOutputs` (line 321)
  - [x] Test `showEmptyStepFeedback` (line 341)
  - [x] Test `buildStepInfos` (line 350)
  - [x] Test `ExitCode` (line 396)
  - [x] Test `WithContext` (line 438)
- [x] Add tests for `ExecuteSingleStep` error paths
  - [x] Test step not found scenario
  - [x] Test workflow not found scenario
  - [x] Test execution failure scenario
- [x] Fix F038 spec status (change "implemented" to "planned")

### P1 — High Priority
- [x] Increase store layer coverage to 80%
  - [x] Test `Save` failure paths (disk full, permission denied)
  - [x] Mock `syscall.Flock` failures
- [x] Update Serena `feature_roadmap` memory for F039
- [x] Check off F039 acceptance criteria in spec
- [x] Update CHANGELOG with F036, F037, F039 entries
- [x] Test interpolation error types
  - [x] Add `TestVariableError_Error`
  - [x] Add `TestExecutionError_ErrorAndUnwrap`
- [x] Test status command display logic
  - [x] Test `toExecutionInfo`
  - [x] Test `displayStatus`

### P2 — Medium Priority
- [x] Test text output modes in `ui/output.go`
  - [x] Test `writeExecutionText`
  - [x] Test `writeRunResultText`
  - [x] Test `writeValidationText`
  - [x] Test `calculateDuration`
  - [x] Test `ParseOutputFormat`
- [x] Test `NewApp` entry point (`root.go:24`)
- [x] Consider marking noOpLogger methods as coverage-excluded

### P3 — Low Priority
- [x] Clarify parallel state in README (type exists but execution not implemented)
- [x] Review CLAUDE.md dependencies (remove errgroup mention until F010)
- [x] Delete or test `Source.String()` method if unused

### Documentation
- [x] Update documentation after all changes
- [x] Verify all cross-references between specs, memories, and README

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
