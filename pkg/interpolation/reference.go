package interpolation

import "strings"

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

// Reference represents a parsed template interpolation reference.
type Reference struct {
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
	"Output":     true,
	"Stderr":     true,
	"ExitCode":   true,
	"Status":     true,
	"Response":   true,
	"TokensUsed": true,
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

// ExtractReferences parses a template string and extracts all interpolation references.
// Returns a slice of Reference structs for each {{...}} pattern found.
// Environment variable references ({{env.VAR}}) are included but do not cause validation errors.
func ExtractReferences(template string) ([]Reference, error) {
	if template == "" {
		return nil, nil
	}

	var refs []Reference
	remaining := template

	for {
		startIdx := strings.Index(remaining, "{{")
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(remaining[startIdx+2:], "}}")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx + 2

		content := remaining[startIdx+2 : endIdx]
		content = strings.TrimSpace(content)

		if content == "" {
			remaining = remaining[endIdx+2:]
			continue
		}

		ref := ParseReference(content)
		ref.Raw = remaining[startIdx : endIdx+2]
		refs = append(refs, ref)

		remaining = remaining[endIdx+2:]
	}

	return refs, nil
}

// ParseReference parses a single reference path (without braces) into a Reference struct.
// Handles Go template syntax with leading dot: ".states.step.output" -> "states.step.output".
func ParseReference(path string) Reference {
	if path == "" {
		return Reference{Type: TypeUnknown}
	}

	path = strings.TrimPrefix(path, ".")

	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return Reference{
			Type:      TypeUnknown,
			Namespace: path,
		}
	}

	namespace := parts[0]
	refType := CategorizeNamespace(namespace)

	ref := Reference{
		Type:      refType,
		Namespace: namespace,
	}

	switch refType {
	case TypeInputs:
		ref.Path = parts[1]
	case TypeStates:
		ref.Path = parts[1]
		if len(parts) >= 3 {
			ref.Property = parts[2]
		}
	case TypeWorkflow:
		ref.Path = parts[1]
	case TypeEnv:
		ref.Path = parts[1]
	case TypeError:
		ref.Path = parts[1]
	case TypeContext:
		ref.Path = parts[1]
	case TypeLoop:
		ref.Path = parts[1]
	default:
		ref.Path = strings.Join(parts[1:], ".")
	}

	return ref
}

// CategorizeNamespace determines the ReferenceType from the first path segment.
func CategorizeNamespace(namespace string) ReferenceType {
	switch namespace {
	case "inputs":
		return TypeInputs
	case "states":
		return TypeStates
	case "workflow":
		return TypeWorkflow
	case "env":
		return TypeEnv
	case "error":
		return TypeError
	case "context":
		return TypeContext
	case "loop":
		return TypeLoop
	default:
		return TypeUnknown
	}
}
