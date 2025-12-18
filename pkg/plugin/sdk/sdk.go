// Package sdk provides the public API for AWF plugin authors.
//
// This package contains interfaces and utilities that plugin developers
// use to create AWF-compatible plugins. Import this package to build
// plugins that integrate with AWF workflows.
//
// Example usage:
//
//	type MyPlugin struct {
//	    sdk.BasePlugin
//	}
//
//	func (p *MyPlugin) Init(ctx context.Context, config map[string]any) error {
//	    // Initialize your plugin
//	    return nil
//	}
//
//	// Implement OperationHandler for custom operations
//	func (p *MyPlugin) Handle(ctx context.Context, inputs map[string]any) (map[string]any, error) {
//	    result := sdk.NewSuccessResult("done", nil)
//	    return result.ToMap(), nil
//	}
package sdk

import (
	"context"
	"errors"
)

// Common errors for plugin implementations.
var (
	// ErrNotImplemented indicates a stub method that needs implementation.
	ErrNotImplemented = errors.New("not implemented")
	// ErrInvalidInput indicates invalid operation input.
	ErrInvalidInput = errors.New("invalid input")
	// ErrOperationFailed indicates the operation failed to execute.
	ErrOperationFailed = errors.New("operation failed")
)

// Plugin is the interface that AWF plugins must implement.
type Plugin interface {
	// Name returns the unique plugin identifier.
	Name() string
	// Version returns the plugin version (semantic versioning).
	Version() string
	// Init initializes the plugin with the provided configuration.
	Init(ctx context.Context, config map[string]any) error
	// Shutdown gracefully stops the plugin.
	Shutdown(ctx context.Context) error
}

// OperationHandler processes operation requests from AWF.
type OperationHandler interface {
	// Handle executes the operation with the given inputs.
	Handle(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

// OperationProvider allows plugins to register multiple operations.
type OperationProvider interface {
	// Operations returns the list of operation names this plugin provides.
	Operations() []string
	// HandleOperation executes the specified operation.
	HandleOperation(ctx context.Context, name string, inputs map[string]any) (*OperationResult, error)
}

// BasePlugin provides a minimal implementation for embedding in plugins.
type BasePlugin struct {
	PluginName    string
	PluginVersion string
}

// Name returns the plugin name.
func (p *BasePlugin) Name() string {
	return p.PluginName
}

// Version returns the plugin version.
func (p *BasePlugin) Version() string {
	return p.PluginVersion
}

// Init is a no-op initialization. Override in your plugin.
func (p *BasePlugin) Init(_ context.Context, _ map[string]any) error {
	return nil
}

// Shutdown is a no-op shutdown. Override in your plugin.
func (p *BasePlugin) Shutdown(_ context.Context) error {
	return nil
}

// OperationResult holds the result of executing a plugin operation.
type OperationResult struct {
	Success bool           // Whether the operation succeeded
	Output  string         // Human-readable output text
	Data    map[string]any // Structured output data
	Error   string         // Error message if failed
}

// NewSuccessResult creates a successful operation result.
func NewSuccessResult(output string, data map[string]any) *OperationResult {
	return &OperationResult{
		Success: true,
		Output:  output,
		Data:    data,
	}
}

// NewErrorResult creates a failed operation result.
func NewErrorResult(errMsg string) *OperationResult {
	return &OperationResult{
		Success: false,
		Error:   errMsg,
	}
}

// NewErrorResultf creates a failed operation result with formatted message.
func NewErrorResultf(format string, args ...any) *OperationResult {
	return &OperationResult{
		Success: false,
		Error:   formatString(format, args...),
	}
}

// ToMap converts the result to a map for handler return values.
func (r *OperationResult) ToMap() map[string]any {
	m := map[string]any{
		"success": r.Success,
		"output":  r.Output,
	}
	if r.Data != nil {
		m["data"] = r.Data
	}
	if r.Error != "" {
		m["error"] = r.Error
	}
	return m
}

// InputSchema defines an input parameter for an operation.
type InputSchema struct {
	Type        string // "string", "integer", "boolean", "array", "object"
	Required    bool
	Default     any
	Description string
	Validation  string // Optional validation rule (e.g., "url", "email")
}

// OperationSchema defines a plugin-provided operation.
type OperationSchema struct {
	Name        string                 // Operation name (e.g., "slack.send")
	Description string                 // Human-readable description
	Inputs      map[string]InputSchema // Input parameters
	Outputs     []string               // Output field names
}

// Input types for operation parameters.
const (
	InputTypeString  = "string"
	InputTypeInteger = "integer"
	InputTypeBoolean = "boolean"
	InputTypeArray   = "array"
	InputTypeObject  = "object"
)

// ValidInputTypes lists all recognized input types.
var ValidInputTypes = []string{
	InputTypeString,
	InputTypeInteger,
	InputTypeBoolean,
	InputTypeArray,
	InputTypeObject,
}

// IsValidInputType checks if the given type is a valid input type.
func IsValidInputType(t string) bool {
	for _, valid := range ValidInputTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// RequiredInput creates a required input schema.
func RequiredInput(inputType, description string) InputSchema {
	return InputSchema{
		Type:        inputType,
		Required:    true,
		Description: description,
	}
}

// OptionalInput creates an optional input schema with a default value.
func OptionalInput(inputType, description string, defaultValue any) InputSchema {
	return InputSchema{
		Type:        inputType,
		Required:    false,
		Default:     defaultValue,
		Description: description,
	}
}

// GetString extracts a string value from inputs.
func GetString(inputs map[string]any, key string) (string, bool) {
	v, ok := inputs[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetStringDefault extracts a string value with default.
func GetStringDefault(inputs map[string]any, key, defaultValue string) string {
	s, ok := GetString(inputs, key)
	if !ok {
		return defaultValue
	}
	return s
}

// GetInt extracts an integer value from inputs.
func GetInt(inputs map[string]any, key string) (int, bool) {
	v, ok := inputs[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// GetIntDefault extracts an integer value with default.
func GetIntDefault(inputs map[string]any, key string, defaultValue int) int {
	n, ok := GetInt(inputs, key)
	if !ok {
		return defaultValue
	}
	return n
}

// GetBool extracts a boolean value from inputs.
func GetBool(inputs map[string]any, key string) (bool, bool) {
	v, ok := inputs[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// GetBoolDefault extracts a boolean value with default.
func GetBoolDefault(inputs map[string]any, key string, defaultValue bool) bool {
	b, ok := GetBool(inputs, key)
	if !ok {
		return defaultValue
	}
	return b
}

// formatString is a simple string formatter for error messages.
func formatString(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	// Use fmt.Sprintf for formatting
	return sprintfImpl(format, args...)
}

// sprintfImpl wraps fmt.Sprintf without importing fmt at top level.
func sprintfImpl(format string, args ...any) string {
	// We need to import fmt, but to avoid circular issues in some cases,
	// we can use a simple implementation for common cases
	result := format
	for _, arg := range args {
		// Simple substitution - replace first %v, %s, %d with value
		switch v := arg.(type) {
		case string:
			result = replaceFirst(result, v)
		case int:
			result = replaceFirst(result, intToString(v))
		case error:
			result = replaceFirst(result, v.Error())
		default:
			result = replaceFirst(result, "<value>")
		}
	}
	return result
}

// replaceFirst replaces first format placeholder with value.
func replaceFirst(s, replacement string) string {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' && (s[i+1] == 'v' || s[i+1] == 's' || s[i+1] == 'd') {
			return s[:i] + replacement + s[i+2:]
		}
	}
	return s
}

// intToString converts int to string without fmt.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
