package ports

import (
	"context"
	"errors"

	"github.com/awf-project/cli/internal/domain/workflow"
)

// Sentinel errors for the facade port layer.
//
// These sentinels are the canonical "what went wrong" signals callers use with
// errors.Is to branch on known failure modes without importing application or
// infrastructure packages. Their relationship to domain/errors.ErrorCode is as
// follows:
//
//   - ErrInvalidRequest   → ErrorCodeUserInputValidationFailed (empty/malformed identifier)
//   - ErrSessionClosed    → ErrorCodeExecutionCommandTimeout / general lifecycle error
//   - ErrDuplicateResponse → no direct ErrorCode; indicates protocol misuse
//   - ErrSessionExists    → ErrorCodeExecutionCommandTimeout / lifecycle error
//   - ErrRunNotFound      → ErrorCodeUserInputMissingFile (run ID not in registry/history)
//
// The application adapter (application.Adapter) is responsible for wrapping
// domain/errors.StructuredError values with these sentinels using fmt.Errorf("%w",
// sentinelErr) so that callers in the interface layers can rely on errors.Is without
// depending on the application or domain/errors packages directly. The domain port
// layer declares only the sentinel values; the wrapping convention belongs to the
// adapter implementation.
var (
	ErrInvalidRequest    = errors.New("invalid request")
	ErrSessionClosed     = errors.New("session closed")
	ErrDuplicateResponse = errors.New("duplicate response")
	ErrSessionExists     = errors.New("session already exists")
	ErrRunNotFound       = errors.New("run not found")
)

// ResumeRequest is the input for resuming a previously paused or interrupted workflow run.
// It mirrors RunRequest so callers that construct both use a consistent pattern.
// Zero-value ResumeRequest (RunID == "") produces ErrInvalidRequest.
type ResumeRequest struct {
	// RunID is the identifier of the run to resume (required).
	RunID string
	// InputOverrides replaces specific workflow input values for the resumed execution.
	// A nil map means "no overrides; use the values stored with the original run".
	InputOverrides map[string]any
	// FromStep names the step at which execution should restart.
	// An empty string means "restart from the current/last persisted step".
	FromStep string
}

// WorkflowFacade is the primary port for workflow orchestration.
// Driving port — called by interface layer (CLI, API, MQ).
// RunStep is intentionally absent: single-step isolation is a CLI-only concern
// (awf run --step) and is exposed through the focused SingleStepRunner port so
// that every other implementer of WorkflowFacade is not burdened with it (M1).
type WorkflowFacade interface {
	List(ctx context.Context) ([]WorkflowSummary, error)
	Validate(ctx context.Context, req RunRequest) (ValidationReport, error)
	Status(ctx context.Context, runID string) (RunStatus, error)
	History(ctx context.Context, filter HistoryFilter) ([]RunRecord, error)
	Run(ctx context.Context, req RunRequest) (RunSession, error)
	// Resume re-drives a previously persisted, non-completed run.
	// Returns ErrRunNotFound when runID is unknown to both the live session registry
	// and the history store. Returns ErrInvalidRequest when req.RunID is empty.
	Resume(ctx context.Context, req ResumeRequest) (RunSession, error)
}

// SingleStepRunner is a focused port for isolated single-step execution.
// It is implemented by application.Adapter and consumed only by the CLI
// `awf run --step` path (runSingleStep). Keeping it separate from WorkflowFacade
// ensures no other interface (API, TUI, ACP, test fakes) is required to implement
// this CLI-only operation (M1).
type SingleStepRunner interface {
	RunStep(ctx context.Context, req RunStepRequest) (StepResult, error)
}

// WorkflowReader is a focused read-only port for workflow-definition introspection
// (loading a single workflow) and history statistics. It is implemented by
// application.Adapter and consumed by the HTTP API for the two read endpoints whose
// payloads have no lightweight DTO equivalent on WorkflowFacade:
// GET /api/workflows/{scope}/{name} and GET /api/history/stats.
//
// It is intentionally kept separate from WorkflowFacade — mirroring the SingleStepRunner
// segregation (M1) — so that the many WorkflowFacade implementers and test fakes that only
// drive execution/listing are not forced to implement definition-introspection methods they
// never use. GetWorkflow returns the domain aggregate (*workflow.Workflow) and HistoryStats
// returns *workflow.HistoryStats directly: these are domain types (no infrastructure leak),
// and returning them verbatim keeps the HTTP response shape byte-identical to the pre-facade
// path, so routing these endpoints through the facade introduces no JSON regression.
type WorkflowReader interface {
	GetWorkflow(ctx context.Context, identifier string) (*workflow.Workflow, error)
	HistoryStats(ctx context.Context, filter HistoryFilter) (*workflow.HistoryStats, error)
}

// RunSession represents an active workflow execution.
// Close is idempotent: multiple calls return the same nil/error without panicking.
type RunSession interface {
	ID() string
	Events() <-chan Event
	Respond(InputResponse) error
	Err() error
	Close() error
}

// FileValidationResult carries the outcome of validating a single workflow file.
// It is the element type of the slice returned by BatchValidator methods so that
// callers can display per-file valid/invalid status without re-parsing results.
type FileValidationResult struct {
	// Name is the bare workflow name (filename without the .yaml extension) as it
	// would appear in `awf validate <name>` or `awf run <name>`.
	Name string
	// Valid is true when the workflow passed all validation checks.
	Valid bool
	// Errors contains structured validation failures; empty when Valid is true.
	Errors []ValidationError
}

// BatchValidator is a focused port for directory- and pack-scoped validation.
// It is implemented by application.Adapter and consumed by the CLI `awf validate --dir`
// and `awf validate --pack` paths (runValidateDir, runValidatePack).
//
// Rationale for keeping this port separate from WorkflowFacade (mirrors the
// SingleStepRunner / WorkflowReader segregation, M1):
//   - WorkflowFacade.Validate validates a SINGLE named workflow through the canonical
//     resolver; it is consumed by every interface layer (CLI, API, ACP).
//   - BatchValidator operates on a raw filesystem directory or an installed pack name,
//     bypassing the resolver — it is a CLI-only convenience for mass validation.
//     Burdening every WorkflowFacade implementer (test fakes, ACP session facades,
//     HTTP API handler fakes) with two extra methods they never invoke would violate ISP.
//
// Both methods accept ValidateOptions so callers can forward --skip-plugins and
// --validator-timeout across all validation paths uniformly.
//
// Both methods return one FileValidationResult per discovered .yaml file. A method-level
// error (e.g. unreadable directory, unknown pack) is returned as the function error; per-file
// validation failures are captured inside FileValidationResult.Valid/Errors instead so that
// the caller can display a full summary even when some files fail.
type BatchValidator interface {
	// ValidateDir validates all .yaml workflow files found directly in dir (non-recursive).
	// Returns ErrInvalidRequest when dir is empty. Returns an OS error when dir cannot be read.
	ValidateDir(ctx context.Context, dir string, opts ValidateOptions) ([]FileValidationResult, error)
	// ValidatePack validates all .yaml workflow files inside the "workflows/" sub-directory
	// of the installed pack identified by packName. Returns ErrInvalidRequest when packName
	// is empty. Returns an error when the pack cannot be located.
	ValidatePack(ctx context.Context, packName string, opts ValidateOptions) ([]FileValidationResult, error)
}
