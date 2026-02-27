package application

import (
	"context"
	"fmt"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/interpolation"
)

// HookExecutor executes workflow and step hooks.
type HookExecutor struct {
	executor ports.CommandExecutor
	logger   ports.Logger
	resolver interpolation.Resolver
}

func NewHookExecutor(
	executor ports.CommandExecutor,
	logger ports.Logger,
	resolver interpolation.Resolver,
) *HookExecutor {
	return &HookExecutor{
		executor: executor,
		logger:   logger,
		resolver: resolver,
	}
}

// ExecuteHooks executes a list of hook actions sequentially.
// If failOnError is false, errors are logged but execution continues.
// If failOnError is true, execution stops on first error.
func (h *HookExecutor) ExecuteHooks(
	ctx context.Context,
	hook workflow.Hook,
	intCtx *interpolation.Context,
	failOnError bool,
) error {
	if len(hook) == 0 {
		return nil
	}

	for _, action := range hook {
		if err := h.executeAction(ctx, action, intCtx); err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("hook cancelled: %w", ctx.Err())
			}

			if failOnError {
				return err
			}
			h.logger.Warn("hook action failed", "error", err)
		}
	}

	return nil
}

func (h *HookExecutor) executeAction(
	ctx context.Context,
	action workflow.HookAction,
	intCtx *interpolation.Context,
) error {
	if action.Log != "" {
		return h.executeLogAction(action.Log, intCtx)
	}

	if action.Command != "" {
		return h.executeCommandAction(ctx, action.Command, intCtx)
	}

	return nil
}

func (h *HookExecutor) executeLogAction(msg string, intCtx *interpolation.Context) error {
	resolved, err := h.resolver.Resolve(msg, intCtx)
	if err != nil {
		return fmt.Errorf("interpolate log message: %w", err)
	}

	h.logger.Info(resolved)
	return nil
}

func (h *HookExecutor) executeCommandAction(
	ctx context.Context,
	command string,
	intCtx *interpolation.Context,
) error {
	resolved, err := h.resolver.Resolve(command, intCtx)
	if err != nil {
		return fmt.Errorf("interpolate command: %w", err)
	}

	h.logger.Debug("executing hook command", "command", resolved)

	cmd := &ports.Command{
		Program: resolved,
	}

	result, err := h.executor.Execute(ctx, cmd)
	if err != nil {
		return fmt.Errorf("execute hook command: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("hook command exited with code %d: %s", result.ExitCode, result.Stderr)
	}

	return nil
}
