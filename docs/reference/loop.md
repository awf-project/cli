---
title: "Loop Reference"
---

This document describes loop control flow in AWF workflows, including while loops, for-each loops, and transition behavior within loop bodies.

## Overview

AWF supports two types of loops for iterative execution:

- **While loops** (`type: while`) - Repeat until a condition becomes false
- **For-each loops** (`type: for_each`) - Iterate over a list of items

Loops can contain transitions within body steps, enabling advanced control flow patterns such as:
- **Intra-body transitions** - Skip steps within the current iteration
- **Early exit** - Break out of the loop before completion
- **Error handling** - Retry patterns with `on_failure` transitions

Loop bodies execute sequentially by default. When transitions are defined in body steps, the loop executor evaluates them after each step and jumps to the target step or exits the loop.

## While Loops

While loops repeat execution until a `break_when` condition evaluates to true or `max_iterations` is reached.

### Syntax

```yaml
my_loop:
  type: while
  while: 'true'  # Optional condition (default: true)
  break_when: 'states.check.Output contains "DONE"'
  max_iterations: 10
  body:
    - step1
    - step2
    - step3
  on_complete: next_state
  on_failure: error_handler
```

### Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `while` | string | No | Condition expression (default: `true`) |
| `break_when` | string | Yes | Exit condition (evaluates each iteration) |
| `max_iterations` | int | No | Maximum iterations (default: unlimited) |
| `body` | array | Yes | List of step names to execute in order |
| `on_complete` | string | No | Next state after loop completes normally |
| `on_failure` | string | No | Next state if loop step fails |

### Execution Flow

1. Evaluate `while` condition - if false, skip loop entirely
2. For each iteration:
   - Execute body steps in order
   - Evaluate transitions after each step (see [Transitions Within Loop Bodies](#transitions-within-loop-bodies))
   - Check `break_when` condition after body completes
   - Exit if condition is true or `max_iterations` reached
3. Transition to `on_complete` state

### Example: Retry Until Success

```yaml
retry_deploy:
  type: while
  while: 'true'
  break_when: 'states.check_deployment.Output contains "SUCCESS"'
  max_iterations: 5
  body:
    - deploy_app
    - check_deployment
  on_complete: notify_success
  on_failure: notify_failure

deploy_app:
  type: step
  command: ./deploy.sh
  on_success: retry_deploy

check_deployment:
  type: step
  command: curl -f https://app.example.com/health
  on_success: retry_deploy
```

## For-Each Loops

For-each loops iterate over a list of items, executing the body once per item.

### Syntax

```yaml
process_files:
  type: for_each
  items:
    - file1.txt
    - file2.txt
    - file3.txt
  body:
    - validate_file
    - process_file
  on_complete: summarize
  on_failure: cleanup
```

### Options

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `items` | array | Yes | List of items to iterate over |
| `body` | array | Yes | List of step names to execute per item |
| `on_complete` | string | No | Next state after all items processed |
| `on_failure` | string | No | Next state if any body step fails |

### Item Access

Within loop body steps, access the current item using `{{.loop.item}}`:

```yaml
process_file:
  type: step
  command: |
    echo "Processing {{.loop.item}}"
    cat "{{.loop.item}}" | process.sh
  on_success: process_files
```

### Example: Process Multiple Environments

```yaml
deploy_all:
  type: for_each
  items: ["dev", "staging", "prod"]
  body:
    - validate_env
    - deploy_to_env
    - verify_env
  on_complete: done

validate_env:
  type: step
  command: |
    echo "Validating {{.loop.item}} environment"
    ./validate.sh --env={{.loop.item}}
  on_success: deploy_all

deploy_to_env:
  type: step
  command: |
    echo "Deploying to {{.loop.item}}"
    ./deploy.sh --env={{.loop.item}}
  on_success: deploy_all

verify_env:
  type: step
  command: |
    curl -f https://{{.loop.item}}.example.com/health
  on_success: deploy_all
```

## Transitions Within Loop Bodies

Loop body steps can define transitions to control execution flow within iterations. The loop executor evaluates transitions after each step and performs one of the following actions:

1. **Intra-body jump** - Target is another step in the loop body → skip to that step
2. **Early exit** - Target is outside the loop body → break loop and goto target
3. **Sequential execution** - No transition matches → continue to next body step
4. **Invalid target** - Target doesn't exist → log warning and continue sequentially

### Target Resolution

When a transition matches, the executor resolves the target as follows:

1. Check if target step exists in `body` array → jump forward or backward within iteration
2. Check if target step exists in workflow states → exit loop and transition to target
3. If target not found → log warning and continue to next body step (graceful degradation)

### Intra-Body Transitions (Skip Steps)

Transitions can jump to later steps in the body, skipping intermediate steps.

```yaml
test_loop:
  type: while
  while: 'true'
  break_when: 'states.run_tests.Output contains "ALL_PASSED"'
  max_iterations: 3
  body:
    - run_tests
    - check_results
    - fix_code      # Skipped when tests pass
    - retry_build   # Skipped when tests pass
    - run_tests
  on_complete: deploy

# When tests pass, skip fix_code and retry_build
check_results:
  type: step
  command: |
    if grep -q "TESTS_PASSED" test-output.txt; then
      echo "TESTS_PASSED"
    else
      echo "TESTS_FAILED"
    fi
  transitions:
    - when: 'states.check_results.Output contains "TESTS_PASSED"'
      goto: run_tests  # Skip to final verification
    - goto: fix_code   # Continue to fix code

fix_code:
  type: step
  command: ./auto-fix.sh
  on_success: test_loop

retry_build:
  type: step
  command: make build
  on_success: test_loop

run_tests:
  type: step
  command: make test > test-output.txt
  on_success: test_loop
```

In this example, when `check_results` detects passing tests, it transitions directly to `run_tests`, skipping both `fix_code` and `retry_build`.

### Early Exit from Loops

Transitions targeting steps outside the loop body cause an immediate loop exit.

```yaml
green_loop:
  type: while
  while: 'true'
  break_when: 'states.verify.Output contains "COMPLETE"'
  max_iterations: 10
  body:
    - implement
    - test
    - verify
  on_complete: done

test:
  type: step
  command: ./run-tests.sh
  transitions:
    - when: 'states.test.ExitCode == 0'
      goto: cleanup  # Exit loop early - target outside body
    - goto: implement

cleanup:
  type: step
  command: ./cleanup.sh
  on_success: done
```

When the test succeeds, the transition to `cleanup` (outside the loop body) causes the loop to exit immediately, bypassing the `verify` step and `break_when` condition.

### Sequential Execution Fallback

If no transition matches or the target is invalid, execution continues to the next body step sequentially. This provides graceful degradation and backward compatibility with existing workflows.

```yaml
# Invalid target example - logs warning but continues
buggy_step:
  type: step
  command: echo "Processing"
  transitions:
    - when: 'states.buggy_step.Output contains "ERROR"'
      goto: nonexistent_step  # Warning logged, continues to next step
```

When an invalid transition target is encountered, AWF logs a warning and continues sequential execution. This prevents workflow failures due to misconfiguration.

## Loop Context Variables

The following variables are available within loop bodies:

| Variable | Type | Availability | Description |
|----------|------|--------------|-------------|
| `{{.loop.index}}` | integer | All loops | Current iteration index (0-based) |
| `{{.loop.item}}` | any | `for_each` only | Current item value |
| `{{.loop.parent.*}}` | any | Nested loops | Parent loop context (see [Nested Loops](#nested-loops)) |

### Example: Using Loop Index

```yaml
retry_with_backoff:
  type: while
  while: 'true'
  break_when: 'states.check.Output contains "SUCCESS"'
  max_iterations: 5
  body:
    - wait_backoff
    - attempt_operation
    - check

wait_backoff:
  type: step
  command: |
    # Exponential backoff: 2^index seconds
    sleep $((2 ** {{.loop.index}}))
  on_success: retry_with_backoff
```

## Nested Loops

Loops can be nested within each other. Each loop maintains its own context, and inner loops can access parent loop variables.

### Nested Loop Context Isolation

Inner loop transitions only affect the inner loop. They cannot jump to steps in the parent loop body.

```yaml
outer:
  type: for_each
  items: ["module1", "module2"]
  body:
    - setup_module
    - inner_loop
    - teardown_module
  on_complete: done

inner_loop:
  type: while
  while: 'true'
  break_when: 'states.test.Output contains "PASSED"'
  max_iterations: 3
  body:
    - build
    - test
  on_complete: outer

test:
  type: step
  command: |
    echo "Testing {{.loop.parent.item}}"
    ./test.sh --module={{.loop.parent.item}}
  transitions:
    - when: 'states.test.ExitCode == 0'
      goto: outer  # Early exit from inner loop
```

In this example:
- Inner loop uses `{{.loop.parent.item}}` to access the outer loop's current item
- Transition to `outer` exits the inner `while` loop but doesn't skip steps in the outer `for_each` body
- After inner loop completes, execution continues to `teardown_module` in the outer loop

### Parent Context Access

Access parent loop variables using the `{{.loop.parent.*}}` prefix:

| Variable | Description |
|----------|-------------|
| `{{.loop.parent.index}}` | Parent loop iteration index |
| `{{.loop.parent.item}}` | Parent loop current item (for_each only) |

## Error Handling in Loops

### On-Failure Transitions

Body steps can use `on_failure` to handle errors within the loop.

```yaml
resilient_loop:
  type: while
  while: 'true'
  break_when: 'states.process.Output contains "DONE"'
  max_iterations: 10
  body:
    - fetch_data
    - process
  on_complete: done
  on_failure: cleanup

fetch_data:
  type: step
  command: curl -f https://api.example.com/data
  on_success: resilient_loop
  on_failure: resilient_loop  # Retry on same step (retry pattern)
```

### Retry Pattern Preservation

Transitioning to the same step name (e.g., `on_failure: resilient_loop`) creates a retry pattern. The loop executor preserves this behavior for backward compatibility with existing workflows.

```yaml
# Retry pattern: on_failure transitions back to the loop itself
flaky_operation:
  type: step
  command: ./flaky-script.sh
  retry:
    max_attempts: 3
    backoff: exponential
  on_success: my_loop
  on_failure: my_loop  # Retry entire iteration
```

### Empty Body Edge Case

Loops with empty bodies or no steps are valid but do nothing. The `break_when` condition is still evaluated each iteration.

```yaml
# Edge case: empty body (valid but not useful)
wait_loop:
  type: while
  while: 'true'
  break_when: 'states.external_check.Output contains "READY"'
  max_iterations: 100
  body: []
  on_complete: proceed
```

This loop waits for an external condition without executing any steps. Consider using polling or event-driven patterns instead.

## Backward Compatibility

Existing workflows without transitions in loop bodies continue to work unchanged. Sequential execution is the default behavior when no transitions are defined.

```yaml
# Backward compatible: no transitions, sequential execution
simple_loop:
  type: while
  while: 'true'
  break_when: 'states.step3.Output contains "DONE"'
  body:
    - step1
    - step2
    - step3
  on_complete: done

step1:
  type: step
  command: echo "Step 1"
  on_success: simple_loop

step2:
  type: step
  command: echo "Step 2"
  on_success: simple_loop

step3:
  type: step
  command: echo "Step 3"
  on_success: simple_loop
```

This workflow executes all three steps sequentially in each iteration, maintaining the behavior from previous AWF versions.

## Examples

### Example 1: TDD Loop with Skip Steps

This example demonstrates the TDD pattern from the F048 specification, where successful tests skip implementation steps.

```yaml
name: tdd-workflow
version: "1.0.0"

states:
  initial: green_loop

  green_loop:
    type: while
    while: 'true'
    break_when: 'states.check_tests_passed.Output contains "TESTS_PASSED"'
    max_iterations: 10
    body:
      - run_tests_green
      - check_tests_passed
      - prepare_impl_prompt
      - implement_item
      - run_fmt
    on_complete: done

  run_tests_green:
    type: step
    command: |
      make test > test-output.txt
      echo "TEST_EXIT_CODE=$?" >> test-output.txt
    on_success: green_loop

  check_tests_passed:
    type: step
    command: |
      if grep -q "TEST_EXIT_CODE=0" test-output.txt; then
        echo "TESTS_PASSED"
      else
        echo "TESTS_FAILED"
      fi
    transitions:
      - when: 'states.check_tests_passed.Output contains "TESTS_PASSED"'
        goto: run_fmt  # Skip prepare_impl_prompt and implement_item
      - goto: prepare_impl_prompt

  prepare_impl_prompt:
    type: step
    command: ./prepare-prompt.sh
    on_success: green_loop

  implement_item:
    type: step
    command: ./implement.sh
    on_success: green_loop

  run_fmt:
    type: step
    command: make fmt
    on_success: green_loop

  done:
    type: terminal
    status: success
```

When tests pass, `check_tests_passed` transitions directly to `run_fmt`, skipping the implementation steps. This prevents unnecessary AI agent execution.

### Example 2: Early Exit on Critical Error

This example shows how to exit a validation loop early when a critical error is detected.

```yaml
name: validate-services
version: "1.0.0"

states:
  initial: validate_loop

  validate_loop:
    type: for_each
    items: ["auth", "api", "database", "cache"]
    body:
      - check_service
      - validate_config
      - test_connectivity
    on_complete: all_healthy
    on_failure: cleanup

  check_service:
    type: step
    command: |
      systemctl is-active "{{.loop.item}}"
    transitions:
      - when: 'states.check_service.ExitCode != 0'
        goto: critical_error  # Exit loop immediately
    on_success: validate_loop

  validate_config:
    type: step
    command: |
      validate-config --service={{.loop.item}}
    on_success: validate_loop

  test_connectivity:
    type: step
    command: |
      curl -f "http://localhost:{{.loop.item}}-port/health"
    on_success: validate_loop

  critical_error:
    type: terminal
    status: failure
    message: "Critical service validation failed"

  all_healthy:
    type: terminal
    status: success
    message: "All services validated"
```

When `check_service` detects a stopped service, it transitions to `critical_error`, exiting the for-each loop immediately without validating remaining services.

### Example 3: Nested Loops with Parent Context

This example demonstrates nested loops where the inner loop uses parent context variables.

```yaml
name: test-matrix
version: "1.0.0"

states:
  initial: env_loop

  env_loop:
    type: for_each
    items: ["dev", "staging", "prod"]
    body:
      - setup_env
      - browser_loop
      - teardown_env
    on_complete: done

  setup_env:
    type: step
    command: |
      echo "Setting up {{.loop.item}} environment"
      ./setup.sh --env={{.loop.item}}
    on_success: env_loop

  browser_loop:
    type: for_each
    items: ["chrome", "firefox", "safari"]
    body:
      - run_browser_tests
    on_complete: env_loop

  run_browser_tests:
    type: step
    command: |
      echo "Testing {{.loop.parent.item}} with {{.loop.item}}"
      ./test.sh --env={{.loop.parent.item}} --browser={{.loop.item}}
    transitions:
      - when: 'states.run_browser_tests.ExitCode != 0'
        goto: test_failed
    on_success: browser_loop

  teardown_env:
    type: step
    command: |
      ./teardown.sh --env={{.loop.item}}
    on_success: env_loop

  test_failed:
    type: terminal
    status: failure
    message: "Browser test failed"

  done:
    type: terminal
    status: success
    message: "All environment and browser tests passed"
```

The inner `browser_loop` accesses the outer loop's environment using `{{.loop.parent.item}}`, creating a test matrix that validates each browser against each environment.

## Known Limitations

### Nested Loop Max Iteration Handling

When a loop reaches `max_iterations` and its body contains nested loops (`for_each`, `while`, `parallel`, or `call_workflow` steps), AWF generates a specific error: **"loop reached maximum iterations with nested complexity"**.

**Current Behavior**: This error occurs even if the nested loop execution is otherwise successful. The outer loop fails when it hits `max_iterations`, regardless of whether the nested steps completed normally.

**Impact**: You cannot rely on `max_iterations` as a safety mechanism when using nested loops in the body. The workflow will fail with a complexity error instead of completing normally.

**Example**:
```yaml
outer_while:
  type: while
  while: 'true'
  break_when: 'false'
  max_iterations: 2        # Will fail with "nested complexity" error
  body:
    - inner_foreach        # Nested loop triggers complexity detection
  on_complete: done

inner_foreach:
  type: for_each
  items: ["x", "y"]
  body:
    - process
```

**Recommendation**:
- Always use `break_when` conditions to exit nested loops naturally
- Set `max_iterations` high enough that it's never reached under normal conditions
- For complex iteration patterns, consider:
  - Breaking the workflow into multiple workflows using `call_workflow` steps
  - Using conditional transitions to flatten nested logic
  - Restructuring data to reduce nesting requirements

**Test Reference**: See `TestExecuteLoopStep_WhileContainingForEach` in `internal/application/execution_service_test.go` for the documented behavior.

**Future Enhancement**: This limitation may be addressed in future AWF versions with improved nested loop iteration tracking.

## See Also

- [Workflow Syntax](../user-guide/workflow-syntax.md) - General workflow structure and state types
- [Variable Interpolation](./interpolation.md) - Using `{{.loop.*}}` and other variables
- [Validation](./validation.md) - Loop validation rules and error messages
