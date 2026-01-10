# Workflow Templates

Templates allow defining reusable step patterns with parameters. Define once, use in multiple workflows.

## Template Definition

Templates are defined in `.awf/templates/` directory.

### Basic Structure

```yaml
# .awf/templates/ai-analyze.yaml
name: ai-analyze
parameters:
  - name: prompt
    required: true
  - name: model
    default: claude
  - name: timeout
    default: 120
states:
  ai-analyze:
    type: step
    command: "{{parameters.model}} -c '{{parameters.prompt}}'"
    timeout: "{{parameters.timeout}}"
    capture:
      stdout: analysis
```

### Parameter Options

| Option | Type | Description |
|--------|------|-------------|
| `name` | string | Parameter identifier |
| `required` | bool | If true, must be provided when using template |
| `default` | any | Default value if not provided |

## Template Usage

Reference templates in workflow steps with `use_template`:

```yaml
# .awf/workflows/my-workflow.yaml
name: my-workflow
version: "1.0.0"

states:
  initial: code_analysis

  code_analysis:
    use_template: ai-analyze
    parameters:
      prompt: "Analyze this code: {{.states.extract.Output}}"
      model: gemini
    on_success: format
    on_failure: error

  format:
    type: step
    command: format-output
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

### Template Step Options

| Option | Description |
|--------|-------------|
| `use_template` | Template name to instantiate |
| `parameters` | Parameter values to pass to template |
| `on_success` | Overrides template's on_success transition |
| `on_failure` | Overrides template's on_failure transition |

## Parameter Interpolation

Template parameters use `{{parameters.name}}` syntax:

```yaml
# Template definition
command: "{{parameters.model}} -c '{{parameters.prompt}}'"
timeout: "{{parameters.timeout}}"

# Workflow usage - these values replace the placeholders
parameters:
  model: claude
  prompt: "Analyze code"
  timeout: 60
```

Parameters are resolved at workflow load time, not runtime.

## Template Discovery

Templates are loaded from (in order):

1. `.awf/templates/` (local project)
2. `$AWF_STORAGE/templates/` (global)

Local templates override global ones with the same name.

## Validation

Templates are validated when workflows are loaded:

- Missing required parameters produce clear errors
- Circular template references are detected
- Use `awf validate` to check template references before execution

```bash
# Validate workflow with templates
awf validate my-workflow
```

### Error Examples

```
Error: template 'ai-analyze' missing required parameter 'prompt'
Error: circular template reference detected: a -> b -> a
Error: template 'unknown-template' not found
```

## Complete Example

### Template: HTTP Request

```yaml
# .awf/templates/http-request.yaml
name: http-request
parameters:
  - name: url
    required: true
  - name: method
    default: GET
  - name: headers
    default: ""
  - name: timeout
    default: 30
states:
  http-request:
    type: step
    command: |
      curl -s -X {{parameters.method}} \
        {{parameters.headers}} \
        --max-time {{parameters.timeout}} \
        "{{parameters.url}}"
    timeout: "{{parameters.timeout}}"
```

### Workflow Using Template

```yaml
# .awf/workflows/api-check.yaml
name: api-check
version: "1.0.0"

inputs:
  - name: api_url
    type: string
    required: true

states:
  initial: health_check

  health_check:
    use_template: http-request
    parameters:
      url: "{{.inputs.api_url}}/health"
      timeout: 10
    on_success: fetch_data
    on_failure: error

  fetch_data:
    use_template: http-request
    parameters:
      url: "{{.inputs.api_url}}/data"
      method: GET
      headers: "-H 'Authorization: Bearer $API_TOKEN'"
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
    status: failure
```

```bash
awf run api-check --input api_url=https://api.example.com
```

## Best Practices

1. **Keep templates focused** - One template, one responsibility
2. **Use sensible defaults** - Make common cases easy
3. **Document parameters** - Add comments explaining each parameter
4. **Validate early** - Run `awf validate` after creating workflows
5. **Version templates** - Use semantic versioning in template names if needed

## See Also

- [Workflow Syntax](workflow-syntax.md) - Full YAML syntax reference
- [Variable Interpolation](../reference/interpolation.md) - Template variables
- [Examples](examples.md) - More workflow examples
