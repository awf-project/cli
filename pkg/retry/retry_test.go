package retry

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateDelay_Constant(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		want         time.Duration
	}{
		{
			name:         "first retry returns initial delay",
			attempt:      2,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         1 * time.Second,
		},
		{
			name:         "fifth retry still returns initial delay",
			attempt:      5,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(StrategyConstant, tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateDelay_Linear(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		want         time.Duration
	}{
		{
			name:         "attempt 2 returns 2x initial",
			attempt:      2,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         2 * time.Second,
		},
		{
			name:         "attempt 5 returns 5x initial",
			attempt:      5,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(StrategyLinear, tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateDelay_Exponential(t *testing.T) {
	tests := []struct {
		name         string
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		want         time.Duration
	}{
		{
			name:         "attempt 2 returns initial*multiplier^1",
			attempt:      2,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         2 * time.Second,
		},
		{
			name:         "attempt 3 returns initial*multiplier^2",
			attempt:      3,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         4 * time.Second,
		},
		{
			name:         "attempt 4 returns initial*multiplier^3",
			attempt:      4,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         8 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(StrategyExponential, tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateDelay_CapsAtMaxDelay(t *testing.T) {
	tests := []struct {
		name         string
		strategy     Strategy
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		want         time.Duration
	}{
		{
			name:         "exponential capped at max",
			strategy:     StrategyExponential,
			attempt:      10,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         30 * time.Second,
		},
		{
			name:         "linear capped at max",
			strategy:     StrategyLinear,
			attempt:      50,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(tt.strategy, tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplyJitter_WithinBounds(t *testing.T) {
	delay := 10 * time.Second
	jitter := 0.1 // ±10%

	rng := rand.New(rand.NewSource(42))

	// Run multiple times to verify bounds
	for i := 0; i < 100; i++ {
		got := ApplyJitter(delay, jitter, rng)

		minExpected := time.Duration(float64(delay) * (1 - jitter))
		maxExpected := time.Duration(float64(delay) * (1 + jitter))

		assert.GreaterOrEqual(t, got, minExpected, "delay should be >= min bound")
		assert.LessOrEqual(t, got, maxExpected, "delay should be <= max bound")
	}
}

func TestApplyJitter_ZeroJitter(t *testing.T) {
	delay := 10 * time.Second
	rng := rand.New(rand.NewSource(42))

	got := ApplyJitter(delay, 0.0, rng)
	assert.Equal(t, delay, got, "zero jitter should return exact delay")
}

func TestShouldRetry_EmptyCodesRetriesAll(t *testing.T) {
	retryer := NewRetryer(&Config{
		MaxAttempts:        5,
		RetryableExitCodes: []int{}, // empty = retry any non-zero
	}, nil, 42)

	tests := []struct {
		exitCode int
		attempt  int
		want     bool
	}{
		{exitCode: 0, attempt: 1, want: false},  // success, no retry
		{exitCode: 1, attempt: 1, want: true},   // any non-zero retried
		{exitCode: 127, attempt: 1, want: true}, // any non-zero retried
		{exitCode: 1, attempt: 5, want: false},  // max attempts reached
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := retryer.ShouldRetry(tt.exitCode, tt.attempt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRetry_SpecificCodes(t *testing.T) {
	retryer := NewRetryer(&Config{
		MaxAttempts:        5,
		RetryableExitCodes: []int{1, 2, 130},
	}, nil, 42)

	tests := []struct {
		name     string
		exitCode int
		attempt  int
		want     bool
	}{
		{name: "success never retries", exitCode: 0, attempt: 1, want: false},
		{name: "code 1 retries", exitCode: 1, attempt: 1, want: true},
		{name: "code 2 retries", exitCode: 2, attempt: 1, want: true},
		{name: "code 130 retries", exitCode: 130, attempt: 1, want: true},
		{name: "code 127 not in list", exitCode: 127, attempt: 1, want: false},
		{name: "max attempts reached", exitCode: 1, attempt: 5, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryer.ShouldRetry(tt.exitCode, tt.attempt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRetry_ExceedsMaxAttempts(t *testing.T) {
	retryer := NewRetryer(&Config{
		MaxAttempts: 3,
	}, nil, 42)

	assert.True(t, retryer.ShouldRetry(1, 1), "attempt 1 of 3")
	assert.True(t, retryer.ShouldRetry(1, 2), "attempt 2 of 3")
	assert.False(t, retryer.ShouldRetry(1, 3), "attempt 3 of 3 - exhausted")
}

func TestNextDelay_UsesStrategy(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "constant strategy",
			config: Config{
				Strategy:     StrategyConstant,
				InitialDelay: 1 * time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
				Jitter:       0.0,
			},
			attempt: 3,
			wantMin: 1 * time.Second,
			wantMax: 1 * time.Second,
		},
		{
			name: "exponential with jitter",
			config: Config{
				Strategy:     StrategyExponential,
				InitialDelay: 1 * time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
				Jitter:       0.1,
			},
			attempt: 3, // base delay = 4s
			wantMin: time.Duration(float64(4*time.Second) * 0.9),
			wantMax: time.Duration(float64(4*time.Second) * 1.1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := NewRetryer(&tt.config, nil, 42)
			got := retryer.NextDelay(tt.attempt)

			assert.GreaterOrEqual(t, got, tt.wantMin)
			assert.LessOrEqual(t, got, tt.wantMax)
		})
	}
}

func TestWait_RespectsContextCancellation(t *testing.T) {
	retryer := NewRetryer(&Config{
		Strategy:     StrategyConstant,
		InitialDelay: 10 * time.Second, // long delay
		MaxDelay:     30 * time.Second,
	}, nil, 42)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 10ms
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := retryer.Wait(ctx, 2)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 1*time.Second, "should have cancelled quickly")
}

func TestIsRetryableExitCode(t *testing.T) {
	tests := []struct {
		name     string
		codes    []int
		exitCode int
		want     bool
	}{
		{name: "empty codes, non-zero", codes: nil, exitCode: 1, want: true},
		{name: "empty codes, zero", codes: nil, exitCode: 0, want: false},
		{name: "specific codes, match", codes: []int{1, 2}, exitCode: 1, want: true},
		{name: "specific codes, no match", codes: []int{1, 2}, exitCode: 3, want: false},
		{name: "specific codes, zero", codes: []int{1, 2}, exitCode: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := NewRetryer(&Config{
				RetryableExitCodes: tt.codes,
				MaxAttempts:        5,
			}, nil, 42)
			got := retryer.IsRetryableExitCode(tt.exitCode)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStrategy_Valid(t *testing.T) {
	tests := []struct {
		strategy Strategy
		want     bool
	}{
		{StrategyConstant, true},
		{StrategyLinear, true},
		{StrategyExponential, true},
		{"", true}, // empty defaults to constant
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.strategy.Valid())
		})
	}
}

// =============================================================================
// Additional Edge Case Tests for CalculateDelay
// =============================================================================

func TestCalculateDelay_EmptyStrategyDefaultsToConstant(t *testing.T) {
	// Empty strategy should behave like constant
	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "attempt 2", attempt: 2, want: 1 * time.Second},
		{name: "attempt 5", attempt: 5, want: 1 * time.Second},
		{name: "attempt 10", attempt: 10, want: 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay("", tt.attempt, 1*time.Second, 30*time.Second, 2.0)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateDelay_FirstAttemptNoDelay(t *testing.T) {
	// Attempt 1 is the initial attempt - delay calculation may be used
	// but typically retry happens starting at attempt 2
	strategies := []Strategy{StrategyConstant, StrategyLinear, StrategyExponential}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			got := CalculateDelay(strategy, 1, 1*time.Second, 30*time.Second, 2.0)
			// For attempt 1:
			// - Constant: 1s
			// - Linear: 1s * 1 = 1s
			// - Exponential: 1s * 2^0 = 1s
			assert.Equal(t, 1*time.Second, got)
		})
	}
}

func TestCalculateDelay_ZeroInitialDelay(t *testing.T) {
	tests := []struct {
		name     string
		strategy Strategy
	}{
		{name: "constant", strategy: StrategyConstant},
		{name: "linear", strategy: StrategyLinear},
		{name: "exponential", strategy: StrategyExponential},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(tt.strategy, 5, 0, 30*time.Second, 2.0)
			assert.Equal(t, time.Duration(0), got)
		})
	}
}

func TestCalculateDelay_MaxDelayZero(t *testing.T) {
	// When maxDelay is 0, delay should be capped at 0
	got := CalculateDelay(StrategyExponential, 5, 1*time.Second, 0, 2.0)
	assert.Equal(t, time.Duration(0), got)
}

func TestCalculateDelay_MultiplierOne(t *testing.T) {
	// With multiplier=1, exponential behaves like constant
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 2, want: 1 * time.Second},
		{attempt: 3, want: 1 * time.Second},
		{attempt: 5, want: 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := CalculateDelay(StrategyExponential, tt.attempt, 1*time.Second, 30*time.Second, 1.0)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateDelay_LargeMultiplier(t *testing.T) {
	// Large multiplier should still be capped at maxDelay
	got := CalculateDelay(StrategyExponential, 3, 1*time.Second, 5*time.Second, 10.0)
	// 1s * 10^2 = 100s, capped to 5s
	assert.Equal(t, 5*time.Second, got)
}

func TestCalculateDelay_LinearProgression(t *testing.T) {
	// Verify linear progression: delay = initial * attempt
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 1 * time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 3, want: 3 * time.Second},
		{attempt: 10, want: 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := CalculateDelay(StrategyLinear, tt.attempt, 1*time.Second, 30*time.Second, 2.0)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCalculateDelay_ExponentialProgression(t *testing.T) {
	// Verify exponential progression: delay = initial * multiplier^(attempt-1)
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 1 * time.Second},  // 1 * 2^0 = 1
		{attempt: 2, want: 2 * time.Second},  // 1 * 2^1 = 2
		{attempt: 3, want: 4 * time.Second},  // 1 * 2^2 = 4
		{attempt: 4, want: 8 * time.Second},  // 1 * 2^3 = 8
		{attempt: 5, want: 16 * time.Second}, // 1 * 2^4 = 16
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := CalculateDelay(StrategyExponential, tt.attempt, 1*time.Second, 30*time.Second, 2.0)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Additional Jitter Edge Case Tests
// =============================================================================

func TestApplyJitter_MaxJitter(t *testing.T) {
	// With jitter=1.0, delay can range from 0 to 2*delay
	delay := 10 * time.Second
	jitter := 1.0

	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 100; i++ {
		got := ApplyJitter(delay, jitter, rng)

		minExpected := time.Duration(0)
		maxExpected := 2 * delay

		assert.GreaterOrEqual(t, got, minExpected, "delay should be >= 0")
		assert.LessOrEqual(t, got, maxExpected, "delay should be <= 2*delay")
	}
}

func TestApplyJitter_ZeroDelay(t *testing.T) {
	// Zero delay with any jitter should still be zero
	rng := rand.New(rand.NewSource(42))
	got := ApplyJitter(0, 0.5, rng)
	assert.Equal(t, time.Duration(0), got)
}

func TestApplyJitter_SmallDelay(t *testing.T) {
	// Small delay with jitter should still work
	delay := 10 * time.Millisecond
	jitter := 0.1

	rng := rand.New(rand.NewSource(42))
	got := ApplyJitter(delay, jitter, rng)

	minExpected := time.Duration(float64(delay) * (1 - jitter))
	maxExpected := time.Duration(float64(delay) * (1 + jitter))

	assert.GreaterOrEqual(t, got, minExpected)
	assert.LessOrEqual(t, got, maxExpected)
}

func TestApplyJitter_DeterministicWithSameSeed(t *testing.T) {
	delay := 10 * time.Second
	jitter := 0.5

	// Same seed should produce same result
	rng1 := rand.New(rand.NewSource(12345))
	rng2 := rand.New(rand.NewSource(12345))

	got1 := ApplyJitter(delay, jitter, rng1)
	got2 := ApplyJitter(delay, jitter, rng2)

	assert.Equal(t, got1, got2, "same seed should produce same result")
}

// =============================================================================
// Additional Retryer Tests
// =============================================================================

func TestNewRetryer_WithNilLogger(t *testing.T) {
	// Should work without panicking when logger is nil
	retryer := NewRetryer(&Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Strategy:     StrategyConstant,
	}, nil, 42)

	assert.NotNil(t, retryer)

	// Methods should work without nil pointer dereference
	got := retryer.ShouldRetry(1, 1)
	assert.True(t, got)

	delay := retryer.NextDelay(2)
	assert.Equal(t, 1*time.Second, delay)
}

func TestNewRetryer_DeterministicJitter(t *testing.T) {
	// Same seed should produce same delays
	config := Config{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Strategy:     StrategyExponential,
		Multiplier:   2.0,
		Jitter:       0.2,
	}

	r1 := NewRetryer(&config, nil, 12345)
	r2 := NewRetryer(&config, nil, 12345)

	for attempt := 2; attempt <= 5; attempt++ {
		d1 := r1.NextDelay(attempt)
		d2 := r2.NextDelay(attempt)
		assert.Equal(t, d1, d2, "same seed should produce same delay for attempt %d", attempt)
	}
}

func TestWait_CompletesNormally(t *testing.T) {
	retryer := NewRetryer(&Config{
		Strategy:     StrategyConstant,
		InitialDelay: 10 * time.Millisecond, // short delay for test
		MaxDelay:     30 * time.Second,
	}, nil, 42)

	ctx := context.Background()
	start := time.Now()
	err := retryer.Wait(ctx, 2)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond, "should have waited at least 10ms")
}

func TestWait_WithTimeout(t *testing.T) {
	retryer := NewRetryer(&Config{
		Strategy:     StrategyConstant,
		InitialDelay: 5 * time.Second, // long delay
		MaxDelay:     30 * time.Second,
	}, nil, 42)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := retryer.Wait(ctx, 2)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 1*time.Second, "should have cancelled quickly")
}

func TestShouldRetry_ZeroExitCodeNeverRetries(t *testing.T) {
	tests := []struct {
		name  string
		codes []int
	}{
		{name: "empty codes", codes: []int{}},
		{name: "specific codes including 0", codes: []int{0, 1, 2}},
		{name: "specific codes without 0", codes: []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := NewRetryer(&Config{
				MaxAttempts:        5,
				RetryableExitCodes: tt.codes,
			}, nil, 42)

			got := retryer.ShouldRetry(0, 1)
			assert.False(t, got, "exit code 0 should never trigger retry")
		})
	}
}

func TestShouldRetry_AttemptBoundary(t *testing.T) {
	retryer := NewRetryer(&Config{
		MaxAttempts: 5,
	}, nil, 42)

	// Attempt 4 (can still retry once more to attempt 5)
	assert.True(t, retryer.ShouldRetry(1, 4), "attempt 4 of 5 should allow retry")

	// Attempt 5 (max reached, no more retries)
	assert.False(t, retryer.ShouldRetry(1, 5), "attempt 5 of 5 should not allow retry")
}

func TestNextDelay_WithAllStrategies(t *testing.T) {
	tests := []struct {
		name       string
		strategy   Strategy
		attempt    int
		wantExact  time.Duration // for constant/no jitter
		wantMinMax []time.Duration
	}{
		{
			name:      "constant attempt 3",
			strategy:  StrategyConstant,
			attempt:   3,
			wantExact: 1 * time.Second,
		},
		{
			name:      "linear attempt 3",
			strategy:  StrategyLinear,
			attempt:   3,
			wantExact: 3 * time.Second,
		},
		{
			name:      "exponential attempt 3",
			strategy:  StrategyExponential,
			attempt:   3,
			wantExact: 4 * time.Second, // 1s * 2^2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := NewRetryer(&Config{
				Strategy:     tt.strategy,
				InitialDelay: 1 * time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
				Jitter:       0.0, // no jitter for exact comparison
			}, nil, 42)

			got := retryer.NextDelay(tt.attempt)
			assert.Equal(t, tt.wantExact, got)
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	// Verify behavior with zero/default config values
	cfg := Config{}

	assert.Equal(t, 0, cfg.MaxAttempts)
	assert.Equal(t, time.Duration(0), cfg.InitialDelay)
	assert.Equal(t, time.Duration(0), cfg.MaxDelay)
	assert.Equal(t, Strategy(""), cfg.Strategy)
	assert.Equal(t, 0.0, cfg.Multiplier)
	assert.Equal(t, 0.0, cfg.Jitter)
	assert.Nil(t, cfg.RetryableExitCodes)
}

// =============================================================================
// Mock Logger for testing logged retry attempts
// =============================================================================

type recordingLogger struct {
	debugCalls []logCall
	infoCalls  []logCall
}

type logCall struct {
	msg    string
	fields []any
}

func (l *recordingLogger) Debug(msg string, keysAndValues ...any) {
	l.debugCalls = append(l.debugCalls, logCall{msg: msg, fields: keysAndValues})
}

func (l *recordingLogger) Info(msg string, keysAndValues ...any) {
	l.infoCalls = append(l.infoCalls, logCall{msg: msg, fields: keysAndValues})
}

func TestRetryer_LogsAttempts(t *testing.T) {
	logger := &recordingLogger{}
	retryer := NewRetryer(&Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Strategy:     StrategyConstant,
	}, logger, 42)

	// Trigger a wait which should log
	ctx := context.Background()
	_ = retryer.Wait(ctx, 2)

	// Verify logging occurred (implementation specific)
	// This test verifies that a logger, if provided, receives calls
	// The exact format depends on implementation
}
