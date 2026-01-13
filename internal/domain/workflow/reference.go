package workflow

// ReferenceType categorizes the namespace of a template reference.
type ReferenceType string

const (
	// TypeInputs references workflow input parameters ({{inputs.name}}).
	TypeInputs ReferenceType = "inputs"
	// TypeStates references step output data ({{states.step.output}}).
	TypeStates ReferenceType = "states"
	// TypeWorkflow references workflow metadata ({{workflow.id}}).
	TypeWorkflow ReferenceType = "workflow"
	// TypeEnv references environment variables ({{env.VAR}}).
	TypeEnv ReferenceType = "env"
	// TypeError references error data in error hooks ({{error.message}}).
	TypeError ReferenceType = "error"
	// TypeContext references runtime context ({{context.working_dir}}).
	TypeContext ReferenceType = "context"
	// TypeLoop references loop runtime data ({{loop.Index}}).
	TypeLoop ReferenceType = "loop"
	// TypeUnknown for unrecognized namespaces.
	TypeUnknown ReferenceType = "unknown"
)

// TemplateReference represents a parsed template interpolation reference.
type TemplateReference struct {
	Type      ReferenceType // namespace type (inputs, states, etc.)
	Namespace string        // first path segment (e.g., "inputs")
	Path      string        // full dot-separated path (e.g., "name" for inputs.name)
	Property  string        // property being accessed (e.g., "output" for states.step.output)
	Raw       string        // original template string (e.g., "{{inputs.name}}")
}

// ValidWorkflowProperties lists known workflow properties that can be referenced.
var ValidWorkflowProperties = map[string]bool{
	"ID":           true,
	"Name":         true,
	"CurrentState": true,
	"StartedAt":    true,
	"Duration":     true,
}

// ValidStateProperties lists known step state properties that can be referenced.
var ValidStateProperties = map[string]bool{
	"Output":   true,
	"Stderr":   true,
	"ExitCode": true,
	"Status":   true,
	"Response": true,
	"Tokens":   true,
}

// lowercaseToUppercase maps lowercase property names to their correct uppercase equivalents.
// Used to provide actionable error messages when users use incorrect casing.
var lowercaseToUppercase = map[string]string{
	"output":    "Output",
	"stderr":    "Stderr",
	"exit_code": "ExitCode",
	"status":    "Status",
	"response":  "Response",
	"tokens":    "Tokens",
}

// lowercaseToUppercaseError maps lowercase error property names to their correct uppercase equivalents.
var lowercaseToUppercaseError = map[string]string{
	"message":   "Message",
	"state":     "State",
	"exit_code": "ExitCode",
	"type":      "Type",
}

// lowercaseToUppercaseContext maps lowercase context property names to their correct uppercase equivalents.
var lowercaseToUppercaseContext = map[string]string{
	"working_dir": "WorkingDir",
	"user":        "User",
	"hostname":    "Hostname",
}

// lowercaseToUppercaseWorkflow maps lowercase workflow property names to their correct uppercase equivalents.
var lowercaseToUppercaseWorkflow = map[string]string{
	"id":            "ID",
	"name":          "Name",
	"current_state": "CurrentState",
	"started_at":    "StartedAt",
	"duration":      "Duration",
}

// ValidErrorProperties lists known error properties in error hooks.
var ValidErrorProperties = map[string]bool{
	"Message":  true,
	"State":    true,
	"ExitCode": true,
	"Type":     true,
}

// ValidContextProperties lists known context properties.
var ValidContextProperties = map[string]bool{
	"WorkingDir": true,
	"User":       true,
	"Hostname":   true,
}

// ValidLoopProperties lists known loop properties accessible during loop iteration.
var ValidLoopProperties = map[string]bool{
	"Item":   true,
	"Index":  true,
	"Index1": true, // 1-based index computed via Index1() method
	"First":  true,
	"Last":   true,
	"Length": true,
	"Parent": true, // nested loop parent reference
}

// TemplateAnalyzer parses templates and extracts interpolation references.
type TemplateAnalyzer interface {
	// ExtractReferences parses a template string and returns all interpolation references.
	ExtractReferences(template string) ([]TemplateReference, error)
}
