# Implementation Plan: F016 - Loop Constructs

## Summary

Implement `for_each` and `while` loop constructs following the established parallel execution pattern. The feature adds two new step types that iterate over items or until a condition is met, exposing `{{loop.item}}`, `{{loop.index}}`, and related variables for template interpolation. Implementation follows strict hexagonal architecture: domain entities first, then interpolation support, infrastructure parsing, and finally application-layer executors.

## ASCII Wireframe: Component Interactions

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         F016 LOOP IMPLEMENTATION                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  YAML Input                                                                 │
│  ┌─────────────────────┐                                                    │
│  │ poll_api:           │                                                    │
│  │   type: for_each    │                                                    │
│  │   items: {{inputs}} │                                                    │
│  │   body: [fetch]     │                                                    │
│  │   max_iterations:100│                                                    │
│  │   on_complete: done │                                                    │
│  └──────────┬──────────┘                                                    │
│             │                                                               │
│             ▼                                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ INFRASTRUCTURE: yaml_types.go + yaml_mapper.go                       │   │
│  │ yamlStep.Items/While/Body/MaxIterations → domain.Step.Loop           │   │
│  └──────────┬──────────────────────────────────────────────────────────┘   │
│             │                                                               │
│             ▼                                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ DOMAIN: loop.go + step.go                                            │   │
│  │ ┌───────────────┐  ┌───────────────┐  ┌────────────────┐            │   │
│  │ │ LoopConfig    │  │ LoopResult    │  │ StepType       │            │   │
│  │ │ - Type        │  │ - Iterations  │  │ + ForEach      │            │   │
│  │ │ - Items       │  │ - TotalCount  │  │ + While        │            │   │
│  │ │ - Condition   │  │ - BrokeAt     │  └────────────────┘            │   │
│  │ │ - Body        │  └───────────────┘                                │   │
│  │ │ - MaxIter     │                                                   │   │
│  │ └───────────────┘                                                   │   │
│  └──────────┬──────────────────────────────────────────────────────────┘   │
│             │                                                               │
│             ▼                                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ PKG/INTERPOLATION: resolver.go + template_resolver.go                │   │
│  │ Context.Loop = &LoopData{Item: "file.txt", Index: 0, First: true}   │   │
│  │ Template: "echo {{loop.item}}" → "echo file.txt"                    │   │
│  └──────────┬──────────────────────────────────────────────────────────┘   │
│             │                                                               │
│             ▼                                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ APPLICATION: loop_executor.go + execution_service.go                 │   │
│  │                                                                      │   │
│  │  LoopExecutor.ExecuteForEach(ctx, step, execCtx)                    │   │
│  │  ┌──────────────────────────────────────────────────┐               │   │
│  │  │ for i, item := range items {                     │               │   │
│  │  │   ctx.Loop = {Item: item, Index: i, ...}         │               │   │
│  │  │   for _, bodyStep := range step.Loop.Body {      │               │   │
│  │  │     executeStep(bodyStep)                        │               │   │
│  │  │   }                                              │               │   │
│  │  │   if breakCondition { break }                    │               │   │
│  │  │ }                                                │               │   │
│  │  └──────────────────────────────────────────────────┘               │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Domain Layer - Loop Entities

**File:** `internal/domain/workflow/loop.go`
**Action:** CREATE
**Complexity:** M

```go
package workflow

import "time"

// LoopType defines the type of loop construct.
type LoopType string

const (
    LoopTypeForEach LoopType = "for_each"
    LoopTypeWhile   LoopType = "while"
)

// LoopConfig holds configuration for loop execution.
type LoopConfig struct {
    Type           LoopType // for_each or while
    Items          string   // template expression for items (for_each)
    Condition      string   // expression to evaluate (while)
    Body           []string // step names to execute each iteration
    MaxIterations  int      // safety limit (default: 100, max: 10000)
    BreakCondition string   // optional early exit expression
    OnComplete     string   // next state after loop completes
}

// DefaultMaxIterations is the default iteration limit.
const DefaultMaxIterations = 100

// MaxAllowedIterations is the hard limit for safety.
const MaxAllowedIterations = 10000

// Validate checks if the loop configuration is valid.
func (c *LoopConfig) Validate() error

// IterationResult holds the result of a single loop iteration.
type IterationResult struct {
    Index       int
    Item        any // for for_each
    StepResults map[string]*StepState
    Error       error
    StartedAt   time.Time
    CompletedAt time.Time
}

// LoopResult holds aggregated results of loop execution.
type LoopResult struct {
    Iterations []IterationResult
    TotalCount int
    BrokeAt    int // -1 if completed normally, index if break triggered
    StartedAt  time.Time
    CompletedAt time.Time
}
```

---

### Step 2: Domain Layer - Extend Step Types

**File:** `internal/domain/workflow/step.go`
**Action:** MODIFY
**Complexity:** S

**Changes:**
1. Add new `StepType` constants:
```go
const (
    StepTypeCommand  StepType = "command"
    StepTypeParallel StepType = "parallel"
    StepTypeTerminal StepType = "terminal"
    StepTypeForEach  StepType = "for_each"  // NEW
    StepTypeWhile    StepType = "while"     // NEW
)
```

2. Add `Loop` field to `Step` struct:
```go
type Step struct {
    // ... existing fields ...
    Loop *LoopConfig // for for_each and while types
}
```

3. Extend `Validate()` method:
```go
case StepTypeForEach:
    if s.Loop == nil {
        return errors.New("loop config is required for for_each steps")
    }
    if s.Loop.Items == "" {
        return errors.New("items is required for for_each steps")
    }
    if len(s.Loop.Body) == 0 {
        return errors.New("body is required for loop steps")
    }
case StepTypeWhile:
    if s.Loop == nil {
        return errors.New("loop config is required for while steps")
    }
    if s.Loop.Condition == "" {
        return errors.New("condition is required for while steps")
    }
    if len(s.Loop.Body) == 0 {
        return errors.New("body is required for loop steps")
    }
```

---

### Step 3: Interpolation Package - Add Loop Context

**File:** `pkg/interpolation/resolver.go`
**Action:** MODIFY
**Complexity:** S

**Changes:**
1. Add `LoopData` struct:
```go
// LoopData holds loop iteration context for interpolation.
type LoopData struct {
    Item   any  // current item value (for_each)
    Index  int  // 0-based iteration index
    First  bool // true on first iteration
    Last   bool // true on last iteration (for_each only)
    Length int  // total items count (for_each only, -1 for while)
}
```

2. Add `Loop` field to `Context`:
```go
type Context struct {
    Inputs   map[string]any
    States   map[string]StepStateData
    Workflow WorkflowData
    Env      map[string]string
    Context  ContextData
    Error    *ErrorData
    Loop     *LoopData  // NEW: loop iteration data
}
```

---

### Step 4: Interpolation Package - Template Support

**File:** `pkg/interpolation/template_resolver.go`
**Action:** MODIFY
**Complexity:** S

**Changes:**
Update `buildTemplateData()` to include loop namespace:
```go
func (r *TemplateResolver) buildTemplateData(ctx *Context) map[string]any {
    data := map[string]any{
        "inputs":   ctx.Inputs,
        "states":   ctx.States,
        "workflow": ctx.Workflow,
        "env":      ctx.Env,
        "context":  ctx.Context,
    }
    if ctx.Error != nil {
        data["error"] = ctx.Error
    }
    if ctx.Loop != nil {  // NEW
        data["loop"] = ctx.Loop
    }
    return data
}
```

---

### Step 5: Infrastructure Layer - YAML Types

**File:** `internal/infrastructure/repository/yaml_types.go`
**Action:** MODIFY
**Complexity:** S

**Changes:**
Add loop fields to `yamlStep`:
```go
type yamlStep struct {
    // ... existing fields ...
    
    // Loop configuration (for for_each and while types)
    Items         any      `yaml:"items"`          // string or []any for for_each
    While         string   `yaml:"while"`          // condition for while loop
    Body          []string `yaml:"body"`           // steps to execute each iteration
    MaxIterations int      `yaml:"max_iterations"` // safety limit
    BreakWhen     string   `yaml:"break_when"`     // optional break condition
    OnComplete    string   `yaml:"on_complete"`    // next state after loop
}
```

---

### Step 6: Infrastructure Layer - YAML Mapper

**File:** `internal/infrastructure/repository/yaml_mapper.go`
**Action:** MODIFY
**Complexity:** M

**Changes:**
1. Update `parseStepType()`:
```go
func parseStepType(s string) (workflow.StepType, error) {
    switch strings.ToLower(s) {
    case "step", "command":
        return workflow.StepTypeCommand, nil
    case "parallel":
        return workflow.StepTypeParallel, nil
    case "terminal":
        return workflow.StepTypeTerminal, nil
    case "for_each":  // NEW
        return workflow.StepTypeForEach, nil
    case "while":     // NEW
        return workflow.StepTypeWhile, nil
    default:
        return "", NewParseError("", "", "unknown step type: "+s)
    }
}
```

2. Add `mapLoopConfig()` function:
```go
func mapLoopConfig(y yamlStep) *workflow.LoopConfig {
    // Determine loop type
    var loopType workflow.LoopType
    var items, condition string
    
    if y.Items != nil {
        loopType = workflow.LoopTypeForEach
        switch v := y.Items.(type) {
        case string:
            items = v
        case []any:
            // Convert to JSON string for later parsing
            b, _ := json.Marshal(v)
            items = string(b)
        }
    } else if y.While != "" {
        loopType = workflow.LoopTypeWhile
        condition = y.While
    } else {
        return nil
    }
    
    maxIter := y.MaxIterations
    if maxIter == 0 {
        maxIter = workflow.DefaultMaxIterations
    }
    
    return &workflow.LoopConfig{
        Type:           loopType,
        Items:          items,
        Condition:      condition,
        Body:           y.Body,
        MaxIterations:  maxIter,
        BreakCondition: y.BreakWhen,
        OnComplete:     y.OnComplete,
    }
}
```

3. Update `mapStep()` to call `mapLoopConfig()`:
```go
func mapStep(filePath, name string, y yamlStep) (*workflow.Step, error) {
    // ... existing code ...
    
    step := &workflow.Step{
        // ... existing fields ...
        Loop: mapLoopConfig(y),  // NEW
    }
    
    // ... rest of function ...
}
```

---

### Step 7: Application Layer - Loop Executor

**File:** `internal/application/loop_executor.go`
**Action:** CREATE
**Complexity:** L

```go
package application

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/vanoix/awf/internal/domain/ports"
    "github.com/vanoix/awf/internal/domain/workflow"
    "github.com/vanoix/awf/pkg/interpolation"
)

// LoopExecutor executes for_each and while loop constructs.
type LoopExecutor struct {
    logger    ports.Logger
    evaluator ExpressionEvaluator
    resolver  interpolation.Resolver
}

// NewLoopExecutor creates a new LoopExecutor.
func NewLoopExecutor(
    logger ports.Logger,
    evaluator ExpressionEvaluator,
    resolver interpolation.Resolver,
) *LoopExecutor

// ExecuteForEach iterates over items and executes body steps for each.
func (e *LoopExecutor) ExecuteForEach(
    ctx context.Context,
    wf *workflow.Workflow,
    step *workflow.Step,
    execCtx *workflow.ExecutionContext,
    stepExecutor StepExecutorFunc,
    buildContext ContextBuilderFunc,
) (*workflow.LoopResult, error) {
    result := &workflow.LoopResult{
        StartedAt: time.Now(),
        BrokeAt:   -1,
    }
    
    // 1. Resolve items expression
    intCtx := buildContext(execCtx)
    itemsStr, err := e.resolver.Resolve(step.Loop.Items, intCtx)
    if err != nil {
        return nil, fmt.Errorf("resolve items: %w", err)
    }
    
    // 2. Parse items (JSON array or comma-separated)
    items, err := e.parseItems(itemsStr)
    if err != nil {
        return nil, fmt.Errorf("parse items: %w", err)
    }
    
    // 3. Check max iterations
    if len(items) > step.Loop.MaxIterations {
        return nil, fmt.Errorf("items count %d exceeds max_iterations %d", 
            len(items), step.Loop.MaxIterations)
    }
    
    // 4. Execute loop body for each item
    for i, item := range items {
        if ctx.Err() != nil {
            return result, ctx.Err()
        }
        
        iterResult := workflow.IterationResult{
            Index:       i,
            Item:        item,
            StepResults: make(map[string]*workflow.StepState),
            StartedAt:   time.Now(),
        }
        
        // Set loop context
        intCtx = buildContext(execCtx)
        intCtx.Loop = &interpolation.LoopData{
            Item:   item,
            Index:  i,
            First:  i == 0,
            Last:   i == len(items)-1,
            Length: len(items),
        }
        
        // Execute body steps
        for _, bodyStepName := range step.Loop.Body {
            if err := stepExecutor(ctx, bodyStepName, intCtx); err != nil {
                iterResult.Error = err
                break
            }
            // Capture step state
            if state, ok := execCtx.GetStepState(bodyStepName); ok {
                stateCopy := state
                iterResult.StepResults[bodyStepName] = &stateCopy
            }
        }
        
        iterResult.CompletedAt = time.Now()
        result.Iterations = append(result.Iterations, iterResult)
        result.TotalCount++
        
        // Check break condition
        if step.Loop.BreakCondition != "" {
            intCtx = buildContext(execCtx)
            intCtx.Loop = &interpolation.LoopData{Item: item, Index: i}
            shouldBreak, err := e.evaluator.Evaluate(step.Loop.BreakCondition, intCtx)
            if err != nil {
                e.logger.Warn("break condition evaluation failed", "error", err)
            } else if shouldBreak {
                result.BrokeAt = i
                break
            }
        }
        
        // Check for iteration error
        if iterResult.Error != nil {
            return result, iterResult.Error
        }
    }
    
    result.CompletedAt = time.Now()
    return result, nil
}

// ExecuteWhile repeats body steps while condition is true.
func (e *LoopExecutor) ExecuteWhile(
    ctx context.Context,
    wf *workflow.Workflow,
    step *workflow.Step,
    execCtx *workflow.ExecutionContext,
    stepExecutor StepExecutorFunc,
    buildContext ContextBuilderFunc,
) (*workflow.LoopResult, error) {
    result := &workflow.LoopResult{
        StartedAt: time.Now(),
        BrokeAt:   -1,
    }
    
    for i := 0; i < step.Loop.MaxIterations; i++ {
        if ctx.Err() != nil {
            return result, ctx.Err()
        }
        
        // Evaluate while condition
        intCtx := buildContext(execCtx)
        intCtx.Loop = &interpolation.LoopData{
            Index:  i,
            First:  i == 0,
            Length: -1, // unknown for while loops
        }
        
        shouldContinue, err := e.evaluator.Evaluate(step.Loop.Condition, intCtx)
        if err != nil {
            return result, fmt.Errorf("evaluate condition: %w", err)
        }
        
        if !shouldContinue {
            break
        }
        
        iterResult := workflow.IterationResult{
            Index:       i,
            StepResults: make(map[string]*workflow.StepState),
            StartedAt:   time.Now(),
        }
        
        // Execute body steps
        for _, bodyStepName := range step.Loop.Body {
            if err := stepExecutor(ctx, bodyStepName, intCtx); err != nil {
                iterResult.Error = err
                break
            }
            if state, ok := execCtx.GetStepState(bodyStepName); ok {
                stateCopy := state
                iterResult.StepResults[bodyStepName] = &stateCopy
            }
        }
        
        iterResult.CompletedAt = time.Now()
        result.Iterations = append(result.Iterations, iterResult)
        result.TotalCount++
        
        // Check break condition
        if step.Loop.BreakCondition != "" {
            intCtx = buildContext(execCtx)
            intCtx.Loop = &interpolation.LoopData{Index: i}
            shouldBreak, _ := e.evaluator.Evaluate(step.Loop.BreakCondition, intCtx)
            if shouldBreak {
                result.BrokeAt = i
                break
            }
        }
        
        if iterResult.Error != nil {
            return result, iterResult.Error
        }
    }
    
    // Check if we hit max iterations without condition becoming false
    if result.TotalCount >= step.Loop.MaxIterations {
        e.logger.Warn("while loop hit max iterations", 
            "step", step.Name, "max", step.Loop.MaxIterations)
    }
    
    result.CompletedAt = time.Now()
    return result, nil
}

// parseItems converts items string to slice.
func (e *LoopExecutor) parseItems(itemsStr string) ([]any, error) {
    // Try JSON array first
    var items []any
    if err := json.Unmarshal([]byte(itemsStr), &items); err == nil {
        return items, nil
    }
    
    // Fallback: comma-separated strings
    parts := strings.Split(itemsStr, ",")
    items = make([]any, len(parts))
    for i, p := range parts {
        items[i] = strings.TrimSpace(p)
    }
    return items, nil
}

// Type definitions for callbacks
type StepExecutorFunc func(ctx context.Context, stepName string, intCtx *interpolation.Context) error
type ContextBuilderFunc func(execCtx *workflow.ExecutionContext) *interpolation.Context
```

---

### Step 8: Application Layer - Integrate Loop Executor

**File:** `internal/application/execution_service.go`
**Action:** MODIFY
**Complexity:** M

**Changes:**
1. Add `loopExecutor` field to `ExecutionService`:
```go
type ExecutionService struct {
    // ... existing fields ...
    loopExecutor *LoopExecutor  // NEW
}
```

2. Update constructors to initialize `loopExecutor`

3. Add `executeLoopStep()` method:
```go
func (s *ExecutionService) executeLoopStep(
    ctx context.Context,
    wf *workflow.Workflow,
    step *workflow.Step,
    execCtx *workflow.ExecutionContext,
) (string, error) {
    startTime := time.Now()
    
    // Apply step timeout
    stepCtx := ctx
    if step.Timeout > 0 {
        var cancel context.CancelFunc
        stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
        defer cancel()
    }
    
    // Execute pre-hooks
    intCtx := s.buildInterpolationContext(execCtx)
    if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx); err != nil {
        s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
    }
    
    // Create step executor callback
    stepExecutor := func(ctx context.Context, stepName string, intCtx *interpolation.Context) error {
        bodyStep, ok := wf.Steps[stepName]
        if !ok {
            return fmt.Errorf("body step not found: %s", stepName)
        }
        _, err := s.executeStep(ctx, wf, bodyStep, execCtx)
        return err
    }
    
    // Execute loop
    var result *workflow.LoopResult
    var err error
    
    if step.Type == workflow.StepTypeForEach {
        result, err = s.loopExecutor.ExecuteForEach(
            stepCtx, wf, step, execCtx, stepExecutor, s.buildInterpolationContext)
    } else {
        result, err = s.loopExecutor.ExecuteWhile(
            stepCtx, wf, step, execCtx, stepExecutor, s.buildInterpolationContext)
    }
    
    // Record loop step state
    loopState := workflow.StepState{
        Name:        step.Name,
        StartedAt:   startTime,
        CompletedAt: time.Now(),
    }
    
    if err != nil {
        loopState.Status = workflow.StatusFailed
        loopState.Error = err.Error()
        execCtx.SetStepState(step.Name, loopState)
        
        // Execute post-hooks even on failure
        intCtx = s.buildInterpolationContext(execCtx)
        s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx)
        
        if step.OnFailure != "" {
            return step.OnFailure, nil
        }
        return "", err
    }
    
    loopState.Status = workflow.StatusCompleted
    loopState.Output = fmt.Sprintf("completed %d iterations", result.TotalCount)
    execCtx.SetStepState(step.Name, loopState)
    
    // Execute post-hooks
    intCtx = s.buildInterpolationContext(execCtx)
    s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx)
    
    return step.Loop.OnComplete, nil
}
```

4. Update main execution loop in `Run()` and `executeFromStep()`:
```go
// In the execution loop, add case for loop types:
if step.Type == workflow.StepTypeParallel {
    nextStep, err = s.executeParallelStep(ctx, wf, step, execCtx)
} else if step.Type == workflow.StepTypeForEach || step.Type == workflow.StepTypeWhile {
    nextStep, err = s.executeLoopStep(ctx, wf, step, execCtx)  // NEW
} else {
    nextStep, err = s.executeStep(ctx, wf, step, execCtx)
}
```

---

### Step 9: Domain Validation - Body Step References

**File:** `internal/domain/workflow/validation.go`
**Action:** MODIFY
**Complexity:** S

**Changes:**
Add validation for loop body step references:
```go
// In ValidateWorkflow or Workflow.Validate():

// Validate loop body references
for name, step := range w.Steps {
    if step.Loop != nil && len(step.Loop.Body) > 0 {
        for _, bodyStepName := range step.Loop.Body {
            if _, ok := w.Steps[bodyStepName]; !ok {
                return fmt.Errorf("step '%s': loop body references unknown step '%s'", 
                    name, bodyStepName)
            }
        }
        if step.Loop.OnComplete != "" {
            if _, ok := w.Steps[step.Loop.OnComplete]; !ok {
                return fmt.Errorf("step '%s': on_complete references unknown state '%s'", 
                    name, step.Loop.OnComplete)
            }
        }
    }
}
```

---

## Test Plan

### Unit Tests

| File | Test Cases |
|------|------------|
| `internal/domain/workflow/loop_test.go` | `TestLoopConfig_Validate`, `TestLoopResult`, `TestIterationResult` |
| `internal/domain/workflow/step_test.go` | `TestStep_Validate_ForEach`, `TestStep_Validate_While` |
| `pkg/interpolation/resolver_test.go` | `TestContext_WithLoop`, `TestLoopData_Fields` |
| `pkg/interpolation/template_resolver_test.go` | `TestResolve_LoopVariables` |
| `internal/application/loop_executor_test.go` | `TestExecuteForEach_*`, `TestExecuteWhile_*`, `TestParseItems` |
| `internal/infrastructure/repository/yaml_mapper_test.go` | `TestMapStep_ForEach`, `TestMapStep_While`, `TestMapLoopConfig` |

### Integration Tests

| File | Test Cases |
|------|------------|
| `tests/integration/loop_test.go` | `TestForEachLoop_Simple`, `TestForEachLoop_WithBreak`, `TestWhileLoop_Simple`, `TestWhileLoop_MaxIterations`, `TestNestedLoops_Rejected` |

### Test Fixtures

Create `tests/fixtures/workflows/loop-foreach.yaml`:
```yaml
name: test-foreach
states:
  initial: process_files
  process_files:
    type: for_each
    items: '["a.txt", "b.txt", "c.txt"]'
    max_iterations: 10
    body:
      - echo_file
    on_complete: done
  echo_file:
    type: step
    command: 'echo "Processing {{loop.item}} ({{loop.index}}/{{loop.length}})"'
    on_success: process_files
  done:
    type: terminal
    status: success
```

Create `tests/fixtures/workflows/loop-while.yaml`:
```yaml
name: test-while
states:
  initial: countdown
  countdown:
    type: while
    while: "states.decrement.exit_code == 0"
    max_iterations: 10
    body:
      - decrement
    on_complete: done
  decrement:
    type: step
    command: 'test {{loop.index}} -lt 5'
    on_success: countdown
    on_failure: countdown
  done:
    type: terminal
    status: success
```

---

## Files to Modify

| File | Action | Complexity | LOC Est. |
|------|--------|------------|----------|
| `internal/domain/workflow/loop.go` | CREATE | M | ~100 |
| `internal/domain/workflow/step.go` | MODIFY | S | +30 |
| `internal/domain/workflow/validation.go` | MODIFY | S | +20 |
| `pkg/interpolation/resolver.go` | MODIFY | S | +20 |
| `pkg/interpolation/template_resolver.go` | MODIFY | S | +5 |
| `internal/infrastructure/repository/yaml_types.go` | MODIFY | S | +10 |
| `internal/infrastructure/repository/yaml_mapper.go` | MODIFY | M | +50 |
| `internal/application/loop_executor.go` | CREATE | L | ~200 |
| `internal/application/execution_service.go` | MODIFY | M | +80 |
| `internal/domain/workflow/loop_test.go` | CREATE | M | ~100 |
| `internal/application/loop_executor_test.go` | CREATE | L | ~200 |
| `tests/integration/loop_test.go` | CREATE | M | ~150 |

**Total estimated LOC:** ~965

---

## Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| **Infinite loops** | High | Mandatory `max_iterations` with default 100, hard limit 10000. Log warning when limit reached. |
| **Nested loops** | Medium | V1: Reject nested loops at validation time. V2: Implement loop stack if needed. |
| **Memory pressure** | Medium | Don't store all iteration outputs by default. Add optional `collect_outputs: true` flag. |
| **State key collisions** | Medium | Use unique keys for iteration states: `{step}[{iteration}]` format. |
| **Break condition errors** | Low | Log warning but don't fail loop if break condition evaluation fails. |
| **Body step not found** | Low | Validate all body step references at parse time. |

---

## Open Questions

1. **Nested loops**: Should we support nested loops in v1? Current plan rejects them.
2. **Output collection**: Should `{{loop.outputs}}` expose all iteration outputs? Memory concern.
3. **Parallel iterations**: Should we support `parallel: true` on for_each for concurrent iteration?
4. **Loop state persistence**: How to persist loop state for resume? Current plan: checkpoint after each iteration.

