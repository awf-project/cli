package acp

import (
	"github.com/awf-project/cli/internal/application"
	sdk "github.com/coder/acp-go-sdk"
)

// toACPError converts an application-layer ACPHandlerError to the SDK RequestError
// variant understood by the ACP transport. Returns nil when e is nil so callers can
// pass handler results directly without an extra nil-check.
//
// Both Message (human-readable detail) and Data (machine-readable code) are carried
// through. The SDK's NewInvalidParams/NewInternalError constructors hardcode Message to
// a generic string ("Invalid params"/"Internal error") and only accept Data, so using
// them would silently DROP e.Message — leaving the editor with no detail (the exact
// regression that defeated the human-readable-message design). A direct *sdk.RequestError
// preserves both fields; sdk.toReqErr passes a *RequestError through unchanged.
func toACPError(e *application.ACPHandlerError) error {
	if e == nil {
		return nil
	}
	switch e.Kind {
	case application.ACPErrMethodNotFound:
		// NewMethodNotFound carries the method name in Data{"method": ...}; the reserved
		// MethodNotFound path has no human-readable detail to preserve beyond it.
		return sdk.NewMethodNotFound(e.Message)
	case application.ACPErrInvalidParams:
		return &sdk.RequestError{Code: -32602, Message: e.Message, Data: e.Data}
	default: // ACPErrInternal
		return &sdk.RequestError{Code: -32603, Message: e.Message, Data: e.Data}
	}
}

func invalidParamsErr(msg string) error {
	return toACPError(&application.ACPHandlerError{Kind: application.ACPErrInvalidParams, Message: msg})
}

func internalErr(msg string) error {
	return toACPError(&application.ACPHandlerError{Kind: application.ACPErrInternal, Message: msg})
}

func methodNotFoundErr(method string) error {
	return toACPError(&application.ACPHandlerError{Kind: application.ACPErrMethodNotFound, Message: method})
}
