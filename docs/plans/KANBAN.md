# AWF Kanban Board

## Backlog

### Phase 2 - Core Features (v0.2.0)
- [F009](features/F009-state-machine.md) State machine avec transitions
- [F010](features/F010-parallel-execution.md) Exécution parallèle (errgroup)
- [F011](features/F011-retry-backoff.md) Retry avec backoff exponentiel
- [F012](features/F012-input-validation.md) Validation des inputs
- [F013](features/F013-workflow-resume.md) Commande resume
- [F014](features/F014-sqlite-history.md) Historique SQLite

### Phase 3 - Advanced Features (v0.3.0)
- [F015](features/F015-conditions.md) Conditions complexes (if/else)
- [F016](features/F016-loops.md) Boucles (for/while)
- [F017](features/F017-workflow-templates.md) Templates de workflows
- [F018](features/F018-encrypted-env.md) Variables d'environnement chiffrées
- [F019](features/F019-dry-run.md) Dry-run mode
- [F020](features/F020-interactive-mode.md) Interactive mode

### Phase 4 - Extensibilité (v0.4.0)
- [F021](features/F021-plugin-system.md) Plugin system
- [F022](features/F022-custom-operations.md) Custom operations
- [F023](features/F023-workflow-composition.md) Workflow composition (sous-workflows)
- [F024](features/F024-remote-workflows.md) Remote workflows (HTTP)

### Phase 5 - Interfaces (v1.0.0)
- [F025](features/F025-rest-api.md) API REST
- [F026](features/F026-webui.md) WebUI
- [F027](features/F027-message-queue.md) Message Queue support
- [F028](features/F028-webhooks.md) Webhooks

---

## Ready

_No features ready yet_

---

## In Progress

_No features in progress_

---

## Review

_No features in review_

---

## Done

### Phase 1 - MVP (v0.1.0)
- [F001](features/F001-hexagonal-architecture.md) Architecture hexagonale de base
- [F002](features/F002-yaml-parser.md) Parsing YAML des workflows
- [F003](features/F003-linear-execution.md) Exécution linéaire de steps
- [F004](features/F004-json-state-persistence.md) Persistance d'état JSON
- [F005](features/F005-cli-basic.md) CLI basique (run, list, status, validate)
- [F006](features/F006-json-logging.md) Logging JSON structuré
- [F007](features/F007-variable-interpolation.md) Interpolation de variables
- [F008](features/F008-hooks.md) Hooks pre/post
- [F029](features/v0.1.0/F029-output-streaming.md) Output streaming (--output flag)
- [F030](features/v0.1.0/F030-xdg-workflow-discovery.md) XDG workflow discovery

---

## Summary

| Phase | Total | Backlog | Ready | In Progress | Review | Done |
|-------|-------|---------|-------|-------------|--------|------|
| 1 - MVP | 10 | 0 | 0 | 0 | 0 | 10 |
| 2 - Core | 6 | 6 | 0 | 0 | 0 | 0 |
| 3 - Advanced | 6 | 6 | 0 | 0 | 0 | 0 |
| 4 - Extensibility | 4 | 4 | 0 | 0 | 0 | 0 |
| 5 - Interfaces | 4 | 4 | 0 | 0 | 0 | 0 |
| **Total** | **30** | **20** | **0** | **0** | **0** | **10** |
