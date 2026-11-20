package notifierwiring

import (
	"github.com/awf-project/cli/internal/domain/ports"
	"github.com/awf-project/cli/internal/infrastructure/notifier"
	"github.com/awf-project/cli/internal/infrastructure/pluginmgr"
)

type Settings struct {
	Backend        string
	DefaultChannel string
}

func ComposeOperationProvider(
	settings Settings,
	notifierEnabled bool,
	external ports.OperationProvider,
) ports.OperationProvider {
	if !notifierEnabled {
		return external
	}

	notifierProvider := notifier.NewProvider(external, settings.Backend, settings.DefaultChannel)
	return pluginmgr.NewCompositeOperationProvider(notifierProvider, external)
}
