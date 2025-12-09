# F010: Exécution Parallèle (errgroup)

## Metadata
- **Statut**: backlog
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priorité**: high
- **Estimation**: L

## Description

Implement parallel step execution using Go's errgroup. Support parallel state type that runs multiple steps concurrently. Implement different strategies: all_succeed, any_succeed, best_effort. Control max concurrency with semaphore.

## Critères d'Acceptance

- [ ] Execute parallel steps concurrently
- [ ] Respect max_concurrent limit
- [ ] all_succeed: fail if any step fails
- [ ] any_succeed: succeed if any step succeeds
- [ ] best_effort: collect all results
- [ ] Cancel remaining on first failure (all_succeed)
- [ ] Each parallel step output accessible separately

## Dépendances

- **Bloqué par**: F009
- **Débloque**: _none_

## Fichiers Impactés

```
internal/application/parallel_executor.go
internal/domain/workflow/parallel.go
internal/domain/workflow/state.go
go.mod (add golang.org/x/sync)
```

## Tâches Techniques

- [ ] Add errgroup dependency
  - [ ] `go get golang.org/x/sync/errgroup`
- [ ] Define ParallelStrategy enum
  - [ ] all_succeed
  - [ ] any_succeed
  - [ ] best_effort
- [ ] Define ParallelState struct
  - [ ] Steps list
  - [ ] Strategy
  - [ ] MaxConcurrent
- [ ] Implement ParallelExecutor
  - [ ] Create errgroup with context
  - [ ] Implement semaphore for max_concurrent
  - [ ] Launch goroutines for each step
  - [ ] Collect results per step
  - [ ] Apply strategy logic
- [ ] Handle context cancellation
  - [ ] Cancel all on first error (all_succeed)
  - [ ] Wait for all (best_effort)
- [ ] Store individual outputs
  - [ ] `{{states.parallel_step.steps.step_name.output}}`
- [ ] Write unit tests with mock executor
- [ ] Write integration tests

## Notes

Implementation pattern:
```go
g, ctx := errgroup.WithContext(ctx)
sem := make(chan struct{}, maxConcurrent)

for _, step := range steps {
    step := step
    g.Go(func() error {
        sem <- struct{}{}
        defer func() { <-sem }()
        return r.executeStep(ctx, step)
    })
}
return g.Wait()
```

Warning: Parallel steps must NOT write to same files (no locking).
