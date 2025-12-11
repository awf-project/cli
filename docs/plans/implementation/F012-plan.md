# Implementation Plan: F012 - Input Validation

## Summary

Implement runtime input validation for workflow inputs against defined rules in `pkg/validation/`. The validator will check required fields, type constraints, regex patterns, enum values, numeric ranges, and file existence. Validation runs in `ExecutionService.Run()` after default values are applied, collecting all errors (not fail-fast) for clear user feedback.

## ASCII Wireframe

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI (run.go)                             │
│  awf run workflow --input email=test --input count=50          │
└────────────────────────────────┬────────────────────────────────┘
                                 │ map[string]any
┌────────────────────────────────▼────────────────────────────────┐
│                   ExecutionService.Run()                        │
│  1. Load workflow                                               │
│  2. Apply defaults ← existing                                   │
│  3. ──────────────────────────────────                          │
│     validation.ValidateInputs(inputs, wf.Inputs)  ← NEW        │
│     ──────────────────────────────────                          │
│  4. Execute state machine                                       │
└────────────────────────────────┬────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────┐
│               pkg/validation/validator.go                       │
│  ValidateInputs(inputs, defs) → error                          │
│    ├── validateRequired()                                       │
│    ├── validateType()                                           │
│    ├── validatePattern()                                        │
│    ├── validateEnum()                                           │
│    ├── validateRange()                                          │
│    ├── validateFileExists()                                     │
│    └── validateFileExtension()                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Create `pkg/validation/validator.go`
- **File:** `pkg/validation/validator.go`
- **Action:** CREATE
- **Changes:**
  ```go
  package validation

  // ValidateInputs validates all inputs against their definitions.
  // Returns aggregated errors (not fail-fast).
  func ValidateInputs(inputs map[string]any, definitions []Input) error

  // Input mirrors workflow.Input for decoupling pkg from internal/domain
  type Input struct {
      Name       string
      Type       string // "string", "integer", "boolean"
      Required   bool
      Validation *Rules
  }

  type Rules struct {
      Pattern       string
      Enum          []string
      Min           *int
      Max           *int
      FileExists    bool
      FileExtension []string
  }

  // Internal validators (unexported)
  func validateRequired(name string, value any, required bool) error
  func validateType(name string, value any, expectedType string) (any, error)
  func validatePattern(name string, value string, pattern string) error
  func validateEnum(name string, value string, allowed []string) error
  func validateRange(name string, value int, min, max *int) error
  func validateFileExists(name string, path string) error
  func validateFileExtension(name string, path string, allowed []string) error
  ```

### Step 2: Create `pkg/validation/validator_test.go`
- **File:** `pkg/validation/validator_test.go`
- **Action:** CREATE
- **Changes:** Table-driven tests covering:
  - Required field present/missing
  - Type validation (string, integer, boolean) including coercion
  - Pattern matching (valid regex, invalid regex, match/no-match)
  - Enum validation (in list, not in list)
  - Range validation (min only, max only, both, boundary cases)
  - File exists (exists, not exists, directory)
  - File extension (valid ext, invalid ext, no ext)
  - Multiple errors aggregation
  - Empty definitions (no validation)

### Step 3: Integrate in `execution_service.go`
- **File:** `internal/application/execution_service.go`
- **Action:** MODIFY (lines 77-87)
- **Changes:**
  ```go
  import "github.com/vanoix/awf/pkg/validation"

  // In Run(), after applying defaults (line 86):
  
  // Validate inputs against definitions
  if err := s.validateInputs(execCtx.Inputs, wf.Inputs); err != nil {
      return nil, fmt.Errorf("input validation failed: %w", err)
  }
  ```
  
  Add helper method:
  ```go
  // validateInputs converts workflow.Input to validation.Input and validates.
  func (s *ExecutionService) validateInputs(inputs map[string]any, defs []workflow.Input) error {
      valDefs := make([]validation.Input, len(defs))
      for i, d := range defs {
          valDefs[i] = validation.Input{
              Name:     d.Name,
              Type:     d.Type,
              Required: d.Required,
          }
          if d.Validation != nil {
              valDefs[i].Validation = &validation.Rules{
                  Pattern:       d.Validation.Pattern,
                  Enum:          d.Validation.Enum,
                  Min:           d.Validation.Min,
                  Max:           d.Validation.Max,
                  FileExists:    d.Validation.FileExists,
                  FileExtension: d.Validation.FileExtension,
              }
          }
      }
      return validation.ValidateInputs(inputs, valDefs)
  }
  ```

### Step 4: Add test fixture
- **File:** `tests/fixtures/workflows/valid-input-validation.yaml`
- **Action:** CREATE
- **Changes:** Workflow with all validation rule types for integration testing

### Step 5: Add unit tests for execution_service integration
- **File:** `internal/application/execution_service_test.go`
- **Action:** MODIFY
- **Changes:** Add tests for:
  - Valid inputs pass validation
  - Invalid inputs return validation error
  - Validation runs after defaults applied

## Test Plan

### Unit Tests (`pkg/validation/validator_test.go`)
| Test Case | Coverage |
|-----------|----------|
| `TestValidateRequired` | present, missing, nil value |
| `TestValidateType_String` | string, coerce from other types |
| `TestValidateType_Integer` | int, int64, coerce string "10" |
| `TestValidateType_Boolean` | bool, coerce "true"/"false" |
| `TestValidatePattern` | match, no-match, invalid regex |
| `TestValidateEnum` | in list, not in list, empty list |
| `TestValidateRange` | min only, max only, both, nil |
| `TestValidateFileExists` | file exists, not exists, is dir |
| `TestValidateFileExtension` | valid ext, invalid ext, no ext |
| `TestValidateInputs_Multiple` | aggregate all errors |
| `TestValidateInputs_NoValidation` | skip validation if rules nil |

### Integration Tests
| Test Case | Location |
|-----------|----------|
| Run workflow with valid inputs | `internal/application/execution_service_test.go` |
| Run workflow with invalid inputs | `internal/application/execution_service_test.go` |
| CLI shows validation errors | `internal/interfaces/cli/run_test.go` |

## Files to Modify

| File | Action | Complexity | Lines Changed |
|------|--------|------------|---------------|
| `pkg/validation/validator.go` | CREATE | M | ~150 |
| `pkg/validation/validator_test.go` | CREATE | M | ~300 |
| `internal/application/execution_service.go` | MODIFY | S | ~25 |
| `internal/application/execution_service_test.go` | MODIFY | S | ~50 |
| `tests/fixtures/workflows/valid-input-validation.yaml` | CREATE | S | ~40 |

## Error Message Format

```
input validation failed: 3 errors:
  - inputs.email: required but not provided
  - inputs.count: value 150 exceeds maximum 100
  - inputs.file: file does not exist: /path/to/missing.txt
```

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Type coercion edge cases (YAML "10" as string vs int) | Medium | Handle both types in `validateType`, document behavior |
| Regex compilation panic | Low | Recover and return clear error message |
| File system race condition | Low | Document that file_exists is point-in-time check |
| Breaking existing workflows | Low | Validation only runs if `validation:` block defined |
| Performance with many file checks | Low | File checks are I/O but typically few inputs |

## Dependencies

- **External packages:** None (stdlib only: `regexp`, `os`, `filepath`, `errors`, `strconv`)
- **Internal dependencies:** None (pkg/validation is standalone)
- **Blocked by:** Nothing (domain entities exist)

## Out of Scope

- Custom validation functions (future: F021 Plugin System)
- Async/parallel validation
- Validation caching
- Remote file validation (URLs)

