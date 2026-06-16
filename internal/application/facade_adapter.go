package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	domainerrors "github.com/awf-project/cli/internal/domain/errors"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/domain/transcript"
	"github.com/awf-project/cli/internal/domain/workflow"
	"github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/google/uuid"
)

// compile-time checks: Adapter implements both WorkflowFacade (SC-001, D15),
// SingleStepRunner (the CLI-only awf run --step path, M1), WorkflowReader,
// and BatchValidator (the CLI-only awf validate --dir/--pack path, M1).
var (
	_ ports.WorkflowFacade   = (*Adapter)(nil)
	_ ports.SingleStepRunner = (*Adapter)(nil)
	_ ports.WorkflowReader   = (*Adapter)(nil)
	_ ports.BatchValidator   = (*Adapter)(nil)
)

// Adapter implements ports.WorkflowFacade by composing services without modifying them (D17).
// It is the sole caller of ports.Recorder.Subscribe — one Subscribe per RunSession (D15, SC-001).
//
// Thread-safety: the optional edge fields (userInputReader, runRecorderFactory) are protected
// by mu. Set* methods acquire the write lock; getters used inside Run/Resume goroutines acquire
// the read lock. The core dependencies (workflowSvc, executionSvc, historySvc, resolver,
// recorder, registry) are set once at construction and are read-only thereafter — no lock needed.
type Adapter struct {
	workflowSvc  *WorkflowService
	executionSvc *ExecutionService
	historySvc   *HistoryService
	resolver     *Resolver
	recorder     ports.Recorder
	registry     *SessionRegistry

	// mu guards the optional edge fields below. Set* acquire the write lock;
	// internal getters (getUserInputReader, getRunRecorderFactory) acquire the read
	// lock so Run/Resume goroutines are safe against concurrent Set* calls.
	mu sync.RWMutex

	// 2 edge bridges (D16) — all optional, nil-safe.
	// eventPublisher, outputWriters, and displayRenderer are NOT wired here:
	// those edges go through ExecutionSetup → ExecutionService directly (execution_setup.go).
	// Only the two bridges that must reach the session lifecycle live on the Adapter.
	userInputReader ports.UserInputReader

	// runRecorderFactory, when set, produces a fresh per-run Recorder keyed by runID.
	// It exists so concurrent async interfaces (HTTP, TUI) get LIVE step/message events:
	// each run owns its own recorder, the execution emits to it (routed via withRecorder),
	// and this session subscribes to it — with no cross-run contamination a single shared
	// recorder would cause. When nil (CLI, ACP) the Adapter uses its shared recorder.
	runRecorderFactory func(runID string) (ports.Recorder, error)
}

// NewAdapter constructs a new Adapter. All parameters are required; optional edge fields
// are configured via the Set* methods after construction.
func NewAdapter(
	workflowSvc *WorkflowService,
	executionSvc *ExecutionService,
	historySvc *HistoryService,
	resolver *Resolver,
	recorder ports.Recorder,
	registry *SessionRegistry,
) *Adapter {
	return &Adapter{
		workflowSvc:  workflowSvc,
		executionSvc: executionSvc,
		historySvc:   historySvc,
		resolver:     resolver,
		recorder:     recorder,
		registry:     registry,
	}
}

// SetUserInputReader wires an optional user-input reader for interactive conversations.
func (a *Adapter) SetUserInputReader(r ports.UserInputReader) {
	a.mu.Lock()
	a.userInputReader = r
	a.mu.Unlock()
}

// SetRunRecorderFactory wires a per-run Recorder factory (HTTP/TUI). With it set, every
// Run/Resume creates its own recorder so live step/message events flow to the right session
// under concurrency. Without it the Adapter uses its single shared recorder (CLI/ACP).
func (a *Adapter) SetRunRecorderFactory(f func(runID string) (ports.Recorder, error)) {
	a.mu.Lock()
	a.runRecorderFactory = f
	a.mu.Unlock()
}

// getUserInputReader returns the userInputReader under a read lock.
func (a *Adapter) getUserInputReader() ports.UserInputReader {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.userInputReader
}

// getRunRecorderFactory returns the runRecorderFactory under a read lock.
func (a *Adapter) getRunRecorderFactory() func(runID string) (ports.Recorder, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.runRecorderFactory
}

// List returns every discoverable workflow (local, global, env, and packs) via the
// workflow service, mapped to lightweight summaries.
func (a *Adapter) List(ctx context.Context) ([]ports.WorkflowSummary, error) {
	entries, err := a.workflowSvc.ListAllWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}
	summaries := make([]ports.WorkflowSummary, len(entries))
	for i := range entries {
		summaries[i] = ports.WorkflowSummary{
			Name:        entries[i].Name,
			Scope:       entries[i].Scope,
			Workflow:    entries[i].Workflow,
			Description: entries[i].Description,
			Version:     entries[i].Version,
		}
	}
	return summaries, nil
}

// Validate resolves the canonical identifier (FR-019) and reports validity.
// A resolver rejection (e.g. empty identifier) propagates as the validation error.
func (a *Adapter) Validate(ctx context.Context, req ports.RunRequest) (ports.ValidationReport, error) {
	if a.resolver == nil {
		return ports.ValidationReport{Valid: true}, nil
	}
	// Resolve (load) the workflow via the canonical identifier. A resolver rejection
	// (empty/malformed identifier, unknown workflow/pack) is a USER error and propagates
	// as the returned error (the CLI maps it to ExitUser).
	wf, err := a.resolver.Resolve(ctx, req.Identifier)
	if err != nil {
		return ports.ValidationReport{}, err
	}
	// Run the full validation pipeline on the loaded workflow (state/expression rules,
	// prompt files, plugin validators, MCP proxy) so the facade does not weaken `awf
	// validate`. ValidateOpts from the request controls the plugin-validator phase.
	// A validation failure is reported in-band (Valid=false) and the CLI maps
	// it to ExitWorkflow — distinct from the not-found USER error above.
	if a.workflowSvc != nil {
		if verr := a.workflowSvc.ValidateLoadedWorkflow(ctx, wf, req.Identifier, req.ValidateOpts); verr != nil {
			ve := ports.ValidationError{
				Code:    MapError(verr),
				Message: verr.Error(),
			}
			return ports.ValidationReport{Valid: false, Errors: []ports.ValidationError{ve}}, nil
		}
	}
	return ports.ValidationReport{Valid: true}, nil
}

// Status returns the current state of a run identified by id.
//
// Priority: live session registry (always wins) → history via GetByID.
// A history-unavailable error (nil store, missing record) degrades to ErrRunNotFound
// rather than panicking (Acceptance #35).
func (a *Adapter) Status(ctx context.Context, id string) (ports.RunStatus, error) {
	// Registry takes priority: a live session always wins over history (Acceptance #33).
	// Derive the live status from the session's non-consuming replay-buffer snapshot so
	// callers see running → completed/failed and the current step, not a bare RunID.
	if sess, ok := a.registry.Get(id); ok {
		if rs, ok := sess.(*RunSession); ok {
			return rs.StatusSnapshot(), nil
		}
		return ports.RunStatus{RunID: id}, nil
	}
	// Registry miss: the run may have completed and left the live registry. Fall back to
	// persisted history via GetByID (O(1) intent; currently an O(N) scan bounded at 10 k
	// records). A nil historySvc or a missing record degrades to ErrRunNotFound.
	if a.historySvc != nil {
		rec, err := a.historySvc.GetByID(ctx, id)
		if err == nil && rec != nil {
			return ports.RunStatus{
				RunID:       rec.ID,
				Status:      ports.RunState(rec.Status),
				StartedAt:   rec.StartedAt,
				CompletedAt: rec.CompletedAt,
			}, nil
		}
		// Any error other than ErrRecordNotFound (store failure, etc.) falls through
		// to ErrRunNotFound — history unavailability never surfaces as a hard error here.
	}
	return ports.RunStatus{}, fmt.Errorf("%w: %s", ports.ErrRunNotFound, id)
}

// History returns past run records from the history service, mapped to facade records.
func (a *Adapter) History(ctx context.Context, filter ports.HistoryFilter) ([]ports.RunRecord, error) { //nolint:gocritic // hugeParam: interface contract requires value type; pointer would break WorkflowFacade conformance
	if a.historySvc == nil {
		return nil, fmt.Errorf("listing history: history service not configured")
	}
	records, err := a.historySvc.List(ctx, &workflow.HistoryFilter{
		WorkflowName: filter.WorkflowName,
		Status:       filter.Status,
		Since:        filter.Since,
		Until:        filter.Until,
		Limit:        filter.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("listing history: %w", err)
	}
	out := make([]ports.RunRecord, len(records))
	for i, rec := range records {
		out[i] = ports.RunRecord{
			RunID:        rec.ID,
			WorkflowID:   rec.WorkflowID,
			WorkflowName: rec.WorkflowName,
			Status:       ports.RunState(rec.Status),
			StartedAt:    rec.StartedAt,
			CompletedAt:  rec.CompletedAt,
			DurationMs:   rec.DurationMs,
			ErrorMessage: rec.ErrorMessage,
		}
	}
	return out, nil
}

// GetWorkflow loads a single workflow definition by its canonical identifier
// (ports.WorkflowReader). It delegates directly to the WorkflowService without going
// through the resolver, mirroring the pre-facade HTTP read path: the identifier is the
// already-recomposed "scope/name" the interface layer passes, and a missing workflow
// surfaces as the underlying StructuredError (ErrorCodeUserInputMissingFile) so callers
// can map it to 404.
func (a *Adapter) GetWorkflow(ctx context.Context, identifier string) (*workflow.Workflow, error) {
	if a.workflowSvc == nil {
		return nil, fmt.Errorf("get workflow: workflow service not configured")
	}
	return a.workflowSvc.GetWorkflow(ctx, identifier)
}

// HistoryStats returns aggregate execution statistics for the given filter
// (ports.WorkflowReader). It mirrors History's filter mapping and returns the domain
// stats value verbatim so the HTTP response shape is unchanged.
func (a *Adapter) HistoryStats(ctx context.Context, filter ports.HistoryFilter) (*workflow.HistoryStats, error) { //nolint:gocritic // hugeParam: interface contract requires value type, consistent with History
	if a.historySvc == nil {
		return nil, fmt.Errorf("history stats: history service not configured")
	}
	return a.historySvc.GetStats(ctx, &workflow.HistoryFilter{
		WorkflowName: filter.WorkflowName,
		Status:       filter.Status,
		Since:        filter.Since,
		Until:        filter.Until,
		Limit:        filter.Limit,
	})
}

// Run resolves the canonical identifier (FR-019), creates a RunSession, subscribes to
// the Recorder exactly once (D15, SC-001), drives execution, and projects transcript
// events into the session. A resolver rejection propagates synchronously without leaking
// a session; execution success/failure is reported via the terminal event.
//
// An empty Identifier returns ErrInvalidRequest (wrapping the StructuredError) so that
// callers can use errors.Is(err, ports.ErrInvalidRequest) without importing the domain.
func (a *Adapter) Run(ctx context.Context, req ports.RunRequest) (ports.RunSession, error) {
	var wf *workflow.Workflow
	if a.resolver != nil {
		resolved, err := a.resolver.Resolve(ctx, req.Identifier)
		if err != nil {
			return nil, wrapIdentifierError(err)
		}
		wf = resolved
	}
	id := uuid.New().String()
	rec := a.recorderForRun(id)
	// A nil workflow (no resolver configured) is a no-op success — preserves the
	// historical newSession behavior used by registry/Status tests.
	sess, err := a.newSession(ctx, id, rec, func(execCtx context.Context) error {
		if wf == nil {
			return nil
		}
		_, err := a.executionSvc.RunWithWorkflowAndRunID(a.routeExec(execCtx, rec), wf, req.Inputs, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	// Persist the run inputs on the session so StatusSnapshot can expose them to
	// awf status without re-deriving them from the event stream (inputs are not
	// emitted as events). A nil map (no inputs provided) is safe — setInputs tolerates it.
	sess.setInputs(req.Inputs)
	return sess, nil
}

// ValidateDir validates all .yaml workflow files found directly in dir (non-recursive).
// It implements ports.BatchValidator. A WorkflowService is constructed ad hoc against
// dir so that the validation runs on exactly the files in that directory, independent of
// the project-level workflowSvc whose repository is scoped to the whole project tree.
//
// ValidateDir uses a dir-scoped ad-hoc WorkflowService with NO plugin validator provider
// wired; therefore opts.SkipPlugins is inherently satisfied and opts.ValidatorTimeout
// bounds nothing here. opts is accepted for BatchValidator interface symmetry and
// forward-compatibility.
//
// Returns ErrInvalidRequest when dir is empty. Returns an OS-level error when dir cannot
// be read. Per-file validation failures are captured in FileValidationResult.Errors rather
// than bubbled as the function error so that the caller can display a full per-file summary.
func (a *Adapter) ValidateDir(ctx context.Context, dir string, opts ports.ValidateOptions) ([]ports.FileValidationResult, error) {
	if dir == "" {
		return nil, fmt.Errorf("%w: dir must not be empty", ports.ErrInvalidRequest)
	}
	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", dir, err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	// Collect bare workflow names from .yaml files in the directory.
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	if len(names) == 0 {
		return []ports.FileValidationResult{}, nil
	}

	// Build a minimal WorkflowService scoped to absDir so validation runs on exactly
	// those files — we never touch the project-level workflowSvc here.
	// repository.SourceLocal is the infra-layer int constant (not ports.SourceLocal which
	// is a WorkflowSource string); SourcedPath.Source is typed as repository.Source (int).
	repo := repository.NewCompositeRepository([]repository.SourcedPath{
		{Path: absDir, Source: repository.SourceLocal},
	})
	validator := expression.NewExprValidator()
	svc := NewWorkflowService(repo, nil, nil, nil, validator)

	results := make([]ports.FileValidationResult, 0, len(names))
	for _, name := range names {
		if verr := svc.ValidateWorkflow(ctx, name); verr != nil {
			results = append(results, ports.FileValidationResult{
				Name:   name,
				Valid:  false,
				Errors: []ports.ValidationError{{Message: verr.Error(), Code: MapError(verr)}},
			})
		} else {
			results = append(results, ports.FileValidationResult{Name: name, Valid: true})
		}
	}
	return results, nil
}

// ValidatePack validates all .yaml workflow files inside the installed pack identified
// by packName. It implements ports.BatchValidator. Pack discovery uses the Adapter's
// PackDiscoverer (wired via the Resolver): workflows whose Name begins with "packName/"
// are loaded by name and validated, so the pack's canonical loader (including its repo
// path resolution) is exercised — not a raw filesystem scan.
//
// Returns ErrInvalidRequest when packName is empty. Returns an error when the pack
// produces no workflows (unknown/uninstalled pack). Per-file failures are captured in
// FileValidationResult.Errors. opts controls the plugin-validator phase (SkipPlugins,
// ValidatorTimeout) and is forwarded to ValidateLoadedWorkflow for each workflow.
func (a *Adapter) ValidatePack(ctx context.Context, packName string, opts ports.ValidateOptions) ([]ports.FileValidationResult, error) {
	if packName == "" {
		return nil, fmt.Errorf("%w: packName must not be empty", ports.ErrInvalidRequest)
	}
	if a.workflowSvc == nil {
		return nil, fmt.Errorf("ValidatePack: workflow service not configured")
	}
	if a.resolver == nil || a.resolver.packDiscoverer == nil {
		return nil, fmt.Errorf("ValidatePack: pack discoverer not configured")
	}

	// Enumerate pack workflows via the canonical discoverer so the same search-path
	// logic used by Resolve applies here too.
	allEntries, err := a.resolver.packDiscoverer.DiscoverWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering pack %q workflows: %w", packName, err)
	}

	prefix := packName + "/"
	var packEntries []workflow.WorkflowEntry
	for _, e := range allEntries {
		if strings.HasPrefix(e.Name, prefix) {
			packEntries = append(packEntries, e)
		}
	}
	if len(packEntries) == 0 {
		return nil, fmt.Errorf("workflow pack %q not found or contains no workflows", packName)
	}

	results := make([]ports.FileValidationResult, 0, len(packEntries))
	for _, entry := range packEntries {
		// bare name = everything after "packName/" (e.g. "hello" from "hello/greet")
		bareName := strings.TrimPrefix(entry.Name, prefix)
		// Load the workflow via the discoverer to get the full *workflow.Workflow for validation.
		wf, loadErr := a.resolver.packDiscoverer.LoadWorkflow(ctx, packName, bareName)
		if loadErr != nil {
			results = append(results, ports.FileValidationResult{
				Name:   bareName,
				Valid:  false,
				Errors: []ports.ValidationError{{Message: fmt.Sprintf("load: %s", loadErr.Error()), Code: MapError(loadErr)}},
			})
			continue
		}
		if verr := a.workflowSvc.ValidateLoadedWorkflow(ctx, wf, entry.Name, opts); verr != nil {
			results = append(results, ports.FileValidationResult{
				Name:   bareName,
				Valid:  false,
				Errors: []ports.ValidationError{{Message: verr.Error(), Code: MapError(verr)}},
			})
			continue
		}
		results = append(results, ports.FileValidationResult{Name: bareName, Valid: true})
	}
	return results, nil
}

// wrapIdentifierError ensures that resolver USER.FACADE.* StructuredErrors are
// also reachable via errors.Is(err, ports.ErrInvalidRequest), satisfying the
// facade contract while preserving the StructuredError for errors.As/Code access.
//
// Only USER.FACADE.IDENTIFIER_EMPTY and USER.FACADE.IDENTIFIER_MALFORMED are
// wrapped with ErrInvalidRequest; not-found codes (PACK_NOT_FOUND, WORKFLOW_NOT_FOUND)
// are returned as-is since they represent a different failure class.
func wrapIdentifierError(err error) error {
	var se *domainerrors.StructuredError
	if !errors.As(err, &se) {
		return err
	}
	switch se.Code {
	case domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
		domainerrors.ErrorCodeUserFacadeIdentifierMalformed:
		return fmt.Errorf("%w: %w", ports.ErrInvalidRequest, err)
	default:
		return err
	}
}

// recorderForRun returns a fresh per-run recorder when a factory is configured (HTTP/TUI),
// so concurrent runs never share a recorder; otherwise the Adapter's shared recorder
// (CLI/ACP). A factory error degrades safely to the shared recorder — execution still runs,
// only live events are lost.
func (a *Adapter) recorderForRun(runID string) ports.Recorder {
	if f := a.getRunRecorderFactory(); f != nil {
		if r, err := f(runID); err == nil && r != nil {
			return r
		}
	}
	return a.recorder
}

// routeExec routes the ExecutionService's transcript emission to rec when rec is a per-run
// recorder, so its events reach this session's subscription; for the shared recorder the
// context is unchanged (ExecutionService already emits to it via its own wiring).
func (a *Adapter) routeExec(ctx context.Context, rec ports.Recorder) context.Context {
	if rec != a.recorder {
		return withRecorder(ctx, rec)
	}
	return ctx
}

// Resume re-drives a previously persisted, non-completed run identified by req.RunID
// (FR: US-resume). It loads the saved ExecutionContext, resumes from req.FromStep (or
// "current" when empty), applies req.InputOverrides when provided, and streams transcript
// events through the same RunSession machinery as Run. The resumed session is registered
// under the original runID so Status/SSE lookups resolve it.
//
// An empty req.RunID returns ErrInvalidRequest so that callers can use
// errors.Is(err, ports.ErrInvalidRequest) without importing the domain.
func (a *Adapter) Resume(ctx context.Context, req ports.ResumeRequest) (ports.RunSession, error) {
	if req.RunID == "" {
		err := domainerrors.NewUserError(
			domainerrors.ErrorCodeUserFacadeIdentifierEmpty,
			"resume: run ID is empty",
			nil,
			nil,
		)
		return nil, fmt.Errorf("%w: %w", ports.ErrInvalidRequest, err)
	}
	effectiveFromStep := req.FromStep
	if effectiveFromStep == "" {
		effectiveFromStep = "current"
	}
	rec := a.recorderForRun(req.RunID)
	return a.newSession(ctx, req.RunID, rec, func(execCtx context.Context) error {
		if a.executionSvc == nil {
			return fmt.Errorf("resume: execution service not configured")
		}
		_, err := a.executionSvc.Resume(a.routeExec(execCtx, rec), req.RunID, req.InputOverrides, effectiveFromStep)
		return err
	})
}

// RunStep executes a single workflow step in isolation by delegating to the underlying
// ExecutionService.ExecuteSingleStep. It maps the application-layer SingleStepResult to
// the facade ports.StepResult so callers never import the application package directly.
// Returns an error when the execution service is not configured (nil) or when
// ExecuteSingleStep itself reports a hard error (workflow/step not found, etc.).
func (a *Adapter) RunStep(ctx context.Context, req ports.RunStepRequest) (ports.StepResult, error) {
	if a.executionSvc == nil {
		return ports.StepResult{}, fmt.Errorf("RunStep: execution service not configured")
	}
	res, err := a.executionSvc.ExecuteSingleStep(ctx, req.Identifier, req.StepName, req.Inputs, req.Mocks)
	if err != nil {
		return ports.StepResult{}, err
	}
	return ports.StepResult{
		StepName:    res.StepName,
		Output:      res.Output,
		Stderr:      res.Stderr,
		ExitCode:    res.ExitCode,
		Status:      ports.RunState(res.Status),
		Error:       res.Error,
		StartedAt:   res.StartedAt,
		CompletedAt: res.CompletedAt,
	}, nil
}

// newSession allocates a RunSession, registers it, wires the sole Recorder subscription
// (D15, SC-001), and spawns a goroutine that runs execution and projects transcript events.
// rec is the recorder this session subscribes to: the Adapter's shared recorder, or a
// per-run recorder produced by recorderForRun. A per-run recorder (rec != a.recorder) is
// owned by the session and closed when it ends, so its transcript file is flushed/released.
func (a *Adapter) newSession(ctx context.Context, id string, rec ports.Recorder, exec func(context.Context) error) (*RunSession, error) {
	session := newRunSession(ctx, id, 0)

	if err := a.registry.Add(session); err != nil {
		return nil, err
	}

	// Do NOT remove the session from the registry on close. Completed sessions must
	// remain reachable via registry.Get so that:
	//   - GET /executions/{id} can report the terminal status from StatusSnapshot()
	//     after the execution goroutine has finished and closed the session.
	//   - SSE /executions/{id}/events can replay the buffered terminal event on a
	//     late reconnect.
	// Sessions closed by explicit cancel (DELETE /executions/{id} → Bridge.Cancel →
	// session.Close()) also stay in the registry so a follow-up GET still sees
	// "cancelled" rather than "not found".
	// Registry entries are bounded by the number of runs since server start; they
	// are small in-memory objects and are reclaimed on server restart.

	// SC-001: exactly one Subscribe call per RunSession — only this method calls Subscribe.
	// A nil recorder is tolerated (ACP routes live events through an EventPublisher, not the
	// facade recorder): no subscription is taken and only the terminal event reaches the
	// session. A nil channel blocks forever in the select, so the run still completes.
	var sub <-chan transcript.ExchangeEvent
	cancelSub := func() {}
	if rec != nil {
		sub, cancelSub = rec.Subscribe()
	}
	ownRec := rec != nil && rec != a.recorder // per-run recorder: this session owns its lifecycle

	go func() {
		// drainDone synchronizes the background drainTranscript goroutine (if spawned).
		// We Wait() before the deferred cleanups fire so that:
		//   - drainTranscript has finished appending to session.events before session.Close()
		//     seals the channel, preventing a send-on-closed race.
		//   - cancelSub() is called only after drainTranscript exits, so the range in
		//     drainTranscript sees the channel close naturally (not by us pulling the sub out).
		var drainDone sync.WaitGroup

		// On exit (LIFO run order): cancelSub() runs first to close the recorder subscription so
		// the range in drainTranscript terminates; drainDone.Wait() then blocks until that
		// goroutine has finished appending its final events (the session is still open, so the
		// appends land safely); session.Close() then seals the events channel; finally a per-run
		// recorder is closed to flush its transcript file. All are idempotent.
		//
		// ORDERING IS LOAD-BEARING: cancelSub MUST run before drainDone.Wait(). drainTranscript
		// ranges over sub and exits only when sub is closed, and nothing else closes it — waiting
		// before cancelling would deadlock the goroutine against its own cleanup (the events
		// channel would never close, and a downstream Drain() would block forever).
		if ownRec {
			defer func() { _ = rec.Close() }() //nolint:errcheck // Close is idempotent; flush best-effort
		}
		defer session.Close() //nolint:errcheck // Close always returns nil
		defer drainDone.Wait()
		defer cancelSub()

		// When an input reader edge is wired (CLI/HTTP/TUI), route conversation user
		// input through this session: bind a session-bound reader so interactive turns
		// park via EventInputRequired and resume via RunSession.Respond, instead of a
		// process-global stdin read that EOFs prematurely under the facade. ACP leaves
		// this edge nil — it answers input requests through its own facadeInputBridge.
		execCtx := ctx
		if a.getUserInputReader() != nil {
			execCtx = withUserInputReader(ctx, newSessionInputReader(session))
		}

		// Execution is started here (not before the goroutine) so the recorder's already-
		// buffered transcript events keep select priority before execDone becomes ready.
		execDone := a.startExecution(execCtx, exec)

		for {
			select {
			case ev, ok := <-sub:
				if !ok {
					// Recorder subscription ended (e.g. a NopRecorder returns an already-
					// closed channel) — stop selecting on it but keep waiting for execution
					// to finish so the terminal event is still emitted. A nil channel blocks
					// forever, so this case never fires again.
					sub = nil
					continue
				}
				if fEv, err := ProjectEvent(ev); err == nil {
					session.appendEvent(fEv)
				}

			case execErr := <-execDone:
				// Execution finished: project any already-buffered transcript events
				// before the terminal event so ordering is deterministic, then drain
				// remaining late events in a bounded background goroutine, waiting for
				// it to finish before the deferred cleanups run.
				a.drainBuffered(session, sub)
				a.emitTerminalEvent(session, execErr)
				// Only keep draining if the subscription is still live; a nil sub (closed
				// recorder) would make drainTranscript block forever and leak.
				if sub != nil {
					drainDone.Add(1)
					go func() {
						defer drainDone.Done()
						a.drainTranscript(session, sub)
					}()
				}
				return

			case <-session.ctx.Done():
				// Session closed manually before execution finished — exit and clean up.
				return
			}
		}
	}()

	return session, nil
}

// drainBuffered projects every transcript event currently buffered on sub (non-blocking)
// into the session. Used before emitting the terminal event so buffered events keep their
// position ahead of EventWorkflowCompleted/Failed.
func (a *Adapter) drainBuffered(session *RunSession, sub <-chan transcript.ExchangeEvent) {
	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				return
			}
			if fEv, err := ProjectEvent(ev); err == nil {
				session.appendEvent(fEv)
			}
		default:
			return
		}
	}
}

// startExecution drives the supplied execution closure (Run's RunWithWorkflowAndRunID or
// Resume's ExecutionService.Resume) in a goroutine and reports completion on the returned
// channel. A panic from the execution service (e.g. unconfigured dependencies) is captured
// as an error so the session still receives a terminal event rather than crashing.
func (a *Adapter) startExecution(ctx context.Context, exec func(context.Context) error) <-chan error {
	execDone := make(chan error, 1)
	go func() {
		var execErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					execErr = fmt.Errorf("execution panic: %v", r)
				}
			}()
			execErr = exec(ctx)
		}()
		execDone <- execErr
	}()
	return execDone
}

// drainTranscript projects any remaining transcript events into the session after the
// terminal event has been emitted, until the recorder subscription is cancelled.
// It must finish before session.Close() is called; the caller uses a sync.WaitGroup
// to enforce this ordering (see newSession's defer drainDone.Wait()).
func (a *Adapter) drainTranscript(session *RunSession, sub <-chan transcript.ExchangeEvent) {
	for ev := range sub {
		if fEv, err := ProjectEvent(ev); err == nil {
			session.appendEvent(fEv)
		}
	}
}

// emitTerminalEvent appends the single terminal event for the run: EventWorkflowCompleted
// on success, or EventWorkflowFailed (with an *ports.EnrichedTerminal payload) on failure
// (Criteria #6/#7). All consumers (run_facade_projector, acp_session_service,
// tui/tab_monitoring) type-assert ev.Payload.(*ports.EnrichedTerminal), so the payload must
// be that concrete type — never a bare ErrorCode string.
//
// EventWorkflowCompleted emits an empty *ports.EnrichedTerminal (not nil) for type-consistency
// with EventWorkflowFailed: consumers that switch on Kind can safely assert Payload as
// *ports.EnrichedTerminal in both branches without a nil guard.
func (a *Adapter) emitTerminalEvent(session *RunSession, execErr error) {
	kind := ports.EventWorkflowCompleted
	payload := &ports.EnrichedTerminal{} // non-nil empty value for type-consistency
	if execErr != nil {
		kind = ports.EventWorkflowFailed
		session.setErr(execErr)
		payload = &ports.EnrichedTerminal{Error: execErr.Error()}
	}
	session.appendEvent(ports.Event{
		Kind:      kind,
		RunID:     session.id,
		Timestamp: time.Now(),
		Payload:   payload,
	})
}
