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
{{.states.step_name.output}}
{{.states.step_name.exit_code}}
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
    claude -c "Analyze: {{.states.read_file.output}}"
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
  analyze "{{.states.process.output}}"
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
