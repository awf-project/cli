// Package validation provides input validation utilities for workflow parameters.
//
// This package implements the public API for validating workflow input parameters
// against type constraints and validation rules. It performs type coercion,
// pattern matching, enum checking, range validation, and file validation.
//
// Key features:
//   - Type validation and coercion (string, integer, boolean)
//   - Pattern matching with regex
//   - Enum validation (allowed values list)
//   - Range validation for integers (min/max)
//   - File validation (existence, extension)
//   - Aggregated error reporting (not fail-fast)
//
// # Core Types
//
// ## Input (validator.go)
//
// Input defines a parameter for validation:
//   - Name: Parameter name
//   - Type: Expected type (string, integer, boolean)
//   - Required: Whether parameter is mandatory
//   - Validation: Optional validation rules
//
// ## Rules (validator.go)
//
// Rules defines validation constraints:
//   - Pattern: Regex pattern for strings
//   - Enum: Allowed values list
//   - Min: Minimum value for integers
//   - Max: Maximum value for integers
//   - FileExists: File must exist on disk
//   - FileExtension: Allowed file extensions
//
// ## ValidationError (validator.go)
//
// ValidationError aggregates multiple validation failures:
//   - Errors: Slice of error messages
//   - Error: Formatted multi-line error message
//
// # Validation Process
//
// Validation follows this order for each input:
//
//  1. Check required constraint (fail if missing and required)
//  2. Validate and coerce type (string, integer, boolean)
//  3. Apply validation rules (pattern, enum, range, file)
//
// All errors are collected and returned together (not fail-fast).
//
// # Type Coercion
//
// ## String Coercion
//
// Convert any value to string:
//   - int -> strconv.Itoa
//   - int64 -> strconv.FormatInt
//   - float64 -> strconv.FormatFloat
//   - bool -> strconv.FormatBool
//   - default -> fmt.Sprintf("%v", v)
//
// ## Integer Coercion
//
// Convert value to int:
//   - int -> keep as-is
//   - int64 -> int(v)
//   - float64 -> int(v) (truncate)
//   - string -> strconv.Atoi
//
// ## Boolean Coercion
//
// Convert string to bool (case-insensitive):
//   - "true", "yes", "1" -> true
//   - "false", "no", "0" -> false
//
// # Validation Rules
//
// ## Pattern Validation
//
// Regex pattern matching for strings:
//   - Empty pattern skips validation
//   - Invalid regex returns error
//   - Pattern must match entire value
//
// ## Enum Validation
//
// Allowed values list for strings:
//   - Empty enum list skips validation
//   - Value must exactly match one allowed value
//   - Case-sensitive comparison
//
// ## Range Validation
//
// Min/max bounds for integers:
//   - Min only: value >= min
//   - Max only: value <= max
//   - Both: min <= value <= max
//
// ## File Validation
//
// File existence and extension checks:
//   - FileExists: os.Stat check
//   - FileExtension: filepath.Ext comparison (case-insensitive)
//   - Empty path skips validation (for optional file inputs)
//
// # Usage Examples
//
// ## Basic Type Validation
//
//	inputs := map[string]any{
//	    "name":  "Alice",
//	    "age":   "30",
//	    "admin": "true",
//	}
//
//	definitions := []validation.Input{
//	    {Name: "name", Type: "string", Required: true},
//	    {Name: "age", Type: "integer", Required: true},
//	    {Name: "admin", Type: "boolean", Required: false},
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil (all valid, types coerced)
//
// ## Pattern Validation
//
//	inputs := map[string]any{
//	    "email": "user@example.com",
//	}
//
//	definitions := []validation.Input{
//	    {
//	        Name:     "email",
//	        Type:     "string",
//	        Required: true,
//	        Validation: &validation.Rules{
//	            Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
//	        },
//	    },
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil (email matches pattern)
//
// ## Enum Validation
//
//	inputs := map[string]any{
//	    "env": "production",
//	}
//
//	definitions := []validation.Input{
//	    {
//	        Name:     "env",
//	        Type:     "string",
//	        Required: true,
//	        Validation: &validation.Rules{
//	            Enum: []string{"development", "staging", "production"},
//	        },
//	    },
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil ("production" is in enum)
//
// ## Range Validation
//
//	inputs := map[string]any{
//	    "count": "5",
//	}
//
//	min := 1
//	max := 10
//	definitions := []validation.Input{
//	    {
//	        Name:     "count",
//	        Type:     "integer",
//	        Required: true,
//	        Validation: &validation.Rules{
//	            Min: &min,
//	            Max: &max,
//	        },
//	    },
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil (5 is within [1, 10])
//
// ## File Validation
//
//	inputs := map[string]any{
//	    "config": "config.yaml",
//	}
//
//	definitions := []validation.Input{
//	    {
//	        Name:     "config",
//	        Type:     "string",
//	        Required: true,
//	        Validation: &validation.Rules{
//	            FileExists:    true,
//	            FileExtension: []string{".yaml", ".yml"},
//	        },
//	    },
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil if file exists with .yaml extension
//	// err: ValidationError if file missing or wrong extension
//
// ## Multiple Validation Errors
//
//	inputs := map[string]any{
//	    "name":  "",
//	    "age":   "invalid",
//	    "email": "not-an-email",
//	}
//
//	definitions := []validation.Input{
//	    {Name: "name", Type: "string", Required: true},
//	    {Name: "age", Type: "integer", Required: true},
//	    {
//	        Name: "email",
//	        Type: "string",
//	        Validation: &validation.Rules{
//	            Pattern: `^.+@.+\..+$`,
//	        },
//	    },
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: ValidationError with 3 errors:
//	//   - inputs.name: required but empty
//	//   - inputs.age: cannot convert "invalid" to integer
//	//   - inputs.email: value "not-an-email" does not match pattern
//
// ## Optional Input with Validation
//
//	inputs := map[string]any{
//	    // email not provided
//	}
//
//	definitions := []validation.Input{
//	    {
//	        Name:     "email",
//	        Type:     "string",
//	        Required: false,
//	        Validation: &validation.Rules{
//	            Pattern: `^.+@.+\..+$`,
//	        },
//	    },
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil (optional input not provided, validation skipped)
//
// ## Type Coercion Examples
//
//	inputs := map[string]any{
//	    "count":   "42",        // string -> int
//	    "enabled": "true",      // string -> bool
//	    "ratio":   3.14,        // float64 -> int (truncate to 3)
//	    "flag":    "yes",       // string -> bool (true)
//	}
//
//	definitions := []validation.Input{
//	    {Name: "count", Type: "integer"},
//	    {Name: "enabled", Type: "boolean"},
//	    {Name: "ratio", Type: "integer"},
//	    {Name: "flag", Type: "boolean"},
//	}
//
//	err := validation.ValidateInputs(inputs, definitions)
//	// err: nil (all coerced successfully)
//
// # Workflow Integration
//
// Validation is performed at workflow execution time before step execution:
//
//	# workflow.yaml
//	inputs:
//	  - name: config_file
//	    type: string
//	    required: true
//	    validation:
//	      file_exists: true
//	      file_extension: [".yaml", ".yml"]
//	  - name: max_retries
//	    type: integer
//	    required: false
//	    validation:
//	      min: 1
//	      max: 10
//	  - name: environment
//	    type: string
//	    required: true
//	    validation:
//	      enum: ["dev", "staging", "prod"]
//
// Command-line invocation:
//
//	awf run workflow.yaml --input config_file=settings.yaml --input max_retries=5 --input environment=prod
//
// # Error Messages
//
// Validation errors use descriptive messages:
//
//	inputs.name: required but not provided
//	inputs.name: required but empty
//	inputs.age: cannot convert "invalid" to integer
//	inputs.count: value 15 exceeds maximum 10
//	inputs.email: value "test" does not match pattern "^.+@.+$"
//	inputs.env: value "qa" not in allowed values [dev, staging, prod]
//	inputs.config: file does not exist: config.yaml
//	inputs.config: file "data.txt" has no extension, allowed: [.yaml, .yml]
//	inputs.config: file extension ".json" not in allowed extensions [.yaml, .yml]
//
// # Design Principles
//
// ## Public API Surface
//
// This is a public package (pkg/) for external consumers:
//   - Stable API with semantic versioning
//   - No internal/ package dependencies
//   - Clean separation from domain layer
//
// ## Aggregated Errors
//
// Collect all errors, don't fail fast:
//   - Users see all validation failures at once
//   - Faster feedback loop (fix all issues in one iteration)
//   - ValidationError type implements error interface
//
// ## Type Coercion
//
// Lenient type handling for user convenience:
//   - String inputs from CLI coerced to target types
//   - Boolean accepts "yes"/"no" in addition to "true"/"false"
//   - Numeric strings converted to int/float
//
// ## Skip Rules for Wrong Types
//
// Validation rules only apply to compatible types:
//   - Pattern validation skipped for non-string values
//   - Enum validation skipped for non-string values
//   - Range validation skipped for non-int values
//   - File validation skipped for non-string values
//
// # Related Documentation
//
// See also:
//   - internal/domain/workflow: Workflow input and validation types
//   - internal/application: Application service using validation
//   - docs/input-validation.md: Complete validation syntax reference
//   - CLAUDE.md: Project conventions and validation semantics
package validation
