# F038: Prompt Storage and Initialization

## Metadata
- **Status**: implemented
- **Phase**: 1-MVP
- **Version**: v0.1.0
- **Priority**: medium
- **Estimation**: S

## Description

Enable AWF to store and manage prompt files in the `.awf/prompts/` directory. Prompts are reusable text templates that can be referenced when providing `--input` values to workflows, making it easier to work with long or complex input text (e.g., detailed AI prompts, multi-line instructions).

This feature extends the `awf init` command to create the prompts directory and allows workflows to reference stored prompts via a special syntax (e.g., `--input prompt=@prompts/my-prompt.md`).

## Acceptance Criteria

- [ ] `awf init` creates `.awf/prompts/` directory alongside `.awf/workflows/`
- [ ] Prompt files can be stored as `.md` or `.txt` files in `.awf/prompts/`
- [ ] Input values prefixed with `@prompts/` are resolved to file content from `.awf/prompts/`
- [ ] Error message shown when referenced prompt file does not exist
- [ ] Prompt file content is trimmed of leading/trailing whitespace before use
- [ ] `awf list prompts` command lists available prompt files
- [ ] Prompt content can be used in any `--input` flag value
- [ ] Nested directories supported (e.g., `@prompts/ai/system-prompt.md`)

## Dependencies

- **Blocked by**: F036 (init command)
- **Unblocks**: _none_

## Impacted Files

```
internal/domain/...
internal/application/...
internal/infrastructure/...
internal/interfaces/cli/...
```

## Technical Tasks

- [ ] Task 1: Extend init command
  - [ ] Add prompts directory creation to init
  - [ ] Create example prompt file
- [ ] Task 2: Implement prompt resolution
  - [ ] Detect `@prompts/` prefix in input values
  - [ ] Read and resolve prompt file content
  - [ ] Handle file not found errors
- [ ] Task 3: Add list prompts command
  - [ ] Implement `awf list prompts` subcommand
  - [ ] Display prompt files with metadata
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Update documentation

## Notes

- Syntax choice: `@prompts/filename` mirrors common conventions (e.g., `@file` in curl)
- Support both `.md` and `.txt` extensions for flexibility
- Consider adding `awf prompt create <name>` command in future iteration
- Prompt files should be git-trackable (not in `.gitignore`)

