# AWF - AI Workflow CLI

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE)

A Go CLI tool for orchestrating AI agents (Claude, Gemini, Codex) through YAML-configured workflows with state machine execution.

## Features

- **State Machine Execution** - Define workflows as state machines with conditional transitions
- **Parallel Execution** - Run multiple steps concurrently with configurable strategies
- **Loop Constructs** - For-each and while loops with full context access
- **Sub-Workflows** - Invoke workflows from other workflows with input/output mapping
- **Retry with Backoff** - Automatic retry with exponential, linear, or constant backoff
- **Workflow Templates** - Reusable step patterns with parameters
- **Input Validation** - Type checking, patterns, enums, file validation
- **Dry-Run Mode** - Preview execution plan without running commands
- **Interactive Mode** - Step-by-step execution with prompts
- **Plugin System** - Extend AWF with custom operations via RPC-based plugins

## Installation

```bash
go install github.com/vanoix/awf/cmd/awf@latest
```

Or build from source:

```bash
git clone https://github.com/vanoix/awf.git
cd awf && make build && make install
```

See [Installation Guide](docs/getting-started/installation.md) for details.

## Quick Start

```bash
# Initialize AWF
awf init

# Run example workflow
awf run example

# Create your own workflow
cat > .awf/workflows/hello.yaml << 'EOF'
name: hello
version: "1.0.0"

states:
  initial: greet
  greet:
    type: step
    command: echo "Hello, {{.inputs.name}}!"
    on_success: done
  done:
    type: terminal
EOF

# Run with input
awf run hello --input name=World
```

See [Quick Start Guide](docs/getting-started/quickstart.md) for more.

## Commands

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in current directory |
| `awf run <workflow>` | Execute a workflow |
| `awf validate <workflow>` | Validate workflow syntax |
| `awf diagram <workflow>` | Generate workflow diagram (DOT format) |
| `awf list` | List available workflows |
| `awf resume` | Resume interrupted workflow |
| `awf history` | Show execution history |
| `awf status <id>` | Check workflow status |
| `awf plugin list` | List installed plugins |
| `awf plugin enable <name>` | Enable a plugin |
| `awf plugin disable <name>` | Disable a plugin |

See [Command Reference](docs/user-guide/commands.md) for all options.

## Example Workflow

```yaml
name: code-review
version: "1.0.0"

inputs:
  - name: file
    type: string
    required: true
    validation:
      file_exists: true

states:
  initial: read
  read:
    type: step
    command: cat "{{.inputs.file}}"
    on_success: analyze
    on_failure: error
  analyze:
    type: step
    command: claude -c "Review: {{.states.read.output}}"
    timeout: 120
    on_success: done
    on_failure: error
  done:
    type: terminal
  error:
    type: terminal
    status: failure
```

```bash
awf run code-review --input file=main.go
```

See [Examples](docs/user-guide/examples.md) for more workflows.

## Documentation

| Section | Description |
|---------|-------------|
| [Getting Started](docs/getting-started/) | Installation and first steps |
| [User Guide](docs/user-guide/) | Commands, workflow syntax, templates |
| [Reference](docs/reference/) | Exit codes, interpolation, validation |
| [Development](docs/development/) | Architecture, contributing, testing |

## Architecture

AWF follows hexagonal (clean) architecture:

```
Interfaces (CLI) → Application (Services) → Domain (Entities, Ports)
                                                    ↑
                        Infrastructure (YAML, JSON, Shell) ─┘
```

See [Architecture](docs/development/architecture.md) for details.

## Development

```bash
make build          # Build binary
make test           # Run all tests
make lint           # Run linter
make test-coverage  # Generate coverage report
```

See [Testing Guide](docs/development/testing.md) for conventions.

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

1. Fork the repository
2. Create a feature branch
3. Write tests for your changes
4. Submit a Pull Request

## Support the Project

AWF is maintained by [Alexandre Balmes](https://github.com/pocky) and relies on community support to ensure continued development, maintenance, and reliability.

### Why Sponsor?

Your support helps:
- **Maintenance**: Bug fixes, security updates, dependency upgrades
- **Improvements**: New features, performance optimizations
- **Quality**: Documentation, testing, code reviews
- **Reliability**: CI/CD, release management, long-term support

### How to Support

**Direct sponsorship:**
- [GitHub Sponsors](https://github.com/sponsors/pocky)

**Commercial engagement:**

Hiring services from the author's companies directly funds AWF development:
- [Vanoix](https://vanoix.com) - Tech Consulting
- [d11n Studio](https://d11n.studio) - AI Solutions
- [akawaka](https://akawaka.fr) - Web Development

All commercial clients receive a free commercial license for AWF.

## License

### Open Source License

This project is licensed under the [EUPL-1.2](LICENSE) (European Union Public License).

- **Individuals**: Free to use, modify, and distribute
- **Companies**: Free to use under EUPL-1.2 terms (copyleft - modifications must be shared)
- **GPL Compatibility**: EUPL-1.2 is [compatible with GPL](https://joinup.ec.europa.eu/collection/eupl/matrix-eupl-compatible-open-source-licences)

### Commercial License

Companies seeking exemption from copyleft obligations can obtain a commercial license.

**Free commercial license for:**
- [Project sponsors](https://github.com/sponsors/pocky)
- Active contributors
- Clients of [Vanoix](https://vanoix.com) (author's company)

**Contact:** alexandre@vanoix.com
