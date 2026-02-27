// Package sdk provides the public API for building AWF plugins.
//
// This package contains interfaces, types, and utilities for developing
// plugins that extend AWF workflow capabilities. Plugin developers import
// this package to create custom operations that integrate with AWF workflows.
//
// Key features:
//   - Plugin interface with lifecycle methods (Init, Shutdown)
//   - OperationHandler and OperationProvider interfaces for custom operations
//   - BasePlugin for embedding (provides default implementations)
//   - OperationResult for structured operation responses
//   - Input helpers for extracting typed values from operation inputs
//   - Schema types for declaring operation signatures
//
// # Core Interfaces
//
// ## Plugin (sdk.go)
//
// Plugin interface defines the plugin lifecycle:
//   - Name: Unique plugin identifier
//   - Version: Semantic version string
//   - Init: Initialize with configuration
//   - Shutdown: Graceful cleanup on exit
//
// ## OperationHandler (sdk.go)
//
// OperationHandler processes single operation requests:
//   - Handle: Execute operation with inputs, return outputs
//
// ## OperationProvider (sdk.go)
//
// OperationProvider for plugins offering multiple operations:
//   - Operations: List of operation names
//   - HandleOperation: Execute named operation with inputs
//
// # Base Types
//
// ## BasePlugin (sdk.go)
//
// BasePlugin provides minimal implementation for embedding:
//   - Implements Plugin interface with no-op Init/Shutdown
//   - Stores PluginName and PluginVersion
//   - Embed in your plugin struct to inherit base behavior
//
// ## OperationResult (sdk.go)
//
// OperationResult holds execution results:
//   - Success: Boolean success flag
//   - Output: Human-readable output text
//   - Data: Structured output data (map[string]any)
//   - Error: Error message if failed
//   - ToMap: Convert to map for handler return values
//
// # Helper Functions
//
// ## Result Constructors (sdk.go)
//
//	NewSuccessResult(output string, data map[string]any) *OperationResult
//	NewErrorResult(errMsg string) *OperationResult
//	NewErrorResultf(format string, args ...any) *OperationResult
//
// ## Input Extractors (sdk.go)
//
// Type-safe input extraction with optional defaults:
//
//	GetString(inputs map[string]any, key string) (string, bool)
//	GetStringDefault(inputs map[string]any, key, defaultValue string) string
//	GetInt(inputs map[string]any, key string) (int, bool)
//	GetIntDefault(inputs map[string]any, key string, defaultValue int) int
//	GetBool(inputs map[string]any, key string) (value, ok bool)
//	GetBoolDefault(inputs map[string]any, key string, defaultValue bool) bool
//
// # Schema Types
//
// ## InputSchema (sdk.go)
//
// InputSchema defines an operation input parameter:
//   - Type: string, integer, boolean, array, object
//   - Required: Whether parameter is mandatory
//   - Default: Default value if not provided
//   - Description: Human-readable description
//   - Validation: Optional validation rule (url, email, etc.)
//
// ## OperationSchema (sdk.go)
//
// OperationSchema defines a plugin-provided operation:
//   - Name: Operation name (e.g., "slack.send")
//   - Description: Human-readable description
//   - Inputs: Map of input parameter schemas
//   - Outputs: List of output field names
//
// ## Schema Helpers (sdk.go)
//
//	RequiredInput(inputType, description string) InputSchema
//	OptionalInput(inputType, description string, defaultValue any) InputSchema
//	IsValidInputType(t string) bool
//
// # Constants
//
// ## Input Types (sdk.go)
//
//	InputTypeString  = "string"
//	InputTypeInteger = "integer"
//	InputTypeBoolean = "boolean"
//	InputTypeArray   = "array"
//	InputTypeObject  = "object"
//
// ## Errors (sdk.go)
//
//	ErrNotImplemented   # Stub method needs implementation
//	ErrInvalidInput     # Invalid operation input
//	ErrOperationFailed  # Operation execution failed
//
// # Usage Examples
//
// ## Simple Plugin with Single Operation
//
//	package main
//
//	import (
//	    "context"
//	    "github.com/awf-project/cli/pkg/plugin/sdk"
//	)
//
//	type SlackPlugin struct {
//	    sdk.BasePlugin
//	    webhookURL string
//	}
//
//	func NewSlackPlugin() *SlackPlugin {
//	    return &SlackPlugin{
//	        BasePlugin: sdk.BasePlugin{
//	            PluginName:    "slack",
//	            PluginVersion: "1.0.0",
//	        },
//	    }
//	}
//
//	func (p *SlackPlugin) Init(ctx context.Context, config map[string]any) error {
//	    url, ok := sdk.GetString(config, "webhook_url")
//	    if !ok {
//	        return sdk.ErrInvalidInput
//	    }
//	    p.webhookURL = url
//	    return nil
//	}
//
//	func (p *SlackPlugin) Handle(ctx context.Context, inputs map[string]any) (map[string]any, error) {
//	    message := sdk.GetStringDefault(inputs, "message", "")
//	    if message == "" {
//	        return sdk.NewErrorResult("message is required").ToMap(), nil
//	    }
//
//	    // Send message to Slack (implementation omitted)
//	    err := p.sendMessage(message)
//	    if err != nil {
//	        return sdk.NewErrorResult(err.Error()).ToMap(), nil
//	    }
//
//	    result := sdk.NewSuccessResult("Message sent", map[string]any{
//	        "timestamp": time.Now().Unix(),
//	    })
//	    return result.ToMap(), nil
//	}
//
// ## Plugin with Multiple Operations
//
//	type GithubPlugin struct {
//	    sdk.BasePlugin
//	    token string
//	}
//
//	func (p *GithubPlugin) Operations() []string {
//	    return []string{"github.create_issue", "github.list_repos"}
//	}
//
//	func (p *GithubPlugin) HandleOperation(ctx context.Context, name string, inputs map[string]any) (*sdk.OperationResult, error) {
//	    switch name {
//	    case "github.create_issue":
//	        return p.createIssue(ctx, inputs)
//	    case "github.list_repos":
//	        return p.listRepos(ctx, inputs)
//	    default:
//	        return sdk.NewErrorResult("unknown operation"), nil
//	    }
//	}
//
//	func (p *GithubPlugin) createIssue(ctx context.Context, inputs map[string]any) (*sdk.OperationResult, error) {
//	    title := sdk.GetStringDefault(inputs, "title", "")
//	    body := sdk.GetStringDefault(inputs, "body", "")
//
//	    // Create issue via GitHub API (implementation omitted)
//	    issueNumber, err := p.api.CreateIssue(title, body)
//	    if err != nil {
//	        return sdk.NewErrorResult(err.Error()), nil
//	    }
//
//	    return sdk.NewSuccessResult("Issue created", map[string]any{
//	        "issue_number": issueNumber,
//	    }), nil
//	}
//
// ## Operation Schema Declaration
//
//	func (p *SlackPlugin) Schema() sdk.OperationSchema {
//	    return sdk.OperationSchema{
//	        Name:        "slack.send",
//	        Description: "Send message to Slack channel",
//	        Inputs: map[string]sdk.InputSchema{
//	            "message": sdk.RequiredInput(sdk.InputTypeString, "Message text"),
//	            "channel": sdk.OptionalInput(sdk.InputTypeString, "Target channel", "#general"),
//	            "urgent":  sdk.OptionalInput(sdk.InputTypeBoolean, "Urgent notification", false),
//	        },
//	        Outputs: []string{"timestamp", "channel_id"},
//	    }
//	}
//
// ## Input Validation and Extraction
//
//	func (p *Plugin) Handle(ctx context.Context, inputs map[string]any) (map[string]any, error) {
//	    // Required string input
//	    name, ok := sdk.GetString(inputs, "name")
//	    if !ok {
//	        return sdk.NewErrorResult("name is required").ToMap(), nil
//	    }
//
//	    // Optional integer input with default
//	    count := sdk.GetIntDefault(inputs, "count", 10)
//
//	    // Optional boolean input
//	    verbose, _ := sdk.GetBool(inputs, "verbose")
//
//	    // Execute operation
//	    result := performOperation(name, count, verbose)
//
//	    return sdk.NewSuccessResult("Done", map[string]any{
//	        "processed": result,
//	    }).ToMap(), nil
//	}
//
// ## Error Handling
//
//	func (p *Plugin) Handle(ctx context.Context, inputs map[string]any) (map[string]any, error) {
//	    url, ok := sdk.GetString(inputs, "url")
//	    if !ok {
//	        // Invalid input - return error result
//	        return sdk.NewErrorResult("url is required").ToMap(), nil
//	    }
//
//	    data, err := fetchURL(url)
//	    if err != nil {
//	        // Operation failed - return error result with formatted message
//	        return sdk.NewErrorResultf("fetch failed: %v", err).ToMap(), nil
//	    }
//
//	    return sdk.NewSuccessResult("Fetched", map[string]any{
//	        "size": len(data),
//	    }).ToMap(), nil
//	}
//
// ## Graceful Shutdown
//
//	func (p *DatabasePlugin) Shutdown(ctx context.Context) error {
//	    // Close database connections
//	    if p.db != nil {
//	        return p.db.Close()
//	    }
//	    return nil
//	}
//
// # Workflow Integration
//
// Plugins are used in workflows via the operation step type:
//
//	steps:
//	  send_notification:
//	    type: operation
//	    operation: slack.send
//	    inputs:
//	      message: "Deployment completed"
//	      channel: "#alerts"
//	      urgent: true
//	    on_success: end
//
// Plugin output is available in subsequent steps:
//
//	{{states.send_notification.Data.timestamp}}
//	{{states.send_notification.Output}}
//
// # Testing Support
//
// ## MockPlugin (testing.go)
//
// MockPlugin provides a test double for plugin testing:
//   - Implements Plugin and OperationHandler interfaces
//   - Configurable responses for testing workflows
//   - Call tracking for verification
//
// Example usage:
//
//	mock := sdk.NewMockPlugin("test-plugin", "1.0.0")
//	mock.SetResponse("success output", map[string]any{"key": "value"})
//
//	result, err := mock.Handle(ctx, inputs)
//	// result contains configured response
//
// # Design Principles
//
// ## Public API Stability
//
// This is the stable plugin SDK for external developers:
//   - Semantic versioning for breaking changes
//   - Backward compatibility for minor/patch releases
//   - Clear deprecation warnings before removal
//
// ## Minimal Dependencies
//
// SDK has minimal external dependencies:
//   - Standard library only (context, errors)
//   - No AWF internal packages imported
//   - Plugin authors control their own dependencies
//
// ## Simple Interface
//
// Easy to implement, hard to misuse:
//   - Single Handle method for simple plugins
//   - Optional OperationProvider for multi-operation plugins
//   - BasePlugin provides sensible defaults
//   - Helper functions reduce boilerplate
//
// ## Type Safety
//
// Structured types for clarity:
//   - OperationResult enforces success/error pattern
//   - InputSchema declares expected types
//   - Helper functions handle type conversions
//
// # Related Documentation
//
// See also:
//   - internal/domain/operation: Domain operation interface and result types
//   - internal/infrastructure/pluginmgr: Plugin loader and registry implementation
//   - docs/plugin-development.md: Complete plugin development guide
//   - examples/plugins/: Example plugin implementations
package sdk
