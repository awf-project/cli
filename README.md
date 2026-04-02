# AI Workflow Framework - CLI

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License: EUPL-1.2](https://img.shields.io/badge/License-EUPL--1.2-blue.svg)](LICENSE)

A Go CLI tool for orchestrating AI agents (Claude, Gemini, Codex, OpenAI-Compatible APIs) through YAML-configured workflows with state machine execution.

## Features

- **State Machine Execution** - Define workflows as state machines with conditional transitions based on exit codes, command output, or custom expressions
- **Inline Error Handling** - Specify error messages and exit codes directly on steps without creating separate terminal states
- **Agent Steps** - Invoke AI agents via CLI tools (Claude, Codex, Gemini) or direct HTTP (OpenAI, Ollama, vLLM, Groq) with prompt templates, response parsing, and accurate token tracking
- **Output Formatting for Agent Steps** - Automatically strip markdown code fences and validate JSON output
- **External Prompt Files** - Load agent prompts from `.md` files with full template interpolation, helper functions, and local override support
- **External Script Files** - Load commands from external script files with shebang-based interpreter dispatch, template interpolation, path resolution, and local override support
- **Conversation Mode** - Multi-turn conversations with native session resume for CLI providers (`claude`, `codex`, `gemini`, `opencode`), automatic context window management for HTTP providers, mid-conversation context injection via `inject_context` field, and token tracking across all turns
- **OpenAI-Compatible Provider** - Use any Chat Completions API (OpenAI, Ollama, vLLM, Groq) with native HTTP integration, accurate token reporting, and no CLI tool required
- **Parallel Execution** - Run multiple steps concurrently with configurable strategies
- **Loop Constructs** - For-each and while loops with full context access
- **Sub-Workflows** - Invoke workflows from other workflows with input/output mapping
- **Retry with Backoff** - Automatic retry with exponential, linear, or constant backoff
- **Workflow Templates** - Reusable step patterns with parameters
- **Input Validation** - Type checking, patterns, enums, file validation
- **Dry-Run Mode** - Preview execution plan without running commands
- **Interactive Mode** - Step-by-step execution with prompts
- **Interactive Input Collection** - Automatically prompt for missing required inputs in terminal sessions
- **Structured Error Codes** - Hierarchical error taxonomy (`USER.INPUT.MISSING_FILE`) with `awf error` lookup command
- **Actionable Error Hints** - Context-aware suggestions ("Did you mean?") with fuzzy matching, suppressible via `--no-hints`
- **Audit Trail** - Structured JSONL audit log with paired start/end entries per execution, secret masking, configurable path, and atomic writes
- **Plugin System** - Extend AWF with custom operations, validators, and step types via gRPC plugins (HashiCorp go-plugin); validators run custom rules during `awf validate`, custom step types register new `type:` values for workflow steps; includes `sdk.Serve()` entry point for plugin authors, and install/update/remove from GitHub Releases with checksum verification
- **Workflow Packs** - Share reusable workflows and prompts via `awf workflow install owner/repo[@version]` from GitHub Releases with manifest validation, checksum verification, and atomic installation; execute with `awf run pack/workflow` namespace syntax; `{{.awf.prompts_dir}}` and `{{.awf.scripts_dir}}` resolve context-aware with 3-tier resolution (user override → pack embedded → global); `call_workflow` within packs resolves relative to pack root; `--global` flag for user-level installation; `awf workflow remove <pack>` for cleanup; source metadata tracking and plugin dependency warnings
- **Built-in GitHub Plugin** - Declarative GitHub operations (get_issue, create_pr, batch) with auth fallback and concurrent execution
- **Built-in HTTP Operation** - Declarative REST API calls (GET, POST, PUT, DELETE) with configurable timeout, response capture, and retryable status codes
- **Built-in Notification Plugin** - Workflow completion alerts via desktop and webhooks with configurable backends

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/awf-project/cli/main/scripts/install.sh | sh
```

Or via Go:

```bash
go install github.com/awf-project/cli/cmd/awf@latest
```

Or build from source:

```bash
git clone https://github.com/awf-project/cli.git
cd cli && make build && make install
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

inputs:
  - name: name
    type: string
    default: World

states:
  initial: greet
  greet:
    type: step
    command: echo "Hello, {{.inputs.name}}!"
    on_success: done
  done:
    type: terminal
    status: success
EOF

# Run with input
awf run hello --input name=World
```

See [Quick Start Guide](docs/getting-started/quickstart.md) for more.

## ⚠️ Security & Risk Disclaimer

AWF is a powerful orchestration tool that grants AI agents and workflows direct access to your system's shell. While this enables complex automation, it carries significant risks:

1. **Arbitrary Code Execution:** Workflows behave like shell scripts. **Never run a workflow definition (YAML) from an untrusted source** without auditing it first.
2. **AI Non-Determinism:** AI agents (LLMs) can produce incorrect, unexpected, or destructive output ("hallucinations"). A prompt that seems safe might generate a harmful command in a specific context.
3. **No Sandboxing:** By default, AWF executes commands with the same privileges as the user running the CLI.

**Best Practices for Safe Execution:**
* **Audit Workflows:** Always review the YAML and prompt files before execution.
* **Use Dry-Run:** Preview the execution plan using `awf run --dry-run`.
* **Interactive Mode:** Use `awf run --interactive` to approve commands step-by-step.
* **Isolate Execution:** Run risky workflows or those from external sources within a sandboxed environment (e.g., Docker, VM).

## Commands

| Command | Description |
|---------|-------------|
| `awf init` | Initialize AWF in current directory |
| `awf run <workflow>` | Execute a workflow |
| `awf validate <workflow>` | Validate workflow syntax |
| `awf diagram <workflow>` | Generate workflow diagram (DOT format) |
| `awf error [code]` | Lookup error codes with descriptions and resolutions |
| `awf list` | List available workflows |
| `awf list prompts` | List available prompt files |
| `awf resume` | Resume interrupted workflow |
| `awf history` | Show execution history |
| `awf status <id>` | Check workflow status |
| `awf config show` | Display project configuration |
| `awf plugin list` | List installed plugins |
| `awf plugin install <owner/repo>` | Install a plugin from GitHub Releases |
| `awf plugin update [name]` | Update an installed plugin |
| `awf plugin remove <name>` | Remove an installed plugin |
| `awf plugin search [query]` | Search for plugins on GitHub |
| `awf plugin enable <name>` | Enable a plugin |
| `awf plugin disable <name>` | Disable a plugin |
| `awf workflow list` | List installed workflow packs |
| `awf workflow info <name>` | Display detailed pack information |
| `awf workflow install <owner/repo>` | Install a workflow pack from GitHub Releases |
| `awf workflow update [name]` | Update an installed workflow pack |
| `awf workflow remove <pack>` | Remove an installed workflow pack |
| `awf workflow search [query]` | Search for workflow packs on GitHub |
| `awf version` | Show version information |
| `awf completion <shell>` | Generate shell autocompletion |

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
    on_failure: {message: "File not found: {{.inputs.file}}", status: 1}

  analyze:
    type: agent
    provider: claude
    prompt: |
      Review this code for bugs, security issues, and improvements:
      {{.states.read.Output}}
    output_format: json
    options:
      model: claude-sonnet-4-20250514
    timeout: 120
    on_success: report
    on_failure: {message: "Analysis failed: {{.states.analyze.Output}}"}

  report:
    type: step
    command: echo "Severity: {{.states.analyze.JSON.severity}} — {{.states.analyze.JSON.summary}}"
    on_success: done

  done:
    type: terminal
    status: success
```

```bash
awf run code-review --input file=main.go
```

See [Examples](docs/user-guide/examples.md) for more workflows.

## Documentation

📖 **[Browse the documentation site](https://awf-project.github.io/cli/)** — searchable, with dark mode.

Or read locally:

| Section | Description |
|---------|-------------|
| [Getting Started](docs/getting-started/) | Installation and first steps |
| [User Guide](docs/user-guide/) | Commands, workflow syntax, templates |
| [Reference](docs/reference/) | Exit codes, error codes, interpolation, validation |
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
make lint-arch      # Check architecture constraints
make test-coverage  # Generate coverage report
make quality        # Run all quality checks
make proto-gen      # Regenerate protobuf Go code
make docs           # Build documentation site
make docs-serve     # Serve documentation site locally
make docs-clean     # Clean documentation build artifacts
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
- Clients of [Vanoix](https://vanoix.com), [akawaka](https://akawaka.fr), [d11n Studio](https://d11n.studio) (author's company)

**Contact:** awf@alexandre.vanoix.com
