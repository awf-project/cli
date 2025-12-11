# Feature Roadmap

## Version Overview

```
v0.1.0 (MVP)        ████████████████████ IMPLEMENTED
v0.2.0 (Core)       ░░░░░░░░░░░░░░░░░░░░ PLANNED
v0.3.0 (Advanced)   ░░░░░░░░░░░░░░░░░░░░ PLANNED
v0.4.0 (AI/Plugins) ░░░░░░░░░░░░░░░░░░░░ PLANNED
v1.0.0 (Interfaces) ░░░░░░░░░░░░░░░░░░░░ PLANNED
```

## v0.1.0 - MVP (IMPLEMENTED)

| ID | Feature | Status |
|----|---------|--------|
| F001 | Hexagonal Architecture | ✅ Done |
| F002 | YAML Parser | ✅ Done |
| F003 | Linear Execution | ✅ Done |
| F004 | JSON State Persistence | ✅ Done |
| F005 | CLI Basic Commands | ✅ Done |
| F006 | JSON Logging | ✅ Done |
| F007 | Variable Interpolation | ✅ Done |
| F008 | Hooks System | ✅ Done |
| F029 | Output Streaming | ✅ Done |
| F030 | XDG Workflow Discovery | ✅ Done |
| F031 | Output Formats | ✅ Done (table format now uses ASCII borders) |
| F035 | Step Working Directory | ✅ Done |
| F036 | CLI Init Command | ✅ Done |
| F037 | Step Success Feedback | ✅ Done |
| F038 | Prompt Storage & Init | 📋 PLANNED |
| F039 | Run Single Step | ✅ Done |
| F040 | Audit-Based Refactoring | 📋 PLANNED |

## v0.2.0 - Core Features (IN PROGRESS)

| ID | Feature | Status |
|----|---------|--------|
| F009 | State Machine Validation | 📋 PLANNED |
| F010 | Parallel Execution (errgroup) | ✅ IMPLEMENTED |
| F011 | Retry with Backoff | ✅ IMPLEMENTED |
| F012 | Input Validation | 📋 PLANNED |
| F013 | Workflow Resume | 📋 PLANNED |
| F014 | SQLite History | 📋 PLANNED |

## v0.3.0 - Advanced Features (PLANNED)

| ID | Feature | Priority |
|----|---------|----------|
| F015 | Conditional Steps | High |
| F016 | Loop Constructs | Medium |
| F017 | Workflow Templates | Medium |
| F018 | Encrypted Environment | Low |
| F019 | Dry Run Mode | Medium |
| F020 | Interactive Mode | Low |

## v0.4.0 - AI & Extensibility (PLANNED)

| ID | Feature | Priority |
|----|---------|----------|
| F021 | Plugin System | High |
| F022 | Custom Operations | Medium |
| F023 | Workflow Composition | Medium |
| F024 | Remote Workflows | Low |
| F032 | Agent Step Type | High |
| F033 | Agent Conversations | Medium |
| F034 | Agent Tool Use | Medium |

## v1.0.0 - Multiple Interfaces (PLANNED)

| ID | Feature | Priority |
|----|---------|----------|
| F025 | REST API | High |
| F026 | Web UI | Medium |
| F027 | Message Queue Integration | Low |
| F028 | Webhooks | Medium |

## Feature Spec Location

All feature specs are in `docs/plans/features/<version>/F<ID>-<name>.md`

Example: `docs/plans/features/v0.2.0/F010-parallel-execution.md`
