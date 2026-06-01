package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/awf-project/cli/internal/domain/workflow"

	domerrors "github.com/awf-project/cli/internal/domain/errors"
)

// assertTraversalBlocked verifies that a traversal attempt produced a structured
// "not found" / "invalid" error and never returned data from outside basePath.
func assertTraversalBlocked(t *testing.T, inputName string, wf *workflow.Workflow, err error) {
	t.Helper()
	if err == nil {
		if wf != nil {
			t.Fatalf("Load(%q) succeeded with wf.Name=%q — traversal was NOT blocked", inputName, wf.Name)
		}
		t.Fatalf("Load(%q) = nil error, nil wf — expected an error (traversal must be blocked)", inputName)
	}
	var structErr *domerrors.StructuredError
	if !errors.As(err, &structErr) {
		t.Fatalf("Load(%q) error type = %T (%v), want *domerrors.StructuredError", inputName, err, err)
	}
	validCodes := []domerrors.ErrorCode{
		domerrors.ErrorCodeUserInputMissingFile,
		domerrors.ErrorCodeUserInputValidationFailed,
	}
	if !slices.Contains(validCodes, structErr.Code) {
		t.Errorf("Load(%q) code = %v, want one of %v (path traversal blocked incorrectly)",
			inputName, structErr.Code, validCodes)
	}
}

// assertLegitimateName verifies that a safe name is not rejected by an
// over-aggressive traversal guard.
func assertLegitimateName(t *testing.T, inputName string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	var structErr *domerrors.StructuredError
	if !errors.As(err, &structErr) {
		t.Fatalf("Load(%q) error type = %T (%v), want *domerrors.StructuredError or nil", inputName, err, err)
	}
	if structErr.Code == domerrors.ErrorCodeUserInputValidationFailed {
		t.Errorf("Load(%q) returned VALIDATION_FAILED for a legitimate name — fix is too aggressive", inputName)
	}
}

// TestYAMLRepository_resolvePath_PathTraversal verifies that Load rejects names
// that would escape basePath via directory traversal sequences.
//
// TDD approach: this test suite was written before the fix. The sub-tests that
// cover absolute paths and pack-level traversal would FAIL against the original
// implementation because filepath.Join(absBase, "/etc/passwd") returns
// "/etc/passwd.yaml" (absolute path takes over), and
// filepath.Join(absBase, "../../secret") resolves to a sibling directory.
func TestYAMLRepository_resolvePath_PathTraversal(t *testing.T) {
	// Use a temporary directory so we can place a sentinel file *outside* base
	// and confirm that a traversal attempt cannot read it.
	tmpRoot := t.TempDir() // e.g. /tmp/TestXxx1234
	baseDir := filepath.Join(tmpRoot, "workflows")
	secretDir := filepath.Join(tmpRoot, "secret")

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll baseDir: %v", err)
	}
	if err := os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatalf("MkdirAll secretDir: %v", err)
	}

	// Place a valid workflow inside base.
	validWF := `name: legit
description: "ok"
initial: done
states:
  initial: done
  done:
    type: terminal
    status: success
    message: ok
`
	if err := os.WriteFile(filepath.Join(baseDir, "legit.yaml"), []byte(validWF), 0o644); err != nil {
		t.Fatalf("WriteFile legit.yaml: %v", err)
	}

	// Place a "secret" YAML outside base that traversal could reach.
	secretWF := `name: secret-file
initial: leak
states:
  initial: leak
  leak:
    type: terminal
    status: success
    message: leaked
`
	// ../secret/stolen is reachable from baseDir via "../secret/stolen"
	if err := os.WriteFile(filepath.Join(secretDir, "stolen.yaml"), []byte(secretWF), 0o644); err != nil {
		t.Fatalf("WriteFile stolen.yaml: %v", err)
	}

	repo := NewYAMLRepository(baseDir)

	tests := []struct {
		name      string
		inputName string
		// wantTraversalBlocked == true: must receive an error (path rejected);
		// must NOT successfully load a workflow from outside baseDir.
		wantTraversalBlocked bool
	}{
		// --- Traversal attempts that MUST be blocked ---
		{
			name:                 "dotdot escapes base (../secret/stolen)",
			inputName:            "../secret/stolen",
			wantTraversalBlocked: true,
		},
		{
			name:                 "double dotdot escapes root",
			inputName:            "../../etc/passwd",
			wantTraversalBlocked: true,
		},
		{
			name:                 "pack-style traversal pack/../../secret/stolen",
			inputName:            "pack/../../secret/stolen",
			wantTraversalBlocked: true,
		},
		{
			name:                 "absolute path bypasses base",
			inputName:            "/etc/passwd",
			wantTraversalBlocked: true,
		},
		{
			name:                 "absolute path to secret file",
			inputName:            filepath.Join(secretDir, "stolen"),
			wantTraversalBlocked: true,
		},
		// --- Legitimate names that MUST NOT be rejected ---
		{
			name:                 "plain workflow name inside base",
			inputName:            "legit",
			wantTraversalBlocked: false,
		},
		{
			name:                 "pack-style name stays inside base",
			inputName:            "speckit/specify",
			wantTraversalBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf, err := repo.Load(context.Background(), tt.inputName)
			if tt.wantTraversalBlocked {
				assertTraversalBlocked(t, tt.inputName, wf, err)
			} else {
				assertLegitimateName(t, tt.inputName, err)
			}
		})
	}
}
