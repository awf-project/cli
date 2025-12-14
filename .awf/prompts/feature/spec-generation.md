# Task: Generate Feature Specification for {{.inputs.feature_id}}

## Prerequisites

**IMPORTANT:** Before starting, read the project constitution at `.specify/memory/constitution.md` to understand governing principles.

## Feature Info

- **ID**: {{.inputs.feature_id}}
- **Version**: {{.inputs.version}}
- **Description**: {{.inputs.description}}

## Template

$(cat .specify/templates/feature.md)

## Instructions

### Step 1: Understand the Feature

1. Read the description carefully
2. Search existing features via GitHub: `gh issue list --search "F0"`
3. Check constitution compliance: `.specify/memory/constitution.md`
4. Identify dependencies on other features (if any)

### Step 2: Generate User Stories (P1/P2/P3)

**Priority levels:**
- **P1 (Must Have)**: Core functionality, feature is useless without it
- **P2 (Should Have)**: Important but can be deferred if needed
- **P3 (Nice to Have)**: Enhancements, edge cases, polish

**For each user story:**
1. Write in "As a... I want... So that..." format
2. Add 2-3 acceptance scenarios (Given/When/Then)
3. Include an independent test description

**Rules:**
- Minimum 1 P1 story, maximum 5 total stories
- Each story must be independently testable
- No implementation details (languages, frameworks)

### Step 3: Define Requirements

**Functional Requirements (FR):**
- Derived from user stories
- Testable and specific
- Examples: "CLI returns exit code 0 on success", "Invalid input shows error message"

**Non-Functional Requirements (NFR):**
- Performance: "Workflow parsing < 100ms"
- Security: "No secrets logged in plaintext"
- Reliability: "Graceful timeout handling"

### Step 4: Identify Key Entities

If the feature involves data:
- List entities with their key attributes
- Keep technology-agnostic (no "JSON field" or "Go struct")

### Step 5: Fill Metadata

| Field | Value |
|-------|-------|
| Status | `backlog` |
| Version | {{.inputs.version}} |
| Priority | Infer from description (high/medium/low) |
| Estimation | Infer from complexity (S/M/L/XL) |

### Step 6: Generate Slug

Create kebab-case slug (2-4 words):
- Examples: `cli-init`, `dry-run-mode`, `loop-context-vars`
- Derived from feature title

## Validation Checklist

Before output, verify:
- [ ] At least 1 P1 user story with acceptance scenarios
- [ ] All requirements are testable
- [ ] No implementation details in user stories
- [ ] Metadata is complete
- [ ] Slug is kebab-case, 2-4 words

## Output Format

Output a JSON object with exactly this structure:

```json
{"slug": "feature-name-slug", "content": "# F0XX: Title\n\n..."}
```

- `slug`: lowercase, kebab-case, 2-4 words
- `content`: full markdown spec following Spec-Kit template

**Output ONLY the JSON, no explanations.**
