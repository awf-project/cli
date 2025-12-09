# F011: Retry avec Backoff Exponentiel

## Metadata
- **Statut**: backlog
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priorité**: high
- **Estimation**: M

## Description

Implement automatic retry for failed steps with configurable backoff strategies. Support exponential, linear, and constant backoff. Add jitter to prevent thundering herd. Allow filtering by exit codes that are retryable.

## Critères d'Acceptance

- [ ] Retry on failure up to max_attempts
- [ ] Support exponential backoff
- [ ] Support linear backoff
- [ ] Support constant backoff
- [ ] Apply jitter to delays
- [ ] Respect max_delay cap
- [ ] Only retry specified exit codes
- [ ] Log each retry attempt

## Dépendances

- **Bloqué par**: F003
- **Débloque**: _none_

## Fichiers Impactés

```
pkg/retry/retry.go
pkg/retry/retry_test.go
internal/domain/workflow/retry.go
internal/application/executor.go
```

## Tâches Techniques

- [ ] Define RetryConfig struct
  - [ ] MaxAttempts
  - [ ] InitialDelay
  - [ ] MaxDelay
  - [ ] Backoff (constant, linear, exponential)
  - [ ] Multiplier
  - [ ] Jitter (0.0-1.0)
  - [ ] RetryableExitCodes
- [ ] Implement Retryer
  - [ ] Execute with retry logic
  - [ ] Calculate delay based on strategy
  - [ ] Apply jitter
  - [ ] Check if exit code is retryable
- [ ] Implement backoff strategies
  - [ ] Constant: always initial_delay
  - [ ] Linear: initial_delay * attempt
  - [ ] Exponential: initial_delay * multiplier^(attempt-1)
- [ ] Implement jitter
  - [ ] delay ± (delay * jitter * random)
- [ ] Integrate with ExecutionService
  - [ ] Wrap step execution in retry
  - [ ] Track attempt number in state
- [ ] Write unit tests for retry logic
- [ ] Write unit tests for backoff calculations

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
