# Workflow Syntax Reference

Complete reference for AWF workflow YAML syntax.

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
    on_success: step2
    on_failure: error

  step2:
    type: step
    command: echo "World"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

## Workflow Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Workflow identifier |
| `version` | string | No | Semantic version |
| `description` | string | No | Human-readable description |
| `inputs` | array | No | Input parameter definitions |
| `states` | object | Yes | State definitions |
| `states.initial` | string | Yes | Name of the starting state |

---

## State Types

| Type | Description |
|------|-------------|
| `step` | Execute a command |
| `terminal` | End state with success/failure status |
| `parallel` | Execute multiple steps concurrently |
| `for_each` | Iterate over a list of items |
| `while` | Repeat until condition is false |

---

## Step State

Execute a shell command.

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

### Step Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `command` | string | - | Shell command to execute |
| `dir` | string | cwd | Working directory (supports interpolation) |
| `timeout` | int | 0 | Execution timeout in seconds (0 = no timeout) |
| `on_success` | string | - | Next state on success (exit code 0) |
| `on_failure` | string | - | Next state on failure (exit code ≠ 0) |
| `continue_on_error` | bool | false | Always follow `on_success` regardless of exit code |
| `retry` | object | - | Retry configuration |
| `transitions` | array | - | Conditional transitions |

---

## Terminal State

End the workflow execution.

```yaml
done:
  type: terminal
  status: success

error:
  type: terminal
  status: failure
```

### Terminal Options

| Option | Type | Values | Description |
|--------|------|--------|-------------|
| `status` | string | `success`, `failure` | Terminal status |

---

## Parallel State

Execute multiple steps concurrently.

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
    - name: build
      command: go build ./cmd/...
  on_success: deploy
  on_failure: error
```

### Parallel Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `steps` | array | - | List of steps to execute concurrently |
| `strategy` | string | `all_succeed` | Execution strategy |
| `max_concurrent` | int | unlimited | Maximum concurrent steps |
| `on_success` | string | - | Next state on success |
| `on_failure` | string | - | Next state on failure |

### Parallel Strategies

| Strategy | Description |
|----------|-------------|
| `all_succeed` | All steps must succeed, cancel remaining on first failure |
| `any_succeed` | Succeed if at least one step succeeds |
| `best_effort` | Collect all results, never cancel early |

### Accessing Parallel Results

```yaml
# Access individual step outputs
command: echo "{{.states.parallel_build.steps.lint.output}}"
```

---

## For-Each Loop

Iterate over a list of items.

```yaml
process_files:
  type: for_each
  items: '["a.txt", "b.txt", "c.txt"]'
  max_iterations: 100
  break_when: "states.process_single.exit_code != 0"
  body:
    - process_single
  on_complete: aggregate

process_single:
  type: step
  command: |
    echo "Processing {{.loop.item}} ({{.loop.index1}}/{{.loop.length}})"
  on_success: process_files
```

### For-Each Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `items` | string | - | Template expression or literal JSON array |
| `body` | array | - | List of step names to execute each iteration |
| `max_iterations` | int/string | 100 | Safety limit (max: 10000). Supports interpolation. |
| `break_when` | string | - | Expression to exit loop early |
| `on_complete` | string | - | Next state after loop completes |

### Loop Context Variables

| Variable | Description |
|----------|-------------|
| `{{.loop.item}}` | Current item value |
| `{{.loop.index}}` | 0-based iteration index |
| `{{.loop.index1}}` | 1-based iteration index |
| `{{.loop.first}}` | True on first iteration |
| `{{.loop.last}}` | True on last iteration |
| `{{.loop.length}}` | Total items count |
| `{{.loop.parent}}` | Parent loop context (nested loops) |

### Dynamic Items

Items can come from a template expression:

```yaml
items: "{{.inputs.files}}"
```

### Dynamic Max Iterations

The `max_iterations` field supports interpolation and arithmetic expressions:

```yaml
# From input parameter
max_iterations: "{{.inputs.retry_count}}"

# From environment variable
max_iterations: "{{.env.MAX_RETRIES}}"

# Arithmetic expression
max_iterations: "{{.inputs.pages * .inputs.retries_per_page}}"
```

Supported arithmetic operators: `+`, `-`, `*`, `/`, `%`

Dynamic values are resolved at loop initialization time. Validation ensures:
- Result is a positive integer
- Value does not exceed 10000 (safety limit)

Use `awf validate` to detect undefined variables before runtime.

---

## While Loop

Repeat until condition becomes false.

```yaml
poll_status:
  type: while
  while: "states.check.output != 'ready'"
  max_iterations: 60
  body:
    - check
    - wait
  on_complete: proceed

check:
  type: step
  command: curl -s https://api.example.com/status
  on_success: poll_status

wait:
  type: step
  command: sleep 5
  on_success: poll_status
```

### While Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `while` | string | - | Condition expression (loop while true). Supports interpolation. |
| `body` | array | - | List of step names to execute each iteration |
| `max_iterations` | int/string | 100 | Safety limit (max: 10000). Supports interpolation. |
| `break_when` | string | - | Expression to exit loop early |
| `on_complete` | string | - | Next state after loop completes |

### While Loop Context

| Variable | Description |
|----------|-------------|
| `{{.loop.index}}` | 0-based iteration index |
| `{{.loop.first}}` | True on first iteration |
| `{{.loop.length}}` | Always -1 (unknown for while loops) |

---

## Nested Loops

Loops can contain other loops. Inner loops access outer loop context via `{{.loop.parent.*}}`:

```yaml
outer_loop:
  type: for_each
  items: '["A", "B"]'
  body:
    - inner_loop
  on_complete: done

inner_loop:
  type: for_each
  items: '["1", "2"]'
  body:
    - process
  on_complete: outer_loop

process:
  type: step
  command: 'echo "outer={{.loop.parent.item}} inner={{.loop.item}}"'
  on_success: inner_loop
```

Parent chains support arbitrary depth: `{{.loop.parent.parent.item}}` for 3-level nesting.

---

## Retry Configuration

Automatic retry for failed steps.

```yaml
flaky_api_call:
  type: step
  command: curl -f https://api.example.com/data
  retry:
    max_attempts: 5
    initial_delay: 1s
    max_delay: 30s
    backoff: exponential
    multiplier: 2
    jitter: 0.1
    retryable_exit_codes: [1, 22]
  on_success: process_data
  on_failure: error
```

### Retry Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_attempts` | int | 1 | Maximum attempts (1 = no retry) |
| `initial_delay` | duration | 0 | Delay before first retry |
| `max_delay` | duration | - | Maximum delay cap |
| `backoff` | string | `constant` | Strategy: `constant`, `linear`, `exponential` |
| `multiplier` | float | 2 | Multiplier for exponential backoff |
| `jitter` | float | 0 | Random jitter factor 0.0-1.0 |
| `retryable_exit_codes` | array | all | Exit codes to retry (empty = all non-zero) |

### Backoff Strategies

| Strategy | Formula |
|----------|---------|
| `constant` | Always `initial_delay` |
| `linear` | `initial_delay * attempt` |
| `exponential` | `initial_delay * multiplier^(attempt-1)` |

---

## Conditional Transitions

Dynamic branching based on expressions.

```yaml
process:
  type: step
  command: analyze.sh
  transitions:
    - when: "states.process.exit_code == 0 and inputs.mode == 'full'"
      goto: full_report
    - when: "states.process.exit_code == 0"
      goto: summary_report
    - goto: error  # default fallback (no when clause)
```

### Transition Options

| Option | Type | Description |
|--------|------|-------------|
| `when` | string | Expression to evaluate (optional for default) |
| `goto` | string | Target state if condition matches |

### Supported Operators

| Type | Operators |
|------|-----------|
| Comparison | `==`, `!=`, `<`, `>`, `<=`, `>=` |
| Logical | `and`, `or`, `not` |
| Grouping | `(expr)` |

### Available Variables

| Variable | Description |
|----------|-------------|
| `inputs.name` | Input values |
| `states.step_name.exit_code` | Step exit code |
| `states.step_name.output` | Step output |
| `env.VAR_NAME` | Environment variables |

Transitions are evaluated in order; first match wins. A transition without `when` acts as default fallback.

---

## Input Definitions

Define and validate workflow inputs.

```yaml
inputs:
  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".go", ".py", ".js"]

  - name: max_tokens
    type: integer
    default: 2000
    validation:
      min: 1
      max: 10000

  - name: env
    type: string
    default: staging
    validation:
      enum: [dev, staging, prod]

  - name: debug
    type: boolean
    default: false
```

### Input Options

| Option | Type | Description |
|--------|------|-------------|
| `name` | string | Input identifier |
| `type` | string | `string`, `integer`, `boolean` |
| `required` | bool | If true, must be provided |
| `default` | any | Default value if not provided |
| `validation` | object | Validation rules |

### Validation Rules

| Rule | Type | Description |
|------|------|-------------|
| `pattern` | string | Regex pattern to match |
| `enum` | array | List of allowed values |
| `min` | int | Minimum value (integers only) |
| `max` | int | Maximum value (integers only) |
| `file_exists` | bool | File must exist on filesystem |
| `file_extension` | array | Allowed file extensions |

### Validation Errors

Errors are collected and reported together:

```
input validation failed: 2 errors:
  - inputs.email: does not match pattern
  - inputs.count: value 150 exceeds maximum 100
```

---

## Variable Interpolation

AWF uses `{{.var}}` syntax (Go template style with dot prefix).

```yaml
# Inputs
command: echo "{{.inputs.variable_name}}"

# Previous step outputs
command: echo "{{.states.step_name.output}}"

# Workflow metadata
command: echo "Workflow ID: {{.workflow.id}}"

# Environment variables
command: echo "Home: {{.env.HOME}}"
```

See [Variable Interpolation Reference](../reference/interpolation.md) for complete details.

---

## Hooks

Execute commands before/after steps.

```yaml
my_step:
  type: step
  command: main-command
  pre_hook:
    command: echo "Before step"
  post_hook:
    command: echo "After step"
  on_success: next
```

### Hook Options

| Option | Type | Description |
|--------|------|-------------|
| `command` | string | Hook command to execute |
| `timeout` | int | Hook timeout in seconds |

---

## Working Directory

Steps can specify a working directory:

```yaml
build:
  type: step
  command: make build
  dir: "{{.inputs.project_path}}"
  on_success: test
```

The `dir` field supports variable interpolation.

---

## See Also

- [Commands](commands.md) - CLI command reference
- [Templates](templates.md) - Reusable workflow templates
- [Examples](examples.md) - Workflow examples
- [Variable Interpolation](../reference/interpolation.md) - Template variables
- [Input Validation](../reference/validation.md) - Validation rules
