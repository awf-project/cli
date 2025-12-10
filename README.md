# AWF - AI Workflow CLI

A Go CLI tool for orchestrating AI agents (Claude, Gemini, Codex) through YAML-configured workflows with state machine execution.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        AWF CLI                              │
│   awf run | awf list | awf status | awf validate           │
└─────────────────────────────┬───────────────────────────────┘
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
| `awf list` | List available workflows |
| `awf status <id>` | Show execution status |
| `awf validate <workflow>` | Validate workflow syntax |
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
  - name: max_tokens
    type: integer
    default: 2000

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
| `parallel` | Execute multiple steps concurrently (future) |

### Step Options

| Option | Description |
|--------|-------------|
| `command` | Shell command to execute |
| `dir` | Working directory (supports interpolation, e.g., `{{.inputs.path}}`) |
| `timeout` | Execution timeout in seconds |
| `on_success` | Next state on success (exit code 0) |
| `on_failure` | Next state on failure (exit code ≠ 0) |
| `continue_on_error` | Always follow `on_success` regardless of exit code |

### Terminal Options

| Option | Description |
|--------|-------------|
| `status` | Terminal status: `success` or `failure` |

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
- GCC (for CGO/SQLite)

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
- [ ] Parallel execution
- [ ] Retry with exponential backoff
- [ ] Input validation
- [ ] Resume command
- [ ] SQLite history

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
