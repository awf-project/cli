//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/internal/infrastructure/executor"
	"github.com/vanoix/awf/internal/infrastructure/repository"
	"github.com/vanoix/awf/internal/testutil"
)

func TestInputValidation_HappyPath(t *testing.T) {
	tests := []struct {
		name         string
		workflowYAML string
		inputs       map[string]interface{}
		wantSuccess  bool
		wantErrMsg   string
	}{
		{
			name: "pattern validation - valid email",
			workflowYAML: `
name: email-validation
version: "1.0.0"
inputs:
  - name: email
    type: string
    required: true
    description: "User email"
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
states:
  initial: success
  success:
    type: step
    command: 'echo "Email validated: {{.inputs.email}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"email": "user@example.com"},
			wantSuccess: true,
		},
		{
			name: "pattern validation - invalid email",
			workflowYAML: `
name: email-validation
version: "1.0.0"
inputs:
  - name: email
    type: string
    required: true
    description: "User email"
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
states:
  initial: success
  success:
    type: step
    command: echo "Should not execute"
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"email": "not-an-email"},
			wantSuccess: false,
			wantErrMsg:  "pattern",
		},
		{
			name: "enum validation - valid value",
			workflowYAML: `
name: env-validation
version: "1.0.0"
inputs:
  - name: environment
    type: string
    required: true
    description: "Deployment environment"
    validation:
      enum: ["dev", "staging", "prod"]
states:
  initial: deploy
  deploy:
    type: step
    command: 'echo "Deploying to {{.inputs.environment}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"environment": "staging"},
			wantSuccess: true,
		},
		{
			name: "enum validation - invalid value",
			workflowYAML: `
name: env-validation
version: "1.0.0"
inputs:
  - name: environment
    type: string
    required: true
    description: "Deployment environment"
    validation:
      enum: ["dev", "staging", "prod"]
states:
  initial: deploy
  deploy:
    type: step
    command: echo "Should not execute"
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"environment": "production"},
			wantSuccess: false,
			wantErrMsg:  "not in allowed values",
		},
		{
			name: "numeric validation - within range",
			workflowYAML: `
name: port-validation
version: "1.0.0"
inputs:
  - name: port
    type: integer
    required: true
    description: "Server port"
    validation:
      min: 1024
      max: 65535
states:
  initial: start
  start:
    type: step
    command: 'echo "Starting on port {{.inputs.port}}"'
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"port": 8080},
			wantSuccess: true,
		},
		{
			name: "numeric validation - below min",
			workflowYAML: `
name: port-validation
version: "1.0.0"
inputs:
  - name: port
    type: integer
    required: true
    description: "Server port"
    validation:
      min: 1024
      max: 65535
states:
  initial: start
  start:
    type: step
    command: echo "Should not execute"
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"port": 80},
			wantSuccess: false,
			wantErrMsg:  "min",
		},
		{
			name: "combined validation - all pass",
			workflowYAML: `
name: complex-validation
version: "1.0.0"
inputs:
  - name: username
    type: string
    required: true
    description: "Username"
    validation:
      pattern: "^[a-z0-9_]{3,20}$"
  - name: role
    type: string
    required: true
    description: "User role"
    validation:
      enum: ["admin", "user", "guest"]
  - name: age
    type: integer
    required: true
    description: "User age"
    validation:
      min: 18
      max: 120
states:
  initial: create
  create:
    type: step
    command: echo "Creating user"
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"username": "john_doe", "role": "user", "age": 25},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			execCtx, err := svc.Run(ctx, "test", tt.inputs)

			if tt.wantSuccess {
				assert.NoError(t, err, "workflow should execute successfully")
				if execCtx != nil {
					assert.NotEmpty(t, execCtx.WorkflowID, "execution should have ID")
					assert.Equal(t, workflow.StatusCompleted, execCtx.Status, "execution should succeed")
				}
			} else {
				assert.Error(t, err, "workflow should fail validation")
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg, "error should mention validation type")
				}
			}
		})
	}
}

// TestInputValidation_EdgeCases tests edge cases for input validation.
func TestInputValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		workflowYAML string
		inputs       map[string]interface{}
		wantSuccess  bool
		wantErrMsg   string
	}{
		{
			name: "empty input with pattern fails",
			workflowYAML: `
name: empty-validation
version: "1.0.0"
inputs:
  - name: required
    type: string
    required: true
    description: "Required field"
    validation:
      pattern: "^.+$"
states:
  initial: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"required": ""},
			wantSuccess: false,
			wantErrMsg:  "pattern",
		},
		{
			name: "unicode input validated correctly",
			workflowYAML: `
name: unicode-validation
version: "1.0.0"
inputs:
  - name: text
    type: string
    required: true
    description: "Unicode text"
    validation:
      pattern: "^.+$"
states:
  initial: process
  process:
    type: step
    command: echo "{{.inputs.text}}"
    on_success: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"text": "你好世界🌍"},
			wantSuccess: true,
		},
		{
			name: "boundary numeric value at min",
			workflowYAML: `
name: boundary-min
version: "1.0.0"
inputs:
  - name: value
    type: integer
    required: true
    description: "Numeric value"
    validation:
      min: 0
      max: 100
states:
  initial: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"value": 0},
			wantSuccess: true,
		},
		{
			name: "boundary numeric value at max",
			workflowYAML: `
name: boundary-max
version: "1.0.0"
inputs:
  - name: value
    type: integer
    required: true
    description: "Numeric value"
    validation:
      min: 0
      max: 100
states:
  initial: end
  end:
    type: terminal
    status: success
`,
			inputs:      map[string]interface{}{"value": 100},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, "workflows")

			require.NoError(t, os.MkdirAll(workflowsDir, 0o755))

			workflowPath := filepath.Join(workflowsDir, "test.yaml")
			require.NoError(t, os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0o644))

			repo := repository.NewYAMLRepository(workflowsDir)
			stateStore := testutil.NewMockStateStore()
			exec := executor.NewShellExecutor()
			log := testutil.NewMockLogger()

			svc := testutil.NewExecutionServiceBuilder().
				WithWorkflowRepository(repo).
				WithStateStore(stateStore).
				WithExecutor(exec).
				WithLogger(log).
				Build()

			execCtx, err := svc.Run(ctx, "test", tt.inputs)

			if tt.wantSuccess {
				assert.NoError(t, err)
				if execCtx != nil {
					assert.Equal(t, workflow.StatusCompleted, execCtx.Status)
				}
			} else {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			}
		})
	}
}
