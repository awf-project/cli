package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

var _ ports.AuditTrailWriter = (*FileAuditTrailWriter)(nil)

var (
	ErrAuditRecordTooLarge = errors.New("audit record exceeds size limit")
	ErrAuditWriterClosed   = errors.New("audit writer is closed")
)

// maxAuditRecordSize bounds the complete serialized JSONL record, including its newline.
const maxAuditRecordSize = 4096

// FileAuditTrailWriter appends JSONL audit entries to a local file.
// Each bounded record is appended in one Write, protected against concurrent Close.
type FileAuditTrailWriter struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileAuditTrailWriter opens or creates the audit log file at path.
// Creates parent directories as needed. File permissions are 0o600.
func NewFileAuditTrailWriter(path string) (*FileAuditTrailWriter, error) {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating audit log directory: %w", err)
	}

	f, err := os.OpenFile(cleanPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path is cleaned above; callers control the input
	if err != nil {
		return nil, fmt.Errorf("opening audit log file: %w", err)
	}

	return &FileAuditTrailWriter{file: f}, nil
}

// Write marshals event to JSONL and appends it to the audit file.
// If the serialized entry exceeds maxAuditRecordSize bytes, input values are truncated
// iteratively (longest first) until the entry fits, and InputsTruncated is set.
func (w *FileAuditTrailWriter) Write(_ context.Context, event *workflow.AuditEvent) error {
	privateEvent, err := cloneAuditEvent(event)
	if err != nil {
		return fmt.Errorf("marshaling audit event: %w", err)
	}

	line, err := marshalJSONL(privateEvent)
	if err != nil {
		return fmt.Errorf("marshaling audit event: %w", err)
	}

	if len(line) > maxAuditRecordSize {
		line, err = truncateInputs(privateEvent)
		if errors.Is(err, ErrAuditRecordTooLarge) && privateEvent.Event == workflow.EventWorkflowCompleted {
			line, err = truncateCompletionError(privateEvent)
		}
		if err != nil {
			return fmt.Errorf("truncating audit event inputs: %w", err)
		}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return fmt.Errorf("writing audit event: %w", ErrAuditWriterClosed)
	}

	if _, err := w.file.Write(line); err != nil {
		return fmt.Errorf("writing audit event: %w", err)
	}

	return nil
}

// cloneAuditEvent converts the event to its JSON representation and back so all
// mutable input maps and nested values belong exclusively to this write.
func cloneAuditEvent(event *workflow.AuditEvent) (*workflow.AuditEvent, error) {
	if event == nil {
		return nil, errors.New("audit event is nil")
	}

	data, err := event.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var clone workflow.AuditEvent
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("decode cloned audit event: %w", err)
	}

	return &clone, nil
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

// truncateInputs visits each input once in deterministic longest-first/key order.
// It retains only replacements that reduce the complete serialized payload.
func truncateInputs(event *workflow.AuditEvent) ([]byte, error) {
	line, err := marshalJSONL(event)
	if err != nil {
		return nil, err
	}

	for _, key := range orderedInputKeys(event.Inputs) {
		original := event.Inputs[key]
		event.Inputs[key] = "…"
		event.InputsTruncated = true

		candidate, marshalErr := marshalJSONL(event)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if len(candidate) >= len(line) {
			event.Inputs[key] = original
			continue
		}

		line = candidate
		if len(line) <= maxAuditRecordSize {
			return line, nil
		}
	}

	return nil, fmt.Errorf("%w: serialized payload is %d bytes", ErrAuditRecordTooLarge, len(line))
}

// orderedInputKeys returns keys by descending serialized value size, breaking
// ties lexically so truncation produces stable records.
func orderedInputKeys(inputs map[string]any) []string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		li := serializedValueLength(inputs[keys[i]])
		lj := serializedValueLength(inputs[keys[j]])
		if li != lj {
			return li > lj
		}
		return keys[i] < keys[j]
	})

	return keys
}

func serializedValueLength(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(data)
}

// truncateCompletionError preserves the terminal identity and outcome even when
// an execution error alone would exceed the record budget. The marker makes the
// loss of diagnostic detail explicit; the caller's original error is untouched.
func truncateCompletionError(event *workflow.AuditEvent) ([]byte, error) {
	const marker = "… [truncated]"
	original := []rune(event.Error)
	event.Error = marker
	best, err := marshalJSONL(event)
	if err != nil {
		return nil, err
	}
	if len(best) > maxAuditRecordSize {
		return nil, fmt.Errorf("%w: terminal identity exceeds record budget", ErrAuditRecordTooLarge)
	}
	low, high := 0, len(original)
	for low <= high {
		mid := low + (high-low)/2
		event.Error = string(original[:mid]) + marker
		candidate, marshalErr := marshalJSONL(event)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if len(candidate) <= maxAuditRecordSize {
			best = candidate
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	return best, nil
}
