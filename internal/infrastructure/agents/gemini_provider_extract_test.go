package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiProvider_extractInitEvent(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantType string
		wantID   string
		wantNil  bool
	}{
		{
			name:     "valid init event",
			output:   `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":"031da63a-73be-42f5-ae0d-890aae0b6323","model":"auto-gemini-3"}`,
			wantType: "init",
			wantID:   "031da63a-73be-42f5-ae0d-890aae0b6323",
			wantNil:  false,
		},
		{
			name:     "init event with trailing newline",
			output:   `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":"550e8400-e29b-41d4-a716-446655440000","model":"auto-gemini-3"}` + "\n",
			wantType: "init",
			wantID:   "550e8400-e29b-41d4-a716-446655440000",
			wantNil:  false,
		},
		{
			name:     "init event in ndjson with other events",
			output:   `{"type":"message","content":"hello"}` + "\n" + `{"type":"init","session_id":"a1b2c3d4-e5f6-47g8-h9i0-j1k2l3m4n5o6"}` + "\n" + `{"type":"end","status":"done"}`,
			wantType: "init",
			wantID:   "a1b2c3d4-e5f6-47g8-h9i0-j1k2l3m4n5o6",
			wantNil:  false,
		},
		{
			name:    "empty output",
			output:  "",
			wantNil: true,
		},
		{
			name:    "no init event found",
			output:  `{"type":"message","content":"hello"}` + "\n" + `{"type":"end","status":"done"}`,
			wantNil: true,
		},
		{
			name:    "invalid json lines",
			output:  `not valid json` + "\n" + `also not json`,
			wantNil: true,
		},
		{
			name:     "empty lines with init",
			output:   "" + "\n" + `{"type":"init","session_id":"test-uuid"}` + "\n" + "",
			wantType: "init",
			wantID:   "test-uuid",
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGeminiProvider()
			evt := p.extractInitEvent(tt.output)

			if tt.wantNil {
				assert.Nil(t, evt)
				return
			}

			require.NotNil(t, evt)
			assert.Equal(t, tt.wantType, evt["type"])
			assert.Equal(t, tt.wantID, evt["session_id"])
		})
	}
}

func TestGeminiProvider_extractSessionID(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantID    string
		wantError bool
		wantMsg   string
	}{
		{
			name:      "valid session id extraction",
			output:    `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","session_id":"031da63a-73be-42f5-ae0d-890aae0b6323","model":"auto-gemini-3"}`,
			wantID:    "031da63a-73be-42f5-ae0d-890aae0b6323",
			wantError: false,
		},
		{
			name:      "session id in ndjson stream",
			output:    `{"type":"message","content":"hello"}` + "\n" + `{"type":"init","session_id":"550e8400-e29b-41d4-a716-446655440000"}` + "\n" + `{"type":"end"}`,
			wantID:    "550e8400-e29b-41d4-a716-446655440000",
			wantError: false,
		},
		{
			name:      "empty output",
			output:    "",
			wantError: true,
			wantMsg:   "empty output",
		},
		{
			name:      "no init event",
			output:    `{"type":"message","content":"hello"}` + "\n" + `{"type":"end"}`,
			wantError: true,
			wantMsg:   "init event not found",
		},
		{
			name:      "init event without session_id field",
			output:    `{"type":"init","timestamp":"2026-04-07T21:55:36.994Z","model":"auto-gemini-3"}`,
			wantError: true,
			wantMsg:   "session_id missing",
		},
		{
			name:      "session_id field is null",
			output:    `{"type":"init","session_id":null}`,
			wantError: true,
			wantMsg:   "session_id missing",
		},
		{
			name:      "session_id is not a string",
			output:    `{"type":"init","session_id":123}`,
			wantError: true,
			wantMsg:   "session_id is not a string",
		},
		{
			name:      "session_id is empty string",
			output:    `{"type":"init","session_id":""}`,
			wantError: true,
			wantMsg:   "session_id is empty",
		},
		{
			name:      "invalid json",
			output:    `not valid json`,
			wantError: true,
			wantMsg:   "init event not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGeminiProvider()
			id, err := p.extractSessionID(tt.output)

			if tt.wantError {
				assert.Error(t, err)
				if tt.wantMsg != "" {
					assert.ErrorContains(t, err, tt.wantMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}
