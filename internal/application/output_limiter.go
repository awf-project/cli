package application

import (
	"fmt"
	"unicode/utf8"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// OutputLimiter handles output size management for step execution.
// C019: Prevents OOM from unbounded StepState.Output/Stderr growth.
type OutputLimiter struct {
	config   workflow.OutputLimits
	streamer *OutputStreamer // C019: Delegates streaming to OutputStreamer
}

// NewOutputLimiter creates a new OutputLimiter with the given configuration.
func NewOutputLimiter(config workflow.OutputLimits) *OutputLimiter {
	return &OutputLimiter{
		config:   config,
		streamer: NewOutputStreamer(config),
	}
}

// LimitResult contains the result of applying output limits.
type LimitResult struct {
	Output     string // Truncated or full output
	Stderr     string // Truncated or full stderr
	OutputPath string // Path to temp file if streamed
	StderrPath string // Path to temp file if streamed
	Truncated  bool   // True if output was truncated
}

// Apply applies output limits to the given output and stderr strings.
// Returns a LimitResult indicating what happened (truncation, streaming, or pass-through).
func (l *OutputLimiter) Apply(output, stderr string) (*LimitResult, error) {
	result := &LimitResult{}

	// Handle unlimited config (pass-through)
	if l.config.MaxSize == 0 {
		result.Output = output
		result.Stderr = stderr
		return result, nil
	}

	// Check if streaming is enabled
	if l.config.StreamLargeOutput {
		// Stream mode: delegate to OutputStreamer
		outputPath, stderrPath, err := l.streamer.StreamBoth(output, stderr)
		if err != nil {
			return nil, fmt.Errorf("failed to stream outputs: %w", err)
		}

		// Populate result based on streaming outcome
		if outputPath != "" {
			result.OutputPath = outputPath
			// Output was streamed, don't store inline
		} else {
			result.Output = output
		}

		if stderrPath != "" {
			result.StderrPath = stderrPath
			// Stderr was streamed, don't store inline
		} else {
			result.Stderr = stderr
		}
		// Note: Truncated remains false - content is streamed, not truncated
	} else {
		// Truncation mode: truncate if over limit
		if l.ShouldLimit(output) {
			result.Output = l.Truncate(output)
			result.Truncated = true
		} else {
			result.Output = output
		}

		if l.ShouldLimit(stderr) {
			result.Stderr = l.Truncate(stderr)
			result.Truncated = true
		} else {
			result.Stderr = stderr
		}
	}

	return result, nil
}

// ShouldLimit returns true if the output exceeds the configured limit.
func (l *OutputLimiter) ShouldLimit(output string) bool {
	// Unlimited config never triggers limit
	if l.config.MaxSize == 0 {
		return false
	}

	// Check if output size exceeds limit
	return int64(len(output)) > l.config.MaxSize
}

// Truncate truncates the output to the configured limit with a truncation marker.
func (l *OutputLimiter) Truncate(output string) string {
	// No truncation needed if within limit
	if !l.ShouldLimit(output) {
		return output
	}

	// Calculate space for truncation marker
	marker := "...[truncated]"
	markerLen := len(marker)
	maxContentLen := int(l.config.MaxSize) - markerLen

	// Edge case: if limit is smaller than marker, just return marker
	if maxContentLen <= 0 {
		return marker
	}

	// Truncate at UTF-8 character boundary
	truncated := truncateAtCharBoundary(output, maxContentLen)

	return truncated + marker
}

// truncateAtCharBoundary truncates a string to at most maxBytes,
// ensuring we don't cut in the middle of a UTF-8 character.
func truncateAtCharBoundary(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// Find the last valid UTF-8 character boundary before maxBytes
	truncated := s[:maxBytes]
	for truncated != "" && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}

	return truncated
}

// Config returns the current output limits configuration.
func (l *OutputLimiter) Config() workflow.OutputLimits {
	return l.config
}
