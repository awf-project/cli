package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// RedactError hides untrusted transport diagnostics (URLs, credentials and
// response payloads), while preserving the cause for errors.Is and errors.As.
func RedactError(err error) error {
	if err == nil {
		return nil
	}
	message := "external operation failed"
	if errors.Is(err, context.Canceled) {
		message = context.Canceled.Error()
	} else if errors.Is(err, context.DeadlineExceeded) {
		message = context.DeadlineExceeded.Error()
	}
	return redactedError{cause: err, message: message}
}

type redactedError struct {
	cause   error
	message string
}

func (e redactedError) Error() string { return e.message }
func (e redactedError) Unwrap() error { return e.cause }

// Format also protects diagnostic verbs that would otherwise reflect fields.
func (e redactedError) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, e.message) //nolint:errcheck // fmt.Formatter cannot return writer errors; fmt owns the destination.
}
