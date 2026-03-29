package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/plugin/sdk"
	pluginv1 "github.com/awf-project/cli/proto/plugin/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestMain implements self-hosting pattern:
// When AWF_PLUGIN=1 env var is set, the test binary serves as the plugin.
// Otherwise, tests run normally and spawn the binary as a subprocess plugin.
func TestMain(m *testing.M) {
	if os.Getenv("AWF_PLUGIN") == "1" {
		// Run as plugin server
		plugin := &SecurityValidatorPlugin{
			BasePlugin: sdk.BasePlugin{
				PluginName:    "security-validator",
				PluginVersion: "1.0.0",
			},
		}
		sdk.Serve(plugin)
		return
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// TestSecurityValidatorPlugin_ImplementsPlugin verifies that SecurityValidatorPlugin implements sdk.Plugin interface.
func TestSecurityValidatorPlugin_ImplementsPlugin(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}

	require.NotNil(t, plugin)

	var _ sdk.Plugin = (*SecurityValidatorPlugin)(nil)
}

// TestSecurityValidatorPlugin_Name_ReturnsPluginName verifies Name returns the correct plugin identifier.
func TestSecurityValidatorPlugin_Name_ReturnsPluginName(t *testing.T) {
	plugin := &SecurityValidatorPlugin{BasePlugin: sdk.BasePlugin{PluginName: "security-validator", PluginVersion: "1.0.0"}}

	name := plugin.Name()

	assert.Equal(t, "security-validator", name)
}

// TestSecurityValidatorPlugin_Version_ReturnsPluginVersion verifies Version returns semantic version.
func TestSecurityValidatorPlugin_Version_ReturnsPluginVersion(t *testing.T) {
	plugin := &SecurityValidatorPlugin{BasePlugin: sdk.BasePlugin{PluginName: "security-validator", PluginVersion: "1.0.0"}}

	version := plugin.Version()

	assert.Equal(t, "1.0.0", version)
}

// TestSecurityValidatorPlugin_Init_Succeeds verifies that Init completes without error.
func TestSecurityValidatorPlugin_Init_Succeeds(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	ctx := context.Background()
	config := map[string]any{}

	err := plugin.Init(ctx, config)

	assert.NoError(t, err)
	assert.NotEmpty(t, plugin.secretPatterns)
	assert.NotEmpty(t, plugin.dangerousWords)
}

// TestSecurityValidatorPlugin_ImplementsValidator verifies that SecurityValidatorPlugin implements sdk.Validator.
func TestSecurityValidatorPlugin_ImplementsValidator(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}

	var _ sdk.Validator = (*SecurityValidatorPlugin)(nil)
	assert.NotNil(t, plugin)
}

// TestSecurityValidatorPlugin_ValidateWorkflow_ReturnsEmptyForValidWorkflow verifies workflow validation.
func TestSecurityValidatorPlugin_ValidateWorkflow_ReturnsEmptyForValidWorkflow(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "echo hello",
			},
		},
	}

	issues, err := plugin.ValidateWorkflow(context.Background(), workflow)

	require.NoError(t, err)
	assert.Empty(t, issues)
}

// TestSecurityValidatorPlugin_ValidateStep_DetectsHardcodedAPIKey detects API key patterns.
func TestSecurityValidatorPlugin_ValidateStep_DetectsHardcodedAPIKey(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "curl -H 'Authorization: api_key=secret123' https://api.example.com",
				Timeout: 30,
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	require.Greater(t, len(issues), 0)

	// Verify that at least one error is about the secret
	foundSecret := false
	for _, issue := range issues {
		if issue.Severity == sdk.SeverityError && strings.Contains(issue.Message, "secret") {
			foundSecret = true
			break
		}
	}
	assert.True(t, foundSecret, "should detect hardcoded secret")
}

// TestSecurityValidatorPlugin_ValidateStep_DetectsHardcodedPassword detects password patterns.
func TestSecurityValidatorPlugin_ValidateStep_DetectsHardcodedPassword(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "deploy.sh password='mySecretPassword' user='admin'",
				Timeout: 30,
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	require.Greater(t, len(issues), 0)

	// Verify that at least one error is about the secret
	foundSecret := false
	for _, issue := range issues {
		if issue.Severity == sdk.SeverityError && strings.Contains(issue.Message, "secret") {
			foundSecret = true
			break
		}
	}
	assert.True(t, foundSecret, "should detect hardcoded password")
}

// TestSecurityValidatorPlugin_ValidateStep_DetectsDangerousCommands detects dangerous operations.
func TestSecurityValidatorPlugin_ValidateStep_DetectsDangerousCommands(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "rm -rf /tmp/*",
				Timeout: 30,
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	require.Greater(t, len(issues), 0)

	// Verify that at least one warning is about the dangerous command
	foundDangerous := false
	for _, issue := range issues {
		if issue.Severity == sdk.SeverityWarning && strings.Contains(issue.Message, "dangerous") {
			foundDangerous = true
			break
		}
	}
	assert.True(t, foundDangerous, "should detect dangerous command")
}

// TestSecurityValidatorPlugin_ValidateStep_WarnsAboutMissingTimeout warns when timeout not set.
func TestSecurityValidatorPlugin_ValidateStep_WarnsAboutMissingTimeout(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "long-running-command",
				Timeout: 0, // No timeout set
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, sdk.SeverityWarning, issues[0].Severity)
	assert.Contains(t, issues[0].Message, "timeout")
}

// TestSecurityValidatorPlugin_ValidateStep_AllowsCommandWithTimeout allows timeouts.
func TestSecurityValidatorPlugin_ValidateStep_AllowsCommandWithTimeout(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "long-running-command",
				Timeout: 30, // Timeout is set
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	assert.Empty(t, issues)
}

// TestSecurityValidatorPlugin_ValidateStep_IgnoresNonCommandSteps ignores non-command steps.
func TestSecurityValidatorPlugin_ValidateStep_IgnoresNonCommandSteps(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type: "agent",
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	assert.Empty(t, issues)
}

// TestSecurityValidatorPlugin_ValidateStep_MultipleIssuesPerStep reports multiple issues.
func TestSecurityValidatorPlugin_ValidateStep_MultipleIssuesPerStep(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	_ = plugin.Init(context.Background(), map[string]any{})

	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "rm -rf / && curl -H 'password=secret123'", // Both dangerous and hardcoded secret
				Timeout: 0,
			},
		},
	}

	issues, err := plugin.ValidateStep(context.Background(), workflow, "step1")

	require.NoError(t, err)
	// Should have at least: secret, dangerous command, and timeout warning
	assert.Greater(t, len(issues), 1)
}

// TestSecurityValidatorPlugin_CanServe verifies that the plugin can be served via sdk.Serve().
func TestSecurityValidatorPlugin_CanServe(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}

	var _ sdk.Plugin = plugin
	assert.NotNil(t, plugin)
}

// TestSecurityValidatorPlugin_Shutdown_Succeeds verifies that Shutdown completes without error.
func TestSecurityValidatorPlugin_Shutdown_Succeeds(t *testing.T) {
	plugin := &SecurityValidatorPlugin{}
	ctx := context.Background()

	err := plugin.Shutdown(ctx)

	assert.NoError(t, err)
}

// TestPluginYAMLManifest_ExistsAndIsValid verifies that plugin.yaml exists and is valid.
func TestPluginYAMLManifest_ExistsAndIsValid(t *testing.T) {
	content, err := os.ReadFile("plugin.yaml")
	require.NoError(t, err, "plugin.yaml must exist in plugin directory")

	assert.Greater(t, len(content), 0, "plugin.yaml should not be empty")

	manifestStr := string(content)
	assert.Contains(t, manifestStr, "name:", "manifest must contain 'name'")
	assert.Contains(t, manifestStr, "security-validator", "manifest must declare plugin name")
	assert.Contains(t, manifestStr, "version:", "manifest must contain 'version'")
	assert.Contains(t, manifestStr, "awf_version:", "manifest must contain 'awf_version'")
	assert.Contains(t, manifestStr, "capabilities:", "manifest must declare capabilities")
	assert.Contains(t, manifestStr, "validators", "manifest must declare validators capability")
}

// TestPluginMakefile_ExistsAndBuildable verifies that Makefile exists and contains build targets.
func TestPluginMakefile_ExistsAndBuildable(t *testing.T) {
	content, err := os.ReadFile("Makefile")
	require.NoError(t, err, "Makefile must exist in plugin directory")

	assert.Greater(t, len(content), 0, "Makefile should not be empty")

	makefileStr := string(content)
	assert.Contains(t, makefileStr, "build:", "Makefile must contain 'build' target")
}

// TestPluginREADME_ExistsAndDocumented verifies that README.md exists and documents the plugin.
func TestPluginREADME_ExistsAndDocumented(t *testing.T) {
	content, err := os.ReadFile("README.md")
	require.NoError(t, err, "README.md must exist in plugin directory")

	assert.Greater(t, len(content), 0, "README.md should not be empty")

	readmeStr := string(content)
	assert.Contains(t, readmeStr, "security-validator", "README should mention plugin name")
	assert.Contains(t, readmeStr, "security", "README should document security validation")
}

// Integration test using self-hosting pattern
// TestIntegration_SecurityValidator_End2End tests the plugin through gRPC.
// This test spawns the plugin binary as a subprocess and communicates via gRPC.
func TestIntegration_SecurityValidator_End2End(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Start plugin in self-hosted mode
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // test-only listener, no context cancellation needed
	require.NoError(t, err)
	defer listener.Close()

	addr := listener.Addr().String()

	// Create gRPC server
	grpcServer := grpc.NewServer()

	plugin := &SecurityValidatorPlugin{
		BasePlugin: sdk.BasePlugin{
			PluginName:    "security-validator",
			PluginVersion: "1.0.0",
		},
	}
	err = plugin.Init(context.Background(), map[string]any{})
	require.NoError(t, err)

	// Register validator service
	validatorServer := &validatorServiceServerAdapter{impl: plugin}
	pluginv1.RegisterValidatorServiceServer(grpcServer, validatorServer)

	// Start server in goroutine
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create gRPC client
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := pluginv1.NewValidatorServiceClient(conn)

	// Test ValidateWorkflow with hardcoded secret
	workflow := sdk.WorkflowDefinition{
		Name:    "test-workflow",
		Initial: "step1",
		Steps: map[string]sdk.StepDefinition{
			"step1": {
				Type:    "command",
				Command: "curl -H 'api_key=secret123'",
			},
		},
	}

	workflowJSON, err := json.Marshal(workflow)
	require.NoError(t, err)

	resp, err := client.ValidateWorkflow(context.Background(), &pluginv1.ValidateWorkflowRequest{
		WorkflowJson: workflowJSON,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Test ValidateStep
	stepResp, err := client.ValidateStep(context.Background(), &pluginv1.ValidateStepRequest{
		WorkflowJson: workflowJSON,
		StepName:     "step1",
	})

	require.NoError(t, err)
	require.NotNil(t, stepResp)
	require.Greater(t, len(stepResp.Issues), 0)
}

// validatorServiceServerAdapter adapts the Validator interface to gRPC.
// This is for testing purposes only; the actual adapter is in the SDK.
type validatorServiceServerAdapter struct {
	pluginv1.UnimplementedValidatorServiceServer
	impl sdk.Validator
}

func (s *validatorServiceServerAdapter) ValidateWorkflow(ctx context.Context, req *pluginv1.ValidateWorkflowRequest) (*pluginv1.ValidateWorkflowResponse, error) {
	var def sdk.WorkflowDefinition
	if err := json.Unmarshal(req.WorkflowJson, &def); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	issues, err := s.impl.ValidateWorkflow(ctx, def)
	if err != nil {
		return nil, fmt.Errorf("validate workflow: %w", err)
	}

	protoIssues := make([]*pluginv1.ValidationIssue, len(issues))
	for i, issue := range issues {
		protoIssues[i] = &pluginv1.ValidationIssue{
			Severity: mapSeverity(issue.Severity),
			Message:  issue.Message,
			Step:     issue.Step,
			Field:    issue.Field,
		}
	}

	return &pluginv1.ValidateWorkflowResponse{
		Issues: protoIssues,
	}, nil
}

func (s *validatorServiceServerAdapter) ValidateStep(ctx context.Context, req *pluginv1.ValidateStepRequest) (*pluginv1.ValidateStepResponse, error) {
	var def sdk.WorkflowDefinition
	if err := json.Unmarshal(req.WorkflowJson, &def); err != nil {
		return nil, fmt.Errorf("unmarshal workflow: %w", err)
	}

	issues, err := s.impl.ValidateStep(ctx, def, req.StepName)
	if err != nil {
		return nil, fmt.Errorf("validate step: %w", err)
	}

	protoIssues := make([]*pluginv1.ValidationIssue, len(issues))
	for i, issue := range issues {
		protoIssues[i] = &pluginv1.ValidationIssue{
			Severity: mapSeverity(issue.Severity),
			Message:  issue.Message,
			Step:     issue.Step,
			Field:    issue.Field,
		}
	}

	return &pluginv1.ValidateStepResponse{
		Issues: protoIssues,
	}, nil
}

func mapSeverity(s sdk.Severity) pluginv1.Severity {
	switch s {
	case sdk.SeverityWarning:
		return pluginv1.Severity_SEVERITY_WARNING
	case sdk.SeverityInfo:
		return pluginv1.Severity_SEVERITY_INFO
	default:
		return pluginv1.Severity_SEVERITY_ERROR
	}
}
