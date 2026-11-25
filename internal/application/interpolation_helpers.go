package application

import (
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// processSnapshot holds the facts of the awf process that every interpolation
// context exposes: its environment, working directory, user and hostname.
//
// buildInterpolationContext runs several times per step. Reading these facts
// on each call copied the whole environment into a new map and made two
// system calls every time, which made it the first source of allocations of a
// workflow run. awf never changes its own environment or working directory
// while a workflow runs, so each executor reads them once, on first use.
//
// The zero value is ready to use. It must not be copied after first use.
type processSnapshot struct {
	once     sync.Once
	env      map[string]string
	wd       string
	user     string
	hostname string
}

// get reads the process facts on the first call and returns them afterwards.
func (p *processSnapshot) get() *processSnapshot {
	p.once.Do(func() {
		p.env = make(map[string]string)
		for _, e := range os.Environ() {
			if key, value, ok := strings.Cut(e, "="); ok {
				p.env[key] = value
			}
		}
		p.wd, _ = os.Getwd()
		p.hostname, _ = os.Hostname()
		p.user = os.Getenv("USER")
	})
	return p
}

// buildInterpolationContext converts ExecutionContext to interpolation.Context.
// Used by both ExecutionService and InteractiveExecutor to ensure consistent context building.
// awfPaths is injected via SetAWFPaths() to provide XDG directory variables (F063/T012).
// process holds the executor's snapshot of the process environment; the
// returned context's Env may be that snapshot's map, and must not be modified.
func buildInterpolationContext(
	execCtx *workflow.ExecutionContext,
	awfPaths map[string]string,
	process *processSnapshot,
) *interpolation.Context {
	// Convert step states - use thread-safe method.
	// Use explicit index iteration to avoid copying the large StepState struct.
	allStates := execCtx.GetAllStepStates()
	states := make(map[string]interpolation.StepStateData, len(allStates))
	for name := range allStates {
		state := allStates[name]
		states[name] = interpolation.StepStateData{
			Output:          state.Output,
			Stderr:          state.Stderr,
			ExitCode:        state.ExitCode,
			Status:          state.Status.String(),
			Response:        state.Response,
			TokensUsed:      state.TokensUsed,
			TokensInput:     state.TokensInput,
			TokensOutput:    state.TokensOutput,
			TokensEstimated: state.TokensEstimated,
			JSON:            state.JSON,
		}
	}

	proc := process.get()

	// Environment: the process's, overridden by the workflow's. Without workflow
	// variables, the snapshot is shared rather than copied: templates only read it.
	env := proc.env
	if len(execCtx.Env) > 0 {
		env = maps.Clone(proc.env)
		maps.Copy(env, execCtx.Env)
	}

	// Populate AWF context with XDG directory paths.
	// Paths are injected via SetAWFPaths() to avoid infrastructure import in application layer.
	awfContext := make(map[string]string, len(awfPaths)+1)
	for key, value := range awfPaths {
		awfContext[key] = value
	}
	awfContext[AWFPackNameKey] = execCtx.PackName

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
			WorkingDir: proc.wd,
			User:       proc.user,
			Hostname:   proc.hostname,
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
