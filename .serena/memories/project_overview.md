# Project Overview: awf (AI Workflow CLI)

## Purpose
CLI tool for orchestrating AI agents (Claude, Gemini, Codex) through YAML-configured workflows with state machine execution.

## Tech Stack
- **Language:** Go 1.21+
- **CLI Framework:** Cobra
- **YAML Parsing:** gopkg.in/yaml.v3
- **Logging:** uber/zap
- **Database:** SQLite3 (CGO required)
- **Testing:** testify (assert, require, mock)
- **Concurrency:** golang.org/x/sync/errgroup

## Architecture
Hexagonal/Clean Architecture with strict dependency inversion:
- `cmd/awf/` - CLI entry point
- `internal/domain/` - Business logic (ZERO external dependencies)
- `internal/application/` - Orchestration services
- `internal/infrastructure/` - Adapters (YAML, JSON, Shell, Logger)
- `internal/interfaces/cli/` - Cobra commands, UI components
- `pkg/` - Public reusable packages (interpolation)

## Key Files
- `internal/domain/ports/` - Interface definitions (Repository, Executor, StateStore, Logger)
- `internal/domain/workflow/` - Core entities (Workflow, Step, Context)
- `internal/application/execution_service.go` - State machine execution
- `internal/infrastructure/repository/yaml_repository.go` - Workflow loading

## Error Taxonomy (Exit Codes)
- 1 = User error (bad input, missing file)
- 2 = Workflow error (invalid state reference, cycle)
- 3 = Execution error (command failed, timeout)
- 4 = System error (IO, permissions)
