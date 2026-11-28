package tracker

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
)

func validateInputs(schema *pluginmodel.OperationSchema, operation string, inputs map[string]any) error {
	for field, value := range inputs {
		input, exists := schema.Inputs[field]
		if !exists || !matchesInputType(value, input.Type) {
			return invalidField(field)
		}
	}
	for field, input := range schema.Inputs {
		if !input.Required {
			continue
		}
		value, exists := inputs[field]
		if !exists || isBlankString(value) {
			return invalidField(field)
		}
	}
	if requiresIdentity(operation) && isBlankInput(inputs["id"]) && isBlankInput(inputs["key"]) {
		return invalidField("id or key")
	}
	return nil
}

func invalidField(field string) error { return fmt.Errorf("%s: %w", field, ErrInvalidTrackerInput) }

func requiresIdentity(operation string) bool {
	switch operation {
	case updateIssueOperation, transitionOperation, addCommentOperation, getIssueOperation:
		return true
	default:
		return false
	}
}

func isBlankInput(value any) bool {
	text, ok := value.(string)
	return !ok || strings.TrimSpace(text) == ""
}

func isBlankString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

func matchesInputType(value any, inputType string) bool {
	if value == nil {
		return false
	}
	switch inputType {
	case pluginmodel.InputTypeString:
		_, ok := value.(string)
		return ok
	case pluginmodel.InputTypeInteger:
		switch number := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case float64:
			return !math.IsNaN(number) && !math.IsInf(number, 0) && number == math.Trunc(number)
		default:
			return false
		}
	case pluginmodel.InputTypeArray:
		return reflect.TypeOf(value).Kind() == reflect.Slice
	default:
		return false
	}
}

func canonicalResult(operation string, result *pluginmodel.OperationResult) (*pluginmodel.OperationResult, error) {
	if result == nil {
		return nil, fmt.Errorf("adapter result: %w", ErrMalformedAdapterResult)
	}
	if !result.Success {
		if strings.TrimSpace(result.Error) == "" {
			return nil, fmt.Errorf("adapter result: %w", ErrMalformedAdapterResult)
		}
		return &pluginmodel.OperationResult{
			Success: false,
			Error:   "tracker adapter reported failure",
		}, nil
	}
	if result.Outputs == nil {
		return nil, fmt.Errorf("adapter result: %w", ErrMalformedAdapterResult)
	}
	schema, found := (&Provider{}).GetOperation(operation)
	if !found {
		return nil, invalidField("operation")
	}
	outputs := make(map[string]any, len(schema.Outputs))
	for _, field := range schema.Outputs {
		value, exists := result.Outputs[field]
		canonicalValue, valid := canonicalOutput(field, value)
		if !exists || !valid {
			return nil, fmt.Errorf("adapter result field %s: %w", field, ErrMalformedAdapterResult)
		}
		outputs[field] = canonicalValue
	}
	return &pluginmodel.OperationResult{Success: true, Outputs: outputs}, nil
}

func canonicalOutput(field string, value any) (any, bool) {
	switch field {
	case "total":
		return value, matchesInputType(value, pluginmodel.InputTypeInteger)
	case "issues":
		return canonicalIssues(value)
	default:
		_, ok := value.(string)
		return value, ok
	}
}

func canonicalIssues(value any) ([]any, bool) {
	if value == nil || reflect.TypeOf(value).Kind() != reflect.Slice {
		return nil, false
	}

	issueFields := []string{"id", "key", "title", "status", "description", "assignee", "url"}
	items := reflect.ValueOf(value)
	issues := make([]any, 0, items.Len())
	for index := 0; index < items.Len(); index++ {
		issue, ok := items.Index(index).Interface().(map[string]any)
		if !ok {
			return nil, false
		}
		canonicalIssue := make(map[string]any, len(issueFields))
		for _, field := range issueFields {
			text, isString := issue[field].(string)
			if !isString {
				return nil, false
			}
			canonicalIssue[field] = text
		}
		issues = append(issues, canonicalIssue)
	}
	return issues, true
}
