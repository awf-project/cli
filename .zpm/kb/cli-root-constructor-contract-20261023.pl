%% zpm snapshot 1792757788
:- dynamic(portray/1).
:- dynamic(source_file/1).
:- dynamic(task_complete/1).
:- dynamic(uses_panic/1).
:- dynamic(branch_feature/2).
:- dynamic(coverage/2).
:- dynamic(depends_on/2).
:- dynamic(imports/2).
:- dynamic(integrity_violation/2).
:- dynamic(layer/2).
:- dynamic(requirement/2).
:- dynamic(task_file/2).
:- dynamic(test_file/2).
:- dynamic(tms_justification/2).
:- dynamic(zpm_source/2).
:- dynamic(adr/3).
:- dynamic(convention/3).
:- dynamic(feedback_rule/3).
:- dynamic(impl_note/3).
:- dynamic(stub/3).
:- dynamic(task/3).
:- dynamic(workflow_run/3).
:- dynamic(component/4).
:- dynamic(decision/4).
:- dynamic(observation/4).
branch_feature('bug/386-b017-audit-events-are-inconsistent-incom',feature_b017).
integrity_violation(layer_inversion,A) :-
   imports(A,B),layer(A,C),layer(B,D),layer_index(C,E),layer_index(D,F),E<F.
integrity_violation(missing_test,A) :-
   source_file(A), \+covered_by(A,_), \+is_main_file(A), \+doc_only_file(A).
integrity_violation(framework_in_domain,A) :-
   layer(A,domain),imports(A,B),web_framework(B).
integrity_violation(panic_in_library,A) :-
   uses_panic(A),layer(A,B),library_layer(B).
integrity_violation(low_coverage,A) :-
   coverage(A,B),B<80.
integrity_violation(layer_inversion,A) :-
   imports(A,B),layer(A,C),layer(B,D),layer_index(C,E),layer_index(D,F),E<F.
integrity_violation(missing_test,A) :-
   source_file(A), \+covered_by(A,_), \+is_main_file(A), \+doc_only_file(A).
integrity_violation(framework_in_domain,A) :-
   layer(A,domain),imports(A,B),web_framework(B).
integrity_violation(panic_in_library,A) :-
   uses_panic(A),layer(A,B),library_layer(B).
integrity_violation(low_coverage,A) :-
   coverage(A,B),B<80.
integrity_violation(layer_inversion,A) :-
   imports(A,B),layer(A,C),layer(B,D),layer_index(C,E),layer_index(D,F),E<F.
integrity_violation(missing_test,A) :-
   source_file(A), \+covered_by(A,_), \+is_main_file(A), \+doc_only_file(A).
integrity_violation(framework_in_domain,A) :-
   layer(A,domain),imports(A,B),web_framework(B).
integrity_violation(panic_in_library,A) :-
   uses_panic(A),layer(A,B),library_layer(B).
integrity_violation(low_coverage,A) :-
   coverage(A,B),B<80.
requirement(ac1,'Bug no longer reproduces with steps above').
requirement(ac2,'No regression introduced in related functionality').
requirement(ac3,'Test added to prevent recurrence').
requirement(ac4,'Edge cases from same code path verified').
requirement(ac5,'At least three successive resumptions of one workflow persist without uniqueness warnings or manual deletion of SQLite or state files.').
requirement(ac6,'Failed, interrupted, and successful attempts remain individually queryable under one stable workflow ID.').
requirement(ac7,'Step-execution failures and history-persistence failures produce distinct messages and cause-appropriate exit behavior.').
requirement(ac8,'Parent and child workflows, resumptions after interruption, and duplicate termination notifications are covered by deterministic tests.').
requirement(ac9,'Concurrent persistence attempts are tested when supported by the selected history contract.').
requirement(ac10,'Existing history data remains readable after any required migration.').
requirement(ac11,'No user workflow or ZPM database is modified as a workaround.').
tms_justification(requirement(ac1,'Bug no longer reproduces with steps above'),spec_rev_b020_1).
tms_justification(requirement(ac2,'No regression introduced in related functionality'),spec_rev_b020_1).
tms_justification(requirement(ac3,'Test added to prevent recurrence'),spec_rev_b020_1).
tms_justification(requirement(ac4,'Edge cases from same code path verified'),spec_rev_b020_1).
tms_justification(requirement(ac5,'At least three successive resumptions of one workflow persist without uniqueness warnings or manual deletion of SQLite or state files.'),spec_rev_b020_1).
tms_justification(requirement(ac6,'Failed, interrupted, and successful attempts remain individually queryable under one stable workflow ID.'),spec_rev_b020_1).
tms_justification(requirement(ac7,'Step-execution failures and history-persistence failures produce distinct messages and cause-appropriate exit behavior.'),spec_rev_b020_1).
tms_justification(requirement(ac8,'Parent and child workflows, resumptions after interruption, and duplicate termination notifications are covered by deterministic tests.'),spec_rev_b020_1).
tms_justification(requirement(ac9,'Concurrent persistence attempts are tested when supported by the selected history contract.'),spec_rev_b020_1).
tms_justification(requirement(ac10,'Existing history data remains readable after any required migration.'),spec_rev_b020_1).
tms_justification(requirement(ac11,'No user workflow or ZPM database is modified as a workaround.'),spec_rev_b020_1).
tms_justification(requirement(ac1,'Bug no longer reproduces with steps above'),spec_rev_b020_1).
tms_justification(requirement(ac2,'No regression introduced in related functionality'),spec_rev_b020_1).
tms_justification(requirement(ac3,'Test added to prevent recurrence'),spec_rev_b020_1).
tms_justification(requirement(ac4,'Edge cases from same code path verified'),spec_rev_b020_1).
tms_justification(requirement(ac5,'At least three successive resumptions of one workflow persist without uniqueness warnings or manual deletion of SQLite or state files.'),spec_rev_b020_1).
tms_justification(requirement(ac6,'Failed, interrupted, and successful attempts remain individually queryable under one stable workflow ID.'),spec_rev_b020_1).
tms_justification(requirement(ac7,'Step-execution failures and history-persistence failures produce distinct messages and cause-appropriate exit behavior.'),spec_rev_b020_1).
tms_justification(requirement(ac8,'Parent and child workflows, resumptions after interruption, and duplicate termination notifications are covered by deterministic tests.'),spec_rev_b020_1).
tms_justification(requirement(ac9,'Concurrent persistence attempts are tested when supported by the selected history contract.'),spec_rev_b020_1).
tms_justification(requirement(ac10,'Existing history data remains readable after any required migration.'),spec_rev_b020_1).
tms_justification(requirement(ac11,'No user workflow or ZPM database is modified as a workaround.'),spec_rev_b020_1).
zpm_source(observation(b017_run_validation_path_gap,architecture,'Adapter.Validate calls WorkflowService.ValidateLoadedWorkflow, but Adapter.Run resolves and dispatches directly to ExecutionService without that validation; validation or preflight intended to protect execution must be wired explicitly into the run path and verified through the production CLI composition root.','2026-10-23'),interactive).
zpm_source(observation(b017_run_validation_path_gap,architecture,'Adapter.Validate calls WorkflowService.ValidateLoadedWorkflow, but Adapter.Run resolves and dispatches directly to ExecutionService without that validation; validation or preflight intended to protect execution must be wired explicitly into the run path and verified through the production CLI composition root.','2026-10-23'),interactive).
zpm_source(observation(cli_root_constructor_contract,architecture,'NewRootCommand builds the public command tree without auto-wiring cfg.Facade; therefore validate intentionally returns a facade-wiring error from this constructor, while run remains functional because its path builds execution dependencies. Public input/configuration regressions required to start at NewRootCommand should use run; validate behavior belongs in facade/application tests unless production wiring is intentionally changed.','2026-10-23'),interactive).
workflow_run(commit,'2026-10-23',success).
workflow_run(commit,'2026-10-23',success).
observation(plan_find_spec_timeout_masks_memory_stage,pitfall,'Global plan workflow run b8ce1144-f9c2-4c6c-9092-030a030bf6de in zpm for B005 failed find_spec at 30s with ExitCode -1/context deadline exceeded. GitHub issue 65 exists; spec-content.md was written two seconds after step start, locating the remaining delay in feature set spec (ZPM persistence/projections), not issue lookup. Exact lock versus ZPM command cause unproven. Global feature-memory state.py uses blocking flock and subprocess.run zpm without timeouts; native.py set also regenerates snapshots. Terminal error_find_spec misleadingly reports issue not found and post hook prints success text on failure.','2026-10-22').
observation(zpm_project_cold_start_performance_20261022,performance,'Measured installed ZPM 0.4.1 against copied /home/pocky/Sites/awf-project/zpm/.zpm in /tmp/zpm-perf-o_v_4z3_: get-persistence-status 5.358s, list-memories 5.315s, feature_b005 empty spec query 5.285s. Moving 14 root kb/*.pl files totaling 22282807 bytes out of copy reduces list-memories to 0.278s; limiting mounts to default+feature_b005 further reduces to 0.057s; empty store 0.030s. Original base unchanged. b004-cleanup-research.pl has 51118 noncomment lines but only 170 distinct, max repetition 2303. Bootstrap loads all root .pl files and mounted segments per process. Global feature-memory/state.py open loops over 14 document queries plus task read and native snapshots; repeated process startup amplifies overhead. Historical B005 init 126.8s and existence check 11.8s. Duplicate origin not established; archive separation and batching are recommendations, not implemented fixes.','2026-10-22').
observation(go_testmain_mrun_discovery,pitfall,'A Go package TestMain that returns without calling m.Run can make go test and focused discovery exit successfully while executing zero tests; verification must require exact test events or listed names.','2026-10-22').
observation(b019_t001_empty_audit_gate,pitfall,'An unfiltered internal/interfaces/cli test run can report all tests passing yet exit nonzero because TestMain verifyRedirectedAudit rejects an empty XDG_DATA_HOME/awf/audit.jsonl; preserve task contracts that intentionally create no global audit entry and address the package verifier in its owning task','2026-10-22').
observation(b019_test_fixture_pattern,pattern,'B019 CLI tests isolate default audit persistence by setting a package-owned temporary XDG_DATA_HOME in TestMain, while setupTestDir remains config-neutral and setupProjectConfig opts individual tests into an exact AWF_CONFIG_PATH; configured-input regressions require successful command execution and assert resolved values in observable output.','2026-10-23').
observation(b019_completeness_review_20261023,review,'Completeness review found T002 complete, T001 and T003 partial because execute-only listed test files existed but were not modified; 17 unplanned changes were recorded, including controller-owned ZPM files.','2026-10-23').
observation(b019_subworkflow_continue_on_error_pattern,pattern,'Failed-step routing through resolveNextStep must treat nextStep nonempty OR ContinueOnError true as handled; checking only a nonempty target regresses ContinueOnError when OnSuccess and transitions are empty. Match ExecutionService and InteractiveExecutor failure handlers.','2026-10-23').
observation(b017_run_validation_path_gap,architecture,'Adapter.Validate calls WorkflowService.ValidateLoadedWorkflow, but Adapter.Run resolves and dispatches directly to ExecutionService without that validation; validation or preflight intended to protect execution must be wired explicitly into the run path and verified through the production CLI composition root.','2026-10-23').
observation(plan_find_spec_timeout_masks_memory_stage,pitfall,'Global plan workflow run b8ce1144-f9c2-4c6c-9092-030a030bf6de in zpm for B005 failed find_spec at 30s with ExitCode -1/context deadline exceeded. GitHub issue 65 exists; spec-content.md was written two seconds after step start, locating the remaining delay in feature set spec (ZPM persistence/projections), not issue lookup. Exact lock versus ZPM command cause unproven. Global feature-memory state.py uses blocking flock and subprocess.run zpm without timeouts; native.py set also regenerates snapshots. Terminal error_find_spec misleadingly reports issue not found and post hook prints success text on failure.','2026-10-22').
observation(zpm_project_cold_start_performance_20261022,performance,'Measured installed ZPM 0.4.1 against copied /home/pocky/Sites/awf-project/zpm/.zpm in /tmp/zpm-perf-o_v_4z3_: get-persistence-status 5.358s, list-memories 5.315s, feature_b005 empty spec query 5.285s. Moving 14 root kb/*.pl files totaling 22282807 bytes out of copy reduces list-memories to 0.278s; limiting mounts to default+feature_b005 further reduces to 0.057s; empty store 0.030s. Original base unchanged. b004-cleanup-research.pl has 51118 noncomment lines but only 170 distinct, max repetition 2303. Bootstrap loads all root .pl files and mounted segments per process. Global feature-memory/state.py open loops over 14 document queries plus task read and native snapshots; repeated process startup amplifies overhead. Historical B005 init 126.8s and existence check 11.8s. Duplicate origin not established; archive separation and batching are recommendations, not implemented fixes.','2026-10-22').
observation(go_testmain_mrun_discovery,pitfall,'A Go package TestMain that returns without calling m.Run can make go test and focused discovery exit successfully while executing zero tests; verification must require exact test events or listed names.','2026-10-22').
observation(b019_t001_empty_audit_gate,pitfall,'An unfiltered internal/interfaces/cli test run can report all tests passing yet exit nonzero because TestMain verifyRedirectedAudit rejects an empty XDG_DATA_HOME/awf/audit.jsonl; preserve task contracts that intentionally create no global audit entry and address the package verifier in its owning task','2026-10-22').
observation(b019_test_fixture_pattern,pattern,'B019 CLI tests isolate default audit persistence by setting a package-owned temporary XDG_DATA_HOME in TestMain, while setupTestDir remains config-neutral and setupProjectConfig opts individual tests into an exact AWF_CONFIG_PATH; configured-input regressions require successful command execution and assert resolved values in observable output.','2026-10-23').
observation(b019_completeness_review_20261023,review,'Completeness review found T002 complete, T001 and T003 partial because execute-only listed test files existed but were not modified; 17 unplanned changes were recorded, including controller-owned ZPM files.','2026-10-23').
observation(b019_subworkflow_continue_on_error_pattern,pattern,'Failed-step routing through resolveNextStep must treat nextStep nonempty OR ContinueOnError true as handled; checking only a nonempty target regresses ContinueOnError when OnSuccess and transitions are empty. Match ExecutionService and InteractiveExecutor failure handlers.','2026-10-23').
observation(b017_run_validation_path_gap,architecture,'Adapter.Validate calls WorkflowService.ValidateLoadedWorkflow, but Adapter.Run resolves and dispatches directly to ExecutionService without that validation; validation or preflight intended to protect execution must be wired explicitly into the run path and verified through the production CLI composition root.','2026-10-23').
observation(cli_root_constructor_contract,architecture,'NewRootCommand builds the public command tree without auto-wiring cfg.Facade; therefore validate intentionally returns a facade-wiring error from this constructor, while run remains functional because its path builds execution dependencies. Public input/configuration regressions required to start at NewRootCommand should use run; validate behavior belongs in facade/application tests unless production wiring is intentionally changed.','2026-10-23').
