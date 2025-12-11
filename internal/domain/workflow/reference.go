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
	"id":            true,
	"name":          true,
	"current_state": true,
	"started_at":    true,
	"duration":      true,
}

// ValidStateProperties lists known step state properties that can be referenced.
var ValidStateProperties = map[string]bool{
	"output":    true,
	"stderr":    true,
	"exit_code": true,
	"status":    true,
}

// ValidErrorProperties lists known error properties in error hooks.
var ValidErrorProperties = map[string]bool{
	"message":   true,
	"state":     true,
	"exit_code": true,
	"type":      true,
}

// ValidContextProperties lists known context properties.
var ValidContextProperties = map[string]bool{
	"working_dir": true,
	"user":        true,
	"hostname":    true,
}

// TemplateAnalyzer parses templates and extracts interpolation references.
type TemplateAnalyzer interface {
	// ExtractReferences parses a template string and returns all interpolation references.
	ExtractReferences(template string) ([]TemplateReference, error)
}
