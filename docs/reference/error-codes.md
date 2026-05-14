---
title: "Error Codes Reference"
---

AWF uses a hierarchical error code taxonomy to provide granular, machine-readable error identification. Each error code follows the format `CATEGORY.SUBCATEGORY.SPECIFIC` and maps to one of the four standard exit codes.

## Quick Reference

Use the `awf error` command to look up error codes:

```bash
# List all error codes
awf error

# Look up specific error code
awf error USER.INPUT.MISSING_FILE

# Look up by category prefix
awf error WORKFLOW.VALIDATION

# Get JSON output
awf error EXECUTION.COMMAND.FAILED --format json
```

## Error Code Format

Error codes follow a three-level hierarchy:

```
CATEGORY.SUBCATEGORY.SPECIFIC
```

- **CATEGORY**: Top-level classification (USER, WORKFLOW, EXECUTION, SYSTEM)
- **SUBCATEGORY**: Mid-level grouping by error type (INPUT, VALIDATION, COMMAND, IO)
- **SPECIFIC**: Precise error identifier (MISSING_FILE, CYCLE_DETECTED, etc.)

Each category maps to a specific exit code:
- `USER.*` → exit code 1
- `WORKFLOW.*` → exit code 2
- `EXECUTION.*` → exit code 3
- `SYSTEM.*` → exit code 4

## USER Category (Exit Code 1)

User-facing input and configuration errors.

### USER.INPUT.MISSING_FILE

**Description:** The specified file was not found at the given path.

**Resolution:** Verify the file path is correct and the file exists. Check for typos in the filename or path.

**Example:**
```bash
awf run deploy --workflow-file missing.yaml
# Error [USER.INPUT.MISSING_FILE]: workflow file 'missing.yaml' not found
```

**Related codes:** `USER.INPUT.INVALID_FORMAT`, `USER.INPUT.MISSING_SKILL`, `SYSTEM.IO.READ_FAILED`

### USER.INPUT.MISSING_SKILL

**Description:** A skill referenced in a workflow step could not be found. Either the skill directory does not exist in any discovery path, or the directory exists but contains no SKILL.md file.

**Resolution:** Verify the skill name matches a directory in one of the discovery paths (`.awf/skills/`, `.agents/skills/`, `.claude/skills/`, or `$XDG_CONFIG_HOME/awf/skills/`). For path-based references, verify the path is correct relative to the workflow file.

**Example:**
```bash
awf run code-review
# Error [USER.INPUT.MISSING_SKILL]: skill 'nonexistent-skill' not found in search paths:
#   .awf/skills/, .agents/skills/, .claude/skills/, ~/.config/awf/skills/
```

**Related codes:** `USER.INPUT.MISSING_FILE`, `WORKFLOW.VALIDATION.INVALID_REFERENCE`

---

### USER.INPUT.INVALID_FORMAT

**Description:** The file format does not match expected structure or contains invalid syntax.

**Resolution:** Check the file format against the documentation. Ensure YAML syntax is valid if applicable.

**Example:**
```bash
awf run deploy --workflow-file malformed.yaml
# Error [USER.INPUT.INVALID_FORMAT]: invalid YAML syntax at line 12
```

**Related codes:** `WORKFLOW.PARSE.YAML_SYNTAX`, `USER.INPUT.VALIDATION_FAILED`

---

### USER.INPUT.VALIDATION_FAILED

**Description:** Input parameter validation failed due to invalid or missing required values.

**Resolution:** Review the command-line arguments and flags. Use `--help` for usage information.

**Example:**
```bash
awf run deploy --input env=invalid
# Error [USER.INPUT.VALIDATION_FAILED]: enum values are [dev, staging, prod]
```

**Related codes:** `USER.INPUT.MISSING_FILE`, `USER.INPUT.INVALID_FORMAT`

---

## WORKFLOW Category (Exit Code 2)

Workflow definition parsing and validation errors.

### WORKFLOW.PARSE.YAML_SYNTAX

**Description:** YAML parsing error due to syntax violation or malformed structure.

**Resolution:** Validate YAML syntax using a YAML linter. Check for indentation errors, missing colons, or invalid characters.

**Example:**
```bash
awf validate my-workflow
# Error [WORKFLOW.PARSE.YAML_SYNTAX]: yaml: line 15: mapping values are not allowed in this context
```

**Related codes:** `WORKFLOW.PARSE.UNKNOWN_FIELD`, `USER.INPUT.INVALID_FORMAT`

---

### WORKFLOW.PARSE.UNKNOWN_FIELD

**Description:** The workflow definition contains an unrecognized field name.

**Resolution:** Check the workflow schema documentation. Remove or rename the unrecognized field.

**Example:**
```bash
awf validate my-workflow
# Error [WORKFLOW.PARSE.UNKNOWN_FIELD]: unknown field 'executes' (did you mean 'execute'?)
```

**Related codes:** `WORKFLOW.PARSE.YAML_SYNTAX`

---

### WORKFLOW.VALIDATION.CYCLE_DETECTED

**Description:** A cycle was detected in the workflow state machine transitions.

**Resolution:** Review state transitions to identify and break the cycle. Ensure all paths lead to a terminal state.

**Example:**
```bash
awf validate my-workflow
# Error [WORKFLOW.VALIDATION.CYCLE_DETECTED]: cycle detected: step1 -> step2 -> step3 -> step1
```

**Related codes:** `WORKFLOW.VALIDATION.INVALID_TRANSITION`, `WORKFLOW.VALIDATION.MISSING_STATE`

---

### WORKFLOW.VALIDATION.MISSING_STATE

**Description:** A state referenced in a transition does not exist in the workflow definition.

**Resolution:** Add the missing state definition or update the transition to reference an existing state.

**Example:**
```bash
awf validate my-workflow
# Error [WORKFLOW.VALIDATION.MISSING_STATE]: state 'cleanup' referenced in 'on_failure' but not defined
```

**Related codes:** `WORKFLOW.VALIDATION.CYCLE_DETECTED`, `WORKFLOW.VALIDATION.INVALID_TRANSITION`

---

### WORKFLOW.VALIDATION.INVALID_TRANSITION

**Description:** A transition rule is malformed or violates state machine constraints.

**Resolution:** Verify transition syntax. Check that source and target states are valid and transition logic is correct.

**Example:**
```bash
awf validate my-workflow
# Error [WORKFLOW.VALIDATION.INVALID_TRANSITION]: transition from terminal state 'success' is not allowed
```

**Related codes:** `WORKFLOW.VALIDATION.MISSING_STATE`, `WORKFLOW.VALIDATION.CYCLE_DETECTED`

---

## EXECUTION Category (Exit Code 3)

Runtime execution failures during workflow execution.

### EXECUTION.COMMAND.FAILED

**Description:** A shell command executed during workflow execution exited with a non-zero status code.

**Resolution:** Check command output for error details. Verify the command syntax and required dependencies are installed.

**Example:**
```bash
awf run build
# Error [EXECUTION.COMMAND.FAILED]: step 'compile' command exited with code 1
```

**Related codes:** `EXECUTION.COMMAND.TIMEOUT`, `SYSTEM.IO.PERMISSION_DENIED`

---

### EXECUTION.COMMAND.TIMEOUT

**Description:** A command execution exceeded the configured timeout duration.

**Resolution:** Increase the timeout value if the operation is expected to take longer, or optimize the command for faster execution.

**Example:**
```bash
awf run long-task
# Error [EXECUTION.COMMAND.TIMEOUT]: step 'process' timed out after 30s
```

**Related codes:** `EXECUTION.COMMAND.FAILED`

---

### EXECUTION.PARALLEL.PARTIAL_FAILURE

**Description:** Some branches in a parallel execution block failed while others succeeded.

**Resolution:** Review logs for failed branches. Fix underlying issues in failed steps or adjust parallel strategy.

**Example:**
```bash
awf run parallel-deploy
# Error [EXECUTION.PARALLEL.PARTIAL_FAILURE]: 2 of 5 parallel branches failed (strategy: all_succeed)
```

**Related codes:** `EXECUTION.COMMAND.FAILED`, `EXECUTION.COMMAND.TIMEOUT`

---

### EXECUTION.EVENT.DELIVERY_FAILED

**Description:** An event could not be delivered to a subscribed plugin via gRPC HandleEvent RPC.

**Resolution:** Check that the target plugin is running and responsive. Review plugin logs for errors in `HandleEvent` implementation. If the plugin is slow, consider offloading work to a goroutine within `HandleEvent`.

**Example:**
```bash
awf run deploy-pipeline
# Error [EXECUTION.EVENT.DELIVERY_FAILED]: failed to deliver event "workflow.completed" to plugin "awf-plugin-notify": context deadline exceeded
```

**Related codes:** `EXECUTION.EVENT.BUFFER_FULL`, `EXECUTION.COMMAND.FAILED`

---

### EXECUTION.EVENT.CYCLE_DETECTED

**Description:** Event propagation reached the maximum depth of 3, indicating a circular event chain between plugins.

**Resolution:** Review your plugins' `HandleEvent` implementations. If Plugin A emits an event that triggers Plugin B, which emits an event triggering Plugin A, the cycle is halted at depth 3. Redesign event flows to avoid circular dependencies.

**Example:**
```bash
awf run workflow
# Warning [EXECUTION.EVENT.CYCLE_DETECTED]: propagation depth exceeded for event "custom.ping" (depth 3); halting to prevent infinite loop
```

**Related codes:** `EXECUTION.EVENT.DELIVERY_FAILED`

---

### EXECUTION.EVENT.BUFFER_FULL

**Description:** A plugin's event buffer (256 events) is full. New events for this plugin are dropped while other plugins continue receiving normally.

**Resolution:** Speed up the plugin's `HandleEvent` implementation (target < 100ms). Use goroutines for blocking I/O. Consider reducing the number of subscribed event patterns.

**Example:**
```bash
awf run high-throughput
# Warning [EXECUTION.EVENT.BUFFER_FULL]: event buffer full for plugin "awf-plugin-metrics"; dropping event "step.completed"
```

**Related codes:** `EXECUTION.EVENT.DELIVERY_FAILED`

---

### EXECUTION.PLUGIN.CHECKSUM_MISMATCH

**Description:** The SHA-256 checksum of a plugin binary does not match the stored value recorded at install time. The plugin was refused before any code from the binary executed.

**Resolution:** The plugin binary may have been corrupted, accidentally overwritten, or tampered with. Reinstall the plugin with `awf plugin install <owner/repo> --force` to get a fresh copy with a valid checksum. If you intentionally replaced the binary (e.g., local development), run `awf plugin verify --update <name>` to recompute and store the new checksum.

**Example:**
```bash
awf run my-workflow
# Error [EXECUTION.PLUGIN.CHECKSUM_MISMATCH]: plugin "awf-plugin-jira" binary hash mismatch (expected a3f9d4..., got x1y2z3...)
```

**Related codes:** `EXECUTION.COMMAND.FAILED`

---

## SYSTEM Category (Exit Code 4)

Infrastructure and system-level failures.

### SYSTEM.IO.READ_FAILED

**Description:** An I/O error occurred while attempting to read from a file or stream.

**Resolution:** Check file permissions, disk space, and file system health. Verify the file is not locked by another process.

**Example:**
```bash
awf run backup
# Error [SYSTEM.IO.READ_FAILED]: failed to read state file: input/output error
```

**Related codes:** `SYSTEM.IO.PERMISSION_DENIED`, `USER.INPUT.MISSING_FILE`

---

### SYSTEM.IO.WRITE_FAILED

**Description:** An I/O error occurred while attempting to write to a file or stream.

**Resolution:** Check available disk space and write permissions. Verify the target directory exists and is writable.

**Example:**
```bash
awf run export
# Error [SYSTEM.IO.WRITE_FAILED]: failed to write output: disk full
```

**Related codes:** `SYSTEM.IO.PERMISSION_DENIED`

---

### SYSTEM.IO.PERMISSION_DENIED

**Description:** Insufficient permissions to access the requested file or directory.

**Resolution:** Check file permissions with `ls -l`. Use `chmod` to grant necessary permissions or run with appropriate user privileges.

**Example:**
```bash
awf run deploy
# Error [SYSTEM.IO.PERMISSION_DENIED]: cannot write to /etc/config: permission denied
```

**Related codes:** `SYSTEM.IO.READ_FAILED`, `SYSTEM.IO.WRITE_FAILED`

---

## Error Output Formats

### Human-Readable Format

Default CLI output shows error messages:

```bash
awf run deploy
# Error: workflow file not found
```

When structured error codes are used (currently being implemented for all commands), output will include the error code:

```bash
awf run deploy
# Error [WORKFLOW.VALIDATION.CYCLE_DETECTED]: cycle detected: step1 -> step2 -> step1
```

### JSON Format

Machine-readable structured error output:

```bash
awf run deploy --format json
```

```json
{
  "success": false,
  "error": {
    "code": 2,
    "error_code": "WORKFLOW.VALIDATION.CYCLE_DETECTED",
    "message": "cycle detected: step1 -> step2 -> step1",
    "details": {
      "cycle_path": ["step1", "step2", "step3", "step1"]
    },
    "timestamp": "2025-01-15T10:30:45Z"
  }
}
```

## Using Error Codes in Scripts

### Shell Script Example

```bash
#!/bin/bash

awf run deploy --input env=prod --format json > result.json
exit_code=$?

if [ $exit_code -eq 0 ]; then
  echo "Deployment successful"
elif [ $exit_code -eq 1 ]; then
  # User error - check input parameters
  error_code=$(jq -r '.error.error_code' result.json)
  if [[ "$error_code" == USER.INPUT.* ]]; then
    echo "Invalid input: check your parameters"
  fi
elif [ $exit_code -eq 2 ]; then
  # Workflow error - validate workflow definition
  error_code=$(jq -r '.error.error_code' result.json)
  echo "Workflow error: $error_code"
  awf error "$error_code"
elif [ $exit_code -eq 3 ]; then
  # Execution error - check command output
  echo "Execution failed - see logs"
elif [ $exit_code -eq 4 ]; then
  # System error - check infrastructure
  echo "System error - check permissions and disk space"
fi
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Run AWF workflow
  id: awf
  run: |
    awf run deploy --format json --input env=staging > output.json
    echo "exit_code=$?" >> $GITHUB_OUTPUT

- name: Handle errors
  if: steps.awf.outputs.exit_code != '0'
  run: |
    ERROR_CODE=$(jq -r '.error.error_code' output.json)
    ERROR_MSG=$(jq -r '.error.message' output.json)

    case "$ERROR_CODE" in
      USER.*)
        echo "::error::User error: $ERROR_MSG"
        ;;
      WORKFLOW.*)
        echo "::error::Workflow error: $ERROR_MSG"
        awf validate deploy
        ;;
      EXECUTION.*)
        echo "::error::Execution error: $ERROR_MSG"
        cat logs/latest.log
        ;;
      SYSTEM.*)
        echo "::error::System error: $ERROR_MSG"
        df -h
        ;;
    esac
    exit 1
```

## Error Code Lookup Command

The `awf error` command provides interactive error code documentation:

```bash
# List all error codes with descriptions
awf error

# Look up specific error code
awf error USER.INPUT.MISSING_FILE

# Look up by category (shows all matching codes)
awf error WORKFLOW.VALIDATION

# Get JSON output for programmatic use
awf error EXECUTION.COMMAND.FAILED --format json
```

**JSON output example:**

```json
{
  "code": "EXECUTION.COMMAND.FAILED",
  "description": "A shell command executed during workflow execution exited with a non-zero status code.",
  "resolution": "Check command output for error details. Verify the command syntax and required dependencies are installed.",
  "related_codes": [
    "EXECUTION.COMMAND.TIMEOUT",
    "SYSTEM.IO.PERMISSION_DENIED"
  ]
}
```

## Migration from Legacy Exit Codes

If you're migrating from AWF versions before v0.4.0, the error code taxonomy preserves backward compatibility:

| Legacy Exit Code | New Error Code Examples | Notes |
|------------------|-------------------------|-------|
| 1 (User Error) | `USER.INPUT.*` | All user-facing errors |
| 2 (Workflow Error) | `WORKFLOW.PARSE.*`, `WORKFLOW.VALIDATION.*` | Workflow definition errors |
| 3 (Execution Error) | `EXECUTION.COMMAND.*`, `EXECUTION.PARALLEL.*`, `EXECUTION.EVENT.*` | Runtime failures |
| 4 (System Error) | `SYSTEM.IO.*` | Infrastructure errors |

Exit codes remain unchanged, but error messages now include structured error codes for programmatic handling.

## Actionable Error Hints

AWF includes a context-aware hint system that provides actionable suggestions to help resolve errors quickly. Hints are displayed automatically after error details and can be suppressed using the `--no-hints` flag.

### Hint Display Format

Hints appear as dimmed text below the error message:

```bash
$ awf run my-workfow.yaml
[USER.INPUT.MISSING_FILE] workflow not found
  Details:
    path: my-workfow.yaml

  Hint: Did you mean 'my-workflow.yaml'?
  Hint: Run 'awf list' to see available workflows
```

### Suppressing Hints

Use `--no-hints` to disable hint suggestions (useful for CI/CD scripts):

```bash
$ awf run missing.yaml --no-hints
[USER.INPUT.MISSING_FILE] workflow not found
  Details:
    path: missing.yaml
```

### Hint Types

#### File Not Found Hints

Suggests similar filenames using fuzzy matching:

```bash
$ awf run deploy-prd.yaml
[USER.INPUT.MISSING_FILE] workflow not found
  Details:
    path: deploy-prd.yaml

  Hint: Did you mean 'deploy-prod.yaml'?
  Hint: Run 'awf list' to see available workflows
```

#### YAML Syntax Hints

Points to the exact line and column of syntax errors:

```bash
$ awf validate broken.yaml
[WORKFLOW.PARSE.YAML_SYNTAX] invalid YAML syntax
  Details:
    column: 5
    line: 12

  Hint: Check line 12, column 5 for syntax errors
  Hint: Validate with: yamllint broken.yaml
```

#### Invalid State Reference Hints

Suggests the closest matching state name:

```bash
$ awf validate deploy.yaml
[WORKFLOW.VALIDATION.MISSING_STATE] state 'proces' not defined
  Details:
    available_states: [start, process, cleanup, done]
    state: proces

  Hint: Did you mean 'process'?
  Hint: Available states: start, process, cleanup, done
```

#### Missing Input Hints

Lists required inputs with example usage:

```bash
$ awf run deploy.yaml
[USER.INPUT.VALIDATION_FAILED] required input missing
  Details:
    input: user_name

  Hint: Required inputs: user_name (string), user_email (string)
  Hint: Example: awf run deploy.yaml --input user_name=john --input user_email=john@example.com
```

#### Command Execution Hints

Provides context for exit codes:

```bash
$ awf run deploy.yaml
[EXECUTION.COMMAND.FAILED] command exited with code 127
  Details:
    command: nonexistent-command
    exit_code: 127

  Hint: Exit code 127 indicates command not found
  Hint: Check if 'nonexistent-command' is installed and in PATH
```

```bash
$ awf run deploy.yaml
[EXECUTION.COMMAND.FAILED] command exited with code 126
  Details:
    command: ./deploy.sh
    exit_code: 126

  Hint: Exit code 126 indicates permission denied
  Hint: Check file permissions with: ls -l ./deploy.sh
  Hint: Add execute permission: chmod +x ./deploy.sh
```

### JSON Format Hints

Hints are included in JSON output as an array:

```bash
$ awf run missing.yaml --format json
```

```json
{
  "success": false,
  "error": {
    "code": 1,
    "error_code": "USER.INPUT.MISSING_FILE",
    "message": "workflow not found",
    "details": {
      "path": "missing.yaml"
    },
    "hints": [
      "Did you mean 'missing-workflow.yaml'?",
      "Run 'awf list' to see available workflows"
    ],
    "timestamp": "2026-01-15T10:30:45Z"
  }
}
```

## See Also

- [Exit Codes](exit-codes.md) - Basic exit code reference
- [Workflow Syntax](../user-guide/workflow-syntax.md) - Workflow definition
- [Commands](../user-guide/commands.md) - CLI command reference
