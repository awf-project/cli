# Next Features to Implement

## Priority Queue (v0.2.0)

### F010: Parallel Execution
**Priority:** High | **Estimation:** L

Execute steps concurrently with errgroup:
- Strategies: `all_succeed`, `any_succeed`, `best_effort`
- Semaphore for `max_concurrent` limit
- Individual step outputs accessible

```yaml
parallel_step:
  type: parallel
  parallel: [step1, step2, step3]
  strategy: all_succeed
  max_concurrent: 2
```

**Key files:**
- `internal/application/parallel_executor.go` (NEW)
- `internal/domain/workflow/parallel.go` (NEW)

### F013: Workflow Resume
**Priority:** High | **Estimation:** M

Resume interrupted workflows:
- `awf resume <workflow-id>`
- `awf resume --list` for resumable workflows
- Skip completed states, reuse outputs
- Allow input override on resume

**Key files:**
- `internal/interfaces/cli/commands/resume.go` (NEW)
- `internal/application/service.go` (MODIFY)

### F011: Retry with Backoff
**Priority:** High | **Estimation:** M

Already partially in domain model (`RetryConfig`), needs:
- Retry executor implementation
- Backoff strategies: constant, linear, exponential
- Jitter support
- Retryable exit codes filter

## Future Highlight: F032 Agent Step Type (v0.4.0)

First-class AI agent integration:
```yaml
- name: analyze
  type: agent
  provider: claude
  prompt: "Analyze this code: {{inputs.code}}"
  options:
    model: claude-sonnet-4-20250514
    max_tokens: 4096
```

Providers: claude, codex, gemini, opencode, custom

**Key principle:** Non-interactive by design - prompts are inputs, no stdin passthrough.

## Implementation Notes

### Parallel Execution Pattern
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

### Resume Flow
1. Load state from JSONStore
2. Validate status != completed
3. Merge input overrides
4. Continue from `current_state`
5. Skip states with status == completed
