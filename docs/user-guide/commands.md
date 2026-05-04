---
title: "CLI Commands"
---

## Overview

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in current directory |
| `awf init --global` | Initialize global prompts and scripts directories |
| `awf run <workflow>` | Execute a local workflow |
| `awf run <pack/workflow>` | Execute a workflow from an installed pack |
| `awf resume [workflow-id]` | Resume an interrupted workflow |
| `awf list` | List available workflows |
| `awf list prompts` | List available prompt files |
| `awf status <id>` | Show execution status |
| `awf tui` | Open interactive terminal user interface |
| `awf validate <workflow>` | Validate workflow syntax |
| `awf diagram <workflow>` | Generate workflow diagram (DOT format) |
| `awf error [code]` | Look up error code documentation |
| `awf history` | Show workflow execution history |
| `awf plugin list` | List installed plugins |
| `awf plugin install <owner/repo>` | Install a plugin from GitHub releases |
| `awf plugin update [name]` | Update an installed plugin |
| `awf plugin remove <name>` | Remove an installed plugin |
| `awf plugin search [query]` | Search for plugins on GitHub |
| `awf plugin enable <name>` | Enable a plugin |
| `awf plugin disable <name>` | Disable a plugin |
| `awf workflow list` | List installed workflow packs |
| `awf workflow info <name>` | Display detailed pack information |
| `awf workflow install <owner/repo>` | Install a workflow pack from GitHub Releases |
| `awf workflow update [name]` | Update an installed workflow pack |
| `awf workflow remove <name>` | Remove an installed workflow pack |
| `awf workflow search [query]` | Search for workflow packs on GitHub |
| `awf config show` | Display project configuration |
| `awf upgrade` | Upgrade AWF to the latest version |
| `awf upgrade --check` | Check for available updates without installing |
| `awf upgrade --version <tag>` | Install a specific version |
| `awf version` | Show version info |
| `awf completion <shell>` | Generate shell autocompletion |

## Global Flags

These flags work with all commands:

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output (displays tool-use markers during agent execution) |
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

Initialize AWF in the current directory or global prompts and scripts directories.

```bash
awf init [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing configuration files |
| `--global` | Initialize global prompts and scripts directories at `$XDG_CONFIG_HOME/awf/` |

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
├── config.yaml        # Project configuration (inputs, telemetry, plugins)
├── workflows/
│   └── example.yaml   # Sample workflow
├── prompts/
│   └── example.md     # Example prompt file
├── scripts/
│   └── example.sh     # Example script file (executable)
├── templates/         # Reusable workflow templates
└── storage/
    ├── states/        # State persistence
    └── logs/          # Log files
```

See [Project Configuration](configuration.md) for details on `.awf/config.yaml`.

### Created Structure (Global)

```
$XDG_CONFIG_HOME/awf/
├── prompts/
│   └── example.md     # Example prompt file
└── scripts/
    └── example.sh     # Example script file (executable)
```

---

## awf run

Execute a workflow.

> **⚠️ Security Warning:** This command executes arbitrary shell commands on your system. Only run workflows from trusted sources. Use `--dry-run` or `--interactive` to audit execution plans from untrusted workflows.

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
| `--skip-plugins` | Skip plugin step type resolution (fails if workflow uses custom step types) |
| `--otel-exporter` | OTLP gRPC endpoint for distributed tracing (e.g., `localhost:4317`); overrides `telemetry.exporter` in `.awf/config.yaml`; omit to use project config or disable |
| `--otel-service-name` | OpenTelemetry service name resource attribute (default: `awf`); overrides `telemetry.service_name` in `.awf/config.yaml` |

### Output Modes

| Mode | Description |
|------|-------------|
| `silent` | No command output displayed (default) |
| `streaming` | Real-time output with [OUT]/[ERR] prefixes; for agent steps, displays human-readable text (or raw NDJSON if `output_format: json`) |
| `buffered` | Show output after each step completes; for agent steps, displays filtered text in post-execution summary (or raw NDJSON if `output_format: json`) |

**Note:** For agent steps, the `output_format` field controls display filtering: `text` or omitted (default) shows human-readable output; `json` shows raw NDJSON. See [Output Formatting](agent-steps.md#streaming-output-display) for details.

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

# Enable distributed tracing via CLI flags (or configure in .awf/config.yaml)
awf run deploy --otel-exporter=localhost:4317 --otel-service-name=my-app

# View workflow help with input parameters
awf run deploy --help
```

### Running Pack Workflows

Installed workflow packs provide namespaced workflows accessible via `pack/workflow` syntax:

```bash
# Run a workflow from an installed pack
awf run speckit/specify --input file=main.go

# Pack workflows resolve prompts and scripts from the pack's embedded files
# User overrides in .awf/prompts/<pack>/ and .awf/scripts/<pack>/ take precedence

# Dry-run a pack workflow
awf run speckit/specify --dry-run

# Local workflows (no namespace) are unaffected
awf run my-local-workflow
```

**Resolution order for pack resources:**
1. `.awf/prompts/<pack>/...` — user override (highest priority)
2. `.awf/workflow-packs/<pack>/prompts/...` — pack embedded
3. `~/.config/awf/prompts/...` — global XDG (lowest priority)

---

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

List available workflows, including pack workflows.

```bash
awf list [flags]
```

### Description

Displays all workflows from local (`.awf/workflows/`), global (`~/.config/awf/workflows/`), and installed pack directories. Pack workflows appear with `pack/workflow` namespace prefix and `pack` source label.

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `prompts` | List available prompt files |

### Examples

```bash
# List all workflows (local, global, and pack)
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

## awf tui

Open an interactive terminal user interface for browsing, executing, and monitoring workflows.

```bash
awf tui [flags]
```

### Description

Launches a full-screen Bubble Tea terminal UI with tab-based navigation providing:

- **Workflows Tab**: Filterable list of available workflows with launch and validation actions
- **Monitoring Tab**: Real-time execution tree visualization with step statuses, durations, and live log streaming
- **History Tab**: Browse past execution records with filtering by workflow name, status, and date range
- **Agent Conversations Tab**: View multi-turn agent interactions with Markdown-formatted responses
- **External Logs Tab**: Live tail of Claude Code JSONL session files for monitoring development sessions

The TUI bridges to existing CLI workflows while providing enhanced visibility into real-time execution and historical data without requiring raw command-line interaction.

### Keyboard Navigation

| Key | Action |
|-----|--------|
| `Tab` | Cycle through tabs (next) |
| `Shift+Tab` | Cycle through tabs (previous) |
| `q` / `Ctrl+C` | Quit the TUI (prompts for confirmation if workflow is running) |
| `↑` / `↓` | Navigate list items |
| `Enter` | Select/activate item, launch workflow, or open details |
| `f` | Filter list (Workflows/History tabs) |
| `v` | Validate selected workflow |
| Space | Toggle selection in multi-select lists |

### Features

#### Workflow Filtering

In the **Workflows** tab, use the search filter to quickly find workflows:

- Filter by workflow name or description
- Sub-second response time
- Match highlighting in results

#### Real-Time Monitoring

In the **Monitoring** tab while a workflow is running:

- Tree visualization with status icons (⏳ pending, ▶ running, ✓ success, ✗ failed)
- Auto-scrolling log viewport showing real-time step output
- Manual scroll-lock to pause auto-scroll
- Step duration tracking
- Auto-selection of failed steps in the tree for immediate error visibility

#### History Exploration

In the **History** tab:

- Chronologically sorted execution records from `history.db`
- Filter by workflow name, execution status (success/failed/cancelled), and date range
- Inspect past execution details including full tree state, timing breakdown, and step outputs

#### Agent Conversations

In the **Agent Conversations** tab:

- View multi-turn agent interactions during workflow execution
- Markdown-formatted responses with proper heading, code block, and list rendering
- Approval prompts (if applicable) with keyboard shortcuts

#### External Logs

In the **External Logs** tab:

- Live tailing of Claude Code JSONL session files
- Automatic detection of latest session file in `~/.claude/projects/...`
- Formatted message display with timestamps
- Graceful fallback if no session is active

### Examples

```bash
# Open the TUI
awf tui

# Launch from a script or CI/CD (non-interactive)
# Note: TUI requires a terminal; use `awf run` or `awf history` in non-interactive contexts
awf tui
```

### Environment

| Variable | Description |
|----------|-------------|
| `AWF_CONFIG` | Path to project configuration file (default: `.awf/config.yaml`) |
| `AWF_STORAGE` | Path to storage directory (default: `.awf/storage/`) |

### Notes

- **Terminal requirements**: Minimum 80x24 character dimensions recommended
- **Color support**: Automatically detects terminal color capabilities (true color, 256-color, basic ANSI)
- **Terminal state**: TUI restores terminal state cleanly on exit, panic recovery, or signal termination (SIGINT/SIGTERM)
- **Secret masking**: All sensitive values (variables starting with `SECRET_`, `API_KEY`, `PASSWORD`) are automatically masked in TUI views
- **Non-interactive contexts**: TUI requires a terminal with TTY support. Use `awf run`, `awf history`, and `awf status` commands for non-interactive automation

### See Also

- [Workflow Execution](workflow-syntax.md) - Workflow syntax and configuration
- [Commands](commands.md) - Full command reference
- [Interactive Inputs](interactive-inputs.md) - Terminal-based input prompting

---

## awf validate

Validate workflow syntax without executing.

```bash
awf validate <workflow> [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--skip-plugins` | Skip plugin validators |
| `--validator-timeout` | Per-plugin validation timeout (default `5s`, e.g., `10s`, `2m`) |

### Validates

- YAML syntax
- State references (no missing states)
- Transition graph (no cycles, no unreachable states)
- Terminal states exist
- Template references valid
- Input definitions valid
- Parallel strategy valid
- Plugin validators (custom rules from enabled validator plugins, skipped with `--skip-plugins`)

### Examples

```bash
# Validate a workflow
awf validate deploy

# Validate with verbose output
awf validate deploy -v

# Skip plugin validators
awf validate deploy --skip-plugins

# Custom validator timeout
awf validate deploy --validator-timeout 10s
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

Show workflow execution history. IDs and workflow names are displayed in full (no truncation), so IDs can be copied directly into commands like `awf status <id>`.

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
| `list` | List all plugins (use `--operations` to show provided operations) |
| `install <owner/repo>` | Install a plugin from GitHub releases |
| `update [name]` | Update an installed plugin to the latest version |
| `remove <name>` | Remove an installed plugin |
| `search [query]` | Search for available plugins on GitHub |
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
| `--operations` | List operations provided by each plugin |
| `--step-types` | List step types provided by each plugin |
| `--validators` | List validator plugins |
| `--details` | List all capabilities (operations, step types, validators) |

> Flags `--operations`, `--step-types`, `--validators`, and `--details` are mutually exclusive.

### Output Columns

| Column | Description |
|--------|-------------|
| Name | Plugin identifier |
| Type | `builtin` or `external` |
| Version | Semantic version |
| Status | `builtin`, `enabled`, `disabled`, or `error` |
| Source | GitHub `owner/repo` for installed plugins, `-` for built-in |
| Description | Brief plugin description |
| Capabilities | Plugin features: `operations`, `step_types`, `validators` |

### Examples

```bash
# List all plugins
awf plugin list

# JSON output for scripting
awf plugin list -f json

# Show operations provided by each plugin
awf plugin list --operations

# Show step types provided by plugins
awf plugin list --step-types

# Show validator plugins
awf plugin list --validators

# Show all capabilities in unified view
awf plugin list --details
```

---

## awf plugin install

Install a plugin from a GitHub repository.

```bash
awf plugin install <owner/repo> [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `owner/repo` | GitHub repository in `owner/repo` format (not a URL) |

### Flags

| Flag | Description |
|------|-------------|
| `--version` | Version constraint (e.g. `">=1.0.0 <2.0.0"`) |
| `--pre-release` | Include pre-release versions (alpha, beta, rc) |
| `--force` | Overwrite existing installation |

### Description

Downloads the latest compatible release from the GitHub repository, verifies the SHA-256 checksum, extracts the `.tar.gz` archive, validates the plugin manifest, and installs atomically. The plugin is enabled automatically after installation.

Release assets must follow the naming convention: `awf-plugin-<name>_<os>_<arch>.tar.gz` with a corresponding `checksums.txt` file.

### Examples

```bash
# Install a plugin
awf plugin install myorg/awf-plugin-jira

# Install with version constraint
awf plugin install myorg/awf-plugin-jira --version ">=1.0.0 <2.0.0"

# Include pre-release versions
awf plugin install myorg/awf-plugin-jira --pre-release

# Force reinstall over existing
awf plugin install myorg/awf-plugin-jira --force
```

### Errors

| Error | Cause |
|-------|-------|
| `invalid format: use owner/repo` | A URL was provided instead of `owner/repo` |
| `invalid format: expected owner/repo` | Missing owner or repo component |
| `already installed` | Plugin exists (use `--force` to overwrite) |
| `checksum mismatch` | Downloaded archive failed SHA-256 verification |
| `no compatible asset` | No release asset matches the current platform |

---

## awf plugin update

Update an installed plugin to the latest compatible version.

```bash
awf plugin update [plugin-name] [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `plugin-name` | Name of the plugin to update (optional with `--all`) |

### Flags

| Flag | Description |
|------|-------------|
| `--all` | Update all externally installed plugins |

### Description

Fetches the latest release from the plugin's source repository and performs an atomic replacement. Built-in plugins cannot be updated. Requires either a plugin name or the `--all` flag.

### Examples

```bash
# Update a specific plugin
awf plugin update jira

# Update all external plugins
awf plugin update --all
```

### Errors

| Error | Cause |
|-------|-------|
| `requires a plugin name or --all flag` | No plugin name and `--all` not specified |
| `plugin "<name>" is not installed` | Plugin name not found in installed plugins |

---

## awf plugin remove

Remove an installed plugin.

```bash
awf plugin remove <plugin-name> [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `plugin-name` | Name of the plugin to remove |

### Flags

| Flag | Description |
|------|-------------|
| `--keep-data` | Preserve plugin configuration and state |

### Description

Removes the plugin binary and manifest from the plugins directory. Plugin state is also cleared unless `--keep-data` is specified. Built-in plugins cannot be removed — use `awf plugin disable` instead.

### Examples

```bash
# Remove a plugin
awf plugin remove jira

# Remove but keep configuration
awf plugin remove jira --keep-data
```

### Errors

| Error | Cause |
|-------|-------|
| `plugin "<name>" is not installed` | Plugin name not found |
| `built-in provider cannot be removed` | Attempted to remove a built-in plugin (use `disable` instead) |

---

## awf plugin search

Search for available AWF plugins on GitHub.

```bash
awf plugin search [query] [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `query` | Optional keyword to filter results |

### Description

Discovers AWF plugins on GitHub by searching repositories tagged with the `awf-plugin` topic. Without a query, lists the full catalog. Results can be output as JSON for scripting.

### Examples

```bash
# List all available plugins
awf plugin search

# Search by keyword
awf plugin search jira

# JSON output for scripting
awf plugin search --output=json
```

---

## awf plugin enable

Enable a disabled plugin. Works with both built-in and external plugins.

```bash
awf plugin enable <name>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `name` | Plugin name to enable |

### Examples

```bash
# Enable an external plugin
awf plugin enable awf-plugin-github

# Enable a built-in plugin
awf plugin enable http
```

### Errors

| Error | Cause |
|-------|-------|
| `unknown plugin "<name>"` | Plugin name is not registered (typo or not installed) |

---

## awf plugin disable

Disable an enabled plugin. Works with both built-in and external plugins.

```bash
awf plugin disable <name>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `name` | Plugin name to disable |

### Examples

```bash
# Disable an external plugin
awf plugin disable awf-plugin-github

# Disable a built-in plugin
awf plugin disable http
```

### Errors

| Error | Cause |
|-------|-------|
| `unknown plugin "<name>"` | Plugin name is not registered (typo or not installed) |

---

## awf workflow

Manage workflow packs installed from GitHub Releases.

```bash
awf workflow <subcommand> [flags]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List installed workflow packs with version, source, and workflows |
| `info <name>` | Display detailed information about an installed pack |
| `install <owner/repo>` | Install a workflow pack from GitHub Releases |
| `update [name]` | Update an installed workflow pack to the latest version |
| `remove <name>` | Remove an installed workflow pack |
| `search [query]` | Search for available workflow packs on GitHub |

---

## awf workflow list

List all installed workflow packs with their version, source, and available workflows.

```bash
awf workflow list
```

### Aliases

`awf workflow ls`

### Description

Discovers packs from local (`.awf/workflow-packs/`) and global (`~/.local/share/awf/workflow-packs/`) directories, deduplicating by name (local takes precedence). Includes a `(local)` pseudo-entry when `.awf/workflows/` contains local workflow files.

### Examples

```bash
# List all installed packs
awf workflow list

# Using alias
awf wf ls
```

### Output

Each pack entry shows:
- **Name** — Pack identifier
- **Version** — Semantic version from manifest
- **Source** — GitHub `owner/repo` or `local`
- **Workflows** — Public workflow names defined in the pack

The `(local)` entry shows the count of workflow files in `.awf/workflows/`.

---

## awf workflow info

Display detailed information about an installed workflow pack.

```bash
awf workflow info <pack-name>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `pack-name` | Name of the installed workflow pack |

### Description

Shows manifest fields (name, version, description, author, license), workflow list, plugin install status with actionable install commands, and embedded README content. Searches local then global workflow-packs directories.

Plugin dependency warnings are emitted to stderr when the pack declares required plugins.

### Examples

```bash
# Show pack details
awf workflow info speckit

# Using workflow alias
awf wf info speckit
```

### Errors

| Error | Cause |
|-------|-------|
| `pack "<name>" not found` | Pack name not found in local or global directories |

---

## awf workflow update

Update an installed workflow pack to the latest version from GitHub Releases.

```bash
awf workflow update [pack-name] [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `pack-name` | Name of the pack to update (optional with `--all`) |

### Flags

| Flag | Description |
|------|-------------|
| `--all` | Update all installed workflow packs |

### Description

Loads the pack's `state.json` to find the source repository, fetches the latest release via `GitHubReleaseClient`, compares versions, and performs an atomic replacement using forced reinstall. User overrides in `.awf/prompts/<pack>/` and `.awf/scripts/<pack>/` are preserved (they live outside the pack directory).

After a successful update, `state.json` is written with the new version and an `updated_at` timestamp.

### Examples

```bash
# Update a specific pack
awf workflow update speckit

# Update all installed packs
awf workflow update --all
```

### Output

```
# When newer version exists
Updated speckit to version 1.3.0

# When already at latest
speckit is already at the latest version (1.2.0)

# When updating all
Updated 2 pack(s).

# When all are current
All packs are at their latest versions.

# When no packs installed
No installed workflow packs to update.
```

### Errors

| Error | Cause |
|-------|-------|
| `pack name required (or use --all)` | Neither pack name nor `--all` provided |
| `cannot specify a pack name with --all` | Both pack name and `--all` provided |
| `workflow pack "<name>" not found` | Pack not found in local or global directories |

---

## awf workflow install

Install a workflow pack from a GitHub repository.

```bash
awf workflow install <owner/repo> [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `owner/repo` | GitHub repository in `owner/repo` format (not a URL) |

### Flags

| Flag | Description |
|------|-------------|
| `--version` | Version constraint (e.g., `">=1.0.0 <2.0.0"` or `"1.2.0"` for exact version) |
| `--global` | Install to global user-level directory (`~/.local/share/awf/workflow-packs/`) instead of local project |
| `--force` | Overwrite existing installation |

### Description

Downloads the latest compatible release from the GitHub repository, verifies the SHA-256 checksum, extracts the `.tar.gz` archive, validates the `manifest.yaml`, checks AWF version compatibility, and installs atomically. The pack directory structure is created with source metadata.

Release assets must include a single `.tar.gz` archive (e.g., `awf-workflow-<name>_<version>.tar.gz`) with a corresponding `checksums.txt` file. Workflow packs are platform-independent — no OS/architecture suffix is needed.

**Pack manifest validation:**
- `name`: Must match `^[a-z][a-z0-9-]*$` (lowercase, hyphens)
- `version`: Must be valid semver
- `awf_version`: Version constraint must be satisfied by current AWF CLI version
- `workflows/`: All referenced workflow files must exist in the pack

**Installation locations:**
- **Local** (default): `.awf/workflow-packs/<name>/` (project-level)
- **Global** (`--global`): `~/.local/share/awf/workflow-packs/<name>/` (user-level, applies to all projects)

**Plugin dependencies:**
If the manifest declares required plugins via the `plugins:` field, warnings are emitted during installation (non-blocking). Install missing plugins separately with `awf plugin install`.

### Examples

```bash
# Install a workflow pack (latest version)
awf workflow install myorg/awf-workflow-speckit

# Install with specific version
awf workflow install myorg/awf-workflow-speckit --version "1.2.0"

# Install with version constraint
awf workflow install myorg/awf-workflow-speckit --version ">=1.0.0 <2.0.0"

# Install globally (available to all projects)
awf workflow install myorg/awf-workflow-speckit --global

# Force reinstall over existing
awf workflow install myorg/awf-workflow-speckit --force
```

### Errors

| Error | Cause |
|-------|-------|
| `invalid format: use owner/repo` | A URL was provided instead of `owner/repo` |
| `invalid format: expected owner/repo` | Missing owner or repo component |
| `already installed` | Pack exists (use `--force` to overwrite) |
| `checksum mismatch` | Downloaded archive failed SHA-256 verification |
| `no .tar.gz archive found` | Release has no `.tar.gz` asset |
| `manifest validation failed` | Pack manifest is invalid or missing required fields |
| `AWF version not compatible` | Current AWF CLI version does not satisfy the pack's `awf_version` constraint |

---

## awf workflow remove

Remove an installed workflow pack.

```bash
awf workflow remove <pack-name> [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `pack-name` | Name of the workflow pack to remove |

### Description

Removes the workflow pack directory and all its contents (workflows, prompts, scripts, state.json). Deletion is immediate — no confirmation prompt is required.

### Examples

```bash
# Remove a locally installed pack
awf workflow remove speckit

# Remove a globally installed pack
awf workflow remove speckit
```

The command automatically detects whether the pack is installed locally or globally and removes it from the appropriate location.

### Errors

| Error | Cause |
|-------|-------|
| `pack "<name>" is not installed` | Pack name not found in local or global directories |

---

## awf workflow search

Search for available AWF workflow packs on GitHub.

```bash
awf workflow search [query] [flags]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `query` | Optional keyword to filter results |

### Flags

| Flag | Description |
|------|-------------|
| `--output` | Output format (`json`) |

### Description

Discovers AWF workflow packs on GitHub by searching repositories tagged with the `awf-workflow` topic. Without a query, lists the full catalog sorted by stars. Results show the repository name, star count, and description.

### Examples

```bash
# List all available workflow packs
awf workflow search

# Search by keyword
awf workflow search speckit

# JSON output for scripting
awf workflow search --output=json
```

### Errors

| Error | Cause |
|-------|-------|
| `GitHub API rate limit exceeded` | Too many unauthenticated requests; set `GITHUB_TOKEN` for higher limits |

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
