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

### 1. Create a workflow

```yaml
# configs/workflows/hello.yaml
name: hello-world
version: "1.0.0"
description: A simple hello world workflow

states:
  initial: greet
  greet:
    type: step
    command: echo "Hello, {{inputs.name}}!"
    on_success: done
  done:
    type: terminal
```

### 2. Run the workflow

```bash
awf run hello --input name=World
```

Output:
```
Running workflow: hello-world
Workflow completed successfully in 2ms
Workflow ID: abc123-def456
```

## Commands

| Command | Description |
|---------|-------------|
| `awf run <workflow>` | Execute a workflow |
| `awf list` | List available workflows |
| `awf status <id>` | Show execution status |
| `awf validate <workflow>` | Validate workflow syntax |
| `awf version` | Show version info |

### Global Flags

```
--verbose, -v     Enable verbose output
--quiet, -q       Suppress non-error output
--no-color        Disable colored output
--config          Path to config file
--storage         Path to storage directory
--log-level       Log level (debug, info, warn, error)
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
      echo "Processing {{inputs.file_path}}"
    timeout: 30
    on_success: step2
    on_failure: error

  step2:
    type: step
    command: |
      claude -c "Analyze: {{states.step1.output}}"
    on_success: done
    on_failure: error

  done:
    type: terminal

  error:
    type: terminal
```

### State Types

| Type | Description |
|------|-------------|
| `step` | Execute a command |
| `terminal` | End state (success or failure) |
| `parallel` | Execute multiple steps concurrently (future) |

### Variable Interpolation

AWF uses `{{var}}` syntax (Go template style):

```yaml
# Inputs
{{inputs.variable_name}}

# Previous step outputs
{{states.step_name.output}}

# Workflow metadata
{{workflow.id}}
{{workflow.name}}

# Environment variables
{{env.VARIABLE_NAME}}
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
│   ├── infrastructure/          # Adapters: YAML, JSON, Shell
│   └── interfaces/cli/          # CLI commands and UI
├── configs/workflows/           # Workflow definitions
├── storage/                     # Runtime: states/, logs/
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

- Go 1.21+
- Make

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

### Phase 1 - MVP (v0.1.0) - In Progress
- [x] Hexagonal architecture
- [x] YAML workflow parsing
- [x] Linear step execution
- [x] JSON state persistence
- [x] CLI (run, list, status, validate)
- [x] JSON structured logging
- [ ] Variable interpolation
- [ ] Pre/post hooks

### Phase 2 - Core Features (v0.2.0)
- [ ] State machine with transitions
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
    command: cat "{{inputs.file}}"
    on_success: analyze
    on_failure: error

  analyze:
    type: step
    command: |
      claude -c "Review this code and suggest improvements:

      {{states.read_file.output}}"
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
