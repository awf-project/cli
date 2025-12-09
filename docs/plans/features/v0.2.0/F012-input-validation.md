# F012: Validation des Inputs

## Metadata
- **Statut**: backlog
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priorité**: high
- **Estimation**: M

## Description

Validate workflow inputs at runtime against defined rules. Support type checking, required fields, regex patterns, enums, numeric ranges, and file existence. Provide clear error messages for validation failures.

## Critères d'Acceptance

- [ ] Validate required inputs are present
- [ ] Validate type (string, integer, boolean)
- [ ] Validate pattern (regex match)
- [ ] Validate enum (value in allowed list)
- [ ] Validate min/max for integers
- [ ] Validate file_exists
- [ ] Validate file_extension
- [ ] Clear error messages with input name
- [ ] Apply default values

## Dépendances

- **Bloqué par**: F002
- **Débloque**: _none_

## Fichiers Impactés

```
pkg/validation/validator.go
pkg/validation/validator_test.go
internal/domain/workflow/input.go
internal/application/validator.go
```

## Tâches Techniques

- [ ] Define InputDefinition struct
  - [ ] Name
  - [ ] Type (string, integer, boolean)
  - [ ] Description
  - [ ] Required
  - [ ] Default
  - [ ] Validation rules
- [ ] Define ValidationRules struct
  - [ ] Pattern (regex)
  - [ ] Enum ([]string)
  - [ ] Min, Max (int)
  - [ ] FileExists (bool)
  - [ ] FileExtension ([]string)
- [ ] Implement InputValidator
  - [ ] ValidateAll(inputs, definitions)
  - [ ] ValidateRequired
  - [ ] ValidateType
  - [ ] ValidatePattern
  - [ ] ValidateEnum
  - [ ] ValidateRange
  - [ ] ValidateFileExists
  - [ ] ValidateFileExtension
- [ ] Apply default values before validation
- [ ] Collect all errors (not fail-fast)
- [ ] Format error messages clearly
- [ ] Write comprehensive unit tests

## Notes

Input definition example:
```yaml
inputs:
  - name: email
    type: string
    required: true
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"

  - name: count
    type: integer
    default: 10
    validation:
      min: 1
      max: 100

  - name: file_path
    type: string
    required: true
    validation:
      file_exists: true
      file_extension: [".py", ".js", ".go"]
```
