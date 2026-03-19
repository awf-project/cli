package application

import (
	"os"
	"strings"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// buildInterpolationContext converts ExecutionContext to interpolation.Context.
// Used by both ExecutionService and InteractiveExecutor to ensure consistent context building.
// awfPaths is injected via SetAWFPaths() to provide XDG directory variables (F063/T012).
func buildInterpolationContext(
	execCtx *workflow.ExecutionContext,
	awfPaths map[string]string,
) *interpolation.Context {
	// Convert step states - use thread-safe method.
	// Use explicit index iteration to avoid copying the large StepState struct.
	allStates := execCtx.GetAllStepStates()
	states := make(map[string]interpolation.StepStateData, len(allStates))
	for name := range allStates {
		state := allStates[name]
		states[name] = interpolation.StepStateData{
			Output:     state.Output,
			Stderr:     state.Stderr,
			ExitCode:   state.ExitCode,
			Status:     state.Status.String(),
			Response:   state.Response,
			TokensUsed: state.TokensUsed,
			JSON:       state.JSON,
		}
	}

	// Get runtime context.
	wd, _ := os.Getwd()
	hostname, _ := os.Hostname()

	// Build environment map (merge os env first, then override with workflow-specific env).
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if parts := strings.SplitN(e, "=", 2); len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	for k, v := range execCtx.Env {
		env[k] = v
	}

	// Populate AWF context with XDG directory paths.
	// Paths are injected via SetAWFPaths() to avoid infrastructure import in application layer.
	awfContext := awfPaths
	if awfContext == nil {
		awfContext = map[string]string{}
	}

	intCtx := &interpolation.Context{
		Inputs: execCtx.Inputs,
		States: states,
		Workflow: interpolation.WorkflowData{
			ID:           execCtx.WorkflowID,
			Name:         execCtx.WorkflowName,
			CurrentState: execCtx.CurrentStep,
			StartedAt:    execCtx.StartedAt,
		},
		Env: env,
		Context: interpolation.ContextData{
			WorkingDir: wd,
			User:       os.Getenv("USER"),
			Hostname:   hostname,
		},
		Error: nil, // Set in error hooks (F008)
		AWF:   awfContext,
	}

	// Include loop context if we're inside a loop (with parent chain for nested loops).
	intCtx.Loop = buildLoopDataChain(execCtx.CurrentLoop)

	return intCtx
}

// interpolateTerminalMessage interpolates a terminal step message template.
// Returns the interpolated message, falling back to the raw template on error
// so the message is never silently lost.
func interpolateTerminalMessage(
	resolver interpolation.Resolver,
	logger ports.Logger,
	message string,
	intCtx *interpolation.Context,
) string {
	if message == "" {
		return ""
	}
	interpolated, err := resolver.Resolve(message, intCtx)
	if err != nil {
		logger.Warn("terminal message interpolation failed", "error", err, "message", message)
		return message
	}
	return interpolated
}
