# CLI Commands

## Overview

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in current directory |
| `awf run <workflow>` | Execute a workflow |
| `awf resume [workflow-id]` | Resume an interrupted workflow |
| `awf list` | List available workflows |
| `awf status <id>` | Show execution status |
| `awf validate <workflow>` | Validate workflow syntax |
| `awf history` | Show workflow execution history |
| `awf version` | Show version info |
| `awf completion <shell>` | Generate shell autocompletion |

## Global Flags

These flags work with all commands:

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output |
| `--quiet, -q` | Suppress non-error output |
| `--no-color` | Disable colored output |
| `--format, -f` | Output format (text, json, table, quiet) |
| `--config` | Path to config file |
| `--storage` | Path to storage directory |
| `--log-level` | Log level (debug, info, warn, error) |

## Output Formats

| Format | Description | Use case |
|--------|-------------|----------|
| `text` | Human-readable with colors (default) | Interactive terminal |
| `json` | Structured JSON | Scripting, pipes, CI/CD |
| `table` | Aligned columns with headers | Lists, reports |
| `quiet` | Minimal output (IDs, exit codes only) | Silent scripts |

```bash
# JSON output for scripting
awf list -f json
awf status abc123 -f json

# Quiet mode for pipelines
WORKFLOW_ID=$(awf run deploy -f quiet)
awf status $WORKFLOW_ID -f quiet
```

---

## awf init

Initialize AWF in the current directory.

```bash
awf init [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing configuration files |

### Examples

```bash
# Initialize a new project
awf init

# Reinitialize (recreate config and example workflow)
awf init --force
```

### Created Structure

```
.awf.yaml              # Configuration file
.awf/
├── workflows/
│   └── example.yaml   # Sample workflow
├── templates/         # Reusable workflow templates
└── storage/
    ├── states/        # State persistence
    └── logs/          # Log files
```

---

## awf run

Execute a workflow.

```bash
awf run <workflow> [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--input, -i` | Input parameter (key=value), can be repeated |
| `--output, -o` | Output mode: silent (default), streaming, buffered |
| `--step, -s` | Execute only a specific step from the workflow |
| `--mock, -m` | Mock state values for single step execution (key=value) |
| `--dry-run` | Show execution plan without running commands |
| `--interactive` | Enable step-by-step mode with prompts |
| `--breakpoint, -b` | Pause only at specific steps (requires --interactive) |

### Output Modes

| Mode | Description |
|------|-------------|
| `silent` | No command output displayed (default) |
| `streaming` | Real-time output with [OUT]/[ERR] prefixes |
| `buffered` | Show output after each step completes |

### Examples

```bash
# Basic execution
awf run deploy

# With inputs
awf run deploy --input env=prod --input version=1.2.3

# With streaming output
awf run deploy -o streaming

# Dry run to see execution plan
awf run deploy --dry-run

# Interactive step-by-step
awf run deploy --interactive

# Pause only at specific steps
awf run deploy --interactive --breakpoint build,deploy

# Execute single step with mocked dependencies
awf run deploy --step deploy_step --mock states.build.output="build-123"
```

### Interactive Mode Actions

When running with `--interactive`, you can choose at each step:

| Action | Key | Description |
|--------|-----|-------------|
| Continue | `c` | Execute the current step |
| Skip | `s` | Skip this step and continue |
| Abort | `a` | Stop workflow execution |
| Inspect | `i` | Show step details |
| Edit | `e` | Modify step parameters |
| Retry | `r` | Retry the last step |

---

## awf resume

Resume an interrupted workflow.

```bash
awf resume [workflow-id] [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--list, -l` | List resumable workflows |
| `--input, -i` | Override input parameter on resume (key=value) |
| `--output, -o` | Output mode: silent (default), streaming, buffered |

### Examples

```bash
# List all resumable (interrupted) workflows
awf resume --list

# Resume a specific workflow
awf resume abc123-def456

# Resume with input override
awf resume abc123-def456 --input max_tokens=5000
```

---

## awf list

List available workflows.

```bash
awf list [flags]
```

### Examples

```bash
# List all workflows
awf list

# JSON output
awf list -f json

# Table format
awf list -f table
```

---

## awf status

Show execution status of a workflow.

```bash
awf status <workflow-id> [flags]
```

### Examples

```bash
# Check status
awf status abc123-def456

# JSON output for scripting
awf status abc123-def456 -f json
```

---

## awf validate

Validate workflow syntax without executing.

```bash
awf validate <workflow> [flags]
```

### Validates

- YAML syntax
- State references (no missing states)
- Transition graph (no cycles, no unreachable states)
- Terminal states exist
- Template references valid
- Input definitions valid
- Parallel strategy valid

### Examples

```bash
# Validate a workflow
awf validate deploy

# Validate with verbose output
awf validate deploy -v
```

---

## awf history

Show workflow execution history.

```bash
awf history [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--workflow, -w` | Filter by workflow name |
| `--status, -s` | Filter by status (success, failed, interrupted) |
| `--since` | Show executions since date (YYYY-MM-DD) |
| `--limit, -n` | Maximum entries to show (default: 20) |
| `--stats` | Show statistics only |

### Examples

```bash
# List recent executions
awf history

# Filter by workflow
awf history --workflow deploy

# Filter by status
awf history --status failed

# Show executions since date
awf history --since 2025-12-01

# Show statistics only
awf history --stats

# JSON output for scripting
awf history -f json

# Combined filters
awf history -w deploy -s success --since 2025-12-01 -n 50
```

---

## awf version

Show version information.

```bash
awf version
```

---

## awf completion

Generate shell autocompletion scripts.

```bash
awf completion <shell>
```

### Supported Shells

- `bash`
- `zsh`
- `fish`
- `powershell`

### Examples

```bash
# Bash
awf completion bash > /etc/bash_completion.d/awf

# Zsh
awf completion zsh > "${fpath[1]}/_awf"

# Fish
awf completion fish > ~/.config/fish/completions/awf.fish

# PowerShell
awf completion powershell > awf.ps1
```
