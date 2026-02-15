package application

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/awf-project/awf/internal/domain/workflow"
)

// OutputStreamer handles streaming large outputs to temporary files.
// C019: Prevents OOM from unbounded StepState.Output/Stderr growth by streaming to disk.
type OutputStreamer struct {
	config  workflow.OutputLimits
	counter uint64 // atomic counter for unique filenames
}

// NewOutputStreamer creates a new OutputStreamer with the given configuration.
func NewOutputStreamer(config workflow.OutputLimits) *OutputStreamer {
	return &OutputStreamer{
		config: config,
	}
}

// StreamOutput writes output to a temp file if it exceeds the configured limit.
// Returns the path to the temp file if streamed, or empty string if not streamed.
// Returns an error if temp file creation fails.
func (s *OutputStreamer) StreamOutput(content string) (string, error) {
	// Return empty string if content is within limit or empty
	if content == "" || int64(len(content)) <= s.config.MaxSize {
		return "", nil
	}

	// Determine temp directory
	tempDir := s.config.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	// Validate temp directory exists
	if _, err := os.Stat(tempDir); err != nil {
		return "", fmt.Errorf("temp directory not accessible: %w", err)
	}

	// Create unique temp file with awf-output prefix
	// Use atomic counter, timestamp, and PID for uniqueness to prevent collisions
	count := atomic.AddUint64(&s.counter, 1)
	timestamp := time.Now().UnixNano()
	pid := os.Getpid()
	filename := fmt.Sprintf("awf-output-%d-%d-%d.txt", pid, timestamp, count)
	path := filepath.Join(tempDir, filename)

	// Write content to file with restrictive permissions
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("failed to write streamed output: %w", err)
	}

	return path, nil
}

// StreamBoth streams both output and stderr if they exceed limits.
// Returns paths to temp files (empty if not streamed) and any error.
func (s *OutputStreamer) StreamBoth(output, stderr string) (outputPath, stderrPath string, err error) {
	// Stream output if needed
	outputPath, err = s.StreamOutput(output)
	if err != nil {
		return "", "", fmt.Errorf("failed to stream output: %w", err)
	}

	// Stream stderr if needed (independent of output)
	stderrPath, err = s.StreamOutput(stderr)
	if err != nil {
		// Cleanup output file if stderr streaming fails
		if outputPath != "" {
			_ = os.Remove(outputPath) // Ignore cleanup error
		}
		return "", "", fmt.Errorf("failed to stream stderr: %w", err)
	}

	return outputPath, stderrPath, nil
}
