---
title: "Retry Configuration Guide"
---

Automatically retry failed steps with configurable backoff strategies and exit code filtering.

## Overview

AWF provides built-in retry functionality for steps and agent calls. When a step fails, you can configure AWF to automatically retry with exponential, linear, or constant backoff delays.

**Common use cases:**
- Transient network errors (429, 502, 503 responses)
- Intermittent service failures
- Rate-limited API calls
- Flaky shell commands

## Basic Retry

The simplest retry configuration retries a step multiple times with default settings:

```yaml
states:
  initial: fetch_data

  fetch_data:
    type: step
    command: curl https://api.example.com/data
    retry:
      max_attempts: 3  # Try 3 times total (default: 1 = no retry)
    on_success: done

  done:
    type: terminal
    status: success
```

With this configuration:
1. `curl` executes
2. If it fails (non-zero exit), AWF retries up to 2 more times
3. Each retry executes immediately (no delay)
4. If all 3 attempts fail, the step is considered failed

## Adding Delays

Use `initial_delay` to add a delay before the first retry:

```yaml
fetch_data:
  type: step
  command: curl https://api.example.com/data
  retry:
    max_attempts: 3
    initial_delay: 1s      # Wait 1 second before first retry
    backoff: constant      # Always wait 1 second between attempts
  on_success: done
```

**Duration format** accepts Go duration strings:
- `100ms` — milliseconds
- `1s` — 1 second
- `30s` — 30 seconds
- `1m30s` — 1.5 minutes

## Backoff Strategies

### Constant Backoff

Retry with a fixed delay:

```yaml
retry:
  max_attempts: 5
  initial_delay: 2s
  backoff: constant
```

Delays: `2s`, `2s`, `2s`, `2s` (always the same)

### Linear Backoff

Delay increases linearly with each attempt:

```yaml
retry:
  max_attempts: 5
  initial_delay: 1s
  backoff: linear
```

Delays: `1s`, `2s`, `3s`, `4s` (multiplied by attempt number)

### Exponential Backoff

Delay increases exponentially (recommended for most use cases):

```yaml
retry:
  max_attempts: 5
  initial_delay: 1s
  backoff: exponential
  multiplier: 2        # Double the delay each time (default: 2.0)
```

Delays: `1s`, `2s`, `4s`, `8s` (multiplied by 2 each time)

**Using a different multiplier:**

```yaml
retry:
  max_attempts: 5
  initial_delay: 500ms
  backoff: exponential
  multiplier: 1.5      # Increase delay by 50% each time
```

Delays: `500ms`, `750ms`, `1.125s`, `1.687s`

## Capping Maximum Delay

Prevent delays from growing too large with `max_delay`:

```yaml
retry:
  max_attempts: 10
  initial_delay: 1s
  backoff: exponential
  multiplier: 2
  max_delay: 30s       # Never wait longer than 30 seconds
```

This configuration:
- Starts with 1 second delays
- Doubles each time: 2s, 4s, 8s, 16s, **30s** (capped), **30s**, **30s**, **30s**, **30s**

**Important:** Always specify `max_delay` to prevent excessively long delays in production.

## Filtering Retryable Exit Codes

By default, AWF retries on any non-zero exit code. Use `retryable_exit_codes` to retry only specific failures:

```yaml
deploy:
  type: step
  command: ./deploy.sh
  retry:
    max_attempts: 3
    initial_delay: 5s
    backoff: exponential
    retryable_exit_codes: [1, 22]  # Only retry on exit codes 1 and 22
  on_success: verify
```

With this configuration:
- Exit code `1` (transient error) → retry
- Exit code `22` (connection error) → retry
- Exit code `5` (invalid config) → fail immediately, don't retry

**Empty array** (the default) retries all non-zero codes:

```yaml
retry:
  max_attempts: 3
  retryable_exit_codes: []        # Retry on any non-zero exit
```

## Agent Step Retry

Retry agent steps the same way you retry command steps:

```yaml
analyze:
  type: agent
  provider: claude
  prompt: "Analyze: {{.inputs.code}}"
  timeout: 120
  retry:
    max_attempts: 3
    initial_delay: 2s
    backoff: exponential
  on_success: done
```

## HTTP Operation Retry

For HTTP operations (REST API calls), AWF retries based on status codes:

```yaml
api_call:
  type: operation
  operation: http.request
  inputs:
    method: POST
    url: https://api.example.com/process
    body: "{{.inputs.data}}"
    retryable_status_codes: [429, 502, 503]  # Retry on rate limit or server error
  retry:
    max_attempts: 5
    initial_delay: 1s
    backoff: exponential
    multiplier: 2
    max_delay: 60s
  on_success: next
```

## Complete Example: Reliable API Integration

This example shows a robust API integration with retry, error handling, and logging:

```yaml
name: reliable-api
version: "1.0.0"

inputs:
  - name: endpoint
    type: string
    required: true
    default: "https://api.example.com"

states:
  initial: fetch_with_retry

  fetch_with_retry:
    type: operation
    operation: http.request
    inputs:
      method: GET
      url: "{{.inputs.endpoint}}/data"
      timeout: 30
      retryable_status_codes: [429, 502, 503, 504]
    retry:
      max_attempts: 5
      initial_delay: 1s
      backoff: exponential
      multiplier: 2
      max_delay: 32s
    on_success: process
    on_failure:
      message: "API call failed after 5 attempts: {{.error.message}}"
      status: 3

  process:
    type: agent
    provider: claude
    prompt: "Process this JSON: {{.states.fetch_with_retry.output}}"
    retry:
      max_attempts: 2
      initial_delay: 2s
      backoff: constant
    on_success: done

  done:
    type: terminal
    status: success
```

## Validation Rules

AWF validates retry configurations to catch mistakes early:

| Rule | Error |
|------|-------|
| `max_attempts < 1` | `max_attempts must be at least 1` |
| `initial_delay` invalid | `invalid initial_delay: expected duration string` |
| `max_delay` invalid | `invalid max_delay: expected duration string` |
| Unknown `backoff` | `invalid backoff strategy: use constant, linear, or exponential` |
| `jitter` outside [0, 1] | `jitter must be between 0.0 and 1.0` |
| `multiplier < 0` | `multiplier must be non-negative` |

Example error:

```
$ awf run my-workflow
ERROR validating workflow: step 'fetch': invalid max_attempts: 0
```

## Common Patterns

### Circuit Breaker (Give Up After Repeated Failures)

Use step transitions to skip retries after a threshold:

```yaml
deploy:
  type: step
  command: ./deploy.sh
  retry:
    max_attempts: 3
    initial_delay: 5s
    backoff: exponential
  on_success: verify
  on_failure: alert_ops

alert_ops:
  type: terminal
  message: "Deployment failed after 3 attempts. Manual intervention required."
  status: 2
```

### Jitter (Randomize Delays to Avoid Thundering Herd)

For distributed systems where many clients retry simultaneously, add randomization:

```yaml
retry:
  max_attempts: 5
  initial_delay: 1s
  backoff: exponential
  multiplier: 2
  jitter: 0.5              # Add ±50% randomness to each delay
```

This prevents multiple clients from retrying at exactly the same time, which can overwhelm the service.

### Escalating Delays

For critical operations, increase delays over multiple retries:

```yaml
critical_task:
  type: step
  command: ./critical-operation.sh
  retry:
    max_attempts: 10
    initial_delay: 500ms
    backoff: exponential
    multiplier: 1.5
    max_delay: 5m           # Cap at 5 minutes
  on_success: done
```

With `multiplier: 1.5`:
1. 500ms
2. 750ms
3. 1.125s
4. 1.687s
5. 2.531s
... eventually capped at 5m

## Troubleshooting

### Retries Not Happening

**Problem:** Your step never retries even though it fails.

**Causes:**
1. `max_attempts` not specified (defaults to 1 = no retry)
2. Exit code not in `retryable_exit_codes` list

**Solution:**
```yaml
# Add explicit retry configuration
retry:
  max_attempts: 3
  initial_delay: 1s
```

### Delays Too Long

**Problem:** Retries take forever.

**Causes:**
1. `max_delay` not specified on exponential backoff
2. `max_attempts` set too high

**Solution:**
```yaml
retry:
  max_attempts: 5          # Reasonable limit
  initial_delay: 1s
  backoff: exponential
  max_delay: 30s           # Always cap exponential backoff
```

### Some Failures Not Retrying

**Problem:** Step fails on certain errors but doesn't retry.

**Causes:**
1. Exit code not in `retryable_exit_codes` list
2. `retryable_exit_codes` too restrictive

**Solution:**
```yaml
# Check which exit code your command produces
$ ./my-script.sh; echo "Exit code: $?"

# Then add it to retryable_exit_codes
retry:
  retryable_exit_codes: [1, 22, 35]
```

## Reference

### Retry Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_attempts` | int | 1 | Maximum number of attempts (1 = no retry) |
| `initial_delay` | duration | 0 | Delay before first retry |
| `max_delay` | duration | unlimited | Maximum delay between retries |
| `backoff` | string | `constant` | Strategy: `constant`, `linear`, `exponential` |
| `multiplier` | float | 2.0 | Multiplier for exponential backoff |
| `jitter` | float | 0.0 | Randomness factor (0.0-1.0) |
| `retryable_exit_codes` | array | all | Exit codes to retry (empty = all non-zero) |

### Backoff Formulas

| Strategy | Formula |
|----------|---------|
| Constant | `initial_delay` |
| Linear | `initial_delay × attempt_number` |
| Exponential | `initial_delay × multiplier^(attempt_number-1)` |

## See Also

- [Workflow Syntax Reference](workflow-syntax.md#retry) — Complete YAML syntax
- [Agent Steps Guide](agent-steps.md) — Retry for AI operations
- [HTTP Operations](plugins.md#http-request) — Retry for REST APIs
