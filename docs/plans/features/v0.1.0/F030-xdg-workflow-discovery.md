# F030: XDG Workflow Discovery

## Metadata
- **Statut**: backlog
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priorité**: medium
- **Estimation**: S

## Description

Implement XDG Base Directory Specification for workflow and configuration storage, plus local workflow discovery in current directory.

**Current behavior:**
- Default storage: `~/.awf/storage/`
- Local detection: only `./configs/workflows/`
- No XDG compliance

**New behavior with search order (priority high to low):**
1. `AWF_WORKFLOWS_PATH` environment variable (unchanged)
2. Local project: `./.awf/workflows/*.yaml`
3. Legacy local: `./configs/workflows/*.yaml` (backward compat)
4. XDG global: `$XDG_CONFIG_HOME/awf/workflows/*.yaml` (default: `~/.config/awf/workflows/`)

Workflows found in multiple locations are merged. Local workflows take precedence over global ones with the same name.

## Critères d'Acceptance

- [ ] Workflows in `./.awf/workflows/` are discovered
- [ ] `~/.config/awf/workflows/` is the new default global location
- [ ] `$XDG_CONFIG_HOME` is respected if set
- [ ] Local workflows override global ones with same name
- [ ] `awf list` shows workflow source (local/global)
- [ ] `awf init` creates `./.awf/workflows/` directory
- [ ] Backward compatibility: `./configs/workflows/` still works
- [ ] `~/.awf/` migration notice if old directory exists

## Dépendances

- **Bloqué par**: F001, F002
- **Débloque**: _none_

## Fichiers Impactés

```
internal/interfaces/cli/config.go              # XDG path resolution
internal/interfaces/cli/list.go                # Multi-source listing
internal/interfaces/cli/run.go                 # Workflow resolution
internal/interfaces/cli/validate.go            # Workflow resolution
internal/interfaces/cli/init.go                # New: init command
internal/infrastructure/repository/yaml_repository.go  # Multi-path support
internal/infrastructure/repository/composite_repository.go  # New: merge repos
```

## Tâches Techniques

- [ ] Add XDG path resolution
  - [ ] Create `xdg.go` helper with `ConfigHome()`, `DataHome()` functions
  - [ ] Respect `$XDG_CONFIG_HOME` or default to `~/.config`
  - [ ] Respect `$XDG_DATA_HOME` or default to `~/.local/share`
- [ ] Update workflow discovery order
  - [ ] 1. `AWF_WORKFLOWS_PATH` env var
  - [ ] 2. `./.awf/workflows/` (local project)
  - [ ] 3. `./configs/workflows/` (legacy, backward compat)
  - [ ] 4. `$XDG_CONFIG_HOME/awf/workflows/` (global)
- [ ] Create CompositeRepository
  - [ ] Aggregate multiple YAMLRepository instances
  - [ ] Local repos take precedence over global
  - [ ] Track source for each workflow
- [ ] Update `awf list` output
  - [ ] Add "Source" column showing local/global
  - [ ] Sort by source then name
- [ ] Add `awf init` command
  - [ ] Create `./.awf/workflows/` directory
  - [ ] Create example workflow file
- [ ] Migration handling
  - [ ] Detect `~/.awf/` directory
  - [ ] Print one-time migration notice with instructions
- [ ] Update storage paths
  - [ ] States: `$XDG_DATA_HOME/awf/states/`
  - [ ] Logs: `$XDG_DATA_HOME/awf/logs/`
  - [ ] History DB: `$XDG_DATA_HOME/awf/history.db`
- [ ] Write unit tests
- [ ] Update CLI help documentation

## Notes

XDG Base Directory Specification:
- `$XDG_CONFIG_HOME`: User config (default `~/.config`)
- `$XDG_DATA_HOME`: User data (default `~/.local/share`)

Proposed AWF structure:
```
# Local (per-project)
./.awf/
└── workflows/
    └── my-workflow.yaml

# Global (user-wide)
~/.config/awf/
├── config.yaml         # Future: global config file
└── workflows/
    └── shared-workflow.yaml

~/.local/share/awf/
├── states/             # Execution state files
├── logs/               # Log files
└── history.db          # SQLite history (F014)
```

All `*.yaml` files in workflow directories are treated as workflows (no content validation).
