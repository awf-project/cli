package github

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
)

// BatchExecutor handles batch execution of GitHub operations with configurable
// concurrency and execution strategies (all_succeed, any_succeed, best_effort).
//
// Follows the proven errgroup + semaphore pattern from AWF's ParallelExecutor.
// See ADR-004 in the implementation plan for rationale.
type BatchExecutor struct {
	provider *GitHubOperationProvider
	logger   ports.Logger
}

// NewBatchExecutor creates a new batch executor for GitHub operations.
//
// Parameters:
//   - provider: GitHub operation provider for executing individual operations
//   - logger: structured logger for operation tracing
//
// Returns:
//   - *BatchExecutor: configured executor ready for batch processing
func NewBatchExecutor(provider *GitHubOperationProvider, logger ports.Logger) *BatchExecutor {
	return &BatchExecutor{
		provider: provider,
		logger:   logger,
	}
}

// BatchConfig defines configuration for batch execution.
type BatchConfig struct {
	// Strategy determines failure handling: "all_succeed", "any_succeed", "best_effort"
	Strategy string
	// MaxConcurrent limits parallel operation execution (default: 3)
	MaxConcurrent int
}

// BatchResult represents the aggregate result of batch execution.
type BatchResult struct {
	Total     int                            // Total operations attempted
	Succeeded int                            // Successfully completed operations
	Failed    int                            // Failed operations
	Results   []*pluginmodel.OperationResult // Individual operation results
}

// Execute runs multiple GitHub operations in batch according to the configured strategy.
//
// Parameters:
//   - ctx: context for cancellation and timeout
//   - operations: array of operation definitions (each contains name and inputs)
//   - config: batch execution configuration (strategy, concurrency)
//
// Returns:
//   - *BatchResult: aggregate results with success/failure counts
//   - error: returns error if strategy is "all_succeed" and any operation fails
//
// Strategies:
//   - all_succeed: cancel remaining on first failure (errgroup.WithContext)
//   - any_succeed: return success if at least one operation succeeds
//   - best_effort: complete all operations regardless of individual failures (default)
func (e *BatchExecutor) Execute(
	ctx context.Context,
	operations []map[string]any,
	config BatchConfig,
) (*BatchResult, error) {
	// Validate context
	if err := ctx.Err(); err != nil {
		return &BatchResult{
			Total:     len(operations),
			Succeeded: 0,
			Failed:    len(operations),
			Results:   []*pluginmodel.OperationResult{},
		}, fmt.Errorf("batch execution cancelled: %w", err)
	}

	// Initialize result
	result := &BatchResult{
		Total:     len(operations),
		Succeeded: 0,
		Failed:    0,
		Results:   make([]*pluginmodel.OperationResult, 0, len(operations)),
	}

	// Handle empty operations
	if len(operations) == 0 {
		return result, nil
	}

	// Normalize strategy
	strategy := config.Strategy
	if strategy == "" {
		strategy = "best_effort"
	}

	// Validate operations format
	for i, op := range operations {
		if err := e.validateOperation(op); err != nil {
			return result, fmt.Errorf("operation %d: %w", i, err)
		}
	}

	// Execute based on strategy
	var err error
	switch strategy {
	case "all_succeed":
		err = e.executeAllSucceed(ctx, operations, config, result)
	case "any_succeed":
		err = e.executeAnySucceed(ctx, operations, config, result)
	case "best_effort":
		err = e.executeBestEffort(ctx, operations, config, result)
	default:
		// Treat unknown strategy as best_effort
		err = e.executeBestEffort(ctx, operations, config, result)
	}

	return result, err
}

// validateOperation checks if an operation has required fields.
func (e *BatchExecutor) validateOperation(op map[string]any) error {
	// Check for "name" field
	nameRaw, hasName := op["name"]
	if !hasName {
		return fmt.Errorf("operation missing required field: name")
	}

	// Validate name is a string
	name, ok := nameRaw.(string)
	if !ok {
		return fmt.Errorf("operation name must be a string, got %T", nameRaw)
	}
	if name == "" {
		return fmt.Errorf("operation name cannot be empty")
	}

	// Check for "inputs" field (can be missing, but if present must be a map)
	if inputsRaw, hasInputs := op["inputs"]; hasInputs {
		if _, ok := inputsRaw.(map[string]any); !ok {
			return fmt.Errorf("operation inputs must be a map, got %T", inputsRaw)
		}
	}

	return nil
}

// executeAllSucceed runs all operations, cancels remaining on first failure.
func (e *BatchExecutor) executeAllSucceed(
	ctx context.Context,
	operations []map[string]any,
	config BatchConfig,
	result *BatchResult,
) error {
	g, gCtx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	sem := e.makeSemaphore(config.MaxConcurrent)

	for _, op := range operations {
		op := op
		g.Go(func() error {
			// Acquire semaphore
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-gCtx.Done():
					return gCtx.Err()
				}
			}

			// Execute operation
			opResult, err := e.executeOperation(gCtx, op)

			mu.Lock()
			defer mu.Unlock()

			result.Results = append(result.Results, opResult)

			if err != nil {
				result.Failed++
				return fmt.Errorf("operation %s failed: %w", op["name"], err)
			}

			result.Succeeded++
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("batch execution: %w", err)
	}
	return nil
}

// executeAnySucceed runs all operations, returns success when first one succeeds.
func (e *BatchExecutor) executeAnySucceed(
	ctx context.Context,
	operations []map[string]any,
	config BatchConfig,
	result *BatchResult,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := e.makeSemaphore(config.MaxConcurrent)

	successChan := make(chan struct{}, 1)
	var firstSuccess bool

	for _, op := range operations {
		op := op
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					result.Results = append(result.Results, &pluginmodel.OperationResult{
						Success: false,
						Error:   ctx.Err().Error(),
						Outputs: make(map[string]any),
					})
					result.Failed++
					mu.Unlock()
					return
				}
			}

			// Execute operation
			opResult, err := e.executeOperation(ctx, op)

			mu.Lock()
			defer mu.Unlock()

			result.Results = append(result.Results, opResult)

			if err != nil {
				result.Failed++
				return
			}

			result.Succeeded++

			// Check if this is the first success
			if !firstSuccess {
				firstSuccess = true
				cancel() // Cancel remaining operations

				// Non-blocking send to success channel
				select {
				case successChan <- struct{}{}:
				default:
				}
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

	if result.Succeeded > 0 {
		return nil
	}

	return fmt.Errorf("all %d operations failed", result.Total)
}

// executeBestEffort runs all operations and collects all results, never fails.
func (e *BatchExecutor) executeBestEffort(
	ctx context.Context,
	operations []map[string]any,
	config BatchConfig,
	result *BatchResult,
) error {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := e.makeSemaphore(config.MaxConcurrent)

	for _, op := range operations {
		op := op
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					result.Results = append(result.Results, &pluginmodel.OperationResult{
						Success: false,
						Error:   ctx.Err().Error(),
						Outputs: make(map[string]any),
					})
					result.Failed++
					mu.Unlock()
					return
				}
			}

			// Execute operation
			opResult, err := e.executeOperation(ctx, op)

			mu.Lock()
			defer mu.Unlock()

			result.Results = append(result.Results, opResult)

			if err != nil {
				result.Failed++
			} else {
				result.Succeeded++
			}
		}()
	}

	wg.Wait()
	return nil // best_effort never returns error
}

// executeOperation executes a single operation via the provider.
func (e *BatchExecutor) executeOperation(
	ctx context.Context,
	op map[string]any,
) (*pluginmodel.OperationResult, error) {
	// Extract operation name (already validated by validateOperation)
	name, ok := op["name"].(string)
	if !ok {
		return nil, fmt.Errorf("operation name must be a string")
	}

	// Extract inputs (may be nil)
	inputs, ok := op["inputs"].(map[string]any)
	if !ok {
		inputs = make(map[string]any)
	}

	// Execute via provider dispatch
	return e.provider.Execute(ctx, name, inputs)
}

// makeSemaphore creates a semaphore channel for concurrency limiting.
// Returns nil if maxConcurrent is 0 or negative (unlimited).
func (e *BatchExecutor) makeSemaphore(maxConcurrent int) chan struct{} {
	if maxConcurrent <= 0 {
		return nil
	}
	return make(chan struct{}, maxConcurrent)
}
