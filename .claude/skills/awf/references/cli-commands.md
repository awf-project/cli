# CLI Commands Reference

## Commands Overview

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in directory |
| `awf run <workflow>` | Execute a workflow |
| `awf resume [id]` | Resume interrupted workflow |
| `awf list` | List available workflows |
| `awf status <id>` | Show execution status |
| `awf validate <workflow>` | Validate syntax |
| `awf history` | Show execution history |
| `awf version` | Show version |
| `awf completion <shell>` | Generate autocompletion |

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Verbose output |
| `--quiet, -q` | Suppress non-error output |
| `--no-color` | Disable colors |
| `--format, -f` | Output format: text, json, table, quiet |
| `--config` | Config file path |
| `--storage` | Storage directory path |
| `--log-level` | Log level: debug, info, warn, error |

## awf init

Initialize AWF in current directory.

```bash
awf init [--force]
```

Creates:
```
.awf.yaml
.awf/
├── workflows/example.yaml
├── templates/
└── storage/
```

## awf run

Execute a workflow.

```bash
awf run <workflow> [flags]
```

| Flag | Description |
|------|-------------|
| `--input, -i` | Input (key=value), repeatable |
| `--output, -o` | Output mode: silent, streaming, buffered |
| `--step, -s` | Execute single step |
| `--mock, -m` | Mock state values (key=value) |
| `--dry-run` | Preview without executing |
| `--interactive` | Step-by-step mode |
| `--breakpoint, -b` | Pause at specific steps |

**Examples:**

```bash
# Basic
awf run deploy

# With inputs
awf run deploy -i env=prod -i version=1.2.3

# Streaming output
awf run deploy -o streaming

# Dry run
awf run deploy --dry-run

# Interactive
awf run deploy --interactive

# Breakpoints
awf run deploy --interactive -b build,deploy

# Single step with mock
awf run deploy -s deploy_step -m states.build.output="v1.2.3"
```

**Interactive Mode Actions:**

| Key | Action |
|-----|--------|
| `c` | Continue - execute step |
| `s` | Skip - skip step |
| `a` | Abort - stop workflow |
| `i` | Inspect - show details |
| `e` | Edit - modify parameters |
| `r` | Retry - retry last step |

## awf resume

Resume interrupted workflow.

```bash
awf resume [workflow-id] [flags]
```

| Flag | Description |
|------|-------------|
| `--list, -l` | List resumable workflows |
| `--input, -i` | Override input on resume |
| `--output, -o` | Output mode |

```bash
# List resumable
awf resume --list

# Resume specific
awf resume abc123-def456

# Resume with override
awf resume abc123 -i max_tokens=5000
```

## awf list

List available workflows.

```bash
awf list [-f json|table]
```

## awf status

Show execution status.

```bash
awf status <workflow-id> [-f json]
```

## awf validate

Validate workflow syntax.

```bash
awf validate <workflow> [-v]
```

**Validates:**
- YAML syntax
- State references
- Transition graph (cycles, unreachable)
- Terminal states
- Template references
- Input definitions
- Parallel strategies

## awf history

Show execution history.

```bash
awf history [flags]
```

| Flag | Description |
|------|-------------|
| `--workflow, -w` | Filter by workflow |
| `--status, -s` | Filter: success, failed, interrupted |
| `--since` | Since date (YYYY-MM-DD) |
| `--limit, -n` | Max entries (default: 20) |
| `--stats` | Statistics only |

```bash
# Recent
awf history

# Filter
awf history -w deploy -s failed --since 2025-12-01

# Stats
awf history --stats
```

## awf completion

Generate shell autocompletion.

```bash
# Bash
awf completion bash > /etc/bash_completion.d/awf

# Zsh
awf completion zsh > "${fpath[1]}/_awf"

# Fish
awf completion fish > ~/.config/fish/completions/awf.fish
```

## Output Formats

| Format | Use |
|--------|-----|
| `text` | Interactive terminal (default) |
| `json` | Scripting, CI/CD |
| `table` | Reports |
| `quiet` | Pipelines (IDs only) |

```bash
# JSON for scripting
awf list -f json
awf status abc123 -f json

# Quiet for pipes
WORKFLOW_ID=$(awf run deploy -f quiet)
```
