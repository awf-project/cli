# Variable Interpolation

AWF uses Go template syntax (`{{.var}}`) for variable interpolation in workflow definitions.

## Syntax

Variables are enclosed in double curly braces with a dot prefix:

```yaml
command: echo "Hello, {{.inputs.name}}!"
```

## Variable Categories

### Input Variables

Access workflow input values:

```yaml
{{.inputs.variable_name}}
```

Example:
```yaml
inputs:
  - name: file_path
    type: string
  - name: max_tokens
    type: integer

states:
  initial: process
  process:
    type: step
    command: |
      process-file "{{.inputs.file_path}}" --tokens={{.inputs.max_tokens}}
```

### State Variables

Access output and exit code from previous steps:

```yaml
{{.states.step_name.Output}}
{{.states.step_name.ExitCode}}
```

Example:
```yaml
read_file:
  type: step
  command: cat "{{.inputs.file}}"
  on_success: analyze

analyze:
  type: step
  command: |
    claude -c "Analyze: {{.states.read_file.Output}}"
```

### Workflow Metadata

Access workflow execution information:

```yaml
{{.workflow.id}}
{{.workflow.name}}
{{.workflow.duration}}
```

Example:
```yaml
log_result:
  type: step
  command: |
    echo "Workflow {{.workflow.name}} ({{.workflow.id}}) completed"
```

### Environment Variables

Access system environment variables:

```yaml
{{.env.VARIABLE_NAME}}
```

Example:
```yaml
deploy:
  type: step
  command: |
    deploy.sh --env={{.env.DEPLOY_ENV}} --token={{.env.API_TOKEN}}
```

### Loop Context Variables

Available inside `for_each` and `while` loops:

```yaml
{{.loop.item}}      # Current item value (for_each only)
{{.loop.index}}     # 0-based iteration index
{{.loop.index1}}    # 1-based iteration index
{{.loop.first}}     # True on first iteration
{{.loop.last}}      # True on last iteration (for_each only)
{{.loop.length}}    # Total items (-1 for while loops)
{{.loop.parent}}    # Parent loop context (nested loops)
```

Example:
```yaml
process_files:
  type: for_each
  items: '["a.txt", "b.txt", "c.txt"]'
  body:
    - process_single
  on_complete: done

process_single:
  type: step
  command: |
    echo "Processing {{.loop.item}} ({{.loop.index1}}/{{.loop.length}})"
    echo "First: {{.loop.first}}, Last: {{.loop.last}}"
```

#### Loop Item JSON Serialization

When `{{.loop.item}}` contains complex types (objects, arrays), it is automatically serialized to JSON:

- **Objects** → JSON object: `{"name":"value","nested":{"key":"data"}}`
- **Arrays** → JSON array: `[1,2,3]` or `["a","b","c"]`
- **Strings** → Pass through unchanged: `"main.go"` stays as `main.go` (not quoted)
- **Numbers** → Converted to string: `42` becomes `"42"`, `3.14` becomes `"3.14"`
- **Booleans** → Converted to string: `true` becomes `"true"`, `false` becomes `"false"`

This is especially useful when passing loop items to `call_workflow`:

```yaml
# Parent workflow with objects
process_reviews:
  type: step
  command: |
    echo '[{"file":"main.go","type":"fix"},{"file":"test.go","type":"chore"}]'
  capture:
    stdout: reviews_json
  on_success: loop_reviews

loop_reviews:
  type: for_each
  items: "{{.states.process_reviews.Output}}"
  body:
    - call_child_workflow

call_child_workflow:
  type: call_workflow
  workflow: review-file
  inputs:
    review: "{{.loop.item}}"  # Passed as valid JSON object to child

# Child workflow receives properly formatted JSON
# {{.inputs.review}} = {"file":"main.go","type":"fix"}
```

**Note**: String items pass through unchanged without JSON quoting. Numbers and booleans are converted to their string representations.

### Nested Loop Parent Access

For nested loops, access outer loop context via `{{.loop.parent}}`:

```yaml
process:
  type: step
  command: |
    echo "outer={{.loop.parent.item}} inner={{.loop.item}}"
    echo "outer_index={{.loop.parent.index}} inner_index={{.loop.index}}"
```

Chain for deeper nesting:
```yaml
command: echo "level1={{.loop.parent.parent.item}} level2={{.loop.parent.item}} level3={{.loop.item}}"
```

### Error Variables (in hooks)

Available in error hooks:

```yaml
{{.error.type}}
{{.error.message}}
```

## Security Considerations

### Shell Injection

User-provided values in commands can be dangerous. AWF provides `ShellEscape()` in `pkg/interpolation` for escaping:

```go
import "github.com/vanoix/awf/pkg/interpolation"

escaped := interpolation.ShellEscape(userInput)
```

### Secret Masking

Variables with these prefixes are masked in logs:
- `SECRET_`
- `API_KEY`
- `PASSWORD`
- `TOKEN`

Example:
```yaml
# In logs: API_KEY=****
command: curl -H "Authorization: Bearer {{.env.API_KEY}}" https://api.example.com
```

## Interpolation in Different Contexts

### Commands

```yaml
command: echo "{{.inputs.message}}"
```

### Working Directory

```yaml
dir: "{{.inputs.project_path}}"
```

### Timeout

```yaml
timeout: "{{.inputs.timeout}}"
```

### Conditional Expressions

```yaml
transitions:
  - when: "inputs.mode == 'full'"
    goto: full_process
```

Note: In `when` expressions, use variable names without `{{}}` and without the dot prefix.

### Loop Items

```yaml
items: "{{.inputs.files}}"
```

Or literal JSON:
```yaml
items: '["a.txt", "b.txt", "c.txt"]'
```

### Loop Bounds (max_iterations)

Loop bounds support interpolation and arithmetic:

```yaml
# From input
max_iterations: "{{.inputs.retry_limit}}"

# From environment
max_iterations: "{{.env.MAX_RETRIES}}"

# Arithmetic expression
max_iterations: "{{.inputs.pages * .inputs.retries_per_page}}"
```

Supported operators: `+`, `-`, `*`, `/`, `%`

Dynamic values are resolved at loop initialization (before first iteration).
Static validation warns about undefined variables during `awf validate`.

### Loop Conditions

The `while` and `until` conditions support interpolation:

```yaml
while: "{{.states.check.Output}} != 'done'"
until: "{{.states.counter.Output}} >= {{.inputs.threshold}}"
```

## Template Parameters

Template parameters use a different syntax: `{{parameters.name}}`

```yaml
# In template definition
command: "{{parameters.model}} -c '{{parameters.prompt}}'"

# Resolved at load time, not runtime
```

See [Templates](../user-guide/templates.md) for details.

## Common Patterns

### Multi-line Commands

```yaml
command: |
  echo "Step 1: Process {{.inputs.file}}"
  process-file "{{.inputs.file}}"
  echo "Step 2: Analyze output"
  analyze "{{.states.process.Output}}"
```

### Conditional Values

Use shell conditionals:
```yaml
command: |
  if [ "{{.inputs.verbose}}" = "true" ]; then
    echo "Verbose mode enabled"
  fi
  run-command --verbose={{.inputs.verbose}}
```

### JSON in Commands

Escape quotes properly:
```yaml
command: |
  curl -X POST -d '{"file": "{{.inputs.file}}"}' https://api.example.com
```

## Debugging

Use `--dry-run` to see resolved values:

```bash
awf run my-workflow --dry-run --input file=test.txt
```

Output shows interpolated commands without executing them.

## See Also

- [Workflow Syntax](../user-guide/workflow-syntax.md) - Complete YAML reference
- [Input Validation](validation.md) - Validation rules
- [Templates](../user-guide/templates.md) - Workflow templates
