% base-schema.pl — Shared AWF knowledge-base schema.
% Loaded automatically from .zpm/kb/default/.

% Dynamic declarations — required by Trealla Prolog so that rules referencing
% these predicates compile even before any facts are asserted at runtime.
:- dynamic(task/3).
:- dynamic(depends_on/2).
:- dynamic(task_complete/1).
:- dynamic(task_file/2).
:- dynamic(component/4).
:- dynamic(stub/3).
:- dynamic(test_file/2).
:- dynamic(source_file/1).
:- dynamic(adr/3).
:- dynamic(workflow_run/3).
:- dynamic(impl_note/3).
:- dynamic(feedback_rule/3).
:- dynamic(imports/2).
:- dynamic(layer/2).
:- dynamic(uses_panic/1).
:- dynamic(coverage/2).
:- dynamic(integrity_violation/2).

% --- Task graph (written by plan, read by implement/fix-errors) ---
% task(Id, Title, Status).  Status ∈ {pending, in_progress, completed, blocked}.
% depends_on(TaskA, TaskB).  TaskA depends on TaskB completing first.
% task_complete(Id).          Written by implement when stubs cleared + tests green.
% incomplete_task(Id).        Derived: task that is pending/in_progress with unresolved stubs.

incomplete_task(Id) :-
    task(Id, _, Status),
    Status \= completed,
    stub(File, _, critical),
    task_file(Id, File).

% --- Component graph (written by plan, read by implement) ---
% component(Id, Name, Path, Type).  Type ∈ {module, file, package, pipeline}.
% pipeline_handled(Path).           Paths managed by pipelines (CHANGELOG, docs/...).
% forbidden_component(Id) :-        Components plan must not include.
%     component(Id, _, Path, _), pipeline_handled(Path).

% forbidden_component/1: Path is bound via component/4, so nonvar-guarded
% pipeline_handled/1 prefix rules fire correctly. Do not call
% pipeline_handled(X) with X unbound — rules will not enumerate.
forbidden_component(Id) :-
    component(Id, _, Path, _),
    pipeline_handled(Path).

% Seed the standard pipeline-handled paths (exact matches)
pipeline_handled('CHANGELOG.md').
pipeline_handled('docs/CHANGELOG.md').
pipeline_handled('README.md').
pipeline_handled('README').
pipeline_handled('CHANGELOG').
pipeline_handled('CLAUDE.md').

% Prefix-based rules (match directories handled by pipelines).
% nonvar/1 guard prevents ZPM 0.1.0 instantiation errors when Path is unbound,
% which would otherwise abort enumeration of the entire predicate (see KNOWN-ISSUES).
% These rules only fire when Path is already bound (the normal use case via forbidden_component/1).
pipeline_handled(Path) :- nonvar(Path), sub_atom(Path, 0, _, _, 'docs/').
pipeline_handled(Path) :- nonvar(Path), sub_atom(Path, 0, _, _, '.specify/memory/').
pipeline_handled(Path) :- nonvar(Path), sub_atom(Path, 0, _, _, 'ADR/').

% --- Stub tracking (written by code, read by implement) ---
% stub(File, Line, Kind).  Kind ∈ {critical, cosmetic, todo, fixme}.
% task_file(TaskId, File).  Links stubs back to tasks.

% --- Test coverage (written by plan/implement, used by audit) ---
% test_file(SourcePath, TestPath).
% covered_by(Source, Test) :- test_file(Source, Test).

% Decision: wrap test_file/2 in catch/3, same reason as current_decision.
% Why: ZPM (Scryer Prolog) raises existence_error when a constitution rule
% evaluates \+ covered_by(File, _) on a fresh KB with zero test_file facts.
% catch/3 intercepts that error and cleanly fails, so the negation succeeds
% ("no test covers this source") and missing_test violations derive correctly.
covered_by(Source, Test) :-
    catch(test_file(Source, Test), _, fail).

% --- ADR (written by plan/spec-generate, read by architecture reviews) ---
% adr(Id, Title, Decision).
% supersedes(NewId, OldId).
% current_decision(Decision) :-
%     adr(Id, _, Decision), \+ supersedes(_, Id).

% Decision: use catch/3 instead of \+/1 for supersedes check.
% Why: ZPM (Scryer Prolog) raises existence_error when \+ calls an
% undefined predicate. catch/3 handles both the undefined-predicate case
% (no supersedes facts yet → succeed) and the normal case (supersedes
% facts exist → \+ evaluates correctly).
% Trade-off: slightly less idiomatic Prolog; necessary for ZPM compatibility.
current_decision(Id, Decision) :-
    adr(Id, _, Decision),
    catch((\+ supersedes(_, Id)), _, true).

% --- Feedback rules (written by feedback workflow, read by review agents) ---
% feedback_rule(Topic, Date, Text).
%   Topic: atom matching CLAUDE.md section slug (e.g. architecture_rules)
%          derived from section name: lowercase, spaces→underscores
%   Date:  ISO 8601 atom 'YYYY-MM-DD' — lexicographic comparison works for
%          date ordering, e.g. Date @>= '2026-04-15' for "last 7 days"
%   Text:  quoted atom with the rule body
% Query "recent feedback" example (caller computes cutoff date):
%   feedback_rule(Topic, Date, Text), Date @>= '2026-04-15'.

% --- Workflow history (written by commit) ---
% workflow_run(Name, Date, Status).  Status ∈ {success, failure, partial}.

% --- LLM-emitted rationale (written by plan/code-review agents, read by audit) ---
% Captured from fenced ```zpm blocks emitted by agents; see
% scripts/zpm/parse-zpm-block.sh.
%
% task_rationale(TaskId, Text).
%   Why this task exists — captured at generate_tasks time.
%   Keyed by TaskId (use upsert: one rationale per task).
%
% task_risk(TaskId, Level, Reason).
%   Level ∈ {low, medium, high}.
%   LLM's self-assessed risk. Keyed by TaskId (use upsert).
%
% review_finding(Issue, File, Line, Severity, Kind).
%   Severity ∈ {info, warning, critical}.
%   Kind: short atom (e.g. 'duplication', 'n_plus_1', 'hard_coded_secret').
%   Multi-valued — many findings per issue. Use assert.
%
% impl_note(File, Symbol, Note).
%   Local implementation decisions captured during the code TDD cycle.
%   Multi-valued — many notes per (File, Symbol). Use assert.

% Convenience rule: high-risk pending tasks (for audit and dashboards).
high_risk_open_task(Id, Title) :-
    task(Id, Title, Status),
    Status \= completed,
    task_risk(Id, high, _).

% --- Generic integrity violation scaffolding ---
% Constitutions assert integrity_violation/2 rules.
% Severity ∈ {error, warning, info}.
% Example (from constitution-go.pl):
%   integrity_violation(layer_inversion, File) :-
%     imports(File, Target),
%     layer(File, domain),
%     layer(Target, infrastructure).

% ─── Project Memory ──────────────────────────────────────────────────────────
:- dynamic(observation/4).
:- dynamic(decision/4).
:- dynamic(convention/3).

% observation(Id, Category, Content, Date)
%   Id: unique atom (auto-generated slug)
%   Category: pattern | convention | quirk | dependency | performance
%   Content: description (single-quoted atom)
%   Date: ISO date atom e.g. '2026-05-24'
%
% decision(Id, What, Why, TradeOff)
%   Architectural/design decisions recorded during workflows
%
% convention(Id, Pattern, Context)
%   Discovered codebase conventions worth preserving

observations_by_category(Cat, Ids) :-
    findall(Id, observation(Id, Cat, _, _), Ids).

decisions_about(Topic, Ids) :-
    findall(Id, (decision(Id, What, _, _), sub_atom(What, _, _, _, Topic)), Ids).
% constitution-go.pl — Go project integrity rules (Prolog).
% Loaded automatically from .zpm/kb/ alongside base-schema.pl.
%
% Encodes a machine-checkable subset of Project Constitution v1.0.0 (go.md).
% Rules produce integrity_violation(Kind, File) facts that audit tooling queries.
%
% ---  7 Principles (verbatim titles + 1-line summaries from go.md)  ----------
%
% P1 — Hexagonal Architecture
%      Domain depends on nothing; application on domain; infrastructure
%      implements ports; interfaces depend on application.
%
% P2 — Go Idioms
%      No panic in library code; interfaces defined by consumer; accept
%      interfaces return structs; explicit error handling.
%
% P3 — Test-Driven Development
%      RED → GREEN → REFACTOR; 80% minimum code coverage; table-driven tests.
%
% P4 — Error Taxonomy
%      Exit codes 1-4 map to user / config / execution / system errors.
%      (Not modelled: purely a runtime/CLI convention, not statically checkable)
%
% P5 — Security First
%      Inputs untrusted; secrets masked; atomic writes; no plaintext creds.
%      (Not modelled: requires data-flow analysis, beyond static ZPM facts)
%
% P6 — Minimal Abstraction
%      No premature optimisation; delete unused code; comments only when needed.
%      (Not modelled: heuristic too weak without AST; no reliable Prolog rule)
%
% P7 — Documentation Co-location
%      README.md, CLAUDE.md, CHANGELOG.md, specs in .specify/.
%      (Not modelled: file-presence checks handled by shell audit scripts)
%
% ---------------------------------------------------------------------------
%
% FACT SCHEMA (asserted by callers, e.g. via zpm-assert.sh):
%
%   layer(File, Layer)          File belongs to Layer.
%                               Layer ∈ {domain, application, infrastructure, interfaces}
%   imports(File, Target)       File depends on Target (path or module import).
%   source_file(File)           File is a Go source file tracked in the project.
%   coverage(File, Pct)         File has measured test coverage Pct (integer 0-100).
%   uses_panic(File)            File contains a panic() call.
%
%   covered_by/2 is defined in base-schema.pl via test_file/2 — do NOT redefine here.
%
% ---------------------------------------------------------------------------

% layer_index/2: numeric order of layers, inner (low) → outer (high).
% Used to detect when a lower-indexed layer imports a higher-indexed layer.

layer_index(domain,         0).
layer_index(application,    1).
layer_index(infrastructure, 2).
layer_index(interfaces,     3).

% --- Violation kinds modelled in this constitution -------------------------
% Enumeration helper for audit tooling. Without this, querying
% integrity_violation(K, F) with both vars unbound returns [] in ZPM 0.1.0.
violation_kind(layer_inversion).
violation_kind(missing_test).
violation_kind(framework_in_domain).
violation_kind(panic_in_library).
violation_kind(low_coverage).

% --- P1: Layer inversion ----------------------------------------------------
% Violation when a file in an inner layer imports a file in an outer layer.

integrity_violation(layer_inversion, File) :-
    imports(File, Target),
    layer(File, LayerA),
    layer(Target, LayerB),
    layer_index(LayerA, IdxA),
    layer_index(LayerB, IdxB),
    IdxA < IdxB.

% --- P3: Missing test -------------------------------------------------------
% A source_file has no covered_by entry (via base-schema.pl test_file/2).
% Excludes main.go (entry point) and _test.go files (they ARE the tests).

is_main_file('main.go').
is_main_file(File) :- atom_concat(_, '/main.go', File).
is_main_file(File) :- atom_concat(_, '_test.go', File).

% doc.go files hold only the package-doc comment (zero executable code), so
% there is nothing to test — exempt them from missing_test like main/_test files.
doc_only_file('doc.go').
doc_only_file(File) :- atom_concat(_, '/doc.go', File).

integrity_violation(missing_test, File) :-
    source_file(File),
    \+ covered_by(File, _),
    \+ is_main_file(File),
    \+ doc_only_file(File).

% --- P2: Framework import in domain layer -----------------------------------
% Domain code must not import web / ORM frameworks (tight coupling to infra).

web_framework('github.com/gin-gonic/gin').
web_framework('github.com/labstack/echo').
web_framework('github.com/labstack/echo/v4').
web_framework('gorm.io/gorm').
web_framework('github.com/gofiber/fiber/v2').
web_framework('github.com/go-chi/chi/v5').
web_framework('github.com/gorilla/mux').

integrity_violation(framework_in_domain, File) :-
    layer(File, domain),
    imports(File, Target),
    web_framework(Target).

% --- P2: Panic in library code ----------------------------------------------
% Library layers (domain, application, infrastructure) must not use panic.
% The interfaces layer (CLI/API entry points) is exempt — it may panic on
% unrecoverable startup errors.

library_layer(domain).
library_layer(application).
library_layer(infrastructure).

integrity_violation(panic_in_library, File) :-
    uses_panic(File),
    layer(File, Layer),
    library_layer(Layer).

% --- P3: Low coverage -------------------------------------------------------
% Any file with a coverage fact below 80% is a violation.

integrity_violation(low_coverage, File) :-
    coverage(File, Pct),
    Pct < 80.

% --- Skipped rules ----------------------------------------------------------
%
% P5 — logic_in_interfaces (draft rule from plan §563-630):
%   The heuristic "interface file has >N lines of non-declaration code" is
%   unreliable without a proper AST. Skipped; catches too many false positives.
%
% P5 — package_name_mismatch:
%   Go package naming is enforced by `go vet` and the compiler. Reproducing
%   that logic in ZPM provides no additional safety signal.
