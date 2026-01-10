// Package retry provides configurable retry logic with backoff strategies.
package retry

import (
	"math/rand"
	"time"
)

// Strategy defines the backoff algorithm to use between retry attempts.
type Strategy string

const (
	// StrategyConstant always waits the initial delay between attempts.
	StrategyConstant Strategy = "constant"
	// StrategyLinear increases delay linearly: initialDelay * attempt.
	StrategyLinear Strategy = "linear"
	// StrategyExponential increases delay exponentially: initialDelay * multiplier^(attempt-1).
	StrategyExponential Strategy = "exponential"
)

// Valid returns true if the strategy is recognized.
func (s Strategy) Valid() bool {
	switch s {
	case StrategyConstant, StrategyLinear, StrategyExponential, "":
		return true
	default:
		return false
	}
}

// CalculateDelay computes the delay for a given attempt using the specified strategy.
// attempt is 1-indexed (first retry is attempt 2).
// The result is capped at maxDelay.
func CalculateDelay(strategy Strategy, attempt int, initialDelay, maxDelay time.Duration, multiplier float64) time.Duration {
	var delay time.Duration

	switch strategy {
	case StrategyLinear:
		// Linear backoff formula: initialDelay * attempt
		delay = time.Duration(int64(initialDelay) * int64(attempt))
	case StrategyExponential:
		// Exponential backoff formula: initialDelay * multiplier^(attempt-1)
		factor := 1.0
		for i := 1; i < attempt; i++ {
			factor *= multiplier
		}
		delay = time.Duration(float64(initialDelay) * factor)
	case StrategyConstant, "":
		// constant: always return initialDelay
		delay = initialDelay
	default:
		// fallback to constant for unknown strategies
		delay = initialDelay
	}

	// Cap at maxDelay (if maxDelay is 0, it means cap at 0)
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// ApplyJitter adds randomness to a delay within ±(delay * jitter).
// jitter should be in range [0.0, 1.0].
// Uses the provided random source for deterministic testing.
func ApplyJitter(delay time.Duration, jitter float64, rng *rand.Rand) time.Duration {
	if jitter == 0 || delay == 0 {
		return delay
	}

	// Generate random value in [-1, 1]
	// rng.Float64() returns [0.0, 1.0), scale to [-1.0, 1.0)
	randomFactor := (rng.Float64() * 2) - 1

	// Apply jitter: delay ± (delay * jitter * random)
	jitterAmount := float64(delay) * jitter * randomFactor
	result := float64(delay) + jitterAmount

	// Ensure non-negative
	if result < 0 {
		result = 0
	}

	return time.Duration(result)
}
