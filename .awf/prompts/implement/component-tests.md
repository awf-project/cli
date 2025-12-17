You are implementing feature $FEATURE_ID using strict TDD.

## PHASE: Write Tests (RED phase - step 2)
## Component: $LOOP_INDEX/$LOOP_LENGTH

## Component Details
```json
$COMPONENT
```

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Write comprehensive tests for THIS component only.

Test files to create (from component.tests):
- See the "tests" array in Component Details above

Requirements:
- Use the stub/interface created in the previous step
- Tests MUST compile (use existing interfaces)
- Tests SHOULD fail when run (stubs return zero/panic)
- Follow table-driven test pattern
- Use testify/require or testify/assert

Test coverage should include:
- Happy path scenarios
- Edge cases and boundary conditions
- Error handling

DO NOT:
- Touch other components
- Implement actual logic
- Modify the stub (tests should use it as-is)

Focus ONLY on this component's test files.

## Available Agents
$AGENTS

## Available Skills
$SKILLS

If agents/skills are specified, use them via Task/Skill tools when appropriate.
