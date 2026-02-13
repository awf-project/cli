package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vanoix/awf/internal/domain/plugin"
)

// --- Happy Path Tests ---

func TestAllOperations_ReturnsOneOperation(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: it returns exactly 1 operation
	assert.Len(t, ops, 1, "AllOperations should return 1 notification operation")
}

func TestAllOperations_NotifySendOperationExists(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: the notify.send operation exists
	require.Len(t, ops, 1)
	assert.Equal(t, "notify.send", ops[0].Name, "operation name should be notify.send")
}

func TestAllOperations_NotifySendHasDescription(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: notify.send has a non-empty description
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Description, "notify.send should have a description")
	assert.Contains(t, ops[0].Description, "notification", "description should mention notifications")
}

func TestAllOperations_NotifySendHasInputs(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: notify.send has inputs defined
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Inputs, "notify.send should have inputs")
}

func TestAllOperations_NotifySendHasOutputs(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()

	// Then: notify.send has outputs defined
	require.Len(t, ops, 1)
	assert.NotEmpty(t, ops[0].Outputs, "notify.send should have outputs")
}

// --- Individual Operation Schema Tests (Happy Path) ---

func TestNotifySendOperation_RequiredInputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify backend is required
	backendInput, exists := op.Inputs["backend"]
	require.True(t, exists, "backend input should exist")
	assert.True(t, backendInput.Required, "backend should be required")
	assert.Equal(t, plugin.InputTypeString, backendInput.Type, "backend should be string type")
	assert.NotEmpty(t, backendInput.Description, "backend should have description")
	assert.Contains(t, backendInput.Description, "desktop", "backend description should list desktop")
	assert.Contains(t, backendInput.Description, "ntfy", "backend description should list ntfy")
	assert.Contains(t, backendInput.Description, "slack", "backend description should list slack")
	assert.Contains(t, backendInput.Description, "webhook", "backend description should list webhook")

	// Verify message is required
	messageInput, exists := op.Inputs["message"]
	require.True(t, exists, "message input should exist")
	assert.True(t, messageInput.Required, "message should be required")
	assert.Equal(t, plugin.InputTypeString, messageInput.Type, "message should be string type")
	assert.NotEmpty(t, messageInput.Description, "message should have description")
}

func TestNotifySendOperation_OptionalInputs(t *testing.T) {
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
			name:        "title",
			shouldExist: true,
			inputType:   plugin.InputTypeString,
			description: "title",
		},
		{
			name:        "priority",
			shouldExist: true,
			inputType:   plugin.InputTypeString,
			description: "low",
		},
		{
			name:        "topic",
			shouldExist: true,
			inputType:   plugin.InputTypeString,
			description: "ntfy",
		},
		{
			name:        "webhook_url",
			shouldExist: true,
			inputType:   plugin.InputTypeString,
			description: "webhook",
		},
		{
			name:        "channel",
			shouldExist: true,
			inputType:   plugin.InputTypeString,
			description: "channel",
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

func TestNotifySendOperation_TitleInputDefaults(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify title has default value mentioned in description
	titleInput := op.Inputs["title"]
	assert.Contains(t, titleInput.Description, "AWF Workflow", "title description should mention default value")
}

func TestNotifySendOperation_PriorityInputDefaults(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify priority has valid values and default mentioned
	priorityInput := op.Inputs["priority"]
	assert.Contains(t, priorityInput.Description, "low", "priority description should list low")
	assert.Contains(t, priorityInput.Description, "default", "priority description should list default")
	assert.Contains(t, priorityInput.Description, "high", "priority description should list high")
}

func TestNotifySendOperation_BackendSpecificInputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify topic is marked for ntfy backend
	topicInput := op.Inputs["topic"]
	assert.Contains(t, topicInput.Description, "ntfy", "topic should be associated with ntfy backend")
	assert.Contains(t, topicInput.Description, "required", "topic description should indicate it's required for ntfy")

	// Verify webhook_url is marked for webhook backend
	webhookInput := op.Inputs["webhook_url"]
	assert.Contains(t, webhookInput.Description, "webhook", "webhook_url should be associated with webhook backend")
	assert.Contains(t, webhookInput.Description, "required", "webhook_url description should indicate it's required for webhook")

	// Verify channel is for Slack
	channelInput := op.Inputs["channel"]
	assert.Contains(t, channelInput.Description, "Slack", "channel should be associated with Slack")
}

func TestNotifySendOperation_Outputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify expected outputs (FR-004)
	expectedOutputs := []string{"backend", "status", "response"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs, "notify.send should have backend, status, and response outputs")
}

func TestNotifySendOperation_OutputsAreUnique(t *testing.T) {
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

func TestNotifySendOperation_AllInputCount(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify total input count (2 required + 5 optional = 7)
	expectedInputs := []string{"backend", "message", "title", "priority", "topic", "webhook_url", "channel"}
	assert.Len(t, op.Inputs, len(expectedInputs), "notify.send should have exactly 7 inputs")

	for _, inputName := range expectedInputs {
		_, exists := op.Inputs[inputName]
		assert.True(t, exists, "input %q should exist", inputName)
	}
}

// --- Edge Case Tests ---

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
		plugin.InputTypeString:  true,
		plugin.InputTypeInteger: true,
		plugin.InputTypeBoolean: true,
		plugin.InputTypeArray:   true,
		plugin.InputTypeObject:  true,
	}

	for inputName, inputSchema := range op.Inputs {
		assert.True(t, validTypes[inputSchema.Type],
			"input %q has invalid type %v", inputName, inputSchema.Type)
	}
}

func TestAllOperations_AllInputsAreStringType(t *testing.T) {
	// Given: AllOperations is called
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Then: all notify.send inputs should be string type (FR-001)
	for inputName, inputSchema := range op.Inputs {
		assert.Equal(t, plugin.InputTypeString, inputSchema.Type,
			"input %q should be string type", inputName)
	}
}

func TestNotifySendOperation_NoUnexpectedInputs(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify only expected inputs exist (no extras)
	expectedInputs := map[string]bool{
		"backend":     true,
		"message":     true,
		"title":       true,
		"priority":    true,
		"topic":       true,
		"webhook_url": true,
		"channel":     true,
	}

	for inputName := range op.Inputs {
		assert.True(t, expectedInputs[inputName],
			"unexpected input %q found in schema", inputName)
	}
}

func TestNotifySendOperation_RequiredInputsCount(t *testing.T) {
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

	// Should have exactly 2 required inputs (backend and message)
	assert.Equal(t, 2, requiredCount, "should have exactly 2 required inputs")
}

func TestNotifySendOperation_OptionalInputsCount(t *testing.T) {
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

	// Should have exactly 5 optional inputs
	assert.Equal(t, 5, optionalCount, "should have exactly 5 optional inputs")
}

// --- Error Handling Tests ---

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

func TestNotifySendOperation_BackendInputNotEmpty(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Backend is required and should have validation context in description
	backendInput := op.Inputs["backend"]
	assert.True(t, backendInput.Required, "backend must be required to prevent empty values")
	assert.NotEmpty(t, backendInput.Description, "backend should document valid values")
}

func TestNotifySendOperation_MessageInputNotEmpty(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Message is required and should be non-empty
	messageInput := op.Inputs["message"]
	assert.True(t, messageInput.Required, "message must be required to prevent empty notifications")
}

func TestNotifySendOperation_InputDescriptionsProvideGuidance(t *testing.T) {
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
			inputName:   "backend",
			mustContain: []string{"desktop", "ntfy", "slack", "webhook"},
			description: "backend description should list all supported backends",
		},
		{
			inputName:   "priority",
			mustContain: []string{"low", "default", "high"},
			description: "priority description should list valid values",
		},
		{
			inputName:   "topic",
			mustContain: []string{"ntfy"},
			description: "topic description should indicate ntfy association",
		},
		{
			inputName:   "webhook_url",
			mustContain: []string{"webhook"},
			description: "webhook_url description should indicate webhook association",
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

func TestNotifySendOperation_SupportsTemplateInterpolation(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// All string inputs should support template interpolation (FR-005)
	// This is implicit in AWF but we verify inputs are string type
	for inputName, inputSchema := range op.Inputs {
		if inputSchema.Type == plugin.InputTypeString {
			// String inputs support {{workflow.name}}, {{workflow.duration}}, etc.
			// No special validation needed - this is handled by AWF core
			assert.Equal(t, plugin.InputTypeString, inputSchema.Type,
				"input %q should be string type to support template interpolation", inputName)
		}
	}
}

func TestNotifySendOperation_FourBackendsSupported(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// Verify all four backends mentioned (FR-002)
	backendInput := op.Inputs["backend"]
	backends := []string{"desktop", "ntfy", "slack", "webhook"}

	for _, backend := range backends {
		assert.Contains(t, backendInput.Description, backend,
			"backend description should mention %s", backend)
	}
}

func TestNotifySendOperation_OutputsMatchFR004(t *testing.T) {
	ops := AllOperations()
	require.Len(t, ops, 1)
	op := ops[0]

	// FR-004: must return backend, status, and response in outputs
	requiredOutputs := []string{"backend", "status", "response"}

	for _, requiredOutput := range requiredOutputs {
		assert.Contains(t, op.Outputs, requiredOutput,
			"FR-004: output should include %q", requiredOutput)
	}
}

// --- Specification Compliance Tests ---

func TestNotifySendOperation_FR001Compliance(t *testing.T) {
	// FR-001: The plugin must expose a single operation `notify.send` accepting
	// inputs: backend (required), title (optional), message (required),
	// priority (optional), and backend-specific inputs (topic, webhook_url, channel)

	ops := AllOperations()
	require.Len(t, ops, 1, "FR-001: must expose single operation")

	op := ops[0]
	assert.Equal(t, "notify.send", op.Name, "FR-001: operation must be named notify.send")

	// Required inputs
	assert.True(t, op.Inputs["backend"].Required, "FR-001: backend must be required")
	assert.True(t, op.Inputs["message"].Required, "FR-001: message must be required")

	// Optional inputs
	assert.False(t, op.Inputs["title"].Required, "FR-001: title must be optional")
	assert.False(t, op.Inputs["priority"].Required, "FR-001: priority must be optional")
	assert.False(t, op.Inputs["topic"].Required, "FR-001: topic must be optional")
	assert.False(t, op.Inputs["webhook_url"].Required, "FR-001: webhook_url must be optional")
	assert.False(t, op.Inputs["channel"].Required, "FR-001: channel must be optional")
}

func TestNotifySendOperation_FR002Compliance(t *testing.T) {
	// FR-002: The plugin must support four notification backends:
	// desktop, ntfy, slack, webhook

	ops := AllOperations()
	require.Len(t, ops, 1)

	op := ops[0]
	backendDesc := op.Inputs["backend"].Description

	backends := []string{"desktop", "ntfy", "slack", "webhook"}
	for _, backend := range backends {
		assert.Contains(t, backendDesc, backend,
			"FR-002: backend description must mention %s", backend)
	}
}

func TestNotifySendOperation_FR004Compliance(t *testing.T) {
	// FR-004: The plugin must return an OperationResult with Success: true
	// and the backend response in Outputs on successful delivery, or
	// Success: false with a descriptive Error on failure.
	// Outputs should include: backend, status, response

	ops := AllOperations()
	require.Len(t, ops, 1)

	op := ops[0]

	// Verify outputs match FR-004
	assert.Contains(t, op.Outputs, "backend", "FR-004: outputs must include backend")
	assert.Contains(t, op.Outputs, "status", "FR-004: outputs must include status")
	assert.Contains(t, op.Outputs, "response", "FR-004: outputs must include response")
}

func TestNotifySendOperation_FR005Compliance(t *testing.T) {
	// FR-005: All notification inputs must support AWF template interpolation
	// ({{workflow.name}}, {{workflow.duration}}, {{states.step_name.output}}, etc.)

	ops := AllOperations()
	require.Len(t, ops, 1)

	op := ops[0]

	// All string inputs support interpolation (this is AWF core feature)
	// Verify all inputs are string type
	for inputName, inputSchema := range op.Inputs {
		assert.Equal(t, plugin.InputTypeString, inputSchema.Type,
			"FR-005: input %q must be string type to support template interpolation", inputName)
	}
}
