% architecture-schema.pl — Durable cross-PR architectural memory.
% Memory segment: architecture (committed to git, mounted rw, accumulates over time).
% Written by commit.yaml promote step (gated on pr_ready). Read by plan/implement.
%
% Upsert-key invariant: ZPM upsert_fact matches by functor + FIRST argument,
% so the first argument of every predicate is its unique key (name/id), never
% a layer or scope — otherwise an upsert would overwrite a sibling fact.
%
% Provenance: every fact carries the source PR/branch id as last argument so a
% reverted PR can be rolled back with retract_pr/1.

:- dynamic(arch_component/4).   % (Name, Layer, Responsibility, PR)  key=Name
:- dynamic(arch_port/3).        % (Name, Layer, PR)                  key=Name
:- dynamic(arch_decision/5).    % (Id, Topic, Choice, Rationale, PR) key=Id
:- dynamic(arch_convention/4).  % (Id, Scope, Rule, PR)              key=Id
:- dynamic(arch_adr/3).         % (Id, Title, PR)                    key=Id
:- dynamic(arch_feature/3).     % (PR, Issue, Summary)  key=PR — one record per PR by design

% Rollback: encodes the retractall set for every fact tagged with a PR (used on
% revert). NOTE: query-logic does not persist WAL writes, so this rule cannot be
% driven persistently via zpm-query.sh. For a durable CLI rollback, call
% clear-context per functor instead (zpm-clear.sh), e.g.:
%   ZPM_MEMORY=architecture zpm-clear.sh "arch_component(_,_,_,'F001')"  (× each functor)
% The rule remains the canonical semantics and works in a live MCP serve session.
retract_pr(PR) :-
    nonvar(PR),
    retractall(arch_component(_,_,_,PR)),
    retractall(arch_port(_,_,PR)),
    retractall(arch_decision(_,_,_,_,PR)),
    retractall(arch_convention(_,_,_,PR)),
    retractall(arch_adr(_,_,PR)),
    retractall(arch_feature(PR,_,_)).
