# F023: Workflow Composition (Sous-workflows)

## Metadata
- **Statut**: backlog
- **Phase**: 4-Extensibility
- **Version**: v0.4.0
- **Priorité**: medium
- **Estimation**: L

## Description

Enable calling one workflow from another as a sub-workflow. Pass inputs, receive outputs, and handle errors. Support both inline and referenced sub-workflows. Enable modular workflow design.

## Critères d'Acceptance

- [ ] `call_workflow:` invokes another workflow
- [ ] Pass inputs to sub-workflow
- [ ] Capture sub-workflow outputs
- [ ] Sub-workflow errors propagate correctly
- [ ] Nested sub-workflows supported
- [ ] Prevent circular workflow calls
- [ ] Sub-workflow state visible in parent
- [ ] Timeout for sub-workflow execution

## Dépendances

- **Bloqué par**: F002, F003, F017
- **Débloque**: _none_

## Fichiers Impactés

```
internal/domain/workflow/subworkflow.go
internal/application/subworkflow_executor.go
internal/domain/workflow/state.go
```

## Tâches Techniques

- [ ] Define SubWorkflowState struct
  - [ ] Workflow name/path
  - [ ] Input mappings
  - [ ] Output mappings
  - [ ] Timeout
- [ ] Implement SubWorkflowExecutor
  - [ ] Load sub-workflow definition
  - [ ] Map parent inputs to sub-workflow inputs
  - [ ] Execute sub-workflow
  - [ ] Map sub-workflow outputs to parent
  - [ ] Handle errors
- [ ] Track call stack
  - [ ] Detect circular calls
  - [ ] Limit nesting depth
- [ ] State naming in parent
  - [ ] `states.subworkflow_state.output`
  - [ ] Access sub-workflow internal states?
- [ ] Handle sub-workflow persistence
  - [ ] Include in parent state?
  - [ ] Separate state files?
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

Sub-workflow invocation:
```yaml
states:
  analyze_all:
    type: call_workflow
    workflow: analyze-single-file
    inputs:
      file_path: "{{loop.item}}"
      max_tokens: "{{inputs.max_tokens}}"
    outputs:
      result: analysis_result
    timeout: 300s
    on_success: aggregate
    on_failure: handle_error
```

Sub-workflow definition (analyze-single-file.yaml):
```yaml
name: analyze-single-file
inputs:
  - name: file_path
    required: true
  - name: max_tokens
    default: 2000
states:
  # ... workflow states
outputs:
  - name: analysis_result
    from: states.format.output
```
