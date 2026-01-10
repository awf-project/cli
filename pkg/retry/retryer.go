package retry

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// Config holds retry behavior configuration.
type Config struct {
	MaxAttempts        int           // max retry attempts (1 = no retry)
	InitialDelay       time.Duration // base delay between attempts
	MaxDelay           time.Duration // maximum delay cap
	Strategy           Strategy      // backoff strategy
	Multiplier         float64       // multiplier for exponential backoff
	Jitter             float64       // randomness factor [0.0, 1.0]
	RetryableExitCodes []int         // exit codes that trigger retry (empty = any non-zero)
}

// Logger defines the logging interface for retry operations.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
}

// Retryer handles retry logic with configurable backoff.
type Retryer struct {
	config Config
	logger Logger
	rng    *rand.Rand
}

// NewRetryer creates a Retryer with the given configuration.
// If logger is nil, retry attempts are not logged.
// Uses a seeded random source for jitter; pass a fixed seed for deterministic tests.
func NewRetryer(config *Config, logger Logger, seed int64) *Retryer {
	return &Retryer{
		config: *config,
		logger: logger,
		rng:    rand.New(rand.NewSource(seed)),
	}
}

// ShouldRetry returns true if the operation should be retried.
// exitCode is the exit code from the failed operation.
// attempt is the current attempt number (1-indexed).
func (r *Retryer) ShouldRetry(exitCode, attempt int) bool {
	// Success never retries
	if exitCode == 0 {
		return false
	}

	// Check if we've exhausted attempts
	if attempt >= r.config.MaxAttempts {
		return false
	}

	// Check if exit code is retryable
	return r.IsRetryableExitCode(exitCode)
}

// NextDelay returns the delay to wait before the next attempt.
// attempt is the current attempt number (1-indexed).
func (r *Retryer) NextDelay(attempt int) time.Duration {
	delay := CalculateDelay(
		r.config.Strategy,
		attempt,
		r.config.InitialDelay,
		r.config.MaxDelay,
		r.config.Multiplier,
	)

	return ApplyJitter(delay, r.config.Jitter, r.rng)
}

// Wait sleeps for the calculated delay, respecting context cancellation.
// Returns an error if the context is cancelled during the wait.
func (r *Retryer) Wait(ctx context.Context, attempt int) error {
	delay := r.NextDelay(attempt)

	if r.logger != nil {
		r.logger.Debug("waiting before retry",
			"attempt", attempt,
			"delay", delay.String(),
		)
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("retry wait interrupted: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// IsRetryableExitCode checks if the given exit code should trigger a retry.
func (r *Retryer) IsRetryableExitCode(exitCode int) bool {
	// Success (exit code 0) never triggers retry
	if exitCode == 0 {
		return false
	}

	// If no specific codes are configured, retry any non-zero exit code
	if len(r.config.RetryableExitCodes) == 0 {
		return true
	}

	// Check if exit code is in the allowed list
	for _, code := range r.config.RetryableExitCodes {
		if code == exitCode {
			return true
		}
	}

	return false
}
