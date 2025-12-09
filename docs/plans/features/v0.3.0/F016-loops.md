# F016: Boucles (for/while)

## Metadata
- **Statut**: backlog
- **Phase**: 3-Advanced
- **Version**: v0.3.0
- **Priorité**: medium
- **Estimation**: L

## Description

Add loop constructs for iterating over lists or until a condition is met. Support for-each iteration over arrays and while loops with conditions. Enable data processing workflows that handle multiple items.

## Critères d'Acceptance

- [ ] `for_each:` iterates over list
- [ ] `while:` repeats until condition false
- [ ] Loop variable accessible via `{{loop.item}}`
- [ ] Loop index via `{{loop.index}}`
- [ ] Max iterations safety limit
- [ ] Break condition support
- [ ] Collect loop outputs

## Dépendances

- **Bloqué par**: F015
- **Débloque**: _none_

## Fichiers Impactés

```
internal/domain/workflow/loop.go
internal/domain/workflow/state.go
internal/application/executor.go
internal/application/loop_executor.go
```

## Tâches Techniques

- [ ] Define LoopState struct
  - [ ] Type (for_each, while)
  - [ ] Items (for for_each)
  - [ ] Condition (for while)
  - [ ] Body (states to execute)
  - [ ] MaxIterations
  - [ ] BreakCondition
- [ ] Implement ForEachExecutor
  - [ ] Parse items from variable or literal
  - [ ] Execute body for each item
  - [ ] Set loop.item, loop.index
  - [ ] Collect outputs
- [ ] Implement WhileExecutor
  - [ ] Evaluate condition
  - [ ] Execute body while true
  - [ ] Check max iterations
  - [ ] Check break condition
- [ ] Add loop context variables
  - [ ] loop.item
  - [ ] loop.index
  - [ ] loop.first
  - [ ] loop.last
  - [ ] loop.length
- [ ] Handle nested loops
- [ ] Write unit tests
- [ ] Write integration tests

## Notes

For-each syntax:
```yaml
process_files:
  type: for_each
  items: "{{inputs.files}}"  # or literal: ["a.txt", "b.txt"]
  max_iterations: 100
  body:
    - process_single:
        type: step
        command: "process {{loop.item}}"
        capture:
          stdout: "result_{{loop.index}}"
  on_complete: aggregate
```

While syntax:
```yaml
poll_status:
  type: while
  condition: "states.check.output != 'ready'"
  max_iterations: 60
  body:
    - check:
        type: step
        command: "curl -s api/status"
        capture:
          stdout: status
    - wait:
        type: step
        command: "sleep 5"
  on_complete: proceed
```
