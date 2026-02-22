package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
)

var _ ports.AuditTrailWriter = (*FileAuditTrailWriter)(nil)

// posixPipeBuf is the POSIX PIPE_BUF limit guaranteeing atomic O_APPEND writes.
const posixPipeBuf = 4096

// FileAuditTrailWriter appends JSONL audit entries to a local file.
// O_APPEND + entries under 4KB guarantee atomic writes per POSIX PIPE_BUF semantics.
type FileAuditTrailWriter struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileAuditTrailWriter opens or creates the audit log file at path.
// Creates parent directories as needed. File permissions are 0o600.
func NewFileAuditTrailWriter(path string) (*FileAuditTrailWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("creating audit log directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening audit log file: %w", err)
	}

	return &FileAuditTrailWriter{file: f}, nil
}

// Write marshals event to JSONL and appends it to the audit file.
// If the serialized entry exceeds posixPipeBuf bytes, input values are truncated
// iteratively (longest first) until the entry fits, and InputsTruncated is set.
func (w *FileAuditTrailWriter) Write(_ context.Context, event *workflow.AuditEvent) error {
	line, err := marshalJSONL(event)
	if err != nil {
		return fmt.Errorf("marshaling audit event: %w", err)
	}

	if len(line) > posixPipeBuf {
		line, err = truncateInputs(event)
		if err != nil {
			return fmt.Errorf("truncating audit event inputs: %w", err)
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Write(line); err != nil {
		return fmt.Errorf("writing audit event: %w", err)
	}

	return nil
}

// Close flushes and closes the underlying file. Safe to call multiple times.
func (w *FileAuditTrailWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	err := w.file.Close()
	w.file = nil

	return err
}

// marshalJSONL serializes event and appends a newline for JSONL format.
func marshalJSONL(event *workflow.AuditEvent) ([]byte, error) {
	data, err := event.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// truncateInputs reduces input values iteratively (longest first) until the
// serialized entry fits within posixPipeBuf. Sets InputsTruncated on the event.
func truncateInputs(event *workflow.AuditEvent) ([]byte, error) {
	event.InputsTruncated = true

	for {
		line, err := marshalJSONL(event)
		if err != nil {
			return nil, err
		}
		if len(line) <= posixPipeBuf {
			return line, nil
		}

		// No inputs left to truncate — emit the entry as-is (best effort).
		if len(event.Inputs) == 0 {
			return line, nil
		}

		// Replace the longest input value with the ellipsis placeholder.
		longestKey := longestInputKey(event.Inputs)
		event.Inputs[longestKey] = "…"
	}
}

// longestInputKey returns the key whose string representation of value is longest.
func longestInputKey(inputs map[string]any) string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		li := valueLength(inputs[keys[i]])
		lj := valueLength(inputs[keys[j]])
		if li != lj {
			return li > lj
		}
		return keys[i] < keys[j]
	})

	return keys[0]
}

// valueLength returns a comparable length for an input value.
func valueLength(v any) int {
	switch val := v.(type) {
	case string:
		return len(val)
	default:
		return 0
	}
}
