# Task: Update Documentation for Feature $FEATURE_ID

## Prerequisites

**IMPORTANT:** Before starting, use the Skill tool with `common:docs` to load documentation standards.

## Context

You are updating AWF (AI Workflow CLI) documentation following the Diátaxis framework.

**Feature Spec:**
```
$SPEC_CONTENT
```

**Implementation Plan:**
```
$PLAN_CONTENT
```

**Staged/Modified Files:**
```
$STAGED_FILES
```

## Instructions

### Step 1: Read Current State

1. Read `README.md` - understand current structure
2. Read `docs/README.md` - documentation index
3. Search for existing references: `grep -r "$FEATURE_ID" docs/`

### Step 2: Update README.md

**Roadmap Section:**
- [ ] Mark feature complete: `- [ ]` → `- [x]`
- [ ] If not listed: add to appropriate phase section
- [ ] If BREAKING change: add marker next to checkbox

**Commands Table (if adds/modifies CLI commands):**
- [ ] Add/update command row
- [ ] Maintain alphabetical order

**Flags Sections (if adds command flags):**
- [ ] Add flag to relevant section (Global/Run/Resume)
- [ ] Format: `--flag-name, -f` | Description
- [ ] Include default value if applicable

**Workflow Syntax Section (if changes YAML syntax):**
- [ ] Update syntax blocks with new fields
- [ ] Add minimal example showing new syntax

### Step 3: Update docs/ Directory

**Determine Documentation Type (Diátaxis):**

| Type | When to Use | Location |
|------|-------------|----------|
| Tutorial | New user learning | `docs/getting-started/` |
| How-To | Task completion guide | `docs/user-guide/` |
| Reference | Technical specs | `docs/reference/` |
| Explanation | Architecture concepts | `docs/development/` |

**Actions:**
1. Check existing docs in appropriate directory
2. **Update existing file** if content fits current scope
3. **Create new file** only if feature introduces new concept
4. Update `docs/README.md` with new links if needed

**Reference Documentation Style:**
- Use tables for flags, exit codes, validation rules
- Include code examples with syntax highlighting
- Keep examples minimal (< 15 lines)

**User Guide Style:**
- Start with problem statement
- Step-by-step instructions
- Include expected output

### Step 4: Validate

Before completing, verify:
- [ ] All applicable checklist items addressed
- [ ] No broken markdown (code fences closed)
- [ ] All `docs/*` links exist
- [ ] YAML/Bash examples are syntactically valid

## Edge Cases

- **Feature not in roadmap**: Add under appropriate phase
- **Breaking change**: Mark with indicator, note migration
- **Deprecation**: Use ~~strikethrough~~, add deprecation notice
- **Missing spec H1**: Use feature ID as title

## Output

Respond with:
1. **Summary**: One sentence (e.g., "Updated README.md roadmap and added --dry-run to docs/user-guide/commands.md")
2. **Files Modified**: List of absolute paths
3. **Next Steps**: If new docs created, note what needs review
