# Exit Codes

AWF uses standardized exit codes to indicate the type of error that occurred.

## Exit Code Reference

| Code | Type | Description | Examples |
|------|------|-------------|----------|
| 0 | Success | Workflow completed successfully | Normal completion |
| 1 | User Error | Bad input or missing file | Invalid flag, missing workflow file |
| 2 | Workflow Error | Invalid workflow definition | Invalid state reference, cycle detected |
| 3 | Execution Error | Command execution failed | Command returned non-zero, timeout |
| 4 | System Error | System-level failure | IO error, permission denied |

## Error Categories

### Exit Code 0: Success

The workflow executed and reached a terminal state with `status: success`.

```bash
awf run my-workflow
echo $?  # 0
```

### Exit Code 1: User Error

The user provided invalid input or the requested resource doesn't exist.

**Common causes:**
- Invalid command-line flag
- Missing required input parameter
- Workflow file not found
- Invalid input value (validation failed)

**Examples:**
```bash
# Missing required input
awf run deploy
# Error: missing required input: env

# Workflow not found
awf run nonexistent
# Error: workflow 'nonexistent' not found

# Invalid input value
awf run deploy --input env=invalid
# Error: input validation failed: enum values are [dev, staging, prod]
```

### Exit Code 2: Workflow Error

The workflow definition contains errors detected at load or validation time.

**Common causes:**
- Invalid state reference (state doesn't exist)
- Cycle detected in state transitions
- Unreachable states
- Missing terminal state
- Invalid template reference
- Forward reference (step A references step B before B executes)

**Examples:**
```bash
# Invalid state reference
awf validate my-workflow
# Error: state 'missing_state' referenced but not defined

# Cycle detected
awf validate my-workflow
# Error: cycle detected: step1 -> step2 -> step1

# Missing terminal
awf validate my-workflow
# Error: no terminal state found
```

### Exit Code 3: Execution Error

A command failed during execution.

**Common causes:**
- Command returned non-zero exit code
- Command timed out
- Step failed after all retry attempts
- Parallel step failed (with `all_succeed` strategy)

**Examples:**
```bash
# Command failed
awf run deploy
# Error: step 'build' failed with exit code 1

# Timeout
awf run long-task
# Error: step 'process' timed out after 30s

# Retry exhausted
awf run flaky-api
# Error: step 'api_call' failed after 5 attempts
```

### Exit Code 4: System Error

A system-level error prevented execution.

**Common causes:**
- Permission denied (file or directory)
- Disk full
- Network error
- Database error (history storage)
- Unable to create temp files

**Examples:**
```bash
# Permission denied
awf run my-workflow
# Error: permission denied: /etc/config

# Storage error
awf history
# Error: failed to open history database
```

## Using Exit Codes in Scripts

```bash
#!/bin/bash

awf run deploy --input env=prod
exit_code=$?

case $exit_code in
  0)
    echo "Deployment successful"
    ;;
  1)
    echo "Invalid input - check your parameters"
    ;;
  2)
    echo "Workflow error - validate your workflow"
    ;;
  3)
    echo "Execution failed - check command output"
    ;;
  4)
    echo "System error - check permissions and disk space"
    ;;
  *)
    echo "Unknown error: $exit_code"
    ;;
esac
```

## JSON Error Output

Use `--format json` for structured error information:

```bash
awf run deploy -f json
```

```json
{
  "success": false,
  "error": {
    "code": 3,
    "type": "execution_error",
    "message": "step 'build' failed with exit code 1",
    "step": "build",
    "details": {
      "exit_code": 1,
      "output": "make: *** No rule to make target 'build'."
    }
  }
}
```

## See Also

- [Commands](../user-guide/commands.md) - CLI command reference
- [Workflow Syntax](../user-guide/workflow-syntax.md) - Workflow definition
