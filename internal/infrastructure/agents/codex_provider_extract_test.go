package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodexProvider_extractSessionID(t *testing.T) {
	provider := NewCodexProvider()

	tests := []struct {
		name    string
		output  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "thread.started event carries thread_id",
			output:  `{"type":"thread.started","thread_id":"019bd456-d3d4-70c3-90de-51d31a6c8571"}`,
			wantID:  "019bd456-d3d4-70c3-90de-51d31a6c8571",
			wantErr: false,
		},
		{
			name:    "thread.started with subsequent events",
			output:  `{"type":"thread.started","thread_id":"abc-123"}` + "\n" + `{"type":"message","content":"Generated code"}` + "\n" + `{"type":"done"}`,
			wantID:  "abc-123",
			wantErr: false,
		},
		{
			name:    "numeric-looking thread_id string",
			output:  `{"type":"thread.started","thread_id":"98765"}`,
			wantID:  "98765",
			wantErr: false,
		},
		{
			name:    "malformed JSON returns error",
			output:  `{"type":"thread.started","thread_id":"incomplete`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "thread.started without thread_id returns error",
			output:  `{"type":"thread.started","status":"ready"}`,
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "null thread_id returns error",
			output:  `{"type":"thread.started","thread_id":null}`,
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
			name:    "thread_id in non-thread.started event is ignored",
			output:  `{"type":"message","thread_id":"should-not-match"}`,
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
