package analyzer

import (
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// InterpolationAnalyzer adapts pkg/interpolation to the workflow.TemplateAnalyzer interface.
type InterpolationAnalyzer struct{}

// NewInterpolationAnalyzer creates a new template analyzer.
func NewInterpolationAnalyzer() *InterpolationAnalyzer {
	return &InterpolationAnalyzer{}
}

// ExtractReferences parses a template string and returns all interpolation references.
func (a *InterpolationAnalyzer) ExtractReferences(template string) ([]workflow.TemplateReference, error) {
	refs, err := interpolation.ExtractReferences(template)
	if err != nil {
		return nil, err
	}

	result := make([]workflow.TemplateReference, len(refs))
	for i, ref := range refs {
		result[i] = workflow.TemplateReference{
			Type:      convertReferenceType(ref.Type),
			Namespace: ref.Namespace,
			Path:      ref.Path,
			Property:  ref.Property,
			Raw:       ref.Raw,
		}
	}

	return result, nil
}

// convertReferenceType maps interpolation.ReferenceType to workflow.ReferenceType.
func convertReferenceType(t interpolation.ReferenceType) workflow.ReferenceType {
	switch t {
	case interpolation.TypeInputs:
		return workflow.TypeInputs
	case interpolation.TypeStates:
		return workflow.TypeStates
	case interpolation.TypeWorkflow:
		return workflow.TypeWorkflow
	case interpolation.TypeEnv:
		return workflow.TypeEnv
	case interpolation.TypeError:
		return workflow.TypeError
	case interpolation.TypeContext:
		return workflow.TypeContext
	default:
		return workflow.TypeUnknown
	}
}

// Ensure InterpolationAnalyzer implements workflow.TemplateAnalyzer.
var _ workflow.TemplateAnalyzer = (*InterpolationAnalyzer)(nil)
