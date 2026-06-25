% ─── PR Tracking Schema ──────────────────────────────────────────────────────
% Memory segment: pr_<branch>
% Lifecycle: created at implement start, gated before commit, archived on merge.
%
% Facts (asserted by scan scripts and LLM):
%   pr_file(Path, ChangeType)        — file in PR scope (changed | added | test)
%   todo(Id, File, Line, Desc)       — TODO/FIXME found in changed code
%   stub(Id, File, Symbol)           — stub/placeholder implementation
%   mock(Id, File, Symbol)           — mock that should be replaced with real impl
%   not_impl(Id, File, Desc)         — "not yet implemented" marker
%   resolved(Type, Id)               — marks a tracked issue as resolved
%   pr_scanned                       — marker: a scan ran (gate fails closed if absent)
%   task_blocked(TaskId, Reason, It) — a task the loop marked blocked, with reason + iteration
%   block_detail(TaskId, File, Kind) — per-file cause of a block (e.g. todo!( in a file)
%
% Dynamic declarations (required by Trealla Prolog for runtime assertion).
:- dynamic(pr_file/2).
:- dynamic(todo/4).
:- dynamic(stub/3).
:- dynamic(mock/3).
:- dynamic(not_impl/3).
:- dynamic(resolved/2).
:- dynamic(pr_scanned/0).
:- dynamic(task_blocked/3).
:- dynamic(block_detail/3).

% ─── Unresolved queries ─────────────────────────────────────────────────────
% Convenience predicates for querying unresolved issues by type.
unresolved_todo(Id, File, Line, Desc) :-
    todo(Id, File, Line, Desc), \+ resolved(todo, Id).
unresolved_stub(Id, File, Symbol) :-
    stub(Id, File, Symbol), \+ resolved(stub, Id).
unresolved_mock(Id, File, Symbol) :-
    mock(Id, File, Symbol), \+ resolved(mock, Id).
unresolved_not_impl(Id, File, Desc) :-
    not_impl(Id, File, Desc), \+ resolved(not_impl, Id).

% A task the loop marked blocked and that nothing later resolved. Surfaced to
% sibling tasks (and re-runs) via inject-zpm-context.sh so they can avoid the
% same dead end (feed-forward). resolved(task, TaskId) clears it.
unresolved_block(TaskId, Reason, Iter) :-
    task_blocked(TaskId, Reason, Iter), \+ resolved(task, TaskId).

% A blocking issue is any tracked issue that has not been resolved.
blocking_issue(Id, todo, File, Desc) :-
    todo(Id, File, _, Desc), \+ resolved(todo, Id).
blocking_issue(Id, stub, File, Symbol) :-
    stub(Id, File, Symbol), \+ resolved(stub, Id).
blocking_issue(Id, mock, File, Symbol) :-
    mock(Id, File, Symbol), \+ resolved(mock, Id).
blocking_issue(Id, not_impl, File, Desc) :-
    not_impl(Id, File, Desc), \+ resolved(not_impl, Id).

% PR is ready ONLY when a scan has run AND zero blocking issues remain.
% The pr_scanned guard prevents a vacuous pass: an empty segment (scan skipped
% or never populated) has no blocking_issue, so \+ blocking_issue alone would
% succeed and green-light an unscanned PR. Requiring pr_scanned fails closed.
pr_ready :- pr_scanned, \+ blocking_issue(_, _, _, _).

% Health summary — counts by category.
pr_health(blocking, N) :-
    findall(I, blocking_issue(I, _, _, _), L), length(L, N).
pr_health(resolved, N) :-
    findall(I, resolved(_, I), L), length(L, N).
pr_health(files, N) :-
    findall(F, pr_file(F, _), L), length(L, N).

% Coverage gap: source file changed without corresponding test file.
coverage_gap(File) :-
    pr_file(File, changed),
    \+ pr_file(File, test),
    \+ test_file(File, _).

% List all blocking issues as Id-Type-File-Desc tuples.
all_blockers(Blockers) :-
    findall(blocker(Id, Type, File, Desc), blocking_issue(Id, Type, File, Desc), Blockers).
