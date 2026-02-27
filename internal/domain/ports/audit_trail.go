package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// AuditTrailWriter defines the contract for appending audit trail entries.
type AuditTrailWriter interface {
	Write(ctx context.Context, event *workflow.AuditEvent) error
	Close() error
}
