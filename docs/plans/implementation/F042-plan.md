# Implementation Plan: F042 - Loop Context Variables

## Summary

F042 is 90% already implemented from F016. The only missing piece is `{{loop.index1}}` (1-based index). Implementation requires adding a single method to `LoopData` struct and corresponding tests. This is an XS change with zero risk to existing functionality.

## ASCII Art: Change Scope

```
┌──────────────────────────────────────────────────────────────────┐
│                    F042 IMPLEMENTATION SCOPE                     │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  pkg/interpolation/resolver.go                                   │
│  ┌────────────────────────────────────────┐                      │
│  │ type LoopData struct {                 │                      │
│  │     Item   any                         │                      │
│  │     Index  int    ◄── existing         │                      │
│  │     First  bool                        │                      │
│  │     Last   bool                        │                      │
│  │     Length int                         │                      │
│  │ }                                      │                      │
│  │                                        │                      │
│  │ + func (l *LoopData) Index1() int {   │ ◄── ADD THIS          │
│  │ +     return l.Index + 1              │                       │
│  │ + }                                   │                       │
│  └────────────────────────────────────────┘                      │
│                                                                  │
│  Result: {{.loop.Index1}} → 1, 2, 3, ...                         │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Add Index1 Method to LoopData
- **File**: `pkg/interpolation/resolver.go`
- **Action**: MODIFY
- **Changes**: Add `Index1()` method after line 28 (after LoopData struct definition)

```go
// Index1 returns the 1-based iteration index.
func (l *LoopData) Index1() int {
    return l.Index + 1
}
```

### Step 2: Add Unit Tests for Index1
- **File**: `pkg/interpolation/resolver_test.go`
- **Action**: MODIFY
- **Changes**: Add test case for `{{.loop.Index1}}` template resolution

```go
func TestTemplateResolver_LoopIndex1(t *testing.T) {
    resolver := NewTemplateResolver()
    ctx := NewContext()
    ctx.Loop = &LoopData{
        Item:   "test",
        Index:  2,
        First:  false,
        Last:   false,
        Length: 5,
    }

    result, err := resolver.Resolve("Item {{.loop.Index1}} of {{.loop.Length}}", ctx)
    
    require.NoError(t, err)
    assert.Equal(t, "Item 3 of 5", result)
}
```

### Step 3: Add Integration Test for Index1
- **File**: `tests/integration/loop_test.go`
- **Action**: MODIFY
- **Changes**: Add test `TestForEachLoop_WithIndex1_Integration`

```go
func TestForEachLoop_WithIndex1_Integration(t *testing.T) {
    // Test that {{.loop.Index1}} produces 1-based output: "1: a", "2: b", "3: c"
}
```

### Step 4: Update Feature Spec
- **File**: `docs/plans/features/v0.3.0/F042-loop-context-variables.md`
- **Action**: MODIFY
- **Changes**: Mark completed acceptance criteria with [x]

## Test Plan

### Unit Tests
| Test | File | Description |
|------|------|-------------|
| `TestTemplateResolver_LoopIndex1` | `pkg/interpolation/resolver_test.go` | Verify `{{.loop.Index1}}` returns Index+1 |
| `TestLoopData_Index1_EdgeCases` | `pkg/interpolation/resolver_test.go` | Test Index=0 → Index1=1, Index=99 → Index1=100 |

### Integration Tests
| Test | File | Description |
|------|------|-------------|
| `TestForEachLoop_WithIndex1_Integration` | `tests/integration/loop_test.go` | E2E test with real shell execution verifying output |

### Manual Verification
```bash
# Create test workflow
cat > /tmp/test-index1.yaml << 'EOF'
name: test-index1
version: "1.0.0"
states:
  initial: loop
  loop:
    type: for_each
    items: '["a", "b", "c"]'
    max_iterations: 10
    body:
      - echo_item
    on_complete: done
  echo_item:
    type: step
    command: 'echo "{{.loop.Index1}}: {{.loop.Item}}"'
    on_success: loop
  done:
    type: terminal
    status: success
EOF

# Run and verify output shows 1: a, 2: b, 3: c
awf run test-index1
```

## Files to Modify

| File | Action | Complexity | LOC |
|------|--------|------------|-----|
| `pkg/interpolation/resolver.go` | MODIFY | XS | +5 |
| `pkg/interpolation/resolver_test.go` | MODIFY | S | +25 |
| `tests/integration/loop_test.go` | MODIFY | S | +50 |
| `docs/plans/features/v0.3.0/F042-loop-context-variables.md` | MODIFY | XS | +10 |

**Total**: ~90 lines of code

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Template method not called | None | Go templates auto-call zero-arg methods returning single value |
| Breaks existing `Index` usage | None | Adding new method doesn't affect existing field access |
| Nil pointer if Loop is nil | None | Template returns error before calling method (existing behavior) |

## Out of Scope (Deferred)

Per F042 notes, these are explicitly deferred:
- `loop.parent` for nested loops (complex, low demand)
- Graceful empty value when accessing loop vars outside loop (would require template function)

## Acceptance Criteria Checklist

After implementation:
- [x] `{{loop.index}}` returns 0-based *(already done)*
- [ ] `{{loop.index1}}` returns 1-based *(this PR)*
- [x] `{{loop.item}}` returns current value *(already done)*
- [x] `{{loop.first}}` returns true first iteration *(already done)*
- [x] `{{loop.last}}` returns true last iteration *(already done)*
- [x] `{{loop.length}}` returns total count *(already done)*
- [x] Works in step command templates *(already done)*
- [x] Works in `when` conditions *(already done)*
- [x] Nested loops have separate contexts *(already done)*
- [x] Loop context cleared after completion *(already done)*

## Commands

```bash
# Run unit tests
make test-unit

# Run integration tests
make test-integration

# Run specific tests
go test -v ./pkg/interpolation/... -run TestLoopData
go test -v -tags=integration ./tests/integration/... -run TestForEachLoop_WithIndex1
```

