You are implementing feature {{.inputs.feature_id}} using strict TDD.

## PHASE: Write Interface/Stub (RED phase - step 1)
## Component: {{.loop.Index1}}/{{.loop.Length}}

## Component Details
```json
$COMPONENT
```

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Create ONLY the interface, types, and stub implementation for THIS component.

The stub should:
- Return zero values or panic("not implemented")
- Allow tests to compile but fail when run
- Follow hexagonal architecture conventions

Files to create/modify (from component.files):
- See the "files" array in Component Details above

DO NOT:
- Touch other components
- Implement actual logic
- Create tests (next step)

Focus ONLY on this component. The code must compile with existing codebase.

## Available Agents
{{.inputs.agents}}

## Available Skills
{{.inputs.skills}}

If agents/skills are specified, use them via Task/Skill tools when appropriate.
