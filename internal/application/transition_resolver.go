package application

import (
	"fmt"

	"github.com/awf-project/awf/internal/domain/ports"
	"github.com/awf-project/awf/internal/domain/workflow"
	"github.com/awf-project/awf/pkg/interpolation"
)

// resolveNextStep evaluates transitions for a step and returns the next step name.
// If transitions are defined and an evaluator is available, it evaluates them first.
// If no transition matches, it falls back to legacy OnSuccess/OnFailure fields.
func resolveNextStep(
	step *workflow.Step,
	intCtx *interpolation.Context,
	success bool,
	evaluator ports.ExpressionEvaluator,
	logger ports.Logger,
) (string, error) {
	if len(step.Transitions) > 0 && evaluator != nil {
		evalFunc := func(expr string) (bool, error) {
			return evaluator.EvaluateBool(expr, intCtx)
		}

		nextStep, found, err := step.Transitions.EvaluateFirstMatch(evalFunc)
		if err != nil {
			return "", fmt.Errorf("evaluate transitions: %w", err)
		}
		if found {
			logger.Debug("transition matched", "step", step.Name, "next", nextStep)
			return nextStep, nil
		}
		logger.Debug("no transition matched, using legacy", "step", step.Name)
	}

	// Legacy fallback: use OnSuccess/OnFailure
	if success {
		return step.OnSuccess, nil
	}
	return step.OnFailure, nil
}
