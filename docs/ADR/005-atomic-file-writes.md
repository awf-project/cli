---
title: "005: Atomic File Writes for State Persistence"
---

**Status**: Accepted
**Date**: 2025-12-01
**Supersedes**: N/A
**Superseded by**: N/A

## Context

AWF persists workflow state to JSON files during execution. Workflows can be long-running (minutes) and may be interrupted by signals, crashes, or concurrent access. A partial write would corrupt the state file, making workflow resumption impossible.

## Candidates

| Option | Pros | Cons |
|--------|------|------|
| Atomic write (temp + rename) | Corruption-proof, OS-guaranteed atomicity on same filesystem | Requires same-filesystem temp, slightly more code |
| Direct write with fsync | Simpler code | Partial writes on crash, no protection against concurrent access |
| SQLite WAL | ACID transactions, concurrent reads | CGO dependency (already present), heavier for simple state |

## Decision

Use temp file + rename pattern for all state file writes:

1. Write to unique temp file (PID + timestamp suffix) in same directory
2. Sync to disk
3. Rename atomically to target path
4. File locking for concurrent access protection

Rules:
- All file writes in infrastructure layer use this pattern
- Temp file names include PID and timestamp for uniqueness
- Same-directory temp files to guarantee same-filesystem rename
- File locking via `flock` for concurrent JSONStore access

## Consequences

**What becomes easier:**
- Workflow resume after crash is always safe
- Concurrent `awf status` reads never see partial state
- No corruption recovery code needed

**What becomes harder:**
- Slightly more complex write path
- Must ensure temp files are cleaned up on error paths

## Constitution Compliance

| Principle | Status | Justification |
|-----------|--------|---------------|
| Security First | Compliant | Prevents data corruption, ensures integrity |
| Go Idioms | Compliant | Uses os.Rename which is atomic on POSIX |
| Error Taxonomy | Compliant | Write failures map to exit code 4 (system error) |
