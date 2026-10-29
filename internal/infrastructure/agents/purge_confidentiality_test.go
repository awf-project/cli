package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

func TestPurgeRemovalErrorConfidentiality(t *testing.T) {
	const secret = "PRIVATE_MCP_ERROR_CANARY" //nolint:gosec // Controlled confidentiality canary, not a credential.
	exec := commandExecutorFunc(func(_ context.Context, command *ports.Command) (*ports.CommandResult, error) {
		if strings.Contains(command.Program, "mcp list") {
			return &ports.CommandResult{}, nil
		}
		return nil, fmt.Errorf("%s: %w", secret, context.Canceled)
	})
	log := &purgeLogCapture{}
	purgeForCLI(context.Background(), exec, log, "gemini", func(string) []string { return []string{"awf-proxy-probe"} }, 25*time.Millisecond)
	if strings.Contains(fmt.Sprint(log.entries), secret) {
		t.Errorf("T037: removal failure log exposes confidential error: %v", log.entries)
	}
}

func TestPurgeInvalidTimeoutDoesNotLogEnvironmentValue(t *testing.T) {
	const canary = "PRIVATE_INVALID_TIMEOUT"
	t.Setenv("AWF_MCP_PROXY_LIST_TIMEOUT", canary)
	log := &purgeLogCapture{}
	if got := resolveListTimeout(log); got != mcpListDefaultTimeout {
		t.Fatalf("invalid duration changed default: %v", got)
	}
	if strings.Contains(fmt.Sprint(log.entries), canary) {
		t.Fatal("invalid environment value appeared in diagnostics")
	}
}
