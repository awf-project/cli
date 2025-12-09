# F031: Output Formats

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: medium
- **Estimation**: M

## Description

Support multiple output formats for CLI commands to enable scripting, CI/CD integration, and different user preferences.

**Formats:**
| Format | Description | Use case |
|--------|-------------|----------|
| `text` | Human-readable with colors (default) | Interactive terminal |
| `json` | Structured JSON | Scripting, pipes, CI/CD |
| `table` | Aligned columns with headers | Lists, reports |
| `quiet` | Minimal output (IDs, exit codes only) | Silent scripts |

**Flag:** `--output` / `-o`

## Critères d'Acceptance

- [ ] `awf list -o json` outputs JSON array of workflows
- [ ] `awf list -o table` outputs aligned table
- [ ] `awf status <id> -o json` outputs execution state as JSON
- [ ] `awf run <wf> -o json` outputs final result as JSON
- [ ] `awf run <wf> -o quiet` outputs only workflow ID
- [ ] `awf history -o json` outputs execution history as JSON
- [ ] Default is `text` (current behavior)
- [ ] Invalid format shows error with valid options
- [ ] JSON output is valid, parseable JSON
- [ ] `--no-color` is respected in text format
- [ ] `-o quiet` suppresses all non-essential output

## Dépendances

- **Bloqué par**: F001, F005
- **Débloque**: _none_

## Fichiers Impactés

```
internal/interfaces/cli/config.go           # Add OutputFormat to Config
internal/interfaces/cli/root.go             # Add --output flag
internal/interfaces/cli/list.go             # Format support
internal/interfaces/cli/status.go           # Format support
internal/interfaces/cli/run.go              # Format support
internal/interfaces/cli/history.go          # Format support
internal/interfaces/cli/ui/formatter.go     # Add format methods
internal/interfaces/cli/ui/json_output.go   # New: JSON formatting
internal/interfaces/cli/ui/table_output.go  # New: Table formatting
```

## Tâches Techniques

- [ ] Define OutputFormat type
  - [ ] Create enum: text, json, table, quiet
  - [ ] Add validation function
- [ ] Add global `--output` flag
  - [ ] Short form `-o`
  - [ ] Default: "text"
  - [ ] Validate against allowed values
- [ ] Implement JSON output
  - [ ] Define JSON structs for each command output
  - [ ] `awf list`: `[{"name": "...", "version": "...", "description": "..."}]`
  - [ ] `awf status`: `{"id": "...", "status": "...", "steps": [...]}`
  - [ ] `awf run`: `{"id": "...", "status": "...", "duration": "...", "steps": [...]}`
  - [ ] `awf history`: `[{"id": "...", "workflow": "...", "status": "...", "started_at": "..."}]`
- [ ] Implement table output
  - [ ] Use tabwriter for alignment
  - [ ] Headers in bold (if colors enabled)
  - [ ] Separator line
- [ ] Implement quiet output
  - [ ] `awf run`: output only workflow ID
  - [ ] `awf list`: output only workflow names (one per line)
  - [ ] `awf status`: output only status string
- [ ] Update Formatter
  - [ ] Add `OutputFormat` field
  - [ ] Add `JSON(v any)` method
  - [ ] Add `Table(headers []string, rows [][]string)` method
- [ ] Update each command
  - [ ] Check format from config
  - [ ] Call appropriate output method
- [ ] Write unit tests
- [ ] Update CLI help documentation

## Notes

JSON output examples:

```bash
# List workflows
$ awf list -o json
[
  {"name": "deploy", "version": "1.0.0", "description": "Deploy to production"},
  {"name": "test", "version": "1.0.0", "description": "Run test suite"}
]

# Run workflow
$ awf run deploy -o json
{
  "id": "abc123",
  "workflow": "deploy",
  "status": "completed",
  "duration": "45s",
  "steps": [
    {"name": "build", "status": "completed", "duration": "30s"},
    {"name": "push", "status": "completed", "duration": "15s"}
  ]
}

# Quiet mode for scripting
$ WORKFLOW_ID=$(awf run deploy -o quiet)
$ awf status $WORKFLOW_ID -o quiet
completed
```

Table output should detect terminal width and truncate long fields with `...`.

Consider `AWF_OUTPUT_FORMAT` environment variable for default format override.
