# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
I now have all the information needed to create the CHANGELOG entry. Based on my analysis:

**F044 XDG Prompt Discovery** is a feature that:
1. Discovers prompts from both local (`.awf/prompts/`) and global (`$XDG_CONFIG_HOME/awf/prompts/`) directories
2. Shows source (local/global) when listing prompts via `awf list prompts`
3. Local prompts take precedence over global when names conflict
4. Supports nested directory structures (e.g., `agents/claude/system.md`)
5. Resolves `@prompts/` prefix in workflow inputs to actual file content
6. Adds `awf init --global` to create global prompts directory
7. Includes path traversal security protection

---

## CHANGELOG Entry for F044

**1. Category:** Added

**2. Subcategory:** CLI & Usability

**3. Entry:**

```markdown
- **F044**: XDG Prompt Discovery
  - `awf list prompts` discovers from local (`.awf/prompts/`) and global (`$XDG_CONFIG_HOME/awf/prompts/`)
  - Local prompts override global when names conflict; source shown in listing
  - `@prompts/` prefix in `--input` resolves to file content (e.g., `--input prompt=@prompts/system.md`)
  - `awf init --global` creates global prompts directory with example template
  - Nested paths supported: `@prompts/agents/claude/system.md`
```

**4. Notes:** No breaking changes. Feature extends existing `awf init` and `awf list` commands. Path traversal attempts (e.g., `@prompts/../secret.txt`) are rejected for security.

#### Execution Modes
- **F020**: Interactive Mode for step-by-step workflow execution
  - `awf run --interactive` enables pause-before-each-step execution
  - Actions: `[c]ontinue`, `[s]kip`, `[a]bort`, `[i]nspect`, `[e]dit`, `[r]etry`
  - `--breakpoint` flag for selective pausing on specific steps
  - Step details and results displayed during execution
- **F019**: Dry-Run Mode
  - `awf run --dry-run` shows execution plan without running commands
  - Displays resolved commands, transitions, hooks, and configuration
  - Supports parallel steps and loops; outputs text or JSON format
- **F039**: Single step execution with `--step` flag
  - Execute specific steps: `awf run workflow.yaml --step=step_name`
  - Mock dependencies: `--mock states.prev_step.output="value"`

#### Loop Constructs
- **F043**: Nested Loop Execution with parent context access
  - Inner loops access outer variables via `{{.loop.Parent.*}}`
  - Arbitrary nesting depth with parent chains
  - Mixed loop types nest correctly
- **F042**: Loop Context Variables
  - `{{.loop.Index1}}` for 1-based index
  - Full context: `Index`, `Index1`, `Item`, `First`, `Last`, `Length`
  - Works in commands and `when` expressions
- **F016**: Loop Constructs (for-each/while)
  - `for_each` iterates over lists; `while` repeats until condition false
  - `max_iterations` safety limit; `break_when` for early exit

#### Workflow Features
- **F017**: Workflow Templates with Parameters
  - Define templates in `.awf/templates/` with parameters
  - Reference via `use_template: <name>` with overrides
  - Circular reference detection; validation at load time
- **F015**: Conditional Branching with `when:` Clauses
  - Dynamic transitions based on expressions
  - Operators: `==`, `!=`, `<`, `>`, `<=`, `>=`, `and`, `or`, `not`
  - Access `inputs.*`, `states.*`, `env.*`, `workflow.*`
- **F041**: Validate Template Interpolation References
  - Static validation of `{{inputs.X}}` and `{{states.X.output}}`
  - Forward reference detection; all errors reported in single pass

#### State Machine & Execution
- **F014**: BadgerDB History
  - `awf history` with `--workflow`, `--status`, `--since` filters
  - Statistics with `--stats`; 30-day auto-cleanup
- **F013**: Resume Command
  - `awf resume <workflow-id>` to continue interrupted workflows
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
  - Terminal states; cycle/unreachable state detection
  - `continue_on_error` flag

#### CLI & Usability
- **F036**: CLI init command (`awf init`)
  - Creates `.awf/workflows/`, `.awf/prompts/` directories
  - Creates example workflow file
- **F037**: Step success feedback for empty-output steps
- ParallelStrategy validation at load time
- Race condition tests for JSONStore
- Expanded CLI integration tests

### Changed
  
- YAML parsing reports all errors instead of silently skipping malformed steps
  - Uses `errors.Join()` for aggregation

### Fixed

- **Race condition in JSONStore**: Concurrent `Save` operations used same temp file; now uses unique names with PID and nanosecond timestamp

### Removed

- **BREAKING**: `Args` field removed from `ports.Command` struct
  - Was unused by `ShellExecutor`
  - Use `ShellEscape()` from `pkg/interpolation` for user-provided values

---

[Unreleased]: https://github.com/vanoix/awf/compare/HEAD
