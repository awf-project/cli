# Implementation Plan: F011 - Retry with Backoff

## Summary

Implement automatic retry for failed steps with configurable backoff strategies (constant, linear, exponential) and jitter support. The domain model (`RetryConfig`) and YAML parsing (`mapRetry`) already exist - the work is creating the `pkg/retry` package for backoff calculations and integrating retry logic into `execution_service.go:executeStep()`.

## Implementation Steps

### Step 1: Create Backoff Strategy Types
- **File:** `pkg/retry/backoff.go`
- **Action:** CREATE
- **Changes:**
  ```go
  type Strategy string
  const (
      StrategyConstant    Strategy = "constant"
      StrategyLinear      Strategy = "linear"
      StrategyExponential Strategy = "exponential"
  )
  
  func CalculateDelay(strategy Strategy, attempt int, initialDelay, maxDelay time.Duration, multiplier float64) time.Duration
  func ApplyJitter(delay time.Duration, jitter float64) time.Duration
  ```
  - Constant: always `initialDelay`
  - Linear: `initialDelay * attempt`
  - Exponential: `initialDelay * multiplier^(attempt-1)`
  - Jitter: `delay ± (delay * jitter * random)`
  - Cap at `maxDelay`

### Step 2: Create Retryer Implementation
- **File:** `pkg/retry/retryer.go`
- **Action:** CREATE
- **Changes:**
  ```go
  type Config struct {
      MaxAttempts        int
      InitialDelay       time.Duration
      MaxDelay           time.Duration
      Strategy           Strategy
      Multiplier         float64
      Jitter             float64
      RetryableExitCodes []int
  }
  
  type Retryer struct { config Config; logger Logger }
  
  func (r *Retryer) ShouldRetry(exitCode int, attempt int) bool
  func (r *Retryer) NextDelay(attempt int) time.Duration
  ```
  - Empty `RetryableExitCodes` = retry any non-zero exit code
  - Respect context cancellation during sleep
  - Log each retry attempt

### Step 3: Create Unit Tests for Retry Package
- **File:** `pkg/retry/retry_test.go`
- **Action:** CREATE
- **Changes:**
  - Table-driven tests for `CalculateDelay` with all strategies
  - Tests for jitter bounds (delay ± jitter%)
  - Tests for `ShouldRetry` with exit code filtering
  - Tests for max delay capping

### Step 4: Integrate Retry into ExecutionService
- **File:** `internal/application/execution_service.go`
- **Action:** MODIFY
- **Changes:**
  - Add `import "github.com/vanoix/awf/pkg/retry"`
  - In `executeStep()` (around line 215), wrap command execution with retry:
  ```go
  // After resolving command, before execute
  if step.Retry != nil && step.Retry.MaxAttempts > 1 {
      result, execErr = s.executeWithRetry(stepCtx, step, cmd)
  } else {
      result, execErr = s.executor.Execute(stepCtx, cmd)
  }
  ```
  - Add new method `executeWithRetry(ctx, step, cmd)`:
    - Loop up to `MaxAttempts`
    - Check `ShouldRetry(exitCode, attempt)`
    - Sleep with calculated delay (respecting ctx)
    - Log retry attempts
    - Track `state.Attempt` (already exists in `StepState`)

### Step 5: Add Retry Configuration Validation
- **File:** `internal/domain/workflow/step.go`
- **Action:** MODIFY (minor)
- **Changes:**
  - Add validation in `Step.Validate()` for `RetryConfig`:
    - `MaxAttempts >= 1` (default 1 = no retry)
    - `Backoff` must be "constant", "linear", "exponential", or empty
    - `Jitter` must be in range `[0.0, 1.0]`
    - `Multiplier >= 1.0` for exponential

### Step 6: Add Integration Test
- **File:** `tests/integration/retry_test.go`
- **Action:** CREATE
- **Changes:**
  - Test workflow with failing step that succeeds on 3rd attempt
  - Verify retry count in execution state
  - Test exit code filtering (retry only on specific codes)

## Test Plan

### Unit Tests (`pkg/retry/retry_test.go`)
- [ ] `TestCalculateDelay_Constant` - always returns initial delay
- [ ] `TestCalculateDelay_Linear` - delay = initial * attempt
- [ ] `TestCalculateDelay_Exponential` - delay = initial * multiplier^(attempt-1)
- [ ] `TestCalculateDelay_CapsAtMaxDelay` - never exceeds max
- [ ] `TestApplyJitter_WithinBounds` - result within ±jitter%
- [ ] `TestApplyJitter_ZeroJitter` - returns exact delay
- [ ] `TestShouldRetry_EmptyCodesRetriesAll` - empty = retry any non-zero
- [ ] `TestShouldRetry_SpecificCodes` - only retry listed codes
- [ ] `TestShouldRetry_ExceedsMaxAttempts` - returns false when exhausted

### Unit Tests (`internal/application/execution_service_test.go`)
- [ ] `TestExecuteStep_WithRetry_SucceedsOnRetry` - fails first, succeeds on retry
- [ ] `TestExecuteStep_WithRetry_ExhaustsAttempts` - fails after max attempts
- [ ] `TestExecuteStep_WithRetry_ContextCancelled` - stops retry on cancellation
- [ ] `TestExecuteStep_NoRetryConfig` - behaves normally without retry

### Integration Tests (`tests/integration/retry_test.go`)
- [ ] End-to-end workflow with retry configuration
- [ ] Verify `Attempt` field populated in state

## Files to Modify

| File | Action | Complexity |
|------|--------|------------|
| `pkg/retry/backoff.go` | CREATE | S |
| `pkg/retry/retryer.go` | CREATE | M |
| `pkg/retry/retry_test.go` | CREATE | M |
| `internal/application/execution_service.go` | MODIFY | M |
| `internal/domain/workflow/step.go` | MODIFY | S |
| `tests/integration/retry_test.go` | CREATE | S |

## Risks

| Risk | Mitigation |
|------|------------|
| **Jitter randomness not deterministic in tests** | Use `rand.New(rand.NewSource(seed))` for testable randomness |
| **Context not checked during sleep** | Use `time.NewTimer` + `select` with `ctx.Done()` |
| **Retry on timeout vs command failure ambiguity** | Timeout errors should NOT trigger retry (context error) |
| **Logging floods on many retries** | Log at Debug level, summary at Info |
| **State persistence between attempts** | Keep intermediate states? No - only final state matters |

## Sequence Diagram

```
executeStep()
    │
    ▼
┌───────────────────┐
│ step.Retry != nil │──No──► Execute once
└─────────┬─────────┘
          │Yes
          ▼
    ┌─────────────┐
    │ attempt = 1 │
    └──────┬──────┘
           │
    ┌──────▼──────┐
    │   Execute   │
    └──────┬──────┘
           │
    ┌──────▼──────────────┐
    │ Success/Exit 0?     │──Yes──► Return success
    └──────┬──────────────┘
           │No
    ┌──────▼──────────────┐
    │ ShouldRetry(exit)?  │──No───► Return failure
    └──────┬──────────────┘
           │Yes
    ┌──────▼──────────────┐
    │ attempt < max?      │──No───► Return failure (exhausted)
    └──────┬──────────────┘
           │Yes
    ┌──────▼──────────────┐
    │ Sleep(delay+jitter) │
    │ (check ctx.Done())  │
    └──────┬──────────────┘
           │
           │ attempt++
           └────────► back to Execute
```

