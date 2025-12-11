// Package retry provides configurable retry utilities with exponential backoff
// for operations that may fail transiently.
//
// Key features:
//   - Exponential and linear backoff strategies
//   - Configurable max attempts, delays, and jitter
//   - Context-aware cancellation
//   - Exit code filtering for selective retries
//
// Example usage:
//
//	cfg := retry.Config{
//	    MaxAttempts:  3,
//	    InitialDelay: time.Second,
//	    Strategy:     retry.StrategyExponential,
//	    Multiplier:   2.0,
//	}
//	retryer := retry.New(cfg, logger)
//	result, err := retryer.Execute(ctx, func() (int, error) {
//	    return runCommand()
//	})
package retry
