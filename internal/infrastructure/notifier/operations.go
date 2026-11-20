package notifier

import (
	"fmt"
	"strings"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

func operationSchemas(defaultChannel string) []*pluginmodel.OperationSchema {
	channelRequired := strings.TrimSpace(defaultChannel) == ""
	channel := pluginmodel.InputSchema{Type: pluginmodel.InputTypeString, Required: channelRequired}
	if !channelRequired {
		channel.Default = defaultChannel
		channel.Validation = "nonblank"
	}
	backend := pluginmodel.InputSchema{Type: pluginmodel.InputTypeString}
	requiredString := pluginmodel.InputSchema{Type: pluginmodel.InputTypeString, Required: true}
	optionalString := pluginmodel.InputSchema{Type: pluginmodel.InputTypeString}

	return []*pluginmodel.OperationSchema{
		{
			Name: OperationSend, PluginName: "notifier",
			Inputs: map[string]pluginmodel.InputSchema{
				"backend": backend, "channel": channel, "text": requiredString,
				"rich": {Type: pluginmodel.InputTypeObject}, "thread_id": optionalString,
			},
			Outputs: []string{"message_id", "channel"},
		},
		{
			Name: OperationUpdate, PluginName: "notifier",
			Inputs: map[string]pluginmodel.InputSchema{
				"backend": backend, "channel": channel, "message_id": requiredString,
				"text": requiredString, "rich": {Type: pluginmodel.InputTypeObject},
			},
			Outputs: []string{"message_id"},
		},
		{
			Name: OperationReact, PluginName: "notifier",
			Inputs: map[string]pluginmodel.InputSchema{
				"backend": backend, "channel": channel, "message_id": requiredString,
				"emoji": requiredString,
			},
			Outputs: []string{"ok"},
		},
		{
			Name: OperationUpload, PluginName: "notifier",
			Inputs: map[string]pluginmodel.InputSchema{
				"backend": backend, "channel": channel, "filename": requiredString,
				"content": requiredString, "title": optionalString,
			},
			Outputs: []string{"file_id", "permalink"},
		},
	}
}

func normalizeRequest(name string, inputs map[string]any, defaults settings) (normalizedRequest, error) {
	required, known := requiredInputs(name)
	if !known {
		return normalizedRequest{}, invalidInput("operation", fmt.Sprintf("unknown operation %q", name))
	}

	backend, err := resolveString(inputs, "backend", defaults.backend, true)
	if err != nil {
		return normalizedRequest{}, err
	}
	if strings.Contains(backend, ".") {
		return normalizedRequest{}, invalidInput("backend", "must not contain dots")
	}
	if backend == "notifier" {
		return normalizedRequest{}, invalidInput("backend", "is reserved")
	}
	channel, err := resolveString(inputs, "channel", defaults.channel, true)
	if err != nil {
		return normalizedRequest{}, err
	}

	for _, field := range required {
		if validationErr := validateString(inputs, field, true); validationErr != nil {
			return normalizedRequest{}, validationErr
		}
	}
	for _, field := range []string{"thread_id", "title"} {
		if validationErr := validateString(inputs, field, false); validationErr != nil {
			return normalizedRequest{}, validationErr
		}
	}
	if rich, exists := inputs["rich"]; exists {
		switch rich.(type) {
		case map[string]any, map[string]string:
		default:
			return normalizedRequest{}, invalidInput("rich", "must be an object")
		}
	}

	adapterInputs := make(map[string]any, len(inputs)+1)
	for key, value := range inputs {
		if key != "backend" {
			adapterInputs[key] = value
		}
	}
	adapterInputs["channel"] = channel
	return normalizedRequest{backend: backend, operation: strings.TrimPrefix(name, "notifier."), inputs: adapterInputs}, nil
}

func requiredInputs(name string) ([]string, bool) {
	switch name {
	case OperationSend:
		return []string{"text"}, true
	case OperationUpdate:
		return []string{"message_id", "text"}, true
	case OperationReact:
		return []string{"message_id", "emoji"}, true
	case OperationUpload:
		return []string{"filename", "content"}, true
	default:
		return nil, false
	}
}

func resolveString(inputs map[string]any, field, fallback string, required bool) (string, error) {
	value, exists := inputs[field]
	if !exists {
		if strings.TrimSpace(fallback) == "" && required {
			return "", invalidInput(field, "is required")
		}
		return fallback, nil
	}
	stringValue, ok := value.(string)
	if !ok {
		return "", invalidInput(field, "must be a string")
	}
	if strings.TrimSpace(stringValue) == "" {
		return "", invalidInput(field, "must not be blank")
	}
	return stringValue, nil
}

func validateString(inputs map[string]any, field string, required bool) error {
	_, err := resolveString(inputs, field, "", required)
	return err
}

func invalidInput(field, reason string) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalidInput, field, reason)
}
