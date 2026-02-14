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

type Plugin interface {
	Name() string
	Version() string
	Init(ctx context.Context, config map[string]any) error
	Shutdown(ctx context.Context) error
}

type OperationHandler interface {
	Handle(ctx context.Context, inputs map[string]any) (map[string]any, error)
}

type OperationProvider interface {
	Operations() []string
	HandleOperation(ctx context.Context, name string, inputs map[string]any) (*OperationResult, error)
}

// BasePlugin provides a minimal implementation for embedding in plugins.
type BasePlugin struct {
	PluginName    string
	PluginVersion string
}

func (p *BasePlugin) Name() string {
	return p.PluginName
}

func (p *BasePlugin) Version() string {
	return p.PluginVersion
}

func (p *BasePlugin) Init(_ context.Context, _ map[string]any) error {
	return nil
}

func (p *BasePlugin) Shutdown(_ context.Context) error {
	return nil
}

type OperationResult struct {
	Success bool
	Output  string
	Data    map[string]any
	Error   string
}

func NewSuccessResult(output string, data map[string]any) *OperationResult {
	return &OperationResult{
		Success: true,
		Output:  output,
		Data:    data,
	}
}

func NewErrorResult(errMsg string) *OperationResult {
	return &OperationResult{
		Success: false,
		Error:   errMsg,
	}
}

func NewErrorResultf(format string, args ...any) *OperationResult {
	return &OperationResult{
		Success: false,
		Error:   formatString(format, args...),
	}
}

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

type InputSchema struct {
	Type        string
	Required    bool
	Default     any
	Description string
	Validation  string
}

type OperationSchema struct {
	Name        string
	Description string
	Inputs      map[string]InputSchema
	Outputs     []string
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

func IsValidInputType(t string) bool {
	for _, valid := range ValidInputTypes {
		if t == valid {
			return true
		}
	}
	return false
}

func RequiredInput(inputType, description string) InputSchema {
	return InputSchema{
		Type:        inputType,
		Required:    true,
		Description: description,
	}
}

func OptionalInput(inputType, description string, defaultValue any) InputSchema {
	return InputSchema{
		Type:        inputType,
		Required:    false,
		Default:     defaultValue,
		Description: description,
	}
}

func GetString(inputs map[string]any, key string) (string, bool) {
	v, ok := inputs[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func GetStringDefault(inputs map[string]any, key, defaultValue string) string {
	s, ok := GetString(inputs, key)
	if !ok {
		return defaultValue
	}
	return s
}

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

func GetIntDefault(inputs map[string]any, key string, defaultValue int) int {
	n, ok := GetInt(inputs, key)
	if !ok {
		return defaultValue
	}
	return n
}

func GetBool(inputs map[string]any, key string) (value, ok bool) {
	v, exists := inputs[key]
	if !exists {
		return false, false
	}
	value, ok = v.(bool)
	return value, ok
}

func GetBoolDefault(inputs map[string]any, key string, defaultValue bool) bool {
	b, ok := GetBool(inputs, key)
	if !ok {
		return defaultValue
	}
	return b
}

func formatString(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return sprintfImpl(format, args...)
}

func sprintfImpl(format string, args ...any) string {
	result := format
	for _, arg := range args {
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

func replaceFirst(s, replacement string) string {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' && (s[i+1] == 'v' || s[i+1] == 's' || s[i+1] == 'd') {
			return s[:i] + replacement + s[i+2:]
		}
	}
	return s
}

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
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
