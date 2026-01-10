package application

import (
	"context"
	"fmt"

	"github.com/vanoix/awf/internal/domain/ports"
	"github.com/vanoix/awf/internal/domain/workflow"
	"github.com/vanoix/awf/pkg/interpolation"
)

// HookExecutor executes workflow and step hooks.
type HookExecutor struct {
	executor    ports.CommandExecutor
	logger      ports.Logger
	resolver    interpolation.Resolver
	failOnError bool
}

// NewHookExecutor creates a new hook executor.
func NewHookExecutor(
	executor ports.CommandExecutor,
	logger ports.Logger,
	resolver interpolation.Resolver,
) *HookExecutor {
	return &HookExecutor{
		executor:    executor,
		logger:      logger,
		resolver:    resolver,
		failOnError: false,
	}
}

// SetFailOnError configures whether hook failures should stop execution.
func (h *HookExecutor) SetFailOnError(fail bool) {
	h.failOnError = fail
}

// ExecuteHooks executes a list of hook actions sequentially.
// By default, errors are logged but execution continues.
// If failOnError is true, execution stops on first error.
func (h *HookExecutor) ExecuteHooks(
	ctx context.Context,
	hook workflow.Hook,
	intCtx *interpolation.Context,
) error {
	if len(hook) == 0 {
		return nil
	}

	for _, action := range hook {
		if err := h.executeAction(ctx, action, intCtx); err != nil {
			// Context cancellation should propagate immediately
			if ctx.Err() != nil {
				return fmt.Errorf("hook cancelled: %w", ctx.Err())
			}

			if h.failOnError {
				return err
			}
			// Log warning and continue
			h.logger.Warn("hook action failed", "error", err)
		}
	}

	return nil
}

// executeAction executes a single hook action.
func (h *HookExecutor) executeAction(
	ctx context.Context,
	action workflow.HookAction,
	intCtx *interpolation.Context,
) error {
	// Handle log action
	if action.Log != "" {
		return h.executeLogAction(action.Log, intCtx)
	}

	// Handle command action
	if action.Command != "" {
		return h.executeCommandAction(ctx, action.Command, intCtx)
	}

	return nil
}

// executeLogAction interpolates and logs a message.
func (h *HookExecutor) executeLogAction(msg string, intCtx *interpolation.Context) error {
	resolved, err := h.resolver.Resolve(msg, intCtx)
	if err != nil {
		return fmt.Errorf("interpolate log message: %w", err)
	}

	h.logger.Info(resolved)
	return nil
}

// executeCommandAction interpolates and executes a shell command.
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
