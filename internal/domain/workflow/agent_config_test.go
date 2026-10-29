package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentConfig_ValidateTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		wantErr string
	}{
		{name: "omitted", timeout: 0},
		{name: "positive", timeout: 60},
		{name: "negative", timeout: -1, wantErr: "timeout must be non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				Provider: "claude",
				Prompt:   "Test",
				Timeout:  tt.timeout,
			}

			err := config.Validate(nil)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAgentConfig_TimeoutSemantics(t *testing.T) {
	omitted := AgentConfig{}
	explicit := AgentConfig{Timeout: 120}

	assert.Zero(t, omitted.Timeout)
	assert.Equal(t, 120, explicit.Timeout)
}
