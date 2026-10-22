package audit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awf-project/cli/internal/infrastructure/audit"
)

func TestNewWriterFromEnv_OffIsCaseSensitive(t *testing.T) {
	for _, value := range []string{"off", "OFF"} {
		t.Run(value, func(t *testing.T) {
			// Relative audit paths must stay inside the test's temporary directory.
			t.Chdir(t.TempDir())
			t.Setenv("AWF_AUDIT_LOG", value)

			writer, cleanup, err := audit.NewWriterFromEnv()
			if err != nil {
				t.Fatalf("create audit writer: %v", err)
			}
			defer cleanup()

			if value == "off" {
				if writer != nil {
					t.Error("expected nil writer when AWF_AUDIT_LOG=off")
				}
				if _, err := os.Stat(value); !os.IsNotExist(err) {
					t.Errorf("disabled audit must not create a file; stat error: %v", err)
				}
				return
			}

			if writer == nil {
				t.Fatal("expected non-nil writer when AWF_AUDIT_LOG=OFF")
			}
			if _, err := os.Stat(value); err != nil {
				t.Errorf("expected audit file named %q: %v", value, err)
			}
		})
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
