# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## MCP Integrations

This project uses several MCP servers for persistent memory and code search.

### Serena MCP — Symbolic Code Analysis & Memory

**Primary use:** Symbolic code navigation (find symbols, references, overview), persistent project memories, code editing at symbol level.

#### At Session Start
1. Check onboarding: `mcp__plugin_common_serena__check_onboarding_performed`
2. List and read relevant memories: `mcp__plugin_common_serena__list_memories`

#### Available Memories
- `project_overview` - Purpose, tech stack, architecture
- `architecture_details` - Hexagonal layers, ports/adapters
- `code_style_conventions` - Go style, rules
- `suggested_commands` - Build, test, CLI commands
- `task_completion_checklist` - Pre-commit checklist
- `development_history` - Git evolution, commits
- `feature_roadmap` - v0.1→v1.0 features status
- `next_features_detail` - F010, F013, F032 specs

#### At Session End (After Significant Work)
Update memories if:
- New patterns or conventions established
- Architecture changes made
- New features implemented
- Important decisions documented

Use `mcp__plugin_common_serena__write_memory` or `mcp__plugin_common_serena__edit_memory`.

### claude-mem MCP — Cross-Session Semantic Memory

**Primary use:** Search past work, decisions, and discoveries across all sessions. 3-layer workflow for token efficiency.

#### Search Workflow (always follow this order)
1. `mcp__plugin_claude-mem_mcp-search__search` — Get index with IDs (~50-100 tokens/result)
2. `mcp__plugin_claude-mem_mcp-search__timeline` — Get context around interesting results
3. `mcp__plugin_claude-mem_mcp-search__get_observations` — Fetch full details ONLY for filtered IDs

#### Save Memory
- `mcp__plugin_claude-mem_mcp-search__save_memory` — Save important findings, decisions, patterns

#### When to Use
- "Did we already solve this?" — search past sessions
- "How did we do X last time?" — find implementation patterns
- Understanding past architectural decisions
- Avoiding re-doing research already completed

### GrepAI MCP — Semantic Code Search & Call Graph

**Primary use:** Natural language code search, function call tracing, dependency analysis.

#### Search Tools
- `mcp__grepai__grepai_search` — Semantic code search with natural language queries (e.g., "user authentication flow", "error handling middleware")
- `mcp__grepai__grepai_index_status` — Check index health and statistics

#### Call Graph Tracing
- `mcp__grepai__grepai_trace_callers` — Find all functions that call a given symbol (understand impact before modifying)
- `mcp__grepai__grepai_trace_callees` — Find all functions called by a given symbol (understand dependencies)
- `mcp__grepai__grepai_trace_graph` — Build complete call graph around a symbol (both callers and callees, configurable depth)

#### When to Use
- Finding code by intent rather than exact name ("how are workflows executed?")
- Impact analysis before refactoring (trace callers)
- Understanding dependency chains (trace callees/graph)
- Exploring unfamiliar parts of the codebase

## Project Overview

**ai-workflow-cli** (`awf`) - A Go CLI tool for orchestrating AI agents (Claude, Gemini, Codex) through YAML-configured workflows with state machine execution.

## Architecture

Hexagonal/Clean Architecture with strict dependency inversion:

```
┌─────────────────────────────────────────────────────────────┐
│                     INTERFACES LAYER                        │
│      CLI (current)  │  API (future)  │  MQ (future)        │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                   APPLICATION LAYER                         │
│   WorkflowService, ExecutionService, StateManager           │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                      DOMAIN LAYER                           │
│   Workflow Entities │ StateMachine │ Context & State        │
│   PORTS (Interfaces): Repository | StateStore | Executor    │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────┴─────────────────────────────────┐
│                  INFRASTRUCTURE LAYER                       │
│   YAMLRepository │ JSONStateStore │ ShellExecutor │ Logger  │
└─────────────────────────────────────────────────────────────┘
```

Domain layer depends on nothing. All other layers depend inward toward domain.

## Project Structure

```
cmd/awf/main.go              # CLI entry point
internal/
├── domain/
│   ├── workflow/            # Workflow, Step, State, Context, Hooks entities
│   ├── operation/           # Operation interface and Result
│   └── ports/               # Repository, StateStore, Executor, Logger interfaces
├── application/             # WorkflowService, ExecutionService, StateManager
├── infrastructure/          # YAML repo, JSON store, Shell executor, Loggers
└── interfaces/cli/          # Cobra commands, UI components
pkg/                         # Public packages: interpolation, validation, retry
configs/workflows/           # YAML workflow definitions
storage/                     # Runtime data: states/, logs/, history.db
```

## Build Commands

```bash
make build              # Build binary to ./bin/awf
make install            # Build and install to /usr/local/bin
make dev                # Run without building: go run ./cmd/awf
make test               # All tests
make test-unit          # Unit tests: ./internal/... ./pkg/...
make test-integration   # Integration tests: ./tests/integration/...
make test-coverage      # Generate coverage.html
make test-race          # Race detector
make lint               # golangci-lint
make fmt                # go fmt
make vet                # go vet
```

## CLI Usage

```bash
awf run <workflow> --input=value    # Execute workflow
awf validate <workflow>             # Static validation
awf list                            # List available workflows
awf status <workflow-id>            # Check running workflow
awf init                            # Initialize AWF in current directory
```

## Error Taxonomy

Exit codes map to error types:
- `1` = User error (bad input, missing file)
- `2` = Workflow error (invalid state reference, cycle)
- `3` = Execution error (command failed, timeout)
- `4` = System error (IO, permissions)

## YAML Workflow Syntax

Template interpolation uses `{{var}}` (Go template style, not `${var}`) to avoid shell conflicts.

Available variables:
- `{{inputs.name}}` - Input values
- `{{states.step_name.output}}` - Step outputs
- `{{workflow.id}}`, `{{workflow.name}}`, `{{workflow.duration}}`
- `{{env.VAR_NAME}}` - Environment variables
- `{{error.type}}`, `{{error.message}}` - In error hooks

State types: `step`, `parallel`, `terminal`

Parallel strategies: `all_succeed`, `any_succeed`, `best_effort`

## Key Implementation Details

### State Persistence
Atomic writes via temp file + rename. File locking for concurrent access.

### Parallel Execution
Uses `golang.org/x/sync/errgroup` with semaphore for controlled concurrency.

### Signal Handling
Context propagation for graceful cancellation. Process groups for clean termination.

### Security
- Shell commands use `/bin/sh -c` by design (supports pipes, redirects)
- `ShellEscape()` in `pkg/interpolation` for user-provided values
- Secret masking in logs (vars starting with `SECRET_`, `API_KEY`, `PASSWORD`)
- Atomic file writes prevent corruption (unique temp files with PID+timestamp)

## Dependencies

Core: `cobra`, `yaml.v3`, `zap`, `sqlite3` (CGO), `fatih/color`, `progressbar`, `errgroup`, `uuid`, `testify`

## Testing Conventions

```go
// Table-driven tests
func TestWorkflowValidation(t *testing.T) {
    tests := []struct {
        name    string
        workflow *workflow.Workflow
        wantErr bool
    }{...}
}

// Integration tests use fixtures from tests/fixtures/
```

## Recent Changes (December 2025)

See `CHANGELOG.md` and `docs/code-review-2025-12.md` for details.

### Breaking Changes
- `Args` field removed from `ports.Command` struct (was unused)

### Bug Fixes
- YAML parsing now reports all errors (was silently skipping malformed steps)
- Race condition in JSONStore fixed (concurrent Save used same temp file)

### New Validations
- `ParallelStrategy` validated: `all_succeed`, `any_succeed`, `best_effort`, or empty

## Architecture Rules

Domain layer packages must restrict dependencies via go-arch-lint rules; domain/operation imports only domain/plugin, domain/ports, and stdlib

Implement domain ports directly on domain types (e.g., OperationRegistry implements ports.OperationProvider) to enable zero-change integration with application services

Use pkg pattern for cross-layer HTTP utilities (pkg/httputil) to avoid infrastructure-to-infrastructure mayDependOn violations; register as commonComponent in go-arch-lint.yml

Implement infrastructure operation providers (e.g., HTTPOperationProvider) with direct domain type implementation to enable zero-change wiring into CompositeOperationProvider

## Common Pitfalls

Preserve existing infrastructure layers when adding domain registries; ADR-004 enforces infrastructure plugin registry coexistence for separate lifecycle concerns

Never duplicate HTTP client logic across notification backends; extract to pkg/httputil with HTTPDoer interface for testability and shared timeout/header handling

Limit HTTP response bodies at operation level (default 1MB via io.LimitReader) with truncated flag; preserve unlimited reads for notify backends by allowing maxBytes=0

## Test Conventions

Integration tests use compile-time interface checks (var _ PortInterface = (*Implementation)(nil)) to verify port implementation at build time

Use HTTPDoer interface in pkg/httputil tests to mock HTTP behavior (timeouts, DNS errors, connection failures) without requiring adapters or *http.Client modifications

## Review Standards

