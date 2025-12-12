# Workflow Syntax Reference

## Basic Structure

```yaml
name: my-workflow
version: "1.0.0"
description: Workflow description

inputs:
  - name: file_path
    type: string
    required: true

states:
  initial: step1

  step1:
    type: step
    command: echo "Hello"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

## State Types

| Type | Description |
|------|-------------|
| `step` | Execute a command |
| `terminal` | End with success/failure |
| `parallel` | Run steps concurrently |
| `for_each` | Iterate over list |
| `while` | Repeat until false |

## Step State

```yaml
my_step:
  type: step
  command: |
    echo "Processing {{.inputs.file}}"
  dir: /tmp/workdir
  timeout: 30
  on_success: next_step
  on_failure: error
  continue_on_error: false
  retry:
    max_attempts: 3
    backoff: exponential
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `command` | string | - | Shell command |
| `dir` | string | cwd | Working directory |
| `timeout` | int | 0 | Timeout in seconds |
| `on_success` | string | - | Next state on success |
| `on_failure` | string | - | Next state on failure |
| `continue_on_error` | bool | false | Always follow on_success |

## Parallel State

```yaml
parallel_build:
  type: parallel
  strategy: all_succeed
  max_concurrent: 3
  steps:
    - name: lint
      command: golangci-lint run
    - name: test
      command: go test ./...
  on_success: deploy
  on_failure: error
```

**Strategies:**
- `all_succeed` - All must succeed, cancel on first failure
- `any_succeed` - Succeed if at least one succeeds
- `best_effort` - Collect all results, never cancel

## For-Each Loop

```yaml
process_files:
  type: for_each
  items: '["a.txt", "b.txt", "c.txt"]'
  max_iterations: 100
  break_when: "states.process.exit_code != 0"
  body:
    - process
  on_complete: aggregate
```

**Loop Context Variables:**

| Variable | Description |
|----------|-------------|
| `{{.loop.item}}` | Current item |
| `{{.loop.index}}` | 0-based index |
| `{{.loop.index1}}` | 1-based index |
| `{{.loop.first}}` | True on first |
| `{{.loop.last}}` | True on last |
| `{{.loop.length}}` | Total count |
| `{{.loop.parent}}` | Parent loop (nested) |

## While Loop

```yaml
poll_status:
  type: while
  while: "states.check.output != 'ready'"
  max_iterations: 60
  body:
    - check
    - wait
  on_complete: proceed
```

## Retry Configuration

```yaml
retry:
  max_attempts: 5
  initial_delay: 1s
  max_delay: 30s
  backoff: exponential
  multiplier: 2
  jitter: 0.1
  retryable_exit_codes: [1, 22]
```

**Backoff Strategies:**
- `constant` - Always initial_delay
- `linear` - initial_delay * attempt
- `exponential` - initial_delay * multiplier^(attempt-1)

## Conditional Transitions

```yaml
process:
  type: step
  command: analyze.sh
  transitions:
    - when: "states.process.exit_code == 0 and inputs.mode == 'full'"
      goto: full_report
    - when: "states.process.exit_code == 0"
      goto: summary_report
    - goto: error  # default
```

**Operators:** `==`, `!=`, `<`, `>`, `<=`, `>=`, `and`, `or`, `not`

## Input Definitions

```yaml
inputs:
  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".go", ".py"]

  - name: count
    type: integer
    default: 10
    validation:
      min: 1
      max: 100

  - name: env
    type: string
    validation:
      enum: [dev, staging, prod]

  - name: debug
    type: boolean
    default: false
```

**Validation Rules:**
- `pattern` - Regex match
- `enum` - Allowed values
- `min`/`max` - Integer bounds
- `file_exists` - Must exist
- `file_extension` - Allowed extensions

## Variable Interpolation

```yaml
# Inputs
command: echo "{{.inputs.variable_name}}"

# Previous outputs
command: echo "{{.states.step_name.output}}"

# Workflow metadata
command: echo "ID: {{.workflow.id}}"

# Environment
command: echo "{{.env.HOME}}"

# Loop context
command: echo "{{.loop.item}} ({{.loop.index1}}/{{.loop.length}})"
```

## Hooks

```yaml
my_step:
  type: step
  command: main-command
  pre_hook:
    command: echo "Before"
  post_hook:
    command: echo "After"
```
