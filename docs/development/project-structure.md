# Project Structure

AWF follows a standard Go project layout aligned with hexagonal architecture.

## Directory Tree

```
awf/
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
│   │   ├── operation/           # Operation interface
│   │   │   ├── operation.go     # Operation contract
│   │   │   └── result.go        # Result struct
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
│   ├── infrastructure/          # External adapters
│   │   ├── repository/          # Workflow loaders
│   │   │   ├── yaml.go          # YAML file repository
│   │   │   └── yaml_test.go
│   │   ├── state/               # State storage
│   │   │   ├── json.go          # JSON file store
│   │   │   └── json_test.go
│   │   ├── executor/            # Command execution
│   │   │   ├── shell.go         # Shell executor
│   │   │   └── shell_test.go
│   │   ├── store/               # Data storage
│   │   │   ├── sqlite_history_store.go      # SQLite history adapter
│   │   │   └── sqlite_history_store_test.go
│   │   ├── logger/              # Logging
│   │   │   └── zap.go           # Zap logger adapter
│   │   └── xdg/                 # XDG directories
│   │       └── xdg.go           # XDG path discovery
│   │
│   └── interfaces/              # External interfaces
│       └── cli/                 # CLI commands
│           ├── root.go          # Root command
│           ├── run.go           # run command
│           ├── validate.go      # validate command
│           ├── list.go          # list command
│           ├── status.go        # status command
│           ├── init.go          # init command
│           ├── resume.go        # resume command
│           ├── history.go       # history command
│           ├── version.go       # version command
│           └── ui/              # UI components
│               ├── colors.go    # Color output
│               ├── progress.go  # Progress bars
│               └── format.go    # Output formatting
│
├── pkg/                         # Public packages
│   ├── interpolation/           # Template interpolation
│   │   ├── interpolate.go       # Variable substitution
│   │   ├── escape.go            # Shell escaping
│   │   └── interpolate_test.go
│   ├── validation/              # Validation utilities
│   │   ├── input.go             # Input validation
│   │   └── input_test.go
│   └── retry/                   # Retry logic
│       ├── backoff.go           # Backoff strategies
│       └── backoff_test.go
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
├── .awf/                        # AWF configuration (example)
│   ├── workflows/               # Workflow definitions
│   ├── templates/               # Workflow templates
│   └── prompts/                 # Prompt templates
│
├── bin/                         # Built binaries (gitignored)
│
├── Makefile                     # Build commands
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
├── .golangci.yml                # Linter configuration
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
    "github.com/vanoix/awf/internal/interfaces/cli"
)

func main() {
    cli.Execute()
}
```

### Domain Entities

**`internal/domain/workflow/workflow.go`** - Core workflow struct

**`internal/domain/ports/`** - Interface definitions that adapters implement

### Application Services

**`internal/application/execution_service.go`** - Main execution engine

### Infrastructure Adapters

**`internal/infrastructure/repository/yaml.go`** - YAML workflow loader

**`internal/infrastructure/state/json.go`** - JSON state persistence

**`internal/infrastructure/executor/shell.go`** - Shell command execution

### Public Packages

**`pkg/interpolation/`** - Variable interpolation (safe to import externally)

## Naming Conventions

| Pattern | Location | Example |
|---------|----------|---------|
| `*_service.go` | Application layer | `workflow_service.go` |
| `*_test.go` | Same directory as tested file | `yaml_test.go` |
| Interfaces | `ports/` directory | `repository.go` |
| Adapters | Infrastructure subdirectories | `repository/yaml.go` |

## Import Paths

```go
// Domain (no external imports)
import "github.com/vanoix/awf/internal/domain/workflow"
import "github.com/vanoix/awf/internal/domain/ports"

// Application (imports domain only)
import "github.com/vanoix/awf/internal/application"

// Infrastructure (imports domain ports)
import "github.com/vanoix/awf/internal/infrastructure/repository"

// CLI (imports application and infrastructure)
import "github.com/vanoix/awf/internal/interfaces/cli"

// Public packages (safe for external use)
import "github.com/vanoix/awf/pkg/interpolation"
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
