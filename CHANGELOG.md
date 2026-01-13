# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

- **[C006]** refactor(application): reduce execution core cognitive complexity through helper extraction
  - `ExecuteConversation`: 29 → ≤18 by extracting sequential pipeline (`validateConversationInputs`, `initializeConversationState`, `executeTurn`, `evaluateTurnCompletion`, `finalizeStopReason`)
  - `executeStep`: 29 → ≤18 by extracting preparation/execution/outcome handlers (`prepareStepExecution`, `resolveStepCommand`, `executeStepCommand`, `recordStepResult`, `handleExecutionError`, `handleNonZeroExit`, `handleSuccess`)
  - `IsProblematicMaxIterationPattern`: 24 → ≤18 by extracting shared pattern detection (`detectLoopPatterns`, `shouldCheckLoopProblems`)
  - `HandleMaxIterationFailure`: 23 → ≤18 by using shared `detectLoopPatterns` plus extraction of `buildLoopFailureError`, `executeLoopPostHooks`
  - Total: 15 helper methods extracted, 3 test files (54KB), eliminated 20+ lines of code duplication
  - All 693 existing tests pass unchanged (100% backward compatibility)
  - Helper methods achieve ~95% average test coverage
- **[C005]** refactor(application): reduce step executors cognitive complexity through helper extraction
  - `TemplateService.expandStep`: 23 → ≤18 by extracting parameter processing pipeline (`validateAndLoadTemplate`, `selectPrimaryStep`, `expandNestedTemplate`, `applyTemplateFields`)
  - `InteractiveExecutor.executeStep`: 22 → ≤18 by extracting result handlers (`HandleExecutionError`, `HandleNonZeroExit`, `HandleSuccess`)
  - `ParallelExecutor.executeAnySucceed`: 22 → ≤18 by extracting branch coordination helpers (`RunBranchWithSemaphore`, `CheckBranchSuccess`)
  - Total: 9 helper methods extracted, 3 new test files (78KB), zero signature changes
  - All 693 existing tests pass unchanged (100% backward compatibility)
- **[C004]** refactor(cli): reduce CLI/UI layer cognitive complexity through systematic helper extraction
  - `formatStep` (dry_run_formatter.go): 42 → 15 by extracting field formatters (`formatFieldIfPresent`, `formatRetry`, `formatCapture`)
  - `generateEdges` (dot_generator.go): 27 → 18 by extracting edge generators (`generateParallelEdges`, `generateLoopEdges`, `generateTransitionEdge`)
  - `writeValidationResultTable` (output.go): 25 → 11 by extracting row builders (`formatInputRow`, `formatStepRow`, `renderStatusHeader`)
  - `collectPromptsFromPaths` (list.go): 22 → 16 by extracting path guards (`shouldProcessEntry`, `buildPromptInfo`)
  - Total reduction: 116 → 60 complexity points across 4 functions (48% improvement, -56 points)
  - All 3,670+ lines of existing CLI/UI tests pass unchanged (100% backward compatibility)
  - Follows proven C001-C003 patterns: guard clauses, type-checked wrappers, helper consolidation
- **[C003]** Reduced cognitive complexity in workflow graph algorithms by extracting type-safe helpers:
  - Introduced `visitState` enum to replace magic numbers in DFS cycle detection
  - Extracted `findCycleStart` and `buildCyclePath` helpers in DetectCycles (27→≤20 complexity)
  - Extracted `enqueueIfNotVisited` helper in ComputeExecutionOrder (23→≤20 complexity)
  - No behavior changes, all 62 existing tests pass
- **[C002]** refactor(agents): reduce agent provider cognitive complexity by extracting shared helpers (C002)
  - Created `helpers.go` with shared utility functions: `estimateTokens`, `cloneState`, type-checked option getters
  - Refactored Claude, Codex, and Gemini providers to use shared helpers
  - Eliminated 5 duplicated `estimateTokens` functions and 3 duplicated `cloneState` functions
  - Maintained 100% backward compatibility with zero test modifications
- **[C001]**  refactor(validation): reduce validateRules cognitive complexity from 31 to ≤20 by extracting type-checked validator wrappers (C001)

### Fixed
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
