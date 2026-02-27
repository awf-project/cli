package http

import (
	"testing"

	"github.com/awf-project/cli/internal/domain/pluginmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllOperations_ReturnsOneOperation(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: it returns exactly 1 operation
	assert.Len(t, ops, 1, "AllOperations should return 1 HTTP operation")
}

func TestAllOperations_HTTPRequestOperationExists(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: the http.request operation exists
	require.Len(t, ops, 1)
	assert.Equal(t, "http.request", ops[0].Name, "operation name should be http.request")
}

func TestAllOperations_HTTPRequestHasDescription(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: http.request has a non-empty description
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Description, "http.request should have a description")
	assert.Contains(t, ops[0].Description, "HTTP", "description should mention HTTP")
}

func TestAllOperations_HTTPRequestHasInputs(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: http.request has inputs defined
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Inputs, "http.request should have inputs")
}

func TestAllOperations_HTTPRequestHasOutputs(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: http.request has outputs defined
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Outputs, "http.request should have outputs")
}

func TestHTTPRequestOperation_RequiredInputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify url is required (FR-002)
	urlInput, exists := op.Inputs["url"]
	require.True(t, exists, "url input should exist")
	assert.True(t, urlInput.Required, "url should be required")
	assert.Equal(t, pluginmodel.InputTypeString, urlInput.Type, "url should be string type")
	assert.NotEmpty(t, urlInput.Description, "url should have description")
	assert.Contains(t, urlInput.Description, "http://", "url description should mention http://")
	assert.Contains(t, urlInput.Description, "https://", "url description should mention https://")
	assert.Equal(t, "url", urlInput.Validation, "url should have url validation rule")

	// Verify method is required (FR-002)
	methodInput, exists := op.Inputs["method"]
	require.True(t, exists, "method input should exist")
	assert.True(t, methodInput.Required, "method should be required")
	assert.Equal(t, pluginmodel.InputTypeString, methodInput.Type, "method should be string type")
	assert.NotEmpty(t, methodInput.Description, "method should have description")
	assert.Contains(t, methodInput.Description, "GET", "method description should list GET")
	assert.Contains(t, methodInput.Description, "POST", "method description should list POST")
	assert.Contains(t, methodInput.Description, "PUT", "method description should list PUT")
	assert.Contains(t, methodInput.Description, "DELETE", "method description should list DELETE")
}

func TestHTTPRequestOperation_OptionalInputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Table-driven test for optional inputs
	optionalInputs := []struct {
		name        string
		shouldExist bool
		inputType   string
		description string
	}{
		{
			name:        "headers",
			shouldExist: true,
			inputType:   pluginmodel.InputTypeObject,
			description: "headers",
		},
		{
			name:        "body",
			shouldExist: true,
			inputType:   pluginmodel.InputTypeString,
			description: "body",
		},
		{
			name:        "timeout",
			shouldExist: true,
			inputType:   pluginmodel.InputTypeInteger,
			description: "timeout",
		},
		{
			name:        "retryable_status_codes",
			shouldExist: true,
			inputType:   pluginmodel.InputTypeArray,
			description: "retryable",
		},
	}

	for _, tt := range optionalInputs {
		t.Run(tt.name, func(t *testing.T) {
			input, exists := op.Inputs[tt.name]
			if tt.shouldExist {
				require.True(t, exists, "%s input should exist", tt.name)
				assert.False(t, input.Required, "%s should be optional", tt.name)
				assert.Equal(t, tt.inputType, input.Type, "%s should be %s type", tt.name, tt.inputType)
				assert.NotEmpty(t, input.Description, "%s should have description", tt.name)
				assert.Contains(t, input.Description, tt.description, "%s description should mention %s", tt.name, tt.description)
			} else {
				assert.False(t, exists, "%s input should not exist", tt.name)
			}
		})
	}
}

func TestHTTPRequestOperation_TimeoutInputDefaults(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify timeout has default value (FR-006)
	timeoutInput := op.Inputs["timeout"]
	assert.Equal(t, 30, timeoutInput.Default, "timeout should default to 30 seconds")
	assert.Contains(t, timeoutInput.Description, "default: 30", "timeout description should mention default value")
}

func TestHTTPRequestOperation_HeadersInputType(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify headers is object type (FR-002)
	headersInput := op.Inputs["headers"]
	assert.Equal(t, pluginmodel.InputTypeObject, headersInput.Type, "headers should be object type")
	assert.Contains(t, headersInput.Description, "key-value", "headers description should mention key-value pairs")
}

func TestHTTPRequestOperation_BodyInputType(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify body is raw string (FR-002, spec clarification)
	bodyInput := op.Inputs["body"]
	assert.Equal(t, pluginmodel.InputTypeString, bodyInput.Type, "body should be string type")
	assert.Contains(t, bodyInput.Description, "raw string", "body description should mention raw string")
	assert.Contains(t, bodyInput.Description, "JSON encoding is caller's responsibility", "body description should clarify JSON encoding responsibility")
}

func TestHTTPRequestOperation_RetryableStatusCodesInputType(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify retryable_status_codes is array type (FR-002)
	retryableInput := op.Inputs["retryable_status_codes"]
	assert.Equal(t, pluginmodel.InputTypeArray, retryableInput.Type, "retryable_status_codes should be array type")
	assert.Contains(t, retryableInput.Description, "retryable failures", "retryable_status_codes description should mention retryable failures")
	assert.Contains(t, retryableInput.Description, "429", "retryable_status_codes description should provide example 429")
	assert.Contains(t, retryableInput.Description, "502", "retryable_status_codes description should provide example 502")
	assert.Contains(t, retryableInput.Description, "503", "retryable_status_codes description should provide example 503")
}

func TestHTTPRequestOperation_Outputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify expected outputs (FR-003)
	expectedOutputs := []string{"status_code", "body", "headers", "body_truncated"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs, "http.request should have status_code, body, headers, and body_truncated outputs")
}

func TestHTTPRequestOperation_OutputsAreUnique(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify no duplicate outputs
	seen := make(map[string]bool)
	for _, output := range op.Outputs {
		assert.False(t, seen[output], "output %q should not be duplicated", output)
		seen[output] = true
	}
}

func TestHTTPRequestOperation_AllInputCount(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify total input count (2 required + 4 optional = 6)
	expectedInputs := []string{"url", "method", "headers", "body", "timeout", "retryable_status_codes"}
	assert.Len(t, op.Inputs, len(expectedInputs), "http.request should have exactly 6 inputs")

	for _, inputName := range expectedInputs {
		_, exists := op.Inputs[inputName]
		assert.True(t, exists, "input %q should exist", inputName)
	}
}

func TestAllOperations_ImmutabilityCheck(t *testing.T) {
	// Given: AllOperations is called twice
	ops1 := AllOperations()
	ops2 := AllOperations()

	// Then: both calls return the same data
	require.Len(t, ops1, len(ops2))
	assert.Equal(t, ops1[0].Name, ops2[0].Name)
	assert.Equal(t, ops1[0].Description, ops2[0].Description)
	assert.Equal(t, len(ops1[0].Inputs), len(ops2[0].Inputs))
	assert.Equal(t, len(ops1[0].Outputs), len(ops2[0].Outputs))
}

func TestAllOperations_NoEmptyName(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: the operation name is not empty
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Name, "operation name should not be empty")
}

func TestAllOperations_AllInputsHaveDescriptions(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Then: all inputs have non-empty descriptions
	for inputName, inputSchema := range op.Inputs {
		assert.NotEmpty(t, inputSchema.Description,
			"input %q should have a description", inputName)
	}
}

func TestAllOperations_AllInputsHaveValidTypes(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Then: all inputs have valid types
	validTypes := map[string]bool{
		pluginmodel.InputTypeString:  true,
		pluginmodel.InputTypeInteger: true,
		pluginmodel.InputTypeBoolean: true,
		pluginmodel.InputTypeArray:   true,
		pluginmodel.InputTypeObject:  true,
	}

	for inputName, inputSchema := range op.Inputs {
		assert.True(t, validTypes[inputSchema.Type],
			"input %q has invalid type %v", inputName, inputSchema.Type)
	}
}

func TestHTTPRequestOperation_NoUnexpectedInputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify only expected inputs exist (no extras)
	expectedInputs := map[string]bool{
		"url":                    true,
		"method":                 true,
		"headers":                true,
		"body":                   true,
		"timeout":                true,
		"retryable_status_codes": true,
	}

	for inputName := range op.Inputs {
		assert.True(t, expectedInputs[inputName],
			"unexpected input %q found in schema", inputName)
	}
}

func TestHTTPRequestOperation_RequiredInputsCount(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Count required inputs
	requiredCount := 0
	for _, inputSchema := range op.Inputs {
		if inputSchema.Required {
			requiredCount++
		}
	}

	// Should have exactly 2 required inputs (url and method)
	assert.Equal(t, 2, requiredCount, "should have exactly 2 required inputs")
}

func TestHTTPRequestOperation_OptionalInputsCount(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Count optional inputs
	optionalCount := 0
	for _, inputSchema := range op.Inputs {
		if !inputSchema.Required {
			optionalCount++
		}
	}

	// Should have exactly 4 optional inputs
	assert.Equal(t, 4, optionalCount, "should have exactly 4 optional inputs")
}

func TestHTTPRequestOperation_URLValidationRule(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify url has validation rule (FR-005)
	urlInput := op.Inputs["url"]
	assert.Equal(t, "url", urlInput.Validation, "url should have 'url' validation rule")
}

func TestHTTPRequestOperation_MethodCaseInsensitivityDocumented(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify method description mentions case-insensitivity (FR-004)
	methodInput := op.Inputs["method"]
	assert.Contains(t, methodInput.Description, "case-insensitive", "method description should mention case-insensitivity")
}

func TestAllOperations_NoEmptyOutputFields(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Then: no output field is empty
	for _, outputField := range op.Outputs {
		assert.NotEmpty(t, outputField, "output field should not be empty")
	}
}

func TestAllOperations_NoDuplicateOutputFields(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Then: no duplicate output fields
	seen := make(map[string]bool)
	for _, outputField := range op.Outputs {
		assert.False(t, seen[outputField],
			"duplicate output field: %q", outputField)
		seen[outputField] = true
	}
}

func TestHTTPRequestOperation_URLInputNotEmpty(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// URL is required and should have validation
	urlInput := op.Inputs["url"]
	assert.True(t, urlInput.Required, "url must be required to prevent empty values")
	assert.NotEmpty(t, urlInput.Validation, "url should have validation rule")
}

func TestHTTPRequestOperation_MethodInputNotEmpty(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Method is required and should be non-empty
	methodInput := op.Inputs["method"]
	assert.True(t, methodInput.Required, "method must be required to prevent empty HTTP requests")
}

func TestHTTPRequestOperation_InputDescriptionsProvideGuidance(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Table-driven test for input description quality
	tests := []struct {
		inputName   string
		mustContain []string
		description string
	}{
		{
			inputName:   "method",
			mustContain: []string{"GET", "POST", "PUT", "DELETE"},
			description: "method description should list all supported methods",
		},
		{
			inputName:   "url",
			mustContain: []string{"http://", "https://"},
			description: "url description should specify protocol requirements",
		},
		{
			inputName:   "timeout",
			mustContain: []string{"seconds"},
			description: "timeout description should specify unit",
		},
		{
			inputName:   "retryable_status_codes",
			mustContain: []string{"429", "502", "503"},
			description: "retryable_status_codes description should provide examples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.inputName, func(t *testing.T) {
			input, exists := op.Inputs[tt.inputName]
			require.True(t, exists, "input %q should exist", tt.inputName)

			for _, keyword := range tt.mustContain {
				assert.Contains(t, input.Description, keyword,
					"%s: description should contain %q", tt.description, keyword)
			}
		})
	}
}

func TestHTTPRequestOperation_SupportsTemplateInterpolation(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// String inputs should support template interpolation (FR-008)
	// Verify url, method, body are string type
	stringInputs := []string{"url", "method", "body"}
	for _, inputName := range stringInputs {
		inputSchema := op.Inputs[inputName]
		assert.Equal(t, pluginmodel.InputTypeString, inputSchema.Type,
			"input %q should be string type to support template interpolation", inputName)
	}
}

func TestHTTPRequestOperation_FourMethodsSupported(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify all four methods mentioned (FR-001)
	methodInput := op.Inputs["method"]
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	for _, method := range methods {
		assert.Contains(t, methodInput.Description, method,
			"method description should mention %s", method)
	}
}

func TestHTTPRequestOperation_OutputsMatchFR003(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// FR-003: must return status_code, body, and headers in outputs
	requiredOutputs := []string{"status_code", "body", "headers"}

	for _, requiredOutput := range requiredOutputs {
		assert.Contains(t, op.Outputs, requiredOutput,
			"FR-003: output should include %q", requiredOutput)
	}
}

func TestHTTPRequestOperation_BodyTruncatedOutputExists(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// NFR-005: body_truncated output signals 1MB limit breach
	assert.Contains(t, op.Outputs, "body_truncated",
		"body_truncated output should exist to signal response body truncation")
}

func TestHTTPRequestOperation_FR001Compliance(t *testing.T) {
	// FR-001: The http.request operation must support HTTP methods: GET, POST, PUT, DELETE

	ops := AllOperations()
	require.Len(t, ops, 1, "FR-001: must expose single operation")

	op := ops[0]
	assert.Equal(t, "http.request", op.Name, "FR-001: operation must be named http.request")

	// Verify method input lists all four methods
	methodInput := op.Inputs["method"]
	assert.Contains(t, methodInput.Description, "GET", "FR-001: should support GET")
	assert.Contains(t, methodInput.Description, "POST", "FR-001: should support POST")
	assert.Contains(t, methodInput.Description, "PUT", "FR-001: should support PUT")
	assert.Contains(t, methodInput.Description, "DELETE", "FR-001: should support DELETE")
}

func TestHTTPRequestOperation_FR002Compliance(t *testing.T) {
	// FR-002: The operation must accept inputs: url (required, string), method (required, string),
	// headers (optional, object), body (optional, string), timeout (optional, integer, seconds),
	// retryable_status_codes (optional, array of integers)

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Required inputs
	assert.True(t, op.Inputs["url"].Required, "FR-002: url must be required")
	assert.Equal(t, pluginmodel.InputTypeString, op.Inputs["url"].Type, "FR-002: url must be string")

	assert.True(t, op.Inputs["method"].Required, "FR-002: method must be required")
	assert.Equal(t, pluginmodel.InputTypeString, op.Inputs["method"].Type, "FR-002: method must be string")

	// Optional inputs
	assert.False(t, op.Inputs["headers"].Required, "FR-002: headers must be optional")
	assert.Equal(t, pluginmodel.InputTypeObject, op.Inputs["headers"].Type, "FR-002: headers must be object")

	assert.False(t, op.Inputs["body"].Required, "FR-002: body must be optional")
	assert.Equal(t, pluginmodel.InputTypeString, op.Inputs["body"].Type, "FR-002: body must be string")

	assert.False(t, op.Inputs["timeout"].Required, "FR-002: timeout must be optional")
	assert.Equal(t, pluginmodel.InputTypeInteger, op.Inputs["timeout"].Type, "FR-002: timeout must be integer")

	assert.False(t, op.Inputs["retryable_status_codes"].Required, "FR-002: retryable_status_codes must be optional")
	assert.Equal(t, pluginmodel.InputTypeArray, op.Inputs["retryable_status_codes"].Type, "FR-002: retryable_status_codes must be array")
}

func TestHTTPRequestOperation_FR003Compliance(t *testing.T) {
	// FR-003: The operation must return outputs: status_code (integer), body (string),
	// headers (object mapping header names to values)

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify required outputs
	assert.Contains(t, op.Outputs, "status_code", "FR-003: outputs must include status_code")
	assert.Contains(t, op.Outputs, "body", "FR-003: outputs must include body")
	assert.Contains(t, op.Outputs, "headers", "FR-003: outputs must include headers")
}

func TestHTTPRequestOperation_FR005Compliance(t *testing.T) {
	// FR-005: The operation must validate that url is a non-empty string starting
	// with http:// or https://

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	urlInput := op.Inputs["url"]
	assert.Equal(t, "url", urlInput.Validation, "FR-005: url must have url validation rule")
	assert.Contains(t, urlInput.Description, "http://", "FR-005: description should mention http://")
	assert.Contains(t, urlInput.Description, "https://", "FR-005: description should mention https://")
}

func TestHTTPRequestOperation_FR006Compliance(t *testing.T) {
	// FR-006: The operation must apply timeout as a per-request deadline; default 30 seconds

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	timeoutInput := op.Inputs["timeout"]
	assert.Equal(t, 30, timeoutInput.Default, "FR-006: timeout must default to 30 seconds")
}

func TestHTTPRequestOperation_FR008Compliance(t *testing.T) {
	// FR-008: The operation must support template interpolation in all string inputs
	// (url, body, header values)

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// All string inputs support interpolation (AWF core feature)
	assert.Equal(t, pluginmodel.InputTypeString, op.Inputs["url"].Type, "FR-008: url must be string for interpolation")
	assert.Equal(t, pluginmodel.InputTypeString, op.Inputs["method"].Type, "FR-008: method must be string for interpolation")
	assert.Equal(t, pluginmodel.InputTypeString, op.Inputs["body"].Type, "FR-008: body must be string for interpolation")
	// Note: header values are within the object type, interpolation handled by AWF core
}

func TestHTTPRequestOperation_FR009Compliance(t *testing.T) {
	// FR-009: The operation must register under the http namespace as http.request

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	assert.Equal(t, "http.request", op.Name, "FR-009: operation must be named http.request")
	assert.True(t, len(op.Name) > 5, "FR-009: operation name should have http. prefix")
	assert.Contains(t, op.Name, "http.", "FR-009: operation name should contain http. namespace")
}

func TestHTTPRequestOperation_FR010Compliance(t *testing.T) {
	// FR-010: When retryable_status_codes is set and the response status matches,
	// the operation must return a retryable error

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify retryable_status_codes input exists and is documented
	retryableInput := op.Inputs["retryable_status_codes"]
	assert.NotEmpty(t, retryableInput.Description, "FR-010: retryable_status_codes should be documented")
	assert.Contains(t, retryableInput.Description, "retryable", "FR-010: description should mention retryable behavior")
}

func TestHTTPRequestOperation_NFR005Compliance(t *testing.T) {
	// NFR-005: Response body capture must be bounded (default 1MB max)

	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify body_truncated output exists (signals 1MB limit)
	assert.Contains(t, op.Outputs, "body_truncated", "NFR-005: body_truncated output must exist to signal 1MB limit")
}
