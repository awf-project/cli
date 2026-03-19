package retry

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateDelay(t *testing.T) {
	tests := []struct {
		name         string
		strategy     Strategy
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		want         time.Duration
	}{
		// Constant strategy
		{name: "constant attempt 2", strategy: StrategyConstant, attempt: 2, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		{name: "constant attempt 5", strategy: StrategyConstant, attempt: 5, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		// Linear strategy
		{name: "linear attempt 2", strategy: StrategyLinear, attempt: 2, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 2 * time.Second},
		{name: "linear attempt 5", strategy: StrategyLinear, attempt: 5, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 5 * time.Second},
		{name: "linear attempt 1", strategy: StrategyLinear, attempt: 1, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		{name: "linear attempt 3", strategy: StrategyLinear, attempt: 3, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 3 * time.Second},
		{name: "linear attempt 10", strategy: StrategyLinear, attempt: 10, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 10 * time.Second},
		// Exponential strategy
		{name: "exponential attempt 2", strategy: StrategyExponential, attempt: 2, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 2 * time.Second},
		{name: "exponential attempt 3", strategy: StrategyExponential, attempt: 3, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 4 * time.Second},
		{name: "exponential attempt 4", strategy: StrategyExponential, attempt: 4, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 8 * time.Second},
		{name: "exponential attempt 5", strategy: StrategyExponential, attempt: 5, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 16 * time.Second},
		{name: "exponential attempt 1", strategy: StrategyExponential, attempt: 1, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		{name: "exponential multiplier=1 attempt 2", strategy: StrategyExponential, attempt: 2, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 1.0, want: 1 * time.Second},
		{name: "exponential multiplier=1 attempt 3", strategy: StrategyExponential, attempt: 3, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 1.0, want: 1 * time.Second},
		{name: "exponential multiplier=1 attempt 5", strategy: StrategyExponential, attempt: 5, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 1.0, want: 1 * time.Second},
		{name: "exponential large multiplier capped", strategy: StrategyExponential, attempt: 3, initialDelay: 1 * time.Second, maxDelay: 5 * time.Second, multiplier: 10.0, want: 5 * time.Second},
		// Max delay capping
		{name: "exponential capped at max", strategy: StrategyExponential, attempt: 10, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 30 * time.Second},
		{name: "linear capped at max", strategy: StrategyLinear, attempt: 50, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 30 * time.Second},
		// Empty strategy defaults to constant
		{name: "empty strategy attempt 2", strategy: "", attempt: 2, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		{name: "empty strategy attempt 5", strategy: "", attempt: 5, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		{name: "empty strategy attempt 10", strategy: "", attempt: 10, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
		// Edge cases
		{name: "constant zero initial delay", strategy: StrategyConstant, attempt: 5, initialDelay: 0, maxDelay: 30 * time.Second, multiplier: 2.0, want: 0},
		{name: "linear zero initial delay", strategy: StrategyLinear, attempt: 5, initialDelay: 0, maxDelay: 30 * time.Second, multiplier: 2.0, want: 0},
		{name: "exponential zero initial delay", strategy: StrategyExponential, attempt: 5, initialDelay: 0, maxDelay: 30 * time.Second, multiplier: 2.0, want: 0},
		{name: "exponential max delay zero", strategy: StrategyExponential, attempt: 5, initialDelay: 1 * time.Second, maxDelay: 0, multiplier: 2.0, want: 16 * time.Second},
		{name: "constant attempt 1", strategy: StrategyConstant, attempt: 1, initialDelay: 1 * time.Second, maxDelay: 30 * time.Second, multiplier: 2.0, want: 1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(tt.strategy, tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplyJitter(t *testing.T) {
	t.Run("zero jitter returns exact delay", func(t *testing.T) {
		rng := rand.New(rand.NewSource(42))
		got := ApplyJitter(10*time.Second, 0.0, rng)
		assert.Equal(t, 10*time.Second, got)
	})

	t.Run("zero delay with jitter returns zero", func(t *testing.T) {
		rng := rand.New(rand.NewSource(42))
		got := ApplyJitter(0, 0.5, rng)
		assert.Equal(t, time.Duration(0), got)
	})

	t.Run("small delay with jitter within bounds", func(t *testing.T) {
		delay := 10 * time.Millisecond
		jitter := 0.1
		rng := rand.New(rand.NewSource(42))
		got := ApplyJitter(delay, jitter, rng)
		minExpected := time.Duration(float64(delay) * (1 - jitter))
		maxExpected := time.Duration(float64(delay) * (1 + jitter))
		assert.GreaterOrEqual(t, got, minExpected)
		assert.LessOrEqual(t, got, maxExpected)
	})

	t.Run("within bounds over multiple iterations", func(t *testing.T) {
		delay := 10 * time.Second
		jitter := 0.1
		rng := rand.New(rand.NewSource(42))

		for i := 0; i < 100; i++ {
			got := ApplyJitter(delay, jitter, rng)
			minExpected := time.Duration(float64(delay) * (1 - jitter))
			maxExpected := time.Duration(float64(delay) * (1 + jitter))
			assert.GreaterOrEqual(t, got, minExpected)
			assert.LessOrEqual(t, got, maxExpected)
		}
	})

	t.Run("max jitter within bounds", func(t *testing.T) {
		delay := 10 * time.Second
		jitter := 1.0
		rng := rand.New(rand.NewSource(42))

		for i := 0; i < 100; i++ {
			got := ApplyJitter(delay, jitter, rng)
			assert.GreaterOrEqual(t, got, time.Duration(0))
			assert.LessOrEqual(t, got, 2*delay)
		}
	})

	t.Run("deterministic with same seed", func(t *testing.T) {
		delay := 10 * time.Second
		jitter := 0.5

		rng1 := rand.New(rand.NewSource(12345))
		rng2 := rand.New(rand.NewSource(12345))

		got1 := ApplyJitter(delay, jitter, rng1)
		got2 := ApplyJitter(delay, jitter, rng2)

		assert.Equal(t, got1, got2)
	})
}

// TestCalculateDelay_MaxDelayGuard verifies the maxDelay > 0 guard prevents
// silently capping delays to zero when maxDelay is omitted (found = 0).
func TestCalculateDelay_MaxDelayGuard(t *testing.T) {
	tests := []struct {
		name         string
		strategy     Strategy
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
		want         time.Duration
	}{
		// When maxDelay=0, delay should NOT be capped to zero.
		// This tests the guard: if maxDelay > 0 && delay > maxDelay
		{
			name:         "exponential with zero max_delay should return computed delay",
			strategy:     StrategyExponential,
			attempt:      3,
			initialDelay: 1 * time.Second,
			maxDelay:     0, // No cap
			multiplier:   2.0,
			want:         4 * time.Second, // 1s * 2^(3-1) = 4s, NOT capped to 0
		},
		{
			name:         "linear with zero max_delay should return computed delay",
			strategy:     StrategyLinear,
			attempt:      5,
			initialDelay: 1 * time.Second,
			maxDelay:     0, // No cap
			multiplier:   2.0,
			want:         5 * time.Second, // 1s * 5 = 5s, NOT capped to 0
		},
		{
			name:         "constant with zero max_delay returns initial delay",
			strategy:     StrategyConstant,
			attempt:      10,
			initialDelay: 2 * time.Second,
			maxDelay:     0, // No cap
			multiplier:   1.0,
			want:         2 * time.Second, // constant = initial, NOT capped to 0
		},
		// When maxDelay > 0, delays SHOULD still be capped normally.
		{
			name:         "exponential capped when max_delay is set and delay exceeds it",
			strategy:     StrategyExponential,
			attempt:      6,
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			multiplier:   2.0,
			want:         30 * time.Second, // 1s * 2^5 = 32s → capped to 30s
		},
		{
			name:         "linear capped when max_delay is set",
			strategy:     StrategyLinear,
			attempt:      100,
			initialDelay: 1 * time.Second,
			maxDelay:     60 * time.Second,
			multiplier:   1.0,
			want:         60 * time.Second, // 1s * 100 = 100s → capped to 60s
		},
		// Edge case: very small but positive maxDelay still caps.
		{
			name:         "exponential capped to 1ms when max_delay=1ms",
			strategy:     StrategyExponential,
			attempt:      5,
			initialDelay: 100 * time.Millisecond,
			maxDelay:     1 * time.Millisecond,
			multiplier:   2.0,
			want:         1 * time.Millisecond, // computed 6.4s → capped to 1ms
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDelay(tt.strategy, tt.attempt, tt.initialDelay, tt.maxDelay, tt.multiplier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		exitCode int
		attempt  int
		want     bool
	}{
		// Empty codes - retries all non-zero
		{name: "empty codes success never retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{}}, exitCode: 0, attempt: 1, want: false},
		{name: "empty codes non-zero retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{}}, exitCode: 1, attempt: 1, want: true},
		{name: "empty codes any code retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{}}, exitCode: 127, attempt: 1, want: true},
		{name: "empty codes max attempts reached", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{}}, exitCode: 1, attempt: 5, want: false},
		// Specific codes
		{name: "specific codes success never retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 130}}, exitCode: 0, attempt: 1, want: false},
		{name: "specific codes code 1 retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 130}}, exitCode: 1, attempt: 1, want: true},
		{name: "specific codes code 2 retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 130}}, exitCode: 2, attempt: 1, want: true},
		{name: "specific codes code 130 retries", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 130}}, exitCode: 130, attempt: 1, want: true},
		{name: "specific codes code 127 not in list", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 130}}, exitCode: 127, attempt: 1, want: false},
		{name: "specific codes max attempts reached", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 130}}, exitCode: 1, attempt: 5, want: false},
		// Max attempts boundary
		{name: "attempt 1 of 3 retries", config: Config{MaxAttempts: 3}, exitCode: 1, attempt: 1, want: true},
		{name: "attempt 2 of 3 retries", config: Config{MaxAttempts: 3}, exitCode: 1, attempt: 2, want: true},
		{name: "attempt 3 of 3 exhausted", config: Config{MaxAttempts: 3}, exitCode: 1, attempt: 3, want: false},
		{name: "attempt 4 of 5 retries", config: Config{MaxAttempts: 5}, exitCode: 1, attempt: 4, want: true},
		{name: "attempt 5 of 5 exhausted", config: Config{MaxAttempts: 5}, exitCode: 1, attempt: 5, want: false},
		// Zero exit code never retries
		{name: "zero exit code with empty codes", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{}}, exitCode: 0, attempt: 1, want: false},
		{name: "zero exit code with specific codes including 0", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{0, 1, 2}}, exitCode: 0, attempt: 1, want: false},
		{name: "zero exit code with specific codes without 0", config: Config{MaxAttempts: 5, RetryableExitCodes: []int{1, 2, 3}}, exitCode: 0, attempt: 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := NewRetryer(&tt.config, nil, 42)
			got := retryer.ShouldRetry(tt.exitCode, tt.attempt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNextDelay(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name: "constant strategy no jitter",
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
			attempt: 3,
			wantMin: time.Duration(float64(4*time.Second) * 0.9),
			wantMax: time.Duration(float64(4*time.Second) * 1.1),
		},
		{
			name: "linear no jitter",
			config: Config{
				Strategy:     StrategyLinear,
				InitialDelay: 1 * time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
				Jitter:       0.0,
			},
			attempt: 3,
			wantMin: 3 * time.Second,
			wantMax: 3 * time.Second,
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

func TestWait(t *testing.T) {
	t.Run("completes normally", func(t *testing.T) {
		retryer := NewRetryer(&Config{
			Strategy:     StrategyConstant,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     30 * time.Second,
		}, nil, 42)

		ctx := context.Background()
		start := time.Now()
		err := retryer.Wait(ctx, 2)
		elapsed := time.Since(start)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, elapsed, 10*time.Millisecond)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		retryer := NewRetryer(&Config{
			Strategy:     StrategyConstant,
			InitialDelay: 10 * time.Second,
			MaxDelay:     30 * time.Second,
		}, nil, 42)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		err := retryer.Wait(ctx, 2)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.Less(t, elapsed, 1*time.Second)
	})

	t.Run("with timeout", func(t *testing.T) {
		retryer := NewRetryer(&Config{
			Strategy:     StrategyConstant,
			InitialDelay: 5 * time.Second,
			MaxDelay:     30 * time.Second,
		}, nil, 42)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := retryer.Wait(ctx, 2)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, elapsed, 1*time.Second)
	})
}

func TestStrategy_Valid(t *testing.T) {
	tests := []struct {
		strategy Strategy
		want     bool
	}{
		{StrategyConstant, true},
		{StrategyLinear, true},
		{StrategyExponential, true},
		{"", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.strategy.Valid())
		})
	}
}

func TestNewRetryer(t *testing.T) {
	t.Run("with nil logger", func(t *testing.T) {
		retryer := NewRetryer(&Config{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Strategy:     StrategyConstant,
		}, nil, 42)

		assert.NotNil(t, retryer)
		assert.True(t, retryer.ShouldRetry(1, 1))
		assert.Equal(t, 1*time.Second, retryer.NextDelay(2))
	})

	t.Run("deterministic jitter with same seed", func(t *testing.T) {
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
			assert.Equal(t, d1, d2)
		}
	})
}

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

	ctx := context.Background()
	err := retryer.Wait(ctx, 2)
	require.NoError(t, err)

	require.NotEmpty(t, logger.debugCalls)

	var foundCall logCall
	found := false
	for _, call := range logger.debugCalls {
		if strings.Contains(call.msg, "retry") || strings.Contains(call.msg, "waiting") {
			found = true
			foundCall = call
			break
		}
	}
	require.True(t, found)
	require.NotEmpty(t, foundCall.fields)

	hasAttempt := false
	hasDelay := false
	for i := 0; i < len(foundCall.fields); i += 2 {
		if i+1 < len(foundCall.fields) {
			key := foundCall.fields[i]
			if key == "attempt" {
				hasAttempt = true
				assert.Equal(t, 2, foundCall.fields[i+1])
			}
			if key == "delay" {
				hasDelay = true
			}
		}
	}
	assert.True(t, hasAttempt)
	assert.True(t, hasDelay)
}
