package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  RetryConfig
		wantErr bool
		errMsg  string
	}{
		// Happy path: max_attempts valid
		{
			name: "valid max_attempts minimum value",
			config: RetryConfig{
				MaxAttempts: 1,
			},
			wantErr: false,
		},
		{
			name: "valid max_attempts typical value",
			config: RetryConfig{
				MaxAttempts: 5,
			},
			wantErr: false,
		},
		{
			name: "valid max_attempts high value",
			config: RetryConfig{
				MaxAttempts: 100,
			},
			wantErr: false,
		},

		// Happy path: backoff strategy valid
		{
			name: "valid backoff constant strategy",
			config: RetryConfig{
				MaxAttempts: 3,
				Backoff:     "constant",
			},
			wantErr: false,
		},
		{
			name: "valid backoff linear strategy",
			config: RetryConfig{
				MaxAttempts: 3,
				Backoff:     "linear",
			},
			wantErr: false,
		},
		{
			name: "valid backoff exponential strategy",
			config: RetryConfig{
				MaxAttempts: 3,
				Backoff:     "exponential",
			},
			wantErr: false,
		},
		{
			name: "valid backoff empty strategy defaults",
			config: RetryConfig{
				MaxAttempts: 3,
				Backoff:     "",
			},
			wantErr: false,
		},

		// Happy path: jitter valid
		{
			name: "valid jitter minimum value",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      0.0,
			},
			wantErr: false,
		},
		{
			name: "valid jitter maximum value",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      1.0,
			},
			wantErr: false,
		},
		{
			name: "valid jitter mid-range value",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      0.5,
			},
			wantErr: false,
		},
		{
			name: "valid jitter small positive value",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      0.1,
			},
			wantErr: false,
		},

		// Happy path: multiplier valid
		{
			name: "valid multiplier zero value",
			config: RetryConfig{
				MaxAttempts: 2,
				Multiplier:  0,
			},
			wantErr: false,
		},
		{
			name: "valid multiplier positive value",
			config: RetryConfig{
				MaxAttempts: 2,
				Multiplier:  2.0,
			},
			wantErr: false,
		},
		{
			name: "valid multiplier fractional value",
			config: RetryConfig{
				MaxAttempts: 2,
				Multiplier:  1.5,
			},
			wantErr: false,
		},

		// Happy path: combined valid config
		{
			name: "valid complete retry config",
			config: RetryConfig{
				MaxAttempts:        3,
				InitialDelayMs:     100,
				MaxDelayMs:         5000,
				Backoff:            "exponential",
				Multiplier:         2.0,
				Jitter:             0.2,
				RetryableExitCodes: []int{1, 2, 3},
			},
			wantErr: false,
		},

		// Error paths: max_attempts invalid
		{
			name: "error max_attempts zero",
			config: RetryConfig{
				MaxAttempts: 0,
			},
			wantErr: true,
			errMsg:  "max_attempts must be >= 1",
		},
		{
			name: "error max_attempts negative",
			config: RetryConfig{
				MaxAttempts: -1,
			},
			wantErr: true,
			errMsg:  "max_attempts must be >= 1",
		},
		{
			name: "error max_attempts large negative",
			config: RetryConfig{
				MaxAttempts: -100,
			},
			wantErr: true,
			errMsg:  "max_attempts must be >= 1",
		},

		// Error paths: backoff strategy invalid
		{
			name: "error backoff invalid strategy",
			config: RetryConfig{
				MaxAttempts: 2,
				Backoff:     "fibonacci",
			},
			wantErr: true,
			errMsg:  "invalid backoff strategy",
		},
		{
			name: "error backoff misspelled strategy",
			config: RetryConfig{
				MaxAttempts: 2,
				Backoff:     "exponental", //nolint:misspell // intentionally misspelled to test validation rejection
			},
			wantErr: true,
			errMsg:  "invalid backoff strategy",
		},
		{
			name: "error backoff random string",
			config: RetryConfig{
				MaxAttempts: 2,
				Backoff:     "unknown",
			},
			wantErr: true,
			errMsg:  "invalid backoff strategy",
		},

		// Error paths: jitter invalid
		{
			name: "error jitter negative",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      -0.1,
			},
			wantErr: true,
			errMsg:  "jitter must be between 0.0 and 1.0",
		},
		{
			name: "error jitter exceeds maximum",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      1.1,
			},
			wantErr: true,
			errMsg:  "jitter must be between 0.0 and 1.0",
		},
		{
			name: "error jitter far exceeds maximum",
			config: RetryConfig{
				MaxAttempts: 2,
				Jitter:      5.0,
			},
			wantErr: true,
			errMsg:  "jitter must be between 0.0 and 1.0",
		},

		// Error paths: multiplier invalid
		{
			name: "error multiplier negative",
			config: RetryConfig{
				MaxAttempts: 2,
				Multiplier:  -0.5,
			},
			wantErr: true,
			errMsg:  "multiplier must be >= 0",
		},
		{
			name: "error multiplier large negative",
			config: RetryConfig{
				MaxAttempts: 2,
				Multiplier:  -10.0,
			},
			wantErr: true,
			errMsg:  "multiplier must be >= 0",
		},

		// Error paths: multiple violations (only first is reported)
		{
			name: "error max_attempts and jitter both invalid",
			config: RetryConfig{
				MaxAttempts: 0,
				Jitter:      1.5,
			},
			wantErr: true,
			errMsg:  "max_attempts must be >= 1",
		},
		{
			name: "error max_attempts and multiplier both invalid",
			config: RetryConfig{
				MaxAttempts: -1,
				Multiplier:  -1.0,
			},
			wantErr: true,
			errMsg:  "max_attempts must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
