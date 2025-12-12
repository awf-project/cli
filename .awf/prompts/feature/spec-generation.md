Generate a feature specification for the awf project.

## Feature Info
- ID: {{.inputs.feature_id}}
- Version: {{.inputs.version}}
- Description: {{.inputs.description}}

## Template to follow
$(cat docs/plans/features/TEMPLATE.md)

## Naming convention
Files are named: F0XX-feature-name-in-kebab-case.md
Examples: F036-cli-init.md, F007-variable-interpolation.md, F029-output-streaming.md

## Instructions
1. Fill in the template with appropriate content based on the description
2. Set Status to 'planned'
3. Set Version to {{.inputs.version}}
4. Generate realistic acceptance criteria
5. Leave Technical Tasks and Impacted Files with placeholders (will be filled after exploration)
6. Generate a short kebab-case slug for the filename (2-4 words max)

## Output format (IMPORTANT)
Output a JSON object with exactly this structure:
{"slug": "feature-name-slug", "content": "# F0XX: Title..."}

The slug should be lowercase, kebab-case, 2-4 words describing the feature.
The content should be the full markdown spec.

Output ONLY the JSON, no explanations.
