package transcript

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/awf-project/cli/internal/domain/transcript"
)

// JSONLWriter appends ExchangeEvent records to a JSONL file.
// Mutex is held on every write to guarantee atomicity beyond PIPE_BUF.
type JSONLWriter struct {
	path      string
	f         *os.File
	mu        sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// NewJSONLWriter opens or creates the JSONL transcript file at path.
// Parent directories are created with mode 0o700. File mode is 0o600.
func NewJSONLWriter(path string) (*JSONLWriter, error) {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating transcript directory: %w", err)
	}

	f, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // path is cleaned above; callers control the input
	if err != nil {
		return nil, fmt.Errorf("opening transcript file: %w", err)
	}

	return &JSONLWriter{path: cleanPath, f: f}, nil
}

// Write marshals event to JSON, appends a newline, and writes atomically under lock.
// Returns ctx.Err() immediately if the context is already done.
func (w *JSONLWriter) Write(ctx context.Context, event transcript.ExchangeEvent) error { //nolint:gocritic // hugeParam: value receiver required so json.Marshal(event) invokes the custom MarshalJSON on value receiver
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("transcript write canceled: %w", err)
	}

	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling exchange event: %w", err)
	}

	jsonBytes = append(jsonBytes, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.f.Write(jsonBytes); err != nil {
		return fmt.Errorf("writing exchange event: %w", err)
	}

	return nil
}

// Close closes the underlying file exactly once. Subsequent calls return nil.
func (w *JSONLWriter) Close() error {
	w.closeOnce.Do(func() {
		w.closeErr = w.f.Close()
	})
	return w.closeErr
}
