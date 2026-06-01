package acp

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
)

// fanoutPublishTimeout is the per-target deadline applied to each Publish call in
// FanoutPublisher. A slow or stuck target cannot block the fan-out for longer than
// this duration; on timeout the failure is logged as a warning and delivery to other
// targets continues unaffected (M-6 fix).
const fanoutPublishTimeout = 5 * time.Second

// FanoutPublisher fans out events to multiple EventPublisher targets sequentially.
// Errors from individual targets are logged but not propagated (best-effort semantics).
type FanoutPublisher struct {
	targets []ports.EventPublisher
	logger  ports.Logger
}

var _ ports.EventPublisher = (*FanoutPublisher)(nil)

// NewFanoutPublisher creates a fan-out wrapper over the given targets.
// Nil targets are filtered out defensively.
func NewFanoutPublisher(logger ports.Logger, targets ...ports.EventPublisher) *FanoutPublisher {
	filtered := make([]ports.EventPublisher, 0, len(targets))
	for _, t := range targets {
		if t != nil {
			filtered = append(filtered, t)
		}
	}
	return &FanoutPublisher{
		targets: filtered,
		logger:  logger,
	}
}

// Publish sends the event to all targets sequentially. Errors from individual targets
// are logged as warnings but not propagated (best-effort delivery). Each target call is
// bounded by fanoutPublishTimeout via context.WithTimeout so a slow or hung target cannot
// block delivery to the remaining targets indefinitely. The parent ctx is respected for
// cancellation (e.g. server shutdown); the timeout adds a per-target upper bound.
//
// Each iteration runs inside an anonymous closure so that defer cancel() is guaranteed
// to execute even if target.Publish panics — preventing timer resource leaks (M-1 fix).
//
// Sequential fan-out is sufficient for the typical 2–3 target production configuration
// and avoids spawning an unbounded number of goroutines per event (issue #3 fix).
func (p *FanoutPublisher) Publish(ctx context.Context, event *pluginmodel.DomainEvent) error {
	if event == nil {
		p.logger.Warn("acp fanout: nil event dropped")
		return nil
	}
	for i, target := range p.targets {
		func() {
			tctx, cancel := context.WithTimeout(ctx, fanoutPublishTimeout)
			defer cancel() // panic-safe: defer guarantees release even if target.Publish panics
			if err := target.Publish(tctx, event); err != nil {
				p.logger.Warn("fanout target publish failed", "index", i, "event_type", event.Type, "err", err.Error())
			}
		}()
	}
	return nil
}

// Close aggregates errors from all targets that support closing.
func (p *FanoutPublisher) Close() error {
	var errs []error
	for _, target := range p.targets {
		if c, ok := target.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
