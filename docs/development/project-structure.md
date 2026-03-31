---
title: "Project Structure"
---

AWF follows a standard Go project layout aligned with hexagonal architecture.

## Directory Tree

```
awf/
├── .github/
│   └── workflows/
│       ├── ci.yaml              # Build, lint, test pipeline
│       ├── hugo.yml             # Documentation site build & deploy
│       └── quality.yml          # Architecture lint enforcement
│
├── cmd/
│   └── awf/
│       └── main.go              # CLI entry point
│
├── internal/                    # Private application code
│   ├── domain/                  # Core business logic
│   │   ├── workflow/            # Workflow entities
│   │   │   ├── workflow.go      # Workflow struct
│   │   │   ├── state.go         # State types
│   │   │   ├── step.go          # Step execution
│   │   │   ├── context.go       # Execution context
│   │   │   ├── hooks.go         # Pre/post hooks
│   │   │   ├── template.go      # Workflow templates
│   │   │   └── validation.go    # Input validation
│   │   ├── operation/           # Operation interface and registry (F057)
│   │   │   ├── doc.go           # Package documentation
│   │   │   ├── operation.go     # Operation interface and sentinel errors
│   │   │   ├── validate.go      # ValidateInputs function
│   │   │   └── registry.go      # OperationRegistry (implements ports.OperationProvider)
│   │   └── ports/               # Port interfaces
│   │       ├── repository.go    # Workflow repository
│   │       ├── store.go         # State store
│   │       ├── executor.go      # Command executor
│   │       └── logger.go        # Logger interface
│   │
│   ├── application/             # Application services
│   │   ├── workflow_service.go  # Workflow loading/validation
│   │   ├── execution_service.go # Workflow execution
│   │   ├── state_manager.go     # State persistence
│   │   └── template_service.go  # Template resolution
│   │
│   ├── infrastructure/          # External adapters (each with doc.go)
│   │   ├── agents/              # AI agent providers (Claude, Gemini, Codex)
│   │   ├── config/              # Configuration file loading
│   │   ├── diagram/             # Workflow diagram generation (DOT)
│   │   ├── errors/              # Error formatting adapters
│   │   ├── executor/            # Shell command executor
│   │   ├── expression/          # Expression evaluator adapter
│   │   ├── github/              # Built-in GitHub operation provider
│   │   ├── logger/              # Zap logger adapter
│   │   ├── notify/              # Built-in notification operation provider
│   │   ├── plugin/              # RPC plugin manager, composite provider
│   │   ├── repository/          # YAML workflow loaders
│   │   ├── store/               # SQLite history, JSON state store
│   │   ├── tokenizer/           # Token counting
│   │   └── xdg/                 # XDG directory discovery
│   │
│   └── interfaces/              # External interfaces
│       └── cli/                 # CLI commands (with doc.go)
│           ├── root.go          # Root command
│           ├── run.go           # run command
│           ├── validate.go      # validate command
│           ├── list.go          # list command
│           ├── status.go        # status command
│           ├── init.go          # init command
│           ├── resume.go        # resume command
│           ├── history.go       # history command
│           ├── version.go       # version command
│           └── ui/              # UI components (with doc.go)
│               ├── colors.go    # Color output
│               ├── output.go    # Output formatting
│               └── formatter.go # Field formatting
│
├── pkg/                         # Public packages
│   ├── expression/              # Expression evaluation utilities
│   ├── httpx/                   # HTTP client helpers (HTTPDoer, size-limited reads)
│   ├── interpolation/           # Template interpolation
│   │   ├── interpolate.go       # Variable substitution
│   │   ├── escape.go            # Shell escaping
│   │   └── interpolate_test.go
│   ├── output/                  # Output formatting utilities
│   ├── plugin/                  # Plugin SDK for plugin authors
│   │   └── sdk/                 # sdk.Serve(), BasePlugin, helpers
│   ├── registry/                # Shared registry transport (C070)
│   │   ├── version.go           # Semantic versioning
│   │   ├── github_client.go     # GitHub Releases API client
│   │   └── downloader.go        # Download, checksum, extraction
│   ├── retry/                   # Retry logic
│   │   ├── backoff.go           # Backoff strategies
│   │   └── backoff_test.go
│   ├── stringutil/              # String manipulation utilities
│   └── validation/              # Validation utilities
│       ├── input.go             # Input validation
│       └── input_test.go
│
├── tests/                       # Integration tests
│   ├── integration/             # End-to-end tests
│   │   ├── cli_test.go          # CLI integration tests
│   │   └── workflow_test.go     # Workflow execution tests
│   └── fixtures/                # Test fixtures
│       └── workflows/           # Test workflow files
│
├── docs/                        # Documentation
│   ├── getting-started/         # Installation, quickstart
│   ├── user-guide/              # Commands, syntax, examples
│   ├── reference/               # Exit codes, interpolation
│   ├── development/             # Architecture, testing
│   └── plans/                   # Feature planning
│       ├── features/            # Feature specifications
│       └── implementation/      # Implementation plans
│
├── site/                        # Hugo documentation site (Doks theme)
│   ├── config/                  # Hugo configuration (split files)
│   ├── content/                 # Site content (landing page, blog)
│   ├── layouts/                 # Custom layout overrides
│   └── package.json             # Node.js dependencies (build-time only)
│
├── .awf/                        # AWF configuration (example)
│   ├── workflows/               # Workflow definitions
│   ├── templates/               # Workflow templates
│   ├── prompts/                 # Prompt templates
│   └── scripts/                 # Script files
│
├── bin/                         # Built binaries (gitignored)
│
├── Makefile                     # Build commands
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── .golangci.yml                # Linter configuration (golangci-lint)
├── .go-arch-lint.yml            # Architecture constraint rules (go-arch-lint)
├── .awf.yaml                    # AWF configuration file
│
├── README.md                    # Project overview
├── CHANGELOG.md                 # Version history
├── CONTRIBUTING.md              # Contributing guide
├── SECURITY.md                  # Security policy
├── CODE_OF_CONDUCT.md           # Code of conduct
└── LICENSE                      # MIT License
```

## Key Files

### Entry Point

**`cmd/awf/main.go`**

```go
package main

import (
    "github.com/awf-project/cli/internal/interfaces/cli"
)

func main() {
    cli.Execute()
}
```

### Package Documentation

Every major package includes a `doc.go` file providing package-level documentation accessible via `go doc`:

```bash
go doc ./internal/domain/workflow
go doc ./internal/infrastructure/agents
go doc ./internal/interfaces/cli
# ... and 18 other documented packages
```

**Documentation coverage**: 21 packages (100% of domain, application, infrastructure, and interface layers)

Each `doc.go` file includes:
- Package purpose and architecture role
- Key types with brief descriptions
- Port implementations (which domain interfaces are satisfied)
- Usage examples
- Links to related packages

See [Package Documentation Guide](../reference/package-documentation.md) for details on discovering and maintaining package docs.

### Domain Entities

**`internal/domain/workflow/workflow.go`** - Core workflow struct

**`internal/domain/ports/`** - Interface definitions that adapters implement

### Application Services

**`internal/application/execution_service.go`** - Main execution engine

### Infrastructure Adapters

**`internal/infrastructure/repository/yaml_repository.go`** - YAML workflow loader

**`internal/infrastructure/store/json_store.go`** - JSON state persistence

**`internal/infrastructure/store/sqlite_history_store.go`** - SQLite execution history

**`internal/infrastructure/executor/shell.go`** - Shell command execution

### Public Packages

**`pkg/registry/`** - Shared transport layer for GitHub Releases (versioning, downloads, checksum verification). Used by the plugin system and forthcoming workflow pack system.

**`pkg/interpolation/`** - Variable interpolation (safe to import externally)

## Naming Conventions

| Pattern | Location | Example |
|---------|----------|---------|
| `doc.go` | Every major package | `infrastructure/agents/doc.go` |
| `*_service.go` | Application layer | `workflow_service.go` |
| `*_test.go` | Same directory as tested file | `yaml_test.go` |
| Interfaces | `ports/` directory | `repository.go` |
| Adapters | Infrastructure subdirectories | `repository/yaml_repository.go` |

## Import Paths

```go
// Domain (no external imports)
import "github.com/awf-project/cli/internal/domain/workflow"
import "github.com/awf-project/cli/internal/domain/ports"

// Application (imports domain only)
import "github.com/awf-project/cli/internal/application"

// Infrastructure (imports domain ports)
import "github.com/awf-project/cli/internal/infrastructure/repository"

// CLI (imports application and infrastructure)
import "github.com/awf-project/cli/internal/interfaces/cli"

// Public packages (safe for external use)
import "github.com/awf-project/cli/pkg/interpolation"
```

## Build Artifacts

| Artifact | Location | Description |
|----------|----------|-------------|
| Binary | `bin/awf` | Compiled binary |
| Coverage | `coverage.html` | Test coverage report |
| State | `$XDG_DATA_HOME/awf/states/` or `~/.local/share/awf/states/` | Workflow state files |
| History | `$XDG_DATA_HOME/awf/history.db` or `~/.local/share/awf/history.db` | SQLite history database |

**Note:** Logs are written to stdout/stderr only. Use `--storage-path` flag to override default state location.

## See Also

- [Architecture](architecture.md) - Hexagonal architecture details
- [Testing](testing.md) - Testing conventions
