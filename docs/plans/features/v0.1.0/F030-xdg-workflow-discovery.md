# F030: XDG Workflow Discovery

## Metadata
- **Status**: done
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
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
3. XDG global: `$XDG_CONFIG_HOME/awf/workflows/*.yaml` (default: `~/.config/awf/workflows/`)

Workflows found in multiple locations are merged. Local workflows take precedence over global ones with the same name.

## Acceptance Criteria

- [x] Workflows in `./.awf/workflows/` are discovered
- [x] `~/.config/awf/workflows/` is the new default global location
- [x] `$XDG_CONFIG_HOME` is respected if set
- [x] Local workflows override global ones with same name
- [x] `awf list` shows workflow source (local/global)
- [x] `awf init` creates `./.awf/workflows/` directory
- [x] `~/.awf/` migration notice if old directory exists

## Dependencies

- **Blocked by**: F001, F002
- **Unblocks**: _none_

## Impacted Files

```
internal/interfaces/cli/config.go              # XDG path resolution
internal/interfaces/cli/list.go                # Multi-source listing
internal/interfaces/cli/run.go                 # Workflow resolution
internal/interfaces/cli/validate.go            # Workflow resolution
internal/interfaces/cli/init.go                # New: init command
internal/infrastructure/repository/yaml_repository.go  # Multi-path support
internal/infrastructure/repository/composite_repository.go  # New: merge repos
```

## Technical Tasks

- [x] Add XDG path resolution
  - [x] Create `xdg.go` helper with `ConfigHome()`, `DataHome()` functions
  - [x] Respect `$XDG_CONFIG_HOME` or default to `~/.config`
  - [x] Respect `$XDG_DATA_HOME` or default to `~/.local/share`
- [x] Update workflow discovery order
  - [x] 1. `AWF_WORKFLOWS_PATH` env var
  - [x] 2. `./.awf/workflows/` (local project)
  - [x] 3. `$XDG_CONFIG_HOME/awf/workflows/` (global)
- [x] Create CompositeRepository
  - [x] Aggregate multiple YAMLRepository instances
  - [x] Local repos take precedence over global
  - [x] Track source for each workflow
- [x] Update `awf list` output
  - [x] Add "Source" column showing local/global
  - [x] Sort by source then name
- [x] Add `awf init` command
  - [x] Create `./.awf/workflows/` directory
  - [x] Create example workflow file
- [x] Migration handling
  - [x] Detect `~/.awf/` directory
  - [x] Print one-time migration notice with instructions
- [x] Update storage paths
  - [x] States: `$XDG_DATA_HOME/awf/states/`
  - [x] Logs: `$XDG_DATA_HOME/awf/logs/`
- [x] Write unit tests
- [x] Update CLI help documentation

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
