package application

import (
	"errors"
	"strings"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
)

// ErrInternal is the fallback ErrorCode when MapError encounters an unrecognized variant.
// Signals a mapping gap: add a case to MapError when a new StructuredError code is introduced.
const ErrInternal = domainerrors.ErrorCodeSystemInternalUnmapped

// MapError extracts the canonical ErrorCode from a StructuredError.
// Returns empty ErrorCode for nil, ErrInternal for non-StructuredError types (fail-closed, D8).
func MapError(err error) domainerrors.ErrorCode {
	if err == nil {
		return ""
	}
	var se *domainerrors.StructuredError
	if errors.As(err, &se) {
		return se.Code
	}
	return ErrInternal
}

// ExitCode maps an ErrorCode to the process exit code for the category.
// Returns 0 for empty code, 1–4 per the error taxonomy in CLAUDE.md.
func ExitCode(code domainerrors.ErrorCode) int {
	if code == "" {
		return 0
	}
	switch code.Category() {
	case "USER":
		return 1
	case "WORKFLOW":
		return 2
	case "EXECUTION":
		return 3
	case "SYSTEM":
		return 4
	default:
		return 1
	}
}

// HTTPStatus maps an ErrorCode to an HTTP status code.
// USER.FACADE.*_NOT_FOUND → 404; SESSION_CLOSED/DUPLICATE_RESPONSE → 409;
// USER.INPUT.* and other USER.FACADE.* → 400; WORKFLOW.* → 422;
// EXECUTION.*.TIMEOUT → 503; SYSTEM.* and other EXECUTION.* → 500.
func HTTPStatus(code domainerrors.ErrorCode) int {
	category := code.Category()
	subcategory := code.Subcategory()
	specific := code.Specific()

	if category == "USER" && subcategory == "FACADE" {
		if strings.HasSuffix(specific, "_NOT_FOUND") {
			return 404
		}
		if specific == "SESSION_CLOSED" || specific == "DUPLICATE_RESPONSE" {
			return 409
		}
		return 400
	}

	if category == "EXECUTION" && specific == "TIMEOUT" {
		return 503
	}

	switch category {
	case "USER":
		return 400
	case "WORKFLOW":
		return 422
	case "EXECUTION", "SYSTEM":
		return 500
	default:
		return 500
	}
}

// RPCCode maps an ErrorCode to a gRPC status code string.
// USER.FACADE.*_NOT_FOUND → NOT_FOUND; SESSION_CLOSED/DUPLICATE_RESPONSE → FAILED_PRECONDITION;
// USER.* and WORKFLOW.* → INVALID_ARGUMENT; EXECUTION.*.TIMEOUT → DEADLINE_EXCEEDED;
// SYSTEM.* and other EXECUTION.* → INTERNAL.
func RPCCode(code domainerrors.ErrorCode) string {
	category := code.Category()
	subcategory := code.Subcategory()
	specific := code.Specific()

	if category == "USER" && subcategory == "FACADE" {
		if strings.HasSuffix(specific, "_NOT_FOUND") {
			return "NOT_FOUND"
		}
		if specific == "SESSION_CLOSED" || specific == "DUPLICATE_RESPONSE" {
			return "FAILED_PRECONDITION"
		}
		return "INVALID_ARGUMENT"
	}

	if category == "EXECUTION" && specific == "TIMEOUT" {
		return "DEADLINE_EXCEEDED"
	}

	switch category {
	case "USER", "WORKFLOW":
		return "INVALID_ARGUMENT"
	case "EXECUTION", "SYSTEM":
		return "INTERNAL"
	default:
		return "INTERNAL"
	}
}
