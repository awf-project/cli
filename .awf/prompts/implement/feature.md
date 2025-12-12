You are implementing feature {{.inputs.feature_id}} using strict TDD.

## PHASE: Implementation (GREEN phase) - Attempt {{.loop.Index1}}/3

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Implement the actual logic to make all tests pass:
- Replace stub implementations with real code
- Follow the implementation steps from the plan
- Ensure code compiles and passes lint
- Make tests pass (GREEN)
{{if not .loop.First}}
## IMPORTANT: Previous attempt failed
This is retry attempt {{.loop.Index1}}. The previous implementation did not pass all tests.
Review the test output and fix the issues before proceeding.
{{end}}

## Available Agents
{{.inputs.agents}}

## Available Skills
{{.inputs.skills}}

If agents/skills are specified, use them via Task/Skill tools when appropriate.
For example, use go:golang-expert for complex Go patterns.
