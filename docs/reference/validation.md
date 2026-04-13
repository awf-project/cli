---
title: "Input Validation"
---

AWF validates workflow inputs at runtime against defined rules.

## Input Definition

```yaml
inputs:
  - name: input_name
    type: string          # string, integer, boolean
    required: true        # must be provided
    default: "value"      # default if not provided
    validation:           # validation rules
      pattern: "^[a-z]+$"
```

## Supported Types

| Type | Description | Example Values |
|------|-------------|----------------|
| `string` | Text value | `"hello"`, `"path/to/file"` |
| `integer` | Whole number | `42`, `100`, `-5` |
| `boolean` | True/false | `true`, `false` |

## Validation Rules

### Pattern (regex)

Validate string matches a regular expression:

```yaml
inputs:
  - name: email
    type: string
    required: true
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"

  - name: version
    type: string
    validation:
      pattern: "^v?[0-9]+\\.[0-9]+\\.[0-9]+$"
```

### Enum (allowed values)

Restrict to a set of allowed values:

```yaml
inputs:
  - name: env
    type: string
    default: staging
    validation:
      enum: [dev, staging, prod]

  - name: log_level
    type: string
    default: info
    validation:
      enum: [debug, info, warn, error]
```

### Min/Max (numeric range)

Validate integer falls within range:

```yaml
inputs:
  - name: count
    type: integer
    default: 10
    validation:
      min: 1
      max: 100

  - name: port
    type: integer
    default: 8080
    validation:
      min: 1024
      max: 65535
```

### File Exists

Validate file exists on filesystem:

```yaml
inputs:
  - name: config_file
    type: string
    required: true
    validation:
      file_exists: true
```

### File Extension

Validate file has allowed extension:

```yaml
inputs:
  - name: source_file
    type: string
    required: true
    validation:
      file_extension: [".go", ".py", ".js", ".ts"]

  - name: config
    type: string
    validation:
      file_exists: true
      file_extension: [".yaml", ".yml", ".json"]
```

## Combining Rules

Multiple validation rules can be combined:

```yaml
inputs:
  - name: source_file
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".go", ".py"]

  - name: threads
    type: integer
    default: 4
    validation:
      min: 1
      max: 32
```

## Error Handling

### Non-Fail-Fast

Validation errors are collected and reported together:

```bash
awf run deploy --input email=invalid --input count=999
```

```
input validation failed: 2 errors:
  - inputs.email: does not match pattern ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$
  - inputs.count: value 999 exceeds maximum 100
```

### Exit Code

Validation failures return exit code 1 (User Error).

### JSON Output

Use `--format json` for structured errors:

```bash
awf run deploy --input email=invalid -f json
```

```json
{
  "success": false,
  "error": {
    "code": 1,
    "type": "validation_error",
    "message": "input validation failed",
    "details": {
      "errors": [
        {
          "input": "email",
          "rule": "pattern",
          "message": "does not match pattern"
        }
      ]
    }
  }
}
```

## Complete Example

```yaml
name: deploy
version: "1.0.0"

inputs:
  - name: env
    type: string
    required: true
    validation:
      enum: [dev, staging, prod]

  - name: version
    type: string
    required: true
    validation:
      pattern: "^v[0-9]+\\.[0-9]+\\.[0-9]+$"

  - name: config_file
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".yaml", ".yml"]

  - name: replicas
    type: integer
    default: 2
    validation:
      min: 1
      max: 10

  - name: dry_run
    type: boolean
    default: false

states:
  initial: validate
  validate:
    type: step
    command: |
      echo "Deploying {{.inputs.version}} to {{.inputs.env}}"
      echo "Config: {{.inputs.config_file}}"
      echo "Replicas: {{.inputs.replicas}}"
      echo "Dry run: {{.inputs.dry_run}}"
    on_success: done
  done:
    type: terminal
```

```bash
# Valid
awf run deploy \
  --input env=prod \
  --input version=v1.2.3 \
  --input config_file=deploy.yaml \
  --input replicas=3

# Invalid - multiple errors
awf run deploy \
  --input env=invalid \
  --input version=1.2.3 \
  --input replicas=99
```

## Validation at Different Stages

| Stage | What's Validated |
|-------|------------------|
| Load time | YAML syntax, state references, template references, transition requirements |
| Runtime | Input values against validation rules |
| `awf validate` | All load-time checks without execution |

### Workflow Structure Rules

Load-time validation enforces structural rules on workflow states:

| Rule | Applies To | Description |
|------|-----------|-------------|
| Transition required | Command steps | `on_success`/`on_failure` or `transitions` must be defined |
| Transition exempt | Parallel branch children | Steps listed in a parallel step's `parallel` field do not require transitions (the parallel executor controls flow) |
| Transition exempt | Loop body steps | Steps listed in a loop's `body` field have relaxed transition target validation |
| Branches required | Parallel steps | `parallel` field must list at least one branch step |
| Valid strategy | Parallel steps | `strategy` must be `all_succeed`, `any_succeed`, `best_effort`, or empty |

See [Workflow Syntax](../user-guide/workflow-syntax.md#parallel-state) for parallel step details.

## Best Practices

1. **Always validate file inputs** - Use `file_exists` for file paths
2. **Use enums for fixed choices** - Prevents typos and invalid values
3. **Set sensible defaults** - Reduce required inputs when possible
4. **Document patterns** - Add comments explaining regex patterns
5. **Combine rules** - Use multiple rules for comprehensive validation

## See Also

- [Agent Model Validation](../user-guide/agent-steps.md#model-validation) - Provider-specific model name validation (Claude, Gemini, Codex)
- [Workflow Syntax](../user-guide/workflow-syntax.md) - Input definition syntax
- [Variable Interpolation](interpolation.md) - Using input values
- [Exit Codes](exit-codes.md) - Error codes
