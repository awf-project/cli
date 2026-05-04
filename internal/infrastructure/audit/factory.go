package audit

import (
	"os"
	"path/filepath"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
)

// NewWriterFromEnv creates a FileAuditTrailWriter based on the AWF_AUDIT_LOG env var.
// Returns (nil, noop, nil) when AWF_AUDIT_LOG=off.
// Returns a writer at the default XDG path when AWF_AUDIT_LOG is empty.
// The caller must defer the returned cleanup func.
func NewWriterFromEnv() (ports.AuditTrailWriter, func(), error) {
	noop := func() {}

	auditLog := os.Getenv("AWF_AUDIT_LOG")
	if auditLog == "off" {
		return nil, noop, nil
	}

	auditPath := auditLog
	if auditPath == "" {
		auditPath = filepath.Join(xdg.AWFDataDir(), "audit.jsonl")
	}

	w, err := NewFileAuditTrailWriter(auditPath)
	if err != nil {
		return nil, noop, err
	}

	cleanup := func() {
		_ = w.Close()
	}
	return w, cleanup, nil
}
