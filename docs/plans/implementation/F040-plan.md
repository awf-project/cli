I now have all the information I need to create a comprehensive implementation plan for F040. Let me write it:

# Implementation Plan: F040 - Audit-Driven Refactoring

## Summary

F040 addresses the 70.9% overall test coverage (below 80% target) identified in the project audit, with the CLI layer being the biggest gap at 53%. The implementation focuses on adding targeted tests for untested functions in `run.go`, `status.go`, `single_step.go`, `json_store.go`, and `pkg/interpolation/errors.go`, along with fixing documentation desynchronization (F038 status, F039 checklist, CHANGELOG entries).

## ASCII Wireframe: Test Coverage Target

```
┌─────────────────────────────────────────────────────────────────┐
│                      CURRENT vs TARGET                          │
├─────────────────────────────────────────────────────────────────┤
│  Layer         Current   Target   Status   Action Required      │
├─────────────────────────────────────────────────────────────────┤
│  Domain        92.2%     80%      ✅ OK    None                 │
│  Application   80.4%     80%      ✅ OK    None                 │
│  Infra/Store   68.5%     80%      ❌ GAP   +12% (~6 tests)      │
│  CLI           53.0%     80%      ❌ GAP   +27% (~15 tests)     │
│  CLI/UI        72.0%     80%      ❌ GAP   +8% (~5 tests)       │
│  Interpolation 78.3%     80%      ❌ GAP   +2% (~2 tests)       │
├─────────────────────────────────────────────────────────────────┤
│  OVERALL       70.9%     80%      ❌ GAP   ~28 new tests        │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Phase 1: Documentation Sync (P1 - Quick Wins)

1. **Fix F038 spec status**
   - File: `docs/plans/features/v0.1.0/F038-prompt-storage.md`
   - Action: MODIFY
   - Changes: Line 3: `Status: implemented` → `Status: planned`

2. **Update F039 acceptance criteria**
   - File: `docs/plans/features/v0.1.0/F039-run-single-step.md`
   - Action: MODIFY
   - Changes: Check off all acceptance criteria boxes (lines 18-25)

3. **Update Serena feature_roadmap memory**
   - File: `.serena/memories/feature_roadmap.md`
   - Action: MODIFY
   - Changes: Mark F039 as `✅ Done`

4. **Add CHANGELOG entries for F036, F037, F039**
   - File: `CHANGELOG.md`
   - Action: MODIFY
   - Changes: Add entries under `[Unreleased]` → `### Added`

### Phase 2: CLI Layer Tests (P0 - Critical)

5. **Add tests for `run.go` helper functions**
   - File: `internal/interfaces/cli/run_test.go`
   - Action: MODIFY
   - Changes: Add table-driven tests for:
     ```go
     // Test showExecutionDetails (line 310)
     func TestShowExecutionDetails(t *testing.T)
     
     // Test showStepOutputs (line 321)
     func TestShowStepOutputs(t *testing.T)
     
     // Test showEmptyStepFeedback (line 341)
     func TestShowEmptyStepFeedback(t *testing.T)
     
     // Test buildStepInfos (line 350)
     func TestBuildStepInfos(t *testing.T)
     
     // Test categorizeError (line 367)
     func TestCategorizeError(t *testing.T)
     
     // Test exitError.ExitCode() (line 396)
     func TestExitError_ExitCode(t *testing.T)
     
     // Test cliLogger.WithContext (line 438)
     func TestCliLogger_WithContext(t *testing.T)
     ```

6. **Add tests for `status.go` display functions**
   - File: `internal/interfaces/cli/status_test.go`
   - Action: MODIFY
   - Changes: Add tests for:
     ```go
     // Test toExecutionInfo conversion (line 71)
     func TestToExecutionInfo(t *testing.T)
     
     // Test displayStatus output (line 115)  
     func TestDisplayStatus(t *testing.T)
     ```

### Phase 3: Interpolation Error Tests (P1)

7. **Add tests for interpolation error types**
   - File: `pkg/interpolation/errors_test.go`
   - Action: CREATE
   - Changes:
     ```go
     func TestUndefinedVariableError_Error(t *testing.T)
     func TestParseError_Error(t *testing.T)
     func TestParseError_Unwrap(t *testing.T)
     ```

### Phase 4: Store Layer Tests (P1)

8. **Add failure path tests for JSONStore**
   - File: `internal/infrastructure/store/json_store_test.go`
   - Action: MODIFY
   - Changes:
     ```go
     // Test Save with read-only directory
     func TestJSONStore_Save_PermissionDenied(t *testing.T)
     
     // Test Load with unreadable file
     func TestJSONStore_Load_PermissionDenied(t *testing.T)
     
     // Test List with invalid glob pattern edge cases
     func TestJSONStore_List_IgnoresNonJSONFiles(t *testing.T)
     ```

### Phase 5: UI Output Tests (P2)

9. **Add tests for text output modes**
   - File: `internal/interfaces/cli/ui/output_test.go`
   - Action: MODIFY
   - Changes:
     ```go
     // Test writeExecutionText (line 400)
     func TestOutputWriter_WriteExecution_Text(t *testing.T)
     
     // Test writeRunResultText (line 426)
     func TestOutputWriter_WriteRunResult_Text(t *testing.T)
     
     // Test writeValidationText (line 438)
     func TestOutputWriter_WriteValidation_Text(t *testing.T)
     
     // Test calculateDuration helper (line 614)
     func TestCalculateDuration(t *testing.T)
     
     // Test ParseOutputFormat (line 38)
     func TestParseOutputFormat(t *testing.T)
     ```

### Phase 6: Root Command Test (P2)

10. **Add test for NewApp entry point**
    - File: `internal/interfaces/cli/root_test.go`
    - Action: MODIFY (or CREATE if doesn't exist)
    - Changes:
      ```go
      func TestNewRootCommand_AllSubcommands(t *testing.T)
      func TestNewApp_ReturnsNonNil(t *testing.T)
      ```

## Test Plan

### Unit Tests
| Test File | New Tests | Coverage Impact |
|-----------|-----------|-----------------|
| `run_test.go` | 7 | CLI +15% |
| `status_test.go` | 2 | CLI +5% |
| `errors_test.go` | 3 | interpolation +2% |
| `json_store_test.go` | 3 | store +5% |
| `output_test.go` | 5 | ui +8% |
| `root_test.go` | 2 | CLI +2% |

### Integration Tests
- Existing integration tests already cover most scenarios
- No new integration tests required

### Verification Commands
```bash
# Run unit tests with coverage
make test-coverage

# Check per-package coverage
go test -cover ./internal/interfaces/cli/...
go test -cover ./internal/infrastructure/store/...
go test -cover ./pkg/interpolation/...

# Run race detector
make test-race
```

## Files to Modify

| File | Action | Complexity | Priority |
|------|--------|------------|----------|
| `docs/plans/features/v0.1.0/F038-prompt-storage.md` | MODIFY | S | P1 |
| `docs/plans/features/v0.1.0/F039-run-single-step.md` | MODIFY | S | P1 |
| `.serena/memories/feature_roadmap.md` | MODIFY | S | P1 |
| `CHANGELOG.md` | MODIFY | S | P1 |
| `internal/interfaces/cli/run_test.go` | MODIFY | L | P0 |
| `internal/interfaces/cli/status_test.go` | MODIFY | M | P0 |
| `pkg/interpolation/errors_test.go` | CREATE | S | P1 |
| `internal/infrastructure/store/json_store_test.go` | MODIFY | M | P1 |
| `internal/interfaces/cli/ui/output_test.go` | MODIFY | M | P2 |
| `internal/interfaces/cli/root_test.go` | MODIFY | S | P2 |

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Helper functions in `run.go` are unexported | Medium | Export functions for testing OR use `_test.go` in same package (`cli` not `cli_test`) to access unexported functions |
| `syscall.Flock` cannot be mocked easily | Low | Test permission-denied paths with read-only directories instead |
| Signal handling in `runWorkflow` is untestable | Low | Accept this gap - signal handling tested via integration tests |
| Some UI formatting is difficult to verify precisely | Low | Test for key content presence, not exact formatting |

## Dependencies

- No external dependencies required
- All test infrastructure already exists (testify, mock patterns)
- Existing mock implementations in `application/*_test.go` can be reused

## Estimated Effort

| Phase | Effort |
|-------|--------|
| Documentation Sync (4 files) | ~1h |
| CLI Layer Tests (7 tests) | ~4h |
| Interpolation Error Tests (3 tests) | ~1h |
| Store Layer Tests (3 tests) | ~2h |
| UI Output Tests (5 tests) | ~2h |
| Root Command Test (2 tests) | ~0.5h |
| **Total** | **~10.5h** |

