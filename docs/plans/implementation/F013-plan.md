# Implementation Plan: F013 - Workflow Resume

## Summary

Implement workflow resumption by adding a `Resume()` method to ExecutionService that loads persisted state, validates resumability, merges input overrides, and continues execution from `CurrentStep` while skipping completed steps. A new CLI command `awf resume` will expose this functionality with `--list` flag support. The existing execution loop will be refactored into a reusable `executeFromStep()` method shared by both `Run()` and `Resume()`.

## ASCII Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          RESUME EXECUTION FLOW                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  awf resume <workflow-id> --input key=override                              │
│       │                                                                     │
│       ▼                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ 1. Load persisted state from JSONStore.Load(workflowID)             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│       │                                                                     │
│       ▼                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ 2. Validate resumable:                                              │   │
│  │    - Status != completed                                            │   │
│  │    - Workflow definition still exists                               │   │
│  │    - CurrentStep exists in workflow                                 │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│       │                                                                     │
│       ▼                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ 3. Merge input overrides into execCtx.Inputs                        │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│       │                                                                     │
│       ▼                                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ 4. executeFromStep(ctx, wf, execCtx, execCtx.CurrentStep)           │   │
│  │    └─ Skip steps where States[name].Status == completed             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Implementation Steps

### Step 1: Refactor ExecutionService execution loop
- **File**: `internal/application/execution_service.go`
- **Action**: MODIFY
- **Changes**:
  1. Extract execution loop (lines 104-148) into private method:
     ```go
     func (s *ExecutionService) executeFromStep(
         ctx context.Context,
         wf *workflow.Workflow,
         execCtx *workflow.ExecutionContext,
         startStep string,
         skipCompleted bool,
     ) error
     ```
  2. Add step-skipping logic at loop start:
     ```go
     if skipCompleted {
         if state, exists := execCtx.States[currentStep]; exists && state.Status == workflow.StatusCompleted {
             currentStep = wf.Steps[currentStep].OnSuccess
             continue
         }
     }
     ```
  3. Modify `Run()` to call `executeFromStep(ctx, wf, execCtx, wf.Initial, false)`

### Step 2: Add Resume method to ExecutionService
- **File**: `internal/application/execution_service.go`
- **Action**: MODIFY
- **Changes**:
  Add after `Run()` method (~line 183):
  ```go
  // Resume continues an interrupted workflow execution.
  func (s *ExecutionService) Resume(
      ctx context.Context,
      workflowID string,
      inputOverrides map[string]any,
  ) (*workflow.ExecutionContext, error) {
      // 1. Load state
      execCtx, err := s.store.Load(ctx, workflowID)
      if err != nil {
          return nil, fmt.Errorf("load state: %w", err)
      }
      if execCtx == nil {
          return nil, fmt.Errorf("workflow execution not found: %s", workflowID)
      }
      
      // 2. Validate resumable
      if execCtx.Status == workflow.StatusCompleted {
          return nil, fmt.Errorf("workflow already completed, cannot resume")
      }
      
      // 3. Load workflow definition
      wf, err := s.workflowSvc.GetWorkflow(ctx, execCtx.WorkflowName)
      if err != nil {
          return nil, fmt.Errorf("load workflow: %w", err)
      }
      if wf == nil {
          return nil, fmt.Errorf("workflow '%s' not found", execCtx.WorkflowName)
      }
      
      // 4. Validate current step exists
      if _, exists := wf.Steps[execCtx.CurrentStep]; !exists {
          return nil, fmt.Errorf("cannot resume: step '%s' no longer exists", execCtx.CurrentStep)
      }
      
      // 5. Merge input overrides
      for k, v := range inputOverrides {
          execCtx.SetInput(k, v)
      }
      
      // 6. Reset status to running
      execCtx.Status = workflow.StatusRunning
      
      // 7. Execute from current step, skipping completed
      s.logger.Info("resuming workflow", "id", workflowID, "from", execCtx.CurrentStep)
      return s.executeFromStep(ctx, wf, execCtx, execCtx.CurrentStep, true)
  }
  ```

### Step 3: Add ListResumable method to ExecutionService
- **File**: `internal/application/execution_service.go`
- **Action**: MODIFY
- **Changes**:
  ```go
  // ListResumable returns all workflow executions that can be resumed.
  func (s *ExecutionService) ListResumable(ctx context.Context) ([]*workflow.ExecutionContext, error) {
      ids, err := s.store.List(ctx)
      if err != nil {
          return nil, err
      }
      
      var resumable []*workflow.ExecutionContext
      for _, id := range ids {
          execCtx, err := s.store.Load(ctx, id)
          if err != nil || execCtx == nil {
              continue
          }
          // Only include non-completed executions
          if execCtx.Status != workflow.StatusCompleted {
              resumable = append(resumable, execCtx)
          }
      }
      return resumable, nil
  }
  ```

### Step 4: Add ResumableInfo struct to UI output
- **File**: `internal/interfaces/cli/ui/output.go`
- **Action**: MODIFY
- **Changes**:
  Add after `ExecutionInfo` struct (~line 105):
  ```go
  // ResumableInfo contains information for resumable workflow display.
  type ResumableInfo struct {
      WorkflowID   string `json:"workflow_id"`
      WorkflowName string `json:"workflow_name"`
      Status       string `json:"status"`
      CurrentStep  string `json:"current_step"`
      UpdatedAt    string `json:"updated_at"`
      Progress     string `json:"progress"` // e.g., "3/5 steps completed"
  }
  ```

### Step 5: Add WriteResumableList to OutputWriter
- **File**: `internal/interfaces/cli/ui/output_writer.go`
- **Action**: MODIFY
- **Changes**:
  Add method:
  ```go
  // WriteResumableList outputs a list of resumable workflows.
  func (w *OutputWriter) WriteResumableList(infos []ResumableInfo) error {
      if w.format == FormatJSON {
          return w.writeJSON(infos)
      }
      if w.format == FormatTable {
          return w.writeResumableTable(infos)
      }
      return nil
  }
  
  func (w *OutputWriter) writeResumableTable(infos []ResumableInfo) error {
      if len(infos) == 0 {
          fmt.Fprintln(w.stdout, "No resumable workflows found")
          return nil
      }
      headers := []string{"ID", "WORKFLOW", "STATUS", "CURRENT STEP", "PROGRESS", "UPDATED"}
      rows := make([][]string, len(infos))
      for i, info := range infos {
          rows[i] = []string{info.WorkflowID, info.WorkflowName, info.Status, info.CurrentStep, info.Progress, info.UpdatedAt}
      }
      return w.renderTable(headers, rows)
  }
  ```

### Step 6: Create resume CLI command
- **File**: `internal/interfaces/cli/resume.go`
- **Action**: CREATE
- **Changes**:
  ```go
  package cli
  
  import (
      "context"
      "fmt"
      "os"
      "os/signal"
      "syscall"
      "time"
  
      "github.com/spf13/cobra"
      "github.com/vanoix/awf/internal/application"
      "github.com/vanoix/awf/internal/domain/workflow"
      "github.com/vanoix/awf/internal/infrastructure/executor"
      "github.com/vanoix/awf/internal/infrastructure/store"
      "github.com/vanoix/awf/internal/interfaces/cli/ui"
      "github.com/vanoix/awf/pkg/interpolation"
  )
  
  func newResumeCommand(cfg *Config) *cobra.Command {
      var listFlag bool
      var inputFlags []string
      var outputMode string
  
      cmd := &cobra.Command{
          Use:   "resume [workflow-id]",
          Short: "Resume an interrupted workflow",
          Long: `Resume a workflow execution from where it left off.
  
  The workflow-id is the unique identifier shown when running a workflow.
  Use --list to see all resumable workflows.
  Input overrides can be provided to change input values on resume.
  
  Examples:
    awf resume --list
    awf resume abc123-def456
    awf resume abc123-def456 --input max_tokens=5000`,
          Args: cobra.MaximumNArgs(1),
          RunE: func(cmd *cobra.Command, args []string) error {
              if listFlag {
                  return runResumeList(cmd, cfg)
              }
              if len(args) == 0 {
                  return fmt.Errorf("workflow-id required (use --list to see resumable workflows)")
              }
              if outputMode != "" {
                  mode, err := ParseOutputMode(outputMode)
                  if err != nil {
                      return err
                  }
                  cfg.OutputMode = mode
              }
              return runResume(cmd, cfg, args[0], inputFlags)
          },
      }
  
      cmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List resumable workflows")
      cmd.Flags().StringArrayVarP(&inputFlags, "input", "i", nil, "Override input parameter (key=value)")
      cmd.Flags().StringVarP(&outputMode, "output", "o", "silent", "Output mode: silent, streaming, buffered")
  
      return cmd
  }
  
  func runResumeList(cmd *cobra.Command, cfg *Config) error {
      ctx := context.Background()
      writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)
      
      stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
      
      // List all state IDs and filter resumable
      ids, err := stateStore.List(ctx)
      if err != nil {
          return fmt.Errorf("list states: %w", err)
      }
      
      var infos []ui.ResumableInfo
      for _, id := range ids {
          execCtx, err := stateStore.Load(ctx, id)
          if err != nil || execCtx == nil {
              continue
          }
          if execCtx.Status == workflow.StatusCompleted {
              continue
          }
          
          // Calculate progress
          completed := 0
          for _, state := range execCtx.States {
              if state.Status == workflow.StatusCompleted {
                  completed++
              }
          }
          progress := fmt.Sprintf("%d steps completed", completed)
          
          infos = append(infos, ui.ResumableInfo{
              WorkflowID:   execCtx.WorkflowID,
              WorkflowName: execCtx.WorkflowName,
              Status:       string(execCtx.Status),
              CurrentStep:  execCtx.CurrentStep,
              UpdatedAt:    execCtx.UpdatedAt.Format(time.RFC3339),
              Progress:     progress,
          })
      }
      
      // Output
      if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable {
          return writer.WriteResumableList(infos)
      }
      
      // Text format
      formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
          NoColor: cfg.NoColor,
      })
      
      if len(infos) == 0 {
          formatter.Info("No resumable workflows found")
          return nil
      }
      
      formatter.Printf("Resumable workflows:\n\n")
      for _, info := range infos {
          formatter.Printf("  %s\n", info.WorkflowID)
          formatter.Printf("    Workflow: %s\n", info.WorkflowName)
          formatter.Printf("    Status:   %s\n", info.Status)
          formatter.Printf("    Current:  %s\n", info.CurrentStep)
          formatter.Printf("    Progress: %s\n", info.Progress)
          formatter.Printf("    Updated:  %s\n\n", info.UpdatedAt)
      }
      
      return nil
  }
  
  func runResume(cmd *cobra.Command, cfg *Config, workflowID string, inputFlags []string) error {
      // Parse input overrides
      inputs, err := parseInputFlags(inputFlags)
      if err != nil {
          return fmt.Errorf("invalid input: %w", err)
      }
      
      // Create output components
      writer := ui.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.OutputFormat, cfg.NoColor)
      formatter := ui.NewFormatter(cmd.OutOrStdout(), ui.FormatOptions{
          Verbose: cfg.Verbose,
          Quiet:   cfg.Quiet,
          NoColor: cfg.NoColor,
      })
      
      // Setup streaming writers if needed
      var stdoutWriter, stderrWriter *ui.PrefixedWriter
      if !cfg.Quiet && cfg.OutputMode == OutputStreaming && !writer.IsJSONFormat() {
          colorizer := ui.NewColorizer(!cfg.NoColor)
          stdoutWriter = ui.NewPrefixedWriter(cmd.OutOrStdout(), ui.PrefixStdout, "", colorizer)
          stderrWriter = ui.NewPrefixedWriter(cmd.ErrOrStderr(), ui.PrefixStderr, "", colorizer)
      }
      
      // Context with signal handling
      ctx, cancel := context.WithCancel(context.Background())
      defer cancel()
      
      sigChan := make(chan os.Signal, 1)
      signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
      go func() {
          <-sigChan
          if cfg.OutputFormat != ui.FormatJSON && cfg.OutputFormat != ui.FormatTable {
              formatter.Warning("\nReceived interrupt signal, cancelling...")
          }
          cancel()
      }()
      
      // Initialize dependencies
      repo := NewWorkflowRepository()
      stateStore := store.NewJSONStore(cfg.StoragePath + "/states")
      shellExecutor := executor.NewShellExecutor()
      silentOutput := cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatTable
      logger := &cliLogger{
          formatter: formatter,
          verbose:   cfg.Verbose && !silentOutput,
          silent:    silentOutput,
      }
      resolver := interpolation.NewTemplateResolver()
      
      // Create services
      wfSvc := application.NewWorkflowService(repo, stateStore, shellExecutor, logger)
      parallelExecutor := application.NewParallelExecutor(logger)
      execSvc := application.NewExecutionService(wfSvc, shellExecutor, parallelExecutor, stateStore, logger, resolver)
      
      if stdoutWriter != nil {
          execSvc.SetOutputWriters(stdoutWriter, stderrWriter)
      }
      
      // Show start message
      if !silentOutput && cfg.OutputFormat != ui.FormatQuiet {
          formatter.Info(fmt.Sprintf("Resuming workflow: %s", workflowID))
      }
      startTime := time.Now()
      
      // Resume execution
      execCtx, execErr := execSvc.Resume(ctx, workflowID, inputs)
      
      // Flush streaming writers
      if stdoutWriter != nil {
          stdoutWriter.Flush()
      }
      if stderrWriter != nil {
          stderrWriter.Flush()
      }
      
      // Calculate duration
      durationMs := time.Since(startTime).Milliseconds()
      
      // Output result (same pattern as runWorkflow)
      if cfg.OutputFormat == ui.FormatJSON || cfg.OutputFormat == ui.FormatQuiet {
          result := ui.RunResult{
              Status:     "completed",
              DurationMs: durationMs,
          }
          if execCtx != nil {
              result.WorkflowID = execCtx.WorkflowID
              result.Status = string(execCtx.Status)
              if cfg.OutputMode == OutputBuffered {
                  result.Steps = buildStepInfos(execCtx)
              }
          }
          if execErr != nil {
              result.Status = "failed"
              result.Error = execErr.Error()
          }
          if err := writer.WriteRunResult(result); err != nil {
              return err
          }
          if execErr != nil {
              return &exitError{code: categorizeError(execErr), err: execErr}
          }
          return nil
      }
      
      // Table format
      if cfg.OutputFormat == ui.FormatTable {
          result := ui.RunResult{
              Status:     "completed",
              DurationMs: durationMs,
          }
          if execCtx != nil {
              result.WorkflowID = execCtx.WorkflowID
              result.Status = string(execCtx.Status)
              result.Steps = buildStepInfos(execCtx)
          }
          if execErr != nil {
              result.Status = "failed"
              result.Error = execErr.Error()
          }
          if err := writer.WriteRunResult(result); err != nil {
              return err
          }
          if execErr != nil {
              return &exitError{code: categorizeError(execErr), err: execErr}
          }
          return nil
      }
      
      // Text format
      duration := time.Since(startTime).Round(time.Millisecond)
      
      if execErr != nil {
          formatter.Error(fmt.Sprintf("Resume failed: %s", execErr))
          if execCtx != nil {
              formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))
          }
          if cfg.OutputMode == OutputBuffered && execCtx != nil {
              showStepOutputs(formatter, execCtx)
          }
          return &exitError{code: categorizeError(execErr), err: execErr}
      }
      
      if cfg.OutputMode != OutputBuffered && execCtx != nil {
          showEmptyStepFeedback(formatter, execCtx)
      }
      
      formatter.Success(fmt.Sprintf("Workflow resumed and completed in %s", duration))
      formatter.Info(fmt.Sprintf("Workflow ID: %s", execCtx.WorkflowID))
      
      if cfg.OutputMode == OutputBuffered && execCtx != nil {
          showStepOutputs(formatter, execCtx)
      }
      
      if cfg.Verbose && execCtx != nil {
          showExecutionDetails(formatter, execCtx)
      }
      
      return nil
  }
  ```

### Step 7: Register resume command
- **File**: `internal/interfaces/cli/root.go`
- **Action**: MODIFY
- **Changes**:
  Add at line ~92 (after `newValidateCommand`):
  ```go
  cmd.AddCommand(newResumeCommand(cfg))
  ```

## Test Plan

### Unit Tests

#### `internal/application/execution_service_test.go`
| Test Case | Description |
|-----------|-------------|
| `TestResume_Success` | Resume interrupted workflow, verify skips completed steps |
| `TestResume_NotFound` | Returns error when workflow ID doesn't exist |
| `TestResume_AlreadyCompleted` | Returns error when workflow already completed |
| `TestResume_WorkflowNotFound` | Returns error when workflow definition deleted |
| `TestResume_StepNotFound` | Returns error when current step no longer in workflow |
| `TestResume_InputOverrides` | Verify input overrides merge correctly |
| `TestResume_ParallelStep` | Resume with parallel step as current |
| `TestListResumable_FiltersCompleted` | Only returns non-completed executions |
| `TestListResumable_Empty` | Returns empty list when no states |

#### `internal/interfaces/cli/resume_test.go`
| Test Case | Description |
|-----------|-------------|
| `TestResumeCommand_ListFlag` | `--list` returns resumable workflows |
| `TestResumeCommand_NoArgs` | Error when no workflow-id and no --list |
| `TestResumeCommand_InputOverrides` | Parses `--input` flags correctly |
| `TestResumeCommand_OutputFormats` | JSON/table/text output formats work |

### Integration Tests

#### `tests/integration/resume_test.go`
| Test Case | Description |
|-----------|-------------|
| `TestResumeWorkflow_E2E` | Full flow: run→interrupt→resume→complete |
| `TestResumeList_E2E` | Create interrupted workflows, verify list shows them |
| `TestResumeWithOverrides_E2E` | Resume with different inputs, verify used |

## Files to Modify

| File | Action | Complexity | Lines Changed |
|------|--------|------------|---------------|
| `internal/application/execution_service.go` | MODIFY | M | ~100 |
| `internal/interfaces/cli/resume.go` | CREATE | M | ~200 |
| `internal/interfaces/cli/root.go` | MODIFY | S | ~1 |
| `internal/interfaces/cli/ui/output.go` | MODIFY | S | ~15 |
| `internal/interfaces/cli/ui/output_writer.go` | MODIFY | S | ~30 |
| `internal/application/execution_service_test.go` | MODIFY | M | ~150 |
| `internal/interfaces/cli/resume_test.go` | CREATE | M | ~100 |
| `tests/integration/resume_test.go` | CREATE | M | ~100 |

**Total estimated changes**: ~700 lines

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| **Workflow definition changed** | Medium | High | Validate `CurrentStep` exists in workflow before resuming. Return clear error. |
| **Partial parallel step** | Low | Medium | Clear branch results for current parallel step and re-execute all branches. |
| **State corruption during resume** | Low | High | Atomic writes via temp+rename already in JSONStore. Add validation on load. |
| **Race: concurrent resume** | Low | Medium | File locking in JSONStore prevents concurrent writes. First resume wins. |
| **Execution loop refactor breaks Run()** | Medium | High | Ensure comprehensive tests for `Run()` pass before/after refactor. |

## Implementation Order

1. **Step 1** (refactor execution loop) - Must be done first, highest risk
2. **Step 2-3** (Resume/ListResumable methods) - Core logic
3. **Step 4-5** (UI output structs) - Required for CLI
4. **Step 6** (resume.go CLI) - Brings it together
5. **Step 7** (register command) - Final wiring
6. **Tests** - Throughout, but especially after Step 1

