package github

import (
	"testing"

	"github.com/awf-project/awf/internal/domain/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllOperations_ReturnsEightOperations(t *testing.T) {
	ops := AllOperations()
	assert.Len(t, ops, 8)
}

func TestAllOperations_AllOperationsHaveValidNames(t *testing.T) {
	ops := AllOperations()

	expectedNames := map[string]bool{
		"github.get_issue":     true,
		"github.get_pr":        true,
		"github.create_pr":     true,
		"github.create_issue":  true,
		"github.add_labels":    true,
		"github.list_comments": true,
		"github.add_comment":   true,
		"github.batch":         true,
	}

	for _, op := range ops {
		assert.True(t, expectedNames[op.Name], "operation name %q not in expected set", op.Name)
		delete(expectedNames, op.Name)
	}

	assert.Empty(t, expectedNames)
}

func TestAllOperations_AllOperationsHaveDescriptions(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		assert.NotEmpty(t, op.Description, "operation %q should have a description", op.Name)
	}
}

func TestAllOperations_AllOperationsHaveInputs(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		assert.NotEmpty(t, op.Inputs, "operation %q should have inputs", op.Name)
	}
}

func TestAllOperations_AllOperationsHaveOutputs(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		assert.NotEmpty(t, op.Outputs, "operation %q should have outputs", op.Name)
	}
}

// --- Individual Operation Schema Tests (Happy Path) ---

func TestGetIssueOperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.get_issue")

	assert.True(t, op.Inputs["number"].Required)
	assert.Equal(t, plugin.InputTypeInteger, op.Inputs["number"].Type)

	assert.False(t, op.Inputs["repo"].Required)
	assert.Equal(t, plugin.InputTypeString, op.Inputs["repo"].Type)

	assert.False(t, op.Inputs["fields"].Required)
	assert.Equal(t, plugin.InputTypeArray, op.Inputs["fields"].Type)

	expectedOutputs := []string{"number", "title", "body", "state", "labels"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs)
}

func TestGetPROperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.get_pr")

	assert.True(t, op.Inputs["number"].Required)
	assert.Equal(t, plugin.InputTypeInteger, op.Inputs["number"].Type)

	assert.False(t, op.Inputs["repo"].Required)
	assert.False(t, op.Inputs["fields"].Required)

	assert.Contains(t, op.Outputs, "headRefName")
	assert.Contains(t, op.Outputs, "baseRefName")
	assert.Contains(t, op.Outputs, "mergeable")
	assert.Contains(t, op.Outputs, "mergedAt")
}

func TestCreatePROperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.create_pr")

	assert.True(t, op.Inputs["title"].Required)
	assert.True(t, op.Inputs["head"].Required)
	assert.True(t, op.Inputs["base"].Required)

	assert.False(t, op.Inputs["body"].Required)
	assert.False(t, op.Inputs["repo"].Required)
	assert.False(t, op.Inputs["draft"].Required)

	assert.Equal(t, plugin.InputTypeBoolean, op.Inputs["draft"].Type)

	assert.Contains(t, op.Outputs, "already_exists")
	assert.Contains(t, op.Outputs, "number")
	assert.Contains(t, op.Outputs, "url")
}

func TestCreateIssueOperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.create_issue")

	assert.True(t, op.Inputs["title"].Required)

	assert.False(t, op.Inputs["body"].Required)
	assert.False(t, op.Inputs["labels"].Required)
	assert.False(t, op.Inputs["assignees"].Required)

	assert.Equal(t, plugin.InputTypeArray, op.Inputs["labels"].Type)
	assert.Equal(t, plugin.InputTypeArray, op.Inputs["assignees"].Type)

	expectedOutputs := []string{"number", "url"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs)
}

func TestAddLabelsOperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.add_labels")

	assert.True(t, op.Inputs["number"].Required)
	assert.True(t, op.Inputs["labels"].Required)
	assert.Equal(t, plugin.InputTypeArray, op.Inputs["labels"].Type)

	assert.False(t, op.Inputs["repo"].Required)

	assert.Contains(t, op.Outputs, "labels")
}

func TestListCommentsOperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.list_comments")

	assert.True(t, op.Inputs["number"].Required)

	assert.False(t, op.Inputs["repo"].Required)
	assert.False(t, op.Inputs["limit"].Required)
	assert.Equal(t, plugin.InputTypeInteger, op.Inputs["limit"].Type)

	expectedOutputs := []string{"comments", "total"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs)
}

func TestAddCommentOperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.add_comment")

	assert.True(t, op.Inputs["number"].Required)
	assert.True(t, op.Inputs["body"].Required)

	assert.False(t, op.Inputs["repo"].Required)

	expectedOutputs := []string{"comment_id", "url"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs)
}

func TestBatchOperation_Schema(t *testing.T) {
	ops := AllOperations()
	op := findOperation(t, ops, "github.batch")

	assert.True(t, op.Inputs["operations"].Required)
	assert.Equal(t, plugin.InputTypeArray, op.Inputs["operations"].Type)

	assert.False(t, op.Inputs["strategy"].Required)
	assert.False(t, op.Inputs["concurrency"].Required)

	assert.Equal(t, plugin.InputTypeString, op.Inputs["strategy"].Type)
	assert.Equal(t, plugin.InputTypeInteger, op.Inputs["concurrency"].Type)

	expectedOutputs := []string{"total", "succeeded", "failed", "results"}
	assert.ElementsMatch(t, expectedOutputs, op.Outputs)
}

func TestAllOperations_ImmutabilityCheck(t *testing.T) {
	ops1 := AllOperations()
	ops2 := AllOperations()

	require.Len(t, ops1, len(ops2))
	for i := range ops1 {
		assert.Equal(t, ops1[i].Name, ops2[i].Name)
		assert.Equal(t, ops1[i].Description, ops2[i].Description)
		assert.Equal(t, len(ops1[i].Inputs), len(ops2[i].Inputs))
		assert.Equal(t, len(ops1[i].Outputs), len(ops2[i].Outputs))
	}
}

func TestAllOperations_NoEmptyNames(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		assert.NotEmpty(t, op.Name, "operation name should not be empty")
	}
}

func TestAllOperations_AllInputsHaveDescriptions(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		for inputName, inputSchema := range op.Inputs {
			assert.NotEmpty(t, inputSchema.Description,
				"input %q of operation %q should have a description",
				inputName, op.Name)
		}
	}
}

func TestAllOperations_AllInputsHaveValidTypes(t *testing.T) {
	ops := AllOperations()

	validTypes := map[string]bool{
		plugin.InputTypeString:  true,
		plugin.InputTypeInteger: true,
		plugin.InputTypeBoolean: true,
		plugin.InputTypeArray:   true,
	}

	for _, op := range ops {
		for inputName, inputSchema := range op.Inputs {
			assert.True(t, validTypes[inputSchema.Type],
				"input %q of operation %q has invalid type %v",
				inputName, op.Name, inputSchema.Type)
		}
	}
}

func TestAllOperations_RepoInputIsConsistent(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		if repoInput, hasRepo := op.Inputs["repo"]; hasRepo {
			assert.False(t, repoInput.Required,
				"operation %q should have optional repo input", op.Name)
			assert.Equal(t, plugin.InputTypeString, repoInput.Type,
				"operation %q repo input should be string type", op.Name)
			assert.Contains(t, repoInput.Description, "auto-detected",
				"operation %q repo description should mention auto-detection", op.Name)
		}
	}
}

func TestAllOperations_NumberInputIsConsistent(t *testing.T) {
	ops := AllOperations()

	// Operations that should have a 'number' input
	numberOps := []string{
		"github.get_issue",
		"github.get_pr",
		"github.add_labels",
		"github.list_comments",
		"github.add_comment",
	}

	for _, opName := range numberOps {
		op := findOperation(t, ops, opName)
		numberInput, hasNumber := op.Inputs["number"]
		require.True(t, hasNumber, "operation %q should have a number input", opName)
		assert.True(t, numberInput.Required, "operation %q number should be required", opName)
		assert.Equal(t, plugin.InputTypeInteger, numberInput.Type,
			"operation %q number should be integer type", opName)
	}
}

// --- Error Handling Tests ---

func TestAllOperations_NoDuplicateNames(t *testing.T) {
	ops := AllOperations()

	seen := make(map[string]bool)
	for _, op := range ops {
		assert.False(t, seen[op.Name], "duplicate operation name: %q", op.Name)
		seen[op.Name] = true
	}
}

func TestAllOperations_NoDuplicateOutputFields(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		seen := make(map[string]bool)
		for _, outputField := range op.Outputs {
			assert.False(t, seen[outputField],
				"operation %q has duplicate output field: %q", op.Name, outputField)
			seen[outputField] = true
		}
	}
}

func TestAllOperations_NoEmptyOutputFields(t *testing.T) {
	ops := AllOperations()

	for _, op := range ops {
		for _, outputField := range op.Outputs {
			assert.NotEmpty(t, outputField,
				"operation %q has an empty output field", op.Name)
		}
	}
}

func TestAllOperations_ValidInputCombinations(t *testing.T) {
	// Table-driven test for input validation rules
	tests := []struct {
		name           string
		operationName  string
		requiredInputs []string
		optionalInputs []string
	}{
		{
			name:           "get_issue has number required, repo and fields optional",
			operationName:  "github.get_issue",
			requiredInputs: []string{"number"},
			optionalInputs: []string{"repo", "fields"},
		},
		{
			name:           "create_pr has title/head/base required",
			operationName:  "github.create_pr",
			requiredInputs: []string{"title", "head", "base"},
			optionalInputs: []string{"body", "repo", "draft"},
		},
		{
			name:           "batch has operations required, strategy and concurrency optional",
			operationName:  "github.batch",
			requiredInputs: []string{"operations"},
			optionalInputs: []string{"strategy", "concurrency"},
		},
	}

	ops := AllOperations()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := findOperation(t, ops, tt.operationName)

			for _, inputName := range tt.requiredInputs {
				inputSchema, exists := op.Inputs[inputName]
				require.True(t, exists, "required input %q not found", inputName)
				assert.True(t, inputSchema.Required, "input %q should be required", inputName)
			}

			for _, inputName := range tt.optionalInputs {
				inputSchema, exists := op.Inputs[inputName]
				require.True(t, exists, "optional input %q not found", inputName)
				assert.False(t, inputSchema.Required, "input %q should be optional", inputName)
			}
		})
	}
}

// findOperation finds an operation by name in the slice
func findOperation(t *testing.T, ops []plugin.OperationSchema, name string) plugin.OperationSchema {
	t.Helper()
	for _, op := range ops {
		if op.Name == name {
			return op
		}
	}
	require.Fail(t, "operation not found", "operation %q not found in slice", name)
	return plugin.OperationSchema{}
}
