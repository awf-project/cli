# F005: CLI Basique (run, list, status)

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: high
- **Estimation**: M

## Description

Implement core CLI commands using Cobra: `run` to execute workflows, `list` to show available workflows, `status` to check running workflow state. This is the primary user interface for AWF.

## Critères d'Acceptance

- [ ] `awf run <workflow> --input=value` executes workflow
- [ ] `awf list` shows available workflows
- [ ] `awf status <workflow-id>` shows execution status
- [ ] `awf version` shows version info
- [ ] `awf help` shows usage
- [ ] Global flags: --verbose, --quiet, --config, --storage
- [ ] Clear error messages with exit codes

## Dépendances

- **Bloqué par**: F001, F002, F003, F004
- **Débloque**: F013

## Fichiers Impactés

```
cmd/awf/main.go
internal/interfaces/cli/app.go
internal/interfaces/cli/commands/run.go
internal/interfaces/cli/commands/list.go
internal/interfaces/cli/commands/status.go
internal/interfaces/cli/commands/version.go
internal/interfaces/cli/ui/formatter.go
internal/interfaces/cli/ui/colors.go
```

## Tâches Techniques

- [ ] Set up Cobra root command
  - [ ] Global flags (verbose, quiet, config, storage, no-color, log-level)
  - [ ] Version command
  - [ ] Help customization
- [ ] Implement `run` command
  - [ ] Parse workflow name argument
  - [ ] Parse input flags dynamically
  - [ ] Call WorkflowService.Run()
  - [ ] Display progress output
  - [ ] Return appropriate exit code
- [ ] Implement `list` command
  - [ ] List workflows from repository
  - [ ] Show name, description, version
  - [ ] Table formatting
- [ ] Implement `status` command
  - [ ] Load state from store
  - [ ] Display current state, progress, duration
  - [ ] Show completed/pending steps
- [ ] Implement output formatter
  - [ ] Color support (fatih/color)
  - [ ] Respect --no-color flag
  - [ ] Verbose vs quiet modes
- [ ] Write integration tests

## Notes

Use Cobra for CLI framework. Flags should map directly to workflow inputs:

```bash
awf run analyze-code --file-path=app.py --max-tokens=2000
```

Exit codes follow error taxonomy:
- 0: Success
- 1: User error
- 2: Workflow error
- 3: Execution error
- 4: System error
