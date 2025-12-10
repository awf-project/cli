# Development History

## Project Evolution

The project follows a feature-driven development approach with feature IDs (F001-F034).

### Initial Commits (Foundation)

| Commit | Feature | Description |
|--------|---------|-------------|
| `b4a4703` | F001 | Hexagonal architecture foundation - ports & adapters |
| `3f24ad9` | F002, F003 | YAML parser + linear execution engine |
| `19b7bdc` | F004 | JSON state persistence with atomic writes |
| `d563b4a` | F005 | Basic CLI commands (run, list, status, validate) |
| `1d7bd98` | F006 | JSON structured logging with secret masking |
| `a460d89` | F007, F008 | Variable interpolation + hooks system |

### Recent Commits (CLI Enhancement)

| Commit | Feature | Description |
|--------|---------|-------------|
| `2e26d0b` | F029 | Output streaming (silent/streaming/buffered modes) |
| `c561480` | F030 | XDG workflow discovery (~/.config/awf/) |
| `9b9a660` | F031 | Output format flag (text/json/table/quiet) |
| `909d95a` | F032-F034 | Agent step type specs (docs only) |
| `661d79f` | F035 | Step working directory (dir field) |
| `pending` | F036 | CLI init command with --force flag |

## Commit Convention

Format: `<type>(<scope>): <description> (Feature ID)`

Examples:
- `feat(core): implement hexagonal architecture foundation (F001)`
- `feat(cli): add output format flag for JSON/table/quiet modes (F031)`
- `docs(features): add agent step type specs for v0.4.0 (F032-F034)`

All commits include:
```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
```

## Key Milestones

1. **Architecture Setup** (F001) - Hexagonal foundation with 41 tests
2. **Core Execution** (F002-F004) - YAML parsing, execution, persistence
3. **CLI MVP** (F005) - Full CLI with 42 unit + 15 integration tests
4. **Observability** (F006-F008) - Logging, interpolation, hooks
5. **UX Polish** (F029-F031) - Streaming, XDG, output formats

## Test Evolution

- F001: 41 tests (domain layer verification)
- F005: 42 unit + 15 integration tests
- Current: ~100+ tests across all packages
