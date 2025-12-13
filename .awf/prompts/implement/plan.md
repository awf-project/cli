Based on your exploration, create a detailed implementation plan for {{.inputs.feature_id}}.

## Output Format (Markdown)
# Implementation Plan: {{.inputs.feature_id}}

## Summary
(2-3 sentences describing the approach)

## Components

CRITICAL: Output a JSON array of components in topological order (dependencies first).
Each component will be implemented separately following TDD (stub → tests → implement).

```json
[
  {
    "name": "component_name",
    "layer": "domain|domain/ports|application|infrastructure|interfaces/cli",
    "description": "What this component does",
    "files": ["internal/path/to/file.go"],
    "tests": ["internal/path/to/file_test.go"],
    "dependencies": []
  }
]
```

Rules:
- Order by dependencies: domain/ports → domain → application → infrastructure → interfaces
- Each component MUST be independently testable
- Group related files (e.g., entity + its methods in same component)
- Max 5-7 components per feature
- Use snake_case for component names
- List only NEW files to create (not existing files to modify unless significant)

## Implementation Steps
1. Step description
   - File: path/to/file.go
   - Action: CREATE/MODIFY
   - Changes: specific changes needed

## Test Plan
- Unit tests for each component
- Integration tests (if needed)

## Risks
- Risk and mitigation

Be specific and actionable.
