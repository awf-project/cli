package application

import (
	"context"
	"fmt"
	"io"
	"maps"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/agents"
	infraexpression "github.com/awf-project/cli/internal/infrastructure/expression"
	"github.com/awf-project/cli/internal/infrastructure/github"
	infrahttp "github.com/awf-project/cli/internal/infrastructure/http"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/awf-project/cli/internal/infrastructure/repository"
	"github.com/awf-project/cli/internal/infrastructure/xdg"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/awf-project/cli/pkg/interpolation"
)

// compositeOperationProvider delegates to multiple OperationProvider implementations.
// It mirrors pluginmgr.CompositeOperationProvider but lives here to avoid the import
// cycle caused by pluginmgr/system.go importing the application package.
type compositeOperationProvider struct {
	providers []ports.OperationProvider
}

func (c *compositeOperationProvider) GetOperation(name string) (*pluginmodel.OperationSchema, bool) {
	for _, p := range c.providers {
		if p == nil {
			continue
		}
		if op, found := p.GetOperation(name); found {
			return op, true
		}
	}
	return nil, false
}

func (c *compositeOperationProvider) ListOperations() []*pluginmodel.OperationSchema {
	var result []*pluginmodel.OperationSchema
	for _, p := range c.providers {
		if p == nil {
			continue
		}
		result = append(result, p.ListOperations()...)
	}
	return result
}

func (c *compositeOperationProvider) Execute(ctx context.Context, name string, inputs map[string]any) (*pluginmodel.OperationResult, error) {
	for _, p := range c.providers {
		if p == nil {
			continue
		}
		if _, found := p.GetOperation(name); found {
			return p.Execute(ctx, name, inputs)
		}
	}
	return nil, fmt.Errorf("operation not found: %s", name)
}

// PluginStateChecker abstracts plugin enable/disable state lookup.
type PluginStateChecker interface {
	IsPluginEnabled(name string) bool
}

// PluginProviders groups optional plugin-provided capabilities.
type PluginProviders struct {
	Operations ports.OperationProvider
	Validators ports.WorkflowValidatorProvider
	StepTypes  ports.StepTypeProvider
}

// NotifyConfig provides notification backend configuration.
type NotifyConfig struct {
	DefaultBackend string
}

// OutputWriterPair holds stdout/stderr writers for streaming mode.
type OutputWriterPair struct {
	Stdout io.Writer
	Stderr io.Writer
}

// SetupOption configures an ExecutionSetup.
type SetupOption func(*setupConfig)

type setupConfig struct {
	notifyConfig    NotifyConfig
	pluginChecker   PluginStateChecker
	pluginProviders PluginProviders
	tracer          ports.Tracer
	auditWriter     ports.AuditTrailWriter
	packName        string
	packResolver    PackWorkflowLoader
	outputWriters   *OutputWriterPair
	userInputReader ports.UserInputReader
	historyStore    ports.HistoryStore
	templatePaths   []string
	pluginService   *PluginService
}

// WithNotifyConfig configures notification backend defaults.
func WithNotifyConfig(cfg NotifyConfig) SetupOption {
	return func(c *setupConfig) { c.notifyConfig = cfg }
}

// WithPluginState configures a checker that gates individual built-in providers.
func WithPluginState(checker PluginStateChecker) SetupOption {
	return func(c *setupConfig) { c.pluginChecker = checker }
}

// WithPluginProviders injects plugin-provided operation, validator, and step-type extensions.
func WithPluginProviders(p PluginProviders) SetupOption {
	return func(c *setupConfig) { c.pluginProviders = p }
}

// WithTracer configures a distributed tracing implementation.
func WithTracer(t ports.Tracer) SetupOption {
	return func(c *setupConfig) { c.tracer = t }
}

// WithAuditWriter configures an audit trail writer for execution events.
func WithAuditWriter(w ports.AuditTrailWriter) SetupOption {
	return func(c *setupConfig) { c.auditWriter = w }
}

// WithPackContext sets the pack name and its associated workflow loader.
// When packName is non-empty, pack-scoped XDG paths are used instead of the global paths.
func WithPackContext(name string, resolver PackWorkflowLoader) SetupOption {
	return func(c *setupConfig) {
		c.packName = name
		c.packResolver = resolver
	}
}

// WithOutputWriters configures streaming output writers for step execution.
func WithOutputWriters(stdout, stderr io.Writer) SetupOption {
	return func(c *setupConfig) {
		c.outputWriters = &OutputWriterPair{Stdout: stdout, Stderr: stderr}
	}
}

// WithUserInputReader configures the source for interactive user input in conversations.
func WithUserInputReader(r ports.UserInputReader) SetupOption {
	return func(c *setupConfig) { c.userInputReader = r }
}

// WithHistoryStore enables execution history recording.
// If the store also implements io.Closer, it will be closed by SetupResult.Cleanup.
func WithHistoryStore(s ports.HistoryStore) SetupOption {
	return func(c *setupConfig) { c.historyStore = s }
}

// WithTemplatePaths registers additional YAML template search paths.
func WithTemplatePaths(paths []string) SetupOption {
	return func(c *setupConfig) { c.templatePaths = paths }
}

// WithPluginService injects the plugin lifecycle manager into the execution service.
func WithPluginService(svc *PluginService) SetupOption {
	return func(c *setupConfig) { c.pluginService = svc }
}

// ExecutionSetup centralizes ExecutionService wiring.
// It is the single authoritative place where all Set*() calls on ExecutionService
// are performed, so both CLI runWorkflow and TUI buildBridge share an identical
// service configuration.
//
// Architecture note: this type lives in the application layer but imports concrete
// infrastructure packages. This is an accepted pragmatic trade-off: pushing all
// provider construction into callers would replicate the buildProviders logic
// across every entry point, defeating the purpose of this centralized builder.
type ExecutionSetup struct {
	repo          ports.WorkflowRepository
	stateStore    ports.StateStore
	shellExecutor ports.CommandExecutor
	logger        ports.Logger
	opts          []SetupOption
}

// NewExecutionSetup creates an ExecutionSetup with the required core dependencies.
// Optional behavior is configured through SetupOption functional options.
func NewExecutionSetup(
	repo ports.WorkflowRepository,
	stateStore ports.StateStore,
	shellExecutor ports.CommandExecutor,
	logger ports.Logger,
	opts ...SetupOption,
) *ExecutionSetup {
	return &ExecutionSetup{
		repo:          repo,
		stateStore:    stateStore,
		shellExecutor: shellExecutor,
		logger:        logger,
		opts:          opts,
	}
}

// SetupResult is returned by Build and bundles the fully wired services together
// with a Cleanup function that releases any resources acquired during construction.
type SetupResult struct {
	ExecService *ExecutionService
	WorkflowSvc *WorkflowService
	HistorySvc  *HistoryService
	// Cleanup releases resources allocated during Build (e.g. closes HistoryStore).
	// It is safe to call multiple times.
	Cleanup func()
}

// Build constructs and wires all services according to the configured options.
// The returned SetupResult.Cleanup must be deferred by the caller.
func (s *ExecutionSetup) Build(_ context.Context) (*SetupResult, error) {
	cfg := &setupConfig{}
	for _, opt := range s.opts {
		opt(cfg)
	}

	var cleanups []func()

	exprValidator := infraexpression.NewExprValidator()
	wfSvc := NewWorkflowService(s.repo, s.stateStore, s.shellExecutor, s.logger, exprValidator)

	var historySvc *HistoryService
	if cfg.historyStore != nil {
		historySvc = NewHistoryService(cfg.historyStore, s.logger)
		if closer, ok := cfg.historyStore.(io.Closer); ok {
			cleanups = append(cleanups, func() { _ = closer.Close() })
		}
	}

	resolver := interpolation.NewTemplateResolver()
	parallelExec := NewParallelExecutor(s.logger)
	exprEvaluator := infraexpression.NewExprEvaluator()
	execSvc := NewExecutionServiceWithEvaluator(
		wfSvc, s.shellExecutor, parallelExec, s.stateStore, s.logger, resolver, historySvc, exprEvaluator,
	)

	// Inject XDG paths: use pack-scoped paths when a pack is active.
	if cfg.packName != "" {
		execSvc.SetAWFPaths(xdg.PackAWFPaths(cfg.packName))
	} else {
		execSvc.SetAWFPaths(xdg.AWFPaths())
	}

	// Wire agent registry and conversation manager when at least one agent is available.
	agentRegistry := agents.NewAgentRegistry()
	if err := agentRegistry.RegisterDefaults(); err == nil {
		execSvc.SetAgentRegistry(agentRegistry)
		convMgr := NewConversationManager(s.logger, resolver, agentRegistry)
		if cfg.userInputReader != nil {
			convMgr.SetUserInputReader(cfg.userInputReader)
		}
		execSvc.SetConversationManager(convMgr)
	}

	if cfg.packResolver != nil {
		execSvc.SetPackWorkflowLoader(cfg.packResolver)
	}

	if cfg.auditWriter != nil {
		execSvc.SetAuditTrailWriter(cfg.auditWriter)
	}

	if cfg.tracer != nil {
		execSvc.SetTracer(cfg.tracer)
	}

	compositeProvider := s.buildProviders(cfg)
	execSvc.SetOperationProvider(compositeProvider)

	if cfg.pluginProviders.Validators != nil {
		wfSvc.SetValidatorProvider(cfg.pluginProviders.Validators)
	}
	if cfg.pluginProviders.StepTypes != nil {
		execSvc.SetStepTypeProvider(cfg.pluginProviders.StepTypes)
	}

	if len(cfg.templatePaths) > 0 {
		templateRepo := repository.NewYAMLTemplateRepository(cfg.templatePaths)
		templateSvc := NewTemplateService(templateRepo, s.logger)
		execSvc.SetTemplateService(templateSvc)
	}

	if cfg.outputWriters != nil {
		execSvc.SetOutputWriters(cfg.outputWriters.Stdout, cfg.outputWriters.Stderr)
	}

	if cfg.pluginService != nil {
		execSvc.SetPluginService(cfg.pluginService)
	}

	// Build cleanup function that runs registered closers in LIFO order.
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	return &SetupResult{
		ExecService: execSvc,
		WorkflowSvc: wfSvc,
		HistorySvc:  historySvc,
		Cleanup:     cleanup,
	}, nil
}

// buildProviders assembles the composite operation provider from built-in and plugin-supplied
// providers, honoring the PluginStateChecker gate for each built-in.
//
// Note: pluginmgr.CompositeOperationProvider cannot be used here because pluginmgr/system.go
// imports the application package, creating an import cycle. The local compositeOperationProvider
// type mirrors its behavior.
func (s *ExecutionSetup) buildProviders(cfg *setupConfig) ports.OperationProvider {
	isEnabled := func(name string) bool {
		if cfg.pluginChecker == nil {
			return true
		}
		return cfg.pluginChecker.IsPluginEnabled(name)
	}

	var providers []ports.OperationProvider

	if isEnabled("github") {
		githubClient := github.NewClient(s.logger)
		providers = append(providers, github.NewGitHubOperationProvider(githubClient, s.logger))
	}

	if isEnabled("notify") {
		notifyProvider := notify.NewNotifyOperationProvider(s.logger)
		desktopBackend := notify.NewDesktopBackend()
		_ = notifyProvider.RegisterBackend("desktop", desktopBackend) //nolint:errcheck // registration of built-in backends cannot fail
		webhookBackend := notify.NewWebhookBackend()
		_ = notifyProvider.RegisterBackend("webhook", webhookBackend) //nolint:errcheck // registration of built-in backends cannot fail
		if cfg.notifyConfig.DefaultBackend != "" {
			notifyProvider.SetDefaultBackend(cfg.notifyConfig.DefaultBackend)
		}
		providers = append(providers, notifyProvider)
	}

	if isEnabled("http") {
		httpClient := httpx.NewClient()
		providers = append(providers, infrahttp.NewHTTPOperationProvider(httpClient, s.logger))
	}

	if cfg.pluginProviders.Operations != nil {
		providers = append(providers, cfg.pluginProviders.Operations)
	}

	return &compositeOperationProvider{providers: providers}
}

// MergeInputs returns configInputs merged with cliInputs. CLI wins on conflict.
// Neither input map is mutated.
func MergeInputs(configInputs, cliInputs map[string]any) map[string]any {
	result := make(map[string]any)
	maps.Copy(result, configInputs)
	maps.Copy(result, cliInputs)
	return result
}
