# Code Review Report: awf (AI Workflow CLI)

**Date:** December 2025
**Reviewers:** Claude Code + Go Expert Agent
**Codebase:** `github.com/vanoix/awf`

---

## Executive Summary

```
┌─────────────────────────────────────────────────────────────┐
│                    QUALITY SCORECARD                        │
├─────────────────────────────────────────────────────────────┤
│  Architecture:     8.5/10  ✓ Clean hexagonal design        │
│  Domain Model:     7.5/10  ⚠ Missing Strategy validation   │
│  Infrastructure:   7.0/10  ⚠ Silent YAML failures          │
│  Testing:          7.5/10  ⚠ Gaps in integration/race      │
│  Error Handling:   7.5/10  ✓ ParseError exists (adequate)  │
│  Security:         8.5/10  ✓ ShellEscape works correctly   │
├─────────────────────────────────────────────────────────────┤
│  OVERALL GRADE:    B+ (7.7/10)                              │
└─────────────────────────────────────────────────────────────┘
```

**Verdict:** Production-ready codebase with solid hexagonal architecture. Minor fixes needed for reliability. No critical security vulnerabilities found.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                        │
│      CLI (Cobra)  │  (API future)  │  (MQ future)          │
│      internal/interfaces/cli/                               │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   APPLICATION LAYER                         │
│   WorkflowService, ExecutionService, HookExecutor           │
│   internal/application/                                     │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                      DOMAIN LAYER                           │
│   workflow/ (entities) │ ports/ (interfaces)               │
│   internal/domain/                                          │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│   repository/  │  store/  │  executor/  │  logger/         │
│   internal/infrastructure/                                  │
└─────────────────────────────────────────────────────────────┘
```

**Compliance:** Hexagonal architecture properly implemented with dependency inversion through ports.

---

## Issues Found

### CRITICAL: Silent YAML Parsing Failures

**Severity:** HIGH
**Location:** `internal/infrastructure/repository/yaml_repository.go:140-149`

**Problem:** Malformed step definitions are silently skipped with `continue`, leaving users unaware of parsing errors.

```go
// CURRENT CODE (lines 140-149)
stepYAML, err := yaml.Marshal(stepMap)
if err != nil {
    continue  // ← Silent failure: user never knows
}

var step yamlStep
if err := yaml.Unmarshal(stepYAML, &step); err != nil {
    continue  // ← Silent failure: malformed step ignored
}
```

**Impact:**
- Workflows load with missing steps
- No error feedback to user
- Debugging becomes difficult

**Fix:** Collect errors and return aggregated error using `errors.Join()` (Go 1.20+).

---

### MEDIUM: Unused `Args` Field in Command Struct

**Severity:** MEDIUM
**Location:** `internal/domain/ports/executor.go:11`

**Problem:** `Args []string` field is defined but never used by `ShellExecutor`.

```go
// internal/domain/ports/executor.go
type Command struct {
    Program string
    Args    []string          // ← Never used
    Dir     string
    Env     map[string]string
    Timeout int
    Stdout  io.Writer
    Stderr  io.Writer
}
```

**Impact:**
- Confusing API: suggests argument array usage, but shell string is used
- Dead code in public interface

**Fix:** Remove the `Args` field since `ShellExecutor` uses `/bin/sh -c` with `Program` as shell string.

---

### MEDIUM: Missing ParallelStrategy Validation

**Severity:** MEDIUM
**Location:** `internal/domain/workflow/step.go:44`

**Problem:** `Strategy` field accepts any string value without validation.

```go
// internal/domain/workflow/step.go
type Step struct {
    // ...
    Strategy string  // ← No validation: any value accepted
    // ...
}

// In Validate() - NO strategy check
func (s *Step) Validate() error {
    // Strategy is NOT validated
}
```

**Valid values:** `"all_succeed"`, `"any_succeed"`, `"best_effort"`, `""`

**Impact:**
- Invalid strategies cause runtime errors, not validation errors
- Poor user experience when typo in YAML (e.g., `stratgey: all_suceed`)

**Fix:** Add validation in `Step.Validate()`:
```go
validStrategies := map[string]bool{
    "": true, "all_succeed": true, "any_succeed": true, "best_effort": true,
}
if s.Type == StepTypeParallel && !validStrategies[s.Strategy] {
    return fmt.Errorf("invalid parallel strategy: %q", s.Strategy)
}
```

---

## Testing Gaps

### Missing Test Categories

| Category | Status | Files Affected |
|----------|--------|----------------|
| Race condition tests | ✓ ADDED | `json_store_test.go` |
| CLI end-to-end tests | ✓ EXPANDED | `cli_test.go` |
| Error recovery scenarios | ⚠ Partial | `execution_test.go` |

### Bug Found During Review

Race condition tests discovered a **real bug**: concurrent `Save` operations to the same workflow ID used the same temp file path, causing corruption. **Fixed** by using unique temp file names with PID and nanosecond timestamp.

### Recommended Tests to Add

1. **Race condition tests for JSONStore**
   ```go
   func TestJSONStore_ConcurrentAccess(t *testing.T) {
       t.Parallel()
       // Test concurrent Save/Load operations
   }
   ```

2. **CLI integration tests**
   - Test `awf run` with valid workflow
   - Test `awf validate` with invalid workflow
   - Test exit codes match error types

---

## False Positives Identified

### NOT a vulnerability: Command Injection

**Initial concern:** `cmd.Program` passed to `/bin/sh -c` could allow injection.

**Reality:**
- Design is intentional to support shell features (pipes, redirects)
- `ShellEscape()` function exists in `pkg/interpolation/escaping.go`
- Interpolation properly escapes user inputs before command construction

**Conclusion:** Security model is sound. No action needed.

### NOT a violation: Logger in CLI Layer

**Initial concern:** `cliLogger` in `internal/interfaces/cli/run.go` instead of infrastructure.

**Reality:**
- Adapters can live in interfaces or infrastructure
- `cliLogger` is CLI-specific output formatting
- No code duplication issue

**Conclusion:** Current location is acceptable.

---

## Strengths

### Architecture
- Clean hexagonal design with proper dependency inversion
- Domain layer has zero infrastructure dependencies
- Ports define clear contracts for adapters

### Code Quality
- Table-driven tests throughout
- Interface-driven design enables mocking
- Atomic file writes with temp+rename pattern

### Security
- Secret masking for sensitive fields (`SECRET_`, `API_KEY`, `PASSWORD`)
- Process group management for clean termination
- Shell escaping for interpolated values

### Infrastructure
- Multiple logger implementations (console, JSON, multi)
- Composite repository with fallback paths
- File locking for concurrent state access

---

## Recommendations

### Immediate Actions (This Sprint)

1. **Fix silent YAML failures** - HIGH priority
2. **Remove unused `Args` field** - Medium priority
3. **Add `ParallelStrategy` validation** - Medium priority

### Future Improvements (Backlog)

- Add race condition tests for `JSONStore`
- Expand CLI integration test coverage
- Consider log rotation for JSON logger
- Add configuration file support

---

## File Reference

| File | Lines | Issue |
|------|-------|-------|
| `internal/infrastructure/repository/yaml_repository.go` | 140-149 | Silent parsing failures |
| `internal/domain/ports/executor.go` | 11 | Unused `Args` field |
| `internal/domain/workflow/step.go` | 44, 56-78 | Missing strategy validation |

---

## Appendix: Validated Architecture

### Dependency Flow (Verified)

```
CLI → Application → Domain → (nothing)
CLI → Infrastructure → Domain → (nothing)
pkg/interpolation → (stdlib only)
```

### No Circular Dependencies

Verified via import analysis. All layers depend inward toward domain.

### Port Implementations

| Port | Implementation | Location |
|------|----------------|----------|
| `WorkflowRepository` | `YAMLRepository`, `CompositeRepository` | `infrastructure/repository/` |
| `CommandExecutor` | `ShellExecutor` | `infrastructure/executor/` |
| `StateStore` | `JSONStore` | `infrastructure/store/` |
| `Logger` | `ConsoleLogger`, `JSONLogger`, `MultiLogger` | `infrastructure/logger/` |

---

*Generated by Claude Code with Go Expert validation*
