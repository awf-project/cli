package application

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
)

// MemoryMonitor provides memory usage monitoring and threshold alerting.
// C019: Tracks workflow memory consumption and logs warnings when thresholds exceeded.
type MemoryMonitor struct {
	config workflow.MemoryConfig
	logger ports.Logger
}

// NewMemoryMonitor creates a new memory monitor with the given configuration.
func NewMemoryMonitor(config workflow.MemoryConfig, logger ports.Logger) *MemoryMonitor {
	return &MemoryMonitor{
		config: config,
		logger: logger,
	}
}

// CheckThreshold checks current memory usage against configured threshold.
// Logs a warning if threshold is exceeded.
// Returns current heap allocation in bytes.
func (m *MemoryMonitor) CheckThreshold(ctx context.Context) (heapAlloc uint64, exceeded bool) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	heapAlloc = mem.HeapAlloc

	// Monitoring disabled or zero/negative threshold means unlimited
	if !m.config.Enabled || m.config.ThresholdBytes <= 0 {
		return heapAlloc, false
	}

	// Check if threshold exceeded (safe conversion: threshold is positive at this point)
	threshold := uint64(m.config.ThresholdBytes) // #nosec G115 - already checked for negative values
	exceeded = heapAlloc > threshold
	if exceeded {
		m.logger.Warn(fmt.Sprintf("Memory threshold exceeded: %d bytes (threshold: %d bytes)",
			heapAlloc, m.config.ThresholdBytes))
	}

	return heapAlloc, exceeded
}

// StartPeriodicMonitoring starts a goroutine that periodically checks memory usage.
// The goroutine stops when the context is canceled.
// Returns a channel that will be closed when monitoring stops.
func (m *MemoryMonitor) StartPeriodicMonitoring(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	// If monitoring is disabled, return immediately
	if !m.config.Enabled {
		close(done)
		return done
	}

	go func() {
		defer close(done)

		// Check memory every 100ms
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.CheckThreshold(ctx)
			}
		}
	}()

	return done
}

// LogMemoryStats logs current memory statistics at INFO level.
func (m *MemoryMonitor) LogMemoryStats(ctx context.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	m.logger.Info(fmt.Sprintf("Memory stats: HeapAlloc=%d bytes, HeapSys=%d bytes, HeapIdle=%d bytes, HeapInuse=%d bytes",
		mem.HeapAlloc, mem.HeapSys, mem.HeapIdle, mem.HeapInuse))
}
