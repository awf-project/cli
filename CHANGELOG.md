# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **C070**: Extracted transport layer from `internal/infrastructure/pluginmgr/` into shared `pkg/registry/` package — version parsing, GitHub Releases client, and download/checksum/extraction utilities are now reusable across plugin and workflow pack systems; zero behavioral change to existing plugin commands

## [0.5.0] - 2026-03-30

### Breaking Changes

- **C069**: Plugin capability `commands` renamed to `step_types` — update `plugin.yaml` manifests declaring `capabilities: [commands]` to use `capabilities: [step_types]`; `commands` was never implemented and has no runtime behavior to migrate
- **F070**: Replaced `custom` agent provider with `openai_compatible` — `provider: custom` workflows fail validation with migration guidance; use `provider: openai_compatible` with `base_url` and `model`
- **C059**: Removed unimplemented `github.set_project_status` operation — workflows using it now fail at validation instead of runtime
- **C058**: Removed `ntfy` and `slack` notification backends — use `webhook` backend instead (supports ntfy, Slack, Discord, Teams, PagerDuty via URL + headers + body)
- **C057**: Removed deprecated `Tokens` field from step state — use `TokensUsed` (unreplaced references silently evaluate to `0`)
- **B001**: Expression context normalized to PascalCase — `output` → `Output`, `exit_code` → `ExitCode`, etc. Validation errors provide suggestions
- **F050**: State property references in templates require uppercase: `.Output`, `.Stderr`, `.ExitCode`, `.Status`
- BadgerDB replaced with SQLite for history storage — existing history not migrated
- `Args` field removed from `ports.Command` struct (was unused)

### Added

#### Agent & Conversation

- **F075**: `inject_context` appends interpolated context to user prompts on turns 2+ in conversation steps
- **F074**: `continue_from` resumes a prior step's session — CLI providers hand off session ID, `openai_compatible` loads turn history
- **F073**: Multi-turn CLI provider session resume — `claude`, `codex`, `gemini`, `opencode` maintain context across turns via native session flags
- **F070**: `openai_compatible` agent provider for any Chat Completions API endpoint (OpenAI, Ollama, vLLM, Groq, LM Studio)
- **F065**: `output_format` field for agent steps — `json` (accessible via `{{.states.step.JSON.field}}`) or `text`
- **F063**: `prompt_file` loads external prompt files with path resolution and interpolation (1MB limit)
- **F033**: `mode: conversation` enables multi-turn agent interactions with context window management, stop conditions, and token tracking
- **F032**: `type: agent` invokes AI CLI tools (Claude, Codex, Gemini, OpenCode) with prompt interpolation and response parsing

#### Operations & Plugins

- **C069**: Plugin validator capability — plugins implementing `sdk.Validator` run custom workflow validation rules during `awf validate`; results display with severity icons (`✗` error, `⚠` warning, `ℹ` info); per-plugin timeout (default 5s via `--validator-timeout`); crashes treated as timeouts; results deduplicated by `(message + step + field)`
- **C069**: Plugin step type capability — plugins implementing `sdk.StepTypeHandler` register custom `type:` values for workflow steps; unknown types routed to matching plugin at runtime; step output accessible via `{{states.step_name.Output}}` and `{{states.step_name.Data.key}}`; first-registered-wins on name conflicts; step type registrations cached at plugin init
- **C069**: `config:` field on workflow steps — passes structured configuration to custom step type plugins (separate from `inputs:` context interpolation)
- **C069**: `--skip-plugins` flag on `awf run` and `awf validate` — bypasses plugin validators and step type resolution; `awf run` fails with clear error when workflow contains a custom step type
- **C069**: `--validator-timeout` flag on `awf validate` — sets per-plugin validation timeout (default 5s)
- **C068**: Plugin registry with `awf plugin install/update/remove/search` — download from GitHub Releases with SHA-256 checksum verification, atomic installation, version constraints, pre-release support, `gh auth token` fallback, and `SOURCE` column in `awf plugin list`
- **C067**: External plugin gRPC transport via HashiCorp go-plugin — `RPCPluginManager` starts real plugin processes, `sdk.Serve()` entry point for plugin authors, echo example plugin in `examples/plugins/` ([ADR-015](docs/ADR/015-grpc-go-plugin-transport-for-external-plugins.md))
- **C066**: Built-in operation providers (GitHub, HTTP, Notify) visible in `awf plugin list` with `--operations` flag
- **F058**: `http.request` operation for declarative REST API calls (GET/POST/PUT/DELETE, timeout, retry, response capture)
- **F056**: `notify.send` operation with desktop and webhook backends, template interpolation
- **F054**: GitHub CLI plugin with 8 operations (`get_issue`, `get_pr`, `create_pr`, `create_issue`, `add_labels`, `list_comments`, `add_comment`, `batch`)
- **F057**: Operation interface and registry foundation for plugin operations
- **F021**: Plugin system with discovery, enable/disable management, SDK, and RPC isolation

#### Workflow Features

- **F068**: Exit code-based transition routing — `when` conditions evaluated on non-zero exit paths
- **F066**: Inline `on_failure` shorthand — `{message: "...", status: N}` instead of named terminal states
- **F064**: `script_file` loads external shell scripts with path resolution and interpolation
- **F043**: Nested loop execution with `{{.loop.Parent.*}}` access to outer variables
- **F042**: Loop context variables: `Index`, `Index1`, `Item`, `First`, `Last`, `Length`
- **F037**: Dynamic variable interpolation in `max_iterations`, `while`, `until`, `break_when`
- **F023**: `call_workflow` for sub-workflow composition with input/output mapping
- **F017**: Workflow templates with `use_template:` and parameter overrides
- **F016**: `for_each` and `while` loop constructs with `max_iterations` and `break_when`
- **F015**: Conditional branching with `when:` clauses and comparison operators
- **F011**: Retry with exponential/linear/constant backoff, jitter, and retryable exit codes
- **F010**: Parallel step execution with `all_succeed`, `any_succeed`, `best_effort` strategies
- **F009**: State machine with `on_success`/`on_failure` transitions and `continue_on_error`

#### CLI & Usability

- **F072**: Hugo documentation site with Doks theme, FlexSearch, dark/light mode
- **F071**: Structured JSONL audit trail (`$XDG_DATA_HOME/awf/audit.jsonl`)
- **C060**: `awf init` creates `.awf/scripts/` directory with example script
- **C048**: Actionable error hints with "Did you mean?", line/column refs, resolution steps (`--no-hints` to suppress)
- **C047**: Structured error codes (`CATEGORY.SUBCATEGORY.SPECIFIC`) with `awf error <code>` lookup
- **F046**: Interactive input collection — prompts for missing inputs with enum lists and validation
- **F045**: `awf diagram` generates DOT/PNG/SVG workflow visualizations
- **F044**: XDG prompt discovery with `awf list prompts` and `@prompts/` prefix in `--input`
- **F039**: `awf run workflow.yaml --step=step_name` for single step execution with `--mock`
- **F036**: `awf init` scaffolding and `.awf/config.yaml` for project-level input defaults
- **F035**: `awf run <workflow> --help` shows workflow-specific input parameters
- **F020**: Interactive mode (`--interactive`) with pause-before-each-step and breakpoints
- **F019**: Dry-run mode (`--dry-run`) shows execution plan without running commands
- **F014**: `awf history` with filters and `--stats`
- **F013**: `awf resume` for interrupted workflow continuation
- **F012**: Input validation with types, patterns, enums, min/max, file checks

### Changed

- **C065**: OpenAI Compatible provider uses `max_completion_tokens` — `max_tokens` accepted as deprecated fallback
- **C064**: `max_delay` omission no longer caps retry delays to zero; `multiplier` defaults to 2.0; invalid durations now produce parse errors
- **C063**: Added `index1` and `parent` to loop accessor; modulo operator supported in `max_iterations`
- **C061**: Timeout ownership centralized in application layer; `handleExecutionError` evaluates transitions before `continue_on_error`
- YAML parsing reports all errors instead of silently skipping malformed steps

### Fixed

- **B011**: `{{.awf.scripts_dir}}` and `{{.awf.prompts_dir}}` in `command:` and `dir:` fields now resolve local-before-global
- **B010**: `awf validate` no longer rejects `{{.states.<step>.JSON.<field>}}` references
- **B009**: `script_file` honors shebang lines — scripts with `#!` use kernel dispatch instead of `$SHELL -c`
- **B008**: Ctrl+C no longer hangs during interactive input prompts
- **B007**: Interactive and dry-run modes now respect `.awf/config.yaml` input defaults
- **B006**: Shell commands no longer fail on Debian/Ubuntu where `/bin/sh` is `dash` — detects `$SHELL` at startup
- **B005**: Local scripts and prompts override global XDG paths in `script_file` and `prompt_file`
- **B004**: Validator no longer forces dead-code transitions on parallel branch children
- **B003**: While loop `break_when` evaluates correctly with arbitrary output values
- **B002**: Agent provider processes no longer persist as orphans after workflow cancellation
- **F051**: Multi-turn conversations resolve prompts correctly past first turn
- **F049**: `awf init` no longer creates unused `storage/` directories
- **F048**: Loop body transitions honored — `goto` targets and early exit work in while/foreach loops
- **F047**: ForEach loop items serialize complex types as JSON instead of Go default format
- **Bug-48**: Multiple `awf` processes can run workflows simultaneously (SQLite WAL mode)
- JSONStore race condition on concurrent `Save` operations

---

[Unreleased]: https://github.com/awf-project/cli/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/awf-project/cli/compare/v0.4.1...v0.5.0
