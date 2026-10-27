package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/testutil/mocks"
	"github.com/awf-project/cli/pkg/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type preflightProvider interface {
	Preflight(context.Context, map[string]any) error
}

type preflightExecutor struct {
	*mocks.MockCLIExecutor
	stdout   []byte
	stderr   []byte
	err      error
	afterRun func()
}

func newPreflightExecutor(stdout, stderr string, err error) *preflightExecutor {
	return &preflightExecutor{
		MockCLIExecutor: mocks.NewMockCLIExecutor(),
		stdout:          []byte(stdout),
		stderr:          []byte(stderr),
		err:             err,
	}
}

func (e *preflightExecutor) Run(ctx context.Context, name string, stdoutW, stderrW io.Writer, args ...string) ([]byte, []byte, error) {
	_, _, _ = e.MockCLIExecutor.Run(ctx, name, stdoutW, stderrW, args...)
	if e.afterRun != nil {
		e.afterRun()
	}
	return e.stdout, e.stderr, e.err
}

var _ ports.CLIExecutor = (*preflightExecutor)(nil)

type preflightHTTPDoer struct {
	calls atomic.Int32
}

type preflightHTTPDoerFunc func(*http.Request) (*http.Response, error)

func (f preflightHTTPDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

// preflightLogCapture records every log message and field supplied during a
// preflight. Tests use it to make the redaction boundary observable.
type preflightLogCapture struct {
	entries []string
}

func (c *preflightLogCapture) Debug(msg string, fields ...any) { c.record(msg, fields...) }
func (c *preflightLogCapture) Info(msg string, fields ...any)  { c.record(msg, fields...) }
func (c *preflightLogCapture) Warn(msg string, fields ...any)  { c.record(msg, fields...) }
func (c *preflightLogCapture) Error(msg string, fields ...any) { c.record(msg, fields...) }
func (c *preflightLogCapture) WithContext(_ map[string]any) ports.Logger {
	return c
}

func (c *preflightLogCapture) record(msg string, fields ...any) {
	c.entries = append(c.entries, msg+fmt.Sprint(fields...))
}
func (c *preflightLogCapture) String() string { return strings.Join(c.entries, "\n") }

func assertPreflightConfidential(t *testing.T, log *preflightLogCapture, err error, canaries ...string) {
	t.Helper()
	for _, canary := range canaries {
		if err != nil {
			assert.NotContains(t, err.Error(), canary)
		}
		assert.NotContains(t, log.String(), canary)
	}
}

func (d *preflightHTTPDoer) Do(_ *http.Request) (*http.Response, error) {
	d.calls.Add(1)
	return nil, errors.New("preflight must not issue an HTTP request")
}

func TestProviderPreflightChecksAuthenticationWithoutExecutingPrompt(t *testing.T) {
	t.Run("CoreCLIPreflight", checkCoreCLIPreflight)
	t.Run("GeminiPreflight", checkGeminiPreflight)
	t.Run("OpenCodePreflight", checkOpenCodePreflight)
	t.Run("UnsupportedCLIPreflight", checkUnsupportedCLIPreflight)
	t.Run("OpenAICompatiblePreflight", checkOpenAICompatiblePreflight)
	t.Run("ProviderAvailability", checkProviderAvailability)
}

func TestCodexPreflightStatusStreams(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		stderr string
		err    error
		reason PreflightFailureReason
	}{
		{name: "stderr authenticated", stderr: "Logged in using ChatGPT\n"},
		{name: "stderr warning and status", stderr: "WARNING: could not create PATH aliases\nLogged in using ChatGPT\n"},
		{name: "stdout warning and status", stdout: "WARNING: startup warning\nLogged in using ChatGPT\n"},
		{name: "api key", stderr: "Logged in using an API key\r\n"},
		{name: "duplicate status", stdout: "Logged in using ChatGPT\n", stderr: "Logged in using ChatGPT\n"},
		{name: "stderr unauthenticated", stderr: "WARNING: startup warning\nNot logged in\n", err: errors.New("exit status 1"), reason: PreflightNotAuthenticated},
		{name: "unsuccessful authenticated command", stderr: "Logged in using ChatGPT\n", err: errors.New("exit status 1"), reason: PreflightStatusMalformed},
		{name: "status substring", stderr: "WARNING: Logged in using ChatGPT\n", reason: PreflightStatusMalformed},
		{name: "status suffix", stderr: "Logged in using ChatGPT account details\n", reason: PreflightStatusMalformed},
		{name: "conflicting streams", stdout: "Logged in using ChatGPT\n", stderr: "Not logged in\n", reason: PreflightStatusMalformed},
		{name: "conflicting lines", stderr: "Not logged in\nLogged in using ChatGPT\n", reason: PreflightStatusMalformed},
		{name: "warning only", stderr: "WARNING: startup warning\n", reason: PreflightStatusMalformed},
		{name: "empty", reason: PreflightStatusMalformed},
	}
	for i := range tests {
		tc := &tests[i]
		t.Run(tc.name, func(t *testing.T) {
			executor := newPreflightExecutor(tc.stdout, tc.stderr, tc.err)
			provider := NewCodexProviderWithOptions(WithCodexExecutor(executor))
			err := provider.Preflight(context.Background(), nil)
			if tc.reason == "" {
				require.NoError(t, err)
				return
			}
			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, tc.reason, preflightErr.Reason)
			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func checkCoreCLIPreflight(t *testing.T) {
	const (
		promptCanary = "PROMPT_MUST_NOT_EXECUTE"
		secretCanary = "SECRET_OPTION_MUST_NOT_LEAK"
		rawCanary    = "RAW_STATUS_MUST_NOT_LEAK"
	)

	// Verified locally from the target executables on 2026-10-20:
	//
	// Claude Code 2.1.301, `claude auth status --help` and isolated
	// `CLAUDE_CONFIG_DIR=<empty> claude auth status --json`:
	// args ["auth", "status", "--json"]; exit 0 with
	// `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty"}`;
	// exit 1 with `{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty"}`.
	// Unknown JSON is malformed; an unknown auth/status command is unsupported.
	// User, organization, directory, and credential values are intentionally omitted.
	//
	// Codex CLI 0.163.0, `codex login status --help` and isolated
	// `CODEX_HOME=<empty> codex login status`:
	// args ["login", "status"]; exit 0 with `Logged in using ChatGPT`; exit 1
	// with `Not logged in`. Unknown text is malformed; an unrecognized status
	// subcommand is unsupported. Account and credential values are omitted.
	providerCases := []struct {
		name       string
		provider   string
		args       []string
		stdout     string
		stderr     string
		execErr    error
		wantReason PreflightFailureReason
		wantCause  error
		new        func(ports.CLIExecutor) preflightProvider
	}{
		{
			name:     "claude authenticated",
			provider: "claude",
			args:     []string{"auth", "status", "--json"},
			stdout:   `{"loggedIn":true,"authMethod":"claude.ai","apiProvider":"firstParty"}`,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name:       "claude unauthenticated",
			provider:   "claude",
			args:       []string{"auth", "status", "--json"},
			stdout:     `{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty"}`,
			execErr:    errors.New("exit status 1"),
			wantReason: PreflightNotAuthenticated,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name:       "claude malformed status",
			provider:   "claude",
			args:       []string{"auth", "status", "--json"},
			stdout:     `{"loggedIn":"` + rawCanary + `"}`,
			wantReason: PreflightStatusMalformed,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name:       "claude unsupported command version",
			provider:   "claude",
			args:       []string{"auth", "status", "--json"},
			stderr:     "error: unknown command 'auth' " + rawCanary,
			execErr:    errors.New("exit status 1"),
			wantReason: PreflightStatusUnsupported,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name:       "claude cancelled status",
			provider:   "claude",
			args:       []string{"auth", "status", "--json"},
			execErr:    context.Canceled,
			wantReason: PreflightStatusMalformed,
			wantCause:  context.Canceled,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name:       "claude timed out status",
			provider:   "claude",
			args:       []string{"auth", "status", "--json"},
			execErr:    context.DeadlineExceeded,
			wantReason: PreflightStatusMalformed,
			wantCause:  context.DeadlineExceeded,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name:     "codex authenticated",
			provider: "codex",
			args:     []string{"login", "status"},
			stdout:   "Logged in using ChatGPT\n",
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
		{
			name:       "codex unauthenticated",
			provider:   "codex",
			args:       []string{"login", "status"},
			stdout:     "Not logged in\n",
			execErr:    errors.New("exit status 1"),
			wantReason: PreflightNotAuthenticated,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
		{
			name:       "codex malformed status",
			provider:   "codex",
			args:       []string{"login", "status"},
			stdout:     "Unexpected status " + rawCanary,
			wantReason: PreflightStatusMalformed,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
		{
			name:       "codex unsupported command version",
			provider:   "codex",
			args:       []string{"login", "status"},
			stderr:     "error: unrecognized subcommand 'status' " + rawCanary,
			execErr:    errors.New("exit status 2"),
			wantReason: PreflightStatusUnsupported,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
		{
			name:       "codex cancelled status",
			provider:   "codex",
			args:       []string{"login", "status"},
			execErr:    context.Canceled,
			wantReason: PreflightStatusMalformed,
			wantCause:  context.Canceled,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
		{
			name:       "codex timed out status",
			provider:   "codex",
			args:       []string{"login", "status"},
			execErr:    context.DeadlineExceeded,
			wantReason: PreflightStatusMalformed,
			wantCause:  context.DeadlineExceeded,
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
	}

	for i := range providerCases {
		tc := &providerCases[i]
		t.Run(tc.name, func(t *testing.T) {
			executor := newPreflightExecutor(tc.stdout, tc.stderr, tc.execErr)
			provider := tc.new(executor)
			options := map[string]any{"credential_canary": secretCanary, "prompt": promptCanary}
			wantOptions := map[string]any{"credential_canary": secretCanary, "prompt": promptCanary}

			err := provider.Preflight(context.Background(), options)

			assert.Equal(t, wantOptions, options)
			require.Len(t, executor.GetCalls(), 1)
			call := executor.GetCalls()[0]
			assert.Equal(t, tc.provider, call.Name)
			assert.Equal(t, tc.args, call.Args)
			assert.False(t, slices.Contains(call.Args, promptCanary))
			assert.False(t, slices.Contains(call.Args, secretCanary))

			if tc.wantReason == "" {
				assert.NoError(t, err)
				return
			}

			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, tc.provider, preflightErr.Provider)
			assert.Equal(t, tc.wantReason, preflightErr.Reason)
			assert.Equal(t, tc.provider+" preflight failed: "+string(tc.wantReason), err.Error())
			assert.NotContains(t, err.Error(), secretCanary)
			assert.NotContains(t, err.Error(), rawCanary)
			if tc.wantCause != nil {
				assert.ErrorIs(t, err, tc.wantCause)
			}
		})
	}

	for _, tc := range []struct {
		name string
		new  func(ports.CLIExecutor) preflightProvider
	}{
		{
			name: "claude cancellation returned as process error",
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewClaudeProviderWithOptions(WithClaudeExecutor(executor))
			},
		},
		{
			name: "codex cancellation returned as process error",
			new: func(executor ports.CLIExecutor) preflightProvider {
				return NewCodexProviderWithOptions(WithCodexExecutor(executor))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			processErr := errors.New("signal: killed")
			executor := newPreflightExecutor("", "", processErr)
			executor.afterRun = cancel

			err := tc.new(executor).Preflight(ctx, nil)

			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, PreflightStatusMalformed, preflightErr.Reason)
			assert.ErrorIs(t, err, context.Canceled)
			assert.ErrorIs(t, err, processErr)
		})
	}
}

func checkGeminiPreflight(t *testing.T) {
	const (
		promptCanary = "PROMPT_MUST_NOT_EXECUTE"
		secretCanary = "SECRET_OPTION_MUST_NOT_LEAK"
		rawCanary    = "RAW_STATUS_MUST_NOT_LEAK"
	)

	// Gemini CLI 0.50.0 `gemini --help` exposes no authentication/status command.
	// Its non-interactive `--prompt` mode is a billable model request, not a status
	// probe. Therefore no authenticated, unauthenticated, or malformed-output fixture
	// can be verified for this version: readiness must fail closed without execution.
	t.Run("gemini unsupported readiness mechanism", func(t *testing.T) {
		executor := newPreflightExecutor(rawCanary, rawCanary, nil)
		provider := NewGeminiProviderWithOptions(WithGeminiExecutor(executor))
		options := map[string]any{"credential_canary": secretCanary, "prompt": promptCanary}
		wantOptions := map[string]any{"credential_canary": secretCanary, "prompt": promptCanary}

		err := provider.Preflight(context.Background(), options)

		assert.Equal(t, wantOptions, options)
		assert.Empty(t, executor.GetCalls())
		var preflightErr *ProviderPreflightError
		require.ErrorAs(t, err, &preflightErr)
		assert.Equal(t, "gemini", preflightErr.Provider)
		assert.Equal(t, PreflightStatusUnsupported, preflightErr.Reason)
		assert.Equal(t, "gemini preflight failed: status_unsupported", err.Error())
		assert.NotContains(t, err.Error(), secretCanary)
		assert.NotContains(t, err.Error(), rawCanary)
	})

	for _, tc := range []struct {
		name  string
		cause error
		ctx   func() (context.Context, context.CancelFunc)
	}{
		{
			name:  "gemini cancelled before unsupported status",
			cause: context.Canceled,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
		},
		{
			name:  "gemini timed out before unsupported status",
			cause: context.DeadlineExceeded,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := newPreflightExecutor("", "", nil)
			provider := NewGeminiProviderWithOptions(WithGeminiExecutor(executor))
			ctx, cancel := tc.ctx()
			defer cancel()

			err := provider.Preflight(ctx, map[string]any{"credential_canary": secretCanary})

			assert.Empty(t, executor.GetCalls())
			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, "gemini", preflightErr.Provider)
			assert.Equal(t, PreflightStatusUnsupported, preflightErr.Reason)
			assert.ErrorIs(t, err, tc.cause)
			assert.NotContains(t, err.Error(), secretCanary)
		})
	}
}

func checkOpenCodePreflight(t *testing.T) {
	const (
		promptCanary = "PROMPT_MUST_NOT_EXECUTE"
		secretCanary = "SECRET_OPTION_MUST_NOT_LEAK"
		rawCanary    = "RAW_STATUS_MUST_NOT_LEAK"
	)

	// OpenCode 1.18.29 was verified on 2026-10-20 using the installed executable's
	// `opencode --version`, `opencode auth --help`, and `opencode auth list --help`.
	// OpenCode's official CLI documentation also identifies `auth list` as the
	// non-interactive command that lists authenticated providers. The exact argv is
	// ["auth", "list"]. Exit 0 with a positive `N credential(s)` footer is
	// authenticated; exit 0 with `0 credentials` is unauthenticated. Sanitized
	// examples are `│  ● Provider account` + `└  1 credential` and
	// `└  0 credentials`. Unknown text is malformed, while an unknown `auth`
	// subcommand/version error is unsupported. No credential values are retained.
	for _, tc := range []struct {
		name       string
		stdout     string
		stderr     string
		execErr    error
		wantReason PreflightFailureReason
		wantCause  error
	}{
		{
			name:   "opencode authenticated",
			stdout: "Credentials /tmp/redacted/opencode/auth.json\n│  ● Provider account\n└  1 credential\n",
		},
		{
			name:       "opencode unauthenticated",
			stdout:     "Credentials /tmp/redacted/opencode/auth.json\n└  0 credentials\n",
			wantReason: PreflightNotAuthenticated,
		},
		{
			name:       "opencode malformed status",
			stdout:     "Unexpected status " + rawCanary,
			execErr:    fmt.Errorf("probe output %s: %w", rawCanary, context.Canceled),
			wantReason: PreflightStatusMalformed,
			wantCause:  context.Canceled,
		},
		{
			name:       "opencode unsupported command version",
			stderr:     "Unknown command: auth " + rawCanary,
			execErr:    fmt.Errorf("probe output %s: %w", rawCanary, context.DeadlineExceeded),
			wantReason: PreflightStatusUnsupported,
			wantCause:  context.DeadlineExceeded,
		},
		{
			name:       "opencode cancelled status",
			execErr:    context.Canceled,
			wantReason: PreflightStatusMalformed,
			wantCause:  context.Canceled,
		},
		{
			name:       "opencode timed out status",
			execErr:    context.DeadlineExceeded,
			wantReason: PreflightStatusMalformed,
			wantCause:  context.DeadlineExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := newPreflightExecutor(tc.stdout, tc.stderr, tc.execErr)
			log := &preflightLogCapture{}
			provider := NewOpenCodeProviderWithOptions(WithOpenCodeExecutor(executor), WithOpenCodeLogger(log))
			options := map[string]any{
				"base_url":          "https://remote.example/v1",
				"credential_canary": secretCanary,
				"model":             "option-model",
				"prompt":            promptCanary,
				"raw_output_canary": rawCanary,
			}
			wantOptions := map[string]any{
				"base_url":          "https://remote.example/v1",
				"credential_canary": secretCanary,
				"model":             "option-model",
				"prompt":            promptCanary,
				"raw_output_canary": rawCanary,
			}

			err := provider.Preflight(context.Background(), options)

			assert.Equal(t, wantOptions, options)
			require.Len(t, executor.GetCalls(), 1)
			call := executor.GetCalls()[0]
			assert.Equal(t, "opencode", call.Name)
			assert.Equal(t, []string{"auth", "list"}, call.Args)
			assert.False(t, slices.Contains(call.Args, promptCanary))
			assert.False(t, slices.Contains(call.Args, secretCanary))
			if tc.wantReason == "" {
				assert.NoError(t, err)
				assertPreflightConfidential(t, log, err, secretCanary, rawCanary, promptCanary, "remote.example", "option-model")
				return
			}

			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, "opencode", preflightErr.Provider)
			assert.Equal(t, tc.wantReason, preflightErr.Reason)
			assert.Equal(t, "opencode preflight failed: "+string(tc.wantReason), err.Error())
			assertPreflightConfidential(t, log, err, secretCanary, rawCanary, promptCanary, "remote.example", "option-model")
			require.Error(t, preflightErr.Cause)
			for _, canary := range []string{secretCanary, rawCanary, promptCanary, "remote.example", "option-model"} {
				assert.NotContains(t, preflightErr.Cause.Error(), canary)
			}
			if tc.wantCause != nil {
				assert.ErrorIs(t, err, tc.wantCause)
				assert.ErrorIs(t, preflightErr.Cause, tc.wantCause)
			}
		})
	}
}

func checkUnsupportedCLIPreflight(t *testing.T) {
	const (
		promptCanary = "PROMPT_MUST_NOT_EXECUTE"
		secretCanary = "SECRET_OPTION_MUST_NOT_LEAK"
		rawCanary    = "RAW_STATUS_MUST_NOT_LEAK"
	)

	// GitHub Copilot CLI documentation checked on 2026-10-20 documents
	// `copilot login`, interactive `/user`, and `gh auth status`; none proves
	// Copilot readiness non-interactively. The local shim exposes no installed
	// version/help output. Mistral Vibe 2.3.0 `vibe --help` exposes `--setup` and
	// billable `--prompt`, but no status probe. Consequently neither provider has
	// an authenticated/unauthenticated output contract or argv to execute; sanitized
	// unsupported examples are "no non-interactive status command". Both must fail
	// closed without probing, prompting, scraping credentials, or checking presence.
	for _, tc := range []struct {
		name     string
		provider string
		new      func(ports.CLIExecutor, ports.Logger) preflightProvider
	}{
		{
			name:     "copilot unsupported readiness mechanism",
			provider: "copilot",
			new: func(executor ports.CLIExecutor, log ports.Logger) preflightProvider {
				return NewCopilotProviderWithOptions(WithCopilotExecutor(executor), WithCopilotLogger(log))
			},
		},
		{
			name:     "mistral vibe unsupported readiness mechanism",
			provider: "mistral_vibe",
			new: func(executor ports.CLIExecutor, log ports.Logger) preflightProvider {
				return NewMistralVibeProviderWithOptions(WithMistralVibeExecutor(executor), WithMistralVibeLogger(log))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := newPreflightExecutor(rawCanary, rawCanary, nil)
			log := &preflightLogCapture{}
			options := map[string]any{"credential_canary": secretCanary, "prompt": promptCanary}
			wantOptions := map[string]any{"credential_canary": secretCanary, "prompt": promptCanary}

			err := tc.new(executor, log).Preflight(context.Background(), options)

			assert.Equal(t, wantOptions, options)
			assert.Empty(t, executor.GetCalls())
			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, tc.provider, preflightErr.Provider)
			assert.Equal(t, PreflightStatusUnsupported, preflightErr.Reason)
			assert.Equal(t, tc.provider+" preflight failed: status_unsupported", err.Error())
			assertPreflightConfidential(t, log, err, secretCanary, rawCanary, promptCanary)
		})
	}

	for _, tc := range []struct {
		name     string
		provider string
		cause    error
		ctx      func() (context.Context, context.CancelFunc)
		new      func(ports.CLIExecutor, ports.Logger) preflightProvider
	}{
		{
			name:     "copilot cancelled before unsupported status",
			provider: "copilot",
			cause:    context.Canceled,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			new: func(executor ports.CLIExecutor, log ports.Logger) preflightProvider {
				return NewCopilotProviderWithOptions(WithCopilotExecutor(executor), WithCopilotLogger(log))
			},
		},
		{
			name:     "copilot timed out before unsupported status",
			provider: "copilot",
			cause:    context.DeadlineExceeded,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			new: func(executor ports.CLIExecutor, log ports.Logger) preflightProvider {
				return NewCopilotProviderWithOptions(WithCopilotExecutor(executor), WithCopilotLogger(log))
			},
		},
		{
			name:     "mistral vibe cancelled before unsupported status",
			provider: "mistral_vibe",
			cause:    context.Canceled,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			new: func(executor ports.CLIExecutor, log ports.Logger) preflightProvider {
				return NewMistralVibeProviderWithOptions(WithMistralVibeExecutor(executor), WithMistralVibeLogger(log))
			},
		},
		{
			name:     "mistral vibe timed out before unsupported status",
			provider: "mistral_vibe",
			cause:    context.DeadlineExceeded,
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			new: func(executor ports.CLIExecutor, log ports.Logger) preflightProvider {
				return NewMistralVibeProviderWithOptions(WithMistralVibeExecutor(executor), WithMistralVibeLogger(log))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := newPreflightExecutor("", "", nil)
			log := &preflightLogCapture{}
			ctx, cancel := tc.ctx()
			defer cancel()

			err := tc.new(executor, log).Preflight(ctx, nil)

			assert.Empty(t, executor.GetCalls())
			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, tc.provider, preflightErr.Provider)
			assert.Equal(t, PreflightStatusUnsupported, preflightErr.Reason)
			assert.ErrorIs(t, err, tc.cause)
			assert.ErrorIs(t, preflightErr.Cause, tc.cause)
			assertPreflightConfidential(t, log, err, secretCanary, rawCanary, promptCanary)
		})
	}
}

func checkOpenAICompatiblePreflight(t *testing.T) {
	const (
		secretCanary = "SECRET_OPTION_MUST_NOT_LEAK"
	)

	t.Setenv("OPENAI_BASE_URL", "https://environment.invalid/v1")
	t.Setenv("OPENAI_MODEL", "environment-model")
	t.Setenv("OPENAI_API_KEY", "environment-"+secretCanary)
	for _, tc := range []struct {
		name         string
		options      map[string]any
		envBaseURL   string
		envModel     string
		unsetBaseURL bool
		unsetModel   bool
		envAPIKey    string
		wantReason   PreflightFailureReason
	}{
		{
			name: "openai compatible option values take precedence",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    "option-model",
				"api_key":  "option-" + secretCanary,
			},
		},
		{
			name:    "openai compatible environment fallbacks",
			options: map[string]any{},
		},
		{
			name:         "openai compatible missing environment URL",
			options:      map[string]any{},
			unsetBaseURL: true,
			wantReason:   PreflightConfigurationInvalid,
		},
		{
			name:       "openai compatible empty environment URL",
			options:    map[string]any{},
			envBaseURL: "",
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name:       "openai compatible whitespace environment URL",
			options:    map[string]any{},
			envBaseURL: "  ",
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name:       "openai compatible missing environment model",
			options:    map[string]any{},
			unsetModel: true,
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name:       "openai compatible empty environment model",
			options:    map[string]any{},
			envModel:   "",
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name:       "openai compatible whitespace environment model",
			options:    map[string]any{},
			envModel:   "  ",
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible remote keyless endpoint",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    "remote-model",
				"api_key":  "",
			},
		},
		{
			name: "openai compatible local keyless endpoint",
			options: map[string]any{
				"base_url": "http://127.0.0.1:11434/v1",
				"model":    "local-model",
			},
			envAPIKey: "",
		},
		{
			name: "openai compatible present empty URL does not use environment",
			options: map[string]any{
				"base_url": "  ",
				"model":    "option-model",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible nil URL does not use environment",
			options: map[string]any{
				"base_url": nil,
				"model":    "option-model",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible non-string URL",
			options: map[string]any{
				"base_url": 42,
				"model":    "option-model",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible malformed URL",
			options: map[string]any{
				"base_url": "https://[::1",
				"model":    "option-model",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible non HTTP URL",
			options: map[string]any{
				"base_url": "file:///tmp/" + secretCanary,
				"model":    "option-model",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible URL missing host",
			options: map[string]any{
				"base_url": "https:///" + secretCanary,
				"model":    "option-model",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible present empty model does not use environment",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    "  ",
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible nil model does not use environment",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    nil,
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible non-string model",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    42,
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible nil API key",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    "option-model",
				"api_key":  nil,
			},
			wantReason: PreflightConfigurationInvalid,
		},
		{
			name: "openai compatible non-string API key",
			options: map[string]any{
				"base_url": "https://remote.example/v1",
				"model":    "option-model",
				"api_key":  42,
			},
			wantReason: PreflightConfigurationInvalid,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unsetBaseURL {
				unsetPreflightEnv(t, "OPENAI_BASE_URL")
			} else if tc.envBaseURL != "" || tc.name == "openai compatible empty environment URL" {
				t.Setenv("OPENAI_BASE_URL", tc.envBaseURL)
			}
			if tc.unsetModel {
				unsetPreflightEnv(t, "OPENAI_MODEL")
			} else if tc.envModel != "" || tc.name == "openai compatible empty environment model" {
				t.Setenv("OPENAI_MODEL", tc.envModel)
			}
			if tc.envAPIKey != "" || tc.name == "openai compatible local keyless endpoint" {
				t.Setenv("OPENAI_API_KEY", tc.envAPIKey)
			}
			doer := &preflightHTTPDoer{}
			provider := NewOpenAICompatibleProvider(WithHTTPClient(httpx.NewClient(httpx.WithDoer(doer))))
			wantOptions := make(map[string]any, len(tc.options))
			for key, value := range tc.options {
				wantOptions[key] = value
			}

			err := provider.Preflight(context.Background(), tc.options)

			assert.Equal(t, wantOptions, tc.options)
			assert.Zero(t, doer.calls.Load())
			if tc.wantReason == "" {
				assert.NoError(t, err)
				return
			}

			var preflightErr *ProviderPreflightError
			require.ErrorAs(t, err, &preflightErr)
			assert.Equal(t, "openai_compatible", preflightErr.Provider)
			assert.Equal(t, PreflightConfigurationInvalid, preflightErr.Reason)
			assert.Equal(t, "openai_compatible preflight failed: configuration_invalid", err.Error())
			assert.NotContains(t, err.Error(), secretCanary)
			assert.NotContains(t, err.Error(), "remote.example")
			assert.NotContains(t, err.Error(), "option-model")
			require.Error(t, preflightErr.Cause)
			for _, canary := range []string{secretCanary, "remote.example", "option-model"} {
				assert.NotContains(t, preflightErr.Cause.Error(), canary)
			}
		})
	}

	t.Run("openai compatible execution uses normalized preflight configuration", func(t *testing.T) {
		var requestURL, requestModel string
		doer := preflightHTTPDoerFunc(func(request *http.Request) (*http.Response, error) {
			requestURL = request.URL.String()
			var payload chatCompletionsRequest
			require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
			requestModel = payload.Model
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"choices":[{"message":{"role":"assistant","content":"ready"},"finish_reason":"stop"}],"usage":{}}`,
				)),
			}, nil
		})
		provider := NewOpenAICompatibleProvider(WithHTTPClient(httpx.NewClient(httpx.WithDoer(doer))))
		options := map[string]any{
			"base_url": "  https://remote.example/v1/  ",
			"model":    "  normalized-model  ",
		}

		require.NoError(t, provider.Preflight(context.Background(), options))
		_, err := provider.Execute(context.Background(), "safe prompt", options, nil, nil)

		require.NoError(t, err)
		assert.Equal(t, "https://remote.example/v1/chat/completions", requestURL)
		assert.Equal(t, "normalized-model", requestModel)
	})
}

func checkProviderAvailability(t *testing.T) {
	// Validate remains the binary-availability boundary. These calls deliberately do
	// not invoke Preflight, so an authenticated CLI cannot mask a missing binary and
	// a rejected readiness probe cannot alter Validate's established contract.
	for _, tc := range []struct {
		name   string
		binary string
		new    func() ports.AgentProvider
	}{
		{
			name:   "opencode",
			binary: "opencode",
			new:    func() ports.AgentProvider { return NewOpenCodeProvider() },
		},
		{
			name:   "copilot",
			binary: "copilot",
			new:    func() ports.AgentProvider { return NewCopilotProvider() },
		},
		{
			name:   "mistral vibe",
			binary: "vibe",
			new:    func() ports.AgentProvider { return NewMistralVibeProvider() },
		},
	} {
		t.Run(tc.name+" Validate rejects an unavailable binary", func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			assert.Error(t, tc.new().Validate())
		})
		t.Run(tc.name+" Validate accepts an available binary", func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.binary)
			require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
			t.Setenv("PATH", dir)
			assert.NoError(t, tc.new().Validate())
		})
	}

	for _, tc := range []struct {
		name  string
		setup func(t *testing.T)
	}{
		{
			name: "openai compatible Validate accepts configured transport",
			setup: func(t *testing.T) {
				t.Setenv("OPENAI_BASE_URL", "https://available.example/v1")
				t.Setenv("OPENAI_MODEL", "available-model")
			},
		},
		{
			name: "openai compatible Validate accepts absent transport configuration",
			setup: func(t *testing.T) {
				t.Setenv("OPENAI_BASE_URL", "")
				t.Setenv("OPENAI_MODEL", "")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)
			assert.NoError(t, NewOpenAICompatibleProvider().Validate())
		})
	}
}

func unsetPreflightEnv(t *testing.T, key string) {
	t.Helper()
	// Setenv registers restoration and enforces testing's environment/parallelism
	// guard even though the contract needs an absent variable rather than "".
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}
