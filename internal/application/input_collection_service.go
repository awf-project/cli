package application

import (
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// InputCollectionService coordinates interactive collection of missing workflow inputs.
// This service detects which required inputs are missing from command-line arguments
// and delegates to the InputCollector port for user interaction.
//
// Key responsibilities:
//   - Detect missing required inputs by comparing workflow definition with provided values
//   - Coordinate input collection via InputCollector port
//   - Merge collected inputs with provided inputs
//   - Handle optional inputs with default values
//
// Usage:
//
//	service := NewInputCollectionService(collector, logger)
//	allInputs, err := service.CollectMissingInputs(workflow, providedInputs)
//	if err != nil {
//	    return fmt.Errorf("input collection failed: %w", err)
//	}
type InputCollectionService struct {
	collector ports.InputCollector
	logger    ports.Logger
}

// NewInputCollectionService creates a new input collection service.
func NewInputCollectionService(
	collector ports.InputCollector,
	logger ports.Logger,
) *InputCollectionService {
	return &InputCollectionService{
		collector: collector,
		logger:    logger,
	}
}

// CollectMissingInputs detects missing required inputs and prompts the user interactively.
//
// Logic:
//   - Iterates through workflow.Inputs
//   - For each required input not in providedInputs: prompt user and collect value
//   - For each optional input not in providedInputs: prompt user (can skip with Enter)
//   - Returns merged map containing both provided and collected inputs
//
// Parameters:
//   - wf: Workflow definition containing input specifications
//   - providedInputs: Input values already provided via command-line flags
//
// Returns:
//   - Complete input map (provided + collected)
//   - Error if collection fails or user cancels
func (s *InputCollectionService) CollectMissingInputs(
	wf *workflow.Workflow,
	providedInputs map[string]any,
) (map[string]any, error) {
	// Handle nil workflow
	if wf == nil {
		return make(map[string]any), nil
	}

	// Create result map, copy provided inputs (do not mutate original)
	result := make(map[string]any)
	if providedInputs == nil {
		providedInputs = make(map[string]any)
	}
	for k, v := range providedInputs {
		result[k] = v
	}

	// Iterate workflow inputs
	for i := range wf.Inputs {
		input := &wf.Inputs[i]

		// Skip if already provided
		if _, exists := result[input.Name]; exists {
			continue
		}

		// Prompt for missing input (both required and optional)
		value, err := s.collector.PromptForInput(input)
		if err != nil {
			return nil, err
		}

		// Add collected value to result
		result[input.Name] = value
	}

	return result, nil
}
