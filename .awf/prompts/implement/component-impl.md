You are implementing feature {{.inputs.feature_id}} using strict TDD.

## PHASE: Implementation (GREEN phase) - Attempt {{.loop.Index1}}/3
## Component: {{.loop.parent.Index1}}/{{.loop.parent.Length}}

## Component Details
```json
$COMPONENT
```

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Implement the actual logic to make THIS component's tests pass.

Files to implement (from component.files):
- See the "files" array in Component Details above

Requirements:
- Replace stub implementations with real code
- Follow the implementation steps from the plan
- Ensure code compiles and passes lint
- Make this component's tests pass (GREEN)

{{if not .loop.First}}
## IMPORTANT: Previous attempt failed
This is retry attempt {{.loop.Index1}}. The previous implementation did not pass all tests.
Review the test output and fix the issues before proceeding.
{{end}}

DO NOT:
- Touch other components
- Modify tests (they should pass with your implementation)
- Break existing functionality

Focus ONLY on this component's implementation files.

## Available Agents
{{.inputs.agents}}

## Available Skills
{{.inputs.skills}}

If agents/skills are specified, use them via Task/Skill tools when appropriate.
For example, use go:golang-expert for complex Go patterns.
