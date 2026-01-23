package application

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// OutputStreamer handles streaming large outputs to temporary files.
// C019: Prevents OOM from unbounded StepState.Output/Stderr growth by streaming to disk.
type OutputStreamer struct {
	config workflow.OutputLimits
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
	// Use timestamp and PID for uniqueness to prevent collisions
	timestamp := time.Now().UnixNano()
	pid := os.Getpid()
	filename := fmt.Sprintf("awf-output-%d-%d.txt", pid, timestamp)
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

// Cleanup removes a temporary file created by streaming.
// Does not error if the file doesn't exist.
func (s *OutputStreamer) Cleanup(path string) error {
	// Return early if path is empty
	if path == "" {
		return nil
	}

	// Remove file, ignore error if file doesn't exist
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup temp file: %w", err)
	}

	return nil
}

// Config returns the current output limits configuration.
func (s *OutputStreamer) Config() workflow.OutputLimits {
	return s.config
}
