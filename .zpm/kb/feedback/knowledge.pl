:- module(feedback, []).
% ─── Feedback Rules Schema ───────────────────────────────────────────────────
% Memory segment: feedback
% Lifecycle: created at project setup, accumulates across workflow runs.
% Replaces markdown-based feedback in CLAUDE.md with queryable Prolog facts.
%
% Facts:
%   rule(Id, Category, Description, Priority, Source)
%     Id: unique atom (e.g. rule_001, rule_avoid_sql_concat)
%     Category: architecture | pitfall | test | review | style
%     Description: the rule text (single-quoted atom)
%     Priority: high | medium | low
%     Source: implement | audit | fix_errors | pr_review
%
%   example(RuleId, Type, Code, Explanation)
%     Type: good | bad
%
%   trigger(RuleId, Pattern, Scope)
%     Pattern: file/path substring to match
%     Scope: file | directory | project
%
% Dynamic declarations (required by Trealla Prolog for runtime assertion).
:- dynamic(rule/5).
:- dynamic(example/4).
:- dynamic(trigger/3).

% Derived rules:

% A rule is applicable to a file if its trigger pattern matches.
applicable(RuleId, File) :-
    trigger(RuleId, Pattern, file),
    sub_atom(File, _, _, _, Pattern).
applicable(RuleId, File) :-
    trigger(RuleId, Pattern, directory),
    sub_atom(File, _, _, _, Pattern).
applicable(RuleId, _File) :-
    trigger(RuleId, _, project).

% Find all rules effective for a list of files (deduplicated).
effective_rules(Files, RuleIds) :-
    findall(Id, (member(F, Files), applicable(Id, F)), All),
    sort(All, RuleIds).

% Get high-priority rules.
priority_rules(Ids) :-
    findall(Id, rule(Id, _, _, high, _), Ids).

% Rules by category.
rules_by_category(Cat, Ids) :-
    findall(Id, rule(Id, Cat, _, _, _), Ids).

% Rules by source workflow.
rules_by_source(Src, Ids) :-
    findall(Id, rule(Id, _, _, _, Src), Ids).

% Full rule detail for display.
rule_detail(Id, Cat, Desc, Prio) :-
    rule(Id, Cat, Desc, Prio, _).

% Count rules.
rule_count(N) :-
    findall(Id, rule(Id, _, _, _, _), L), length(L, N).
