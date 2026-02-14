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
// attempt is 1-indexed (first retry is attempt 2). The result is capped at maxDelay.
func CalculateDelay(strategy Strategy, attempt int, initialDelay, maxDelay time.Duration, multiplier float64) time.Duration {
	var delay time.Duration

	switch strategy {
	case StrategyLinear:
		delay = time.Duration(int64(initialDelay) * int64(attempt))
	case StrategyExponential:
		factor := 1.0
		for i := 1; i < attempt; i++ {
			factor *= multiplier
		}
		delay = time.Duration(float64(initialDelay) * factor)
	case StrategyConstant, "":
		delay = initialDelay
	default:
		delay = initialDelay
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// ApplyJitter adds randomness to a delay within ±(delay * jitter).
// jitter should be in range [0.0, 1.0]. Uses the provided random source for deterministic testing.
func ApplyJitter(delay time.Duration, jitter float64, rng *rand.Rand) time.Duration {
	if jitter == 0 || delay == 0 {
		return delay
	}

	randomFactor := (rng.Float64() * 2) - 1
	jitterAmount := float64(delay) * jitter * randomFactor
	result := float64(delay) + jitterAmount

	if result < 0 {
		result = 0
	}

	return time.Duration(result)
}
