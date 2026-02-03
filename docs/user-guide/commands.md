# CLI Commands

## Overview

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in current directory |
| `awf init --global` | Initialize global prompts directory |
| `awf run <workflow>` | Execute a workflow |
| `awf resume [workflow-id]` | Resume an interrupted workflow |
| `awf list` | List available workflows |
| `awf list prompts` | List available prompt files |
| `awf status <id>` | Show execution status |
| `awf validate <workflow>` | Validate workflow syntax |
| `awf diagram <workflow>` | Generate workflow diagram (DOT format) |
| `awf error [code]` | Look up error code documentation |
| `awf history` | Show workflow execution history |
| `awf plugin list` | List installed plugins |
| `awf plugin enable <name>` | Enable a plugin |
| `awf plugin disable <name>` | Disable a plugin |
| `awf config show` | Display project configuration |
| `awf version` | Show version info |
| `awf completion <shell>` | Generate shell autocompletion |

## Global Flags

These flags work with all commands:

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output |
| `--quiet, -q` | Suppress non-error output |
| `--no-color` | Disable colored output |
| `--no-hints` | Disable error hint suggestions |
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

Initialize AWF in the current directory or global prompts directory.

```bash
awf init [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing configuration files |
| `--global` | Initialize global prompts directory at `$XDG_CONFIG_HOME/awf/prompts/` |

### Examples

```bash
# Initialize a new project (local)
awf init

# Reinitialize (recreate config and example workflow)
awf init --force

# Initialize global prompts directory
awf init --global
```

### Created Structure (Local)

```
.awf.yaml              # Configuration file
.awf/
├── config.yaml        # Project configuration (inputs pre-population)
├── workflows/
│   └── example.yaml   # Sample workflow
├── prompts/
│   └── example.md     # Example prompt file
├── templates/         # Reusable workflow templates
└── storage/
    ├── states/        # State persistence
    └── logs/          # Log files
```

See [Project Configuration](configuration.md) for details on `.awf/config.yaml`.

### Created Structure (Global)

```
$XDG_CONFIG_HOME/awf/
└── prompts/
    └── example.md     # Example prompt file
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
| `--help, -h` | Show workflow-specific help with input parameters |
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

# Interactive input collection (prompted for missing required inputs)
awf run deploy
# Prompts: Enter value for 'env' (string, required): _

# With streaming output
awf run deploy -o streaming

# Dry run to see execution plan
awf run deploy --dry-run

# Interactive step-by-step
awf run deploy --interactive

# Pause only at specific steps
awf run deploy --interactive --breakpoint build,deploy

# Execute single step with mocked dependencies
awf run deploy --step deploy_step --mock states.build.Output="build-123"

# View workflow help with input parameters
awf run deploy --help
```

### Interactive Input Collection

When required workflow inputs are missing and stdin is connected to a terminal, AWF automatically prompts for missing values:

**Behavior:**
- **Terminal environment**: Prompts interactively for each missing required input
- **Non-terminal environment** (pipes, scripts): Returns error with message to use `--input` flags
- **All inputs provided**: Executes immediately without prompts

**Prompt display:**
- Input name, type, and required/optional status
- Description and help text from workflow definition
- Enum options as numbered list (1-9) for constrained inputs
- Default value shown for optional inputs

**Validation:**
- Type checking (string, integer, boolean)
- Enum constraint validation
- Pattern matching (regex)
- Immediate error feedback with retry on invalid input
- Empty input accepted for optional parameters (uses default)

See [Interactive Input Collection Guide](interactive-inputs.md) for detailed examples.

### Workflow-Specific Help

View input parameters and details for a specific workflow before running it:

```bash
awf run <workflow> --help
```

This displays:
- Workflow description (if defined)
- All input parameters with their types (string, integer, boolean)
- Required/optional status for each input
- Default values for optional inputs
- Input descriptions

#### Example Output

```
Execute a workflow by name with optional input parameters.

Description: Deploy application to specified environment

Input Parameters:
  NAME          TYPE      REQUIRED    DEFAULT       DESCRIPTION
  env           string    yes         -             Target environment (dev, staging, prod)
  version       string    yes         -             Version tag to deploy
  dry_run       boolean   no          false         Run deployment checks without applying
  timeout       integer   no          300           Deployment timeout in seconds

Usage:
  awf run deploy [flags]

Flags:
  -h, --help              help for run
  -i, --input strings     Input parameter (key=value)
  ...
```

#### Help for Non-Existent Workflow

```bash
awf run unknown-workflow --help
# Error: workflow "unknown-workflow" not found
# exit code 1
```

---

### Interactive Input Collection

When you run a workflow with missing required inputs **from a terminal**, AWF automatically prompts you for each missing value instead of failing immediately.

#### How It Works

1. **Detection**: AWF detects missing required inputs not provided via `--input` flags
2. **Terminal Check**: If stdin is connected to a terminal, AWF enters interactive mode
3. **Prompting**: You are prompted for each missing required input in order
4. **Validation**: Invalid input values are rejected with error messages; you can retry
5. **Execution**: Once all required inputs are provided, the workflow executes

#### Examples

**Without Interactive Input Collection (Non-Interactive Context):**

```bash
# In a script or piped context:
awf run deploy < /dev/null
# Error: required input "env" not provided
# exit code 1
```

**With Interactive Input Collection (Terminal):**

```bash
# From your terminal:
awf run deploy

# Output:
# env (string, required):
# > prod
#
# version (string, required):
# > 1.2.3
#
# Workflow started...
```

#### Enum Constraints

When an input has enum constraints, AWF displays numbered options:

```bash
awf run deploy

# Output:
# env (string, required):
# Available options:
#   1) dev
#   2) staging
#   3) prod
# Select option (1-3):
# > 2
#
# Workflow started...
```

#### Optional Inputs

Optional inputs can be skipped by pressing Enter without providing a value:

```bash
awf run deploy

# Output:
# env (string, required):
# > prod
#
# timeout (integer, optional, default: 300):
# >
#
# Using default value for timeout: 300
# Workflow started...
```

#### When Interactive Mode Is NOT Available

Interactive input collection requires:
- ✅ Running in a terminal (stdin connected to TTY)
- ✅ Workflow has required inputs
- ❌ **Not available** in scripts (`< file`, pipes `|`)
- ❌ **Not available** in CI/CD pipelines
- ❌ **Not available** with `--input` providing all required values

In non-interactive contexts, you must provide all required inputs via `--input` flags:

```bash
# Provide all required inputs explicitly
awf run deploy --input env=prod --input version=1.2.3

# Or in a script
awf run deploy --input env=prod --input version=1.2.3 < /dev/null
```

---

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

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `prompts` | List available prompt files |

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

## awf list prompts

List available prompt files from local and global directories.

```bash
awf list prompts [flags]
```

### Description

Displays all prompt files discovered from:
1. `.awf/prompts/` (local project)
2. `$XDG_CONFIG_HOME/awf/prompts/` (global, default: `~/.config/awf/prompts/`)

Local prompts override global prompts with the same name. The output shows the source (local/global) for each prompt.

### Examples

```bash
# List all prompts
awf list prompts

# JSON output
awf list prompts -f json

# Table format
awf list prompts -f table
```

### Output Fields

| Field | Description |
|-------|-------------|
| Name | Relative path from prompts directory |
| Source | Origin: `local` or `global` |
| Path | Absolute file path |
| Size | File size in bytes |
| ModTime | Last modification time |

### Using Prompts

Reference prompts in workflow inputs using the `@prompts/` prefix:

```bash
awf run my-workflow --input prompt=@prompts/system.md
awf run ai-task --input context=@prompts/ai/agents/analyzer.md
```

The `@prompts/` prefix searches local directory first, then global.

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

## awf diagram

Generate a visual diagram of a workflow in DOT format (Graphviz).

```bash
awf diagram <workflow> [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--output, -o` | Output file path (format detected from extension: .dot, .png, .svg, .pdf) |
| `--direction` | Graph layout direction: TB (top-bottom, default), LR (left-right), BT, RL |
| `--highlight` | Step name to visually emphasize |

### Output Formats

| Extension | Description | Requires |
|-----------|-------------|----------|
| `.dot` | DOT source format | Nothing |
| `.png` | PNG image | Graphviz |
| `.svg` | SVG vector image | Graphviz |
| `.pdf` | PDF document | Graphviz |

Without `--output`, DOT format is printed to stdout.

### Step Shapes

Each step type renders with a distinct shape:

| Step Type | Shape | Description |
|-----------|-------|-------------|
| `command` | Box | Standard command execution |
| `parallel` | Diamond | Parallel branch point |
| `terminal` | Oval | Workflow end (doubleoval for failure) |
| `for_each` | Hexagon | Loop iteration |
| `while` | Hexagon | Conditional loop |
| `operation` | Box3D | Plugin operation |
| `call_workflow` | Folder | Sub-workflow invocation |

### Edge Styles

| Transition | Style |
|------------|-------|
| `on_success` | Solid line |
| `on_failure` | Dashed red line |

### Examples

```bash
# Output DOT to stdout
awf diagram deploy

# Pipe to graphviz
awf diagram deploy | dot -Tpng -o workflow.png

# Direct image export (requires graphviz)
awf diagram deploy --output workflow.png

# Left-to-right layout
awf diagram deploy --direction LR

# Highlight a specific step
awf diagram deploy --highlight build_step

# Save DOT file
awf diagram deploy --output workflow.dot

# Combined flags
awf diagram deploy -o diagram.svg --direction LR --highlight deploy
```

### Graphviz Installation

For image export (PNG, SVG, PDF), install Graphviz:

```bash
# macOS
brew install graphviz

# Ubuntu/Debian
apt install graphviz

# Fedora
dnf install graphviz
```

DOT format output works without Graphviz.

---

## awf error

Look up error code documentation and display detailed information.

```bash
awf error [code] [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `code` | Error code or prefix to look up (optional) |

### Description

Without arguments, lists all available error codes with descriptions and resolutions.
With a code argument, displays detailed information for that specific error code.
Supports prefix matching to show all codes in a category.

Error codes follow a three-level hierarchy: `CATEGORY.SUBCATEGORY.SPECIFIC`

### Examples

```bash
# List all error codes
awf error

# Look up a specific error code
awf error USER.INPUT.MISSING_FILE

# Prefix match - show all workflow validation errors
awf error WORKFLOW.VALIDATION

# JSON output for programmatic use
awf error EXECUTION.COMMAND.FAILED -f json
```

### Output Fields

| Field | Description |
|-------|-------------|
| Code | Error code identifier (e.g., `USER.INPUT.MISSING_FILE`) |
| Description | What the error means |
| Resolution | How to fix the error |
| Related Codes | Other related error codes |

### See Also

- [Error Codes Reference](../reference/error-codes.md) - Complete error code taxonomy
- [Exit Codes](../reference/exit-codes.md) - Exit code categories

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
| `--status, -s` | Filter by status (success, failed, cancelled) |
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

## awf plugin

Manage AWF plugins.

```bash
awf plugin <subcommand> [flags]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List all discovered plugins |
| `enable <name>` | Enable a disabled plugin |
| `disable <name>` | Disable an enabled plugin |

---

## awf plugin list

List all installed plugins with their status.

```bash
awf plugin list [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `-f, --format` | Output format (text, json) |

### Output Columns

| Column | Description |
|--------|-------------|
| Name | Plugin identifier |
| Version | Semantic version |
| Status | `enabled`, `disabled`, or `error` |
| Description | Brief plugin description |
| Capabilities | Plugin features: `operations`, `commands`, `validators` |

### Examples

```bash
# List all plugins
awf plugin list

# JSON output for scripting
awf plugin list -f json
```

---

## awf plugin enable

Enable a disabled plugin.

```bash
awf plugin enable <name>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `name` | Plugin name to enable |

### Examples

```bash
# Enable a plugin
awf plugin enable awf-plugin-slack
```

---

## awf plugin disable

Disable an enabled plugin.

```bash
awf plugin disable <name>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `name` | Plugin name to disable |

### Examples

```bash
# Disable a plugin
awf plugin disable awf-plugin-slack
```

---

## awf config show

Display project configuration values from `.awf/config.yaml`.

```bash
awf config show [flags]
```

### Description

Shows all configured input values from the project configuration file. If no configuration file exists, displays a message suggesting to run `awf init`.

### Flags

| Flag | Description |
|------|-------------|
| `-f, --format` | Output format: `text` (default), `json`, `quiet` |

### Output Formats

| Format | Description |
|--------|-------------|
| `text` | Human-readable table with keys and values |
| `json` | Structured JSON with path, exists flag, and inputs |
| `quiet` | Keys only, one per line (sorted alphabetically) |

### Examples

```bash
# Display configuration in default format
awf config show

# JSON output for scripting
awf config show --format json

# List just the configured input keys
awf config show --format quiet
```

### Example Output

**Text format:**
```
Project Configuration (.awf/config.yaml)

  project: my-project
  env: staging
  count: 42
```

**JSON format:**
```json
{
  "path": ".awf/config.yaml",
  "exists": true,
  "inputs": {
    "project": "my-project",
    "env": "staging",
    "count": 42
  }
}
```

### See Also

- [Project Configuration](configuration.md) - Configuration file reference

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
