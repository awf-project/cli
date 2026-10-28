package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/awf-project/cli/internal/application/tools"
	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/pkg/display"
	"github.com/awf-project/cli/pkg/interpolation"
	"github.com/awf-project/cli/pkg/output"
	"github.com/awf-project/cli/pkg/retry"
	"github.com/awf-project/cli/pkg/validation"
	"github.com/google/uuid"
)

// PackWorkflowLoader loads a workflow from an installed pack.
// Returns the loaded workflow, the resolved pack directory, and an error.
// Note: duplicates resolvePackWorkflow in internal/interfaces/cli/pack_resolver.go
// — cross-layer import (application→interfaces) is forbidden by go-arch-lint.
type PackWorkflowLoader func(ctx context.Context, packName, workflowName string) (*workflow.Workflow, string, error)

// ConversationExecutor defines the interface for executing multi-turn conversations.
// This interface allows for dependency injection and testing with mocks.
type ConversationExecutor interface {
	ExecuteConversation(
		ctx context.Context,
		step *workflow.Step,
		config *workflow.ConversationConfig,
		execCtx *workflow.ExecutionContext,
		buildContext ContextBuilderFunc,
		workflowDir string,
		stdoutW, stderrW io.Writer,
	) (*workflow.ConversationResult, error)
}

// ExecutionService orchestrates workflow execution.
type ExecutionService struct {
	workflowSvc            *WorkflowService
	executor               ports.CommandExecutor
	parallelExecutor       ports.ParallelExecutor
	store                  ports.StateStore
	logger                 ports.Logger
	resolver               interpolation.Resolver
	evaluator              ports.ExpressionEvaluator
	hookExecutor           *HookExecutor
	loopExecutor           *LoopExecutor
	stdoutWriter           io.Writer
	stderrWriter           io.Writer
	displayRendererFactory func(stepID string) display.EventRenderer
	historySvc             *HistoryService
	templateSvc            *TemplateService
	operationProvider      ports.OperationProvider
	agentRegistry          ports.AgentRegistry
	pluginSvc              *PluginService
	stepTypeProvider       ports.StepTypeProvider
	conversationMgr        ConversationExecutor
	outputLimiter          *OutputLimiter
	awfPaths               map[string]string
	auditTrailWriter       ports.AuditTrailWriter
	packWorkflowLoader     PackWorkflowLoader
	tracer                 ports.Tracer
	eventPublisher         ports.EventPublisher
	skillRepo              ports.SkillRepository
	agentRoleRepo          ports.AgentRoleRepository
	toolProxy              *tools.ProxyService
	recorder               ports.Recorder
	recorderFactory        ports.RecorderFactory
	transcriptDir          string
	agentOutputNormalizer  ports.AgentOutputNormalizer
	resumeValidationMu     sync.Mutex
}

type terminalOutcome struct {
	err               error
	cancellationCause error
	errorMessage      string
}

type terminalFinalizer struct {
	finalized bool
}

var (
	errHistoryFinalizationInProgress = errors.New("history finalization already in progress")
	errResumeInProgress              = errors.New("workflow resume already in progress")
	resumeLifecycleMu                sync.Mutex
	activeResumes                    = make(map[string]struct{})
)

func tryBeginResume(workflowID string) bool {
	resumeLifecycleMu.Lock()
	defer resumeLifecycleMu.Unlock()
	if _, active := activeResumes[workflowID]; active {
		return false
	}
	activeResumes[workflowID] = struct{}{}
	return true
}

func finishResume(workflowID string) {
	resumeLifecycleMu.Lock()
	defer resumeLifecycleMu.Unlock()
	delete(activeResumes, workflowID)
}

func (s *ExecutionService) finalizeAttemptHistory(
	ctx context.Context,
	execCtx *workflow.ExecutionContext,
	errorMessage string,
	prepareTerminalSnapshot func(),
) error {
	if !execCtx.TryBeginHistoryFinalization() {
		_, _, _, recorded := execCtx.GetHistoryAttempt()
		if !recorded {
			return errHistoryFinalizationInProgress
		}
		return nil
	}

	if prepareTerminalSnapshot != nil {
		prepareTerminalSnapshot()
	}
	if execCtx.TryBeginCompletionAudit() {
		s.emitAuditCompleted(ctx, execCtx, errorMessage)
	}

	recordID, attempt, resumedFromID, _ := execCtx.GetHistoryAttempt()
	if !s.historySvc.IsEnabled() {
		// The terminal snapshot is the workflow state source of truth even when
		// optional history projection is disabled. Keep resume non-blocking and
		// omit history diagnostics in this mode, as there is no history record to
		// claim or retry.
		if err := s.store.Save(ctx, execCtx); err != nil {
			s.logger.Debug("terminal snapshot save failed while history disabled",
				"workflow_id", execCtx.WorkflowID,
				"history_record_id", recordID,
				"attempt", attempt,
			)
		}
		execCtx.FinishHistoryFinalization(false)
		return nil
	}

	if err := s.store.Save(ctx, execCtx); err != nil {
		execCtx.FinishHistoryFinalization(false)
		s.warnHistoryPersistenceFailure(execCtx, recordID, attempt)
		return fmt.Errorf("persist terminal history snapshot: %w", err)
	}

	record := buildExecutionRecord(execCtx, recordID, attempt, resumedFromID)
	if err := s.historySvc.Record(ctx, record); err != nil {
		execCtx.FinishHistoryFinalization(false)
		s.warnHistoryPersistenceFailure(execCtx, recordID, attempt)
		return fmt.Errorf("persist execution history: %w", err)
	}

	execCtx.FinishHistoryFinalization(true)
	if err := s.store.Save(ctx, execCtx); err != nil {
		s.logger.Warn("history recorded marker save failed",
			"workflow_id", execCtx.WorkflowID,
			"history_record_id", recordID,
			"attempt", attempt,
		)
	}
	return nil
}

func (s *ExecutionService) warnHistoryPersistenceFailure(execCtx *workflow.ExecutionContext, recordID string, attempt int) {
	s.logger.Warn("history persistence failed",
		"workflow_id", execCtx.WorkflowID,
		"history_record_id", recordID,
		"attempt", attempt,
	)
}

func buildExecutionRecord(execCtx *workflow.ExecutionContext, recordID string, attempt int, resumedFromID string) *workflow.ExecutionRecord {
	record := &workflow.ExecutionRecord{
		ID:            recordID,
		WorkflowID:    execCtx.WorkflowID,
		Attempt:       attempt,
		ResumedFromID: resumedFromID,
		WorkflowName:  execCtx.WorkflowName,
		Status:        execCtx.GetStatus().String(),
		ExitCode:      execCtx.ExitCode,
		StartedAt:     execCtx.StartedAt,
		CompletedAt:   execCtx.GetCompletedAt(),
	}
	if !record.CompletedAt.IsZero() {
		record.DurationMs = record.CompletedAt.Sub(record.StartedAt).Milliseconds()
	}

	var selectedName string
	var selectedState workflow.StepState
	for stepName, state := range execCtx.GetAllStepStates() { //nolint:gocritic // rangeValCopy: selecting from a thread-safe snapshot requires value semantics
		if state.Error == "" {
			continue
		}
		if selectedName == "" || state.CompletedAt.After(selectedState.CompletedAt) ||
			(state.CompletedAt.Equal(selectedState.CompletedAt) && stepName < selectedName) {
			selectedName = stepName
			selectedState = state
		}
	}
	record.ErrorMessage = selectedState.Error
	return record
}

func (s *ExecutionService) finalizeTerminalOutcome(
	ctx context.Context,
	execCtx *workflow.ExecutionContext,
	finalizer *terminalFinalizer,
	outcome terminalOutcome,
) {
	if finalizer.finalized {
		return
	}
	finalizer.finalized = true

	finalizationErr := s.finalizeAttemptHistory(ctx, execCtx, outcome.errorMessage, func() {
		switch {
		case outcome.err == nil:
			execCtx.SetStatus(workflow.StatusCompleted)
		case errors.Is(outcome.cancellationCause, context.Canceled) ||
			errors.Is(outcome.cancellationCause, context.DeadlineExceeded) ||
			errors.Is(outcome.err, context.Canceled) || errors.Is(outcome.err, context.DeadlineExceeded):
			execCtx.SetStatus(workflow.StatusCancelled)
		default:
			execCtx.SetStatus(workflow.StatusFailed)
		}
		execCtx.SetCompletedAt(time.Now())
		normalizeTerminalExitCode(execCtx, outcome.err)
	})
	if finalizationErr != nil {
		recordID, attempt, _, _ := execCtx.GetHistoryAttempt()
		s.logger.Debug("terminal history finalization deferred",
			"workflow_id", execCtx.WorkflowID,
			"history_record_id", recordID,
			"attempt", attempt,
		)
	}
	s.emitWorkflowTerminalEvent(ctx, execCtx, outcome.err)
}

func (s *ExecutionService) executeStartedAttempt(ctx context.Context, wf *workflow.Workflow, execCtx *workflow.ExecutionContext, run func() error) error {
	finalizer := &terminalFinalizer{}
	outcome := func() (result terminalOutcome) {
		defer func() {
			result.cancellationCause = ctx.Err()
			if recover() != nil {
				result.err = errors.New("workflow execution panic")
				result.errorMessage = result.err.Error()
			}
		}()
		s.emitAuditStarted(ctx, execCtx, execCtx.Inputs)
		s.emitEvent(ctx, workflow.EventWorkflowStarted, map[string]string{
			"workflow_id": execCtx.WorkflowID, "workflow_name": execCtx.WorkflowName,
		})
		if err := s.hookExecutor.ExecuteHooks(ctx, wf.Hooks.WorkflowStart, s.buildInterpolationContext(execCtx), true); err != nil {
			result.err = fmt.Errorf("workflow_start hook failed: %w", err)
			result.errorMessage = result.err.Error()
			return result
		}
		result.err = run()
		if result.err != nil {
			result.errorMessage = result.err.Error()
		}
		return result
	}()

	finalizationCtx := context.WithoutCancel(ctx)
	s.finalizeTerminalOutcome(finalizationCtx, execCtx, finalizer, outcome)
	s.executeTerminalHook(finalizationCtx, wf, execCtx, outcome.err)
	return outcome.err
}

func (s *ExecutionService) executeTerminalHook(ctx context.Context, wf *workflow.Workflow, execCtx *workflow.ExecutionContext, execErr error) {
	intCtx := s.buildInterpolationContext(execCtx)
	hookName := "workflow_end"
	hooks := wf.Hooks.WorkflowEnd
	if execErr != nil {
		if execCtx.GetStatus() == workflow.StatusCancelled {
			hookName, hooks = "workflow_cancel", wf.Hooks.WorkflowCancel
		} else {
			hookName, hooks = "workflow_error", wf.Hooks.WorkflowError
			intCtx.Error = &interpolation.ErrorData{Type: classifyErrorType(execErr), Message: execErr.Error(), State: execCtx.CurrentStep}
		}
	}
	for _, action := range hooks {
		if err := s.hookExecutor.ExecuteHooks(ctx, workflow.Hook{action}, intCtx, true); err != nil {
			s.logger.Warn(hookName+" hook failed", "workflow_id", execCtx.WorkflowID, "workflow_name", execCtx.WorkflowName, "error", err)
		}
	}
}

// SetOutputWriters configures streaming output writers.
func (s *ExecutionService) SetOutputWriters(stdout, stderr io.Writer) {
	s.stdoutWriter = stdout
	s.stderrWriter = stderr
}

// SetDisplayRendererFactory installs a per-step renderer factory. When set, each agent
// step's context carries the renderer returned for that step name, enabling transports
// (e.g. ACP) to receive typed display events. A factory returning nil leaves the step
// using the default (inner-writer) rendering path.
func (s *ExecutionService) SetDisplayRendererFactory(f func(stepID string) display.EventRenderer) {
	s.displayRendererFactory = f
}

// SetTemplateService configures the template service for expanding template references.
func (s *ExecutionService) SetTemplateService(svc *TemplateService) {
	s.templateSvc = svc
}

// SetOperationProvider configures the plugin operation provider for F021.
// When set, operation-type steps can execute plugin-provided operations.
func (s *ExecutionService) SetOperationProvider(provider ports.OperationProvider) {
	s.operationProvider = provider
}

// SetAgentRegistry configures the agent registry for F039 agent step execution.
// When set, agent-type steps can execute AI provider operations.
func (s *ExecutionService) SetAgentRegistry(registry ports.AgentRegistry) {
	s.agentRegistry = registry
}

// SetToolProxyService configures the MCP tool proxy for F099 per-step proxy lifecycle.
// Must be called after SetAgentRegistry and before SetConversationManager.
// When nil, proxy behavior is skipped and existing flows are unaffected.
func (s *ExecutionService) SetToolProxyService(svc *tools.ProxyService) {
	s.toolProxy = svc
}

// SetEventPublisher configures the event publisher for workflow lifecycle event emission.
func (s *ExecutionService) SetEventPublisher(p ports.EventPublisher) {
	s.eventPublisher = p
}

// SetEvaluator configures the expression evaluator for conditional transitions.
// When set, enables evaluation of "when" clauses in workflow transitions.
func (s *ExecutionService) SetEvaluator(evaluator ports.ExpressionEvaluator) {
	s.evaluator = evaluator
}

// SetConversationManager configures the conversation manager for F033 multi-turn conversations.
// When set, agent-type steps with mode: conversation can execute managed conversations.
// Accepts ConversationExecutor interface to allow dependency injection and testing with mocks.
func (s *ExecutionService) SetConversationManager(mgr ConversationExecutor) {
	s.conversationMgr = mgr
	// Route the multi-turn loop's per-turn emission through this service's recorder-backed
	// transcript emitters (the ConversationManager owns no recorder). This makes the agent's
	// questions and the user's answers visible to event-only consumers (TUI monitoring, SSE).
	if e, ok := mgr.(interface {
		SetTurnEmitter(func(ctx context.Context, ec *workflow.ExecutionContext, role workflow.TurnRole, provider, content string))
	}); ok {
		e.SetTurnEmitter(s.emitConversationTurn)
	}
}

// emitConversationTurn records a single conversation turn as a transcript message event,
// reusing the same recorder-backed emitters as single-turn agent steps. User turns capture
// the outgoing prompt/answer (router fidelity); assistant turns capture the agent's response
// text (no raw NDJSON is available per turn here, so the text is emitted as a single block).
func (s *ExecutionService) emitConversationTurn(ctx context.Context, ec *workflow.ExecutionContext, role workflow.TurnRole, provider, content string) {
	switch role {
	case workflow.TurnRoleUser:
		s.emitTranscriptAgentMessage(ctx, ec, content, "")
	case workflow.TurnRoleAssistant:
		s.emitTranscriptAgentResponse(ctx, ec, provider, "", content)
	case workflow.TurnRoleSystem:
		// System turns are composed into the user message's system block; not emitted here.
	}
}

func (s *ExecutionService) SetSkillRepository(repo ports.SkillRepository) {
	s.skillRepo = repo
}

func (s *ExecutionService) SetAgentRoleRepository(repo ports.AgentRoleRepository) {
	s.agentRoleRepo = repo
}

func (s *ExecutionService) buildRoleAwareSystemPrompt(ctx context.Context, step *workflow.Step, workflowDir string, intCtx *interpolation.Context) (string, error) {
	return BuildRoleSystemPrompt(ctx, s.agentRoleRepo, s.resolver, step, workflowDir, intCtx)
}

// SetAWFPaths configures the AWF XDG directory paths for F063 template interpolation.
// Keys: prompts_dir, config_dir, data_dir, workflows_dir, plugins_dir, scripts_dir.
func (s *ExecutionService) SetAWFPaths(paths map[string]string) {
	s.awfPaths = paths
}

// SetAuditTrailWriter configures the audit trail writer for F071 structured audit events.
// When nil, audit emission is skipped without error.
func (s *ExecutionService) SetAuditTrailWriter(w ports.AuditTrailWriter) {
	s.auditTrailWriter = w
}

// SetRecorder configures the transcript recorder for F106 canonical exchange transcript.
// When nil, transcript emission is skipped without error.
func (s *ExecutionService) SetRecorder(r ports.Recorder) {
	s.recorder = r
}

// SetRecorderFactory configures the factory used to create child recorders for
// call_workflow sub-runs. When nil, child transcript recording is skipped.
func (s *ExecutionService) SetRecorderFactory(f ports.RecorderFactory) {
	s.recorderFactory = f
}

// SetTranscriptDir configures the directory where transcript files are written.
// Derived from the same config as the parent recorder's path.
func (s *ExecutionService) SetTranscriptDir(dir string) {
	s.transcriptDir = dir
}

// SetAgentOutputNormalizer configures the normalizer that converts provider raw output
// into transcript ContentBlocks for F106 message.assistant emission. When nil, agent
// output is not normalized into the transcript (graceful no-op).
func (s *ExecutionService) SetAgentOutputNormalizer(n ports.AgentOutputNormalizer) {
	s.agentOutputNormalizer = n
}

// transcriptBaseDir returns the configured transcript directory, falling back
// to "storage/transcripts" when none is set.
func (s *ExecutionService) transcriptBaseDir() string {
	if s.transcriptDir != "" {
		return s.transcriptDir
	}
	return "storage/transcripts"
}

// SetPluginService configures the plugin service for disabled-plugin detection.
// When set, executePluginOperation checks if the plugin is disabled before lookup.
// When nil, the check is skipped (backward compatible).
func (s *ExecutionService) SetPluginService(svc *PluginService) {
	s.pluginSvc = svc
}

// SetStepTypeProvider configures the plugin step type provider for C069 custom step type execution.
// When set, unknown step types are delegated to the provider via ExecuteStep.
// When nil, unknown step types fall through to the default command executor (backward compatible).
func (s *ExecutionService) SetStepTypeProvider(provider ports.StepTypeProvider) {
	s.stepTypeProvider = provider
}

// SetPackWorkflowLoader configures the pack workflow loader for C072 namespaced call_workflow resolution.
// When set, namespaced call_workflow references (e.g. "pack/workflow") route through this loader.
// When nil, namespaced references return an error (backward compatible for unnamespaced workflows).
func (s *ExecutionService) SetPackWorkflowLoader(loader PackWorkflowLoader) {
	s.packWorkflowLoader = loader
}

func (s *ExecutionService) SetTracer(tracer ports.Tracer) {
	s.tracer = tracer
}

func (s *ExecutionService) startSpan(ctx context.Context, name string) (context.Context, ports.Span) {
	t := s.tracer
	if t == nil {
		t = ports.NopTracer{}
	}
	return t.Start(ctx, name)
}

func (s *ExecutionService) resolveAuditUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return "unknown"
}

// secretPrefixes are the patterns checked (case-insensitive prefix match) to identify secret keys.
// This mirrors infrastructure/logger.DefaultSecretPatterns without importing infrastructure.
var secretPrefixes = []string{"SECRET_", "API_KEY", "PASSWORD", "TOKEN"}

// isSecretInputKey returns true if the key matches any known secret prefix.
func isSecretInputKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, prefix := range secretPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func (s *ExecutionService) emitAuditStarted(ctx context.Context, execCtx *workflow.ExecutionContext, inputs map[string]any) {
	if s.auditTrailWriter == nil {
		return
	}

	// Build masked copy of inputs: secret keys replaced with "***"
	var maskedInputs map[string]any
	if len(inputs) > 0 {
		maskedInputs = make(map[string]any, len(inputs))
		for k, v := range inputs {
			if isSecretInputKey(k) {
				maskedInputs[k] = "***"
			} else {
				maskedInputs[k] = v
			}
		}
	}

	auditUser := s.resolveAuditUser()
	event := workflow.NewStartedEvent(execCtx, maskedInputs, auditUser)
	if err := s.auditTrailWriter.Write(ctx, &event); err != nil {
		if s.logger != nil {
			s.logger.Warn("audit trail write failed", "error", err, "event", workflow.EventWorkflowStarted)
		}
	}
}

func (s *ExecutionService) emitAuditCompleted(ctx context.Context, execCtx *workflow.ExecutionContext, errorMsg string) {
	if s.auditTrailWriter == nil {
		return
	}

	// Ensure CompletedAt is set so duration calculation is meaningful.
	// Guarantee at least 1ms elapsed to avoid zero-duration audit events.
	if execCtx.CompletedAt.IsZero() {
		completedAt := time.Now()
		if !completedAt.After(execCtx.StartedAt.Add(time.Millisecond - 1)) {
			completedAt = execCtx.StartedAt.Add(time.Millisecond)
		}
		execCtx.CompletedAt = completedAt
	}

	auditUser := s.resolveAuditUser()
	event := workflow.NewCompletedEvent(execCtx, auditUser, errorMsg)
	if err := s.auditTrailWriter.Write(ctx, &event); err != nil {
		if s.logger != nil {
			s.logger.Warn("audit trail write failed", "error", err, "event", workflow.EventWorkflowCompleted)
		}
	}
}

func (s *ExecutionService) emitEvent(ctx context.Context, eventType string, metadata map[string]string) {
	if s.eventPublisher == nil {
		return
	}

	masked := make(map[string]string, len(metadata))
	for k, v := range metadata {
		if isSecretInputKey(k) {
			masked[k] = "***"
		} else {
			masked[k] = v
		}
	}

	event := pluginmodel.NewDomainEvent(eventType, "core", masked, nil)
	if err := s.eventPublisher.Publish(ctx, event); err != nil {
		if s.logger != nil {
			s.logger.Warn("event publish failed", "event", eventType, "error", err)
		}
	}
}

func (s *ExecutionService) emitWorkflowTerminalEvent(ctx context.Context, execCtx *workflow.ExecutionContext, execErr error) {
	if execErr != nil {
		s.emitEvent(ctx, workflow.EventWorkflowFailed, map[string]string{
			"workflow_id":   execCtx.WorkflowID,
			"workflow_name": execCtx.WorkflowName,
			"error":         execErr.Error(),
		})
	} else {
		s.emitEvent(ctx, workflow.EventWorkflowCompleted, map[string]string{
			"workflow_id":   execCtx.WorkflowID,
			"workflow_name": execCtx.WorkflowName,
			"duration":      execCtx.CompletedAt.Sub(execCtx.StartedAt).String(),
		})
	}
}

// NewExecutionService - historySvc can be nil to disable history recording.
func NewExecutionService(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	parallelExecutor ports.ParallelExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
	historySvc *HistoryService,
) *ExecutionService {
	return &ExecutionService{
		workflowSvc:      wfSvc,
		executor:         executor,
		parallelExecutor: parallelExecutor,
		store:            store,
		logger:           logger,
		resolver:         resolver,
		hookExecutor:     NewHookExecutor(executor, logger, resolver),
		loopExecutor:     NewLoopExecutor(logger, nil, resolver),
		historySvc:       historySvc,
		outputLimiter:    NewOutputLimiter(workflow.DefaultOutputLimits()),
	}
}

// NewExecutionServiceWithEvaluator enables conditional transitions using the `when:` clause.
func NewExecutionServiceWithEvaluator(
	wfSvc *WorkflowService,
	executor ports.CommandExecutor,
	parallelExecutor ports.ParallelExecutor,
	store ports.StateStore,
	logger ports.Logger,
	resolver interpolation.Resolver,
	historySvc *HistoryService,
	evaluator ports.ExpressionEvaluator,
) *ExecutionService {
	return &ExecutionService{
		workflowSvc:      wfSvc,
		executor:         executor,
		parallelExecutor: parallelExecutor,
		store:            store,
		logger:           logger,
		resolver:         resolver,
		evaluator:        evaluator,
		hookExecutor:     NewHookExecutor(executor, logger, resolver),
		loopExecutor:     NewLoopExecutor(logger, evaluator, resolver),
		historySvc:       historySvc,
		outputLimiter:    NewOutputLimiter(workflow.DefaultOutputLimits()),
	}
}

// RunWithWorkflowAndRunID executes a pre-loaded workflow using runID as the execution's
// WorkflowID. This lets callers (e.g. the CLI run command) reuse the same identifier for
// the transcript filename (<run-id>.jsonl) and the run_id stamped on every emitted event,
// keeping the on-disk file and its contents correlatable (F106 SC-001).
func (s *ExecutionService) RunWithWorkflowAndRunID(
	ctx context.Context,
	wf *workflow.Workflow,
	inputs map[string]any,
	runID string,
) (*workflow.ExecutionContext, error) {
	return s.runWithCallStackAndWorkflow(ctx, "", wf, inputs, nil, runID, "")
}

// prepareExecution handles workflow setup: span creation, template expansion, context
// initialization, input validation, audit emission, and workflow_start hooks.
// Returns the span-enriched context, the span (caller must End() it), and the execution context.
//
// runID, when non-empty, is used verbatim as the execution's WorkflowID so the
// transcript filename (<run-id>.jsonl) and the run_id stamped on every emitted event
// are the same identifier. An empty runID falls back to a generated UUID. parentRunID
// is propagated onto the child context for F106 sub-workflow transcript linkage.
func (s *ExecutionService) prepareExecution(
	ctx context.Context,
	wf *workflow.Workflow,
	inputs map[string]any,
	parentCallStack []string,
	runID, parentRunID string,
) (context.Context, ports.Span, *workflow.ExecutionContext, error) {
	ctx, span := s.startSpan(ctx, "workflow.run")
	span.SetAttribute("workflow.name", wf.Name)
	span.SetAttribute("workflow.version", wf.Version)
	span.SetAttribute("user", s.resolveAuditUser())

	if s.templateSvc != nil {
		if err := s.templateSvc.ExpandWorkflow(ctx, wf); err != nil {
			span.End()
			return ctx, nil, nil, fmt.Errorf("expand templates: %w", err)
		}
	}

	resolvedRunID := runID
	if resolvedRunID == "" {
		resolvedRunID = uuid.New().String()
	}
	execCtx := workflow.NewExecutionContext(resolvedRunID, wf.Name)
	execCtx.ParentRunID = parentRunID
	execCtx.Status = workflow.StatusRunning
	span.SetAttribute("execution_id", execCtx.WorkflowID)

	if len(parentCallStack) > 0 {
		execCtx.CallStack = make([]string, len(parentCallStack))
		copy(execCtx.CallStack, parentCallStack)
	}

	for _, inp := range wf.Inputs {
		if _, provided := inputs[inp.Name]; !provided && inp.Default != nil {
			execCtx.SetInput(inp.Name, inp.Default)
		}
	}
	for k, v := range inputs {
		execCtx.SetInput(k, v)
	}

	if err := s.validateInputs(execCtx.Inputs, wf.Inputs); err != nil {
		span.End()
		return ctx, nil, nil, fmt.Errorf("input validation failed: %w", err)
	}

	s.logger.Info("starting workflow", "workflow", wf.Name, "id", execCtx.WorkflowID)
	return ctx, span, execCtx, nil
}

// dispatchStep routes a single non-terminal step to the appropriate executor based on
// its type. It is shared by runExecutionLoop and executeFromStep to keep the dispatch
// logic in one place — any new step type must be added here only.
func (s *ExecutionService) dispatchStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	switch step.Type {
	case workflow.StepTypeCommand:
		return s.executeStep(ctx, wf, step, execCtx)
	case workflow.StepTypeParallel:
		return s.executeParallelStep(ctx, wf, step, execCtx)
	case workflow.StepTypeForEach, workflow.StepTypeWhile:
		return s.executeLoopStep(ctx, wf, step, execCtx)
	case workflow.StepTypeOperation:
		return s.executePluginOperation(ctx, step, execCtx)
	case workflow.StepTypeCallWorkflow:
		return s.executeCallWorkflowStep(ctx, wf, step, execCtx)
	case workflow.StepTypeAgent:
		return s.executeAgentStep(ctx, wf, step, execCtx)
	default:
		return s.executeCustomStepType(ctx, wf, step, execCtx)
	}
}

// runExecutionLoop runs the step-by-step state machine and post-execution hooks.
//
//nolint:gocognit,cyclop // Execution loop handles state transitions, error handling, and hook execution.
func (s *ExecutionService) runExecutionLoop(
	ctx context.Context,
	wf *workflow.Workflow,
	execCtx *workflow.ExecutionContext,
) error {
	var execErr error
	currentStep := wf.Initial
	for {
		step, ok := wf.Steps[currentStep]
		if !ok {
			execCtx.SetStatus(workflow.StatusFailed)
			execErr = fmt.Errorf("step not found: %s", currentStep)
			break
		}

		execCtx.CurrentStep = currentStep

		if step.Type == workflow.StepTypeTerminal {
			if step.Status == workflow.TerminalFailure {
				execCtx.SetStatus(workflow.StatusFailed)
				execCtx.ExitCode = step.ExitCode
				if msg := s.interpolateTerminalMessage(step.Message, s.buildInterpolationContext(execCtx)); msg != "" {
					execErr = errors.New(msg)
				} else {
					execErr = fmt.Errorf("workflow reached terminal failure state: %s", currentStep)
				}
			} else {
				execCtx.SetStatus(workflow.StatusCompleted)
			}
			s.logger.Info("workflow completed", "step", currentStep, "status", execCtx.GetStatus())
			break
		}

		var nextStep string
		var err error

		s.logger.Debug("executing step", "step", step.Name)
		s.emitEvent(ctx, workflow.EventStepStarted, map[string]string{
			"workflow_id": execCtx.WorkflowID,
			"step_name":   step.Name,
		})

		nextStep, err = s.dispatchStep(ctx, wf, step, execCtx)
		if err != nil {
			s.emitEvent(ctx, workflow.EventStepFailed, map[string]string{
				"workflow_id": execCtx.WorkflowID,
				"step_name":   step.Name,
				"error":       err.Error(),
			})
			execCtx.Status = workflow.StatusFailed
			s.logger.Error("step failed", "step", step.Name, "error", err)
			execErr = err
			break
		}

		if ss, ok := execCtx.GetStepState(step.Name); ok && ss.Status == workflow.StatusFailed {
			s.emitEvent(ctx, workflow.EventStepFailed, map[string]string{
				"workflow_id": execCtx.WorkflowID,
				"step_name":   step.Name,
				"error":       ss.Error,
			})
		} else {
			s.emitEvent(ctx, workflow.EventStepCompleted, map[string]string{
				"workflow_id": execCtx.WorkflowID,
				"step_name":   step.Name,
			})
		}
		s.checkpoint(ctx, execCtx)
		currentStep = nextStep
	}

	return execErr
}

// runWithCallStack executes a workflow by name with an optional parent call stack.
// This is used internally by executeCallWorkflowStep to preserve circular detection.
func (s *ExecutionService) runWithCallStack(
	ctx context.Context,
	workflowName string,
	inputs map[string]any,
	parentCallStack []string,
	runID, parentRunID string,
) (*workflow.ExecutionContext, error) {
	return s.runWithCallStackAndWorkflow(ctx, workflowName, nil, inputs, parentCallStack, runID, parentRunID)
}

// runWithCallStackAndWorkflow executes a workflow with an optional parent call stack.
// If wf is nil, loads the workflow by name. Otherwise uses the provided workflow.
// runID/parentRunID thread the F106 run identity through to prepareExecution.
func (s *ExecutionService) runWithCallStackAndWorkflow(
	ctx context.Context,
	workflowName string,
	wf *workflow.Workflow,
	inputs map[string]any,
	parentCallStack []string,
	runID, parentRunID string,
) (*workflow.ExecutionContext, error) {
	if wf == nil {
		var err error
		wf, err = s.workflowSvc.GetWorkflow(ctx, workflowName)
		if err != nil {
			return nil, fmt.Errorf("load workflow: %w", err)
		}
	}

	spanCtx, span, execCtx, err := s.prepareExecution(ctx, wf, inputs, parentCallStack, runID, parentRunID)
	if err != nil {
		// execCtx is non-nil when a workflow_start hook aborted after the execution
		// context was built and marked failed; propagate it so callers (and the test
		// suite) can observe the failed-but-recorded execution rather than a bare error.
		return execCtx, err
	}
	defer span.End()
	if err := s.prepareInitialHistoryAttempt(spanCtx, execCtx); err != nil {
		return nil, err
	}

	return execCtx, s.executeStartedAttempt(spanCtx, wf, execCtx, func() error {
		return s.runExecutionLoop(spanCtx, wf, execCtx)
	})
}

func (s *ExecutionService) prepareInitialHistoryAttempt(ctx context.Context, execCtx *workflow.ExecutionContext) error {
	if err := execCtx.BeginInitialHistoryAttempt(uuid.NewString()); err != nil {
		return fmt.Errorf("allocate initial history attempt: %w", err)
	}
	return s.persistHistoryAttemptAllocation(ctx, execCtx)
}

func (s *ExecutionService) persistHistoryAttemptAllocation(ctx context.Context, execCtx *workflow.ExecutionContext) error {
	if err := s.store.Save(ctx, execCtx); err != nil {
		return fmt.Errorf("persist history attempt allocation: %w", err)
	}
	return nil
}

func (s *ExecutionService) prepareResumedHistoryAttempt(ctx context.Context, execCtx *workflow.ExecutionContext) error {
	previousID := execCtx.HistoryRecordID
	previousAttempt := execCtx.HistoryAttempt
	if previousAttempt == 0 {
		var err error
		previousID, previousAttempt, err = s.normalizeLegacyHistoryAttempt(ctx, execCtx.WorkflowID)
		if err != nil {
			return err
		}
	}
	if previousAttempt < 0 || previousAttempt == math.MaxInt || (previousAttempt > 0 && previousID == "") {
		return fmt.Errorf("invalid saved history lineage: %w", workflow.ErrInvalidHistoryAttempt)
	}

	var err error
	if previousAttempt == 0 {
		err = execCtx.BeginInitialHistoryAttempt(uuid.NewString())
	} else {
		err = execCtx.BeginResumedHistoryAttempt(uuid.NewString(), previousID, previousAttempt+1)
	}
	if err != nil {
		return fmt.Errorf("allocate resumed history attempt: %w", err)
	}
	execCtx.StartedAt = time.Now()
	execCtx.UpdatedAt = execCtx.StartedAt
	execCtx.CompletedAt = time.Time{}
	return s.persistHistoryAttemptAllocation(ctx, execCtx)
}

func (s *ExecutionService) normalizeLegacyHistoryAttempt(
	ctx context.Context,
	workflowID string,
) (recordID string, attempt int, err error) {
	if !s.historySvc.IsEnabled() {
		return "", 0, nil
	}
	records, err := s.historySvc.List(ctx, &workflow.HistoryFilter{WorkflowID: workflowID, Limit: math.MaxInt})
	if err != nil {
		return "", 0, fmt.Errorf("list workflow history for attempt allocation: %w", err)
	}
	recordsByID, err := indexLegacyHistoryRecords(records, workflowID)
	if err != nil {
		return "", 0, err
	}
	idsByAttempt := make(map[int]string, len(records))
	maxAttempt := 0
	maxID := ""
	for _, record := range records {
		if record.WorkflowID != workflowID {
			continue
		}
		if err := validateLegacyHistoryRecord(record, recordsByID); err != nil {
			return "", 0, err
		}
		if existingID, exists := idsByAttempt[record.Attempt]; exists && existingID != record.ID {
			return "", 0, fmt.Errorf("conflicting workflow history ordinal %d: %w", record.Attempt, workflow.ErrInvalidHistoryAttempt)
		}
		idsByAttempt[record.Attempt] = record.ID
		if record.Attempt > maxAttempt {
			maxAttempt, maxID = record.Attempt, record.ID
		}
	}
	if maxAttempt == math.MaxInt {
		return "", 0, fmt.Errorf("workflow history attempt overflow: %w", workflow.ErrInvalidHistoryAttempt)
	}
	return maxID, maxAttempt, nil
}

func indexLegacyHistoryRecords(records []*workflow.ExecutionRecord, workflowID string) (map[string]*workflow.ExecutionRecord, error) {
	recordsByID := make(map[string]*workflow.ExecutionRecord, len(records))
	for _, record := range records {
		if record == nil {
			return nil, fmt.Errorf("nil workflow history record: %w", workflow.ErrInvalidHistoryAttempt)
		}
		previous, exists := recordsByID[record.ID]
		if exists && (record.WorkflowID == workflowID || previous.WorkflowID == workflowID) &&
			(record.WorkflowID != previous.WorkflowID || record.Attempt != previous.Attempt || record.ResumedFromID != previous.ResumedFromID) {
			return nil, fmt.Errorf("conflicting workflow history identity: %w", workflow.ErrInvalidHistoryAttempt)
		}
		recordsByID[record.ID] = record
	}
	return recordsByID, nil
}

func validateLegacyHistoryRecord(record *workflow.ExecutionRecord, recordsByID map[string]*workflow.ExecutionRecord) error {
	if record.ID == "" || record.Attempt < 1 ||
		(record.Attempt == 1 && record.ResumedFromID != "") ||
		(record.Attempt > 1 && record.ResumedFromID == "") || record.ResumedFromID == record.ID {
		return fmt.Errorf("invalid workflow history lineage: %w", workflow.ErrInvalidHistoryAttempt)
	}
	// Retention may remove a predecessor. Validate its relationship only when
	// the referenced record is still present, independently of result ordering.
	if predecessor, exists := recordsByID[record.ResumedFromID]; record.ResumedFromID != "" && exists &&
		(predecessor.WorkflowID != record.WorkflowID || predecessor.Attempt != record.Attempt-1) {
		return fmt.Errorf("inconsistent workflow history predecessor: %w", workflow.ErrInvalidHistoryAttempt)
	}
	return nil
}

// executeStep executes a single step and returns the next step name.
func (s *ExecutionService) executeStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	ctx, span := s.startSpan(ctx, "step."+step.Name)
	defer span.End()
	span.SetAttribute("step.name", step.Name)
	span.SetAttribute("step.type", string(step.Type))
	s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
	defer s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)

	startTime := time.Now()

	// T007: Prepare step execution (timeout, pre-hooks, interpolation)
	stepCtx, intCtx, cancel, err := s.prepareStepExecution(ctx, step, execCtx)
	if cancel != nil {
		defer cancel()
	}
	if err != nil {
		return "", err
	}

	// T007: Resolve command and directory
	cmd, err := s.resolveStepCommand(stepCtx, wf, step, intCtx)
	if err != nil {
		return "", err
	}

	// Mark step as running so SSE handler can observe it before completion.
	execCtx.SetStepState(step.Name, workflow.StepState{
		Name:      step.Name,
		Status:    workflow.StatusRunning,
		StartedAt: startTime,
	})

	// T008: Execute command (with retry if configured)
	result, attempt, execErr := s.executeStepCommand(stepCtx, step, cmd)

	// T008: Record step state (timing, output, attempt)
	state := s.recordStepResult(step, startTime, result, attempt)

	// T009: Determine outcome and return next step
	if execErr != nil {
		return s.handleExecutionError(ctx, stepCtx, step, execCtx, &state, execErr)
	}

	if result.ExitCode != 0 {
		return s.handleNonZeroExit(stepCtx, step, execCtx, &state, result)
	}

	return s.handleSuccess(stepCtx, step, execCtx, &state)
}

// resolveNextStep determines the next step using conditional transitions or legacy OnSuccess/OnFailure.
// If step has Transitions defined, evaluates them in order (first match wins).
// If no transition matches and no default, falls back to OnSuccess/OnFailure based on success parameter.
func (s *ExecutionService) resolveNextStep(
	step *workflow.Step,
	intCtx *interpolation.Context,
	success bool,
) (string, error) {
	return resolveNextStep(step, intCtx, success, s.evaluator, s.logger)
}

// checkpoint saves the current execution state.
// Failures are logged but not fatal - execution continues.
func (s *ExecutionService) checkpoint(ctx context.Context, execCtx *workflow.ExecutionContext) {
	if err := s.store.Save(ctx, execCtx); err != nil {
		s.logger.Warn("checkpoint failed", "workflow_id", execCtx.WorkflowID, "error", err)
	}
}

// recordHistory saves execution to history when workflow reaches terminal state.
// Failures are logged but not fatal - execution continues.
func (s *ExecutionService) recordHistory(ctx context.Context, execCtx *workflow.ExecutionContext) {
	if s.historySvc == nil {
		return
	}

	// Only record terminal states
	if execCtx.Status != workflow.StatusCompleted &&
		execCtx.Status != workflow.StatusFailed &&
		execCtx.Status != workflow.StatusCancelled {
		return
	}

	// Map ExecutionContext to ExecutionRecord
	record := &workflow.ExecutionRecord{
		ID:           execCtx.WorkflowID,
		WorkflowID:   execCtx.WorkflowID,
		WorkflowName: execCtx.WorkflowName,
		Status:       execCtx.Status.String(),
		StartedAt:    execCtx.StartedAt,
		CompletedAt:  execCtx.CompletedAt,
		ExitCode:     execCtx.ExitCode,
	}

	// Calculate duration
	if !execCtx.CompletedAt.IsZero() {
		record.DurationMs = execCtx.CompletedAt.Sub(execCtx.StartedAt).Milliseconds()
	}

	// Find the last executed step for its error message. ExitCode is normalized
	// once at the completion boundary and shared by every persistence projection.
	allStates := execCtx.GetAllStepStates()
	if len(allStates) > 0 {
		var lastState workflow.StepState
		var foundLast bool
		for name := range allStates {
			state := allStates[name]
			if !foundLast || state.CompletedAt.After(lastState.CompletedAt) {
				lastState = state
				foundLast = true
			}
		}
		if foundLast {
			if lastState.Error != "" {
				record.ErrorMessage = lastState.Error
			}
		}
	}

	if err := s.historySvc.Record(ctx, record); err != nil {
		s.logger.Warn("failed to record history", "workflow_id", execCtx.WorkflowID, "error", err)
	} else {
		s.logger.Debug("recorded execution history", "workflow_id", execCtx.WorkflowID, "status", record.Status)
	}
}

func (s *ExecutionService) recordExecutionEnd(
	ctx context.Context,
	execCtx *workflow.ExecutionContext,
	completionErr error,
	errorMsg string,
) {
	normalizeTerminalExitCode(execCtx, completionErr)
	if execCtx.GetCompletedAt().IsZero() {
		execCtx.SetCompletedAt(time.Now())
	}
	// Persist the final outcome after normalization, including when execution was
	// cancelled. Earlier step checkpoints may still contain a zero exit code.
	s.checkpoint(context.WithoutCancel(ctx), execCtx)
	s.recordHistory(context.WithoutCancel(ctx), execCtx)
	s.emitAuditCompleted(ctx, execCtx, errorMsg)
}

func normalizeTerminalExitCode(execCtx *workflow.ExecutionContext, completionErr error) {
	if execCtx.ExitCode != 0 || execCtx.GetStatus() == workflow.StatusCompleted {
		return
	}

	currentStep := execCtx.GetCurrentStep()
	if currentState, ok := execCtx.GetStepState(currentStep); ok &&
		currentState.Status == workflow.StatusFailed && currentState.ExitCode != 0 {
		execCtx.ExitCode = currentState.ExitCode
		return
	}

	var selectedName string
	var selectedState workflow.StepState
	for stepName, state := range execCtx.GetAllStepStates() { //nolint:gocritic // rangeValCopy: selecting from a thread-safe snapshot requires value semantics
		if state.Status != workflow.StatusFailed || state.ExitCode == 0 || state.CompletedAt.IsZero() {
			continue
		}
		if selectedState.CompletedAt.IsZero() || state.CompletedAt.After(selectedState.CompletedAt) ||
			(state.CompletedAt.Equal(selectedState.CompletedAt) && stepName < selectedName) {
			selectedName = stepName
			selectedState = state
		}
	}
	if selectedState.ExitCode != 0 {
		execCtx.ExitCode = selectedState.ExitCode
		return
	}

	var structuredErr *domainerrors.StructuredError
	if errors.As(completionErr, &structuredErr) && structuredErr != nil && structuredErr.ExitCode() != 0 {
		execCtx.ExitCode = structuredErr.ExitCode()
		return
	}

	status := execCtx.GetStatus()
	if status == workflow.StatusFailed || status == workflow.StatusCancelled {
		execCtx.ExitCode = 3
	}
}

// buildLoopDataChain recursively converts domain LoopContext to interpolation LoopData.
// This enables {{.loop.Parent.*}} template access for nested loops.
// F043: Nested Loop Execution
func buildLoopDataChain(domainLoop *workflow.LoopContext) *interpolation.LoopData {
	if domainLoop == nil {
		return nil
	}
	return &interpolation.LoopData{
		Item:   domainLoop.Item,
		Index:  domainLoop.Index,
		First:  domainLoop.First,
		Last:   domainLoop.Last,
		Length: domainLoop.Length,
		Parent: buildLoopDataChain(domainLoop.Parent), // Recursive for parent chain
	}
}

// buildInterpolationContext converts ExecutionContext to interpolation.Context.
// Delegates to the package-level helper; see interpolation_helpers.go.
func (s *ExecutionService) buildInterpolationContext(
	execCtx *workflow.ExecutionContext,
) *interpolation.Context {
	return buildInterpolationContext(execCtx, s.awfPaths)
}

// executeWithRetry executes a command with retry logic.
// Returns the final result, number of attempts made, and error.
func (s *ExecutionService) executeWithRetry(
	ctx context.Context,
	step *workflow.Step,
	cmd *ports.Command,
) (*ports.CommandResult, int, error) {
	// Convert domain RetryConfig to retry.Config
	retryCfg := retry.Config{
		MaxAttempts:        step.Retry.MaxAttempts,
		InitialDelay:       time.Duration(step.Retry.InitialDelayMs) * time.Millisecond,
		MaxDelay:           time.Duration(step.Retry.MaxDelayMs) * time.Millisecond,
		Strategy:           retry.Strategy(step.Retry.Backoff),
		Multiplier:         step.Retry.Multiplier,
		Jitter:             step.Retry.Jitter,
		RetryableExitCodes: step.Retry.RetryableExitCodes,
	}

	// Create retryer with current time as seed for random jitter
	retryer := retry.NewRetryer(&retryCfg, &retryLoggerAdapter{s.logger}, time.Now().UnixNano())

	var result *ports.CommandResult
	var execErr error

	for attempt := 1; attempt <= retryCfg.MaxAttempts; attempt++ {
		result, execErr = s.executor.Execute(ctx, cmd)

		// Determine exit code
		exitCode := 0
		if result != nil {
			exitCode = result.ExitCode
		}
		if execErr != nil && exitCode == 0 {
			// Execution error without exit code (e.g., context canceled)
			// Don't retry on context errors
			if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
				return result, attempt, execErr
			}
			exitCode = 1 // Treat as generic failure
		}

		// Success - no retry needed
		if execErr == nil && exitCode == 0 {
			return result, attempt, nil
		}

		// Check if we should retry
		if !retryer.ShouldRetry(exitCode, attempt) {
			s.logger.Debug(
				"not retrying",
				"step", step.Name,
				"attempt", attempt,
				"exit_code", exitCode,
				"max_attempts", retryCfg.MaxAttempts,
			)
			return result, attempt, execErr
		}

		// Log retry
		s.logger.Info(
			"retrying step",
			"step", step.Name,
			"attempt", attempt,
			"exit_code", exitCode,
			"max_attempts", retryCfg.MaxAttempts,
		)
		s.emitEvent(ctx, workflow.EventStepRetrying, map[string]string{
			"step_name": step.Name,
			"attempt":   fmt.Sprint(attempt),
		})

		// Wait before next attempt
		if err := retryer.Wait(ctx, attempt); err != nil {
			// Context cancelled during wait
			return result, attempt, err
		}
	}

	return result, retryCfg.MaxAttempts, execErr
}

// retryLoggerAdapter adapts ports.Logger to retry.Logger interface.
type retryLoggerAdapter struct {
	logger ports.Logger
}

func (a *retryLoggerAdapter) Debug(msg string, keysAndValues ...any) {
	a.logger.Debug(msg, keysAndValues...)
}

func (a *retryLoggerAdapter) Info(msg string, keysAndValues ...any) {
	a.logger.Info(msg, keysAndValues...)
}

// executeParallelStep executes a parallel step using the ParallelExecutor.
func (s *ExecutionService) executeParallelStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	ctx, span := s.startSpan(ctx, "parallel")
	defer span.End()
	span.SetAttribute("step.name", step.Name)
	span.SetAttribute("parallel.strategy", string(step.Strategy))
	span.SetAttribute("parallel.branches", len(step.Branches))
	s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
	defer s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)

	startTime := time.Now()

	// apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// build interpolation context for hooks
	intCtx := s.buildInterpolationContext(execCtx)

	// execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// build parallel config
	config := workflow.ParallelConfig{
		Strategy:      workflow.ParseParallelStrategy(step.Strategy),
		MaxConcurrent: step.MaxConcurrent,
	}

	// create step executor adapter
	adapter := &stepExecutorAdapter{
		execSvc: s,
	}

	// execute parallel branches
	s.logger.Info("executing parallel step", "step", step.Name, "branches", step.Branches, "strategy", config.Strategy)
	result, err := s.parallelExecutor.Execute(stepCtx, wf, step.Branches, config, execCtx, adapter)

	// copy branch results to execCtx.States so they're available for interpolation
	if result != nil {
		for branchName, branchResult := range result.Results {
			state := workflow.StepState{
				Name:        branchName,
				StartedAt:   branchResult.StartedAt,
				CompletedAt: branchResult.CompletedAt,
				Output:      branchResult.Output,
				Stderr:      branchResult.Stderr,
				ExitCode:    branchResult.ExitCode,
			}
			switch {
			case branchResult.Error != nil:
				state.Status = workflow.StatusFailed
				state.Error = branchResult.Error.Error()
			case branchResult.ExitCode != 0:
				state.Status = workflow.StatusFailed
			default:
				state.Status = workflow.StatusCompleted
			}
			execCtx.SetStepState(branchName, state)
		}
	}

	// record parallel step state
	parallelState := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if err != nil {
		// Check if the PARENT context was cancelled
		if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			parallelState.Status = workflow.StatusFailed
			parallelState.Error = err.Error()
			execCtx.SetStepState(step.Name, parallelState)
			return "", fmt.Errorf("parallel step %s: %w", step.Name, err)
		}

		parallelState.Status = workflow.StatusFailed
		parallelState.Error = err.Error()
		execCtx.SetStepState(step.Name, parallelState)

		// execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("parallel step %s: %w", step.Name, err)
	}

	parallelState.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, parallelState)

	// execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions (if defined) or fallback to OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

type loopBodyTransitionLimitError struct {
	loopName  string
	iteration int
	from      string
	to        string
	limit     int
	output    string
}

func (e *loopBodyTransitionLimitError) Error() string {
	message := fmt.Sprintf(
		"loop body transition limit %d exceeded in loop %s iteration %d after backward transition %s -> %s; review intra-body transitions for a cycle",
		e.limit,
		e.loopName,
		e.iteration,
		e.from,
		e.to,
	)
	if e.output != "" {
		message += fmt.Sprintf("; last step output: %s", e.output)
	}
	return message
}

// executeLoopStep executes a for_each or while loop step.
//
//nolint:gocognit // Complexity 35: loop executor handles iteration, loop state, break/continue, and error cases. Complexity inherent to loop semantics.
func (s *ExecutionService) executeLoopStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	loopSpanName := "loop.while"
	if step.Type == workflow.StepTypeForEach {
		loopSpanName = "loop.for_each"
	}
	ctx, loopSpan := s.startSpan(ctx, loopSpanName)
	defer loopSpan.End()
	loopSpan.SetAttribute("step.name", step.Name)
	loopSpan.SetAttribute("loop.type", string(step.Type))
	s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
	defer s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)

	startTime := time.Now()

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Execute pre-hooks
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	var iterationIndex int
	bodyStepIndices := make(map[string]int, len(step.Loop.Body))
	for index, bodyStepName := range step.Loop.Body {
		bodyStepIndices[bodyStepName] = index
	}
	transitionLimit := step.Loop.MaxIterations
	if transitionLimit <= 0 || step.Loop.IsMaxIterationsDynamic() {
		transitionLimit = workflow.DefaultMaxIterations
	}
	transitionIteration := -1
	backwardTransitions := 0

	// Create step executor callback that executes body steps
	// Supports nested loops: body steps can be loops themselves (F043)
	// F048: Updated to return (nextStep, error) to support transitions within loop body
	stepExecutor := func(ctx context.Context, stepName string, loopIntCtx *interpolation.Context) (string, error) {
		iterationIndex++
		ctx, iterSpan := s.startSpan(ctx, fmt.Sprintf("loop.iteration.%d", iterationIndex))
		defer iterSpan.End()
		iterSpan.SetAttribute("iteration.index", iterationIndex)

		bodyStep, ok := wf.Steps[stepName]
		if !ok {
			return "", fmt.Errorf("body step not found: %s", stepName)
		}
		// Handle nested loops and special step types in body
		var nextStep string
		var err error
		switch bodyStep.Type {
		case workflow.StepTypeCommand:
			nextStep, err = s.executeStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeForEach, workflow.StepTypeWhile:
			nextStep, err = s.executeLoopStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeParallel:
			nextStep, err = s.executeParallelStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeCallWorkflow:
			nextStep, err = s.executeCallWorkflowStep(ctx, wf, bodyStep, execCtx)
		case workflow.StepTypeAgent:
			nextStep, err = s.executeAgentStep(ctx, wf, bodyStep, execCtx)
		default:
			nextStep, err = s.executeCustomStepType(ctx, wf, bodyStep, execCtx)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		// Transition evaluation can cancel the context while returning a destination.
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			if state, exists := execCtx.GetStepState(stepName); exists &&
				bodyStep.Type == workflow.StepTypeCommand && state.Status == workflow.StatusFailed &&
				state.ExitCode != 0 && strings.TrimSpace(state.Output) != "" {
				return "", fmt.Errorf("step %s output: %s: %w", stepName, strings.TrimSpace(state.Output), err)
			}
			return "", err
		}
		loopIteration := 0
		if loopIntCtx != nil && loopIntCtx.Loop != nil {
			loopIteration = loopIntCtx.Loop.Index
		}
		if loopIteration != transitionIteration {
			transitionIteration = loopIteration
			backwardTransitions = 0
		}
		targetIndex, internal := bodyStepIndices[nextStep]
		if internal && targetIndex <= bodyStepIndices[stepName] {
			backwardTransitions++
			if backwardTransitions > transitionLimit {
				var output string
				if state, exists := execCtx.GetStepState(stepName); exists {
					output = strings.TrimSpace(state.Output)
				}
				return "", &loopBodyTransitionLimitError{
					loopName:  step.Name,
					iteration: loopIteration,
					from:      stepName,
					to:        nextStep,
					limit:     transitionLimit,
					output:    output,
				}
			}
		}
		// Internal destinations are handled routing; loop-self targets retain retry semantics.
		// Only failed destinations outside this loop body are fatal escapes.
		if nextStep != "" && nextStep != step.Name && !internal {
			// Skip escape detection for continue_on_error steps (they intentionally proceed despite failure)
			if !bodyStep.ContinueOnError {
				if state, exists := execCtx.GetStepState(stepName); exists && state.Status == workflow.StatusFailed {
					if strings.TrimSpace(state.Output) != "" {
						return "", fmt.Errorf("step %s failed: %s", stepName, strings.TrimSpace(state.Output))
					}
					if state.Error != "" {
						return "", fmt.Errorf("step %s failed: %s", stepName, state.Error)
					}
					return "", fmt.Errorf("step %s failed with exit code %d", stepName, state.ExitCode)
				}
			}
		}
		// F048: Return nextStep to enable transition handling in loop executor
		return nextStep, nil
	}

	// Execute loop based on type
	var result *workflow.LoopResult
	var err error

	s.logger.Info("executing loop step", "step", step.Name, "type", step.Loop.Type)

	if step.Type == workflow.StepTypeForEach {
		result, err = s.loopExecutor.ExecuteForEach(
			stepCtx, wf, step, execCtx, stepExecutor, s.buildInterpolationContext,
		)
	} else {
		result, err = s.loopExecutor.ExecuteWhile(
			stepCtx, wf, step, execCtx, stepExecutor, s.buildInterpolationContext,
		)
	}

	// Record loop step state
	loopState := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}

	if err != nil {
		// Check if the PARENT context was cancelled
		if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			loopState.Status = workflow.StatusFailed
			loopState.Error = err.Error()
			execCtx.SetStepState(step.Name, loopState)
			return "", fmt.Errorf("loop step %s: %w", step.Name, err)
		}

		loopState.Status = workflow.StatusFailed
		loopState.Error = err.Error()
		execCtx.SetStepState(step.Name, loopState)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", err
	}

	// F048: Check if loop hit iteration limit with problematic patterns
	// While loops that run to MaxIterations with step failures or complex nesting
	// indicate potential infinite loop patterns that should be caught.
	if s.IsProblematicMaxIterationPattern(result, step, wf) {
		return s.HandleMaxIterationFailure(stepCtx, result, step, wf, execCtx, &loopState)
	}

	loopState.Status = workflow.StatusCompleted
	if result != nil {
		loopState.Output = fmt.Sprintf("completed %d iterations", result.TotalCount)
	}
	execCtx.SetStepState(step.Name, loopState)

	// Execute post-hooks
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// F048 T007: Return nextStep from loop result if early exit occurred
	if result != nil && result.NextStep != "" {
		return result.NextStep, nil
	}

	return step.Loop.OnComplete, nil
}

// detectLoopPatterns examines loop iterations to detect step failures and complex step types.
// This helper extracts shared logic used by IsProblematicMaxIterationPattern and HandleMaxIterationFailure.
// Returns two booleans: hadFailures (true if any step failed), hasComplexSteps (true if loop contains while/foreach/parallel/callworkflow).
func (s *ExecutionService) detectLoopPatterns(result *workflow.LoopResult, wf *workflow.Workflow) (hadFailures, hasComplexSteps bool) {
	// Handle nil inputs gracefully
	if result == nil || result.Iterations == nil {
		return false, false
	}

	// Iterate through all iterations looking for failures and complex steps
	for _, iteration := range result.Iterations {
		// Skip iterations with nil step results
		if iteration.StepResults == nil {
			continue
		}

		// Check each step result in this iteration
		for stepName, stepState := range iteration.StepResults {
			// Check for failures
			if stepState != nil && stepState.Status == workflow.StatusFailed {
				hadFailures = true
			}

			// Check for complex step types - requires workflow definition
			if wf != nil && wf.Steps != nil {
				if stepDef, exists := wf.Steps[stepName]; exists {
					if isComplexStepType(stepDef.Type) {
						hasComplexSteps = true
					}
				}
			}

			// Early exit optimization: if both conditions found, no need to continue
			if hadFailures && hasComplexSteps {
				return true, true
			}
		}
	}

	return hadFailures, hasComplexSteps
}

// isComplexStepType returns true if the step type represents a complex/nested structure
// that makes loop debugging difficult (while, foreach, parallel, call_workflow).
func isComplexStepType(stepType workflow.StepType) bool {
	switch stepType {
	case workflow.StepTypeWhile,
		workflow.StepTypeForEach,
		workflow.StepTypeParallel,
		workflow.StepTypeCallWorkflow:
		return true
	default:
		return false
	}
}

// shouldCheckLoopProblems determines if we should analyze a loop for problematic patterns.
// Returns true if the loop is a while loop with max iterations that completed by hitting the limit (didn't break early).
func (s *ExecutionService) shouldCheckLoopProblems(result *workflow.LoopResult, step *workflow.Step) bool {
	// Only check while loops with max iterations configured
	if result == nil || step.Type != workflow.StepTypeWhile || step.Loop.MaxIterations <= 0 {
		return false
	}

	// Check if loop completed by hitting max iterations (didn't break early)
	if result.TotalCount < step.Loop.MaxIterations || result.BrokeAt != -1 {
		return false
	}

	return true
}

// buildLoopFailureError constructs an error message based on detected loop patterns.
// It appends " with step failures" if failures occurred, or " with nested complexity" if complex steps exist.
func (s *ExecutionService) buildLoopFailureError(hadFailures, hasComplexSteps bool) string {
	errMsg := "loop reached maximum iterations"
	if hadFailures {
		errMsg += " with step failures"
	} else if hasComplexSteps {
		errMsg += " with nested complexity"
	}
	return errMsg
}

// executeLoopPostHooks executes post-failure hooks for a loop step.
// It logs a warning if hook execution fails but does not propagate the error.
func (s *ExecutionService) executeLoopPostHooks(ctx context.Context, step *workflow.Step, execCtx *workflow.ExecutionContext) {
	// Build interpolation context for hook execution
	intCtx := s.buildInterpolationContext(execCtx)

	// Execute post-hooks with failOnError to detect failures for logging purposes
	// (we still don't propagate the error to the caller)
	if err := s.hookExecutor.ExecuteHooks(ctx, step.Hooks.Post, intCtx, true); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}
}

// prepareStepExecution sets up the execution context for a step, including timeout,
// interpolation context building, and pre-hook execution.
// Returns the step context (with timeout if configured) and interpolation context.
// NOTE: Caller is responsible for calling returned cancel function to prevent context leak.
func (s *ExecutionService) prepareStepExecution(
	ctx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (stepCtx context.Context, intCtx *interpolation.Context, cancel context.CancelFunc, err error) {
	// apply step timeout
	stepCtx = ctx
	if step.Timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second) //nolint:gosec // G118: cancel is returned to caller who defers it
	}

	// build interpolation context
	intCtx = s.buildInterpolationContext(execCtx)

	// execute pre-hooks
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); hookErr != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", hookErr)
	}

	return stepCtx, intCtx, cancel, nil
}

// resolveStepCommand interpolates and resolves the step's command and directory,
// building a ports.Command struct ready for execution.
// Returns the fully resolved command or an interpolation error.
func (s *ExecutionService) resolveStepCommand(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	intCtx *interpolation.Context,
) (*ports.Command, error) {
	commandToResolve := step.Command
	if step.ScriptFile != "" {
		loadedContent, err := loadScriptFile(ctx, step.ScriptFile, wf, intCtx)
		if err != nil {
			return nil, err
		}
		commandToResolve = loadedContent
	}
	resolvedCmd, err := s.resolver.Resolve(commandToResolve, intCtx)
	if err != nil {
		return nil, fmt.Errorf("interpolate command: %w", err)
	}
	// FR-001: Apply local-over-global resolution to command field after interpolation
	resolvedCmd = resolveCommandAWFPaths(resolvedCmd, wf.SourceDir, s.awfPaths)

	// resolve dir if specified
	resolvedDir := ""
	if step.Dir != "" {
		resolvedDir, err = s.resolver.Resolve(step.Dir, intCtx)
		if err != nil {
			return nil, fmt.Errorf("interpolate dir: %w", err)
		}
		// FR-002: Apply local-over-global resolution to dir field after interpolation
		resolvedDir = resolveCommandAWFPaths(resolvedDir, wf.SourceDir, s.awfPaths)
	}

	// build command with env for secret masking
	env := make(map[string]string)
	for k, v := range intCtx.Inputs {
		// Convert input values to strings for env map
		if strVal, ok := v.(string); ok {
			env[k] = strVal
		}
	}

	cmd := &ports.Command{
		Program:      resolvedCmd,
		Dir:          resolvedDir,
		Env:          env,
		IsScriptFile: step.ScriptFile != "",
		Stdout:       s.stdoutWriter,
		Stderr:       s.stderrWriter,
	}

	return cmd, nil
}

// executeStepCommand executes the command with retry logic if configured.
// Returns the command result, attempt number, and any execution error.
func (s *ExecutionService) executeStepCommand(
	ctx context.Context,
	step *workflow.Step,
	cmd *ports.Command,
) (*ports.CommandResult, int, error) {
	ctx, span := s.startSpan(ctx, "shell.execute")
	defer span.End()
	if cmd != nil {
		span.SetAttribute("shell.command", cmd.Program)
	}

	// Handle nil executor for testing (temporary for T010 RED phase tests)
	if s.executor == nil {
		return nil, 0, nil
	}

	if step.Retry != nil && step.Retry.MaxAttempts > 1 {
		return s.executeWithRetry(ctx, step, cmd)
	}

	result, err := s.executor.Execute(ctx, cmd)
	return result, 1, err
}

// recordStepResult builds a workflow.StepState from execution timing and command result.
// Returns the step state with populated fields (without Status, which is set by outcome handlers).
func (s *ExecutionService) recordStepResult(
	step *workflow.Step,
	startTime time.Time,
	result *ports.CommandResult,
	attempt int,
) workflow.StepState {
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     attempt,
	}

	if result != nil {
		// C019: Apply output limits to prevent OOM from large outputs
		limitResult, err := s.outputLimiter.Apply(result.Stdout, result.Stderr)
		if err != nil {
			// Log error but don't fail the step - store raw output
			s.logger.Error("Failed to apply output limits", "step", step.Name, "error", err)
			state.Output = result.Stdout
			state.Stderr = result.Stderr
		} else {
			state.Output = limitResult.Output
			state.Stderr = limitResult.Stderr
			state.OutputPath = limitResult.OutputPath
			state.StderrPath = limitResult.StderrPath
			state.Truncated = limitResult.Truncated
		}
		state.ExitCode = result.ExitCode
	}

	return state
}

// handleExecutionError processes execution errors, distinguishing between workflow-level
// cancellation (propagate) and step-level failures (follow OnFailure/ContinueOnError).
// Updates state, executes post-hooks, and returns the next step or error.
func (s *ExecutionService) handleExecutionError(
	ctx context.Context,
	stepCtx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	state *workflow.StepState,
	execErr error,
) (string, error) {
	// Check if the PARENT context was cancelled (workflow-level cancellation)
	// This is different from step timeout (stepCtx deadline exceeded)
	// Step timeout should follow OnFailure, workflow cancellation should propagate
	// When workflow is cancelled, propagate error immediately regardless of execErr type
	if ctx.Err() != nil {
		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, *state)
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	state.Status = workflow.StatusFailed
	state.Error = execErr.Error()
	execCtx.SetStepState(step.Name, *state)

	// execute post-hooks even on failure
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	// Transitions take priority over ContinueOnError/OnFailure (ADR-001).
	// Pass ContinueOnError as success so the legacy fallback selects OnSuccess vs OnFailure correctly.
	nextStep, err := s.resolveNextStep(step, intCtx, step.ContinueOnError)
	if err != nil {
		return "", err
	}
	if nextStep != "" || step.ContinueOnError {
		return nextStep, nil
	}

	if step.OnFailure != "" {
		return step.OnFailure, nil
	}
	return "", fmt.Errorf("step %s: %w", step.Name, execErr)
}

// handleNonZeroExit processes non-zero exit codes, following OnFailure or ContinueOnError paths.
// Updates state, executes post-hooks, and returns the next step or error.
// Transitions are evaluated first (ADR-001); ContinueOnError/OnFailure are legacy fallback.
func (s *ExecutionService) handleNonZeroExit(
	stepCtx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	state *workflow.StepState,
	result *ports.CommandResult,
) (string, error) {
	state.Status = workflow.StatusFailed
	if result.Stderr != "" {
		state.Error = result.Stderr
	} else {
		state.Error = fmt.Sprintf("exit code %d", result.ExitCode)
	}
	execCtx.SetStepState(step.Name, *state)

	// execute post-hooks even on failure
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	// Transitions take priority over ContinueOnError/OnFailure (ADR-001).
	// Pass ContinueOnError as success so the legacy fallback selects OnSuccess vs OnFailure correctly.
	nextStep, err := s.resolveNextStep(step, intCtx, step.ContinueOnError)
	if err != nil {
		return "", err
	}
	if nextStep != "" || step.ContinueOnError {
		return nextStep, nil
	}

	if result.Stderr != "" {
		return "", fmt.Errorf("step %s: %s", step.Name, result.Stderr)
	}
	return "", fmt.Errorf("step %s: exit code %d", step.Name, result.ExitCode)
}

// handleSuccess processes successful step execution, executing post-hooks and resolving
// the next step via transitions or OnSuccess.
// Returns the next step name or error.
func (s *ExecutionService) handleSuccess(
	stepCtx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
	state *workflow.StepState,
) (string, error) {
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, *state)

	// execute post-hooks on success
	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); err != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", err)
	}

	// Resolve next step using transitions (if defined) or fallback to OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// IsProblematicMaxIterationPattern checks if a loop hit max iterations with problematic patterns.
// Returns true if the loop completed by hitting max iterations AND has step failures or complex nesting.
func (s *ExecutionService) IsProblematicMaxIterationPattern(
	result *workflow.LoopResult,
	step *workflow.Step,
	wf *workflow.Workflow,
) bool {
	// Guard clause: only check while loops with max iterations configured
	if !s.shouldCheckLoopProblems(result, step) {
		return false
	}

	// Check for step failures or complex body steps using extracted helper
	hadFailures, hasComplexSteps := s.detectLoopPatterns(result, wf)

	return hadFailures || hasComplexSteps
}

// HandleMaxIterationFailure handles the case when a loop hits max iterations with problematic patterns.
// If OnComplete is configured, completes successfully via that transition (e.g., retry patterns).
// Otherwise, treats as failure and returns error or transitions via OnFailure.
func (s *ExecutionService) HandleMaxIterationFailure(
	ctx context.Context,
	result *workflow.LoopResult,
	step *workflow.Step,
	wf *workflow.Workflow,
	execCtx *workflow.ExecutionContext,
	loopState *workflow.StepState,
) (string, error) {
	// Use extracted helper to determine pattern type
	hadFailures, hasComplexSteps := s.detectLoopPatterns(result, wf)

	// If OnComplete is configured, treat max iterations as successful completion
	// This supports retry patterns where on_failure loops back until max iterations
	if step.Loop != nil && step.Loop.OnComplete != "" {
		loopState.Status = workflow.StatusCompleted
		if result != nil {
			loopState.Output = fmt.Sprintf("completed %d iterations", result.TotalCount)
		}
		execCtx.SetStepState(step.Name, *loopState)

		// Use extracted helper to execute post-hooks
		s.executeLoopPostHooks(ctx, step, execCtx)

		return step.Loop.OnComplete, nil
	}

	// No OnComplete configured - treat as failure
	loopState.Status = workflow.StatusFailed

	// Use extracted helper to build error message
	errMsg := s.buildLoopFailureError(hadFailures, hasComplexSteps)
	loopState.Error = errMsg
	execCtx.SetStepState(step.Name, *loopState)

	// Use extracted helper to execute post-hooks
	s.executeLoopPostHooks(ctx, step, execCtx)

	if step.OnFailure != "" {
		return step.OnFailure, nil
	}
	return "", fmt.Errorf("while loop %s: %s", step.Name, errMsg)
}

// stepExecutorAdapter adapts ExecutionService to the ports.StepExecutor interface.
type stepExecutorAdapter struct {
	execSvc *ExecutionService
}

// ExecuteStep implements ports.StepExecutor.
func (a *stepExecutorAdapter) ExecuteStep(
	ctx context.Context,
	wf *workflow.Workflow,
	stepName string,
	execCtx *workflow.ExecutionContext,
) (*workflow.BranchResult, error) {
	step, ok := wf.Steps[stepName]
	if !ok {
		return nil, fmt.Errorf("step not found: %s", stepName)
	}

	startTime := time.Now()
	result := &workflow.BranchResult{
		Name:      stepName,
		StartedAt: startTime,
	}

	// Execute the step using the appropriate execution method based on step type
	var err error
	switch step.Type {
	case workflow.StepTypeAgent:
		_, err = a.execSvc.executeAgentStep(ctx, wf, step, execCtx)
	case workflow.StepTypeParallel:
		_, err = a.execSvc.executeParallelStep(ctx, wf, step, execCtx)
	case workflow.StepTypeCommand:
		_, err = a.execSvc.executeStep(ctx, wf, step, execCtx)
	case workflow.StepTypeForEach, workflow.StepTypeWhile:
		_, err = a.execSvc.executeLoopStep(ctx, wf, step, execCtx)
	case workflow.StepTypeOperation:
		_, err = a.execSvc.executePluginOperation(ctx, step, execCtx)
	case workflow.StepTypeCallWorkflow:
		_, err = a.execSvc.executeCallWorkflowStep(ctx, wf, step, execCtx)
	default:
		_, err = a.execSvc.executeCustomStepType(ctx, wf, step, execCtx)
	}

	result.CompletedAt = time.Now()

	// Get the step state that was recorded by the execution method
	if state, exists := execCtx.GetStepState(stepName); exists {
		result.Output = state.Output
		result.Stderr = state.Stderr
		result.ExitCode = state.ExitCode
		if state.Error != "" {
			result.Error = errors.New(state.Error)
		}
	}

	if err != nil {
		result.Error = err
	}

	return result, err
}

// validateInputs converts workflow.Input to validation.Input and validates.
func (s *ExecutionService) validateInputs(inputs map[string]any, defs []workflow.Input) error {
	valDefs := make([]validation.Input, len(defs))
	for i, d := range defs {
		valDefs[i] = validation.Input{
			Name:     d.Name,
			Type:     d.Type,
			Required: d.Required,
		}
		if d.Validation != nil {
			valDefs[i].Validation = &validation.Rules{
				Pattern:       d.Validation.Pattern,
				Enum:          d.Validation.Enum,
				Min:           d.Validation.Min,
				Max:           d.Validation.Max,
				FileExists:    d.Validation.FileExists,
				FileExtension: d.Validation.FileExtension,
			}
		}
	}
	return validation.ValidateInputs(inputs, valDefs)
}

// Resume continues an interrupted workflow execution from where it left off.
// It loads persisted state, validates resumability, merges input overrides,
// and continues execution from the resolved fromStep while skipping completed steps.
// fromStep may be "current", "previous", or a literal step name present in States.
// ErrExecutionNotFound is returned by Resume when the requested workflow execution
// record does not exist in the state store (Load returned nil without error).
// Callers should test with errors.Is(err, ErrExecutionNotFound) rather than
// inspecting the error message string.
var ErrExecutionNotFound = errors.New("execution not found")

func (s *ExecutionService) Resume(
	ctx context.Context,
	workflowID string,
	inputOverrides map[string]any,
	fromStep string,
) (*workflow.ExecutionContext, error) {
	if !tryBeginResume(workflowID) {
		return nil, fmt.Errorf("resume workflow %s: %w", workflowID, errResumeInProgress)
	}
	defer finishResume(workflowID)

	if leaseStore, ok := s.store.(ports.ResumeLeaseStore); ok {
		release, err := leaseStore.AcquireResumeLease(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("acquire resume lease: %w", err)
		}
		defer func() {
			if releaseErr := release(); releaseErr != nil {
				s.logger.Warn("release resume lease failed", "workflow_id", workflowID, "error", releaseErr)
			}
		}()
	}

	// 1. Load state
	execCtx, err := s.store.Load(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}
	if execCtx == nil {
		return nil, fmt.Errorf("workflow execution not found: %s: %w", workflowID, ErrExecutionNotFound)
	}

	// 2. Validate resumable (not completed)
	if execCtx.Status == workflow.StatusCompleted {
		return nil, fmt.Errorf("workflow already completed, cannot resume")
	}

	// 3. Load workflow definition
	wf, err := s.workflowSvc.GetWorkflow(ctx, execCtx.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("load workflow: %w", err)
	}

	// 4. Validate current step exists
	if _, exists := wf.Steps[execCtx.CurrentStep]; !exists {
		return nil, fmt.Errorf("cannot resume: step '%s' no longer exists in workflow", execCtx.CurrentStep)
	}

	// 5. Resolve the from-step target
	targetStep, err := s.resolveFromStep(execCtx, wf, fromStep)
	if err != nil {
		return nil, err
	}

	// Validate the effective inputs before changing state or emitting a new start.
	effectiveInputs := MergeInputs(execCtx.GetInputs(), inputOverrides)
	s.resumeValidationMu.Lock()
	validationErr := s.workflowSvc.ValidateLoadedWorkflowWithInputs(ctx, wf, execCtx.WorkflowName, ports.ValidateOptions{}, effectiveInputs)
	s.resumeValidationMu.Unlock()
	if validationErr != nil {
		return nil, fmt.Errorf("validate workflow before resume: %w", validationErr)
	}

	stagedCtx, err := cloneExecutionContext(execCtx)
	if err != nil {
		return nil, fmt.Errorf("stage resume state: %w", err)
	}
	execCtx = stagedCtx

	// Reconcile a terminal predecessor before replacing its durable attempt identity.
	// The detached context lets persistence finish even when the execution context
	// supplied to Resume has already been cancelled.
	_, previousAttempt, _, previousRecorded := execCtx.GetHistoryAttempt()
	if previousAttempt > 0 && !previousRecorded && isTerminalExecutionStatus(execCtx.GetStatus()) {
		if err := s.finalizeAttemptHistory(context.WithoutCancel(ctx), execCtx, historyErrorMessage(execCtx), nil); err != nil {
			return nil, fmt.Errorf("reconcile pending attempt history: %w", err)
		}
	}

	// 6. Stage cleanup, inputs, target, status, and attempt identity. The
	// mandatory allocation save below commits them as one durable snapshot.
	if targetStep != execCtx.CurrentStep {
		s.cleanupStatesAfter(execCtx, targetStep)
	}

	// 7. Merge input overrides
	for k, v := range inputOverrides {
		execCtx.SetInput(k, v)
	}

	// 8. Set current step to resolved target
	execCtx.CurrentStep = targetStep

	// 9. Reset status to running
	execCtx.Status = workflow.StatusRunning
	if err := s.prepareResumedHistoryAttempt(ctx, execCtx); err != nil {
		return nil, err
	}
	// 10. Execute from resolved step
	s.logger.Info("resuming workflow", "id", workflowID, "from", execCtx.CurrentStep)

	return execCtx, s.executeStartedAttempt(ctx, wf, execCtx, func() error {
		_, runErr := s.executeFromStep(ctx, wf, execCtx)
		return runErr
	})
}

func cloneExecutionContext(execCtx *workflow.ExecutionContext) (*workflow.ExecutionContext, error) {
	if execCtx == nil {
		return nil, errors.New("execution context cannot be nil")
	}
	var currentLoop *workflow.LoopContext
	if execCtx.CurrentLoop != nil {
		loopCopy := *execCtx.CurrentLoop
		currentLoop = &loopCopy
	}
	recordID, attempt, resumedFromID, recorded := execCtx.GetHistoryAttempt()
	return &workflow.ExecutionContext{
		WorkflowID:              execCtx.WorkflowID,
		WorkflowName:            execCtx.WorkflowName,
		ParentRunID:             execCtx.ParentRunID,
		Status:                  execCtx.GetStatus(),
		CurrentStep:             execCtx.GetCurrentStep(),
		ExitCode:                execCtx.ExitCode,
		Inputs:                  execCtx.GetInputs(),
		States:                  execCtx.GetAllStepStates(),
		Env:                     maps.Clone(execCtx.Env),
		StartedAt:               execCtx.StartedAt,
		UpdatedAt:               execCtx.GetUpdatedAt(),
		CompletedAt:             execCtx.GetCompletedAt(),
		CurrentLoop:             currentLoop,
		CallStack:               append([]string(nil), execCtx.CallStack...),
		HistoryRecordID:         recordID,
		HistoryAttempt:          attempt,
		PreviousHistoryRecordID: resumedFromID,
		HistoryRecorded:         recorded,
	}, nil
}

func isTerminalExecutionStatus(status workflow.ExecutionStatus) bool {
	return status == workflow.StatusCompleted || status == workflow.StatusFailed || status == workflow.StatusCancelled
}

func historyErrorMessage(execCtx *workflow.ExecutionContext) string {
	record := buildExecutionRecord(execCtx, "", 0, "")
	return record.ErrorMessage
}

// resolveFromStep resolves the --from flag value to a concrete step name.
// Accepts "current", "previous", or a literal step name that exists in the workflow.
func (s *ExecutionService) resolveFromStep(execCtx *workflow.ExecutionContext, _ *workflow.Workflow, fromStep string) (string, error) {
	switch fromStep {
	case "current":
		return execCtx.CurrentStep, nil
	case "previous":
		var bestName string
		var bestTime time.Time
		allStates := execCtx.GetAllStepStates()
		for name, state := range allStates { //nolint:gocritic // rangeValCopy: iterating defensive copy from GetAllStepStates, value semantics required for timestamp comparison
			if name == execCtx.CurrentStep {
				continue
			}
			if state.CompletedAt.IsZero() {
				continue
			}
			if state.CompletedAt.After(bestTime) || (state.CompletedAt.Equal(bestTime) && name > bestName) {
				bestTime = state.CompletedAt
				bestName = name
			}
		}
		if bestName == "" {
			return "", domainerrors.NewUserError(
				domainerrors.ErrorCodeUserInputValidationFailed,
				"no prior step: no completed step found to resume from",
				nil,
				nil,
			)
		}
		return bestName, nil
	default:
		allStates := execCtx.GetAllStepStates()
		if _, exists := allStates[fromStep]; !exists {
			return "", domainerrors.NewUserError(
				domainerrors.ErrorCodeUserInputValidationFailed,
				fmt.Sprintf("step not found in execution history: %s", fromStep),
				map[string]any{"step": fromStep},
				nil,
			)
		}
		return fromStep, nil
	}
}

// cleanupStatesAfter deletes all States entries whose CompletedAt is strictly after the target step's CompletedAt.
// The target step itself is preserved.
func (s *ExecutionService) cleanupStatesAfter(execCtx *workflow.ExecutionContext, targetStepName string) {
	targetState, ok := execCtx.GetStepState(targetStepName)
	if !ok {
		return
	}
	cutoff := targetState.CompletedAt
	allStates := execCtx.GetAllStepStates()
	for name, state := range allStates { //nolint:gocritic // rangeValCopy: iterating defensive copy from GetAllStepStates, value semantics required for timestamp comparison
		if name == targetStepName {
			continue
		}
		if state.CompletedAt.After(cutoff) {
			execCtx.DeleteStepState(name)
		}
	}
}

// ListResumable returns all workflow executions that can be resumed.
// A workflow is resumable if its status is not completed.
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

// executeFromStep continues workflow execution from the specified starting step.
// It handles the execution loop, hooks, and state transitions.
// Step dispatch is delegated to dispatchStep (same file) to keep the routing logic in one place.
//
//nolint:gocognit // Complexity 31: main execution loop orchestrates step dispatch, hooks, cancellation, and error handling as a cohesive unit.
func (s *ExecutionService) executeFromStep(
	ctx context.Context,
	wf *workflow.Workflow,
	execCtx *workflow.ExecutionContext,
) (*workflow.ExecutionContext, error) {
	var execErr error
	currentStep := execCtx.CurrentStep

	for {
		step, ok := wf.Steps[currentStep]
		if !ok {
			execCtx.Status = workflow.StatusFailed
			execErr = fmt.Errorf("step not found: %s", currentStep)
			break
		}

		execCtx.CurrentStep = currentStep

		if step.Type == workflow.StepTypeTerminal {
			// Check terminal status: failure or success (default)
			if step.Status == workflow.TerminalFailure {
				execCtx.Status = workflow.StatusFailed
				execCtx.ExitCode = step.ExitCode
				if msg := s.interpolateTerminalMessage(step.Message, s.buildInterpolationContext(execCtx)); msg != "" {
					execErr = errors.New(msg)
				} else {
					execErr = fmt.Errorf("workflow reached terminal failure state: %s", currentStep)
				}
			} else {
				execCtx.Status = workflow.StatusCompleted
			}
			s.logger.Info("workflow completed", "step", currentStep, "status", execCtx.Status)
			break
		}

		var nextStep string
		var err error

		s.logger.Debug("executing step", "step", step.Name)
		s.emitEvent(ctx, workflow.EventStepStarted, map[string]string{
			"workflow_id": execCtx.WorkflowID,
			"step_name":   step.Name,
		})

		nextStep, err = s.dispatchStep(ctx, wf, step, execCtx)
		if err != nil {
			s.emitEvent(ctx, workflow.EventStepFailed, map[string]string{
				"workflow_id": execCtx.WorkflowID,
				"step_name":   step.Name,
				"error":       err.Error(),
			})
			execCtx.Status = workflow.StatusFailed
			s.logger.Error("step failed", "step", step.Name, "error", err)
			execErr = err
			break
		}

		if ss, ok := execCtx.GetStepState(step.Name); ok && ss.Status == workflow.StatusFailed {
			s.emitEvent(ctx, workflow.EventStepFailed, map[string]string{
				"workflow_id": execCtx.WorkflowID,
				"step_name":   step.Name,
				"error":       ss.Error,
			})
		} else {
			s.emitEvent(ctx, workflow.EventStepCompleted, map[string]string{
				"workflow_id": execCtx.WorkflowID,
				"step_name":   step.Name,
			})
		}

		s.checkpoint(ctx, execCtx)

		currentStep = nextStep
	}

	return execCtx, execErr
}

// ErrNoStepTypeProvider is returned when a custom step type is executed without a configured provider.
var ErrNoStepTypeProvider = errors.New("step type provider not configured")

func (s *ExecutionService) executeCustomStepType(
	ctx context.Context,
	_ *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()
	s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
	defer s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)

	if s.stepTypeProvider == nil {
		state := workflow.StepState{
			Name:        step.Name,
			StartedAt:   startTime,
			CompletedAt: time.Now(),
			Status:      workflow.StatusFailed,
			Error:       ErrNoStepTypeProvider.Error(),
			Attempt:     1,
		}
		execCtx.SetStepState(step.Name, state)
		return "", fmt.Errorf("step %s: %w", step.Name, ErrNoStepTypeProvider)
	}

	if !s.stepTypeProvider.HasStepType(string(step.Type)) {
		errMsg := fmt.Sprintf("unsupported step type %q", step.Type)
		state := workflow.StepState{
			Name:        step.Name,
			StartedAt:   startTime,
			CompletedAt: time.Now(),
			Status:      workflow.StatusFailed,
			Error:       errMsg,
			Attempt:     1,
		}
		execCtx.SetStepState(step.Name, state)
		return "", fmt.Errorf("step %s: %s", step.Name, errMsg)
	}

	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	intCtx := s.buildInterpolationContext(execCtx)
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	req := ports.StepExecuteRequest{
		StepName: step.Name,
		StepType: string(step.Type),
		Config:   step.Config,
		Inputs:   execCtx.Inputs,
	}

	s.logger.Debug("executing custom step type", "step", step.Name, "type", step.Type)
	result, execErr := s.stepTypeProvider.ExecuteStep(stepCtx, req)

	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Output:      result.Output,
		Data:        result.Data,
		ExitCode:    result.ExitCode,
		Attempt:     1,
	}

	if execErr != nil {
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		execCtx.SetStepState(step.Name, state)

		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	if result.ExitCode != 0 {
		execCtx.SetStepState(step.Name, state)
		// Route through handleNonZeroExit for transition evaluation (F068)
		cmdResult := &ports.CommandResult{
			Stdout:   result.Output,
			ExitCode: result.ExitCode,
		}
		return s.handleNonZeroExit(stepCtx, step, execCtx, &state, cmdResult)
	}

	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	return s.resolveNextStep(step, intCtx, true)
}

// ErrNoOperationProvider is returned when an operation step is executed without a configured provider.
var ErrNoOperationProvider = errors.New("operation provider not configured")

// executePluginOperation executes a plugin-provided operation step.
// F021: Plugin System - Executes operations registered by plugins.
// Returns ErrNoOperationProvider if no operation provider is configured.
//
//nolint:gocognit // Complexity 31: plugin operation executor handles validation, resolution, state management. Plugin interface requires comprehensive error handling.
func (s *ExecutionService) executePluginOperation(
	ctx context.Context,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	ctx, span := s.startSpan(ctx, "plugin.rpc")
	defer span.End()
	span.SetAttribute("plugin.name", step.Operation)
	s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
	defer s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)

	// Validate provider is configured
	if s.operationProvider == nil {
		return "", fmt.Errorf("step %s: %w", step.Name, ErrNoOperationProvider)
	}

	// Check if operation's plugin is disabled before attempting lookup
	if s.pluginSvc != nil {
		parts := strings.SplitN(step.Operation, ".", 2)
		if len(parts) == 2 {
			pluginPrefix := parts[0]
			if !s.pluginSvc.IsPluginEnabled(pluginPrefix) {
				return "", domainerrors.NewExecutionError(
					domainerrors.ErrorCodeExecutionPluginDisabled,
					fmt.Sprintf("step %s: plugin %q is disabled", step.Name, pluginPrefix),
					map[string]any{
						"plugin":    pluginPrefix,
						"operation": step.Operation,
						"step":      step.Name,
					},
					nil,
				)
			}
		}
	}

	// Validate operation exists
	_, found := s.operationProvider.GetOperation(step.Operation)
	if !found {
		return "", fmt.Errorf("step %s: operation %q not found", step.Name, step.Operation)
	}

	// Apply step timeout
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Build interpolation context
	intCtx := s.buildInterpolationContext(execCtx)

	// Execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// Resolve operation inputs via interpolation
	resolvedInputs, err := s.resolveOperationInputs(step.OperationInputs, intCtx)
	if err != nil {
		return "", fmt.Errorf("step %s: resolve inputs: %w", step.Name, err)
	}

	// Execute the operation
	s.logger.Debug("executing plugin operation", "step", step.Name, "operation", step.Operation)
	result, execErr := s.operationProvider.Execute(stepCtx, step.Operation, resolvedInputs)

	// Record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	// Serialize operation outputs to step output and Response (for interpolation)
	if result != nil && result.Outputs != nil {
		state.Output = s.serializeOperationOutputs(result.Outputs)
		state.Response = result.Outputs
	}

	// Handle execution error (e.g., context canceled, provider error)
	if execErr != nil {
		// Check if parent context was cancelled (workflow-level cancellation)
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		s.logger.Warn("operation failed", "step", step.Name, "operation", step.Operation, "error", execErr)
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	// Handle operation failure (Success=false in result)
	if result != nil && !result.Success {
		state.Status = workflow.StatusFailed
		if result.Error != "" {
			state.Error = result.Error
		}
		s.logger.Warn("operation returned failure", "step", step.Name, "operation", step.Operation, "error", result.Error)
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		errMsg := "operation failed"
		if result.Error != "" {
			errMsg = result.Error
		}
		return "", fmt.Errorf("step %s: %s", step.Name, errMsg)
	}

	// Success
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// Execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// ErrNoAgentRegistry is returned when an agent step is executed without a configured registry.
var ErrNoAgentRegistry = errors.New("agent registry not configured")

// executeAgentStep executes an AI agent step.
// F039: Agent Step Type - Executes AI provider operations via CLI interfaces.
// Returns ErrNoAgentRegistry if no agent registry is configured.
//
//nolint:gocognit // Complexity 39: agent step execution handles conversation/single modes, retries, context windows, token management. Inherent to agent orchestration.
func (s *ExecutionService) executeAgentStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	ctx, span := s.startSpan(ctx, "agent.call")
	defer span.End()
	s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepStarted)
	defer s.emitTranscriptStep(ctx, execCtx, step, transcript.EventTypeStepCompleted)

	// Validate registry is configured
	if s.agentRegistry == nil {
		// Record failed state before returning error to maintain consistent state tracking
		state := workflow.StepState{
			Name:        step.Name,
			StartedAt:   startTime,
			CompletedAt: time.Now(),
			Status:      workflow.StatusFailed,
			Error:       ErrNoAgentRegistry.Error(),
			Attempt:     1,
		}
		execCtx.SetStepState(step.Name, state)
		return "", fmt.Errorf("step %s: %w", step.Name, ErrNoAgentRegistry)
	}

	// Validate agent config exists
	if step.Agent == nil {
		return "", fmt.Errorf("step %s: agent configuration missing", step.Name)
	}

	// Apply step timeout (use agent timeout if step timeout not set)
	stepCtx := ctx
	timeout := step.Timeout
	if timeout == 0 && step.Agent.Timeout > 0 {
		timeout = step.Agent.Timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	// Inject the per-step display renderer (ACP typed streaming). No-op for all other
	// entry points, which never set a factory. Covers the conversation-substruct,
	// resumable, and interactive (executeConversationStep) paths — all use stepCtx.
	if s.displayRendererFactory != nil {
		if r := s.displayRendererFactory(step.Name); r != nil {
			stepCtx = display.WithRenderer(stepCtx, r)
		}
	}

	// Build interpolation context
	intCtx := s.buildInterpolationContext(execCtx)

	// Execute pre-hooks
	if err := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Pre, intCtx, false); err != nil {
		s.logger.Warn("pre-hook failed", "step", step.Name, "error", err)
	}

	// F033: Route to conversation execution if mode is "conversation"
	if step.Agent.Mode == "conversation" {
		return s.executeConversationStep(stepCtx, wf, step, execCtx)
	}

	// F063: Load prompt from file if prompt_file is specified
	promptToResolve := step.Agent.Prompt
	if step.Agent.PromptFile != "" {
		loadedPrompt, err := loadPromptFile(stepCtx, step.Agent.PromptFile, wf, intCtx)
		if err != nil {
			return "", fmt.Errorf("step %s: load prompt file: %w", step.Name, err)
		}
		promptToResolve = loadedPrompt
	}

	// Resolve prompt via interpolation
	resolvedPrompt, err := s.resolver.Resolve(promptToResolve, intCtx)
	if err != nil {
		return "", fmt.Errorf("step %s: resolve prompt: %w", step.Name, err)
	}

	if len(step.Skills) > 0 && s.skillRepo != nil {
		skillContent, skillErr := ResolveAndFormatSkills(stepCtx, s.skillRepo, step.Skills, wf.SourceDir)
		if skillErr != nil {
			return "", fmt.Errorf("step %s: resolve skills: %w", step.Name, skillErr)
		}
		if skillContent != "" {
			resolvedPrompt = skillContent + "\n\n" + resolvedPrompt
		}
	}

	resolvedProvider, err := s.resolver.Resolve(step.Agent.Provider, intCtx)
	if err != nil {
		return "", fmt.Errorf("step %s: resolve provider: %w", step.Name, err)
	}

	provider, err := s.agentRegistry.Get(resolvedProvider)
	if err != nil {
		return "", fmt.Errorf("step %s: %w", step.Name, err)
	}

	// Audit log if dangerously_skip_permissions is enabled
	if step.Agent.Options != nil {
		if skipPerms, ok := step.Agent.Options["dangerously_skip_permissions"].(bool); ok && skipPerms {
			s.logger.Warn("dangerously_skip_permissions enabled",
				"workflow", execCtx.WorkflowName,
				"step", step.Name,
				"provider", resolvedProvider,
				"timestamp", time.Now().Format(time.RFC3339))
		}
	}

	// Execute the agent
	s.logger.Debug("executing agent step", "step", step.Name, "provider", resolvedProvider)
	opts, err := s.resolveAgentOptions(step.Agent.Options, intCtx)
	if err != nil {
		return "", fmt.Errorf("step %s: resolve agent options: %w", step.Name, err)
	}
	opts = cloneAndInjectOutputFormat(opts, step.Agent.OutputFormat)

	composedPrompt, roleErr := s.buildRoleAwareSystemPrompt(stepCtx, step, wf.SourceDir, intCtx)
	if roleErr != nil {
		return "", fmt.Errorf("step %s: resolve role: %w", step.Name, roleErr)
	}
	s.emitTranscriptAgentMessage(ctx, execCtx, resolvedPrompt, composedPrompt)
	if composedPrompt != "" {
		if _, exists := opts["system_prompt"]; exists {
			s.logger.Warn("options.system_prompt overridden by composed role+system_prompt", "step", step.Name)
		}
		opts["system_prompt"] = composedPrompt
	}

	// F099: Start MCP tool proxy if configured for this step. Injects the temp config
	// path into opts so the provider's MCP injector can reference it; cleanup runs after
	// provider.Execute / executeResumableAgentCall returns.
	proxyCleanup, proxyErr := s.startToolProxy(stepCtx, step, opts, resolvedProvider, provider, execCtx)
	if proxyErr != nil {
		return "", fmt.Errorf("step %s: %w", step.Name, proxyErr)
	}
	defer func() {
		if cleanupErr := proxyCleanup(); cleanupErr != nil {
			s.logger.Warn("tool proxy cleanup failed", "step", step.Name, "error", cleanupErr)
		}
	}()

	// Record step state
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	// When the step declares a conversation sub-struct, route through
	// provider.ExecuteConversation to enable session tracking and continue_from
	// resume. This is a single-turn call (no interactive loop) — the loop only
	// applies when mode == "conversation".
	var result *workflow.AgentResult
	var execErr error
	if step.Agent.Conversation != nil {
		var convResult *workflow.ConversationResult
		convResult, execErr = s.executeResumableAgentCall(stepCtx, step, provider, resolvedProvider, resolvedPrompt, opts, execCtx, wf.SourceDir)
		if convResult != nil {
			state.Output = convResult.Output
			state.DisplayOutput = convResult.DisplayOutput
			state.Response = convResult.Response
			state.TokensUsed = convResult.TokensTotal
			state.TokensInput = convResult.TokensInput
			state.TokensOutput = convResult.TokensOutput
			state.TokensEstimated = convResult.TokensEstimated
			state.Conversation = convResult.State
			result = &workflow.AgentResult{
				Provider:    convResult.Provider,
				Output:      convResult.Output,
				RawOutput:   convResult.RawOutput,
				Response:    convResult.Response,
				Tokens:      convResult.TokensTotal,
				StartedAt:   convResult.StartedAt,
				CompletedAt: convResult.CompletedAt,
			}
			if formatErr := s.applyOutputFormat(step, &state, execCtx); formatErr != nil {
				return "", formatErr
			}
		}
	} else {
		result, execErr = provider.Execute(stepCtx, resolvedPrompt, opts, s.stdoutWriter, s.stderrWriter)
		if result != nil {
			state.Output = result.Output
			state.DisplayOutput = result.DisplayOutput
			// AC5: JSON auto-parsed to states.step_name.Response
			state.Response = result.Response
			// AC6: Token usage in states.step_name.tokens_used
			state.TokensUsed = result.Tokens
			state.TokensOutput = result.Tokens
			state.TokensEstimated = result.TokensEstimated

			// F065: Apply output format post-processing
			if formatErr := s.applyOutputFormat(step, &state, execCtx); formatErr != nil {
				return "", formatErr
			}
		}
	}

	if result != nil {
		span.SetAttribute("provider", resolvedProvider)
		if model, ok := opts["model"].(string); ok && model != "" {
			span.SetAttribute("model", model)
		}
		span.SetAttribute("tokens_used", result.Tokens)
		// F106 US2: normalize the provider's RAW stream (NDJSON) into a message.assistant
		// event, falling back to the extracted text for providers with no raw stream.
		// Emitted before error handling so partial/dangling output is still captured.
		s.emitTranscriptAgentResponse(ctx, execCtx, resolvedProvider, result.RawOutput, result.Output)
	}

	// Handle execution error (e.g., context canceled, provider error)
	if execErr != nil {
		// Check if parent context was cancelled (workflow-level cancellation)
		if ctx.Err() != nil && (errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded)) {
			state.Status = workflow.StatusFailed
			state.Error = execErr.Error()
			if result != nil {
				state.Response = result.Response
				state.TokensUsed = result.Tokens
			}
			execCtx.SetStepState(step.Name, state)
			return "", fmt.Errorf("step %s: %w", step.Name, execErr)
		}

		state.Status = workflow.StatusFailed
		state.Error = execErr.Error()
		if result != nil {
			state.Response = result.Response
			state.TokensUsed = result.Tokens
		}
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: %w", step.Name, execErr)
	}

	// Handle agent result error
	if result != nil && result.Error != nil {
		state.Status = workflow.StatusFailed
		state.Error = result.Error.Error()
		execCtx.SetStepState(step.Name, state)

		// Execute post-hooks even on failure
		intCtx = s.buildInterpolationContext(execCtx)
		if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
			s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
		}

		if step.ContinueOnError {
			return step.OnSuccess, nil
		}
		if step.OnFailure != "" {
			return step.OnFailure, nil
		}
		return "", fmt.Errorf("step %s: agent error: %w", step.Name, result.Error)
	}

	// Success
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// Execute post-hooks on success
	intCtx = s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(stepCtx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// executeResumableAgentCall runs a single-turn agent call with conversation
// state tracking. It is used when a single-mode agent step declares a
// `conversation:` sub-struct, which opts the step into session tracking —
// either to establish a new session (continue_from empty) or to resume a
// prior step's session (continue_from set). Unlike executeConversationStep,
// this does not enter the interactive user-input loop; it runs exactly one
// agent turn and returns.
// resolveAgentOptions renders string values in agent options with the same
// interpolation context as the prompt, including nested maps and slices.
func (s *ExecutionService) resolveAgentOptions(options map[string]any, intCtx *interpolation.Context) (map[string]any, error) {
	if options == nil {
		return nil, nil
	}

	resolved := make(map[string]any, len(options))
	for key, value := range options {
		resolvedValue, err := s.resolveAgentOptionValue(value, intCtx)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		resolved[key] = resolvedValue
	}

	return resolved, nil
}

func (s *ExecutionService) resolveAgentOptionValue(value any, intCtx *interpolation.Context) (any, error) {
	switch v := value.(type) {
	case string:
		return s.resolver.Resolve(v, intCtx)
	case map[string]any:
		return s.resolveAgentOptions(v, intCtx)
	case []any:
		resolved := make([]any, len(v))
		for i, item := range v {
			itemValue, err := s.resolveAgentOptionValue(item, intCtx)
			if err != nil {
				return nil, err
			}
			resolved[i] = itemValue
		}
		return resolved, nil
	default:
		return value, nil
	}
}

func (s *ExecutionService) executeResumableAgentCall(
	ctx context.Context,
	step *workflow.Step,
	provider ports.AgentProvider,
	resolvedProvider string,
	resolvedPrompt string,
	opts map[string]any,
	execCtx *workflow.ExecutionContext,
	workflowDir string,
) (*workflow.ConversationResult, error) {
	// Resolve composed prompt BEFORE state init so the persisted state
	// carries the full role+inline system prompt (mirrors the ConversationManager fix).
	composed, roleErr := s.buildRoleAwareSystemPrompt(ctx, step, workflowDir, s.buildInterpolationContext(execCtx))
	if roleErr != nil {
		return nil, fmt.Errorf("resolve role: %w", roleErr)
	}
	state, err := s.buildResumableState(step, resolvedProvider, execCtx, composed)
	if err != nil {
		return nil, err
	}
	// Providers consume system_prompt via the options map (same convention as
	// ConversationManager). Inject it on fresh sessions only; on resumed
	// sessions the provider retains the system prompt from the prior turn.
	if composed != "" && state.SessionID == "" {
		opts["system_prompt"] = composed
	}
	return provider.ExecuteConversation(ctx, state, resolvedPrompt, opts, s.stdoutWriter, s.stderrWriter)
}

// buildResumableState constructs the conversation state seed for a resumable
// agent call. When continue_from is set, it clones the referenced step's
// conversation state; otherwise it creates a fresh state with the system prompt.
func (s *ExecutionService) buildResumableState(
	step *workflow.Step,
	resolvedProvider string,
	execCtx *workflow.ExecutionContext,
	composedSystemPrompt string,
) (*workflow.ConversationState, error) {
	cfg := step.Agent.Conversation
	if cfg == nil || cfg.ContinueFrom == "" {
		return workflow.NewConversationState(composedSystemPrompt), nil
	}

	priorStepState, ok := execCtx.GetStepState(cfg.ContinueFrom)
	if !ok {
		return nil, fmt.Errorf("continue_from: step %q not found in execution context", cfg.ContinueFrom)
	}
	if priorStepState.Conversation == nil {
		return nil, fmt.Errorf("continue_from: step %q has no conversation state", cfg.ContinueFrom)
	}
	prior := priorStepState.Conversation
	if prior.SessionID == "" && len(prior.Turns) == 0 {
		return nil, fmt.Errorf("continue_from: step %q has no session ID or conversation history to resume", cfg.ContinueFrom)
	}
	if resolvedProvider == openAICompatibleProviderName && len(prior.Turns) == 0 {
		return nil, fmt.Errorf("continue_from: step %q has no conversation turns for HTTP-based provider %q", cfg.ContinueFrom, resolvedProvider)
	}

	cloned := &workflow.ConversationState{
		SessionID:   prior.SessionID,
		Turns:       make([]workflow.Turn, len(prior.Turns)),
		TotalTurns:  prior.TotalTurns,
		TotalTokens: prior.TotalTokens,
	}
	copy(cloned.Turns, prior.Turns)
	return cloned, nil
}

// executeConversationStep orchestrates a multi-turn agent conversation following F033 spec.
// It delegates to ConversationManager which handles:
// - Turn iteration with conversation history
// - Context window management (token counting, truncation strategies)
// - Stop condition evaluation (expression-based or max limits)
// - Conversation state persistence in step state
//
// Flow:
//  1. Validate conversation manager is configured
//  2. Extract conversation config from step.Agent.Conversation
//  3. Delegate to ConversationManager.ExecuteConversation
//  4. Map ConversationResult to StepState
//  5. Execute hooks and resolve next step
//
// F051: T009 - Implement delegation to ConversationManager
func (s *ExecutionService) executeConversationStep(
	ctx context.Context,
	wf *workflow.Workflow,
	step *workflow.Step,
	execCtx *workflow.ExecutionContext,
) (string, error) {
	startTime := time.Now()

	// 1. Validate conversation manager is configured
	if s.conversationMgr == nil {
		return "", errors.New("conversation manager not configured")
	}

	// 2. Validate agent config exists
	if step.Agent == nil {
		return "", errors.New("agent config is nil")
	}

	// 3. Create buildContext closure for interpolation
	buildContext := func(ec *workflow.ExecutionContext) *interpolation.Context {
		return s.buildInterpolationContext(ec)
	}

	// 5. Delegate to ConversationManager
	s.logger.Debug("executing conversation step", "step", step.Name, "provider", step.Agent.Provider)
	result, err := s.conversationMgr.ExecuteConversation(
		ctx,
		step,
		step.Agent.Conversation,
		execCtx,
		buildContext,
		wf.SourceDir,
		s.stdoutWriter,
		s.stderrWriter,
	)

	// 6. Map ConversationResult to StepState
	state := workflow.StepState{
		Name:        step.Name,
		StartedAt:   startTime,
		CompletedAt: time.Now(),
		Attempt:     1,
	}

	if result != nil {
		state.Output = result.Output
		state.DisplayOutput = result.DisplayOutput
		state.Response = result.Response
		state.TokensUsed = result.TokensTotal
		state.Conversation = result.State

		// F065: Apply output format post-processing
		if formatErr := s.applyOutputFormat(step, &state, execCtx); formatErr != nil {
			return "", formatErr
		}
	}

	// 7. Handle execution error
	if err != nil {
		state.Status = workflow.StatusFailed
		state.Error = err.Error()
		execCtx.SetStepState(step.Name, state)
		return "", fmt.Errorf("agent conversation step %q failed: %w", step.Name, err)
	}

	// 8. Mark as completed
	state.Status = workflow.StatusCompleted
	execCtx.SetStepState(step.Name, state)

	// 9. Execute post-hooks on success
	intCtx := s.buildInterpolationContext(execCtx)
	if hookErr := s.hookExecutor.ExecuteHooks(ctx, step.Hooks.Post, intCtx, false); hookErr != nil {
		s.logger.Warn("post-hook failed", "step", step.Name, "error", hookErr)
	}

	// 10. Resolve next step using transitions or OnSuccess
	return s.resolveNextStep(step, intCtx, true)
}

// cloneAndInjectOutputFormat shallow-clones opts and injects output_format as string.
// The original map is never mutated (FR-009). Precedence: an explicit
// options["output_format"] set by the user wins (display-only intent); otherwise
// the top-level step.Agent.OutputFormat is injected; otherwise defaults to text.
// This keeps F065 post-processing (top-level) decoupled from F082 display intent (options).
func cloneAndInjectOutputFormat(opts map[string]any, format workflow.OutputFormat) map[string]any {
	cloned := make(map[string]any, len(opts)+2)
	maps.Copy(cloned, opts)
	if _, userSet := cloned["output_format"]; userSet {
		return cloned
	}
	if format == workflow.OutputFormatNone {
		format = workflow.OutputFormatText
	}
	cloned["output_format"] = string(format)
	return cloned
}

// resolveOperationInputs resolves all string values in operation inputs via interpolation.
func (s *ExecutionService) resolveOperationInputs(
	inputs map[string]any,
	intCtx *interpolation.Context,
) (map[string]any, error) {
	if inputs == nil {
		return make(map[string]any), nil
	}

	resolved := make(map[string]any, len(inputs))
	for key, value := range inputs {
		resolvedValue, err := s.resolveOperationValue(value, intCtx)
		if err != nil {
			return nil, fmt.Errorf("input %q: %w", key, err)
		}
		resolved[key] = resolvedValue
	}
	return resolved, nil
}

// resolveOperationValue recursively resolves interpolation in operation input values.
func (s *ExecutionService) resolveOperationValue(value any, intCtx *interpolation.Context) (any, error) {
	switch v := value.(type) {
	case string:
		return s.resolver.Resolve(v, intCtx)
	case map[string]any:
		resolved := make(map[string]any, len(v))
		for k, val := range v {
			resolvedVal, err := s.resolveOperationValue(val, intCtx)
			if err != nil {
				return nil, err
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil
	case []any:
		resolved := make([]any, len(v))
		for i, val := range v {
			resolvedVal, err := s.resolveOperationValue(val, intCtx)
			if err != nil {
				return nil, err
			}
			resolved[i] = resolvedVal
		}
		return resolved, nil
	default:
		// Non-string primitives (int, bool, etc.) pass through unchanged
		return value, nil
	}
}

// serializeOperationOutputs converts operation outputs to a string for step state.
func (s *ExecutionService) serializeOperationOutputs(outputs map[string]any) string {
	if len(outputs) == 0 {
		return ""
	}
	// Simple key=value format for now
	var result strings.Builder
	first := true
	for k, v := range outputs {
		if !first {
			result.WriteString("\n")
		}
		fmt.Fprintf(&result, "%s=%v", k, v)
		first = false
	}
	return result.String()
}

// classifyErrorType categorizes errors into types matching CLI exit code taxonomy.
// Returns: "execution", "workflow", "user", or "system".
//
// C-2 fix: typed error inspection takes priority over string matching.
// Priority order:
//  1. context.Canceled / context.DeadlineExceeded — stdlib sentinels, always reliable.
//  2. domainerrors.StructuredError — domain-layer typed errors with an ErrorCode category.
//  3. String matching on err.Error() — fallback for unstructured errors from external
//     processes (shell executor stderr, plugin output). These cannot carry typed codes
//     and their messages are the only signal available. Limited to well-known patterns.
func classifyErrorType(err error) string {
	if err == nil {
		return ""
	}

	// 1. Context errors: deadline/timeout and cancellation are execution-layer events.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "execution"
	}

	// 2. StructuredError carries an ErrorCode whose Category() maps directly to our taxonomy.
	var se *domainerrors.StructuredError
	if errors.As(err, &se) {
		switch se.Code.Category() {
		case "USER":
			return "user"
		case "WORKFLOW":
			return "workflow"
		case "EXECUTION":
			return "execution"
		case "SYSTEM":
			return "system"
		}
	}

	// 3. String-based fallback for unstructured external-process errors (shell, plugin RPC).
	// Checked in specificity order so "terminal failure" wins over "exit code" when both
	// are present. This matches the original intent without relying solely on string matching
	// for structured domain errors.
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "terminal failure"),
		strings.Contains(errStr, "step not found"),
		strings.Contains(errStr, "invalid state"),
		strings.Contains(errStr, "cycle detected"):
		return "workflow"
	case strings.Contains(errStr, "not found"),
		strings.Contains(errStr, "missing"),
		strings.Contains(errStr, "invalid input"),
		strings.Contains(errStr, "validation"):
		return "user"
	case strings.Contains(errStr, "permission"),
		strings.Contains(errStr, "access denied"),
		strings.Contains(errStr, "IO error"),
		strings.Contains(errStr, "file system"):
		return "system"
	default:
		// exit code, timeout, command failed, and any other execution-layer errors.
		return "execution"
	}
}

// applyOutputFormat applies output_format post-processing to agent step output.
// It strips code fences and optionally validates/parses JSON, updating state accordingly.
func (s *ExecutionService) applyOutputFormat(step *workflow.Step, state *workflow.StepState, execCtx *workflow.ExecutionContext) error {
	if step.Agent.OutputFormat == workflow.OutputFormatNone {
		return nil
	}
	processedOutput, parsedJSON, formatErr := output.ProcessOutputFormat(state.Output, string(step.Agent.OutputFormat))
	if formatErr != nil {
		state.Status = workflow.StatusFailed
		state.Error = formatErr.Error()
		execCtx.SetStepState(step.Name, *state)
		return fmt.Errorf("step %s: output format processing: %w", step.Name, formatErr)
	}
	state.Output = processedOutput
	if parsedJSON != nil {
		state.JSON = parsedJSON
	}
	return nil
}

// interpolateTerminalMessage interpolates a terminal step message template using the current execution context.
// Returns the interpolated message, falling back to the raw template on interpolation error.
// Delegates to the package-level helper; see interpolation_helpers.go.
func (s *ExecutionService) interpolateTerminalMessage(message string, intCtx *interpolation.Context) string {
	return interpolateTerminalMessage(s.resolver, s.logger, message, intCtx)
}
