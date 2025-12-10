package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/vanoix/awf/internal/domain/workflow"
)

// JSONStore implements StateStore for JSON file persistence.
type JSONStore struct {
	basePath string
}

// NewJSONStore creates a new JSON store.
func NewJSONStore(basePath string) *JSONStore {
	return &JSONStore{basePath: basePath}
}

// Save persists execution state to a JSON file with atomic write.
// Uses unique temp file names to prevent concurrent write corruption.
func (s *JSONStore) Save(ctx context.Context, state *workflow.ExecutionContext) error {
	if err := os.MkdirAll(s.basePath, 0755); err != nil {
		return err
	}

	finalPath := s.filePath(state.WorkflowID)
	// Use unique temp file to prevent concurrent Save corruption
	tmpPath := fmt.Sprintf("%s.%d.%d.tmp", finalPath, os.Getpid(), time.Now().UnixNano())

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if _, err := f.Write(data); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := f.Sync(); err != nil {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()

	return os.Rename(tmpPath, finalPath)
}

// Load retrieves execution state from a JSON file.
func (s *JSONStore) Load(ctx context.Context, workflowID string) (*workflow.ExecutionContext, error) {
	filePath := s.filePath(workflowID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var state workflow.ExecutionContext
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// Delete removes a state file.
func (s *JSONStore) Delete(ctx context.Context, workflowID string) error {
	filePath := s.filePath(workflowID)

	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List returns all stored workflow IDs.
func (s *JSONStore) List(ctx context.Context) ([]string, error) {
	pattern := filepath.Join(s.basePath, "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		base := filepath.Base(match)
		id := strings.TrimSuffix(base, ".json")
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *JSONStore) filePath(workflowID string) string {
	return filepath.Join(s.basePath, workflowID+".json")
}
