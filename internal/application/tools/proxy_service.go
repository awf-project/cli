package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
)

// ProviderFactory creates tool providers from a proxy config.
// Injected so T013 can supply the real factory without modifying ProxyService.
type ProviderFactory func(cfg ProxyConfig) ([]ports.ToolProvider, error)

// noopCleanup is a shared no-op cleanup returned when the proxy is not started
// or when registration fails before a cleanup is established.
func noopCleanup() error { return nil }

// ProxyService orchestrates the MCP tool proxy lifecycle for a workflow step.
type ProxyService struct {
	cliExec         ports.CLIExecutor
	tracer          ports.Tracer
	logger          ports.Logger
	providerFactory ProviderFactory
}

// NewProxyService creates a configured ProxyService.
func NewProxyService(cliExec ports.CLIExecutor, tracer ports.Tracer, logger ports.Logger, providerFactory ProviderFactory) *ProxyService {
	return &ProxyService{
		cliExec:         cliExec,
		tracer:          tracer,
		logger:          logger,
		providerFactory: providerFactory,
	}
}

// proxyConfigJSON is the on-disk format for the tmp MCP proxy config file.
// Enable is intentionally omitted: the file is only written when Enable=true,
// so mcp-serve never needs to re-check the flag.
type proxyConfigJSON struct {
	InterceptBuiltins bool             `json:"intercept_builtins"`
	PluginTools       []PluginToolSpec `json:"plugin_tools"`
}

// StartForStdio writes a tmp MCP config and spawns `awf mcp-serve --config=<path>`.
// Returns ("", noopCleanup, nil) when cfg.Enable is false or no tools are configured.
// cleanup is idempotent: second call returns nil.
func (s *ProxyService) StartForStdio(ctx context.Context, cfg ProxyConfig) (mcpConfigPath string, cleanup func() error, err error) {
	if !cfg.Enable || (!cfg.InterceptBuiltins && len(cfg.PluginTools) == 0) {
		return "", noopCleanup, nil
	}

	// Stdio mode does not consume in-process providers: the spawned `awf mcp-serve`
	// subprocess builds its own providers from the on-disk config. The previous
	// `providerFactory(cfg)` call here was a defensive pre-validation that allocated
	// PluginToolAdapter instances only to discard them — a future Adapter that opens
	// connections in its constructor would silently leak. Domain-level workflow
	// validation already catches malformed plugin specs before this point.
	tmp, err := os.CreateTemp("", "awf-mcp-proxy-*.json")
	if err != nil {
		return "", noopCleanup, fmt.Errorf("failed to create proxy config: %w", err)
	}
	tmpPath := tmp.Name()

	data, err := json.Marshal(proxyConfigJSON{
		InterceptBuiltins: cfg.InterceptBuiltins,
		PluginTools:       cfg.PluginTools,
	})
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", noopCleanup, fmt.Errorf("failed to marshal proxy config: %w", err)
	}

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", noopCleanup, fmt.Errorf("failed to write proxy config: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", noopCleanup, fmt.Errorf("failed to close proxy config: %w", err)
	}

	awfBin, err := os.Executable()
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", noopCleanup, fmt.Errorf("failed to resolve awf binary: %w", err)
	}

	proc, err := s.cliExec.Start(ctx, awfBin, "mcp-serve", "--config="+tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", noopCleanup, fmt.Errorf("failed to spawn awf mcp-serve: %w", err)
	}

	var once sync.Once
	cleanupFn := func() error {
		var retErr error
		once.Do(func() {
			defer func() { _ = os.Remove(tmpPath) }()

			_ = proc.Signal(os.Interrupt) //nolint:errcheck // best-effort; SIGKILL fallback handles failure
			select {
			case <-proc.Done():
			case <-time.After(5 * time.Second):
				_ = proc.Signal(syscall.SIGKILL) //nolint:errcheck // last-resort kill; error not actionable
				<-proc.Done()
			}
			retErr = proc.Wait()
		})
		return retErr
	}

	return tmpPath, cleanupFn, nil
}

// StartForHTTP builds an in-process router for OpenAI Compatible transport.
// Returns (nil, noopCleanup, nil) when cfg.Enable is false or no tools are configured.
func (s *ProxyService) StartForHTTP(ctx context.Context, cfg ProxyConfig) (router *Router, cleanup func() error, err error) {
	if !cfg.Enable || (!cfg.InterceptBuiltins && len(cfg.PluginTools) == 0) {
		return nil, noopCleanup, nil
	}

	providers, err := s.providerFactory(cfg)
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("proxy provider factory: %w", err)
	}

	r := NewRouter(s.tracer, s.logger)
	registered := false
	defer func() {
		// If registration did not complete successfully, close any partially-registered
		// providers to avoid resource leaks from providers that open connections on Register.
		// context.Background() is used here because the caller's ctx may already be cancelled
		// when this deferred cleanup runs (e.g. on error return), matching the pattern used
		// in base_cli_provider.go for geminiMCPInjector cleanup.
		if !registered {
			_ = r.Close(context.Background()) //nolint:errcheck // best-effort cleanup on partial registration
		}
	}()

	for _, p := range providers {
		if regErr := r.Register(ctx, p); regErr != nil {
			return nil, noopCleanup, fmt.Errorf("router registration: %w", regErr)
		}
	}
	registered = true

	// context.Background() is used in the cleanup closure so it succeeds even when
	// the caller's ctx is already cancelled at teardown time.
	return r, func() error { return r.Close(context.Background()) }, nil
}
