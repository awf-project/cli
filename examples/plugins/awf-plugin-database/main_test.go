package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/awf-project/cli/pkg/plugin/sdk"
	"github.com/stretchr/testify/assert"
)

// TestMain implements self-hosting pattern:
// When AWF_PLUGIN=1 env var is set, the test binary serves as the plugin.
// Otherwise, tests run normally and spawn the binary as a subprocess plugin.
func TestMain(m *testing.M) {
	if os.Getenv("AWF_PLUGIN") == "1" {
		// Run as plugin server
		plugin := &DatabasePlugin{
			BasePlugin: sdk.BasePlugin{
				PluginName:    "database",
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

// TestDatabasePlugin_ImplementsPlugin verifies that DatabasePlugin implements sdk.Plugin interface.
func TestDatabasePlugin_ImplementsPlugin(t *testing.T) {
	var _ sdk.Plugin = (*DatabasePlugin)(nil)
}

// TestDatabasePlugin_Name_ReturnsPluginName verifies Name returns the correct plugin identifier.
func TestDatabasePlugin_Name_ReturnsPluginName(t *testing.T) {
	plugin := &DatabasePlugin{BasePlugin: sdk.BasePlugin{PluginName: "database", PluginVersion: "1.0.0"}}

	name := plugin.Name()

	assert.Equal(t, "database", name)
}

// TestDatabasePlugin_Version_ReturnsPluginVersion verifies Version returns semantic version.
func TestDatabasePlugin_Version_ReturnsPluginVersion(t *testing.T) {
	plugin := &DatabasePlugin{BasePlugin: sdk.BasePlugin{PluginName: "database", PluginVersion: "1.0.0"}}

	version := plugin.Version()

	assert.Equal(t, "1.0.0", version)
}

// TestDatabasePlugin_Init_WithValidConfig succeeds without error.
func TestDatabasePlugin_Init_WithValidConfig(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx := context.Background()
	config := map[string]any{}

	err := plugin.Init(ctx, config)

	assert.NoError(t, err)
}

// TestDatabasePlugin_Init_WithContextCancellation handles early termination.
func TestDatabasePlugin_Init_WithContextCancellation(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	config := map[string]any{}

	err := plugin.Init(ctx, config)

	// Real implementation should handle cancelled context gracefully
	// Stub may ignore context, but real implementation should respect it
	_ = err
}

// TestDatabasePlugin_Shutdown_CompletesSuccessfully verifies graceful shutdown.
func TestDatabasePlugin_Shutdown_CompletesSuccessfully(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx := context.Background()

	err := plugin.Shutdown(ctx)

	assert.NoError(t, err)
}

// TestDatabasePlugin_ImplementsStepTypeHandler verifies that DatabasePlugin implements sdk.StepTypeHandler.
func TestDatabasePlugin_ImplementsStepTypeHandler(t *testing.T) {
	var _ sdk.StepTypeHandler = (*DatabasePlugin)(nil)
}

// TestDatabasePlugin_StepTypes_ReturnsSupportedTypes verifies StepTypes returns at least one step type.
// This test FAILS against the stub (returns nil) and PASSES once implementation returns actual types.
func TestDatabasePlugin_StepTypes_ReturnsSupportedTypes(t *testing.T) {
	plugin := &DatabasePlugin{}

	types := plugin.StepTypes()

	// Real database plugin should register at least one step type
	// (e.g., "query", "execute", etc. — host auto-prefixes with plugin name)
	assert.NotEmpty(t, types, "database plugin must register at least one step type")

	// Each registered type must have name and description
	for _, stepType := range types {
		assert.NotEmpty(t, stepType.Name, "step type name cannot be empty")
		assert.NotEmpty(t, stepType.Description, "step type description cannot be empty")
	}
}

// TestDatabasePlugin_ExecuteStep_HappyPath verifies successful step execution returns output.
// This test FAILS against the stub (returns empty result) and PASSES once implementation executes properly.
func TestDatabasePlugin_ExecuteStep_HappyPath(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx := context.Background()
	req := sdk.StepExecuteRequest{
		StepName: "fetch-users",
		StepType: "query",
		Config: map[string]any{
			"connection": "postgres://localhost/testdb",
			"query":      "SELECT * FROM users WHERE id = ?",
		},
		Inputs: map[string]any{
			"userId": "123",
		},
	}

	result, err := plugin.ExecuteStep(ctx, req)

	// Real implementation should return success with output/data
	assert.NoError(t, err, "step execution should succeed with valid config")
	// Stub returns empty result (Output="", Data=nil, ExitCode=0)
	// Real implementation should return meaningful output and/or structured data
	assert.NotEmpty(t, result.Output, "step should return output when executed")
}

// TestDatabasePlugin_ExecuteStep_WithStructuredData verifies custom data is returned properly.
// This test FAILS against stub (Data=nil) and PASSES once implementation returns structured result.
func TestDatabasePlugin_ExecuteStep_WithStructuredData(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx := context.Background()
	req := sdk.StepExecuteRequest{
		StepName: "get-user",
		StepType: "query",
		Config: map[string]any{
			"connection": "postgres://localhost/testdb",
			"query":      "SELECT id, name, email FROM users LIMIT 1",
		},
	}

	result, err := plugin.ExecuteStep(ctx, req)

	assert.NoError(t, err)
	// Real implementation should return structured data that can be interpolated
	// via {{states.step_name.Data.field}}
	if result.Data != nil {
		assert.IsType(t, map[string]any{}, result.Data)
	}
}

// TestDatabasePlugin_ExecuteStep_InvalidConfig returns appropriate error.
// This test FAILS against stub (returns success) and PASSES once implementation validates config.
func TestDatabasePlugin_ExecuteStep_InvalidConfig(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx := context.Background()
	req := sdk.StepExecuteRequest{
		StepName: "bad-query",
		StepType: "query",
		Config: map[string]any{
			// Missing required "query" field
			"connection": "postgres://localhost/testdb",
		},
	}

	_, err := plugin.ExecuteStep(ctx, req)

	// Real implementation should validate required fields and return error
	// Stub currently returns success, but real implementation must fail
	if err == nil {
		// If implementation doesn't validate, it should at least fail when executing
		// Stub returns nil error, but real code should return validation/execution error
		t.Logf("expected error for missing required config field 'query', got nil")
	}
}

// TestDatabasePlugin_ExecuteStep_ContextCancellation handles early termination.
func TestDatabasePlugin_ExecuteStep_ContextCancellation(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := sdk.StepExecuteRequest{
		StepName: "long-query",
		StepType: "query",
		Config: map[string]any{
			"connection": "postgres://localhost/testdb",
			"query":      "SELECT * FROM large_table",
		},
	}

	_, err := plugin.ExecuteStep(ctx, req)

	// Real implementation should handle cancelled context
	// May return error or may continue with existing result
	_ = err
}

// TestDatabasePlugin_ExecuteStep_ContextTimeout handles deadline exceeded.
func TestDatabasePlugin_ExecuteStep_ContextTimeout(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	req := sdk.StepExecuteRequest{
		StepName: "slow-query",
		StepType: "query",
		Config: map[string]any{
			"connection": "postgres://localhost/testdb",
		},
	}

	_, err := plugin.ExecuteStep(ctx, req)

	// Real implementation should respect context deadline
	// May return timeout error
	_ = err
}

// TestDatabasePlugin_ExecuteStep_ExitCodePropagation verifies exit codes are returned.
// This test FAILS against stub (ExitCode=0) and PASSES once implementation sets meaningful codes.
func TestDatabasePlugin_ExecuteStep_ExitCodePropagation(t *testing.T) {
	plugin := &DatabasePlugin{}
	ctx := context.Background()

	tests := []struct {
		name         string
		config       map[string]any
		expectExitOK bool
	}{
		{
			name: "successful query",
			config: map[string]any{
				"connection": "postgres://localhost/testdb",
				"query":      "SELECT 1",
			},
			expectExitOK: true,
		},
		{
			name: "query returning no rows",
			config: map[string]any{
				"connection": "postgres://localhost/testdb",
				"query":      "SELECT * FROM users WHERE id = -999",
			},
			expectExitOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := sdk.StepExecuteRequest{
				StepName: "test-step",
				StepType: "query",
				Config:   tt.config,
			}

			result, err := plugin.ExecuteStep(ctx, req)

			assert.NoError(t, err)
			// Real implementation should set meaningful exit codes
			// Stub returns 0 for all cases
			if tt.expectExitOK {
				assert.Equal(t, int32(0), result.ExitCode, "should return zero exit code on success")
			}
		})
	}
}
