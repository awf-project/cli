package notifier

import (
	"errors"

	"github.com/awf-project/cli/internal/domain/ports"
)

const (
	OperationSend   = "notifier.send"
	OperationUpdate = "notifier.update"
	OperationReact  = "notifier.react"
	OperationUpload = "notifier.upload"
)

var (
	ErrInvalidInput       = errors.New("notifier: invalid input")
	ErrAdapterUnavailable = errors.New("notifier: adapter operation unavailable")
)

type Provider struct {
	adapters       ports.OperationProvider
	defaultBackend string
	defaultChannel string
}

type settings struct {
	backend string
	channel string
}

type normalizedRequest struct {
	backend   string
	operation string
	inputs    map[string]any
}
