# Task: Generate Feature Specification for {{.inputs.feature_id}}

## Prerequisites

**IMPORTANT:** Before starting, use the Skill tool with `common:docs` to load documentation standards.

## Feature Info

- **ID**: {{.inputs.feature_id}}
- **Version**: {{.inputs.version}}
- **Description**: {{.inputs.description}}

## Template

$(cat docs/plans/features/TEMPLATE.md)

## Naming Convention

Files: `F0XX-feature-name-in-kebab-case.md`
Examples: `F036-cli-init.md`, `F007-variable-interpolation.md`, `F029-output-streaming.md`

## Instructions

### Step 1: Understand the Feature

1. Read the description carefully
2. Search existing features for related functionality: `grep -r "related-term" docs/plans/features/`
3. Identify dependencies on other features (if any)

### Step 2: Fill the Template

| Section | Guidelines |
|---------|------------|
| **Title** | `# F0XX: Short Title` (3-6 words, action-oriented) |
| **Status** | Set to `planned` |
| **Version** | Set to {{.inputs.version}} |
| **Priority** | `high`, `medium`, or `low` based on user impact |
| **Summary** | 2-3 sentences explaining WHAT and WHY |
| **Acceptance Criteria** | 3-7 testable criteria using checkboxes |
| **Technical Tasks** | Leave as `[ ] TBD - to be filled after codebase exploration` |
| **Impacted Files** | Leave as `TBD - to be identified during implementation planning` |

### Step 3: Write Quality Acceptance Criteria

**Good criteria:**
- [ ] `awf run --flag` displays expected output
- [ ] Invalid input shows error message with exit code 1
- [ ] Configuration persists across sessions

**Bad criteria:**
- [ ] Feature works correctly (too vague)
- [ ] Code is clean (not testable)
- [ ] User is happy (subjective)

**Rules:**
- Each criterion must be independently testable
- Use specific values/behaviors, not vague terms
- Include both happy path and error cases
- Reference CLI commands, flags, or outputs when applicable

### Step 4: Generate Slug

Create a kebab-case slug (2-4 words):
- `cli-init` for CLI initialization feature
- `dry-run-mode` for dry-run execution
- `loop-context-vars` for loop context variables

## Validation

Before output, verify:
- [ ] Title is concise (3-6 words)
- [ ] Summary explains WHAT and WHY
- [ ] Acceptance criteria are testable (3-7 items)
- [ ] Status is `planned`
- [ ] Version matches {{.inputs.version}}
- [ ] Slug is kebab-case, 2-4 words

## Output Format

Output a JSON object with exactly this structure:

```json
{"slug": "feature-name-slug", "content": "# F0XX: Title\n\n..."}
```

- `slug`: lowercase, kebab-case, 2-4 words
- `content`: full markdown spec following template

**Output ONLY the JSON, no explanations.**
