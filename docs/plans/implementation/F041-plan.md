The plan already exists and is comprehensive. My exploration confirmed:

## Plan Validation Summary

**✓ Architecture alignment verified:**
- `pkg/interpolation/` exists with `resolver.go`, `template_resolver.go`, `errors.go` - good location for `reference.go`
- `internal/domain/workflow/` has `validation_errors.go`, `graph.go`, `workflow.go` - matches plan targets
- Existing `ValidationResult` infrastructure at lines 46-104 is reusable

**✓ Key patterns identified:**
- `FindReachableStates()` in `graph.go:88-119` - BFS pattern for `ComputeExecutionOrder`
- `ValidationResult.AddError()` API exists
- `Context` struct in `resolver.go:10-18` defines valid namespaces
- `StepStateData` fields: `Output`, `Stderr`, `ExitCode`, `Status`

**Minor corrections needed in plan:**
1. YAML fixture should use `type: step` not `type: command` (YAML mapper accepts both but "step" is convention)
2. YAML structure has `states.initial: start` not top-level `initial: start`

**One question**: The plan wires validation into `Workflow.Validate()` at line 90. However, `ValidateGraph()` is a separate function called elsewhere. Should template validation be part of `Workflow.Validate()` (current plan) or a separate step in the CLI validate command? The current approach is simpler but couples domain validation tightly.

Do you want me to:
1. Start implementing as-is (recommended - plan is solid)
2. Apply the minor fixture corrections first
3. Discuss the validation wiring question before proceeding

