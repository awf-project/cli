package application_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/workflow"
	testmocks "github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F077/T005: Verify execution_service.go audit log reads "dangerously_skip_permissions"
// (snake_case) and not "dangerouslySkipPermissions" (camelCase).
//
// These tests fail against the old code (which used camelCase as the map key) and
// pass once the rename is applied.

func TestExecutionService_AgentStep_DangerouslySkipPermissions_AuditLog(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		expectWarn bool
	}{
		{
			name:       "snake_case key triggers audit warning",
			options:    map[string]any{"dangerously_skip_permissions": true},
			expectWarn: true,
		},
		{
			// Old camelCase key must no longer be recognized; no warning should fire.
			name:       "camelCase key does not trigger audit warning",
			options:    map[string]any{"dangerouslySkipPermissions": true},
			expectWarn: false,
		},
		{
			name:       "snake_case false does not trigger warning",
			options:    map[string]any{"dangerously_skip_permissions": false},
			expectWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &workflow.Workflow{
				Name:    "audit-test",
				Initial: "ask",
				Steps: map[string]*workflow.Step{
					"ask": {
						Name: "ask",
						Type: workflow.StepTypeAgent,
						Agent: &workflow.AgentConfig{
							Provider: "claude",
							Prompt:   "test prompt",
							Options:  tt.options,
						},
						OnSuccess: "done",
					},
					"done": {
						Name: "done",
						Type: workflow.StepTypeTerminal,
					},
				},
			}

			registry := testmocks.NewMockAgentRegistry()
			provider := testmocks.NewMockAgentProvider("claude")
			provider.SetExecuteFunc(func(_ context.Context, _ string, _ map[string]any, _, _ io.Writer) (*workflow.AgentResult, error) {
				return &workflow.AgentResult{
					Provider:    "claude",
					Output:      "ok",
					StartedAt:   time.Now(),
					CompletedAt: time.Now(),
				}, nil
			})
			require.NoError(t, registry.Register(provider))

			svc, mocks := NewTestHarness(t).
				WithWorkflow("audit-test", wf).
				Build()
			svc.SetAgentRegistry(registry)

			_, err := svc.Run(context.Background(), "audit-test", nil)
			require.NoError(t, err)

			warnMessages := mocks.Logger.GetMessagesByLevel("WARN")

			if tt.expectWarn {
				var found bool
				for _, msg := range warnMessages {
					if msg.Msg == "dangerously_skip_permissions enabled" {
						found = true
						break
					}
				}
				assert.True(t, found, "expected WARN %q not emitted; got: %v", "dangerously_skip_permissions enabled", warnMessages)
			} else {
				for _, msg := range warnMessages {
					assert.NotContains(t, msg.Msg, "skip_permissions",
						"unexpected audit warning emitted for skip_permissions option")
				}
			}
		})
	}
}
