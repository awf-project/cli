package ports

import (
	"context"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

type EventPublisher interface {
	Publish(ctx context.Context, event *pluginmodel.DomainEvent) error
	Close() error
}
