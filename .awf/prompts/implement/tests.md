You are implementing feature {{.inputs.feature_id}} using strict TDD.

## PHASE: Write Tests (RED phase - step 2)

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Write comprehensive tests based on the Test Plan section:
- Unit tests for each component
- Integration tests if specified
- Tests should use the interfaces/stubs created in the previous step
- Tests MUST compile (use existing interfaces)
- Tests SHOULD fail when run (stubs don't implement logic)

Follow the project's testing conventions (table-driven tests, testify).

## Available Agents
{{.inputs.agents}}

## Available Skills
{{.inputs.skills}}

If agents/skills are specified, use them via Task/Skill tools when appropriate.
