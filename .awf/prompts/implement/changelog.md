# Task: Generate CHANGELOG Entry for Feature $FEATURE_ID

## Prerequisites

**IMPORTANT:** Before starting, use the Skill tool with `common:docs` to load documentation standards (Keep a Changelog format).

## Context

You are creating a changelog entry following [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) for AWF (AI Workflow CLI).

**Feature Spec:**
```
$SPEC_CONTENT
```

**Implementation Plan:**
```
$PLAN_CONTENT
```

## Instructions

### Step 1: Read Current Changelog

Read `CHANGELOG.md` to understand:
- Existing entry style (bullet format, backtick usage)
- Current version section (should be "Unreleased")
- Grouping pattern (subcategories like "#### Execution Modes")

### Step 2: Determine Category

| Category | Use When | Semver |
|----------|----------|--------|
| **Added** | New feature | MINOR |
| **Changed** | Modification to existing | MINOR/PATCH |
| **Deprecated** | Marked for removal | MINOR |
| **Removed** | Feature deleted | MAJOR |
| **Fixed** | Bug fix | PATCH |
| **Security** | Vulnerability fix | PATCH |

**Rules:**
- Implementation plan shows `CREATE` new feature → **Added**
- Implementation plan shows `MODIFY` existing → **Changed**
- Spec mentions "BREAKING" or removes functionality → **Changed** with BREAKING note or **Removed**
- Title contains "fix", "bug", "issue" → **Fixed**

### Step 3: Extract Title

1. Look for H1 (`# F0XX: Title`) in spec
2. If no H1, derive from filename: `F038-prompt-storage.md` → "Prompt Storage"
3. Keep concise: 3-8 words
4. Format: `**$FEATURE_ID**: Title`

### Step 4: Write User-Facing Bullets

Create 3-5 bullets focusing on **what users can do**, not implementation.

**Good bullets:**
- `awf run --dry-run` shows execution plan without running commands
- `--breakpoint` flag pauses at specific steps in interactive mode
- Loop context variables: `{{.loop.Index1}}`, `{{.loop.First}}`, `{{.loop.Last}}`

**Bad bullets:**
- Implemented DryRunExecutor in application layer
- Added tests for JSONStore race condition
- Refactored state machine logic

**Formatting:**
- Use backticks for: commands, flags, config keys, variables
- Start with action verb when possible: "Execute", "Display", "Support"
- Include defaults if relevant: `--limit` (default: 20)

### Step 5: Check for Breaking Changes

Add `**BREAKING**:` prefix if ANY of these:
- Removes existing functionality
- Changes existing behavior incompatibly
- Requires user migration
- Renames CLI flags

**Example:**
```markdown
- **BREAKING**: `--output` flag renamed to `--format`
```

### Step 6: Determine Subcategory (optional)

Check if feature belongs to existing subcategory in CHANGELOG:
- Execution Modes
- Loop Constructs
- Workflow Features
- State Machine & Execution
- CLI & Usability

If yes, place under that subcategory. If no subcategory fits, add directly under main category.

## Validation

Before output, verify:
- [ ] Title is concise (3-8 words)
- [ ] All bullets are user-facing
- [ ] Backticks used for code terms
- [ ] BREAKING prefix if applicable
- [ ] 3-5 bullets (not more)
- [ ] Matches existing CHANGELOG style

## Output

Provide:

1. **Category**: Keep a Changelog category (Added/Changed/Fixed/etc.)
2. **Subcategory**: If applicable (e.g., "Execution Modes"), else "None"
3. **Entry**: The markdown entry ready to insert:

```markdown
- **$FEATURE_ID**: [Title]
  - [Bullet 1]
  - [Bullet 2]
  - [Bullet 3]
```

4. **Notes**: Any context (e.g., "Breaking change - update migration guide")
