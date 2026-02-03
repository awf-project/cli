# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

- **F051**: Multi-Turn Conversation Workflow Failures
  - Fixed empty prompt bug preventing conversations from continuing past first turn
  - Consolidated duplicate code in ConversationManager by replacing inline logic with existing helper methods
  - Implemented `executeConversationStep` in ExecutionService to properly delegate to ConversationManager
  - Multi-turn conversations now resolve prompts correctly for subsequent turns using configured prompt template
  - Added comprehensive unit tests for multi-turn prompt resolution and conversation step execution
  - Added integration test suite covering multi-turn workflows, stop conditions, and backward compatibility
  - Fixes: conversation-simple, conversation-multiturn, conversation-max-turns, conversation-error workflows
  - Impact: 391 lines added, 102 lines removed across 4 application layer files

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
  - `custom` provider for unsupported CLIs via command template with `{{prompt}}` placeholder
  - Prompt templates with full variable interpolation (`{{.inputs.*}}`, `{{.states.*}}`, `{{.env.*}}`)
  - Automatic JSON response parsing stored in `{{.states.step_name.response}}`
  - Token usage tracking accessible via `{{.states.step_name.tokens}}`
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

[Unreleased]: https://github.com/vanoix/awf/compare/HEAD
