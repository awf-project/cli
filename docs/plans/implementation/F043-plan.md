# Implementation Plan: F043 - Nested Loop Execution

## Summary

Implement nested loop support by adding a `Parent` reference to `LoopContext` and `LoopData` structures, enabling a linked-list stack pattern. The `LoopExecutor` will push/pop context on loop entry/exit, automatically preserving outer loop state. Template access via `{{.loop.parent.*}}` chains will be natively supported through recursive structure traversal.

## ASCII Wireframe: Data Flow

```
NESTED LOOP EXECUTION FLOW:
┌─────────────────────────────────────────────────────────────┐
│  outer_loop (for_each: ["A", "B"])                         │
│  ├── CurrentLoop: {Item:"A", Index:0, Parent:nil}          │
│  │                                                          │
│  └── inner_loop (for_each: ["1", "2"])                     │
│      ├── PUSH → CurrentLoop: {Item:"1", Parent:→outer}     │
│      │   └── {{.loop.Item}} = "1"                          │
│      │   └── {{.loop.parent.Item}} = "A"                   │
│      │                                                      │
│      └── POP → CurrentLoop: restored to outer              │
│          └── {{.loop.Item}} = "A" (restored)               │
└─────────────────────────────────────────────────────────────┘

STATE PERSISTENCE (JSON):
{
  "CurrentLoop": {
    "Item": "1",
    "Index": 0,
    "Parent": {           ← Recursive JSON serialization
      "Item": "A",
      "Index": 0,
      "Parent": null
    }
  }
}
```

## Implementation Steps

### Step 1: Add Parent Reference to Domain LoopContext
- **File**: `internal/domain/workflow/context.go`
- **Action**: MODIFY
- **Changes**:
  ```go
  // Line 34-40: Add Parent field to LoopContext
  type LoopContext struct {
      Item   any
      Index  int
      First  bool
      Last   bool
      Length int
      Parent *LoopContext // NEW: reference to enclosing loop
  }
  ```

### Step 2: Add Parent Reference to Interpolation LoopData
- **File**: `pkg/interpolation/resolver.go`
- **Action**: MODIFY
- **Changes**:
  ```go
  // Line 22-28: Add Parent field to LoopData
  type LoopData struct {
      Item   any
      Index  int
      First  bool
      Last   bool
      Length int
      Parent *LoopData // NEW: for {{.loop.parent.*}} access
  }
  
  // Add helper method for parent chain access
  func (l *LoopData) GetParent() *LoopData {
      return l.Parent
  }
  ```

### Step 3: Implement Push/Pop in LoopExecutor
- **File**: `internal/application/loop_executor.go`
- **Action**: MODIFY
- **Changes**:
  - Add helper methods after line 39:
    ```go
    // pushLoopContext sets a new loop context, linking to existing as parent.
    func (e *LoopExecutor) pushLoopContext(
        execCtx *workflow.ExecutionContext,
        item any,
        index int,
        first, last bool,
        length int,
    ) {
        newCtx := &workflow.LoopContext{
            Item:   item,
            Index:  index,
            First:  first,
            Last:   last,
            Length: length,
            Parent: execCtx.CurrentLoop, // Link to outer loop
        }
        execCtx.CurrentLoop = newCtx
    }
    
    // popLoopContext restores the parent loop context.
    func (e *LoopExecutor) popLoopContext(execCtx *workflow.ExecutionContext) {
        if execCtx.CurrentLoop != nil {
            execCtx.CurrentLoop = execCtx.CurrentLoop.Parent
        }
    }
    ```
  
  - Modify `ExecuteForEach` (lines 86-92): Replace direct assignment with push:
    ```go
    // OLD: execCtx.CurrentLoop = &workflow.LoopContext{...}
    // NEW:
    e.pushLoopContext(execCtx, item, i, i == 0, i == len(items)-1, len(items))
    ```
  
  - Modify `ExecuteForEach` (lines 141-142): Replace clear with pop:
    ```go
    // OLD: execCtx.CurrentLoop = nil
    // NEW:
    e.popLoopContext(execCtx)
    ```
  
  - Apply same changes to `ExecuteWhile` (lines 165-170 and 227-228)

### Step 4: Update Context Builder for Parent Chain
- **File**: `internal/application/execution_service.go`
- **Action**: MODIFY
- **Changes**:
  - Add helper function before `buildInterpolationContext`:
    ```go
    // buildLoopDataChain recursively converts domain LoopContext to interpolation LoopData.
    func buildLoopDataChain(domainLoop *workflow.LoopContext) *interpolation.LoopData {
        if domainLoop == nil {
            return nil
        }
        return &interpolation.LoopData{
            Item:   domainLoop.Item,
            Index:  domainLoop.Index,
            First:  domainLoop.First,
            Last:   domainLoop.Last,
            Length: domainLoop.Length,
            Parent: buildLoopDataChain(domainLoop.Parent), // Recursive
        }
    }
    ```
  
  - Modify `buildInterpolationContext` (lines 529-537): Replace manual construction:
    ```go
    // OLD:
    // if execCtx.CurrentLoop != nil {
    //     intCtx.Loop = &interpolation.LoopData{...}
    // }
    
    // NEW:
    intCtx.Loop = buildLoopDataChain(execCtx.CurrentLoop)
    ```

### Step 5: Add Unit Tests for Push/Pop Operations
- **File**: `internal/application/loop_executor_test.go`
- **Action**: MODIFY (or CREATE if doesn't exist)
- **Changes**:
  ```go
  func TestLoopExecutor_PushPopContext(t *testing.T) {
      exec := NewLoopExecutor(nil, nil, nil)
      execCtx := workflow.NewExecutionContext("test-id", "test-wf")
      
      // Push outer loop
      exec.pushLoopContext(execCtx, "outer", 0, true, true, 1)
      assert.Equal(t, "outer", execCtx.CurrentLoop.Item)
      assert.Nil(t, execCtx.CurrentLoop.Parent)
      
      // Push inner loop
      exec.pushLoopContext(execCtx, "inner", 0, true, true, 1)
      assert.Equal(t, "inner", execCtx.CurrentLoop.Item)
      assert.NotNil(t, execCtx.CurrentLoop.Parent)
      assert.Equal(t, "outer", execCtx.CurrentLoop.Parent.Item)
      
      // Pop inner
      exec.popLoopContext(execCtx)
      assert.Equal(t, "outer", execCtx.CurrentLoop.Item)
      
      // Pop outer
      exec.popLoopContext(execCtx)
      assert.Nil(t, execCtx.CurrentLoop)
  }
  
  func TestLoopExecutor_TripleNesting(t *testing.T) {
      exec := NewLoopExecutor(nil, nil, nil)
      execCtx := workflow.NewExecutionContext("test-id", "test-wf")
      
      exec.pushLoopContext(execCtx, "L1", 0, true, true, 1)
      exec.pushLoopContext(execCtx, "L2", 0, true, true, 1)
      exec.pushLoopContext(execCtx, "L3", 0, true, true, 1)
      
      // Verify chain
      assert.Equal(t, "L3", execCtx.CurrentLoop.Item)
      assert.Equal(t, "L2", execCtx.CurrentLoop.Parent.Item)
      assert.Equal(t, "L1", execCtx.CurrentLoop.Parent.Parent.Item)
      assert.Nil(t, execCtx.CurrentLoop.Parent.Parent.Parent)
  }
  ```

### Step 6: Add Unit Tests for Context Building
- **File**: `internal/application/execution_service_test.go`
- **Action**: MODIFY
- **Changes**:
  ```go
  func TestBuildLoopDataChain(t *testing.T) {
      // Test nil
      assert.Nil(t, buildLoopDataChain(nil))
      
      // Test single level
      single := &workflow.LoopContext{Item: "A", Index: 0}
      data := buildLoopDataChain(single)
      assert.Equal(t, "A", data.Item)
      assert.Nil(t, data.Parent)
      
      // Test nested
      outer := &workflow.LoopContext{Item: "outer", Index: 0}
      inner := &workflow.LoopContext{Item: "inner", Index: 1, Parent: outer}
      data = buildLoopDataChain(inner)
      assert.Equal(t, "inner", data.Item)
      assert.Equal(t, "outer", data.Parent.Item)
  }
  ```

### Step 7: Enable Pending Integration Tests
- **File**: `tests/integration/loop_test.go`
- **Action**: MODIFY
- **Changes**:
  - Line 1610: Remove `t.Skip("PENDING: nested loops not yet implemented...")`
  - Line 1694: Remove `t.Skip("PENDING: nested loops not yet implemented...")`
  - Line 1783: Remove `t.Skip("PENDING: nested loops not yet implemented...")`
  - Line 1876: Remove `t.Skip("PENDING: nested loops not yet implemented...")`

### Step 8: Update Fixture for Parent Access Test
- **File**: `tests/fixtures/workflows/loop-nested.yaml`
- **Action**: MODIFY
- **Changes**: Add parent access test case:
  ```yaml
  # Line 34: Update command to test parent access
  command: 'echo "OUTER: {{.loop.Item}} ({{.loop.Index1}}/{{.loop.Length}})"'
  
  # Line 48: Update inner command to demonstrate parent access
  command: 'echo "  INNER: {{.loop.Item}} ({{.loop.Index1}}/{{.loop.Length}}) parent={{.loop.Parent.Item}}"'
  ```

### Step 9: Add New Integration Test for Parent Access
- **File**: `tests/integration/loop_test.go`
- **Action**: MODIFY
- **Changes**: Add after existing nested tests:
  ```go
  func TestF043_NestedLoops_ParentAccess_Integration(t *testing.T) {
      if testing.Short() {
          t.Skip("skipping integration test")
      }
      
      tmpDir := t.TempDir()
      logFile := filepath.Join(tmpDir, "parent_access.log")
      
      wfYAML := `name: nested-parent-access
  version: "1.0.0"
  states:
    initial: outer
    outer:
      type: for_each
      items: '["A", "B"]'
      max_iterations: 10
      body:
        - inner
      on_complete: done
    inner:
      type: for_each
      items: '["1", "2"]'
      max_iterations: 10
      body:
        - log_both
      on_complete: outer
    log_both:
      type: step
      command: 'echo "outer={{.loop.Parent.Item}} inner={{.loop.Item}}" >> ` + logFile + `'
      on_success: inner
    done:
      type: terminal
      status: success
  `
      // ... setup and execution ...
      
      expected := `outer=A inner=1
  outer=A inner=2
  outer=B inner=1
  outer=B inner=2
  `
      assert.Equal(t, expected, string(data))
  }
  ```

## Test Plan

### Unit Tests
| Test | Location | Purpose |
|------|----------|---------|
| `TestLoopExecutor_PushPopContext` | `loop_executor_test.go` | Verify push/pop mechanics |
| `TestLoopExecutor_TripleNesting` | `loop_executor_test.go` | Verify deep nesting |
| `TestBuildLoopDataChain` | `execution_service_test.go` | Verify context building |
| `TestLoopContext_ParentSerialization` | `context_test.go` | Verify JSON marshal/unmarshal |

### Integration Tests (Enable Pending)
| Test | Line | Purpose |
|------|------|---------|
| `TestF042_NestedForEachLoops_Integration` | 1609 | Basic nested for_each |
| `TestF042_NestedLoops_ContextRestored_Integration` | 1693 | Context preservation |
| `TestF042_NestedWhileInForEach_Integration` | 1782 | Mixed loop types |
| `TestF042_TripleNestedLoops_Integration` | 1875 | 3-level nesting |

### New Integration Tests
| Test | Purpose |
|------|---------|
| `TestF043_NestedLoops_ParentAccess_Integration` | Verify `{{.loop.Parent.*}}` works |
| `TestF043_NestedLoops_DeepParentChain_Integration` | Verify `{{.loop.Parent.Parent.*}}` |
| `TestF043_NestedLoops_Resume_Integration` | Verify state persistence with nesting |

## Files to Modify

| File | Action | Complexity | Lines Changed |
|------|--------|------------|---------------|
| `internal/domain/workflow/context.go` | MODIFY | S | +1 field |
| `pkg/interpolation/resolver.go` | MODIFY | S | +2 (field + method) |
| `internal/application/loop_executor.go` | MODIFY | M | +30 (helpers + refactor) |
| `internal/application/execution_service.go` | MODIFY | S | +15 (helper + refactor) |
| `internal/application/loop_executor_test.go` | MODIFY | M | +50 (new tests) |
| `internal/application/execution_service_test.go` | MODIFY | S | +20 (new tests) |
| `tests/integration/loop_test.go` | MODIFY | M | +60 (enable + new tests) |
| `tests/fixtures/workflows/loop-nested.yaml` | MODIFY | S | +2 lines |

**Total Estimated Changes**: ~180 lines

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Deep nesting memory | Stack overflow for extreme depth | Document max 10 levels; add runtime check |
| JSON serialization cycles | Panic on marshal | Parent is pointer, JSON handles naturally |
| Template resolution `nil.Parent` | Panic on access | Go templates return `<no value>` for nil safely |
| Resume from nested position | Complex state restoration | Parent chain serializes naturally via JSON |
| Integration test evaluator | May not handle all expressions | Extend `loopContextEvaluator` for parent access |

## Verification Checklist

- [ ] `make test-unit` passes
- [ ] `make test-integration` passes (with enabled nested tests)
- [ ] `make lint` passes
- [ ] `loop-nested.yaml` fixture executes correctly
- [ ] Parent chain serializes/deserializes correctly (manual verification)
- [ ] 3-level nesting test passes
- [ ] Resume from nested loop position works

