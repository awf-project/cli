# Task: Create Implementation Plan for {{.inputs.feature_id}}

## Prerequisites

**IMPORTANT:** Before starting, read the project constitution at `.specify/memory/constitution.md` to understand governing principles.

## Inputs

Based on your exploration, create a detailed implementation plan.

## Output Files

Generate the following files:

### 1. Implementation Plan (plan.md)

```markdown
# Implementation Plan: {{.inputs.feature_id}}

## Summary
(2-3 sentences describing the approach)

## Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| Hexagonal Architecture | COMPLIANT | [how] |
| Go Idioms | COMPLIANT | [how] |
| TDD Required | COMPLIANT | [how] |
| ... | ... | ... |

## Technical Context
- Language: Go 1.21+
- Framework: Cobra CLI
- Architecture: Hexagonal
- Key patterns: [identified patterns]

## Architecture Decisions

### ADR-001: [Decision Title]
- **Context**: [Why this decision is needed]
- **Decision**: [What we decided]
- **Rationale**: [Why this option]
- **Alternatives**: [Other options considered]

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
    "dependencies": [],
    "user_story": "US1"
  }
]
```

Rules:
- Order by dependencies: domain/ports → domain → application → infrastructure → interfaces
- Each component MUST be independently testable
- Group related files (e.g., entity + its methods in same component)
- Max 5-7 components per feature
- Use snake_case for component names
- Link components to user stories (US1, US2, etc.)

## Implementation Steps
1. Step description
   - File: path/to/file.go
   - Action: CREATE/MODIFY
   - Changes: specific changes needed

## Test Plan
- Unit tests for each component
- Integration tests (if needed)
- Feature tests tagged with {{.inputs.feature_id}}

## Risks
- Risk and mitigation
```

### 2. Research Document (research.md) - Optional

If technical decisions required research:

```markdown
# Research: {{.inputs.feature_id}}

## Questions Investigated

### Q1: [Technical question]
- **Finding**: [What was discovered]
- **Sources**: [Docs, code examples, patterns]
- **Recommendation**: [What to do]

## Best Practices Applied
- [Pattern 1]: [Why applicable]
- [Pattern 2]: [Why applicable]

## Dependencies Identified
- [Package]: [Purpose]
```

### 3. Data Model (data-model.md) - If entities involved

```markdown
# Data Model: {{.inputs.feature_id}}

## Entities

### [EntityName]

| Field | Type | Description | Constraints |
|-------|------|-------------|-------------|
| ID | string | Unique identifier | Required, UUID |
| ... | ... | ... | ... |

### Relationships
- [Entity1] has many [Entity2]
- [Entity2] belongs to [Entity1]

### Lifecycle States
- Created → Active → Completed
```

## Validation Checklist

Before output, verify:
- [ ] All components linked to user stories
- [ ] Components ordered by dependencies
- [ ] Constitution compliance checked
- [ ] At least one ADR documented
- [ ] Test plan includes unit + integration

## Output Format

Output a JSON object:

```json
{
  "plan": "# Implementation Plan...",
  "research": "# Research... (or null if not needed)",
  "data_model": "# Data Model... (or null if no entities)"
}
```

**Be specific and actionable. Reference constitution principles.**
