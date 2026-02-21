# Package Documentation Guide

AWF provides package-level documentation via `doc.go` files, enabling developers to discover implementation details, architecture patterns, and usage examples without reading raw source code.

## Discovering Package Documentation

### Using `go doc` Command

The fastest way to explore any AWF package:

```bash
# View domain layer documentation
go doc ./internal/domain/workflow
go doc ./internal/domain/ports

# View application layer documentation
go doc ./internal/application

# View infrastructure adapters
go doc ./internal/infrastructure/agents
go doc ./internal/infrastructure/plugin
go doc ./internal/infrastructure/executor
go doc ./internal/infrastructure/repository
go doc ./internal/infrastructure/logger
go doc ./internal/infrastructure/expression
go doc ./internal/infrastructure/store

# View CLI interfaces
go doc ./internal/interfaces/cli
go doc ./internal/interfaces/cli/ui

# View public packages
go doc ./pkg/interpolation
go doc ./pkg/validation
go doc ./pkg/retry
```

### Using `go doc` with Patterns

Search for specific types or functions:

```bash
# Find a specific type
go doc ./internal/domain/workflow Workflow
go doc ./internal/infrastructure/agents ProviderRegistry

# List all exported symbols in a package
go doc -all ./internal/application
```

### Viewing HTML Documentation

Generate interactive HTML documentation:

```bash
# Start local Go documentation server
godoc -http=:6060

# Then visit http://localhost:6060/pkg/github.com/awf-project/awf/internal/domain/workflow/
```

## Documentation Structure

Each `doc.go` file follows a consistent structure:

```go
// Package <name> provides <primary responsibility>.
//
// # Architecture Role
//
// <Layer and responsibility within hexagonal architecture>
//
// # Key Types
//
//   - TypeName - Brief description
//   - AnotherType - What it does
//
// # Usage Example
//
//   // Example code demonstrating typical usage
//
// # Port Implementations
//
// This package implements these domain ports:
//   - ports.PortName - Description
//
// See [parent_package] for more context.
package <name>
```

### Documentation Tiers

Documentation depth varies by package complexity:

#### Concise Style (20-30 lines)

Used for single-concern packages with straightforward purposes.

**Examples**: `executor`, `expression`, `store`

```go
// Package executor provides shell command execution.
//
// The ShellExecutor adapts the ports.CommandExecutor interface, enabling
// AWF to invoke arbitrary shell commands with environment context,
// working directory support, and process group management for graceful
// termination.
//
// Usage:
//
//	executor := executor.NewShellExecutor(logger)
//	result, err := executor.Execute(ctx, ports.Command{
//		Name:  "sh",
//		Args:  []string{"-c", "echo 'Hello'"},
//		Env:   []string{"KEY=value"},
//		Dir:   "/tmp",
//	})
//
// See [ports.CommandExecutor] for the interface definition.
package executor
```

#### Medium Style (40-50 lines)

Used for packages with 4-8 files or moderate complexity.

**Examples**: `logger`, `cli/ui`

```go
// Package ui provides output formatting and interactive prompts.
//
// # Architecture Role
//
// This package implements presentation layer concerns for the CLI interface:
// colored output, progress indicators, interactive step execution feedback,
// and dry-run visualization.
//
// # Key Types
//
//   - OutputWriter - Text and JSON output formatting
//   - CLIPrompt - Interactive step execution feedback
//   - DryRunFormatter - Workflow preview visualization
//
// Usage:
//
//	output := ui.NewOutputWriter(os.Stdout, "json")
//	output.Success("Workflow completed")
//
//	prompt := ui.NewCLIPrompt(stdio, logger)
//	action, err := prompt.PromptAction(ctx, step)
//
// See [../] for CLI command integration.
package ui
```

#### Comprehensive Style (60-80 lines)

Used for large packages with 8+ files or complex cross-concerns.

**Examples**: `agents`, `plugin`, `repository`, `cli`

```go
// Package agents provides AI agent provider integrations.
//
// # Architecture Role
//
// This package implements the application-level agent execution layer, coordinating
// Claude, Gemini, Codex, OpenCode, and OpenAI-compatible agent providers. It handles provider
// selection, prompt templating, response parsing, and integration with the
// ExecutionService for multi-turn conversations.
//
// # Key Types
//
// Providers (implement ports.AgentProvider):
//   - ClaudeProvider - Claude via claude CLI
//   - GeminiProvider - Gemini API
//   - CodexProvider - OpenAI Codex (legacy)
//   - OpenAICompatibleProvider - Any OpenAI-compatible API endpoint
//
// Execution:
//   - CLIExecutor - Invokes shell commands with context
//   - ProviderRegistry - Manages available providers
//   - AgentStep - Represents a single agent invocation
//
// # Usage Example
//
//	registry := agents.NewProviderRegistry(logger)
//	provider := registry.Get("claude")
//	response, err := provider.Execute(ctx, ports.AgentRequest{...})
//
// # Port Implementations
//
//   - ports.AgentProvider - Multi-turn conversation interface
//   - ports.CommandExecutor - Shell command execution
//
// # Design Principles
//
//   - Provider agnostic: Each provider is swappable via registry
//   - Context-aware: Supports conversation state across turns
//   - Error resilience: Graceful degradation on provider unavailability
//
// See [../application] for integration with ExecutionService.
package agents
```

## Content Guidelines

### What to Include

- **Purpose**: What the package does in 1-2 sentences
- **Architecture Role**: Where it fits in hexagonal layers
- **Key Types**: Exported types with brief descriptions
- **Port Implementations**: Which domain ports this package implements
- **Usage Example**: Minimal runnable code showing typical usage
- **Related Links**: Links to related packages or documentation

### What to Avoid

- Implementation details (private functions, internal algorithms)
- Test or example-only content
- Deprecated patterns or legacy code
- External dependency documentation (users should consult upstream docs)
- Comments better suited for code-level documentation

### Formatting Rules

- Use `# Section Headers` (Markdown style) for organization
- Indent code blocks with tabs (Go doc convention)
- Link to related packages using `[package.Type]` syntax
- Use `---` for section breaks when needed
- Keep lines under 80 characters for readability

## Integration with Development Workflow

### When Writing New Code

If you add a new package:

1. Create `doc.go` file in the package root
2. Start with concise style (20-30 lines)
3. Include at least one usage example
4. Run `go doc ./<package>` to verify output
5. Update parent package's `see also` reference if relevant

### Maintaining Existing Documentation

When modifying a package:

- Update `doc.go` if you change exported APIs
- Add new types to the "Key Types" section
- Update usage examples if patterns change
- Keep documentation in sync with code

### Code Review Checklist

For PRs affecting documented packages:

- [ ] `go doc ./...` produces valid output
- [ ] New exported types documented in `doc.go`
- [ ] Usage examples compile and are accurate
- [ ] No conflicting package comments in non-doc.go files

## Architecture Coverage

All key packages now have documentation:

### Domain Layer (4 packages)
- `internal/domain/workflow` - Workflow entities and validation
- `internal/domain/ports` - Port interfaces (adapters implement these)
- `internal/domain/operation` - Operation interface
- `internal/domain/errors` - Structured error types and codes

### Application Layer (1 package)
- `internal/application` - Execution engine and services

### Infrastructure Layer (11 packages)
- `internal/infrastructure/agents` - AI provider adapters
- `internal/infrastructure/executor` - Shell command execution
- `internal/infrastructure/expression` - Expression evaluation
- `internal/infrastructure/logger` - Logging adapters
- `internal/infrastructure/plugin` - Plugin system
- `internal/infrastructure/repository` - Workflow loading
- `internal/infrastructure/store` - State and history persistence
- `internal/infrastructure/config` - Configuration management
- `internal/infrastructure/errors` - Error formatting
- `internal/infrastructure/diagram` - DOT diagram generation

### Interface Layer (2 packages)
- `internal/interfaces/cli` - CLI commands and structure
- `internal/interfaces/cli/ui` - Output formatting and prompts

### Public Packages (3 packages)
- `pkg/interpolation` - Template variable substitution
- `pkg/validation` - Input validation rules
- `pkg/retry` - Backoff strategies

**Total: 21 documented packages covering 100% of public APIs.**

## See Also

- [Code Quality](code-quality.md) - Linting and formatting standards
- [Project Structure](../development/project-structure.md) - Codebase organization
- [Architecture](../development/architecture.md) - Hexagonal design principles
- [Go Documentation Best Practices](https://go.dev/blog/godoc)
