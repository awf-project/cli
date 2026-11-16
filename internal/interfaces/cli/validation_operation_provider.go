package cli

import (
	"github.com/awf-project/cli/internal/application"
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/github"
	infrahttp "github.com/awf-project/cli/internal/infrastructure/http"
	"github.com/awf-project/cli/internal/infrastructure/notify"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
)

func buildValidationOperationProvider(
	pluginService *application.PluginService,
	external ports.OperationProvider,
	logger ports.Logger,
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

	return pluginmgr.NewCompositeOperationProvider(githubProvider, notifyProvider, httpProvider, external)
}
