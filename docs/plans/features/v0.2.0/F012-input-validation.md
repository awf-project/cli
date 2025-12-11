# F012: Validation des Inputs

## Metadata
- **Status**: implemented
- **Phase**: 2-Core
- **Version**: v0.2.0
- **Priority**: high
- **Estimation**: M

## Description

Validate workflow inputs at runtime against defined rules. Support type checking, required fields, regex patterns, enums, numeric ranges, and file existence. Provide clear error messages for validation failures.

## Acceptance Criteria

- [x] Validate required inputs are present
- [x] Validate type (string, integer, boolean)
- [x] Validate pattern (regex match)
- [x] Validate enum (value in allowed list)
- [x] Validate min/max for integers
- [x] Validate file_exists
- [x] Validate file_extension
- [x] Clear error messages with input name
- [x] Apply default values

## Dependencies

- **Blocked by**: F002
- **Unblocks**: _none_

## Impacted Files

```
pkg/validation/validator.go
pkg/validation/validator_test.go
internal/domain/workflow/input.go
internal/application/validator.go
```

## Technical Tasks

- [x] Define InputDefinition struct
  - [x] Name
  - [x] Type (string, integer, boolean)
  - [x] Description
  - [x] Required
  - [x] Default
  - [x] Validation rules
- [x] Define ValidationRules struct
  - [x] Pattern (regex)
  - [x] Enum ([]string)
  - [x] Min, Max (int)
  - [x] FileExists (bool)
  - [x] FileExtension ([]string)
- [x] Implement InputValidator
  - [x] ValidateAll(inputs, definitions)
  - [x] ValidateRequired
  - [x] ValidateType
  - [x] ValidatePattern
  - [x] ValidateEnum
  - [x] ValidateRange
  - [x] ValidateFileExists
  - [x] ValidateFileExtension
- [x] Apply default values before validation
- [x] Collect all errors (not fail-fast)
- [x] Format error messages clearly
- [x] Write comprehensive unit tests

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
