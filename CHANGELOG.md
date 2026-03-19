# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **C064**: Retry configuration audit — 7 documentation-vs-implementation gap fixes
  - **Finding 1 (Critical)**: Guard `CalculateDelay` against `maxDelay=0` silently nullifying all retry delays — omitting `max_delay` no longer caps delays to zero
  - **Finding 2 (High)**: Default `multiplier` to 2.0 when omitted in YAML (was 0.0, degrading exponential backoff to constant delays)
  - **Finding 3 (Medium)**: Replaced deprecated `initial_delay_ms` with `initial_delay` duration strings in 3 doc files (`workflow-syntax.md`, `examples.md`, `plugins.md`)
  - **Finding 4 (Medium)**: Fixed `conversation-error.yaml` fixture using non-existent `delay:` field (renamed to `initial_delay:`)
  - **Finding 5 (Low)**: Surface duration parse errors in `mapRetry()` instead of silently defaulting to 0ms — invalid `initial_delay` / `max_delay` values now produce parse errors
  - **Finding 6 (Low)**: Added `RetryConfig.Validate()` covering `max_attempts >= 1`, valid backoff strategy, `jitter ∈ [0.0, 1.0]`, `multiplier >= 0`; wired into `Step.Validate()`
  - **Finding 7 (Info)**: Added missing `Jitter` and `RetryableExitCodes` fields to `DryRunRetry` struct and mapping in `dry_run_executor.go`
  - New unit and integration tests for maxDelay guard, multiplier default, duration error propagation, retry validation, and dry-run field mapping
- **C063**: Loop options audit — 6 documentation-vs-implementation discrepancies
  - **Finding 1**: Replaced ~56 lowercase loop variable references (`loop.item`, `loop.index`) with PascalCase (`loop.Item`, `loop.Index`) across 4 doc files (`workflow-syntax.md`, `loop.md`, `interpolation.md`, `examples.md`)
  - **Finding 1 — Code**: Added missing `index1` and `parent` keys to `makeLoopAccessor` function-call map in `template_resolver.go`; `parent` uses recursive `serializeLoopData` with nil guard
  - **Finding 2**: Replaced invalid arithmetic `max_iterations` example with working single-variable template resolution pattern
  - **Finding 3**: Added `%` (modulo) to operator detection in `parseMaxIterationsValue()` — was silently treating modulo expressions as literal strings
  - **Finding 4**: Updated while loop context table in `workflow-syntax.md` to include `Index1` and notes for `Item` (nil), `Last` (false), `Parent` (nested only)
  - **Finding 5**: Replaced misleading "Supports interpolation" with accurate expression syntax description for `while` and `break_when` fields
  - **Finding 6**: Removed redundant `LoopData` manual override in `ExecuteForEach` that dropped `Parent` chain — `buildContext(execCtx)` already produces correct context via `buildLoopDataChain`
  - New tests for `index1`/`parent` interpolation, modulo operator routing, and nested `for_each` parent chain preservation
- **C062**: Agent state options audit — 7 documentation and validation gaps
  - **Finding 1–2**: Documented `continue_from` and `inject_context` fields in Conversation Configuration table (`workflow-syntax.md`)
  - **Finding 3**: Fixed `strategy` default value from `sliding_window` to `-` (no context management when omitted)
  - **Finding 4**: Enumerated all valid strategy values (`sliding_window`, `summarize`, `truncate_middle`) in documentation
  - **Finding 5**: Documented conversation mode limitation — full history continuity only with `openai_compatible`; CLI providers execute turns independently
  - **Finding 6**: `ConversationConfig.Validate()` now rejects `continue_from` and `inject_context` with "not yet implemented" errors (previously silently ignored)
  - **Finding 7**: `ConversationConfig.Validate()` now rejects `summarize` and `truncate_middle` strategies with "not yet implemented; use sliding_window" error
  - Updated `conversation-multiturn.yaml` and `conversation-window.yaml` fixtures for new validation rules
- **C061**: Step options audit — 3 documentation/implementation gaps
  - `timeout` field documentation now reflects both integer seconds and Go duration string syntax (`"1m30s"`, `"500ms"`) across step and agent option tables
  - Removed redundant `context.WithTimeout` from `ShellExecutor.Execute` — application layer is now the single timeout owner via `prepareStepExecution`
  - `handleExecutionError` now evaluates transitions before `ContinueOnError` fallback, matching `handleNonZeroExit` behavior (ADR-001 transition priority contract)
  - Removed dead `Timeout` field from `ports.Command` struct and all assignment sites
  - New unit and integration tests covering transition priority on execution errors

### Added
- **F072**: Hugo documentation site with Doks theme
  - Full documentation site served from existing `docs/` via Hugo module mounts (zero file duplication)
  - Landing page with project positioning, feature highlights, install snippet, and CTAs
  - Blog section with placeholder post structure
  - GitHub Actions workflow for automated build and deploy to GitHub Pages
  - FlexSearch full-text documentation search via Doks built-in integration
  - Dark/light mode toggle via Doks theme
  - `make docs`, `make docs-serve`, `make docs-clean` targets for local development
- **C060**: `awf init` now creates `.awf/scripts/` directory with `example.sh`
  - Local init creates `.awf/scripts/` with `0o755` permissions and an executable `example.sh` demonstrating shebang-based execution
  - Global init (`--global`) creates `$XDG_CONFIG_HOME/awf/scripts/` alongside the existing prompts directory
  - `--force` flag overwrites existing example script
  - Handles partial initialization: creates scripts directory even if prompts already exists
  - Completes AWF directory convention symmetry with `.awf/workflows/`, `.awf/prompts/`, and `.awf/scripts/`

### Breaking Changes

- **F070**: Replaced `custom` agent provider with `openai_compatible` provider
  - **BREAKING**: `provider: custom` workflows will fail validation with migration guidance
  - **BREAKING**: `AgentConfig.Command` field removed from domain model and YAML mapping
  - New `openai_compatible` provider speaks Chat Completions API over HTTP (OpenAI, Ollama, vLLM, Groq, LM Studio)
  - Native multi-turn conversation support via `ExecuteConversation` (replaces "not implemented" error)
  - Accurate token tracking from API `usage` fields instead of character-based estimation
  - Structured HTTP error mapping: 401→auth, 429→rate limit, 5xx→server, timeout→deadline
  - API key resolution: `options.api_key` first, falls back to `OPENAI_API_KEY` env var
  - Response body limited to 10MB via `io.LimitReader`; API keys never logged or exposed in errors
  - Deleted: `custom_provider.go`, `custom_provider_test.go`, `custom_provider_unit_test.go`
  - **Migration (CLI tools)**: Use `type: step` with `command:` instead of `provider: custom`
  - **Migration (LLM APIs)**: Use `provider: openai_compatible` with `base_url` and `model` options

- **C059**: Removed unimplemented GitHub plugin stubs and dead HTTP code
  - Removed `github.set_project_status` operation stub (was never implemented, returned error at runtime)
  - Removed `RunHTTP()` method from HTTP client (was speculative functionality that never materialized)
  - GitHub plugin now provides 8 operations instead of 9
  - **Rationale**: Dead code removal following YAGNI principles — if these capabilities are needed later, they should be implemented properly with full design rather than resurrected from error-returning stubs
  - **Migration**: No action required; removed operations were non-functional and returned errors
  - **Impact**: Workflows attempting to use `github.set_project_status` will fail validation instead of runtime

- **C058**: Removed `ntfy` and `slack` notification backends — use `webhook` backend
  - The `webhook` backend is a superset: any HTTP POST target (ntfy, Slack, Discord, Teams, PagerDuty) works via URL + headers + body configuration
  - Removed fields: `ntfy_url` and `slack_webhook_url` from `.awf/config.yaml`
  - Removed files: `ntfy.go`, `ntfy_test.go`, `slack.go`, `slack_test.go`
  - **Migration (ntfy)** — use `http.request` for custom payload format:
    ```yaml
    # Before
    notify:
      type: operation
      operation: notify.send
      inputs:
        backend: ntfy
        title: "Build done"
        message: "Workflow complete"
        topic: my-topic

    # After (option A: webhook with fixed JSON structure)
    notify:
      type: operation
      operation: notify.send
      inputs:
        backend: webhook
        webhook_url: "https://ntfy.sh/my-topic"
        title: "Build done"
        message: "Workflow complete"

    # After (option B: http.request for full control)
    notify:
      type: operation
      operation: http.request
      inputs:
        url: "https://ntfy.sh/my-topic"
        method: POST
        body: '{"topic":"my-topic","title":"Build done","message":"Workflow complete"}'
    ```
  - **Migration (Slack)** — use `http.request` for Slack's JSON format:
    ```yaml
    # Before
    notify:
      type: operation
      operation: notify.send
      inputs:
        backend: slack
        title: "Build done"
        message: "Workflow complete"

    # After (http.request for Slack payload format)
    notify:
      type: operation
      operation: http.request
      inputs:
        url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
        method: POST
        headers:
          Content-Type: application/json
        body: '{"text":"*Build done*\nWorkflow complete"}'
    ```
  - **Risk**: Workflows using `backend: ntfy` or `backend: slack` will fail with unknown backend error

- **C057**: Removed deprecated `Tokens` field from `StepState` — use `TokensUsed`
  - Template interpolation: `{{states.step_name.Tokens}}` → `{{states.step_name.TokensUsed}}`
  - Expression conditions: `states.step_name.Tokens > 0` → `states.step_name.TokensUsed > 0`
  - **Migration**: Search workflow YAML files for `.Tokens` and replace with `.TokensUsed`
  - **Risk**: Unreplaced references silently evaluate to `0` (expr-lang zero-value semantics)

### Fixed
- **C061**: Fixed timeout documentation, removed redundant timeout handling, and aligned error transition priority
  - **Documentation**: `timeout` field on step states now documents both integer seconds and Go duration string syntax with examples (`"1m30s"`, `"500ms"`)
  - **Timeout Refactor**: Removed redundant `context.WithTimeout` that was applied in both application layer (`prepareStepExecution`) and infrastructure layer (`ShellExecutor.Execute`)
    - Timeout responsibility now owned solely by `ExecutionService.executeStep` via context-based cancellation
    - Removed `Timeout` field from `ports.Command` struct (was dead code, only set by infrastructure, never used)
    - Removed timeout parameter passing from `shell_executor.go`, `interactive_executor.go`, and `single_step.go`
    - Updated executor package documentation to clarify timeout ownership model
  - **Transition Priority Fix**: `handleExecutionError()` now evaluates transitions before `continue_on_error` fallback (matching `handleNonZeroExit()` behavior)
    - Execution errors (timeouts, command-not-found) now route through transition conditions first
    - `continue_on_error` only applies when no matching transition is found
    - Aligns with documented transition priority contract (ADR-001)
  - Added functional test in `tests/integration/execution/error_transition_priority_test.go` validating transition priority with execution errors and timeouts
- **B012**: Fixed 120 failing integration tests across 9 packages
  - Updated test assertions to match current production behavior after accumulated drift from F063–F072 and B005–B011
  - 8 root cause categories addressed: terminal failure assertions (C1), CLI exit codes (C2), JSON deserialization (C3), missing fixtures (C4), conversation test drift (C5), loop transitions (C6), cleanup meta-tests (C7), hint system output (C8)
  - No tests deleted — only assertions and expectations updated (NFR-001)
  - No production code changes — all fixes confined to `tests/integration/`
- **B011**: `{{.awf.scripts_dir}}` and `{{.awf.prompts_dir}}` in `command:` and `dir:` fields now resolve with local-before-global resolution
    - Previously, these template variables always resolved to global XDG paths, bypassing the local override that `script_file:` and `prompt_file:` fields already provided
    - Fix applied to all three executors: standard, single-step, and interactive mode
    - `InteractiveExecutor` now populates AWF map in interpolation context (was missing entirely)
    - `SingleStepExecutor` now populates AWF map in interpolation context (was missing entirely)
    - No change to existing behavior when no local file exists — falls back to global path
- **B010**: `awf validate` no longer rejects `{{.states.<step>.JSON.<field>}}` references as invalid
  - Added `JSON` entry to domain-layer `ValidStateProperties` map (was present only in runtime `pkg/interpolation`)
  - Added `json` → `JSON` casing normalization for actionable error hints
  - Root cause: F065 (JSON output format) added the entry to `pkg/interpolation` but missed the duplicate map in `internal/domain/workflow`
- **B009**: `script_file` now honors shebang lines for interpreter dispatch
  - Scripts with a shebang (`#!/usr/bin/env python3`, `#!/bin/bash`, etc.) are written to a temp file and executed directly, letting the kernel dispatch the correct interpreter
  - Scripts without a shebang fall back to `$SHELL -c` (backward compatible)
  - Temp files use `0o700` permissions and are cleaned up on success, failure, and cancellation
  - Inline `command` field behavior is unchanged
- **B008**: Ctrl+C no longer hangs during interactive input prompts
  - `PromptForInput()` and `PromptAction()` now use context-aware readline that respects cancellation
  - Pressing Ctrl+C during any input prompt terminates the process immediately instead of hanging indefinitely
  - No goroutine leaks: blocked reads are cleaned up at process exit
  - EOF handling (Ctrl+D) continues to work as before
- **B007**: Interactive and dry-run modes now respect `.awf/config.yaml` input defaults
  - `runInteractive()` and `runDryRun()` now load project config and merge inputs (same pattern as `runWorkflow()`)
  - Config values are pre-filled, reducing re-prompting for already-configured inputs
  - CLI flags still override config values (CLI wins)
  - Fixes regression where interactive input collection always prompted for all required inputs, ignoring config.yaml
  - Affects: interactive mode (`awf run --interactive`), dry-run mode (`awf run --dry-run`), and config-based input defaults

- **B006**: Shell commands no longer fail on Debian/Ubuntu where `/bin/sh` is `dash`
  - `ShellExecutor` now detects the user's preferred shell via `$SHELL` environment variable at construction time
  - Falls back to `/bin/sh` if `$SHELL` is unset, relative, or points to a non-existent binary
  - Bash-dependent workflow commands (arrays, `[[`, process substitution, brace expansion) now work automatically for users with bash
  - `WithShell(path)` functional option allows explicit shell override for testing and special cases
  - See [ADR-0012](docs/ADR/0012-runtime-shell-detection.md) for decision rationale and trade-offs
- **B005**: Local scripts and prompts now override global XDG paths when using `{{.awf.scripts_dir}}` or `{{.awf.prompts_dir}}`
  - `loadExternalFile()` now checks `<workflow_dir>/scripts/` (or `prompts/`) before falling back to the global XDG directory
  - New `resolveLocalOverGlobal()` helper detects XDG-prefixed paths and substitutes local equivalents when they exist
  - Applies to both `script_file` and `prompt_file` fields
  - **Workaround removal**: Users no longer need to use relative paths to get local-first resolution

- **B004**: Validator no longer forces dead-code transitions on parallel branch children
  - Parallel branch children (`step.Branches`) can now omit `on_success`/`on_failure` and `Transitions`
  - The execution engine discards these transitions, so requiring them was forcing dead code
  - Existing workflows with transitions on parallel children continue to validate (backward compatible)

### Changed

- **C051**: Fixed DIP violation in application layer
  - Removed direct `infrastructure/expression` import from `service.go`
  - `WorkflowService` now receives `ExpressionValidator` via constructor injection
  - Application layer has zero infrastructure dependencies (verified by C042 integration tests)

- **C049**: Refactored `InteractivePrompt` interface following Interface Segregation Principle
  - Split 11-method monolithic interface into 3 focused interfaces: `StepPresenter` (4 methods), `StatusPresenter` (4 methods), `UserInteraction` (3 methods)
  - Composite `InteractivePrompt` embeds all three for backward compatibility
  - Added compile-time interface satisfaction checks for `CLIPrompt`
  - AST-based integration test verifies ISP compliance
  - Zero breaking changes: all existing consumers compile unchanged

### Added
- **F071**: Structured Audit Trail for Workflow Executions
- **F070**: Replace Custom Agent Provider with OpenAI-Compatible Provider
- **F066**: Inline Error Terminal Shorthand for on_failure
  - `on_failure` now accepts an inline object `{message: "...", status: N}` in addition to the existing string form
  - Inline objects are synthesized into anonymous terminal states at parse time — no changes to execution engine
  - `message` field supports full template interpolation (`{{inputs.*}}`, `{{states.*}}`, `{{env.*}}`)
  - `status` defaults to exit code `1` when omitted
  - `awf validate` reports clear errors for missing or empty `message`
  - Works in parallel step branches
  - Full backward compatibility: existing string-form `on_failure` references are unchanged
- **F068**: Exit Code Based Transition Routing
  - Transitions with `when` conditions are now evaluated on non-zero exit paths, not just success paths
  - `states.<step>.ExitCode` supports all 6 numeric comparison operators (`==`, `!=`, `<`, `>`, `<=`, `>=`) in transition conditions
  - Transitions take priority over `continue_on_error` and `on_failure` when defined (first matching transition wins)
  - Exit code and output conditions can be mixed freely in the same workflow (e.g., step 1 routes by `ExitCode`, step 2 routes by `Output`)
  - Extracted shared `resolveNextStep` into package-level function to eliminate duplication between `ExecutionService` and `InteractiveExecutor`
  - Full backward compatibility: workflows without `ExitCode` transitions behave unchanged
- **F065**: Output Format for Agent Steps
  - New `output_format` field on agent steps: `json` (fence stripping + JSON validation) or `text` (fence stripping only)
  - Automatic markdown code fence stripping from agent output (outermost `` ```lang...``` `` removed)
  - Parsed JSON accessible via `{{.states.step.JSON.field}}` dot notation in templates
  - Invalid JSON with `output_format: json` fails the step with descriptive error (first 200 chars of malformed output)
  - Domain validation rejects unknown `output_format` values at `awf validate` time
  - New `pkg/output` package for reusable output processing utilities
  - Dry-run mode displays configured output format
  - Full backward compatibility: steps without `output_format` behave unchanged
- **F064**: Script File Loading for Step States
  - New `script_file` field on step states to reference external shell scripts instead of inline `command`
  - Mutual exclusivity validation: `command` and `script_file` cannot both be set on the same step
  - Path resolution supports relative (to workflow source dir), absolute, tilde-expansion, and `{{.awf.scripts_dir}}` template paths
  - Loaded script contents pass through Go template interpolation with full workflow context (inputs, step outputs, AWF variables)
  - Shared `loadExternalFile()` helper in application layer, reused by both prompt file and script file loading
  - Dry-run mode displays resolved script file path and loaded content
  - 1MB file size limit via `io.LimitReader` to prevent memory issues
  - YAML mapping for `script_file` field in workflow definitions

- **F063**: Prompt File Loading for Agent Steps

- **F058**: Built-in HTTP Operation
  - New `http` operation provider with `http.request` operation for declarative REST API calls
  - Supports HTTP methods: `GET`, `POST`, `PUT`, `DELETE`
  - Configurable timeout (default 30 seconds) and retryable status codes (429, 502, 503, etc.)
  - Response capture: status code, body, headers — accessible via template interpolation
  - Template interpolation in URL, headers, and body for dynamic requests
  - 1MB response body limit to prevent memory exhaustion
  - Integration with existing step-level retry mechanism for transient failures
  - Wired into CLI via `CompositeOperationProvider` alongside GitHub and Notify providers
  - Comprehensive integration tests covering 5 user stories (GET, POST/PUT/DELETE, response capture, timeout, retry)
  - Shared `pkg/httpx` package with `Client` and `Response` utilities for HTTP operations
  - Key Components:
    - `internal/infrastructure/http/provider.go` - HTTPOperationProvider implementing `ports.OperationProvider`
    - `internal/infrastructure/http/operations.go` - OperationSchema definition for `http.request`
    - `pkg/httpx/client.go` - HTTP client with configurable timeout and convenience methods (Get/Post/Put/Delete)
    - `pkg/httpx/response.go` - Response reading with bounded body support
    - `tests/integration/http_operation_test.go` - 27 integration tests validating all user stories
  - Refactored notify backends (ntfy, slack, webhook) to use shared `pkg/httpx.Client`, deleted `internal/infrastructure/notify/http.go`
  - Documentation:
    - `docs/user-guide/plugins.md` - HTTP Operation plugin section with examples
    - `docs/user-guide/workflow-syntax.md` - HTTP Operations reference section
    - `docs/user-guide/examples.md` - HTTP API integration example workflow
    - `README.md` - Updated feature list

- **F057**: Operation Interface and Registry Foundation
  - New `internal/domain/operation/` package with `Operation` interface defining `Name()`, `Execute()`, `Schema()` contract
  - `OperationRegistry` with thread-safe `Register`/`Unregister`/`Get`/`List` lifecycle management (sync.RWMutex)
  - Standalone `ValidateInputs` function for runtime input validation against `OperationSchema` (required fields, type correctness, default values)
  - Type validation for `string`, `integer`, `boolean`, `array`, `object` with JSON float64→int coercion
  - Registry implements `ports.OperationProvider` for seamless `ExecutionService` integration
  - Sentinel errors: `ErrOperationAlreadyRegistered`, `ErrOperationNotFound`, `ErrInvalidOperation`
  - Updated `go-arch-lint` configuration with `domain-operation` component enforcing domain purity
  - Comprehensive unit and integration test suites covering all 4 user stories

- **F056**: Workflow Completion Notification Plugin
  - New `notify` operation provider with `notify.send` operation dispatching to 4 backends: `desktop`, `ntfy`, `slack`, `webhook`
  - Desktop backend uses OS-native notifications (`notify-send` on Linux, `osascript` on macOS)
  - HTTP backends (ntfy, slack, webhook) share a 10-second timeout to prevent workflow stalls
  - `CompositeOperationProvider` enables multiple built-in providers to coexist (github + notify)
  - Configurable via `.awf/config.yaml`: `ntfy_url`, `slack_webhook_url`, `default_backend`
  - All inputs support AWF template interpolation (`{{workflow.name}}`, `{{workflow.duration}}`, etc.)
  - Webhook backend sends generic JSON POST for integration with Discord, Teams, PagerDuty, etc.
  - Wired into CLI via `CompositeOperationProvider` wrapping both GitHub and Notify providers in `run.go`

- **F054**: GitHub CLI plugin for declarative operations
  - New `github` operation provider with 8 operation types: `get_issue`, `get_pr`, `create_pr`, `create_issue`, `add_labels`, `list_comments`, `add_comment`, `batch`
  - Batch executor with 3 strategies: `all_succeed` (fail-fast), `any_succeed` (first-wins), `best_effort` (run-all)
  - Configurable concurrency limiting via semaphore pattern (default: 3)
  - Authentication fallback chain: gh CLI → GITHUB_TOKEN → structured error with remediation hints
  - Auto-detection of repository from git remote when `repo` parameter omitted
  - Field selection via `fields` parameter to limit output data
  - Structured outputs accessible via `{{states.step_name.output.field}}` interpolation
  - Schema-driven input validation for all operation types
  - Wired into CLI via `GitHubOperationProvider` registered in `run.go`

- **C056**: Add doc.go to 9 Key Infrastructure and Interface Packages
  - Created `doc.go` files for 9 undocumented packages: `agents`, `executor`, `expression`, `logger`, `plugin`, `repository`, `store`, `cli`, `cli/ui`
  - Three documentation tiers scaled by package complexity: concise (~20-30 lines), medium (~40-50 lines), comprehensive (~60-80 lines)
  - Removed 5 stale package comments from `agents/helpers.go` and 4 `plugin/` files to consolidate documentation per Go convention
  - All 17 key packages now have `doc.go` files (previously 8 of 17)
  - AST-based integration test validates all 9 new files follow Go doc conventions
  - `go doc ./internal/infrastructure/<package>` now displays meaningful output for every infrastructure package

- **C054**: Increased Application Layer Test Coverage to 87% (target 85%)
  - Added comprehensive tests for 9 under-covered functions: `resolveOperationValue`, `resolveNextStep`, `classifyErrorType`, `executeFromStep`, `executePluginOperation`, `ValidateWorkflow`, `ExecuteSingleStep`, `ExecuteStep`, and `executeLoopStep`
  - Created 5 new test files with 300+ LOC: `execution_service_resolve_test.go`, `execution_service_transitions_test.go`, `execution_service_errors_test.go`, `execution_service_resume_test.go`, `execution_service_plugin_test.go`
  - Extended 3 existing test files with 200+ LOC: `service_validator_injection_test.go`, `single_step_test.go`, `execution_service_loop_test.go`
  - All new tests follow ServiceTestHarness builder pattern and table-driven conventions
  - No existing tests modified (additive-only changes)
  - All tests pass with zero race condition warnings

- **C051**: Architecture enforcement with go-arch-lint
  - Added `.go-arch-lint.yml` configuration defining hexagonal architecture constraints
  - New `make lint-arch` target replaces `check-domain` with comprehensive AST-based validation
  - New `make lint-arch-map` target shows component-to-package mapping
  - Added `.github/workflows/quality.yml` for CI architecture enforcement

- **C048**: Actionable error message hints
  - Error messages now include contextual suggestions: "Did you mean?", line/column references, required inputs, and resolution steps
  - Covers file not found, YAML syntax errors, invalid state references, missing inputs, and command failures
  - Added `--no-hints` global flag to suppress suggestions
  - JSON output includes `hints` array

- **C047**: Structured Error Codes Taxonomy
  - Implemented hierarchical error code system with `CATEGORY.SUBCATEGORY.SPECIFIC` format (e.g., `USER.INPUT.MISSING_FILE`, `WORKFLOW.VALIDATION.CYCLE_DETECTED`)
  - Created `StructuredError` domain type with Code, Message, Details, Cause, and Timestamp fields
  - Defined 14 error codes mapped to existing exit codes: 1=USER.*, 2=WORKFLOW.*, 3=EXECUTION.*, 4=SYSTEM.*
  - Added `ErrorFormatter` port interface with JSON and human-readable adapters in infrastructure layer
  - Implemented `awf error <code>` CLI command for error code lookup with descriptions, resolution hints, and related codes
  - Enhanced `categorizeError()` with two-phase detection: checks for `StructuredError` via `errors.As()` first, falls back to legacy string matching
  - Extended `WriteError()` in UI layer to detect and format `StructuredError` instances with error codes
  - JSON output mode produces structured error objects with code, message, details, and timestamp fields
  - Human-readable output includes `[ERROR_CODE]` prefix for reference and programmatic parsing
  - Error catalog in domain layer (`catalog.go`) maps codes to descriptions, resolutions, and related error codes
  - Package documentation added to `internal/domain/errors/` and `internal/infrastructure/errors/`
  - Comprehensive integration tests in `tests/integration/c047_error_codes_test.go` covering AC1-AC4 acceptance criteria
  - Backward compatible: existing errors continue to work via string matching fallback during migration period
  - Domain layer imports only stdlib (errors, fmt, time, strings) maintaining hexagonal architecture purity
  - Files added: 15 new files (domain entities, ports, adapters, tests), 8 files modified (CLI integration, application service)
  - Impact: Enables programmatic error handling in CI/CD pipelines, searchable error documentation, and consistent error messages across output formats

### Fixed
- **B010**: `awf validate` no longer rejects `{{.states.<step>.JSON.<field>}}` references as invalid
  - Added `JSON` entry to domain-layer `ValidStateProperties` map (was present only in runtime `pkg/interpolation`)
  - Added `json` → `JSON` casing normalization for actionable error hints
  - Root cause: F065 (JSON output format) added the entry to `pkg/interpolation` but missed the duplicate map in `internal/domain/workflow`
- **B009**: `script_file` now honors shebang lines for interpreter dispatch
  - Scripts with a shebang (`#!/usr/bin/env python3`, `#!/bin/bash`, etc.) are written to a temp file and executed directly, letting the kernel dispatch the correct interpreter
  - Scripts without a shebang fall back to `$SHELL -c` (backward compatible)
  - Temp files use `0o700` permissions and are cleaned up on success, failure, and cancellation
  - Inline `command` field behavior is unchanged
- **B007**: Interactive input prompt and dry-run mode now respect `.awf/config.yaml` input defaults
  - `runInteractive()` and `runDryRun()` now load project config and merge inputs (same pattern as `runWorkflow()`)
  - Config values are pre-filled, reducing re-prompting for already-configured inputs
  - CLI flags still override config values (CLI wins)
  - Fixes regression where interactive input collection always prompted for all required inputs, ignoring config.yaml
  - Affects: interactive mode (`awf run --interactive`), dry-run mode (`awf run --dry-run`), and config-based input defaults
- **B006**: Shell commands no longer fail on Debian/Ubuntu where `/bin/sh` is `dash`
  - `ShellExecutor` now detects the user's preferred shell via `$SHELL` environment variable at construction time
  - Falls back to `/bin/sh` if `$SHELL` is unset, relative, or points to a non-existent binary
  - Bash-dependent workflow commands (arrays, `[[`, process substitution, brace expansion) now work automatically for users with bash
  - `WithShell(path)` functional option allows explicit shell override for testing and special cases
  - See [ADR-0012](docs/ADR/0012-runtime-shell-detection.md) for decision rationale and trade-offs
- **B005**: Local scripts and prompts now override global XDG paths when using `{{.awf.scripts_dir}}` or `{{.awf.prompts_dir}}`
  - `loadExternalFile()` now checks `<workflow_dir>/scripts/` (or `prompts/`) before falling back to the global XDG directory
  - New `resolveLocalOverGlobal()` helper detects XDG-prefixed paths and substitutes local equivalents when they exist
  - Applies to both `script_file` and `prompt_file` fields
  - **Workaround removal**: Users no longer need to use relative paths to get local-first resolution

- **B003**: Fixed while loop `break_when` condition not evaluating correctly
  - Root cause: Integration tests used `simpleExpressionEvaluator` mock with outdated lowercase keys (`output`, `exit_code`) and hardcoded value matching
  - Solution: Replaced mock with real `expression.NewExprEvaluator()` across all integration tests, updated expressions to PascalCase per B001 convention
  - Impact: `break_when` conditions now work with arbitrary Output values, not just hardcoded "ready"/"stop"
  - Files: 57+ evaluator replacements in tests/integration/ (loop_test.go, loop_dynamic_test.go, loop_json_serialization_test.go, loop_transitions_test.go, subworkflow_functional_test.go)
  - Removed deprecated test helpers: `simpleExpressionEvaluator`, `simpleExpressionEvaluatorT009`

- **C041**: Removed TODO(#150) comment from MockLogger.WithContext implementation
  - Replaced stub implementation with full context accumulation logic
  - Method now properly merges context fields with log message fields
  - Completes technical debt cleanup for issue #150

- **C052**: Fixed flaky `TestRun_SetsProcessGroup` orphan process detection tests
  - Removed non-deterministic `pgrep -f "sleep 10"` assertions from three process group test functions
  - Root cause: `pgrep` searches all system processes and matches unrelated commands from parallel CI jobs
  - Solution: Removed cleanup verification blocks (6 lines from TestRun_SetsProcessGroup, 5 lines each from EdgeCases and ErrorHandling)
  - Rationale: Context cancellation is already deterministically verified via `errors.Is(err, context.Canceled)`; process group signal delivery is an OS guarantee, not application logic
  - Removed unused `cleanupCheckDelay` field and associated `time.Sleep` calls
  - Removed `os/exec` import (only used for pgrep commands)
  - Impact: Tests now pass reliably under race detector and in shared CI environments with concurrent job execution
  - Verification: All 10 subtests pass in 0.417s without flaky failures; no other test files affected

- **F051**: Multi-Turn Conversation Workflow Failures
  - Fixed empty prompt bug preventing conversations from continuing past first turn
  - Consolidated duplicate code in ConversationManager by replacing inline logic with existing helper methods
  - Implemented `executeConversationStep` in ExecutionService to properly delegate to ConversationManager
  - Multi-turn conversations now resolve prompts correctly for subsequent turns using configured prompt template
  - Added comprehensive unit tests for multi-turn prompt resolution and conversation step execution
  - Added integration test suite covering multi-turn workflows, stop conditions, and backward compatibility
  - Fixes: conversation-simple, conversation-multiturn, conversation-max-turns, conversation-error workflows
  - Impact: 391 lines added, 102 lines removed across 4 application layer files

### Chores

- **C053**: Clean up 50+ problematic test skips across codebase
  - Removed 12 commented-out `t.Skip()` lines from T009 tests
  - Removed 19 dead `defer recover()` blocks from graphviz tests
  - Removed 7 dead conditional skips for existing directories/fixtures
  - Deleted 7 untestable/duplicate test stubs
  - Deleted 8 feature placeholder tests
  - Implemented nil-guard checks in `HandleExecutionError` and `HandleNonZeroExit`
  - Converted 1 unconditional skip to `testing.Short()` pattern
- **C055**: Remove stale `checkUnknownKeys` WARNING comments
  - Removed 4 WARNING comments from `loader_test.go` that incorrectly stated `checkUnknownKeys` is not implemented
  - Inverted `TestWarningComments_IssueTracking_Integration` assertion to verify zero stale comments remain
  - Updated `c043_verify.sh` bash script to expect zero WARNING comment matches
  - Zero production code changes: comment-only cleanup with coordinated test updates

### Breaking Changes

- **[B001] Expression context normalization to PascalCase**
  - Expression evaluator now uses PascalCase keys matching Go struct fields
  - **State fields:** `output` → `Output`, `exit_code` → `ExitCode`, `stderr` → `Stderr`, `status` → `Status`
  - **Added state fields:** `Response`, `Tokens` for agent steps
  - **Workflow fields:** `id` → `ID`, `name` → `Name`, `current_state` → `CurrentState`
  - **Added workflow field:** `Duration` (method call)
  - **Error fields:** `message` → `Message`, `state` → `State`, `exit_code` → `ExitCode`, `type` → `Type`
  - **Context fields:** `working_dir` → `WorkingDir`, `user` → `User`, `hostname` → `Hostname`
  - **Added namespaces:** `loop.*` (Index, Index1, Item, First, Last, Length, Parent), `error.*`, `context.*`
  - **Migration:** Update all expression conditions (`when`, `break_when`, `while`, `until`) to use PascalCase
  - Validation errors provide exact suggestions for incorrect casing
  - **Example:** `break_when: 'states.check.exit_code != 0'` → `break_when: 'states.check.ExitCode != 0'`
  - Affects: Expression evaluation in `when` clauses, loop conditions, break conditions

- **[F050] Standardize state property casing to uppercase**
  - State property references in templates now require uppercase: `.Output`, `.Stderr`, `.ExitCode`, `.Status`
  - Previous lowercase syntax (`.output`, `.stderr`, etc.) was never functional with Go templates
  - **Migration:** Update all workflow YAML files to use uppercase property names
  - Validation errors now include suggested uppercase corrections
  - Affects: Template interpolation, workflow validation, all state references

### Added
- **F071**: Structured Audit Trail for Workflow Executions
- **F070**: Replace Custom Agent Provider with OpenAI-Compatible Provider
- **F066**: Inline Error Terminal Shorthand for on_failure
  - `on_failure` accepts inline object `{message: "...", status: N}` as shorthand for named terminal states
  - Synthesized at parse time; `message` supports interpolation; `status` defaults to `1`
- **F068**: Exit Code Based Transition Routing
- **F065**: Output Format for Agent Steps
- **F063**: Prompt File Loading for Agent Steps
- **C056**: Add doc.go to 9 Key Infrastructure and Interface Packages
  - Created `doc.go` files for 9 undocumented packages: `agents`, `executor`, `expression`, `logger`, `plugin`, `repository`, `store`, `cli`, `cli/ui`
  - Three documentation tiers scaled by package complexity: concise (~20-30 lines), medium (~40-50 lines), comprehensive (~60-80 lines)
  - Removed 5 stale package comments to consolidate documentation per Go convention
  - All 17 key packages now have `doc.go` files (previously 8 of 17)

- **F052**: Renovate Dependency Management
  - Automated dependency updates via Renovate bot for Go modules and GitHub Actions
  - Weekly update schedule (Sunday early morning UTC) to minimize workflow disruption
  - Automerge enabled for minor and patch updates with passing CI tests
  - Manual review required for major version updates with breaking changes
  - Configuration in `renovate.json` with `config:recommended` base preset
  - Comprehensive documentation in `docs/development/dependency-management.md`
  - PR limit of 5 concurrent updates to prevent overwhelming maintainers
  - Dependency grouping by type to reduce noise and improve update coherence

- **F053**: Go Quality Tooling Modernization
  - Added 7 modern linters to golangci-lint configuration:
    - gofumpt: Stricter formatting than gofmt
    - gocognit: Cognitive complexity thresholds (limit 15)
    - gocritic: Advanced static analysis (diagnostic, style, performance checks)
    - exhaustive: Enum switch exhaustiveness verification
    - noctx: HTTP requests require context propagation
    - prealloc: Slice capacity optimization hints
    - wrapcheck: Error wrapping discipline at package boundaries
  - Makefile now uses gofumpt instead of gofmt for stricter formatting
  - New `make quality` target runs all quality checks (lint+fmt+vet+test)
  - New `make fix` target auto-fixes linter issues
  - Comprehensive code quality documentation in `docs/development/code-quality.md`
  - Updated CONTRIBUTING.md with code quality requirements for PRs

### Changed

- **C050**: PluginManager Interface ISP Compliance Review (Reaffirmed)
  - Reaffirmed C037 decision: Keep PluginManager unified (7 methods) after full ISP analysis
  - Single consumer (PluginService) uses all 7 methods with cross-concern coupling in DisablePlugin
  - Added AST-based structural tests verifying method counts for all 8 plugin interfaces
  - Created integration test suite in `tests/integration/c050_isp_compliance_test.go`
  - Removed dead code: `ErrVersionNotImplemented` (version.go), `ErrLoaderNotImplemented` (loader.go)
  - Updated architecture.md with current 7-method PluginManager definition and Interface Design Decisions section
  - All tests pass with race detector (`make test-race`)
  - Impact: +570 LOC (tests, documentation), -20 LOC (dead error variables), net +550 LOC

- **C046**: Replace context.TODO() with context.Background() in Test Files
  - Replaced `context.TODO()` with `context.Background()` in `internal/testutil/mocks_cli_executor_test.go:306` and `internal/infrastructure/agents/cli_executor_test.go:254`
  - Improves semantic accuracy by using the appropriate context function for test scenarios
  - `context.Background()` correctly indicates an intentional empty context for testing, not a pending implementation
  - Aligns with Go documentation: `context.Background()` is the recommended choice for tests, main functions, and initialization
  - Zero production impact: both functions return identical `context.emptyCtx` values at runtime
  - Follows established codebase pattern: 127+ test functions already use `context.Background()`

- **C045**: Add Package-Level Documentation to Core Packages
  - Created `internal/domain/workflow/doc.go` documenting workflow entities, state machine, and 65+ types across 12 categories (Workflow, Step, State, Context, Configuration, Results, Hooks, Templates, Conversations, Validation, Errors, Constants)
  - Created `internal/domain/ports/doc.go` cataloging 26 port interfaces grouped by architectural concern (Repository, Execution, Agent, Plugin, Interactive, Logging, History)
  - Created `internal/application/doc.go` describing 19+ application services grouped by responsibility (Core Orchestration, Specialized Executors, Supporting Services, Utilities)
  - Removed duplicate package comment from `internal/application/plugin_service.go` to comply with Go's one-package-comment convention
  - Added integration test suite in `tests/integration/c045_package_documentation_test.go` using `go/parser` to verify doc.go format compliance
  - Improved developer onboarding with `go doc ./internal/domain/workflow`, `go doc ./internal/domain/ports`, `go doc ./internal/application`
  - All documentation follows existing style from `internal/testutil/doc.go` and `internal/infrastructure/config/doc.go`

- **C044**: Fix Test Layer Purity Violations in Template Service Tests
  - Moved `TemplateNotFoundError` from infrastructure layer to `internal/domain/workflow/template_errors.go`
  - Created centralized `MockTemplateRepository` in `internal/testutil/mocks.go` with thread-safe patterns
  - Migrated `template_service_test.go` and `template_service_helpers_test.go` to use testutil mocks
  - Removed all `internal/infrastructure/repository` imports from application layer test files
  - Deleted 88 lines of duplicate local mock implementations (`mockTemplateRepository`, `mockLogger`)
  - Added type alias in infrastructure layer for backward compatibility: `type TemplateNotFoundError = workflow.TemplateNotFoundError`
  - Created architecture guard test in `template_service_architecture_test.go` to prevent regression
  - Application layer tests now depend only on domain ports and testutil, enforcing hexagonal architecture
  - Added comprehensive unit tests for `TemplateNotFoundError` in `internal/domain/workflow/template_errors_test.go`
  - Added unit tests for `MockTemplateRepository` in `internal/testutil/mocks_template_repository_test.go`
  - Added integration tests in `tests/integration/c044_template_test_layer_purity_test.go` validating mock functionality and import purity
  - All tests pass with race detector (`make test-race`)
  - Impact: +370 LOC (mocks, tests, guard), -90 LOC (duplicate local mocks, infrastructure imports), net +280 LOC

- **C043**: Code Quality Quick Wins
  - Fixed documentation inconsistency in `docs/user-guide/commands.md` where status filter value incorrectly listed "interrupted" instead of "cancelled" to match implementation (`StatusCancelled` constant)
  - Added GitHub issue #169 tracking references to four WARNING comments documenting unimplemented `checkUnknownKeys` feature in `internal/infrastructure/config/loader_test.go`
  - Updated comments at lines 487, 512, 627, and 729 to include "TODO(#169)" for technical debt tracking
  - Improved traceability of known limitations and future enhancements

- **C042**: Fix DIP Violations in Application Layer
  - Moved `ExpressionEvaluator` interface from application layer to `internal/domain/ports/expression_evaluator.go`
  - Created infrastructure adapter in `internal/infrastructure/expression/expr_evaluator.go` implementing the port
  - Added `EvaluateInt()` method for arithmetic expression evaluation alongside existing `EvaluateBool()`
  - Removed direct `expr-lang/expr` import from `internal/application/loop_executor.go`
  - Removed local `ExpressionEvaluator` interface definition from `internal/application/execution_service.go`
  - Updated `LoopExecutor` to accept `ExpressionEvaluator` via constructor injection
  - Created `MockExpressionEvaluator` in `internal/testutil/mocks.go` with thread-safe patterns
  - Deleted `evaluateArithmeticExpression` method from `loop_executor.go` (30 lines removed)
  - Application layer now has zero infrastructure imports, restoring hexagonal architecture compliance
  - Added compile-time interface verification in adapter: `var _ ports.ExpressionEvaluator = (*ExprEvaluator)(nil)`
  - All existing tests pass without modification; 100% test coverage maintained
  - Architecture verification test added in `tests/integration/c042_dip_compliance_test.go`

- **C041**: Mock Enhancement: Context Field Merging and SetResult Deprecation Migration
  - `testutil.MockLogger.WithContext()` now properly accumulates context fields following production logger patterns
  - Context fields are merged with message-level fields when calling Debug/Info/Warn/Error methods
  - Implemented thread-safe context storage using sync.RWMutex for concurrent access
  - Migrated 14 deprecated `SetResult()` calls to `SetCommandResult()` across test files:
    - 11 calls in `internal/testutil/mocks_test.go`
    - 3 calls in `pkg/plugin/sdk/testing_test.go`
  - Added `SetCommandResult()` method to `MockOperationProvider` following delegation pattern
  - Updated 4 documentation examples in `internal/testutil/doc.go` to use non-deprecated API
  - Deprecated methods remain as thin wrappers for backward compatibility (marked with TODO(#150))
  - All tests pass race detector validation confirming thread safety

- **C036**: Split PluginStateStore interface to fix ISP violation
  - Created `PluginStore` interface for persistence operations (Save, Load, GetState, ListDisabled)
  - Created `PluginConfig` interface for configuration management (SetEnabled, IsEnabled, GetConfig, SetConfig)
  - Maintained backward compatibility via interface embedding in `PluginStateStore`
  - No breaking changes to existing consumer code in application, infrastructure, or CLI layers
  - Enables future consumers to depend on narrower contracts for improved testability
  - Added compile-time verification tests for both new interfaces

- **C027**: Improve Application Layer Test Coverage to 80%+
  - **Phase 1 - Zero Coverage Setters (Quick Wins):**
    - ExecutionService setters: `SetOperationProvider`, `SetAgentRegistry`, `SetEvaluator`, `SetConversationManager` with nil safety tests
    - InteractiveExecutor setters: `SetTemplateService`, `SetOutputWriters` (2 methods × 2 scenarios = 4 tests)
    - DryRunExecutor setters: `SetTemplateService` (1 method × 2 scenarios = 2 tests)
  - **Phase 2 - Error Handlers:**
    - `HandleMaxIterationFailure` error handler in execution loop with 6 comprehensive test scenarios
    - Plugin lifecycle error paths: `ShutdownPlugin`, `EnablePlugin`, `DisablePlugin`, `SetPluginConfig` (8 error injection tests)
  - **Phase 3 - Complex Business Logic:**
    - ExecutionService resolve functions: `resolveNextStep`, `resolveStepCommand`, `resolveOperationValue` with full recursion support (31+ test cases)
    - InteractiveExecutor loop functions: `executeLoopStep` with for-each/while loop state management (24+ test scenarios)
  - **Test Infrastructure Additions:**
    - Implemented `MockOperationProvider` and `MockOperation` in `execution_service_specialized_mocks_test.go`
    - Extended `ExecutionServiceBuilder` with `WithEvaluator` method for expression evaluator dependency injection
    - Created comprehensive integration test suite in `tests/integration/c027_application_test_coverage_test.go` with 18 test functions
  - **Template Resolver Enhancements:**
    - Added function-style accessors: `{{inputs.*}}`, `{{states.*}}`, `{{workflow.*}}`, `{{env.*}}`
    - Fixed nested map/response path resolution for agent step responses
    - Enhanced type safety with proper error handling for missing keys
  - **Coverage Metrics:**
    - Application layer coverage increased from 79.2% to 80.5% (exceeds project threshold)
    - Added 2,112 lines of comprehensive tests across 12 files (10 test files, 2 source files)
    - Files modified: execution_service.go (+6 LOC SetEvaluator), template_resolver.go (+46 LOC function accessors)
  - **Architecture Compliance:**
    - All tests follow established patterns: table-driven with testify, ServiceTestHarness, testutil mocks
    - Zero infrastructure imports in application tests (maintains C038 architectural purity)
    - All tests pass race detector validation (`make test-race`)
  - **Integration Test Coverage:**
    - Coverage threshold validation (≥80%)
    - Template accessor end-to-end tests (happy path + edge cases)
    - SetEvaluator integration verification
    - Architecture compliance checks (no infrastructure imports)
    - Acceptance criteria validation (5 ACs verified)

- **C038**: Fix Application Test Layer Purity
  - Eliminated infrastructure layer imports from application test files to enforce hexagonal architecture
  - Created centralized `MockAgentRegistry` and `MockAgentProvider` in `internal/testutil/mocks.go`
  - Both mocks implement thread-safe patterns (sync.RWMutex) matching C037 standards
  - Migrated 4 test files to use centralized mocks: `conversation_manager_test.go`, `agent_step_test.go`, `execution_service_conversation_test.go`, `execution_service_hooks_test.go`
  - Removed all local mock implementations (mockAgentProvider, mockAgentRegistry wrappers)
  - Application layer tests now depend only on domain ports interfaces, not concrete infrastructure
  - All tests pass with race detector (`make test-race`)
  - Impact: +220 LOC centralized mocks, -170 LOC duplicate local mocks, net +50 LOC
  - Reference: ADR-001 (separate mocks), ADR-002 (builder already interface-typed), ADR-003 (delete local mocks)

- **C037**: PluginManager Interface ISP Compliance Review
  - Architectural decision: Keep PluginManager interface unified (7 methods)
  - Analysis confirmed high cohesion - single consumer (PluginService) uses all methods
  - Cross-concern coupling: DisablePlugin uses Get() before Shutdown()
  - Added cohesion analysis documentation test in `internal/domain/ports/plugin_test.go`
  - Removed duplicate mockPluginManager implementations (consolidated to single source)
  - Fixed unused `shutdownErrors` variable in `RPCPluginManager.ShutdownAll`
  - Standardized compile-time interface check to 1-line pattern (was 9 lines)
  - Impact: -88 LOC of duplicate/dead code, +50 LOC documentation
  - Reference: ADR-001 (keep unified), ADR-002 (cohesion analysis pattern)

- **C031** Implemented plugin manifest validation to replace ErrNotImplemented stub
  - Replaced `Manifest.Validate()` stub with comprehensive field validation logic
  - Name validation: enforces `^[a-z][a-z0-9-]*$` pattern (lowercase, starts with letter, alphanumeric + hyphens)
  - Version validation: ensures non-empty version string
  - AWFVersion validation: requires non-empty AWF version constraint
  - Capabilities validation: verifies all capabilities against whitelist (operations, commands, validators)
  - Config field validation: validates field types (string, integer, boolean), enum constraints (string only), default type matching
  - Added 15 table-driven unit test functions with 130+ test cases covering valid/invalid names, versions, capabilities, and config fields
  - Added 18 integration tests verifying parser-to-validation workflow with realistic YAML fixtures
  - Achieved 99% test coverage for manifest.go validation logic (exceeds 80% requirement)
  - Breaking change: `Manifest.Validate()` now returns `nil` for valid manifests instead of `ErrNotImplemented`
  - All validation errors provide descriptive messages indicating which field failed and why

- **C030** Reduced test skip count by 84% (823 → 128 skipped tests)
  - Converted 400+ integration test runtime skips to `//go:build integration` tags
  - Removed 250+ obsolete conditional skip guards from plugin infrastructure tests (state store, loader, registry, version)
  - Deleted validation stub tests from manifest_test.go after C029 implementation completion
  - Created standardized skip helper functions in `tests/integration/test_helpers_skip_test.go`
  - Added `test-external` Makefile target for tests requiring external CLI dependencies
  - Fixed `test-integration` target to run all integration tests (was only running cli/ subdirectory)
  - Created comprehensive testing documentation in `docs/development/testing.md`:
    - Build tag usage guide (integration, external, slow, !short)
    - Skip helper function patterns for environment checks
    - Formal skip policy with categorization and message format requirements
    - Verification commands for skip count and format validation
  - Created audit script `scripts/audit-skips.sh` for categorizing skip patterns
  - Removed redundant `testing.Short()` checks from files with `//go:build integration` tags
  - All remaining skips follow standardized format with category prefixes (ENVIRONMENT, PLATFORM, PERMISSION)

- **C022** Fixed DIP violation in ExecutionService AgentRegistry dependency
  - Changed `agentRegistry` field from concrete `*agents.AgentRegistry` to `ports.AgentRegistry` interface
  - Updated `SetAgentRegistry()` setter to accept interface type
  - Removed `internal/infrastructure/agents` import from `execution_service.go`
  - Added `Has()` method to `agents.AgentRegistry` to complete interface implementation
  - Updated `ExecutionServiceBuilder` in testutil to use interface type
  - Restored hexagonal architecture compliance: application layer now depends only on domain ports

- **C018** Improved test coverage in pkg/ layer
  - Added tests for LoopData.Parent field (nested loop support)
  - Added tests for StepStateData.Response/Tokens fields (agent step data)
  - Added tests for expression namespaces (loop.*, context.*, error.*)
  - Fixed test data mutation in resolver_test.go
  - Completed TestRetryer_LogsAttempts assertions

- **C028** Improved CLI test coverage from 77.6% to 80%+ by adding targeted integration tests
  - Added comprehensive integration tests for `runValidate` (12.4% → covered): All output formats (text, JSON, table, quiet), workflow not found errors, validation error formatting, template reference validation
  - Added integration tests for `runDiagram` (34.6% → covered): Invalid direction handling, workflow not found errors, stdout DOT output, file output modes, graphviz availability checks
  - Added integration tests for `runStatus` (47.4% → covered): State loading, all format variations (text, JSON, quiet), execution not found errors
  - Added integration tests for plugin commands (55% → 80%+): Enable/disable lifecycle, plugin not found errors, environment-based plugin discovery, state persistence
  - Added unit tests for `CheckMigration` (27.8% → covered): Legacy directory detection, XDG migration notice display, singleton suppression pattern
  - Created 5 new test files: `tests/integration/cli/validate_coverage_test.go`, `tests/integration/cli/diagram_coverage_test.go`, `tests/integration/cli/status_coverage_test.go`, `tests/integration/cli/plugin_cmd_coverage_test.go`, `internal/interfaces/cli/migration_coverage_test.go`
  - Deleted obsolete `validate_coverage_test.go` containing 10 assertion-free tests providing false coverage confidence
  - Tests use `//go:build integration` tags for proper test categorization and isolation

- **C017** Reorganized CLI tests to separate unit and integration concerns
  - Moved 280 integration-style tests from `internal/interfaces/cli/` to `tests/integration/cli/` with `//go:build integration` tags
  - Retained 176 unit tests in-place focused on flag parsing, help text, and command registration
  - Created shared workflow fixtures in `internal/testutil/cli_fixtures.go` to eliminate duplication across test files
  - Fixed thread-safety issues by replacing `os.Chdir` with `t.TempDir()` and `t.Setenv()` patterns
  - Added `AWF_CONFIG_PATH` environment variable to override config file location (enables thread-safe config tests)
  - Updated Makefile targets: `make test-unit` excludes integration tests, `make test-integration` includes CLI integration tests
  - Consolidated duplicate test helpers (`setupWorkflows`, `setupWorkflow`, `setTestEnv`) into shared utilities
  - Improved test maintainability with clear boundaries: interface layer unit tests verify only the interface contract
  - Faster feedback loops: unit tests run without integration overhead (workflow parsing, state management, shell execution)

- **C016** Added comprehensive unit tests for input validation and state persistence layers
  - Input validation tests: Pattern matching, enum constraints, min/max bounds, file existence checks
  - SQLite History Store tests (2,082 lines): CRUD operations, error paths, edge cases, nil handling, concurrent access (20-goroutine stress tests)
  - JSON Store corruption recovery tests: Empty files, whitespace-only, truncated JSON, invalid UTF-8, null bytes, excessive nesting, duplicate keys, mixed line endings, very large files (10MB), trailing garbage
  - JSON Store concurrent access tests: 20-goroutine concurrent read validation
  - Interface compliance checks: Compile-time assertions for all 5 testutil mocks
  - Fixed nil record validation in SQLiteHistoryStore.Record() preventing segmentation fault
  - Enabled StopCondition syntax validation in ConversationConfig (previously skipped test)

- **C013** Split large domain test files into focused modules
  - Split 3 large domain test files (4,317 LOC total) into 11 focused test modules organized by functionality
  - Deleted `step_test.go` (1,615 lines) → split into 4 files: `step_command_test.go`, `step_parallel_test.go`, `step_loop_test.go`, `step_agent_test.go`
  - Split `agent_config_test.go` (1,819 lines) → split into 3 files: `agent_config_config_test.go`, `agent_config_result_test.go`, `agent_config_conversation_test.go` (retained 169-line stub)
  - Created `domain_test_helpers_test.go` (5.7K) with shared test utilities to prevent duplication across split files
  - All 170 original tests preserved across reorganized files; each new file <600 lines for improved maintainability

- **C012** Created ServiceTestHarness for fluent test setup
  - Created `ServiceTestHarness` in `internal/application/testutil_test.go` with fluent API for test setup
  - Reduced test boilerplate by 29% (13,676→9,753 LOC) across 3 test files via harness consolidation
  - Replaced 200+ repetitive mock setup patterns with 3-line harness chains (71% setup reduction)
  - Added comprehensive functional test suite (18 tests) validating end-to-end workflows:

- **C011** Added integration test suites for hooks, validation, secrets, and CLI
  - Added 4 integration test suites (1430 LOC): hooks lifecycle, input validation, secret masking, CLI exit codes
  - Created 9 YAML fixtures (345 LOC) for hooks, validation, secrets, and exit code test scenarios
  - Fixed `MaskText()` never invoked in shell executor; hook failures not enforced; error.type missing

- **C010** Added signal handling tests
  - Added `signal_test.go` (690 LOC): SIGINT/SIGTERM graceful shutdown, state preservation, parallel cancellation
  - Created 3 signal test fixtures; added state checkpointing on workflow cancellation in ExecutionService

- **C009** Added parallel execution tests and fixed race condition
  - Added `parallel_test.go` (948 LOC): all_succeed/any_succeed/best_effort strategies with timing assertions
  - Fixed race condition in ExecutionContext with RWMutex; added `GetAllStepStates()` thread-safe iterator

- **C008** Split execution_service_test.go into thematic files
  - Split 1,923-line `execution_service_test.go` into 6 thematic files (core/hooks/loop/parallel/retry)

- **C007** Modernized test infrastructure
  - Migrated 359 `os.Setenv` to `t.Setenv()`; created `internal/testutil` with 7 mocks and fluent builders
  - Added fixture factories (Simple/Linear/Parallel/Loop/ConversationWorkflow); 93% test setup reduction

- **C006** Reduced cognitive complexity in conversation and step execution
  - Reduced `ExecuteConversation` 29→18, `executeStep` 29→18 via pipeline and handler extraction

- **C005** Reduced cognitive complexity in step expansion and parallel execution
  - Reduced `expandStep` 23→18, `executeStep` 22→18, `executeAnySucceed` 22→18 via helper extraction

- **C004** Reduced CLI cognitive complexity
  - Reduced CLI complexity 116→60 points: `formatStep`, `generateEdges`, `writeValidationResultTable`

- **C003** Reduced graph traversal complexity
  - Reduced graph complexity via `visitState` enum and extracted helpers (27/23→≤20)

- **C002** Created shared helpers to eliminate duplication
  - Created `helpers.go` with shared `estimateTokens`, `cloneState`; eliminated 8 duplicated functions

- **C001** Reduced validation complexity
  - Reduced `validateRules` complexity 31→20 via type-checked validator wrappers

### Fixed
- **B010**: `awf validate` no longer rejects `{{.states.<step>.JSON.<field>}}` references as invalid
  - Added `JSON` entry to domain-layer `ValidStateProperties` map (was present only in runtime `pkg/interpolation`)
  - Added `json` → `JSON` casing normalization for actionable error hints
  - Root cause: F065 (JSON output format) added the entry to `pkg/interpolation` but missed the duplicate map in `internal/domain/workflow`
- **B009**: `script_file` now honors shebang lines for interpreter dispatch
  - Scripts with a shebang (`#!/usr/bin/env python3`, `#!/bin/bash`, etc.) are written to a temp file and executed directly, letting the kernel dispatch the correct interpreter
  - Scripts without a shebang fall back to `$SHELL -c` (backward compatible)
  - Temp files use `0o700` permissions and are cleaned up on success, failure, and cancellation
  - Inline `command` field behavior is unchanged
- **B006**: Shell commands no longer fail on Debian/Ubuntu where `/bin/sh` is `dash`
  - `ShellExecutor` now detects the user's preferred shell via `$SHELL` environment variable at construction time
  - Falls back to `/bin/sh` if `$SHELL` is unset, relative, or points to a non-existent binary
  - Bash-dependent workflow commands (arrays, `[[`, process substitution, brace expansion) now work automatically for users with bash
  - `WithShell(path)` functional option allows explicit shell override for testing and special cases
  - See [ADR-0012](docs/ADR/0012-runtime-shell-detection.md) for decision rationale and trade-offs
- **B005**: Local scripts and prompts now override global XDG paths when using `{{.awf.scripts_dir}}` or `{{.awf.prompts_dir}}`
  - `loadExternalFile()` now checks `<workflow_dir>/scripts/` (or `prompts/`) before falling back to the global XDG directory
  - New `resolveLocalOverGlobal()` helper detects XDG-prefixed paths and substitutes local equivalents when they exist
  - Applies to both `script_file` and `prompt_file` fields
  - **Workaround removal**: Users no longer need to use relative paths to get local-first resolution

- **[B002] CLIExecutor Missing Process Group Management**
  - Fixed orphan process accumulation when agent provider processes (claude, codex, gemini, opencode) persist after workflow cancellation
  - Added process group isolation using `SysProcAttr.Setpgid` and `syscall.Kill(-pid, SIGKILL)` pattern from ShellExecutor
  - Processes now terminate within 100ms of context cancellation, preventing memory leaks
  - Impact: Prevents accumulation of orphaned agent processes consuming system memory
  - Technical: Applied same 3-step pattern from ShellExecutor (Setpgid, Cancel callback, WaitDelay)
  - Testing: Added 5 unit tests and 2 integration tests verifying process group cleanup
  - Cleanup: Removed dead nil checks (bytes.Buffer.Bytes() never returns nil)

- **F049**: Storage Directory Documentation Mismatch
  - Removed unused `.awf/storage/states/` and `.awf/storage/logs/` directory creation from `awf init` command
  - Updated documentation to accurately reflect XDG-compliant storage paths (`~/.local/share/awf/` or `$XDG_DATA_HOME/awf/`)
  - `awf init` now creates only `workflows/` and `prompts/` directories as intended
  - State persistence continues to use XDG directories with `--storage-path` flag available for custom locations

### Agent
- **F032**: Agent Step Type
  - `type: agent` invokes AI CLI tools (Claude, Codex, Gemini, OpenCode) as workflow steps
  - Provider registry with configurable `model`, `max_tokens`, `temperature`, and `timeout`
  - `openai_compatible` provider for any Chat Completions API endpoint (see F070)
  - Prompt templates with full variable interpolation (`{{.inputs.*}}`, `{{.states.*}}`, `{{.env.*}}`)
  - Automatic JSON response parsing stored in `{{.states.step_name.Response}}`
  - Token usage tracking accessible via `{{.states.step_name.TokensUsed}}`
  - Dry-run mode displays resolved prompts without execution
- **F033**: Multi-turn Conversation Mode with Context Window Management
  - `mode: conversation` enables iterative agent interactions within a single step
  - Automatic context window management with `sliding_window` strategy (drops oldest turns, preserves system prompt)
  - `max_turns` and `max_context_tokens` configuration to control conversation bounds
  - Stop conditions with expression syntax: `response contains 'text'`, `response matches 'regex'`, `turn_count >= N`, `tokens_used > N`
  - Token tracking per turn with total consumption metrics in conversation state
  - Conversation state accessible in subsequent steps via `{{.states.step_name.conversation.*}}`
  - Provider support: Claude, Gemini, Codex with mock provider for testing

#### Interactive Input Collection
- **F046**: Interactive Mode for Incomplete Command Inputs
  - Automatically prompts for missing required workflow inputs when not provided via `--input` flags
  - Displays enum options as numbered lists (1-9) for constrained inputs
  - Validates input values against defined constraints (type, enum, pattern) with immediate error feedback
  - Supports optional inputs with default values (press Enter to skip)
  - Graceful error handling with retry on validation failure
  - Only activates in terminal environments; provides clear error message for non-interactive contexts
  - Seamless integration: runs interactively if stdin is terminal, fails fast otherwise

#### Workflow Visualization
- **F045**: Workflow Diagram Generation
  - `awf diagram <workflow>` outputs DOT format to stdout for visualization
  - `--output workflow.png` exports to image (PNG, SVG, PDF) via graphviz
  - `--direction <TB|LR|BT|RL>` controls graph layout direction
  - `--highlight <step>` emphasizes specific steps visually
  - Step shapes: command→box, parallel→diamond, terminal→oval, loop→hexagon
- **F023**: Workflow Composition with Sub-Workflows
  - `call_workflow` step type invokes another workflow as a sub-workflow
  - Input mapping passes parent variables to child workflow via `inputs:`
  - Output mapping captures child results via `outputs:` for parent access
  - Circular call detection prevents infinite recursion (A→B→A)
  - Nested sub-workflows supported with depth tracking
- **F035**: Workflow Arguments Help Command
  - `awf run <workflow> --help` displays workflow-specific input parameters
  - Shows input name, type, required/optional status, default values, and description
  - Workflow description displayed at top when present
  - Graceful handling for workflows with no inputs or non-existent workflows

#### Extensibility
- **F021**: Plugin System
  - Discover plugins from `$XDG_DATA_HOME/awf/plugins/` with YAML manifests
  - `awf plugin list`, `awf plugin enable`, `awf plugin disable` for management
  - Plugins register custom operations usable as workflow steps
  - Plugin SDK (`pkg/plugin/sdk`) for third-party plugin development
  - RPC-based architecture with process isolation and graceful shutdown

#### CLI & Usability
- **F044**: XDG Prompt Discovery
  - `awf list prompts` discovers from local (`.awf/prompts/`) and global (`$XDG_CONFIG_HOME/awf/prompts/`)
  - Local prompts override global when names conflict; source shown in listing
  - `@prompts/` prefix in `--input` resolves to file content (e.g., `--input prompt=@prompts/system.md`)
  - `awf init --global` creates global prompts directory with example template
  - Nested paths supported: `@prompts/agents/claude/system.md`

#### Execution Modes
- **F020**: Interactive Mode
  - `awf run --interactive` enables pause-before-each-step execution
  - Actions: `[c]ontinue`, `[s]kip`, `[a]bort`, `[i]nspect`, `[e]dit`, `[r]etry`
  - `--breakpoint` flag for selective pausing on specific steps
- **F019**: Dry-Run Mode
  - `awf run --dry-run` shows execution plan without running commands
  - Displays resolved commands, transitions, hooks, and configuration
  - Outputs text or JSON format
- **F039**: Single Step Execution
  - `awf run workflow.yaml --step=step_name` executes specific steps
  - `--mock states.prev_step.output="value"` for dependency mocking

#### Loop Constructs
- **F043**: Nested Loop Execution
  - Inner loops access outer variables via `{{.loop.Parent.*}}`
  - Arbitrary nesting depth with parent chains
- **F042**: Loop Context Variables
  - `{{.loop.Index1}}` for 1-based index
  - Full context: `Index`, `Index1`, `Item`, `First`, `Last`, `Length`
- **F037**: Dynamic Variable Interpolation in Loops
  - `max_iterations` supports `{{.inputs.*}}` and `{{.env.*}}` interpolation
  - Arithmetic expressions: `{{.inputs.pages * .inputs.retries_per_page}}`
  - Loop conditions (`while`, `until`) support dynamic interpolation
  - Parse-time validation warns about undefined variables via `awf validate`
- **F016**: Loop Constructs
  - `for_each` iterates over lists; `while` repeats until condition false
  - `max_iterations` safety limit; `break_when` for early exit

#### Workflow Features
- **F017**: Workflow Templates
  - Define templates in `.awf/templates/` with parameters
  - Reference via `use_template: <name>` with overrides
  - Circular reference detection at load time
- **F015**: Conditional Branching
  - `when:` clauses for dynamic transitions
  - Operators: `==`, `!=`, `<`, `>`, `<=`, `>=`, `and`, `or`, `not`
  - Access `inputs.*`, `states.*`, `env.*`, `workflow.*`
- **F041**: Template Reference Validation
  - Static validation of `{{inputs.X}}` and `{{states.X.output}}`
  - Forward reference detection; all errors in single pass

#### State Machine & Execution
- **F014**: Workflow History
  - `awf history` with `--workflow`, `--status`, `--since` filters
  - Statistics with `--stats`; 30-day auto-cleanup
- **F013**: Resume Command
  - `awf resume <workflow-id>` continues interrupted workflows
  - `awf resume --list` shows resumable workflows
- **F012**: Input Validation
  - Types: `string`, `integer`, `boolean`
  - Rules: `pattern`, `enum`, `min`/`max`, `file_exists`, `file_extension`
- **F011**: Retry with Exponential Backoff
  - Strategies: `exponential`, `linear`, `constant`
  - Jitter support; `retryable_exit_codes` filter
- **F010**: Parallel Step Execution
  - Strategies: `all_succeed`, `any_succeed`, `best_effort`
  - `max_concurrent` limit; context cancellation on failure
- **F009**: State Machine with Transitions
  - `on_success`/`on_failure` transitions
  - Terminal states; cycle/unreachable detection
  - `continue_on_error` flag

#### CLI & Usability
- **F036**: CLI Init Command
  - `awf init` creates `.awf/workflows/`, `.awf/prompts/` directories
  - Creates example workflow file

#### Configuration
- **F036**: Project Configuration File
  - `.awf/config.yaml` for project-level input defaults
  - Auto-population of workflow inputs from config
  - CLI `--input` flags override config values
  - `awf config show` displays current configuration (text/JSON/quiet formats)
  - Validation with clear error messages for invalid YAML
  - Warnings for unknown configuration keys

### Changed

- YAML parsing reports all errors instead of silently skipping malformed steps

### Fixed
- **B010**: `awf validate` no longer rejects `{{.states.<step>.JSON.<field>}}` references as invalid
  - Added `JSON` entry to domain-layer `ValidStateProperties` map (was present only in runtime `pkg/interpolation`)
  - Added `json` → `JSON` casing normalization for actionable error hints
  - Root cause: F065 (JSON output format) added the entry to `pkg/interpolation` but missed the duplicate map in `internal/domain/workflow`
- **B009**: `script_file` now honors shebang lines for interpreter dispatch
  - Scripts with a shebang (`#!/usr/bin/env python3`, `#!/bin/bash`, etc.) are written to a temp file and executed directly, letting the kernel dispatch the correct interpreter
  - Scripts without a shebang fall back to `$SHELL -c` (backward compatible)
  - Temp files use `0o700` permissions and are cleaned up on success, failure, and cancellation
  - Inline `command` field behavior is unchanged
- **B006**: Shell commands no longer fail on Debian/Ubuntu where `/bin/sh` is `dash`
  - `ShellExecutor` now detects the user's preferred shell via `$SHELL` environment variable at construction time
  - Falls back to `/bin/sh` if `$SHELL` is unset, relative, or points to a non-existent binary
  - Bash-dependent workflow commands (arrays, `[[`, process substitution, brace expansion) now work automatically for users with bash
  - `WithShell(path)` functional option allows explicit shell override for testing and special cases
  - See [ADR-0012](docs/ADR/0012-runtime-shell-detection.md) for decision rationale and trade-offs
- **B005**: Local scripts and prompts now override global XDG paths when using `{{.awf.scripts_dir}}` or `{{.awf.prompts_dir}}`
  - `loadExternalFile()` now checks `<workflow_dir>/scripts/` (or `prompts/`) before falling back to the global XDG directory
  - New `resolveLocalOverGlobal()` helper detects XDG-prefixed paths and substitutes local equivalents when they exist
  - Applies to both `script_file` and `prompt_file` fields
  - **Workaround removal**: Users no longer need to use relative paths to get local-first resolution

- **F048**: Loop body transitions now honored (while and foreach loops)
  - Loop executor now evaluates transitions after each body step execution
  - Transitions with `goto` targets within loop body skip intermediate steps
  - Transitions to steps outside loop body trigger early loop exit
  - Modified `StepExecutorFunc` signature to propagate transition results
  - Fixes workflows where conditional transitions should skip unnecessary steps
- **F047**: ForEach loop items serialize using Go's default format instead of JSON
  - Template interpolation now uses JSON marshalling for complex types in `{{.loop.Item}}`
  - Primitive values (string, int, float, bool) pass through unchanged for backward compatibility
  - Added `SerializeLoopItem()` helper in `pkg/interpolation` for type-aware serialization
  - Fixes workflows using `for_each` with `call_workflow` receiving malformed data format
- **Bug-48**: Cannot run two workflows simultaneously
  - Replaced BadgerDB with SQLite WAL mode for concurrent access
  - Multiple `awf` processes can now execute workflows in parallel
  - History store uses `busy_timeout` for automatic retry on contention
- **JSONStore race condition**: Concurrent `Save` operations used same temp file
  - Now uses unique names with PID and nanosecond timestamp

### Removed

- **BREAKING**: BadgerDB replaced with SQLite for history storage
  - Existing history data not migrated (fresh start)
  - New location: `~/.local/share/awf/history.db`
  - No code changes required; history is ephemeral operational data
- **BREAKING**: `Args` field removed from `ports.Command` struct
  - Was unused; use `ShellEscape()` from `pkg/interpolation` instead

---

[Unreleased]: https://github.com/awf-project/cli/compare/HEAD
