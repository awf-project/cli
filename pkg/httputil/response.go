package httputil

import (
	"errors"
	"fmt"
	"io"
)

// Response represents an HTTP response with parsed components.
// StatusCode is the HTTP status code (e.g., 200, 404, 503).
// Body is the response body as a string.
// Headers maps canonicalized header names to their values (multi-value headers joined with ", ").
// Truncated indicates whether the response body was truncated due to size limits.
type Response struct {
	StatusCode int
	Body       string
	Headers    map[string]string
	Truncated  bool
}

// ReadBody reads an HTTP response body with optional size limiting.
// If maxBodyBytes <= 0, reads the entire body without limit (existing notify behavior).
// If maxBodyBytes > 0, limits reading to maxBodyBytes and sets truncated=true if exceeded.
// Uses io.LimitReader to detect truncation by reading maxBodyBytes+1.
// Tolerates EOF and ErrUnexpectedEOF errors per existing responseToString pattern.
// Returns the body string and truncation status, or an error on read failure.
func ReadBody(body io.ReadCloser, maxBodyBytes int64) (bodyStr string, truncated bool, err error) {
	var data []byte

	if maxBodyBytes <= 0 {
		data, err = io.ReadAll(body)
		if err != nil && !isTolerableEOF(err) {
			return "", false, fmt.Errorf("failed to read response body: %w", err)
		}
		return string(data), false, nil
	}

	limitedReader := io.LimitReader(body, maxBodyBytes+1)
	data, err = io.ReadAll(limitedReader)

	if err != nil && !isTolerableEOF(err) {
		return "", false, fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(data)) > maxBodyBytes {
		return string(data[:maxBodyBytes]), true, nil
	}

	return string(data), false, nil
}

// isTolerableEOF checks if an error is EOF or ErrUnexpectedEOF.
// These errors are tolerated per the existing notify/responseToString pattern.
func isTolerableEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
