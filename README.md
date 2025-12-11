# AWF - AI Workflow CLI

A Go CLI tool for orchestrating AI agents (Claude, Gemini, Codex) through YAML-configured workflows with state machine execution.

## Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                            AWF CLI                                  │
│  awf run | awf resume | awf list | awf status | awf validate | awf history │
└─────────────────────────────────┬───────────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                    YAML Workflow                            │
│   States → Transitions → Commands → Outputs                 │
└─────────────────────────────┬───────────────────────────────┘
                              │
┌─────────────────────────────┴───────────────────────────────┐
│                  Execution Engine                           │
│   Shell Executor │ State Persistence │ Signal Handling      │
└─────────────────────────────────────────────────────────────┘
```

## Installation

### From source

```bash
git clone https://github.com/vanoix/awf.git
cd awf
make build
make install
```

### Quick install

```bash
go install github.com/vanoix/awf/cmd/awf@latest
```

## Quick Start

### 1. Initialize AWF

```bash
awf init
```

This creates:
```
.awf.yaml              # Configuration file
.awf/
├── workflows/
│   └── example.yaml   # Sample workflow
└── storage/
    ├── states/        # State persistence
    └── logs/          # Log files
```

### 2. Run the example workflow

```bash
awf run example
```

Output:
```
Hello from AWF!
Workflow completed successfully
```

### 3. Create your own workflow

```yaml
# .awf/workflows/hello.yaml
name: hello
version: "1.0.0"
description: A simple hello world workflow

states:
  initial: greet
  greet:
    type: step
    command: echo "Hello, {{.inputs.name}}!"
    on_success: done
  done:
    type: terminal
```

```bash
awf run hello --input name=World
```

## Commands

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

### Init Command Flags

```
--force    Overwrite existing configuration files
```

```bash
# Initialize a new project
awf init

# Reinitialize (recreate config and example workflow)
awf init --force
```

### Resume Command Flags

```
--list, -l        List resumable workflows
--input, -i       Override input parameter on resume (key=value), can be repeated
--output, -o      Output mode: silent (default), streaming, buffered
```

```bash
# List all resumable (interrupted) workflows
awf resume --list

# Resume a specific workflow
awf resume abc123-def456

# Resume with input override
awf resume abc123-def456 --input max_tokens=5000
```

### History Command Flags

```
--workflow, -w    Filter by workflow name
--status, -s      Filter by status (success, failed, interrupted)
--since           Show executions since date (YYYY-MM-DD)
--limit, -n       Maximum entries to show (default: 20)
--stats           Show statistics only
```

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

### Global Flags

```
--verbose, -v     Enable verbose output
--quiet, -q       Suppress non-error output
--no-color        Disable colored output
--format, -f      Output format (text, json, table, quiet) - default: text
--config          Path to config file
--storage         Path to storage directory
--log-level       Log level (debug, info, warn, error)
```

### Output Formats

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

### Run Command Flags

```
--input, -i       Input parameter (key=value), can be repeated
--output, -o      Output mode: silent (default), streaming, buffered
                  - silent: No command output displayed
                  - streaming: Real-time output with [OUT]/[ERR] prefixes
                  - buffered: Show output after each step completes
--step, -s        Execute only a specific step from the workflow
--mock, -m        Mock state values for single step execution (key=value)
                  Example: --mock states.step1.output="mocked value"
```

## Workflow Discovery

AWF discovers workflows from multiple locations (priority high to low):

1. `AWF_WORKFLOWS_PATH` environment variable
2. `./.awf/workflows/` (local project)
3. `$XDG_CONFIG_HOME/awf/workflows/` (global, default: `~/.config/awf/workflows/`)

Local workflows override global ones with the same name. Use `awf list` to see all workflows with their source.

### XDG Base Directory

AWF follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html):

```
~/.config/awf/              # Config ($XDG_CONFIG_HOME/awf)
└── workflows/              # Global workflows

~/.local/share/awf/         # Data ($XDG_DATA_HOME/awf)
├── states/                 # Execution states
└── logs/                   # Log files
```

## Workflow Syntax

### Basic Structure

```yaml
name: my-workflow
version: "1.0.0"
description: Workflow description

inputs:
  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".go", ".py", ".js"]
  - name: max_tokens
    type: integer
    default: 2000
    validation:
      min: 1
      max: 10000

states:
  initial: step1

  step1:
    type: step
    command: |
      echo "Processing {{.inputs.file_path}}"
    dir: "/tmp/workdir"  # optional working directory
    timeout: 30
    on_success: step2
    on_failure: error

  step2:
    type: step
    command: |
      claude -c "Analyze: {{.states.step1.output}}"
    on_success: done
    on_failure: error

  done:
    type: terminal
    status: success

  error:
    type: terminal
    status: failure
```

### State Types

| Type | Description |
|------|-------------|
| `step` | Execute a command |
| `terminal` | End state with `status: success` or `status: failure` |
| `parallel` | Execute multiple steps concurrently |

### Step Options

| Option | Description |
|--------|-------------|
| `command` | Shell command to execute |
| `dir` | Working directory (supports interpolation, e.g., `{{.inputs.path}}`) |
| `timeout` | Execution timeout in seconds |
| `on_success` | Next state on success (exit code 0) |
| `on_failure` | Next state on failure (exit code ≠ 0) |
| `continue_on_error` | Always follow `on_success` regardless of exit code |
| `retry` | Retry configuration (see below) |

### Retry Options

| Option | Description |
|--------|-------------|
| `max_attempts` | Maximum number of attempts (default: 1 = no retry) |
| `initial_delay` | Delay before first retry (e.g., `1s`, `500ms`) |
| `max_delay` | Maximum delay cap (e.g., `30s`) |
| `backoff` | Strategy: `constant`, `linear`, or `exponential` |
| `multiplier` | Multiplier for exponential backoff (default: 2) |
| `jitter` | Random jitter factor 0.0-1.0 (e.g., 0.1 = ±10%) |
| `retryable_exit_codes` | List of exit codes to retry (empty = retry all non-zero) |

**Backoff Strategies:**
- `constant`: Always wait `initial_delay`
- `linear`: Wait `initial_delay * attempt`
- `exponential`: Wait `initial_delay * multiplier^(attempt-1)`

**Example:**
```yaml
flaky_api_call:
  type: step
  command: curl -f https://api.example.com/data
  retry:
    max_attempts: 5
    initial_delay: 1s
    max_delay: 30s
    backoff: exponential
    multiplier: 2
    jitter: 0.1
    retryable_exit_codes: [1, 22]  # curl exit codes
  on_success: process_data
  on_failure: error
```

### Terminal Options

| Option | Description |
|--------|-------------|
| `status` | Terminal status: `success` or `failure` |

### Parallel Options

| Option | Description |
|--------|-------------|
| `steps` | List of steps to execute concurrently |
| `strategy` | `all_succeed` (default), `any_succeed`, or `best_effort` |
| `max_concurrent` | Maximum concurrent steps (default: unlimited) |
| `on_success` | Next state on success |
| `on_failure` | Next state on failure |

**Strategies:**
- `all_succeed`: All steps must succeed, cancel remaining on first failure
- `any_succeed`: Succeed if at least one step succeeds
- `best_effort`: Collect all results, never cancel early

**Example:**
```yaml
parallel_analysis:
  type: parallel
  strategy: all_succeed
  max_concurrent: 3
  steps:
    - name: lint
      command: golangci-lint run
    - name: test
      command: go test ./...
    - name: build
      command: go build ./cmd/...
  on_success: deploy
  on_failure: error
```

Access individual step outputs: `{{.states.parallel_analysis.steps.lint.output}}`

### Variable Interpolation

AWF uses `{{.var}}` syntax (Go template style with dot prefix):

```yaml
# Inputs
{{.inputs.variable_name}}

# Previous step outputs
{{.states.step_name.output}}

# Workflow metadata
{{.workflow.id}}
{{.workflow.name}}

# Environment variables
{{.env.VARIABLE_NAME}}
```

### Input Validation

Inputs can be validated at runtime using the `validation` block:

| Option | Type | Description |
|--------|------|-------------|
| `pattern` | string | Regex pattern the value must match |
| `enum` | []string | List of allowed values |
| `min` | int | Minimum value (integers only) |
| `max` | int | Maximum value (integers only) |
| `file_exists` | bool | File must exist on filesystem |
| `file_extension` | []string | File must have one of these extensions |

**Supported types:** `string`, `integer`, `boolean`

**Example:**
```yaml
inputs:
  - name: email
    type: string
    required: true
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"

  - name: env
    type: string
    default: staging
    validation:
      enum: [dev, staging, prod]

  - name: count
    type: integer
    default: 10
    validation:
      min: 1
      max: 100

  - name: config_file
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".yaml", ".yml", ".json"]
```

Validation errors are collected (not fail-fast) and reported together:
```
input validation failed: 2 errors:
  - inputs.email: does not match pattern
  - inputs.count: value 150 exceeds maximum 100
```

## Architecture

AWF follows Hexagonal/Clean Architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                        │
│      CLI (current)  │  API (future)  │  MQ (future)        │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   APPLICATION LAYER                         │
│   WorkflowService │ ExecutionService │ StateManager         │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                      DOMAIN LAYER                           │
│   Workflow │ Step │ ExecutionContext │ Ports (Interfaces)   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│   YAMLRepository │ JSONStateStore │ ShellExecutor          │
└─────────────────────────────────────────────────────────────┘
```

### Project Structure

```
awf/
├── cmd/awf/main.go              # CLI entry point
├── internal/
│   ├── domain/
│   │   ├── workflow/            # Entities: Workflow, Step, Context
│   │   └── ports/               # Interfaces: Repository, Store, Executor
│   ├── application/             # Services: WorkflowService, ExecutionService
│   ├── infrastructure/          # Adapters: YAML, JSON, Shell, XDG
│   └── interfaces/cli/          # CLI commands and UI
└── tests/                       # Integration tests
```

## Exit Codes

| Code | Type | Description |
|------|------|-------------|
| 0 | Success | Workflow completed |
| 1 | User | Bad input, missing file |
| 2 | Workflow | Invalid state reference |
| 3 | Execution | Command failed, timeout |
| 4 | System | IO error, permissions |

## Development

### Prerequisites

**Required:**
- Go 1.21+
- Make

**For development:**
- [golangci-lint](https://golangci-lint.run/welcome/install/) - Linter

```bash
# Arch Linux
paru -S golangci-lint

# macOS
brew install golangci-lint

# Other (via Go)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Build & Test

```bash
make build          # Build binary to ./bin/awf
make test           # Run all tests
make test-unit      # Unit tests only
make test-coverage  # Generate coverage report
make lint           # Run linter
make fmt            # Format code
```

### Dependencies

- `spf13/cobra` - CLI framework
- `gopkg.in/yaml.v3` - YAML parsing
- `fatih/color` - Terminal colors
- `google/uuid` - UUID generation
- `stretchr/testify` - Testing
- `golang.org/x/sync/errgroup` - Parallel execution
- `dgraph-io/badger/v4` - History storage

## Roadmap

### Phase 1 - MVP (v0.1.0) - Complete
- [x] Hexagonal architecture
- [x] YAML workflow parsing
- [x] Linear step execution
- [x] JSON state persistence
- [x] CLI (run, list, status, validate, init)
- [x] JSON structured logging
- [x] Variable interpolation
- [x] Pre/post hooks
- [x] Output streaming (--output flag)
- [x] XDG workflow discovery
- [x] Output formats (--format flag)
- [x] Step working directory (dir field)
- [x] CLI init command (--force flag)
- [x] Run single step (--step flag)

### Phase 2 - Core Features (v0.2.0)
- [x] State machine with transitions (cycle detection, unreachable states, terminal status)
- [x] Parallel execution (errgroup with strategies)
- [x] Retry with exponential backoff
- [x] Input validation
- [x] Template reference validation (detect undefined inputs, missing steps, forward references)
- [x] Resume command
- [x] BadgerDB history

### Phase 3+ - Advanced Features
- [ ] Conditions (if/else)
- [ ] Loops (for/while)
- [ ] Workflow templates
- [ ] Plugin system
- [ ] REST API
- [ ] Web UI

## Examples

### Code Analysis Workflow

```yaml
name: analyze-code
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true

states:
  initial: read_file

  read_file:
    type: step
    command: cat "{{.inputs.file}}"
    on_success: analyze
    on_failure: error

  analyze:
    type: step
    command: |
      claude -c "Review this code and suggest improvements:

      {{.states.read_file.output}}"
    timeout: 120
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
```

Run:
```bash
awf run analyze-code --input file=main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Write tests for your changes
4. Commit with conventional commits (`git commit -m 'feat: add amazing feature'`)
5. Push and open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.
