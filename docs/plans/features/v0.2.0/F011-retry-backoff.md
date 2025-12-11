# F011: Retry avec Backoff Exponentiel

## Metadata
- **Status**: implemented
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priority**: high
- **Estimation**: M

## Description

Implement automatic retry for failed steps with configurable backoff strategies. Support exponential, linear, and constant backoff. Add jitter to prevent thundering herd. Allow filtering by exit codes that are retryable.

## Acceptance Criteria

- [x] Retry on failure up to max_attempts
- [x] Support exponential backoff
- [x] Support linear backoff
- [x] Support constant backoff
- [x] Apply jitter to delays
- [x] Respect max_delay cap
- [x] Only retry specified exit codes
- [x] Log each retry attempt

## Dependencies

- **Blocked by**: F003
- **Unblocks**: _none_

## Impacted Files

```
pkg/retry/retry.go
pkg/retry/retry_test.go
internal/domain/workflow/retry.go
internal/application/executor.go
```

## Technical Tasks

- [x] Define RetryConfig struct
  - [x] MaxAttempts
  - [x] InitialDelay
  - [x] MaxDelay
  - [x] Backoff (constant, linear, exponential)
  - [x] Multiplier
  - [x] Jitter (0.0-1.0)
  - [x] RetryableExitCodes
- [x] Implement Retryer
  - [x] Execute with retry logic
  - [x] Calculate delay based on strategy
  - [x] Apply jitter
  - [x] Check if exit code is retryable
- [x] Implement backoff strategies
  - [x] Constant: always initial_delay
  - [x] Linear: initial_delay * attempt
  - [x] Exponential: initial_delay * multiplier^(attempt-1)
- [x] Implement jitter
  - [x] delay ± (delay * jitter * random)
- [x] Integrate with ExecutionService
  - [x] Wrap step execution in retry
  - [x] Track attempt number in state
- [x] Write unit tests for retry logic
- [x] Write unit tests for backoff calculations

## Notes

Retry timing example (exponential, multiplier=2, jitter=0.1):
```
Attempt 1: immediate
Attempt 2: after 1s   (±0.1s)
Attempt 3: after 2s   (±0.2s)
Attempt 4: after 4s   (±0.4s)
Attempt 5: after 8s   (±0.8s)
```

YAML configuration:
```yaml
retry:
  max_attempts: 5
  initial_delay: 1s
  max_delay: 30s
  backoff: exponential
  multiplier: 2
  jitter: 0.1
  retryable_exit_codes: [1, 2, 130]
```
