I've prepared a detailed implementation plan for F038. Here's the summary:

---

# Implementation Plan: F038 - Prompt Storage and Initialization

## Summary

Extend AWF to support prompt file storage in `.awf/prompts/` directory with resolution via `@prompts/` prefix in `--input` values. The implementation touches only the CLI layer (interfaces) with a minor infrastructure helper, requiring no domain or application layer changes.

## Implementation Steps

| Step | File | Action | Description |
|------|------|--------|-------------|
| 1 | `internal/infrastructure/xdg/xdg.go` | MODIFY | Add `LocalPromptsDir()` helper returning `.awf/prompts` |
| 2 | `internal/interfaces/cli/init.go` | MODIFY | Add prompts directory creation + example prompt file |
| 3 | `internal/interfaces/cli/run.go` | MODIFY | Implement `@prompts/` prefix resolution in `parseInputFlags()` |
| 4 | `internal/interfaces/cli/list.go` | MODIFY | Add `awf list prompts` subcommand |
| 5 | `internal/interfaces/cli/ui/output.go` | MODIFY | Add `PromptInfo` type and `WritePrompts()` method |
| 6 | `internal/interfaces/cli/init.go` | MODIFY | Update help text to mention prompts |

## Files to Modify

| File | Action | Complexity |
|------|--------|------------|
| `internal/infrastructure/xdg/xdg.go` | MODIFY | S |
| `internal/interfaces/cli/init.go` | MODIFY | S |
| `internal/interfaces/cli/run.go` | MODIFY | M |
| `internal/interfaces/cli/list.go` | MODIFY | M |
| `internal/interfaces/cli/ui/output.go` | MODIFY | S |
| Test files (5) | MODIFY | S-M |

## Test Plan

**Unit Tests:**
- Prompts directory created by init
- `@prompts/file.md` resolution
- Path traversal blocked
- Whitespace trimming
- `awf list prompts` with various states

**Integration Tests:**
- Full workflow: init → create prompt → run with `@prompts/` input

## Risks

| Risk | Mitigation |
|------|------------|
| Path traversal attack | Validate resolved path stays within `.awf/prompts/` using `filepath.Clean` + prefix check |
| Large file loading | Could add size limit (defer to future) |
| Missing .awf directory | Clear error: "Run 'awf init' first" |

**Total LOC estimate**: ~385 lines

Want me to save this plan to the docs folder, or shall I proceed with implementation?

