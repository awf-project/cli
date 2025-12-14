# Task: Clarify Feature Specification

## Context

You are reviewing a feature specification to identify and resolve ambiguities before implementation planning.

## Specification to Review

$SPEC_CONTENT

## Ambiguity Taxonomy

Scan the specification for ambiguities in these categories:

| Category | Examples |
|----------|----------|
| **Functional Scope** | Unclear success criteria, missing edge cases, undefined boundaries |
| **Domain & Data** | Undefined entities, unclear relationships, missing lifecycle states |
| **Interaction & UX** | Unclear user journeys, missing error states, undefined feedback |
| **Non-Functional** | Vague performance targets, undefined scalability needs |
| **Integration** | Unclear external dependencies, undefined protocols |
| **Edge Cases** | Missing negative scenarios, undefined conflict resolution |
| **Terminology** | Ambiguous terms, inconsistent naming |

## Process

### Step 1: Identify Ambiguities

Scan the specification and list potential ambiguities. For each:
- Identify the category
- Explain why it's ambiguous
- Assess impact (high/medium/low)

### Step 2: Generate Questions

Create **maximum 5 questions** to resolve the highest-impact ambiguities.

**Question format:**
- Multiple choice (2-4 mutually exclusive options), OR
- Short phrase answer (5 words or less)

### Step 3: Ask User

Use the `AskUserQuestion` tool to present the questions to the user.

**Example question structure:**
```json
{
  "question": "How should the system handle invalid workflow YAML?",
  "header": "Error handling",
  "options": [
    {"label": "Fail fast with error", "description": "Stop immediately and show parse error"},
    {"label": "Warn and skip", "description": "Log warning and continue with valid parts"},
    {"label": "Interactive fix", "description": "Prompt user to correct the error"}
  ],
  "multiSelect": false
}
```

### Step 4: Record Clarifications

After receiving answers, output a JSON object with the clarifications to be merged into the spec.

## Output Format

```json
{
  "clarifications": [
    {
      "category": "Functional Scope",
      "question": "How should the system handle...",
      "answer": "Fail fast with error",
      "impact": "Modifies FR-001"
    }
  ],
  "updated_requirements": [
    "FR-001: CLI exits with code 1 and displays parse error on invalid YAML"
  ],
  "updated_nfr": [],
  "updated_scenarios": []
}
```

## Rules

1. **Maximum 5 questions** per clarification session
2. **Prioritize by impact**: Focus on ambiguities that could lead to rework
3. **No leading questions**: Options should be genuinely distinct
4. **Skip if clear**: If spec is unambiguous, output empty clarifications
5. **Constitution compliance**: Ensure clarifications align with project principles

## Skip Conditions

If the specification is clear and complete, output:
```json
{
  "clarifications": [],
  "message": "Specification is sufficiently clear. No clarification needed."
}
```
