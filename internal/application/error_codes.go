package application

import (
	"errors"

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
// Returns 0 for empty code, 1–4 per the error taxonomy in CLAUDE.md:
//
//	USER      → 1 (bad input, missing file)
//	WORKFLOW  → 2 (invalid state reference, cycle)
//	EXECUTION → 3 (command failed, timeout)
//	SYSTEM    → 4 (IO, permissions, internal unmapped)
//
// Unknown categories return 4 (system/internal) rather than 1, so that
// unrecognized codes do not masquerade as user errors and are clearly
// distinguished from successful exit (0).
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
		// Unknown category: treated as a system/internal error (exit 4) rather
		// than a user error (exit 1) to prevent miscategorised codes from silently
		// mapping to the user-error bucket and confusing callers.
		return 4
	}
}
