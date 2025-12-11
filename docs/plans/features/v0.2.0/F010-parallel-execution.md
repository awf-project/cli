# F010: Exécution Parallèle (errgroup)

## Metadata
- **Status**: implemented
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priority**: high
- **Estimation**: L

## Description

Implement parallel step execution using Go's errgroup. Support parallel state type that runs multiple steps concurrently. Implement different strategies: all_succeed, any_succeed, best_effort. Control max concurrency with semaphore.

## Acceptance Criteria

- [x] Execute parallel steps concurrently
- [x] Respect max_concurrent limit
- [x] all_succeed: fail if any step fails
- [x] any_succeed: succeed if any step succeeds
- [x] best_effort: collect all results
- [x] Cancel remaining on first failure (all_succeed)
- [x] Each parallel step output accessible separately

## Dependencies

- **Blocked by**: F009
- **Unblocks**: _none_

## Impacted Files

```
internal/application/parallel_executor.go
internal/domain/workflow/parallel.go
internal/domain/workflow/state.go
go.mod (add golang.org/x/sync)
```

## Technical Tasks

- [x] Add errgroup dependency
  - [x] `go get golang.org/x/sync/errgroup`
- [x] Define ParallelStrategy enum
  - [x] all_succeed
  - [x] any_succeed
  - [x] best_effort
- [x] Define ParallelState struct
  - [x] Steps list
  - [x] Strategy
  - [x] MaxConcurrent
- [x] Implement ParallelExecutor
  - [x] Create errgroup with context
  - [x] Implement semaphore for max_concurrent
  - [x] Launch goroutines for each step
  - [x] Collect results per step
  - [x] Apply strategy logic
- [x] Handle context cancellation
  - [x] Cancel all on first error (all_succeed)
  - [x] Wait for all (best_effort)
- [x] Store individual outputs
  - [x] `{{states.parallel_step.steps.step_name.output}}`
- [x] Write unit tests with mock executor
- [x] Write integration tests

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
