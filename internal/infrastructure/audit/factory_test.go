package audit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/audit"
)

func TestNewWriterFromEnv_Off(t *testing.T) {
	t.Setenv("AWF_AUDIT_LOG", "off")
	w, cleanup, err := audit.NewWriterFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if w != nil {
		t.Error("expected nil writer when AWF_AUDIT_LOG=off")
	}
}

func TestNewWriterFromEnv_CustomPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	t.Setenv("AWF_AUDIT_LOG", path)

	w, cleanup, err := audit.NewWriterFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if w == nil {
		t.Fatal("expected non-nil writer for custom path")
	}
}

func TestNewWriterFromEnv_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWF_AUDIT_LOG", "")
	t.Setenv("XDG_DATA_HOME", dir)

	w, cleanup, err := audit.NewWriterFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if w == nil {
		t.Fatal("expected non-nil writer for default path")
	}

	expectedPath := filepath.Join(dir, "awf", "audit.jsonl")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected audit file at %s: %v", expectedPath, err)
	}
}
