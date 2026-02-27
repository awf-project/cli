package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/workflow"
)

// ParallelExecutor executes parallel branches using errgroup with semaphore.
type ParallelExecutor struct {
	logger ports.Logger
}

// NewParallelExecutor creates a new ParallelExecutor.
func NewParallelExecutor(logger ports.Logger) *ParallelExecutor {
	return &ParallelExecutor{
		logger: logger,
	}
}

// Execute runs multiple branches concurrently according to the given strategy.
func (e *ParallelExecutor) Execute(
	ctx context.Context,
	wf *workflow.Workflow,
	branches []string,
	config workflow.ParallelConfig,
	execCtx *workflow.ExecutionContext,
	stepExecutor ports.StepExecutor,
) (*workflow.ParallelResult, error) {
	result := workflow.NewParallelResult()

	// Check for already cancelled context
	if err := ctx.Err(); err != nil {
		result.CompletedAt = time.Now()
		return result, fmt.Errorf("parallel execution cancelled: %w", err)
	}

	// Handle empty branches
	if len(branches) == 0 {
		result.CompletedAt = time.Now()
		return result, nil
	}

	// Normalize strategy
	strategy := config.Strategy
	if strategy == "" {
		strategy = workflow.DefaultParallelStrategy
	}

	// Execute based on strategy
	var err error
	switch strategy {
	case workflow.StrategyAllSucceed:
		err = e.executeAllSucceed(ctx, wf, branches, config, execCtx, stepExecutor, result)
	case workflow.StrategyAnySucceed:
		err = e.executeAnySucceed(ctx, wf, branches, config, execCtx, stepExecutor, result)
	case workflow.StrategyBestEffort:
		err = e.executeBestEffort(ctx, wf, branches, config, execCtx, stepExecutor, result)
	default:
		// Default to all_succeed
		err = e.executeAllSucceed(ctx, wf, branches, config, execCtx, stepExecutor, result)
	}

	result.CompletedAt = time.Now()
	return result, err
}

// executeAllSucceed runs all branches, cancels remaining on first failure.
func (e *ParallelExecutor) executeAllSucceed(
	ctx context.Context,
	wf *workflow.Workflow,
	branches []string,
	config workflow.ParallelConfig,
	execCtx *workflow.ExecutionContext,
	stepExecutor ports.StepExecutor,
	result *workflow.ParallelResult,
) error {
	g, gCtx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	sem := e.makeSemaphore(config.MaxConcurrent)

	for _, branch := range branches {
		branch := branch
		g.Go(func() error {
			startTime := time.Now()
			// Acquire semaphore
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-gCtx.Done():
					return gCtx.Err()
				}
			}

			branchResult, err := stepExecutor.ExecuteStep(gCtx, wf, branch, execCtx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.AddResult(&workflow.BranchResult{
					Name:        branch,
					Error:       err,
					StartedAt:   startTime,
					CompletedAt: time.Now(),
				})
				return err
			}

			result.AddResult(branchResult)

			// Check if the branch failed (non-zero exit code)
			if !branchResult.Success() {
				err := fmt.Errorf("branch %s failed with exit code %d", branch, branchResult.ExitCode)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("parallel execution: %w", err)
	}
	return nil
}

// executeAnySucceed runs all branches, returns success when first one succeeds.
func (e *ParallelExecutor) executeAnySucceed(
	ctx context.Context,
	wf *workflow.Workflow,
	branches []string,
	config workflow.ParallelConfig,
	execCtx *workflow.ExecutionContext,
	stepExecutor ports.StepExecutor,
	result *workflow.ParallelResult,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := e.makeSemaphore(config.MaxConcurrent)

	successChan := make(chan struct{}, 1)
	var firstSuccess bool

	for _, branch := range branches {
		branch := branch
		wg.Add(1)
		go func() {
			defer wg.Done()

			branchResult, err := e.RunBranchWithSemaphore(ctx, branch, sem, stepExecutor, wf, execCtx, result, &mu)

			// Check if branch succeeded and trigger cancellation
			if err == nil && branchResult != nil {
				e.CheckBranchSuccess(branchResult, &firstSuccess, &mu, successChan, cancel)
			}
		}()
	}

	// Wait for either success or all to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-successChan:
		// At least one succeeded, wait for remaining to be cancelled
		<-done
	case <-done:
		// All completed
	}

	if result.AnySucceeded() {
		return nil
	}

	if result.FirstError != nil {
		return result.FirstError
	}

	return errors.New("all branches failed")
}

// RunBranchWithSemaphore executes a single branch with semaphore control.
// It acquires the semaphore, executes the branch, and adds the result to the parallel result.
// Returns the branch result and any error encountered during execution.
//
// NOTE: Exported temporarily for T007 testing. Will be made private after TDD cycle completes.
func (e *ParallelExecutor) RunBranchWithSemaphore(
	ctx context.Context,
	branch string,
	sem chan struct{},
	stepExecutor ports.StepExecutor,
	wf *workflow.Workflow,
	execCtx *workflow.ExecutionContext,
	result *workflow.ParallelResult,
	mu *sync.Mutex,
) (*workflow.BranchResult, error) {
	// Acquire semaphore
	if sem != nil {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	branchResult, err := stepExecutor.ExecuteStep(ctx, wf, branch, execCtx)
	if err != nil {
		// If ExecuteStep returned a result with an error, add it
		if branchResult != nil {
			mu.Lock()
			defer mu.Unlock()
			result.AddResult(branchResult)
			return branchResult, err
		}

		// ExecuteStep returned nil result (e.g., cancelled before execution started)
		// Don't create an artificial error result, just propagate the error
		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	result.AddResult(branchResult)
	return branchResult, nil
}

// CheckBranchSuccess checks if a branch succeeded and signals cancellation if it's the first success.
// Handles synchronization for success detection in any_succeed strategy.
//
// NOTE: Exported temporarily for T008 testing. Will be made private after TDD cycle completes.
func (e *ParallelExecutor) CheckBranchSuccess(
	branchResult *workflow.BranchResult,
	firstSuccess *bool,
	mu *sync.Mutex,
	successChan chan struct{},
	cancel context.CancelFunc,
) {
	// Handle nil branch result safely
	if branchResult == nil {
		return
	}

	// Thread-safe check: only proceed if branch succeeded AND we haven't succeeded yet
	mu.Lock()
	defer mu.Unlock()

	// If already succeeded, do nothing (idempotent)
	if *firstSuccess {
		return
	}

	// Check if this branch succeeded (exit code 0 and no error)
	if branchResult.Success() {
		// Mark as first success
		*firstSuccess = true

		// Cancel remaining branches
		cancel()

		// Non-blocking send to success channel (default case prevents deadlock)
		select {
		case successChan <- struct{}{}:
			// Signal sent successfully
		default:
			// Channel full, skip sending (first signal already there)
		}
	}
}

// executeBestEffort runs all branches and collects all results, never fails.
func (e *ParallelExecutor) executeBestEffort(
	ctx context.Context,
	wf *workflow.Workflow,
	branches []string,
	config workflow.ParallelConfig,
	execCtx *workflow.ExecutionContext,
	stepExecutor ports.StepExecutor,
	result *workflow.ParallelResult,
) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := e.makeSemaphore(config.MaxConcurrent)

	for _, branch := range branches {
		branch := branch
		wg.Add(1)
		go func() {
			defer wg.Done()
			startTime := time.Now()

			// Acquire semaphore
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					result.AddResult(&workflow.BranchResult{
						Name:        branch,
						Error:       ctx.Err(),
						StartedAt:   startTime,
						CompletedAt: time.Now(),
					})
					mu.Unlock()
					return
				}
			}

			branchResult, err := stepExecutor.ExecuteStep(ctx, wf, branch, execCtx)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.AddResult(&workflow.BranchResult{
					Name:        branch,
					Error:       err,
					StartedAt:   startTime,
					CompletedAt: time.Now(),
				})
				return
			}

			result.AddResult(branchResult)
		}()
	}

	wg.Wait()
	return nil // best_effort never returns error
}

// makeSemaphore creates a semaphore channel for concurrency limiting.
// Returns nil if maxConcurrent is 0 (unlimited).
func (e *ParallelExecutor) makeSemaphore(maxConcurrent int) chan struct{} {
	if maxConcurrent <= 0 {
		return nil
	}
	return make(chan struct{}, maxConcurrent)
}

// Verify interface compliance at compile time.
var _ ports.ParallelExecutor = (*ParallelExecutor)(nil)
