# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Serena MCP Integration

**IMPORTANT:** This project uses Serena MCP for persistent memory across sessions.

### At Session Start
1. Activate project: `mcp__plugin_common_serena__activate_project` with path `/home/pocky/Sites/vanoix/gustave`
2. Check onboarding: `mcp__plugin_common_serena__check_onboarding_performed`
3. List and read relevant memories: `mcp__plugin_common_serena__list_memories`
4. Attach repomix output: `mcp__plugin_common_repomix__attach_packed_output` with path `.repomix`

### Available Memories
- `project_overview` - Purpose, tech stack, architecture
- `architecture_details` - Hexagonal layers, ports/adapters
- `code_style_conventions` - Go style, rules
- `suggested_commands` - Build, test, CLI commands
- `task_completion_checklist` - Pre-commit checklist
- `development_history` - Git evolution, commits
- `feature_roadmap` - v0.1→v1.0 features status
- `next_features_detail` - F010, F013, F032 specs

### At Session End (After Significant Work)
Update memories if:
- New patterns or conventions established
- Architecture changes made
- New features implemented
- Important decisions documented

Use `mcp__plugin_common_serena__write_memory` or `mcp__plugin_common_serena__edit_memory`.

## Repomix MCP Integration

**IMPORTANT:** This project uses Repomix MCP for codebase analysis.

### Storage Location
- Output directory: `.repomix/`
- Output file: `.repomix/repomix-output.xml`
- Ignored in `.gitignore`

### At Session Start
Attach existing pack: `mcp__plugin_common_repomix__attach_packed_output` with path `.repomix`

### After Significant Code Changes
Update the pack:
1. `mcp__plugin_common_repomix__pack_codebase` with:
   - `directory`: `/home/pocky/Sites/vanoix/gustave`
   - `ignorePatterns`: `.git/**,storage/**,bin/**,coverage.*,*.db,*.log`
2. Copy output to `.repomix/`: `cp /tmp/repomix/mcp-outputs/*/repomix-output.xml .repomix/`

### Usage
- `grep_repomix_output` - Search patterns in packed codebase
- `read_repomix_output` - Read content with line ranges

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
awf resume <workflow-id>            # Resume interrupted workflow
awf validate <workflow>             # Static validation
awf list                            # List available workflows
awf status <workflow-id>            # Check running workflow
awf history                         # Execution history
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
