package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeProvider_extractSessionID(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "step_start event carries sessionID",
			output:  `{"type":"step_start","timestamp":1775599542766,"sessionID":"ses_296052f0bffeFudXE4xOn0vSEJ"}`,
			wantID:  "ses_296052f0bffeFudXE4xOn0vSEJ",
			wantErr: false,
		},
		{
			name:    "step_start with subsequent events",
			output:  `{"type":"step_start","sessionID":"ses_abc123"}` + "\n" + `{"type":"step_end","output":"done"}`,
			wantID:  "ses_abc123",
			wantErr: false,
		},
		{
			name:    "numeric-looking sessionID string",
			output:  `{"type":"step_start","sessionID":"98765"}`,
			wantID:  "98765",
			wantErr: false,
		},
		{
			name:    "malformed JSON returns error",
			output:  `{"type":"step_start","sessionID":"incomplete`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "step_start without sessionID returns error",
			output:  `{"type":"step_start","timestamp":12345}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "null sessionID returns error",
			output:  `{"type":"step_start","sessionID":null}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "empty output returns error",
			output:  "",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "plain text returns error",
			output:  "plain text output with no JSON",
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "sessionID in non-step_start event is ignored",
			output:  `{"type":"step_end","sessionID":"should-not-match"}`,
			wantID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.extractSessionID(tt.output)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, got)
			}
		})
	}
}

func TestOpenCodeProvider_extractStepStartEvent(t *testing.T) {
	provider := NewOpenCodeProvider()

	tests := []struct {
		name      string
		output    string
		wantType  string
		wantHasID bool
		wantErr   bool
	}{
		{
			name:      "step_start event is found and parsed",
			output:    `{"type":"step_start","timestamp":1775599542766,"sessionID":"ses_abc123"}`,
			wantType:  "step_start",
			wantHasID: true,
			wantErr:   false,
		},
		{
			name:      "step_start event among multiple events",
			output:    `{"type":"status","message":"starting"}` + "\n" + `{"type":"step_start","sessionID":"ses_xyz789"}` + "\n" + `{"type":"step_end","output":"result"}`,
			wantType:  "step_start",
			wantHasID: true,
			wantErr:   false,
		},
		{
			name:      "step_start with minimal fields",
			output:    `{"type":"step_start"}`,
			wantType:  "step_start",
			wantHasID: false,
			wantErr:   false,
		},
		{
			name:      "no step_start event returns error",
			output:    `{"type":"step_end","output":"done"}` + "\n" + `{"type":"status","code":0}`,
			wantType:  "",
			wantHasID: false,
			wantErr:   true,
		},
		{
			name:      "empty output returns error",
			output:    "",
			wantType:  "",
			wantHasID: false,
			wantErr:   true,
		},
		{
			name:      "only whitespace lines returns error",
			output:    "   \n  \n",
			wantType:  "",
			wantHasID: false,
			wantErr:   true,
		},
		{
			name:      "malformed JSON is skipped",
			output:    `{"type":"step_start", invalid json}` + "\n" + `{"type":"other"}`,
			wantType:  "",
			wantHasID: false,
			wantErr:   true,
		},
		{
			name:      "first step_start event is returned when multiple exist",
			output:    `{"type":"step_start","sessionID":"first"}` + "\n" + `{"type":"step_start","sessionID":"second"}`,
			wantType:  "step_start",
			wantHasID: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.extractStepStartEvent(tt.output)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.wantType, got["type"])
				if tt.wantHasID {
					assert.NotEmpty(t, got["sessionID"])
				}
			}
		})
	}
}
