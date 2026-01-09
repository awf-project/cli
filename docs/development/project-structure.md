# Project Structure

AWF follows a standard Go project layout aligned with hexagonal architecture.

**Total:** 279 Go files (181 implementation + 98 tests)

## Directory Tree

```
awf/
├── cmd/
│   └── awf/
│       └── main.go                  # CLI entry point
│
├── internal/                        # Private application code
│   ├── domain/                      # Core business logic (95 files)
│   │   ├── workflow/                # Workflow entities (63 files)
│   │   │   ├── workflow.go          # Workflow struct
│   │   │   ├── step.go              # Step definition
│   │   │   ├── parallel.go          # Parallel step
│   │   │   ├── loop.go              # Loop construct (for_each, while)
│   │   │   ├── subworkflow.go       # Nested workflow step
│   │   │   ├── hooks.go             # Pre/post hooks
│   │   │   ├── agent_config.go      # Agent step configuration
│   │   │   ├── conversation.go      # Multi-turn conversations
│   │   │   ├── context.go           # Execution context
│   │   │   ├── context_window.go    # Conversation window management
│   │   │   ├── template.go          # Template variables
│   │   │   ├── validation.go        # Workflow validation
│   │   │   ├── condition.go         # Conditional logic
│   │   │   ├── graph.go             # Workflow graph analysis
│   │   │   └── ...
│   │   ├── ports/                   # Port interfaces (18 files)
│   │   │   ├── repository.go        # Workflow loading
│   │   │   ├── store.go             # State persistence
│   │   │   ├── executor.go          # Command execution
│   │   │   ├── step_executor.go     # Single step execution
│   │   │   ├── parallel_executor.go # Parallel execution
│   │   │   ├── logger.go            # Logging
│   │   │   ├── input_collector.go   # User input
│   │   │   ├── interactive.go       # Interactive prompting
│   │   │   ├── history.go           # Execution history
│   │   │   ├── agent_provider.go    # AI agent providers
│   │   │   ├── plugin.go            # Plugin execution
│   │   │   └── tokenizer.go         # Token counting
│   │   └── plugin/                  # Plugin domain model (14 files)
│   │       ├── manifest.go          # Plugin manifest
│   │       ├── operation.go         # Plugin operation
│   │       ├── state.go             # Plugin state
│   │       └── info.go              # Plugin metadata
│   │
│   ├── application/                 # Application services (31 files)
│   │   ├── service.go               # Main application service
│   │   ├── execution_service.go     # Workflow execution
│   │   ├── conversation_manager.go  # Multi-turn conversation orchestration
│   │   ├── hook_executor.go         # Lifecycle hooks
│   │   ├── loop_executor.go         # Loop state machine
│   │   ├── parallel_executor.go     # Parallel step execution
│   │   ├── single_step.go           # Single step execution
│   │   ├── subworkflow_executor.go  # Nested workflow invocation
│   │   ├── plugin_service.go        # Plugin lifecycle management
│   │   ├── template_service.go      # Template resolution
│   │   ├── dry_run_executor.go      # Dry-run execution
│   │   ├── interactive_executor.go  # Interactive mode
│   │   ├── history_service.go       # Execution history
│   │   └── input_collection_service.go # Input handling
│   │
│   ├── infrastructure/              # External adapters (100 files)
│   │   ├── repository/              # Workflow loaders (8 files)
│   │   │   ├── yaml_repository.go   # YAML file loader
│   │   │   ├── yaml_mapper.go       # YAML to domain mapping
│   │   │   ├── yaml_types.go        # YAML struct definitions
│   │   │   ├── composite_repository.go # Multi-source repository
│   │   │   ├── template_repository.go  # Template loading
│   │   │   └── source.go            # Repository source interface
│   │   ├── store/                   # State & history storage (4 files)
│   │   │   ├── json_store.go        # JSON state persistence
│   │   │   └── sqlite_history_store.go # SQLite execution history
│   │   ├── executor/                # Command execution (2 files)
│   │   │   └── shell_executor.go    # Shell command executor
│   │   ├── logger/                  # Logging (8 files)
│   │   │   ├── console_logger.go    # Console output
│   │   │   ├── json_logger.go       # JSON structured logging
│   │   │   ├── multi_logger.go      # Multi-target logging
│   │   │   └── masker.go            # Secret masking
│   │   ├── agents/                  # AI agent providers (14 files)
│   │   │   ├── registry.go          # Agent provider registry
│   │   │   ├── claude_provider.go   # Anthropic Claude
│   │   │   ├── gemini_provider.go   # Google Gemini
│   │   │   ├── codex_provider.go    # OpenAI Codex
│   │   │   ├── opencode_provider.go # OpenCode
│   │   │   ├── custom_provider.go   # Custom HTTP provider
│   │   │   └── mock_provider.go     # Mock for testing
│   │   ├── tokenizer/               # Token counting (4 files)
│   │   │   ├── tiktoken_tokenizer.go      # TikToken-based
│   │   │   └── approximation_tokenizer.go # Approximation fallback
│   │   ├── plugin/                  # Plugin management (12 files)
│   │   │   ├── loader.go            # Plugin binary loader
│   │   │   ├── registry.go          # Plugin registry
│   │   │   ├── rpc_manager.go       # Plugin RPC communication
│   │   │   ├── manifest_parser.go   # Manifest parsing
│   │   │   ├── state_store.go       # Plugin state storage
│   │   │   └── version.go           # Version compatibility
│   │   ├── diagram/                 # Workflow visualization (6 files)
│   │   │   ├── graphviz.go          # Graphviz integration
│   │   │   ├── dot_generator.go     # DOT format generation
│   │   │   └── config.go            # Diagram configuration
│   │   ├── analyzer/                # Template analysis (2 files)
│   │   │   └── template_analyzer.go # Template variable analysis
│   │   ├── config/                  # Configuration loading (4 files)
│   │   │   ├── loader.go            # Config file parser
│   │   │   └── types.go             # Config data structures
│   │   └── xdg/                     # XDG directories (2 files)
│   │       └── xdg.go               # XDG path discovery
│   │
│   └── interfaces/                  # External interfaces
│       └── cli/                     # CLI commands (70 files)
│           ├── root.go              # Root command + version
│           ├── config.go            # Configuration management
│           ├── run.go               # run command
│           ├── run_help.go          # Workflow-specific help
│           ├── resume.go            # resume command
│           ├── validate.go          # validate command
│           ├── list.go              # list command
│           ├── status.go            # status command
│           ├── init.go              # init command
│           ├── history.go           # history command
│           ├── diagram.go           # diagram command
│           ├── plugin_cmd.go        # plugin command
│           ├── config_cmd.go        # config command
│           ├── migration.go         # XDG migration
│           ├── exitcodes.go         # Exit code mapping
│           └── ui/                  # UI components (14 files)
│               ├── formatter.go     # Text formatting
│               ├── output_writer.go # Structured output
│               ├── output.go        # Output formats
│               ├── colors.go        # ANSI colors
│               ├── input_collector.go    # User input collection
│               ├── interactive_prompt.go # Interactive prompts
│               └── dry_run_formatter.go  # Dry-run output
│
├── pkg/                             # Public packages (23 files)
│   ├── expression/                  # Expression evaluation
│   │   └── evaluator.go             # Expression parser
│   ├── interpolation/               # Template interpolation
│   │   ├── resolver.go              # Variable resolution
│   │   ├── template_resolver.go     # Template-specific resolver
│   │   ├── reference.go             # Reference parsing
│   │   ├── escaping.go              # Shell escaping
│   │   ├── serializer.go            # Value serialization
│   │   └── errors.go                # Error types
│   ├── validation/                  # Input validation
│   │   └── validator.go             # Validation logic
│   ├── retry/                       # Retry logic
│   │   ├── retryer.go               # Retry implementation
│   │   └── backoff.go               # Backoff strategies
│   └── plugin/sdk/                  # Plugin SDK (public API)
│       ├── sdk.go                   # Plugin SDK interface
│       └── testing.go               # Plugin testing utilities
│
├── tests/                           # Integration tests
│   ├── integration/                 # End-to-end tests (29 files)
│   │   ├── cli_test.go              # CLI command tests
│   │   ├── execution_test.go        # Workflow execution
│   │   ├── agent_test.go            # Agent step execution
│   │   ├── conversation_test.go     # Multi-turn conversations
│   │   ├── loop_test.go             # Loop execution
│   │   ├── parallel_execution_test.go # Parallel steps
│   │   ├── subworkflow_test.go      # Nested workflows
│   │   ├── plugin_test.go           # Plugin execution
│   │   ├── history_test.go          # Execution history
│   │   ├── resume_test.go           # Resume functionality
│   │   └── ...
│   └── fixtures/                    # Test fixtures
│       ├── workflows/               # 42 test workflow YAML files
│       ├── templates/               # 7 template files
│       ├── config/                  # 7 config files
│       ├── plugins/                 # 8 plugin directories
│       └── prompts/                 # 8 prompt directories
│
├── docs/                            # Documentation
│   ├── getting-started/             # Installation, quickstart
│   ├── user-guide/                  # Commands, syntax, examples
│   ├── reference/                   # Exit codes, interpolation
│   ├── development/                 # Architecture, testing
│   └── plans/                       # Feature planning
│       ├── features/                # Feature specifications
│       └── implementation/          # Implementation plans
│
├── .awf/                            # Local AWF configuration
│   ├── workflows/                   # Local workflow definitions
│   └── prompts/                     # Local prompt templates
│
├── bin/                             # Built binaries (gitignored)
│
├── Makefile                         # Build commands
├── go.mod                           # Go module definition
├── go.sum                           # Dependency checksums
├── .golangci.yml                    # Linter configuration
├── .awf.yaml                        # AWF configuration file
│
├── README.md                        # Project overview
├── CHANGELOG.md                     # Version history
├── CONTRIBUTING.md                  # Contributing guide
├── SECURITY.md                      # Security policy
├── CODE_OF_CONDUCT.md               # Code of conduct
└── LICENSE                          # MIT License
```

## CLI Commands

| Command | Description | Key Flags |
|---------|-------------|-----------|
| `awf run` | Execute workflow | `--input`, `--output`, `--dry-run`, `--interactive`, `--step` |
| `awf resume` | Resume interrupted execution | `--list`, `--input` |
| `awf validate` | Validate workflow syntax | - |
| `awf list` | List workflows/prompts | `list prompts` subcommand |
| `awf status` | Show execution status | workflow-id argument |
| `awf init` | Initialize project | `--force`, `--global` |
| `awf history` | Execution history | `--workflow`, `--status`, `--since`, `--stats` |
| `awf diagram` | Generate visualization | `--output`, `--direction`, `--highlight` |
| `awf plugin` | Manage plugins | `list`, `enable`, `disable` subcommands |
| `awf config` | Manage configuration | `show` subcommand |
| `awf version` | Show version info | - |

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

### Domain Layer

| File | Purpose |
|------|---------|
| `domain/workflow/workflow.go` | Core workflow entity |
| `domain/workflow/conversation.go` | Multi-turn conversation model |
| `domain/workflow/loop.go` | Loop constructs (for_each, while) |
| `domain/ports/*.go` | Port interfaces (dependency inversion) |
| `domain/plugin/manifest.go` | Plugin manifest model |

### Application Services

| Service | Purpose |
|---------|---------|
| `execution_service.go` | Main workflow execution |
| `conversation_manager.go` | Multi-turn conversation orchestration |
| `loop_executor.go` | Loop state machine |
| `parallel_executor.go` | Parallel step execution |
| `plugin_service.go` | Plugin lifecycle |

### Infrastructure Adapters

| Adapter | Purpose |
|---------|---------|
| `repository/yaml_repository.go` | YAML workflow loader |
| `store/json_store.go` | JSON state persistence |
| `store/sqlite_history_store.go` | SQLite execution history |
| `agents/claude_provider.go` | Claude AI integration |
| `tokenizer/tiktoken_tokenizer.go` | Token counting |

### Public Packages

**`pkg/interpolation/`** - Variable interpolation (safe to import externally)

**`pkg/plugin/sdk/`** - Plugin SDK for building AWF plugins

## Naming Conventions

| Pattern | Location | Example |
|---------|----------|---------|
| `*_service.go` | Application layer | `execution_service.go` |
| `*_executor.go` | Application layer | `loop_executor.go` |
| `*_provider.go` | Infrastructure/agents | `claude_provider.go` |
| `*_repository.go` | Infrastructure/repository | `yaml_repository.go` |
| `*_store.go` | Infrastructure/store | `json_store.go` |
| `*_test.go` | Same directory as tested file | `yaml_repository_test.go` |
| Interfaces | `ports/` directory | `agent_provider.go` |

## Import Paths

```go
// Domain (no external imports)
import "github.com/vanoix/awf/internal/domain/workflow"
import "github.com/vanoix/awf/internal/domain/ports"
import "github.com/vanoix/awf/internal/domain/plugin"

// Application (imports domain only)
import "github.com/vanoix/awf/internal/application"

// Infrastructure (imports domain ports)
import "github.com/vanoix/awf/internal/infrastructure/repository"
import "github.com/vanoix/awf/internal/infrastructure/agents"

// CLI (imports application and infrastructure)
import "github.com/vanoix/awf/internal/interfaces/cli"

// Public packages (safe for external use)
import "github.com/vanoix/awf/pkg/interpolation"
import "github.com/vanoix/awf/pkg/plugin/sdk"
```

## Runtime Data Storage

AWF follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/).

### Storage Locations

| Data | Path | Description |
|------|------|-------------|
| States | `$XDG_DATA_HOME/awf/states/` | Workflow state files (JSON) |
| History | `$XDG_DATA_HOME/awf/history.db` | SQLite execution history |
| Plugins | `$XDG_DATA_HOME/awf/plugins/` | Plugin binaries |
| Global workflows | `$XDG_CONFIG_HOME/awf/workflows/` | Global workflow definitions |
| Global prompts | `$XDG_CONFIG_HOME/awf/prompts/` | Global prompt templates |

Default paths (when XDG vars not set):
- `$XDG_DATA_HOME` = `~/.local/share`
- `$XDG_CONFIG_HOME` = `~/.config`

### Local Project Structure

Created by `awf init`:

```
.awf.yaml              # Project configuration
.awf/
├── workflows/         # Local workflow definitions
└── prompts/           # Local prompt templates
```

### Discovery Priority

**Workflows** (high to low):
1. `$AWF_WORKFLOWS_PATH` environment variable
2. `./.awf/workflows/` (local project)
3. `$XDG_CONFIG_HOME/awf/workflows/` (global)

**Plugins** (high to low):
1. `$AWF_PLUGINS_PATH` environment variable
2. `./.awf/plugins/` (local project)
3. `$XDG_DATA_HOME/awf/plugins/` (global)

## Build Artifacts

| Artifact | Location | Description |
|----------|----------|-------------|
| Binary | `bin/awf` | Compiled binary |
| Coverage | `coverage.html` | Test coverage report |

## See Also

- [Architecture](architecture.md) - Hexagonal architecture details
- [Testing](testing.md) - Testing conventions
