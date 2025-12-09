# AWF Development Tracking

Kanban-based feature tracking system for ai-workflow-cli development.

## Structure

```
docs/plans/
├── KANBAN.md           # Main Kanban board
├── features/           # Individual feature specs
│   ├── TEMPLATE.md     # Feature template
│   ├── F001-*.md
│   └── ...
└── README.md           # This file
```

## Workflow

### Kanban Columns

| Column | Description |
|--------|-------------|
| **Backlog** | Identified, not prioritized |
| **Ready** | Specs clear, dependencies resolved |
| **In Progress** | Currently being developed |
| **Review** | Code review / testing |
| **Done** | Merged to main |

### Feature Lifecycle

1. Create feature file from `TEMPLATE.md`
2. Fill in description, acceptance criteria, tasks
3. Move to **Ready** when specs are complete
4. Move to **In Progress** when starting work
5. Update task checkboxes as you progress
6. Move to **Review** when PR is opened
7. Move to **Done** when merged

## Feature File Format

Each feature includes:
- **Metadata**: Status, phase, version, priority, estimation
- **Description**: What and why
- **Acceptance Criteria**: Definition of done
- **Dependencies**: Blocking/blocked relationships
- **Files Impacted**: Code locations
- **Technical Tasks**: Implementation checklist

## Estimation Scale

| Size | Duration | Scope |
|------|----------|-------|
| **S** | 1-2 days | Single component change |
| **M** | 3-5 days | Multiple files, one module |
| **L** | 1-2 weeks | Cross-module changes |
| **XL** | 2+ weeks | Architectural changes |

## Phases

| Phase | Version | Focus |
|-------|---------|-------|
| 1 | v0.1.0 | MVP - Core engine |
| 2 | v0.2.0 | Core features |
| 3 | v0.3.0 | Advanced features |
| 4 | v0.4.0 | Extensibility |
| 5 | v1.0.0 | Additional interfaces |

## Commands

```bash
# View Kanban board
cat docs/plans/KANBAN.md

# List features by status
grep -l "Statut.*in-progress" docs/plans/features/*.md

# Find blocking dependencies
grep -l "Bloqué par" docs/plans/features/*.md
```
