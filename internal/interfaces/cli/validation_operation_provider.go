package cli

import (
	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/github"
	infrahttp "github.com/awf-project/cli/internal/infrastructure/http"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
	"github.com/awf-project/cli/internal/infrastructure/tracker"
	"github.com/awf-project/cli/internal/interfaces/notifierwiring"
)

func buildValidationOperationProvider(
	pluginService *application.PluginService,
	external ports.OperationProvider,
	logger ports.Logger,
	notifierSettings notifierwiring.Settings,
) ports.OperationProvider {
	var githubProvider ports.OperationProvider
	if pluginService.IsPluginEnabled("github") {
		githubProvider = github.NewGitHubOperationProvider(nil, logger)
	}

	var notifyProvider ports.OperationProvider
	if pluginService.IsPluginEnabled("notify") {
		notifyProvider = notify.NewNotifyOperationProvider(logger)
	}

	var httpProvider ports.OperationProvider
	if pluginService.IsPluginEnabled("http") {
		httpProvider = infrahttp.NewHTTPOperationProvider(nil, logger)
	}

	notifierProvider := notifierwiring.ComposeOperationProvider(
		notifierSettings,
		pluginService.IsPluginEnabled("notifier"),
		external,
	)
	var trackerProvider ports.OperationProvider
	if pluginService.IsPluginEnabled("tracker") {
		trackerProvider = tracker.NewProvider(nil, "", logger)
	}
	return pluginmgr.NewCompositeOperationProvider(githubProvider, notifyProvider, httpProvider, notifierProvider, trackerProvider)
}
