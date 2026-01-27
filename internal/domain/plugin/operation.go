package plugin

// Valid input types for operation parameters.
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

// OperationSchema defines a plugin-provided operation.
type OperationSchema struct {
	Name        string                 // Operation name (e.g., "slack.send")
	Description string                 // Human-readable description
	Inputs      map[string]InputSchema // Input parameters
	Outputs     []string               // Output field names
	PluginName  string                 // Owning plugin name
}

// Validate checks if the operation schema is valid.
// TODO(#148): Implement validation logic.
func (o *OperationSchema) Validate() error {
	return ErrNotImplemented
}

// GetRequiredInputs returns a list of required input parameter names.
// TODO(#148): Implement this method.
func (o *OperationSchema) GetRequiredInputs() []string {
	return nil // stub
}

// InputSchema defines an input parameter for an operation.
type InputSchema struct {
	Type        string // "string", "integer", "boolean", "array", "object"
	Required    bool
	Default     any
	Description string
	Validation  string // Optional validation rule (e.g., "url", "email")
}

// Validate checks if the input schema is valid.
// TODO(#148): Implement validation logic.
func (i *InputSchema) Validate() error {
	return ErrNotImplemented
}

// IsValidType checks if the input type is a recognized type.
// TODO(#148): Implement this method.
func (i *InputSchema) IsValidType() bool {
	return false // stub
}

// OperationResult holds the result of executing a plugin operation.
type OperationResult struct {
	Success bool
	Outputs map[string]any
	Error   string
}

// IsSuccess returns true if the operation completed successfully.
func (r *OperationResult) IsSuccess() bool {
	return r.Success
}

// HasError returns true if the operation has an error message.
func (r *OperationResult) HasError() bool {
	return r.Error != ""
}

// GetOutput retrieves a specific output value by key.
func (r *OperationResult) GetOutput(key string) (any, bool) {
	if r.Outputs == nil {
		return nil, false
	}
	val, ok := r.Outputs[key]
	return val, ok
}
