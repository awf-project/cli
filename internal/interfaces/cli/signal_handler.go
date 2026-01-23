package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler starts a goroutine that cancels ctx on SIGINT/SIGTERM.
// If onSignal is not nil, it's called when a signal is received before cancelling.
// Returns a cleanup function that MUST be deferred to prevent goroutine leaks.
func setupSignalHandler(ctx context.Context, cancel context.CancelFunc, onSignal func()) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			if onSignal != nil {
				onSignal()
			}
			cancel()
		case <-ctx.Done():
			// Context cancelled externally, exit cleanly
		}
	}()
	return func() { signal.Stop(sigChan) }
}
