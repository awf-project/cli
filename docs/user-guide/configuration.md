# Project Configuration

AWF supports project-level configuration through a YAML file that pre-populates workflow inputs, reducing repetitive command-line arguments.

## Overview

The project configuration file allows you to define default input values that are automatically applied when running workflows. This is useful for:

- Setting project-specific defaults (e.g., project ID, environment)
- Reducing repetitive `--input` flags in daily workflow usage
- Sharing common configuration across team members via version control

---

## Configuration File Location

The configuration file is located at:

```
.awf/config.yaml
```

This path is relative to the current working directory. AWF searches for this file when executing commands like `run`, `config show`, etc.

---

## Configuration Format

The configuration file uses YAML format with the following structure:

```yaml
# Project configuration for AWF
# Values defined here are used as defaults for workflow inputs

inputs:
  # String value
  project: "my-project-id"

  # Environment setting
  env: "staging"

  # Numeric value
  max_tokens: 4000

  # Boolean value
  debug: false
```

### Supported Value Types

| Type | Example | Description |
|------|---------|-------------|
| String | `name: "value"` | Text values (quotes optional for simple strings) |
| Integer | `count: 42` | Whole numbers |
| Float | `ratio: 3.14` | Decimal numbers |
| Boolean | `enabled: true` | `true` or `false` |

---

## Input Pre-population

When you run a workflow, AWF automatically merges inputs from the config file with any CLI-provided inputs.

### Priority Order

Inputs are resolved in the following order (later sources override earlier ones):

```
Config File < CLI Flags
```

1. **Config file** (`.awf/config.yaml`): Base defaults
2. **CLI flags** (`--input`): Override config values

### Example

Given this configuration:

```yaml
inputs:
  env: "staging"
  project: "my-app"
```

And this command:

```bash
awf run deploy --input env=production
```

The workflow receives:
- `env` = `production` (CLI override wins)
- `project` = `my-app` (from config)

---

## Initialization

The `awf init` command creates a template configuration file:

```bash
awf init
```

This generates `.awf/config.yaml` with commented examples:

```yaml
# AWF Project Configuration
# Uncomment and modify values as needed

inputs:
  # project: "my-project"
  # env: "development"
```

If a config file already exists, `awf init` preserves it (use `--force` to overwrite).

---

## Viewing Configuration

Use the `awf config show` command to display the current configuration:

```bash
# Display all configured inputs
awf config show

# JSON output for scripting
awf config show --format json
```

See [Commands](commands.md) for full command reference.

---

## Validation and Errors

AWF validates the configuration file when loading it.

### Invalid YAML

If the configuration file contains invalid YAML syntax, AWF reports an error with the file path and stops execution:

```
Error: failed to parse config file .awf/config.yaml: yaml: line 3: mapping values are not allowed in this context
```

Fix the YAML syntax and retry the command.

### Unknown Keys

If the configuration contains unrecognized keys (anything other than `inputs:`), AWF logs a warning but continues execution:

```
Warning: unknown key in config file: deprecated_setting
```

This allows forward compatibility while alerting you to potential typos or outdated settings.

---

## Best Practices

### Do Not Store Secrets

The configuration file should **not** contain sensitive information:

- API keys
- Passwords or credentials
- Authentication tokens
- Private keys

Instead, use environment variables for secrets:

```yaml
# Good: Reference environment variable in workflow
# inputs.api_key: "{{.env.MY_API_KEY}}"

# Bad: Never store secrets directly
# inputs:
#   api_key: "sk-1234567890abcdef"
```

### Version Control

Include `.awf/config.yaml` in version control to share project defaults with your team. Since it should not contain secrets, it's safe to commit.

### Comments

Use YAML comments to document your configuration:

```yaml
inputs:
  # Jira project key for issue tracking
  project: "MYAPP"

  # Default environment for deployments
  # Options: development, staging, production
  env: "staging"
```

---

## See Also

- [Commands](commands.md) - CLI command reference
- [Workflow Syntax](workflow-syntax.md) - Workflow YAML syntax
