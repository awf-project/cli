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
- Shell commands use the user's detected shell (`$SHELL`, fallback `/bin/sh`) with `-c` (supports pipes, redirects)
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

### Breaking Changes
- `Args` field removed from `ports.Command` struct (was unused)

### Bug Fixes
- YAML parsing now reports all errors (was silently skipping malformed steps)
- Race condition in JSONStore fixed (concurrent Save used same temp file)

### New Validations
- `ParallelStrategy` validated: `all_succeed`, `any_succeed`, `best_effort`, or empty

## Architecture Rules

- Pass optional turn-specific configuration (e.g., system_prompt) through options map in application layer; keeps infrastructure providers independent of turn logic
- Validate agent provider options only against what each CLI actually accepts; do not validate against API documentation if the underlying CLI rejects the option
- Plugin binaries must be discoverable at <plugins_dir>/<plugin_name>/awf-plugin-<plugin_name>; host validates binary existence and version compatibility via gRPC handshake after process start
- Commit generated protobuf files (.pb.go, _grpc.pb.go) to git; treat as source artifacts for build reproducibility, not ephemeral build outputs
- CLI command implementations must call infrastructure layer methods rather than reimplementing HTTP requests, parsing, or validation; avoid logic duplication
- Application layer must persist source metadata (SetSourceData) after successful infrastructure installation; omitting state blocks downstream operations like updates
- Use dual import aliases (e.g., infrastructurePlugin + registry) when consuming refactored packages; explicitly requalify all symbol references to prevent ambiguity
- Keep thin wrapper functions in original location for backward compatibility; delegate completely to extracted packages to maintain single source of truth
- Verify pkg/ package extractions are complete by confirming orphaned imports are removed and make lint passes with zero violations
- Extract duplicate interface types across packages when structurally identical; avoid declaring the same type signature in multiple infrastructure files
- Extract shared configuration keys as named constants in the application layer (e.g., AWFPackNameKey); import and use throughout codebase rather than duplicating string literals
- Wire optional dependencies via Set*() calls in consistent order; SetConversationManager must follow SetAgentRegistry to ensure agent registry is available before conversation manager initialization
- Initialize ApproximationTokenizer immediately before NewConversationManager in interfaces layer; token counting must be ready before conversation context is established
- Resolve user-provided interpolated variables before registry or dependency lookups to enable dynamic selection at runtime; apply identical resolution logic across all related code paths
- Implement per-provider flag mapping without shared abstraction when CLI syntax diverges fundamentally; document divergence (Claude: --flag-name, Gemini: --flag-name=value, Codex: --flag-name) inline
- Synchronize provider CLI flag changes across both implementation files and central options configuration (options.go); verify declarations and validation rules align
- When extracting shared infrastructure behavior across multiple provider implementations, apply the delegation pattern uniformly; partial refactoring creates inconsistent ownership
- When wiring optional transformations across multiple execution paths (ExecuteConversation, runWorkflow, etc.), apply consistently to all paths; missing stubs in any path indicates incomplete cross-layer wiring
- When adding hook fields to shared infrastructure types, implement (with stubs acceptable for future providers) across all concrete providers in the same layer; missing implementations in any provider blocks deployment
- Use function type interfaces (DisplayEventParser) for provider-specific implementations when output formats diverge; enables independent testing and future provider additions without modifying existing code
- Wire optional render callbacks alongside event parsers in stream processors; decouples rendering from parsing and enables multiple render modes (DefaultMode, VerboseMode) without modifying parser implementations
- When integrating external UI frameworks, create Bridge adapters in the interface layer that wrap application services; maintain zero infrastructure imports in bridge implementation

## Common Pitfalls

- Use 0o755 for executable scripts, 0o644 for data files, 0o700 for private temp files; match permissions to file purpose and access expectations
- When adding new scaffolded directories to init, replicate existing implementation patterns (e.g., createExampleScript mirrors createExamplePrompt) for consistency
- Always update user-facing documentation (docs/reference/, docs/user-guide/) and CHANGELOG.md when implementing features or behavior changes
- Halt implementation immediately when scope deviations are discovered; update plan and communicate changes before continuing work
- Apply identical error handling patterns across similar functions; handleNonZeroExit and handleExecutionError must both evaluate transitions before fallbacks
- When removing redundant infrastructure code, document the architectural ownership pattern; explain which layer assumed responsibility and why the field was removed
- Always apply code deletions before writing tests that validate the deletion effect; tests may pass against overridden behavior instead of the intended code path
- Wrap YAML/JSON mapping errors (duration parse, type conversion) in domain error types; surface failures immediately to prevent silent defaults
- Never merge infrastructure provider stubs; always implement ExecuteConversation fully or return NotImplementedError with linked tracking issue
- When enabling session persistence in CLI providers, force JSON output format for reliable field extraction; document as known limitation that overrides user-specified format
- Always provide graceful fallback to stateless mode when optional session ID extraction fails; never fail the entire operation due to extraction errors
- When migrating API JSON field names, parse both old and new keys with new key preferred; use dual-key parsing for backwards compatibility without validation errors
- Leverage Go's map[string]any behavior to silently ignore unsupported provider options; avoids validation errors while maintaining clear intent
- Avoid variable shadowing; never redeclare outer-scope variables with := in inner blocks
- Use index-based loops or pointer ranges when iterating large structs (>128 bytes); avoid per-iteration copying
- Limit function return values to 5; return a struct for 6+ outputs to maintain readability
- Always defer cancel() immediately after context.WithCancel() to prevent goroutine leaks
- Always validate shell paths with filepath.Clean before file operations to prevent path traversal violations
- Always include explanatory comments with //nolint directives; document why the warning is a false positive (e.g., 'controlled test input', 'architecturally safe conversion')
- File descriptor conversions (uintptr→int) are architecturally safe on all supported platforms; add //nolint:gosec G115 with explanation rather than runtime guards
- Use atomic file renames (os.Rename after close) for executable test fixtures; prevents 'text file busy' errors when replacing binaries that running tests currently execute from
- Never hardcode resource identifiers (filenames, URLs) in lookup operations; derive from actual source data or pass as parameter to enable correct matching
- Never include test-only fallback logic (localhost URLs, test servers) in production code paths; use env vars or dependency injection for test mocking instead
- Always shutdown gRPC plugin connections before removing plugin binaries; prevents 'text file busy' errors and ensures graceful termination
- Never commit CLI command implementations as stubs; implement fully or return explicit NotImplementedError with linked issue
- Always use http.NewRequestWithContext instead of http.NewRequest; pass context.Background() or appropriate request context
- Always limit external file downloads with size caps; use httpx.ReadBody instead of io.ReadAll for untrusted sources to prevent OOM attacks
- Always URL-encode user input via url.QueryEscape before concatenating into API URLs to prevent query parameter injection
- Delete empty placeholder files created during refactoring; verify no unintended artifacts remain before committing
- Always validate user-provided pack and workflow names from YAML input; use filepath.Clean and verify no path traversal patterns before filepath.Join operations
- Never rely on single-error checks in file operations; handle os.IsPermission and os.IsTimeout separately from os.IsNotExist to avoid silent failures
- Never panic on nil input in public infrastructure functions; return explicit error type to enable proper error handling by callers
- Never wire optional infrastructure in runDryRun or runInteractive execution paths; these preview modes must remain infrastructure-free to avoid polluting dry-run output
- Use EvalSymlinks + atomic rename for production binary replacement; include cross-FS fallback to handle moves across partitions
- Warn when binary is in package-manager paths; allow --force override rather than blocking to enable advanced workflows
- Delete dead helper functions immediately when their call sites are removed; verify zero references via grep before committing to prevent stale code
- Enforce consistent configuration option key naming across all providers using type-safe accessor helpers (getBoolOption, getStringOption); verify no camelCase remnants via grep before final validation
- When implementing validation constraints (e.g., model name prefixes), grep all test files for old values; update integration tests outside immediate scope before marking work complete
- Preserve event metadata (EventType, timestamps) when provider output lacks optional fields; never propagate zero-value event type regardless of missing nested data
- When removing provider-specific methods across multiple providers (e.g., parseClaudeStreamLine), delete all similar methods in single commit; prevents asymmetric cleanup breaking provider consistency
- Never commit doc.go files with minimal content in interface layer; mirror established patterns like cli/doc.go with architecture overviews and component descriptions (150+ lines)

## Test Conventions

- Use distinct file naming for unit vs integration tests: *_unit_test.go vs *_test.go; prevents error analysis tools from reporting incorrect file scopes
- Never hardcode OS-specific values in test assertions (usernames, paths, shell names); use `os/user.Current()` or mock dependencies for reproducible tests across environments
- Test context cancellation with context.WithCancel() and early ctx.Err() checks; verify operation fails with wrapped context.Canceled error within timeout
- Mock evaluators must have pre-configured results for every expression input; unconfigured expressions return zero value, which may bypass validation checks in evaluation pipelines
- Distinguish fixture path updates (allowed without review) from content changes (require explicit review); document rationale for content modifications in commit message
- Use _Integration suffix for tests requiring live agent execution or system dependencies; keep unit tests suffix-less in domain/application/infrastructure packages
- Separate provider output format validation tests into dedicated *_extract_test.go files; verify extraction patterns before session resume integration tests
- Document provider output format assumptions (JSON wrapper field names, text patterns) in code comments; validate assumptions with assertion-based tests before production
- Update all YAML fixtures when removing option support from code; synchronize fixtures with validation rule changes to prevent accidental bypass of removed validations
- Add //nolint:gosec to test code with controlled inputs when GOSEC flags false positives
- When replacing stub implementations with real methods, delete tests validating stub-only behavior (e.g., ReturnsNotImplemented assertions); stub tests become false positives after implementation replaces the stub
- For plugin lifecycle testing, use self-hosting pattern: detect AWF_PLUGIN env var in TestMain to serve subprocess plugin; eliminates need for separate plugin test binaries
- Write integration tests covering all command lifecycle paths (success, errors, state transitions) before marking implementation complete; include platform detection edge cases
- Extract repeated test assertion patterns (>5 duplicates) into table-driven or closure-based helpers to eliminate code duplication
- Extract HTTP server setup patterns from integration tests into helper functions; eliminate duplication across multiple test functions
- When flipping integration test assertions for newly-enabled features, transition from 'not configured' errors to provider-level implementation errors; verify assertions change state, not disappear
- Create separate test files for delegation patterns (*_delegation_test.go) to validate shared behavior independently from provider-specific unit tests
- When adding fields to internal state types (DisplayOutput, cache fields, etc.), write explicit tests verifying the field is NOT resolvable in template interpolation context; prevents accidental exposure of implementation details
- Add BenchmarkXX functions for new I/O processing components; measure throughput, memory allocation, and verify capacity constraints (1MB buffer, etc.) are respected
- Test event metadata persistence across all input variations for provider translation; include cases with missing optional nested fields to prevent silent metadata loss

## Review Standards

