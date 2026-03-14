# AWF Documentation

Welcome to the AWF (AI Workflow CLI) documentation.

> **📖 Online documentation:** These docs are also available at **[awf-project.github.io/cli](https://awf-project.github.io/cli/)** with full-text search and dark mode support.

> **⚠️ Security Warning:** AWF executes shell commands and interacts with AI agents. Agents can "hallucinate" and produce destructive commands. Never run untrusted workflows and always review them using `--dry-run` or `--interactive` mode. See [Security Policy](../.github/SECURITY.md) for more.

## Quick Navigation

| Section | Description |
|---------|-------------|
| [Getting Started](getting-started/) | Installation and first steps |
| [User Guide](user-guide/) | Commands, workflow syntax, templates |
| [Reference](reference/) | Exit codes, error codes, interpolation, validation, loops |
| [Development](development/) | Architecture, contributing, testing |

## Getting Started

New to AWF? Start here:

1. [Installation](getting-started/installation.md) - Install AWF on your system
2. [Quick Start](getting-started/quickstart.md) - Create and run your first workflow

## User Guide

Learn how to use AWF effectively:

- [Commands](user-guide/commands.md) - All CLI commands and flags
- [Interactive Input Collection](user-guide/interactive-inputs.md) - Automatic prompting for missing workflow inputs
- [Agent Steps](user-guide/agent-steps.md) - Invoke AI agents via CLI (Claude, Codex, Gemini) or HTTP APIs (OpenAI, Ollama, vLLM, Groq)
  - [Output Formatting](user-guide/agent-steps.md#output-formatting) - Automatic code fence stripping and JSON validation (`output_format: json|text`)
  - [External Prompt Files](user-guide/agent-steps.md#external-prompt-files) - Load prompts from Markdown files with template interpolation
- [Conversation Mode](user-guide/conversation-steps.md) - Multi-turn conversations with context window management
- [Configuration](user-guide/configuration.md) - Project configuration file
- [Workflow Syntax](user-guide/workflow-syntax.md) - YAML workflow definition reference
  - [Inline Error Shorthand](user-guide/workflow-syntax.md#inline-error-shorthand) - Specify error messages directly on steps without separate terminal states
  - [External Script Files](user-guide/workflow-syntax.md#external-script-files) - Load shell commands from external `.sh` files with template interpolation and shebang-based interpreter dispatch
  - [Shebang Support](user-guide/workflow-syntax.md#shebang-support) - Execute scripts via their declared interpreter (Python, Perl, bash, etc.)
  - [GitHub Operations](user-guide/workflow-syntax.md#github-operations) - Built-in GitHub plugin with declarative operations
  - [HTTP Operations](user-guide/workflow-syntax.md#http-operations) - Built-in HTTP operation for REST API calls
  - [Notification Operations](user-guide/workflow-syntax.md#notification-operations) - Built-in notification plugin with desktop and webhook backends
- [Templates](user-guide/templates.md) - Reusable workflow templates
- [Plugins](user-guide/plugins.md) - Extend AWF with custom operations
- [Audit Trail](user-guide/audit-trail.md) - Structured execution audit log with JSONL output
- [Examples](user-guide/examples.md) - Real-world workflow examples

## Reference

Technical reference documentation:

- [Exit Codes](reference/exit-codes.md) - Error codes and their meanings
- [Error Codes](reference/error-codes.md) - Structured error taxonomy and lookup guide
- [Variable Interpolation](reference/interpolation.md) - Template variables and syntax
- [Input Validation](reference/validation.md) - Validation rules for workflow inputs
- [Loop Reference](reference/loop.md) - Loop control flow and transitions
- [Audit Trail Schema](reference/audit-trail-schema.md) - JSONL entry format, fields, and constraints
- [Package Documentation](reference/package-documentation.md) - Discovering code documentation with `go doc`

## Development

For contributors and developers:

- [Architecture](development/architecture.md) - Hexagonal architecture overview
- [Project Structure](development/project-structure.md) - Codebase organization
- [Code Quality](development/code-quality.md) - Linters, formatters, and quality tooling
- [Testing](development/testing.md) - Testing conventions and commands

## Building Documentation Locally

To build and serve the documentation site locally:

```bash
make docs        # Build the site with minification
make docs-serve  # Serve at http://localhost:1313 with live reload
make docs-clean  # Clean build artifacts
```

The documentation site is built with [Hugo](https://gohugo.io) and the [Doks](https://getdoks.org) theme. Documentation source files in this directory are mounted directly into the site via Hugo module configuration — changes are reflected immediately on rebuild without requiring file copies.

**Requirements:** Hugo 0.147+ with extended edition (includes CSS/JS transpiling)

## Additional Resources

- [CHANGELOG](../CHANGELOG.md) - Version history
- [CONTRIBUTING](../CONTRIBUTING.md) - How to contribute
- [LICENSE](../LICENSE) - EUPL-1.2 License
