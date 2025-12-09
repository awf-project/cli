package cli_test

import (
	"testing"

	"github.com/vanoix/awf/internal/interfaces/cli"
)

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"success", cli.ExitSuccess, 0},
		{"user error", cli.ExitUser, 1},
		{"workflow error", cli.ExitWorkflow, 2},
		{"execution error", cli.ExitExecution, 3},
		{"system error", cli.ExitSystem, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, tt.code)
			}
		})
	}
}
