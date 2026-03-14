---
title: "Project Configuration"
---

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

### Environment Variable Override

You can override the configuration file path using the `AWF_CONFIG_PATH` environment variable:

```bash
# Use a custom config file location
export AWF_CONFIG_PATH="/path/to/custom/config.yaml"
awf run my-workflow

# Or inline for a single command
AWF_CONFIG_PATH=./configs/staging.yaml awf run deploy
```

This is useful for:
- Testing with different configurations
- CI/CD pipelines with environment-specific configs
- Running multiple AWF instances with isolated configurations

| Variable | Default | Description |
|----------|---------|-------------|
| `AWF_CONFIG_PATH` | `.awf/config.yaml` | Absolute or relative path to the configuration file |
| `AWF_AUDIT_LOG` | `$XDG_DATA_HOME/awf/audit.jsonl` | Audit trail file path; set to `off` to disable |

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

When you run a workflow, AWF automatically merges inputs from the config file with any CLI-provided inputs. This applies to all execution modes: standard `awf run`, `--interactive`, and `--dry-run`.

### Priority Order

Inputs are resolved in the following order (later sources override earlier ones):

```
Config File < CLI Flags
```

1. **Config file** (`.awf/config.yaml`): Base defaults
2. **CLI flags** (`--input`): Override config values

### Standard Execution

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

### Interactive Mode

In interactive mode (`awf run --interactive`), config values reduce prompting:

```bash
$ awf run deploy --interactive --input env=production

# env is NOT prompted (provided via CLI)
# project is NOT prompted (in config.yaml)
# Only missing required inputs are prompted
```

If the workflow had a third required input not in config or CLI:

```bash
$ awf run deploy --interactive

# env = "staging" (from config, not prompted)
# project = "my-app" (from config, not prompted)
# Only other_required_input is prompted interactively

Enter value for 'other_required_input' (string, required)
> value
```

### Dry-Run Mode

In dry-run mode (`awf run --dry-run`), config values are included in the execution plan:

```bash
$ awf run deploy --dry-run

# Execution plan shows:
# - env = "staging" (from config)
# - project = "my-app" (from config)
```

This allows you to verify that config values are correctly applied without actually executing the workflow.

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

## Workflow-Level Configuration

In addition to project inputs, workflows can define configuration options that control execution behavior and resource management.

### Output Configuration

Control how step outputs are captured and stored:

```yaml
# In workflow YAML file
output:
  max_size: "1MB"             # Maximum output size per step (default: 1MB)
  stream_large_output: false  # Stream large outputs to temp files (default: false)
  temp_dir: "/tmp/awf"        # Directory for temp files (default: system temp)
```

#### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_size` | string | `"1MB"` | Maximum size for captured output/stderr per step. Accepts units: `B`, `KB`, `MB`, `GB` |
| `stream_large_output` | bool | `false` | When `true`, outputs exceeding `max_size` are streamed to temporary files instead of truncated |
| `temp_dir` | string | system temp | Directory for temporary output files when streaming is enabled |

#### Behavior

- **Truncation (default)**: When `max_size` is exceeded and `stream_large_output` is `false`, output is truncated with a warning logged
- **Streaming**: When `max_size` is exceeded and `stream_large_output` is `true`, output is written to a temp file and accessible via `{{.states.step_name.OutputPath}}`
- **Backward compatibility**: Omitting `output` config preserves existing behavior (unlimited output capture)

#### Example

```yaml
name: large-log-workflow
version: "1.0.0"

output:
  max_size: "500KB"
  stream_large_output: true
  temp_dir: "/var/tmp/workflow-outputs"

states:
  initial: generate_logs

  generate_logs:
    type: step
    command: generate-large-logfile
    on_success: process_logs

  process_logs:
    type: step
    # If output was streamed, OutputPath contains file location
    command: |
      if [ -n "{{.states.generate_logs.OutputPath}}" ]; then
        process-file "{{.states.generate_logs.OutputPath}}"
      else
        echo "{{.states.generate_logs.Output}}" | process-stdin
      fi
    on_success: done

  done:
    type: terminal
    status: success
```

### Loop Configuration

Configure memory management for loop iterations:

```yaml
# In workflow YAML file
loop:
  max_retained_iterations: 100  # Keep only last N iterations (default: 0 = unlimited)
```

#### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_retained_iterations` | int | `0` | Maximum loop iterations retained in memory. `0` = unlimited (backward compatible) |

#### Behavior

- **Rolling window**: When set, only the last N iteration results are kept in memory
- **Pruned iterations**: Earlier iterations are discarded and counted in `{{.states.loop_name.PrunedCount}}`
- **Memory efficiency**: Prevents unbounded memory growth in long-running loops (1000+ iterations)
- **Backward compatibility**: Default `0` preserves existing behavior

#### Example

```yaml
name: long-running-poll
version: "1.0.0"

loop:
  max_retained_iterations: 50  # Keep only last 50 iterations

states:
  initial: poll_api

  poll_api:
    type: while
    while: "states.check_status.Output != 'complete'"
    max_iterations: 10000
    body:
      - check_status
      - wait
    on_complete: report

  check_status:
    type: step
    command: curl -s https://api.example.com/job/status
    on_success: poll_api

  wait:
    type: step
    command: sleep 10
    on_success: poll_api

  report:
    type: step
    command: |
      echo "Total iterations: {{.states.poll_api.Iterations | len}}"
      echo "Pruned iterations: {{.states.poll_api.PrunedCount}}"
    on_success: done

  done:
    type: terminal
    status: success
```

### Memory Monitoring

Enable memory usage logging for workflows:

```yaml
# In workflow YAML file
monitoring:
  enabled: true              # Enable monitoring (default: false)
  memory_threshold: "500MB"  # Log warning when exceeded (default: 500MB)
```

#### Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `false` | Enable memory monitoring and logging |
| `memory_threshold` | string | `"500MB"` | Threshold for logging warnings. Accepts units: `B`, `KB`, `MB`, `GB` |

#### Behavior

- When enabled, AWF logs memory usage at key execution points
- Warnings are logged when heap allocation exceeds `memory_threshold`
- Useful for debugging memory issues in long-running workflows
- No performance impact when disabled (default)

#### Example

```yaml
name: memory-intensive-workflow
version: "1.0.0"

monitoring:
  enabled: true
  memory_threshold: "1GB"

output:
  max_size: "1MB"

loop:
  max_retained_iterations: 100

states:
  initial: process_data
  # ... workflow states
```

---

## Plugin Configuration

Configure built-in and external plugins in `.awf/config.yaml` under the `plugins:` key. Each plugin has its own configuration section.

### Notification Plugin

```yaml
plugins:
  notify:
    default_backend: "desktop"
```

| Key | Description |
|-----|-------------|
| `default_backend` | Backend used when `backend` input is omitted from `notify.send` |

When both a config `default_backend` and an explicit `backend` input are set on a step, the explicit input takes precedence.

### External Plugins

```yaml
plugins:
  awf-plugin-github:
    token: "${GITHUB_TOKEN}"
```

Environment variables in config values are expanded at runtime.

See [Plugins](plugins.md) for full plugin documentation.

---

## Environment Variables

AWF respects the following environment variables to control behavior and override defaults:

### Core Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AWF_CONFIG_PATH` | `.awf/config.yaml` | Path to project configuration file (absolute or relative) |

### Audit Trail

| Variable | Default | Description |
|----------|---------|-------------|
| `AWF_AUDIT_LOG` | `$XDG_DATA_HOME/awf/audit.jsonl` | Path to audit trail file. Set to `off` to disable audit recording. |

See [Audit Trail](audit-trail.md) for complete audit trail configuration and usage guide.

### XDG Directories

AWF follows the [XDG Base Directory](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html) standard:

| Variable | Default (Linux) | Purpose |
|----------|-----------------|---------|
| `XDG_CONFIG_HOME` | `~/.config` | Project configuration directory (not typically overridden) |
| `XDG_DATA_HOME` | `~/.local/share` | Audit trail and runtime data storage |

When `$XDG_DATA_HOME` is not set, AWF defaults to `~/.local/share`.

### Examples

```bash
# Use custom config file
export AWF_CONFIG_PATH="/etc/awf/config.yaml"

# Store audit trail in syslog-compatible location
export AWF_AUDIT_LOG="/var/log/awf/audit.jsonl"

# Disable audit trail for rapid iteration
export AWF_AUDIT_LOG=off

# Use custom data directory (affects audit trail storage)
export XDG_DATA_HOME="/var/lib/awf"
awf run my-workflow
```

---

## See Also

- [Commands](commands.md) - CLI command reference
- [Interactive Input Collection](interactive-inputs.md) - Automatic prompting for missing inputs with config pre-population
- [Workflow Syntax](workflow-syntax.md) - Workflow YAML syntax
- [Plugins](plugins.md) - Plugin system and configuration
- [Audit Trail](audit-trail.md) - Structured audit logging for workflow executions
