Generate a CHANGELOG entry for feature {{.inputs.feature_id}}.

## Feature Spec
$SPEC_CONTENT

## Implementation Plan
$PLAN_CONTENT

## Your Task
Create a rich changelog entry following this format:
- **{{.inputs.feature_id}}**: Short title (from spec H1)
  - Key feature/capability 1 (use backticks for code terms)
  - Key feature/capability 2
  - Key feature/capability 3
  - (3-5 bullets max, focus on user-facing changes)

Rules:
1. Title: Extract from spec H1, keep concise (5-10 words)
2. Bullets: Focus on WHAT it does, not HOW it's implemented
3. Use backticks for: commands, flags, config keys, code terms
4. Skip implementation details (internal refactors, test changes)
5. If breaking change: add 'BREAKING:' prefix to relevant bullet

Output ONLY the markdown entry, nothing else.
